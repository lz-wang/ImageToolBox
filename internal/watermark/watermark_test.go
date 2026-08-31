package watermark

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
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
