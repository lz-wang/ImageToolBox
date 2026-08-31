package convert

import (
	"fmt"
	"image/color"
	"path/filepath"
	"strings"

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

	// 输入统一走 imageio.OpenStatic：严格限定 JPEG/PNG/WebP，并把
	// JPEG EXIF Orientation 烘焙进像素。所有 transform（convert/
	// resize/crop/watermark）共用同一入口，CLI、HTTP 与 Domain 的
	// 格式契约和 orientation 行为一致。
	img, err := imageio.OpenStatic(inputPath)
	if err != nil {
		return err
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
