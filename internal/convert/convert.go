package convert

import (
	"fmt"
	"image/color"
	"strings"

	"imagetoolbox/internal/imageio"
)

type Options struct {
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
}

type resolvedOptions struct {
	format     imageio.Format
	quality    int
	lossless   bool
	background color.NRGBA
}

func resolveOptions(outputPath string, opts Options) (resolvedOptions, error) {
	opts.Normalize()
	if opts.Quality < 1 || opts.Quality > 100 {
		return resolvedOptions{}, fmt.Errorf("quality must be between 1 and 100")
	}

	format, err := imageio.FormatFromPath(outputPath)
	if err != nil {
		return resolvedOptions{}, err
	}
	if opts.Lossless && format != imageio.FormatPNG && format != imageio.FormatWEBP {
		return resolvedOptions{}, fmt.Errorf("lossless is only supported for png and webp")
	}

	resolved := resolvedOptions{format: format, quality: opts.Quality, lossless: opts.Lossless}
	if format != imageio.FormatJPEG {
		return resolved, nil
	}

	background, err := imageio.ParseHexColor(opts.Background)
	if err != nil {
		return resolvedOptions{}, fmt.Errorf("invalid background color: %w", err)
	}
	// JPEG 没有透明背景；零值颜色（如 #00000000）还会被 imageio Encode
	// 当作“未设置”而静默变成默认白色，必须在领域层拒绝。
	if background.A != 255 {
		return resolvedOptions{}, fmt.Errorf("background color must be opaque for jpeg output")
	}
	resolved.background = background
	return resolved, nil
}

func ConvertFile(inputPath, outputPath string, opts Options) error {
	resolved, err := resolveOptions(outputPath, opts)
	if err != nil {
		return err
	}
	if err := imageio.RejectSameFile(inputPath, outputPath); err != nil {
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

	return imageio.SaveWithFormat(outputPath, img, resolved.format, imageio.SaveOptions{
		Quality:    resolved.quality,
		Lossless:   resolved.lossless,
		Background: resolved.background,
	})
}
