package compare

import (
	"context"
	"image"
	"image/color"
	"math"
	"testing"
)

// comparePSNR 走领域入口 CompareImages 验证 PSNR：全局 MSE 覆盖全部
// 活动通道样本。
func comparePSNR(t *testing.T, src, dst image.Image) float64 {
	t.Helper()
	res, err := CompareImages(context.Background(), src, dst, Options{Metrics: MetricPSNR})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return res.PSNR
}

func TestPSNRIdenticalIsInfinite(t *testing.T) {
	src := fillNRGBA(8, 8, func(x, y int) color.NRGBA {
		return color.NRGBA{R: uint8(x * 30), G: uint8(y * 25), B: 128, A: 255}
	})
	if got := comparePSNR(t, src, src); !math.IsInf(got, 1) {
		t.Fatalf("psnr = %v, want +Inf for identical images", got)
	}
}

// 常量图：全部样本差 10 → MSE = 100 → PSNR = 10·log10(65025/100)。
func TestPSNRConstantImages(t *testing.T) {
	got := comparePSNR(t,
		solidNRGBA(4, 4, color.NRGBA{A: 255}),
		solidNRGBA(4, 4, color.NRGBA{R: 10, G: 10, B: 10, A: 255}))
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

	got := comparePSNR(t, src, dst)
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

	if a, b := comparePSNR(t, src, dst), comparePSNR(t, dst, src); a != b {
		t.Fatalf("psnr(A,B) = %v, psnr(B,A) = %v, want equal", a, b)
	}
}

// 完全透明区域隐藏的 RGB 不参与比较：两图 alpha 全 0、RGB 不同 → +Inf。
func TestPSNRTransparentHiddenRGBIgnored(t *testing.T) {
	got := comparePSNR(t,
		solidNRGBA(4, 4, color.NRGBA{R: 255, A: 0}),
		solidNRGBA(4, 4, color.NRGBA{G: 255, A: 0}))
	if !math.IsInf(got, 1) {
		t.Fatalf("psnr = %v, want +Inf when hidden RGB differs behind full transparency", got)
	}
}

// alpha 丢失必须被检测：RGB 全 0，A 从 255 变 128，
// 只有 alpha 通道差 127 → MSE = 127²/4 → PSNR = 10·log10(65025·4/127²)。
func TestPSNRAlphaDifferenceDetected(t *testing.T) {
	got := comparePSNR(t,
		solidNRGBA(2, 2, color.NRGBA{A: 255}),
		solidNRGBA(2, 2, color.NRGBA{A: 128}))
	if want := 12.0753; math.Abs(got-want) > 1e-3 {
		t.Fatalf("psnr = %v, want %v", got, want)
	}
}

func TestPSNRCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := CompareImages(ctx, solidNRGBA(4, 4, opaque()), solidNRGBA(4, 4, opaque()), Options{Metrics: MetricPSNR})
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
}

// psnrChannel 全程零分配；psnrFromSumSq 的 +Inf 语义由上面的领域入口
// 测试覆盖。
func TestPSNRChannelZeroAlloc(t *testing.T) {
	p := mustSSIMPair(t, gradientImage(64, 64), distortImage(gradientImage(64, 64)))
	if n := testing.AllocsPerRun(10, func() {
		_, _ = psnrChannel(context.Background(), p.src[0], p.dst[0], p.width, p.height)
	}); n != 0 {
		t.Fatalf("psnrChannel allocs = %v per run, want 0", n)
	}
}
