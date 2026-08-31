package inspect

import (
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/deepteams/webp"
)

func File(path string, opts Options) (*Result, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("读取文件信息失败: %w", err)
	}
	if stat.IsDir() {
		return nil, fmt.Errorf("输入路径是目录，不是图片文件: %s", path)
	}

	absPath, _ := filepath.Abs(path)

	header, err := readHeader(path, 512)
	if err != nil {
		return nil, err
	}

	result := &Result{
		SchemaVersion: SchemaVersion,
		File: FileInfo{
			Path:       path,
			AbsPath:    absPath,
			Name:       filepath.Base(path),
			Ext:        strings.ToLower(filepath.Ext(path)),
			SizeBytes:  stat.Size(),
			Mode:       stat.Mode().String(),
			ModifiedAt: stat.ModTime(),
			MIMEType:   http.DetectContentType(header),
			MagicHex:   firstHex(header, 4),
		},
		Warnings: []string{},
	}

	if !opts.NoHash {
		hashes, err := ComputeAllHashes(path)
		if err != nil {
			return nil, err
		}
		result.Hashes = hashes
	}

	imgInfo, decodeErr := decodeImageConfig(path)
	if decodeErr != nil {
		if opts.Strict {
			return nil, fmt.Errorf("解析图片元数据失败: %w", decodeErr)
		}

		result.Error = &InfoError{
			Code:    "decode_config_failed",
			Message: decodeErr.Error(),
		}
	} else {
		result.Image = imgInfo
		// WebP 动画信息来自 VP8X 头嗅探，无需完整解码即可断言
		if imgInfo.Format == "webp" {
			animated, known := webpAnimation(header)
			imgInfo.Animated = animated
			imgInfo.AnimationKnown = known
		}
	}

	// 完整解码：捕获"header 正常但文件后半部分损坏"，GIF 额外解析
	// 帧数与动画状态
	if opts.FullDecode && result.Image != nil {
		if err := applyFullDecode(path, result.Image); err != nil {
			if opts.Strict {
				return nil, fmt.Errorf("完整解码失败: %w", err)
			}
			result.Warnings = append(result.Warnings, fmt.Sprintf("full decode failed: %v", err))
		}
	}

	if opts.Detail {
		detail := &DetailInfo{
			MagicBytes:  firstHex(header, 10),
			HeaderBytes: firstHex(header, 32),
			DetectedBy:  "image.DecodeConfig",
		}

		if result.Image != nil {
			detail.ExtensionMatchesFormat = extensionMatchesFormat(result.File.Ext, result.Image.Format)
		}

		result.Detail = detail
	}

	return result, nil
}

func decodeImageConfig(path string) (*ImageInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开图片失败: %w", err)
	}
	defer f.Close()

	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return nil, err
	}

	info := &ImageInfo{
		Format:         format,
		Width:          cfg.Width,
		Height:         cfg.Height,
		AspectRatio:    aspectRatio(cfg.Width, cfg.Height),
		Megapixels:     float64(cfg.Width*cfg.Height) / 1_000_000,
		ColorModel:     fmt.Sprintf("%T", cfg.ColorModel),
		HasAlpha:       hasAlpha(cfg.ColorModel),
		DecodeConfigOK: true,
	}
	// JPEG/PNG 是静态格式，header 阶段即可断言非动画；
	// GIF 需要完整解码数帧，WebP 由 VP8X 嗅探判断
	switch format {
	case "jpeg", "png":
		info.AnimationKnown = true
	}
	return info, nil
}

func readHeader(path string, n int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	buf := make([]byte, n)
	readN, err := f.Read(buf)
	if err != nil && readN == 0 {
		return nil, fmt.Errorf("读取文件头失败: %w", err)
	}

	return buf[:readN], nil
}

func firstHex(data []byte, n int) string {
	if len(data) == 0 {
		return ""
	}
	if len(data) < n {
		n = len(data)
	}
	return hex.EncodeToString(data[:n])
}

func aspectRatio(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	g := gcd(width, height)
	return fmt.Sprintf("%d:%d", width/g, height/g)
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	return a
}

func hasAlpha(model color.Model) bool {
	switch model {
	case color.AlphaModel,
		color.Alpha16Model,
		color.NRGBAModel,
		color.NRGBA64Model,
		color.RGBAModel,
		color.RGBA64Model:
		return true
	default:
		return false
	}
}

func extensionMatchesFormat(ext string, format string) bool {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	format = strings.ToLower(format)

	switch ext {
	case "jpg", "jpeg":
		return format == "jpeg"
	case "png":
		return format == "png"
	case "gif":
		return format == "gif"
	case "webp":
		return format == "webp"
	default:
		return false
	}
}
