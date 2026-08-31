package convert

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"imagetoolbox/internal/imageio"
)

// TestConvertPNGToJPEGBackground 验证 PNG 透明区域按 --background 铺底。
func TestConvertPNGToJPEGBackground(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.png")
	output := filepath.Join(dir, "output.jpg")

	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			if x < 4 {
				img.SetNRGBA(x, y, color.NRGBA{R: 255, A: 0}) // 左半透明
			} else {
				img.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255}) // 右半不透明
			}
		}
	}
	writePNG(t, input, img)

	if err := ConvertFile(input, output, Options{
		To:         "jpg",
		Quality:    95,
		Background: "#00FF00",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out, err := imageio.DetectFormat(output); err != nil || out != imageio.FormatJPEG {
		t.Fatalf("detect output format = %s, %v; want jpeg", out, err)
	}
	decoded, err := readImage(t, output)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	assertColorNear(t, decoded.At(1, 4), color.NRGBA{G: 255, A: 255}, "transparent half")
	assertColorNear(t, decoded.At(6, 4), color.NRGBA{R: 255, A: 255}, "opaque half")
}

// TestConvertJPEGToWEBP 验证 JPEG 输入的基本转换路径与输出格式。
func TestConvertJPEGToWEBP(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.jpg")
	output := filepath.Join(dir, "output.webp")

	writeJPEG(t, input, image.NewNRGBA(image.Rect(0, 0, 8, 8)))

	if err := ConvertFile(input, output, Options{To: "webp", Quality: 90}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out, err := imageio.DetectFormat(output); err != nil || out != imageio.FormatWEBP {
		t.Fatalf("detect output format = %s, %v; want webp", out, err)
	}
	decoded, err := readImage(t, output)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if got := decoded.Bounds(); got != image.Rect(0, 0, 8, 8) {
		t.Fatalf("output bounds = %v, want 8x8", got)
	}
}

// TestConvertPNGToWEBPLossyPreservesAlpha 验证默认（有损）WebP 转换
// 保留 Alpha：透明 PNG 不会被静默铺底。
func TestConvertPNGToWEBPLossyPreservesAlpha(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.png")
	output := filepath.Join(dir, "output.webp")

	img := image.NewNRGBA(image.Rect(0, 0, 4, 1))
	for x, a := range []uint8{0, 64, 128, 255} {
		img.SetNRGBA(x, 0, color.NRGBA{R: 255, A: a})
	}
	writePNG(t, input, img)

	if err := ConvertFile(input, output, Options{To: "webp", Quality: 90}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded, err := readImage(t, output)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	for x, want := range []uint8{0, 64, 128, 255} {
		_, _, _, a := decoded.At(x, 0).RGBA()
		if got := uint8(a >> 8); got != want {
			t.Errorf("alpha at (%d,0) = %d, want %d", x, got, want)
		}
	}
}

// TestConvertPNGToWEBPLossless 验证 --lossless WebP 转换逐像素无损。
func TestConvertPNGToWEBPLossless(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.png")
	output := filepath.Join(dir, "output.webp")

	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(x * 60),
				G: uint8(y * 60),
				B: uint8((x + y) * 30),
				A: uint8((x*3 + y*5) * 15 % 256),
			})
		}
	}
	writePNG(t, input, img)

	if err := ConvertFile(input, output, Options{To: "webp", Lossless: true, Quality: 80}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded, err := readImage(t, output)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	nrgba, ok := decoded.(*image.NRGBA)
	if !ok {
		t.Fatalf("decoded image type = %T, want *image.NRGBA", decoded)
	}
	for y := range 4 {
		for x := range 4 {
			if got, want := nrgba.NRGBAAt(x, y), img.NRGBAAt(x, y); got != want {
				t.Errorf("pixel (%d,%d) = %+v, want %+v", x, y, got, want)
			}
		}
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
		{name: "short background", opts: Options{To: "png", Background: "#fff"}, want: Options{To: "png", Quality: DefaultQuality, Background: "#fff"}},
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

// Validate 契约测试：参数语义按目标格式收口。

func validate(t *testing.T, opts Options) error {
	t.Helper()
	opts.Normalize()
	return opts.Validate()
}

func TestValidateInvalidJPEGBackground(t *testing.T) {
	if err := validate(t, Options{To: "jpg", Background: "invalid"}); err == nil {
		t.Fatal("Validate() = nil, want error for invalid background with jpeg target")
	}
}

func TestValidateIgnoresBackgroundForPNG(t *testing.T) {
	if err := validate(t, Options{To: "png", Background: "invalid"}); err != nil {
		t.Fatalf("Validate() = %v, want nil (background is ignored for png)", err)
	}
}

func TestValidateIgnoresBackgroundForWEBP(t *testing.T) {
	if err := validate(t, Options{To: "webp", Background: "invalid"}); err != nil {
		t.Fatalf("Validate() = %v, want nil (background is ignored for webp)", err)
	}
}

func TestValidateRejectsLosslessJPEG(t *testing.T) {
	if err := validate(t, Options{To: "jpg", Lossless: true}); err == nil {
		t.Fatal("Validate() = nil, want error for lossless jpeg")
	}
}

func TestValidateAllowsLosslessPNG(t *testing.T) {
	if err := validate(t, Options{To: "png", Lossless: true}); err != nil {
		t.Fatalf("Validate() = %v, want nil (accepted no-op for png)", err)
	}
}

func TestValidateAllowsLosslessWEBP(t *testing.T) {
	if err := validate(t, Options{To: "webp", Lossless: true}); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
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

// TestConvertJPEGAutoOrientation 锁定 EXIF Orientation 契约：转换时把
// Orientation 应用到实际像素，输出不再依赖 Orientation metadata。
// 存储 120×80 + Orientation=6（显示需顺时针旋转 90°）→ 输出 80×120。
func TestConvertJPEGAutoOrientation(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.jpg")
	output := filepath.Join(dir, "output.png")

	// 存储图像：左边缘红、右边缘蓝、中间白；旋转后红/蓝分别落到
	// 输出的顶部/底部，用于验证方向而不是只看尺寸。
	img := image.NewNRGBA(image.Rect(0, 0, 120, 80))
	for y := range 80 {
		for x := range 120 {
			switch {
			case x < 8:
				img.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
			case x >= 112:
				img.SetNRGBA(x, y, color.NRGBA{B: 255, A: 255})
			default:
				img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}
	}
	writeExifJPEG(t, input, img, 6)

	if err := ConvertFile(input, output, Options{To: "png"}); err != nil {
		t.Fatalf("ConvertFile() error = %v", err)
	}

	decoded, err := readImage(t, output)
	if err != nil {
		t.Fatalf("read output png: %v", err)
	}
	if got := decoded.Bounds(); got != image.Rect(0, 0, 80, 120) {
		t.Fatalf("output bounds = %v, want 80x120", got)
	}
	// Orientation=6 = 顺时针旋转 90°：存储左边缘（红）→ 输出顶部，
	// 存储右边缘（蓝）→ 输出底部。
	assertColorNear(t, decoded.At(40, 4), color.NRGBA{R: 255, A: 255}, "top band")
	assertColorNear(t, decoded.At(40, 116), color.NRGBA{B: 255, A: 255}, "bottom band")
	assertColorNear(t, decoded.At(40, 60), color.NRGBA{R: 255, G: 255, B: 255, A: 255}, "center")
}

// writeExifJPEG 就地合成带 EXIF Orientation 的 JPEG：在 SOI 后注入
// APP1/Exif 段（TIFF little-endian，IFD0 仅含 Orientation tag），
// 避免在仓库中保存二进制 fixture。
func writeExifJPEG(t *testing.T, path string, img image.Image, orientation uint16) {
	t.Helper()
	var body bytes.Buffer
	if err := jpeg.Encode(&body, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}

	exif := []byte{'E', 'x', 'i', 'f', 0, 0}
	exif = append(exif,
		'I', 'I', 0x2A, 0x00, // TIFF header, little-endian
		0x08, 0x00, 0x00, 0x00, // IFD0 offset = 8
		0x01, 0x00, // 1 entry
		0x12, 0x01, // tag 0x0112 Orientation
		0x03, 0x00, // type SHORT
		0x01, 0x00, 0x00, 0x00, // count 1
		byte(orientation), byte(orientation>>8), 0x00, 0x00, // value
		0x00, 0x00, 0x00, 0x00, // next IFD = 0
	)

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create jpeg: %v", err)
	}
	defer f.Close()
	segment := []byte{0xFF, 0xE1, byte((len(exif)+2)>>8), byte(len(exif) + 2)}
	if _, err := f.Write(body.Bytes()[:2]); err != nil { // SOI
		t.Fatalf("write SOI: %v", err)
	}
	if _, err := f.Write(segment); err != nil {
		t.Fatalf("write APP1: %v", err)
	}
	if _, err := f.Write(exif); err != nil {
		t.Fatalf("write exif: %v", err)
	}
	if _, err := f.Write(body.Bytes()[2:]); err != nil { // SOI 之后的内容
		t.Fatalf("write jpeg body: %v", err)
	}
}

func readPNG(t *testing.T, path string) (image.Image, error) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return png.Decode(bytes.NewReader(data))
}

// readImage 按已注册 decoder 读取任意受支持输出（PNG/WebP/JPEG）。
func readImage(t *testing.T, path string) (image.Image, error) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

func writeJPEG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create jpeg: %v", err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
}

func assertColorNear(t *testing.T, got color.Color, want color.NRGBA, where string) {
	t.Helper()
	r, g, b, _ := got.RGBA()
	near := func(v uint32, want uint8) bool {
		d := int(v>>8) - int(want)
		return d >= -40 && d <= 40
	}
	if !near(r, want.R) || !near(g, want.G) || !near(b, want.B) {
		t.Errorf("%s color = %v, want near %+v", where, got, want)
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
