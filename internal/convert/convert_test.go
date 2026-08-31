package convert

import (
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"imagetoolbox/internal/imageio"
)

func TestConvertPNGToJPEGWithBackground(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.png")
	output := filepath.Join(dir, "output.jpg")

	img := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	img.Set(0, 0, color.NRGBA{0, 0, 0, 0})
	img.Set(5, 5, color.NRGBA{255, 0, 0, 255})

	writePNG(t, input, img)

	if err := ConvertFile(input, output, Options{
		To:         "jpg",
		Quality:    80,
		Background: "#00FF00",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out, err := imageio.DetectFormat(output)
	if err != nil {
		t.Fatalf("detect output format: %v", err)
	}
	if out != imageio.FormatJPEG {
		t.Fatalf("got %s, want jpeg", out)
	}
}

func TestConvertPNGToWEBP(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.png")
	output := filepath.Join(dir, "output.webp")

	img := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	img.Set(2, 2, color.NRGBA{255, 0, 0, 255})
	writePNG(t, input, img)

	if err := ConvertFile(input, output, Options{
		To:      "webp",
		Quality: 80,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out, err := imageio.DetectFormat(output)
	if err != nil {
		t.Fatalf("detect output format: %v", err)
	}
	if out != imageio.FormatWEBP {
		t.Fatalf("got %s, want webp", out)
	}
}

func TestDefaultOutputPath(t *testing.T) {
	got, err := DefaultOutputPath("/tmp/a.png", "jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/tmp/a_converted.jpeg" {
		t.Fatalf("got %s", got)
	}
}

func TestOptionsNormalizeAndValidate(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		want    Options
		wantErr bool
	}{
		{name: "defaults and jpg normalization", opts: Options{To: " .JPG "}, want: Options{To: "jpg", Quality: DefaultQuality, Background: DefaultBackground}},
		{name: "invalid negative quality", opts: Options{To: "webp", Quality: -1}, wantErr: true},
		{name: "invalid excessive quality", opts: Options{To: "webp", Quality: 101}, wantErr: true},
		{name: "lossless jpeg", opts: Options{To: "jpeg", Lossless: true}, wantErr: true},
		{name: "lossless webp", opts: Options{To: "webp", Lossless: true}, want: Options{To: "webp", Quality: DefaultQuality, Lossless: true, Background: DefaultBackground}},
		{name: "lossless png", opts: Options{To: "png", Lossless: true}, want: Options{To: "png", Quality: DefaultQuality, Lossless: true, Background: DefaultBackground}},
		{name: "short background", opts: Options{To: "png", Background: "#fff"}, want: Options{To: "png", Quality: DefaultQuality, Background: "#fff"}},
		{name: "invalid background", opts: Options{To: "jpg", Background: "invalid"}, wantErr: true},
		// background 只服务 JPEG 铺底，PNG/WebP 忽略该参数（Phase 2 语义）。
		{name: "invalid background ignored for png", opts: Options{To: "png", Background: "invalid"}, want: Options{To: "png", Quality: DefaultQuality, Background: "invalid"}},
		{name: "invalid background ignored for webp", opts: Options{To: "webp", Background: "invalid"}, want: Options{To: "webp", Quality: DefaultQuality, Background: "invalid"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.opts.Normalize()
			if !tt.wantErr && tt.opts != tt.want {
				t.Fatalf("Normalize() = %+v, want %+v", tt.opts, tt.want)
			}
			if err := tt.opts.Validate(); (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConvertFileUsesDomainDefaults(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.png")
	output := filepath.Join(dir, "output.webp")
	writePNG(t, input, image.NewNRGBA(image.Rect(0, 0, 10, 10)))

	if err := ConvertFile(input, output, Options{To: "webp"}); err != nil {
		t.Fatalf("ConvertFile() error = %v", err)
	}
}

// TestConvertRejectsGIFInput 锁定输入格式边界：convert 只接受
// JPEG/PNG/WebP，GIF 等可解码但产品不支持的格式必须在解码前被拒绝，
// 防止 animated GIF 被静默转成首帧。
func TestConvertRejectsGIFInput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.gif")
	output := filepath.Join(dir, "output.webp")

	gifImg := image.NewPaletted(image.Rect(0, 0, 4, 4), color.Palette{color.Gray{0}, color.Gray{255}})
	f, err := os.Create(input)
	if err != nil {
		t.Fatalf("create gif: %v", err)
	}
	defer f.Close()
	if err := gif.Encode(f, gifImg, nil); err != nil {
		t.Fatalf("encode gif: %v", err)
	}

	err = ConvertFile(input, output, Options{To: "webp", Quality: 80})
	if !errors.Is(err, imageio.ErrUnsupportedFormat) {
		t.Fatalf("ConvertFile(gif) error = %v, want imageio.ErrUnsupportedFormat", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("output should not be created, stat err = %v", err)
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
