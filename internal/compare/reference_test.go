package compare

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"testing"
)

// 本文件是 correctness 加固层：固化参考向量（Reference Vectors）与
// 指标性质（Property Tests），不引入新 feature。
//
// 参考常量全部离线计算后写死，运行期只依赖 Go 标准库现场生成的
// 确定性图片，不依赖 Python / fixture。

// caseBImages 构造 Case B：512×512 确定性 RGB 渐变 + 固定幅度偏差。
func caseBImages(t *testing.T) (*image.NRGBA, *image.NRGBA) {
	t.Helper()

	src := fillNRGBA(512, 512, func(x, y int) color.NRGBA {
		return color.NRGBA{
			R: uint8((x * 255) / 511),
			G: uint8((y * 255) / 511),
			B: uint8((x + y) % 256),
			A: 255,
		}
	})
	// 偏差模式：R ±8、G ∓4、B 不变（确定性，无 clamp 边界折叠）
	dst := fillNRGBA(512, 512, func(x, y int) color.NRGBA {
		sign := 1
		if (x+y)%2 == 1 {
			sign = -1
		}
		s := src.NRGBAAt(x, y)
		return color.NRGBA{
			R: clamp8(int(s.R) + sign*8),
			G: clamp8(int(s.G) - sign*4),
			B: s.B,
			A: 255,
		}
	})
	return src, dst
}

// Case A：identical 参考向量——PSNR=+Inf、SSIM=1、MS-SSIM=1。
func TestReferenceVectorIdentical(t *testing.T) {
	src := gradientImage(512, 512)
	res, err := CompareImages(context.Background(), src, src, Options{Metrics: allMetrics})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !math.IsInf(res.PSNR, 1) {
		t.Fatalf("PSNR = %v, want +Inf", res.PSNR)
	}
	if math.Abs(res.SSIM-1) > 1e-12 {
		t.Fatalf("SSIM = %v, want 1", res.SSIM)
	}
	if math.Abs(res.MSSSIM-1) > 1e-12 {
		t.Fatalf("MS-SSIM = %v, want 1", res.MSSSIM)
	}
}

// Case B：512×512 渐变 + 固定幅度偏差的参考数值（容差 1e-5）。
// 常量由 naive 参考实现离线计算固化，运行期不依赖外部工具。
func TestReferenceVectorGradientDistortion(t *testing.T) {
	src, dst := caseBImages(t)
	res, err := CompareImages(context.Background(), src, dst, Options{Metrics: allMetrics})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 常量由 naive 参考实现离线计算固化（容差 1e-5）：
	// PSNR=33.9615395392 SSIM=0.7585984850 MS-SSIM=0.9859134186
	const wantPSNR = 33.961540
	const wantSSIM = 0.758598
	const wantMSSSIM = 0.985913
	if math.Abs(res.PSNR-wantPSNR) > 1e-5 {
		t.Fatalf("PSNR = %v, want reference %v", res.PSNR, wantPSNR)
	}
	if math.Abs(res.SSIM-wantSSIM) > 1e-5 {
		t.Fatalf("SSIM = %v, want reference %v", res.SSIM, wantSSIM)
	}
	if math.Abs(res.MSSSIM-wantMSSSIM) > 1e-5 {
		t.Fatalf("MS-SSIM = %v, want reference %v", res.MSSSIM, wantMSSSIM)
	}
}

