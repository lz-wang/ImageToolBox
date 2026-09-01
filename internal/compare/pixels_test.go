package compare

import (
	"context"
	"image"
	"image/color"
	"strings"
	"testing"
)

// fillNRGBA 生成一张逐像素由 f 决定的 NRGBA 图片。
func fillNRGBA(w, h int, f func(x, y int) color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, f(x, y))
		}
	}
	return img
}

// solidNRGBA 生成纯色 NRGBA 图片。
func solidNRGBA(w, h int, c color.NRGBA) *image.NRGBA {
	return fillNRGBA(w, h, func(int, int) color.NRGBA { return c })
}

func opaque() color.NRGBA { return color.NRGBA{R: 1, G: 2, B: 3, A: 255} }

func TestNewPixelPlanesDimensionMismatch(t *testing.T) {
	tests := []struct {
		name string
		src  image.Image
		dst  image.Image
	}{
		{"宽度不一致", solidNRGBA(3, 2, opaque()), solidNRGBA(2, 2, opaque())},
		{"高度不一致", solidNRGBA(2, 2, opaque()), solidNRGBA(2, 3, opaque())},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newPixelPlanes(context.Background(), tt.src, tt.dst)
			if err == nil || !strings.Contains(err.Error(), "图片尺寸不一致") {
				t.Fatalf("error = %v, want dimension mismatch", err)
			}
		})
	}
}

// 错误信息必须携带两侧实际尺寸，便于定位。
func TestNewPixelPlanesDimensionMismatchIncludesSizes(t *testing.T) {
	_, err := newPixelPlanes(context.Background(), solidNRGBA(3, 2, opaque()), solidNRGBA(2, 2, opaque()))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "src=3x2, dst=2x2") {
		t.Fatalf("error should include both sizes, got: %v", err)
	}
}

func TestNewPixelPlanesOpaqueUsesRGB(t *testing.T) {
	c := color.NRGBA{R: 10, G: 20, B: 30, A: 255}
	p, err := newPixelPlanes(context.Background(), solidNRGBA(4, 3, c), solidNRGBA(4, 3, c))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.channels != opaqueChannelCount {
		t.Fatalf("channels = %d, want %d for opaque images", p.channels, opaqueChannelCount)
	}
	if p.width != 4 || p.height != 3 {
		t.Fatalf("dimensions = %dx%d, want 4x3", p.width, p.height)
	}
	for c, want := range []float32{10, 20, 30} {
		if got := p.src[c][0]; got != want {
			t.Fatalf("src plane[%d][0] = %v, want %v", c, got, want)
		}
	}
}

func TestNewPixelPlanesAlphaModePremultiplies(t *testing.T) {
	src := solidNRGBA(2, 1, color.NRGBA{R: 255, G: 0, B: 0, A: 128})
	dst := solidNRGBA(2, 1, color.NRGBA{R: 10, G: 20, B: 30, A: 255})

	p, err := newPixelPlanes(context.Background(), src, dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.channels != alphaChannelCount {
		t.Fatalf("channels = %d, want %d when transparency exists", p.channels, alphaChannelCount)
	}

	// alpha=128 时 premultiplied R = 255*128/255 = 128（与 color.NRGBA.RGBA() 一致）
	wantSrc := [alphaChannelCount]float32{128, 0, 0, 128}
	for c, want := range wantSrc {
		if got := p.src[c][0]; got != want {
			t.Fatalf("src plane[%d][0] = %v, want %v", c, got, want)
		}
	}
	// 不透明像素的 premultiplied 值等于原值
	wantDst := [alphaChannelCount]float32{10, 20, 30, 255}
	for c, want := range wantDst {
		if got := p.dst[c][0]; got != want {
			t.Fatalf("dst plane[%d][0] = %v, want %v", c, got, want)
		}
	}
}

// 完全透明区域隐藏的 RGB 不影响比较：RGBA(255,0,0,0) 与 RGBA(0,255,0,0)
// premultiplied 后都是 (0,0,0,0)。
func TestNewPixelPlanesTransparentHiddenRGBIgnored(t *testing.T) {
	src := solidNRGBA(2, 2, color.NRGBA{R: 255, G: 0, B: 0, A: 0})
	dst := solidNRGBA(2, 2, color.NRGBA{R: 0, G: 255, B: 0, A: 0})

	p, err := newPixelPlanes(context.Background(), src, dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for c := 0; c < p.channels; c++ {
		for i := range p.src[c] {
			if p.src[c][i] != p.dst[c][i] {
				t.Fatalf("plane[%d][%d] differs (%v vs %v) for fully transparent pixels",
					c, i, p.src[c][i], p.dst[c][i])
			}
		}
	}
}

// alpha 本身是比较通道：RGB 相同而 alpha 不同必须被检测出来。
func TestNewPixelPlanesAlphaDifferenceDetected(t *testing.T) {
	src := solidNRGBA(2, 2, color.NRGBA{R: 0, G: 0, B: 0, A: 255})
	dst := solidNRGBA(2, 2, color.NRGBA{R: 0, G: 0, B: 0, A: 128})

	p, err := newPixelPlanes(context.Background(), src, dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.channels != alphaChannelCount {
		t.Fatalf("channels = %d, want %d", p.channels, alphaChannelCount)
	}
	if p.src[3][0] == p.dst[3][0] {
		t.Fatal("alpha channel should expose the difference")
	}
}

// 不透明像素上 NRGBA 快速路径、RGBA 存储、通用 At().RGBA() 路径必须一致。
func TestNewPixelPlanesOpaqueConsistentAcrossTypes(t *testing.T) {
	nrgba := solidNRGBA(3, 3, color.NRGBA{R: 200, G: 100, B: 50, A: 255})

	rgba := image.NewRGBA(image.Rect(0, 0, 3, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			rgba.SetRGBA(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}

	pn, err := newPixelPlanes(context.Background(), nrgba, rgba)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for c := 0; c < opaqueChannelCount; c++ {
		for i := range pn.src[c] {
			if pn.src[c][i] != pn.dst[c][i] {
				t.Fatalf("NRGBA vs RGBA plane[%d][%d] = %v vs %v", c, i, pn.src[c][i], pn.dst[c][i])
			}
		}
	}

	// Gray 走通用路径，三个通道应全部等于灰度值
	gray := image.NewGray(image.Rect(0, 0, 3, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			gray.SetGray(x, y, color.Gray{Y: 77})
		}
	}
	pg, err := newPixelPlanes(context.Background(), gray, gray)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for c := 0; c < opaqueChannelCount; c++ {
		if got := pg.src[c][0]; got != 77 {
			t.Fatalf("gray plane[%d][0] = %v, want 77", c, got)
		}
	}
}

func TestNewPixelPlanesCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := newPixelPlanes(ctx, solidNRGBA(2, 2, opaque()), solidNRGBA(2, 2, opaque()))
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
}
