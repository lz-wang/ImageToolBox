package watermark

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"imagetoolbox/internal/imageio"
)

func TestAddImageWatermark(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.png")
	logo := filepath.Join(dir, "logo.png")
	output := filepath.Join(dir, "output.png")

	base := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	fill(base, color.NRGBA{255, 255, 255, 255})
	writePNG(t, input, base)

	wm := image.NewNRGBA(image.Rect(0, 0, 20, 20))
	fill(wm, color.NRGBA{255, 0, 0, 255})
	writePNG(t, logo, wm)

	got, err := AddImageWatermark(input, output, &ImageOptions{
		ImagePath:  logo,
		Position:   BottomRight,
		ScaleRatio: float64Ptr(0.2),
		Opacity:    float64Ptr(1),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r, g, b, _ := got.At(95, 95).RGBA()
	if r == 0 || g != 0 || b != 0 {
		t.Fatal("expected bottom-right watermark pixel to be red")
	}
}

func TestAddFileDispatchValidation(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		wantErr bool
	}{
		{name: "neither source", wantErr: true},
		{name: "both sources", opts: Options{Text: "mark", ImagePath: "logo.png"}, wantErr: true},
		{name: "image repeat", opts: Options{ImagePath: "logo.png", Mode: ModeRepeat}, wantErr: true},
		{name: "invalid mode", opts: Options{Text: "mark", Mode: Mode("invalid")}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AddFile("input.png", "output.png", tt.opts)
			if (err != nil) != tt.wantErr {
				t.Fatalf("AddFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAddFileRejectsSameFile(t *testing.T) {
	input := filepath.Join(t.TempDir(), "input.png")
	if err := os.WriteFile(input, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := AddFile(input, input, Options{Text: "mark"})
	if !errors.Is(err, imageio.ErrSameFile) {
		t.Fatalf("AddFile() error = %v, want imageio.ErrSameFile", err)
	}
}

func TestAddFileTextPositionAndRepeat(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.png")
	writePNG(t, input, image.NewNRGBA(image.Rect(0, 0, 100, 100)))

	for _, opts := range []Options{{Text: "mark"}, {Text: "mark", Mode: ModeRepeat}} {
		output := filepath.Join(dir, string(opts.Mode)+".png")
		if err := AddFile(input, output, opts); err != nil {
			t.Fatalf("AddFile(%q) error = %v", opts.Mode, err)
		}
		if _, err := os.Stat(output); err != nil {
			t.Fatalf("output was not created: %v", err)
		}
	}
}

func fill(img *image.NRGBA, c color.NRGBA) {
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
}

func TestOptionsValidate(t *testing.T) {
	intPtr := func(v int) *int { return &v }
	tests := []struct {
		name    string
		opts    Options
		wantErr bool
	}{
		{name: "text defaults", opts: Options{Text: "mark"}},
		{name: "image defaults", opts: Options{ImagePath: "logo.png"}},
		{name: "normalized mode position", opts: Options{Text: "mark", Mode: ModePosition}},
		{name: "opacity zero", opts: Options{Text: "mark", Opacity: float64Ptr(0)}},
		{name: "opacity one", opts: Options{Text: "mark", Opacity: float64Ptr(1)}},
		{name: "opacity negative", opts: Options{Text: "mark", Opacity: float64Ptr(-0.1)}, wantErr: true},
		{name: "opacity above one", opts: Options{Text: "mark", Opacity: float64Ptr(1.1)}, wantErr: true},
		{name: "opacity NaN", opts: Options{Text: "mark", Opacity: float64Ptr(math.NaN())}, wantErr: true},
		{name: "opacity Inf", opts: Options{Text: "mark", Opacity: float64Ptr(math.Inf(1))}, wantErr: true},
		{name: "invalid position", opts: Options{Text: "mark", Position: Position("middle")}, wantErr: true},
		{name: "font size max", opts: Options{Text: "mark", FontSize: intPtr(MaxFontSize)}},
		{name: "font size too large", opts: Options{Text: "mark", FontSize: intPtr(MaxFontSize + 1)}, wantErr: true},
		{name: "font size negative", opts: Options{Text: "mark", FontSize: intPtr(-1)}, wantErr: true},
		{name: "space negative", opts: Options{Text: "mark", Space: intPtr(-1)}, wantErr: true},
		{name: "angle boundary", opts: Options{Text: "mark", Mode: ModeRepeat, Angle: intPtr(-360)}},
		{name: "angle out of range", opts: Options{Text: "mark", Mode: ModeRepeat, Angle: intPtr(361)}, wantErr: true},
		{name: "margin negative", opts: Options{Text: "mark", Margin: float64Ptr(-0.1)}, wantErr: true},
		{name: "margin NaN", opts: Options{Text: "mark", Margin: float64Ptr(math.NaN())}, wantErr: true},
		{name: "scale zero", opts: Options{ImagePath: "logo.png", Scale: float64Ptr(0)}, wantErr: true},
		{name: "scale NaN", opts: Options{ImagePath: "logo.png", Scale: float64Ptr(math.NaN())}, wantErr: true},
		{name: "scale Inf", opts: Options{ImagePath: "logo.png", Scale: float64Ptr(math.Inf(-1))}, wantErr: true},
		{name: "invalid color", opts: Options{Text: "mark", Color: "notacolor"}, wantErr: true},
		{name: "valid color", opts: Options{Text: "mark", Color: "#FF8800CC"}},
		{name: "text exceeds rune limit", opts: Options{Text: strings.Repeat("水", MaxTextRunes+1)}, wantErr: true},
		{name: "text at rune limit", opts: Options{Text: strings.Repeat("水", MaxTextRunes)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.opts
			opts.Normalize()
			err := opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
}

func float64Ptr(v float64) *float64 {
	return &v
}
