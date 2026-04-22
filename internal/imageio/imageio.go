package imageio

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/deepteams/webp"
)

type Format string

const (
	FormatJPEG Format = "jpeg"
	FormatPNG  Format = "png"
	FormatWEBP Format = "webp"
)

type SaveOptions struct {
	Quality    int
	Lossless   bool
	Background color.NRGBA
	Flatten    bool
}

func NormalizeFormat(value string) (Format, error) {
	switch strings.ToLower(strings.TrimPrefix(strings.TrimSpace(value), ".")) {
	case "jpg", "jpeg":
		return FormatJPEG, nil
	case "png":
		return FormatPNG, nil
	case "webp":
		return FormatWEBP, nil
	default:
		return "", fmt.Errorf("unsupported image format: %s", value)
	}
}

func FormatFromPath(path string) (Format, error) {
	return NormalizeFormat(filepath.Ext(path))
}

func DetectFormat(path string) (Format, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, format, err := image.DecodeConfig(f)
	if err != nil {
		return "", err
	}

	return NormalizeFormat(format)
}

func Save(path string, img image.Image, opts SaveOptions) error {
	format, err := FormatFromPath(path)
	if err != nil {
		return err
	}
	return SaveWithFormat(path, img, format, opts)
}

func SaveWithFormat(path string, img image.Image, format Format, opts SaveOptions) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	return Encode(out, img, format, opts)
}

func Encode(w io.Writer, img image.Image, format Format, opts SaveOptions) error {
	quality := opts.Quality
	if quality <= 0 {
		quality = 100
	}
	background := opts.Background
	if background == (color.NRGBA{}) {
		background = color.NRGBA{255, 255, 255, 255}
	}

	switch format {
	case FormatJPEG:
		flattened := Flatten(img, background)
		return jpeg.Encode(w, flattened, &jpeg.Options{Quality: quality})
	case FormatPNG:
		if opts.Flatten {
			return png.Encode(w, Flatten(img, background))
		}
		return png.Encode(w, img)
	case FormatWEBP:
		encodeImg := img
		if opts.Flatten {
			encodeImg = Flatten(img, background)
		}
		return encodeWEBP(w, encodeImg, opts.Lossless, quality)
	default:
		return fmt.Errorf("unsupported image format: %s", format)
	}
}

func encodeWEBP(w io.Writer, img image.Image, lossless bool, quality int) error {
	return webp.Encode(w, img, &webp.EncoderOptions{
		Lossless: lossless,
		Quality:  float32(quality),
		Method:   4,
	})
}

func SupportsWEBPEncoding() bool {
	return true
}

func Flatten(img image.Image, bg color.NRGBA) image.Image {
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, &image.Uniform{C: bg}, image.Point{}, draw.Src)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Over)
	return rgba
}

func ParseHexColor(s string) (color.NRGBA, error) {
	str := strings.TrimSpace(s)
	if str == "" {
		return color.NRGBA{}, fmt.Errorf("color must not be empty")
	}
	str = strings.TrimPrefix(str, "#")
	switch len(str) {
	case 3:
		str = fmt.Sprintf("%c%c%c%c%c%c", str[0], str[0], str[1], str[1], str[2], str[2])
	case 6, 8:
	default:
		return color.NRGBA{}, fmt.Errorf("invalid color format: %q", s)
	}

	var r, g, b, a uint8
	hexRGB := str
	if len(str) == 8 {
		hexRGB = str[:6]
	}
	if _, err := fmt.Sscanf(hexRGB, "%02x%02x%02x", &r, &g, &b); err != nil {
		return color.NRGBA{}, err
	}
	if len(str) == 8 {
		if _, err := fmt.Sscanf(str[6:], "%02x", &a); err != nil {
			return color.NRGBA{}, err
		}
	} else {
		a = 255
	}

	return color.NRGBA{R: r, G: g, B: b, A: a}, nil
}
