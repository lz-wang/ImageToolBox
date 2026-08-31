package convert

import (
	"fmt"
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
	if _, err := imageio.ParseHexColor(o.Background); err != nil {
		return fmt.Errorf("invalid background color: %w", err)
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

	img, err := imaging.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input image: %w", err)
	}

	background, err := imageio.ParseHexColor(opts.Background)
	if err != nil {
		return fmt.Errorf("invalid background color: %w", err)
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
