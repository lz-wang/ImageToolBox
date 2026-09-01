package compare

import (
	"context"
	"image/color"
	"math"
	"testing"
)

func TestPSNRIdenticalIsInfinite(t *testing.T) {
	src := fillNRGBA(8, 8, func(x, y int) color.NRGBA {
		return color.NRGBA{R: uint8(x * 30), G: uint8(y * 25), B: 128, A: 255}
	})
	p, err := newPixelPlanes(context.Background(), src, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := psnr(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !math.IsInf(got, 1) {
		t.Fatalf("psnr = %v, want +Inf for identical images", got)
	}
}

// 常量图：全部样本差 10 → MSE = 100 → PSNR = 10·log10(65025/100)。
func TestPSNRConstantImages(t *testing.T) {
	p, err := newPixelPlanes(context.Background(),
		solidNRGBA(4, 4, color.NRGBA{A: 255}),
		solidNRGBA(4, 4, color.NRGBA{R: 10, G: 10, B: 10, A: 255}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := psnr(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := 28.1308; math.Abs(got-want) > 1e-3 {
		t.Fatalf("psnr = %v, want %v", got, want)
	}
}

// 已知单通道误差：2×2 图只有一个 R 样本差 10，
// MSE = 100/(2·2·3) → PSNR = 10·log10(65025·12/100)。
func TestPSNRKnownOneChannelError(t *testing.T) {
	src := fillNRGBA(2, 2, func(int, int) color.NRGBA {
		return color.NRGBA{R: 100, G: 50, B: 25, A: 255}
	})
	dst := fillNRGBA(2, 2, func(x, y int) color.NRGBA {
		if x == 0 && y == 0 {
			return color.NRGBA{R: 110, G: 50, B: 25, A: 255}
		}
		return color.NRGBA{R: 100, G: 50, B: 25, A: 255}
	})

	p, err := newPixelPlanes(context.Background(), src, dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := psnr(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := 38.9226; math.Abs(got-want) > 1e-3 {
		t.Fatalf("psnr = %v, want %v", got, want)
	}
}

func TestPSNRSymmetry(t *testing.T) {
	src := fillNRGBA(16, 16, func(x, y int) color.NRGBA {
		return color.NRGBA{R: uint8(x * 16), G: uint8(y * 16), B: uint8(x * y), A: 255}
	})
	dst := fillNRGBA(16, 16, func(x, y int) color.NRGBA {
		return color.NRGBA{R: uint8(x*16 + 7), G: uint8(y * 16), B: uint8(x * y), A: 255}
	})

	pab, err := newPixelPlanes(context.Background(), src, dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pba, err := newPixelPlanes(context.Background(), dst, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	a, err := psnr(context.Background(), pab)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := psnr(context.Background(), pba)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a != b {
		t.Fatalf("psnr(A,B) = %v, psnr(B,A) = %v, want equal", a, b)
	}
}

// 完全透明区域隐藏的 RGB 不参与比较：两图 alpha 全 0、RGB 不同 → +Inf。
func TestPSNRTransparentHiddenRGBIgnored(t *testing.T) {
	p, err := newPixelPlanes(context.Background(),
		solidNRGBA(4, 4, color.NRGBA{R: 255, A: 0}),
		solidNRGBA(4, 4, color.NRGBA{G: 255, A: 0}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := psnr(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !math.IsInf(got, 1) {
		t.Fatalf("psnr = %v, want +Inf when hidden RGB differs behind full transparency", got)
	}
}

// alpha 丢失必须被检测：RGB 全 0，A 从 255 变 128，
// 只有 alpha 通道差 127 → MSE = 127²/4 → PSNR = 10·log10(65025·4/127²)。
func TestPSNRAlphaDifferenceDetected(t *testing.T) {
	p, err := newPixelPlanes(context.Background(),
		solidNRGBA(2, 2, color.NRGBA{A: 255}),
		solidNRGBA(2, 2, color.NRGBA{A: 128}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := psnr(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := 12.0753; math.Abs(got-want) > 1e-3 {
		t.Fatalf("psnr = %v, want %v", got, want)
	}
}

func TestPSNRCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p, err := newPixelPlanes(context.Background(), solidNRGBA(4, 4, opaque()), solidNRGBA(4, 4, opaque()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := psnr(ctx, p); err == nil {
		t.Fatal("expected context error, got nil")
	}
}
