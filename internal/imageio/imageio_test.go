package imageio

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestNormalizeFormat(t *testing.T) {
	tests := []struct {
		value string
		want  Format
	}{
		{value: "png", want: FormatPNG},
		{value: ".PNG", want: FormatPNG},
		{value: "jpg", want: FormatJPEG},
		{value: " jpeg ", want: FormatJPEG},
		{value: "webp", want: FormatWEBP},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := NormalizeFormat(tt.value)
			if err != nil {
				t.Fatalf("NormalizeFormat(%q) error = %v", tt.value, err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeFormat(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
	t.Run("unsupported format is a typed error", func(t *testing.T) {
		_, err := NormalizeFormat("gif")
		if !errors.Is(err, ErrUnsupportedFormat) {
			t.Fatalf("NormalizeFormat(gif) error = %v, want ErrUnsupportedFormat", err)
		}
	})
}

// TestEncodeWEBPLossyPreservesAlpha 锁定 codec 契约：WebP 无论有损/无损
// 都保留 Alpha，调用方无法再通过 SaveOptions 让 lossy WebP 丢失透明度。
func TestEncodeWEBPLossyPreservesAlpha(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 1))
	alphas := []uint8{0, 64, 128, 255}
	for x, a := range alphas {
		img.SetNRGBA(x, 0, color.NRGBA{R: 255, G: 0, B: 0, A: a})
	}

	var buf bytes.Buffer
	if err := Encode(&buf, img, FormatWEBP, SaveOptions{Quality: 90}); err != nil {
		t.Fatalf("Encode(webp lossy) error = %v", err)
	}

	decoded, _, err := image.Decode(&buf)
	if err != nil {
		t.Fatalf("decode webp: %v", err)
	}
	for x, want := range alphas {
		_, _, _, a := decoded.At(x, 0).RGBA()
		got := uint8(a >> 8)
		if got != want {
			t.Errorf("alpha at (%d,0) = %d, want %d", x, got, want)
		}
	}
}

// TestEncodeWEBPLosslessRoundTrip 验证 lossless WebP 逐像素无损往返；
// Exact=true 保证透明像素下的 RGB 也完整保留。
func TestEncodeWEBPLosslessRoundTrip(t *testing.T) {
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
	// 透明像素保留非零 RGB，验证 Exact 语义。
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 0, B: 0, A: 0})

	var buf bytes.Buffer
	if err := Encode(&buf, img, FormatWEBP, SaveOptions{Lossless: true, Quality: 80}); err != nil {
		t.Fatalf("Encode(webp lossless) error = %v", err)
	}

	decoded, _, err := image.Decode(&buf)
	if err != nil {
		t.Fatalf("decode webp: %v", err)
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

// TestEncodeJPEGFlattensAlpha 锁定 JPEG（不支持 Alpha）在 Encode 内部
// 固定铺底的行为，调用方无需也不能干预。
func TestEncodeJPEGFlattensAlpha(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			if x < 4 {
				img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 0, B: 0, A: 0}) // 左半透明
			} else {
				img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 0, B: 0, A: 255}) // 右半不透明
			}
		}
	}

	var buf bytes.Buffer
	background := color.NRGBA{R: 0, G: 255, B: 0, A: 255}
	if err := Encode(&buf, img, FormatJPEG, SaveOptions{Quality: 95, Background: background}); err != nil {
		t.Fatalf("Encode(jpeg) error = %v", err)
	}

	decoded, err := jpeg.Decode(&buf)
	if err != nil {
		t.Fatalf("decode jpeg: %v", err)
	}
	// 远离左右分界处的色度抽样边界，各取内部像素验证。
	tolerance := 24
	for x, want := range map[int]color.RGBA{
		1: {R: 0, G: 255, B: 0, A: 255}, // 透明区域 → 背景色
		6: {R: 255, G: 0, B: 0, A: 255}, // 不透明区域 → 原色
	} {
		r, g, b, _ := decoded.At(x, 4).RGBA()
		got := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255}
		dr := int(got.R) - int(want.R)
		dg := int(got.G) - int(want.G)
		db := int(got.B) - int(want.B)
		if dr < -tolerance || dr > tolerance || dg < -tolerance || dg > tolerance || db < -tolerance || db > tolerance {
			t.Errorf("pixel (%d,4) = %+v, want near %+v", x, got, want)
		}
	}
}
