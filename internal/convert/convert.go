package convert

import (
	"fmt"
	"image/color"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"imagetoolbox/internal/imageio"
)

type Options struct {
	To         string
	Quality    int
	Lossless   bool
	Background string
}

const (
	DefaultQuality    = 80
	DefaultBackground = "#FFFFFF"
)

// Normalize applies the domain defaults shared by every adapter.
func (o *Options) Normalize() {
	if o.Quality == 0 {
		o.Quality = DefaultQuality
	}
	if strings.TrimSpace(o.Background) == "" {
		o.Background = DefaultBackground
	}
	o.To = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(o.To), "."))
}

// Validate verifies conversion options before decoding the input image.
//
// 参数语义按目标格式收口：
//   - quality 对 JPEG/WebP 生效，PNG 忽略（无需区分用户显式传入与默认值）；
//   - lossless 仅 WebP 有实际意义；PNG 本身始终无损，作为兼容性 no-op 接受；
//   - background 只在输出 JPEG（不支持 Alpha、必须铺底）时生效并被校验。
func (o Options) Validate() error {
	format, err := imageio.NormalizeFormat(o.To)
	if err != nil {
		return err
	}
	if o.Quality < 1 || o.Quality > 100 {
		return fmt.Errorf("quality must be between 1 and 100")
	}
	if o.Lossless && format != imageio.FormatPNG && format != imageio.FormatWEBP {
		return fmt.Errorf("lossless is only supported for png and webp")
	}
	if format == imageio.FormatJPEG {
		background, err := imageio.ParseHexColor(o.Background)
		if err != nil {
			return fmt.Errorf("invalid background color: %w", err)
		}
		// JPEG 没有透明背景；零值颜色（如 #00000000）还会被 imageio
		// Encode 当作"未设置"而静默变成默认白色，必须在领域层拒绝。
		if background.A != 255 {
			return fmt.Errorf("background color must be opaque for jpeg output")
		}
	}
	return nil
}

func ConvertFile(inputPath, outputPath string, opts Options) error {
	opts.Normalize()
	if err := opts.Validate(); err != nil {
		return err
	}
	format, err := imageio.NormalizeFormat(opts.To)
	if err != nil {
		return err
	}

	// 输入严格限定 JPEG/PNG/WebP：imaging 能解码 GIF/BMP/TIFF 等更多
	// 格式，放行会造成 animated GIF → 首帧这类静默语义损失。
	if _, err := imageio.DetectFormat(inputPath); err != nil {
		return fmt.Errorf("unsupported input image: %w", err)
	}

	// JPEG EXIF Orientation 应用到实际像素后输出，结果不依赖
	// Orientation metadata（imaging 仅解析 JPEG EXIF；WebP 携带的
	// orientation 元数据当前不处理）。仅 convert 开启：
	// resize/crop/watermark 的 Probe →
	// Resolve 资源准入基于原始物理尺寸，decode 旋转会造成推导与实际
	// 输出不一致，待统一的 oriented probe 落地后再放开。
	img, err := imaging.Open(inputPath, imaging.AutoOrientation(true))
	if err != nil {
		return fmt.Errorf("open input image: %w", err)
	}

	var background color.NRGBA
	if format == imageio.FormatJPEG {
		background, err = imageio.ParseHexColor(opts.Background)
		if err != nil {
			return fmt.Errorf("invalid background color: %w", err)
		}
		if background.A != 255 {
			return fmt.Errorf("background color must be opaque for jpeg output")
		}
	}

	return imageio.SaveWithFormat(outputPath, img, format, imageio.SaveOptions{
		Quality:    opts.Quality,
		Lossless:   opts.Lossless,
		Background: background,
	})
}

func DefaultOutputPath(inputPath string, to string) (string, error) {
	format, err := imageio.NormalizeFormat(to)
	if err != nil {
		return "", err
	}

	ext := "." + string(format)
	return filepath.Join(filepath.Dir(inputPath), imageio.SuffixedName(filepath.Base(inputPath), "_converted", ext)), nil
}
