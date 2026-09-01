package compare

import (
	"context"
	"image"
	"image/color"
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

// testPlanes 是测试专用的全通道物化。生产 CompareImages 逐通道流式
// 处理，峰值只保留一对通道平面；测试物化全部活动通道用于逐通道
// 交叉验证与 kernel benchmark。
type testPlanes struct {
	width, height, channels int
	src, dst                [][]float32
}

// materializePlanes 物化一对图片的全部活动通道（testing.TB 同时服务
// 测试与 benchmark）。
func materializePlanes(t testing.TB, src, dst image.Image) *testPlanes {
	t.Helper()

	sb, db := src.Bounds(), dst.Bounds()
	if sb.Dx() != db.Dx() || sb.Dy() != db.Dy() {
		t.Fatalf("图片尺寸不一致: %dx%d vs %dx%d", sb.Dx(), sb.Dy(), db.Dx(), db.Dy())
	}
	premultiply := hasTransparency(src) || hasTransparency(dst)
	channels := opaqueChannelCount
	if premultiply {
		channels = alphaChannelCount
	}

	p := &testPlanes{
		width:    sb.Dx(),
		height:   sb.Dy(),
		channels: channels,
		src:      make([][]float32, channels),
		dst:      make([][]float32, channels),
	}
	for c := 0; c < channels; c++ {
		p.src[c] = make([]float32, p.width*p.height)
		p.dst[c] = make([]float32, p.width*p.height)
		if err := extractChannel(context.Background(), src, p.src[c], c, premultiply); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := extractChannel(context.Background(), dst, p.dst[c], c, premultiply); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	return p
}

// mustSSIMPair 构造 src/dst 的全通道测试平面。
func mustSSIMPair(t *testing.T, src, dst *image.NRGBA) *testPlanes {
	t.Helper()
	return materializePlanes(t, src, dst)
}

// channelPlane 提取图片单个活动通道（1×w×h），供逐通道断言。
func channelPlane(t *testing.T, img image.Image, c int, premultiply bool) []float32 {
	t.Helper()
	b := img.Bounds()
	plane := make([]float32, b.Dx()*b.Dy())
	if err := extractChannel(context.Background(), img, plane, c, premultiply); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return plane
}

func TestActiveChannelCount(t *testing.T) {
	opaqueImg := solidNRGBA(2, 2, opaque())
	withAlpha := solidNRGBA(2, 2, color.NRGBA{R: 1, G: 2, B: 3, A: 128})

	tests := []struct {
		name       string
		src, dst   image.Image
		wantCh     int
		wantPremul bool
	}{
		{"都不透明", opaqueImg, opaqueImg, opaqueChannelCount, false},
		{"src 有透明", withAlpha, opaqueImg, alphaChannelCount, true},
		{"dst 有透明", opaqueImg, withAlpha, alphaChannelCount, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := activeChannelCount(tt.src, tt.dst); got != tt.wantCh {
				t.Fatalf("activeChannelCount = %d, want %d", got, tt.wantCh)
			}
		})
	}
}

// 不透明图片只提取 R/G/B 三个通道，样本是原始 0..255 值。
func TestExtractChannelOpaqueUsesRGB(t *testing.T) {
	c := color.NRGBA{R: 10, G: 20, B: 30, A: 255}
	img := solidNRGBA(4, 3, c)

	for ch, want := range []float32{10, 20, 30} {
		plane := channelPlane(t, img, ch, false)
		if len(plane) != 12 {
			t.Fatalf("channel %d len = %d, want 12", ch, len(plane))
		}
		if got := plane[0]; got != want {
			t.Fatalf("channel %d plane[0] = %v, want %v", ch, got, want)
		}
	}
}

// alpha 模式下 R/G/B 写入预乘值，A 本身是第四通道。
func TestExtractChannelAlphaModePremultiplies(t *testing.T) {
	img := solidNRGBA(2, 1, color.NRGBA{R: 255, G: 0, B: 0, A: 128})

	// alpha=128 时 premultiplied R = 255*128/255 = 128（与 color.NRGBA.RGBA() 一致）
	for ch, want := range []float32{128, 0, 0, 128} {
		plane := channelPlane(t, img, ch, true)
		if got := plane[0]; got != want {
			t.Fatalf("channel %d plane[0] = %v, want %v", ch, got, want)
		}
	}
}

// 完全透明区域隐藏的 RGB 不参与比较：RGBA(255,0,0,0) 与 RGBA(0,255,0,0)
// premultiplied 后都是 (0,0,0,0)。
func TestExtractChannelTransparentHiddenRGBIgnored(t *testing.T) {
	src := solidNRGBA(2, 2, color.NRGBA{R: 255, G: 0, B: 0, A: 0})
	dst := solidNRGBA(2, 2, color.NRGBA{R: 0, G: 255, B: 0, A: 0})

	for c := 0; c < alphaChannelCount; c++ {
		s := channelPlane(t, src, c, true)
		d := channelPlane(t, dst, c, true)
		for i := range s {
			if s[i] != d[i] {
				t.Fatalf("channel %d plane[%d] differs (%v vs %v) for fully transparent pixels",
					c, i, s[i], d[i])
			}
		}
	}
}

// alpha 本身是比较通道：RGB 相同而 alpha 不同必须被检测出来。
func TestExtractChannelAlphaDifferenceDetected(t *testing.T) {
	src := solidNRGBA(2, 2, color.NRGBA{R: 0, G: 0, B: 0, A: 255})
	dst := solidNRGBA(2, 2, color.NRGBA{R: 0, G: 0, B: 0, A: 128})

	s := channelPlane(t, src, 3, true)
	d := channelPlane(t, dst, 3, true)
	if s[0] == d[0] {
		t.Fatal("alpha channel should expose the difference")
	}
}

// 不透明像素上 NRGBA 快速路径、RGBA 存储、通用 At().RGBA() 路径必须一致。
func TestExtractChannelOpaqueConsistentAcrossTypes(t *testing.T) {
	nrgba := solidNRGBA(3, 3, color.NRGBA{R: 200, G: 100, B: 50, A: 255})

	rgba := image.NewRGBA(image.Rect(0, 0, 3, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			rgba.SetRGBA(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}

	for c := 0; c < opaqueChannelCount; c++ {
		pn := channelPlane(t, nrgba, c, false)
		pr := channelPlane(t, rgba, c, false)
		for i := range pn {
			if pn[i] != pr[i] {
				t.Fatalf("NRGBA vs RGBA channel %d plane[%d] = %v vs %v", c, i, pn[i], pr[i])
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
	for c := 0; c < opaqueChannelCount; c++ {
		if got := channelPlane(t, gray, c, false)[0]; got != 77 {
			t.Fatalf("gray channel %d plane[0] = %v, want 77", c, got)
		}
	}
}

// YCbCr/Gray/Gray16/CMYK 天然不透明：hasTransparency 不做整图遍历
// 也必须返回 false。
func TestHasTransparencyOpaqueTypesFastPath(t *testing.T) {
	gray := image.NewGray(image.Rect(4, 4, 20, 20))
	for y := gray.Bounds().Min.Y; y < gray.Bounds().Max.Y; y++ {
		for x := gray.Bounds().Min.X; x < gray.Bounds().Max.X; x++ {
			gray.SetGray(x, y, color.Gray{Y: uint8(x)})
		}
	}
	gray16 := image.NewGray16(image.Rect(0, 0, 4, 4))
	ycbcr := image.NewYCbCr(image.Rect(0, 0, 4, 4), image.YCbCrSubsampleRatio444)
	cmyk := image.NewCMYK(image.Rect(0, 0, 4, 4))

	for name, img := range map[string]image.Image{
		"YCbCr":  ycbcr,
		"Gray":   gray,
		"Gray16": gray16,
		"CMYK":   cmyk,
	} {
		if hasTransparency(img) {
			t.Fatalf("%s 应天然不透明", name)
		}
	}

	// NRGBA 存在 alpha != 255 时仍必须检测到
	if !hasTransparency(solidNRGBA(2, 2, color.NRGBA{A: 254})) {
		t.Fatal("NRGBA alpha=254 应被检测为透明")
	}
	if hasTransparency(solidNRGBA(2, 2, color.NRGBA{A: 255})) {
		t.Fatal("NRGBA 全不透明不应报透明")
	}
}

func TestExtractChannelCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	plane := make([]float32, 4)
	if err := extractChannel(ctx, solidNRGBA(2, 2, opaque()), plane, 0, false); err == nil {
		t.Fatal("expected context error, got nil")
	}
}