// Case B 交叉验证：SSIM/MS-SSIM 与 naive 逐窗口参考实现一致。
func TestReferenceVectorMatchesNaive(t *testing.T) {
	src, dst := caseBImages(t)
	p := mustSSIMPair(t, src, dst)

	gotSSIM, err := ssim(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var naiveSum float64
	for c := 0; c < p.channels; c++ {
		v, _ := naiveSSIMPlane(p.src[c], p.dst[c], p.width, p.height)
		naiveSum += v
	}
	if want := naiveSum / float64(p.channels); math.Abs(gotSSIM-want) > 1e-9 {
		t.Fatalf("SSIM = %v, want naive %v", gotSSIM, want)
	}

	gotMS, err := msSSIM(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var naiveMSSum float64
	for c := 0; c < p.channels; c++ {
		v := naiveMSSSIMChannel(p.src[c], p.dst[c], p.width, p.height)
		naiveMSSum += v
	}
	if want := naiveMSSum / float64(p.channels); math.Abs(gotMS-want) > 1e-9 {
		t.Fatalf("MS-SSIM = %v, want naive %v", gotMS, want)
	}
}

// naiveMSSSIMChannel 是 MS-SSIM 的独立参考实现：复用 naive 逐窗口
// SSIM，按固定权重组合，幂运算前非负钳制。
func naiveMSSSIMChannel(x, y []float32, width, height int) float64 {
	xs, ys := x, y
	w, h := width, height
	var csMeans [msSSIMScales - 1]float64
	ssim5 := 0.0
	for scale := 0; scale < msSSIMScales; scale++ {
		s, cs := naiveSSIMPlane(xs, ys, w, h)
		if scale == msSSIMScales-1 {
			ssim5 = s
			break
		}
		csMeans[scale] = cs
		nw, nh := (w+1)/2, (h+1)/2
		nextX := downsample2x2(xs, w, h, nw, nh)
		nextY := downsample2x2(ys, w, h, nw, nh)
		xs, ys, w, h = nextX, nextY, nw, nh
	}
	result := math.Pow(max(ssim5, 0), msSSIMWeights[msSSIMScales-1])
	for j := range csMeans {
		result *= math.Pow(max(csMeans[j], 0), msSSIMWeights[j])
	}
	return result
}

// Case C：JPEG 压缩伪影——q=70 编码再解码，与原始图比较。
// 这是最贴近 image-tool-box 实际用途的回归。
func TestReferenceVectorJPEGArtifact(t *testing.T) {
	src := gradientImage(320, 256)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 70}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded, _, err := image.Decode(&buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// jpeg.Decode 产生 *image.YCbCr，同时覆盖通用提取路径
	if _, ok := decoded.(*image.YCbCr); !ok {
		t.Fatalf("expected *image.YCbCr from jpeg.Decode, got %T", decoded)
	}

	res, err := CompareImages(context.Background(), src, decoded, Options{Metrics: allMetrics})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if math.IsInf(res.PSNR, 1) || math.IsNaN(res.PSNR) {
		t.Fatalf("PSNR = %v, want finite", res.PSNR)
	}
	if res.SSIM <= 0 || res.SSIM >= 1 {
		t.Fatalf("SSIM = %v, want in (0,1)", res.SSIM)
	}
	if res.MSSSIM <= 0 || res.MSSSIM >= 1 {
		t.Fatalf("MS-SSIM = %v, want in (0,1)", res.MSSSIM)
	}

	// 数值回归：q=70 的 JPEG 伪影指标冻结在窄区间
	if res.PSNR < 25 || res.PSNR > 45 {
		t.Fatalf("PSNR = %v, want in [25,45] dB for q=70 gradient", res.PSNR)
	}
	if res.SSIM < 0.85 || res.SSIM > 0.99 {
		t.Fatalf("SSIM = %v, want in [0.85,0.99] for q=70 gradient", res.SSIM)
	}
	if res.MSSSIM < 0.9 || res.MSSSIM > 0.999 {
		t.Fatalf("MS-SSIM = %v, want in [0.9,0.999] for q=70 gradient", res.MSSSIM)
	}
}

// 性质：metric(A,B) == metric(B,A)，三个指标全部对称。
func TestPropertySymmetry(t *testing.T) {
	src, dst := caseBImages(t)

	ab, err := CompareImages(context.Background(), src, dst, Options{Metrics: allMetrics})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ba, err := CompareImages(context.Background(), dst, src, Options{Metrics: allMetrics})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ab.PSNR != ba.PSNR {
		t.Fatalf("PSNR not symmetric: %v vs %v", ab.PSNR, ba.PSNR)
	}
	if math.Abs(ab.SSIM-ba.SSIM) > 1e-12 {
		t.Fatalf("SSIM not symmetric: %v vs %v", ab.SSIM, ba.SSIM)
	}
	if math.Abs(ab.MSSSIM-ba.MSSSIM) > 1e-12 {
		t.Fatalf("MS-SSIM not symmetric: %v vs %v", ab.MSSSIM, ba.MSSSIM)
	}
}

// 性质：identical 是所有指标的最大值；Inf 只允许出现在 identical 的
// PSNR 上。
func TestPropertyIdenticalIsMaximum(t *testing.T) {
	src, dst := caseBImages(t)

	res, err := CompareImages(context.Background(), src, dst, Options{Metrics: allMetrics})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.IsInf(res.PSNR, 1) {
		t.Fatal("PSNR must be finite for different images")
	}
	if res.SSIM > 1+1e-12 || res.MSSSIM > 1+1e-12 {
		t.Fatalf("distorted metrics exceed 1: SSIM=%v MS-SSIM=%v", res.SSIM, res.MSSSIM)
	}

	self, err := CompareImages(context.Background(), src, src, Options{Metrics: allMetrics})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !math.IsInf(self.PSNR, 1) || self.PSNR <= res.PSNR {
		t.Fatalf("identical PSNR %v must dominate distorted %v", self.PSNR, res.PSNR)
	}
	if self.SSIM <= res.SSIM || self.MSSSIM <= res.MSSSIM {
		t.Fatalf("identical must dominate: SSIM %v vs %v, MS-SSIM %v vs %v",
			self.SSIM, res.SSIM, self.MSSSIM, res.MSSSIM)
	}
}

// 性质：NaN 永不逃逸——反相、棋盘反相关、alpha 混合等极端输入下
// 三个指标都必须是有限数或 +Inf。
func TestPropertyNaNNeverEscapes(t *testing.T) {
	size := 256
	src := gradientImage(size, size)

	inverted := fillNRGBA(size, size, func(x, y int) color.NRGBA {
		s := src.NRGBAAt(x, y)
		return color.NRGBA{R: 255 - s.R, G: 255 - s.G, B: 255 - s.B, A: 255}
	})
	checker := fillNRGBA(size, size, func(x, y int) color.NRGBA {
		v := uint8(((x + y) % 2) * 255)
		return color.NRGBA{R: v, G: v, B: v, A: 255}
	})
	antiChecker := fillNRGBA(size, size, func(x, y int) color.NRGBA {
		v := uint8(255 - ((x+y)%2)*255)
		return color.NRGBA{R: v, G: v, B: v, A: 255}
	})
	withAlpha := fillNRGBA(size, size, func(x, y int) color.NRGBA {
		s := src.NRGBAAt(x, y)
		return color.NRGBA{R: s.R, G: s.G, B: s.B, A: uint8((x * 255) / (size - 1))}
	})

	cases := []struct {
		name string
		a, b image.Image
	}{
		{"反相渐变", src, inverted},
		{"棋盘反相关", checker, antiChecker},
		{"alpha 渐变 vs 不透明", withAlpha, src},
		{"alpha 渐变自身", withAlpha, withAlpha},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			res, err := CompareImages(context.Background(), tt.a, tt.b, Options{Metrics: allMetrics})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for name, v := range map[string]float64{"PSNR": res.PSNR, "SSIM": res.SSIM, "MS-SSIM": res.MSSSIM} {
				if math.IsNaN(v) {
					t.Fatalf("%s is NaN", name)
				}
			}
			if res.MSSSIM < 0 || res.MSSSIM > 1 {
				t.Fatalf("MS-SSIM = %v, want within [0,1]", res.MSSSIM)
			}
		})
	}
}
