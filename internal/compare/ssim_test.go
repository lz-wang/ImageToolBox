package compare

import (
	"context"
	"errors"
	"image"
	"image/color"
	"math"
	"testing"
)

// gradientImage 生成确定性的 8 位 RGB 渐变图（无 fixture，现场合成）。
func gradientImage(w, h int) *image.NRGBA {
	return fillNRGBA(w, h, func(x, y int) color.NRGBA {
		return color.NRGBA{
			R: uint8((x * 255) / max(w-1, 1)),
			G: uint8((y * 255) / max(h-1, 1)),
			B: uint8((x + y) % 256),
			A: 255,
		}
	})
}

// distortImage 施加确定性的小幅偏差（-4..+4）。
func distortImage(img *image.NRGBA) *image.NRGBA {
	b := img.Bounds()
	return fillNRGBA(b.Dx(), b.Dy(), func(x, y int) color.NRGBA {
		c := img.NRGBAAt(x, y)
		delta := int((x*x+y*7)%9) - 4
		return color.NRGBA{
			R: clamp8(int(c.R) + delta),
			G: clamp8(int(c.G) - delta),
			B: c.B,
			A: 255,
		}
	})
}

// clamp8 把数值钳制回 [0,255]。
func clamp8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// naiveSSIMPlane 是按公式直接逐窗口计算的参考实现（O(w·h·121)，
// 不做 separable/streaming 优化），用于交叉验证 streaming 实现的数值。
func naiveSSIMPlane(x, y []float32, width, height int) (meanSSIM, meanCS float64) {
	kernel := gaussianKernel(ssimWindow, ssimSigma)
	var sumSSIM, sumCS float64
	var count int
	for oy := ssimRadius; oy+ssimRadius < height; oy++ {
		for ox := ssimRadius; ox+ssimRadius < width; ox++ {
			var vx, vy, vx2, vy2, vxy float64
			for j := 0; j < ssimWindow; j++ {
				for i := 0; i < ssimWindow; i++ {
					w := kernel[j] * kernel[i]
					xv := float64(x[(oy-ssimRadius+j)*width+ox-ssimRadius+i])
					yv := float64(y[(oy-ssimRadius+j)*width+ox-ssimRadius+i])
					vx += w * xv
					vy += w * yv
					vx2 += w * xv * xv
					vy2 += w * yv * yv
					vxy += w * xv * yv
				}
			}
			sxx := vx2 - vx*vx
			syy := vy2 - vy*vy
			sxy := vxy - vx*vy
			num1 := 2*vx*vy + ssimC1
			num2 := 2*sxy + ssimC2
			den1 := vx*vx + vy*vy + ssimC1
			den2 := sxx + syy + ssimC2
			sumSSIM += (num1 * num2) / (den1 * den2)
			sumCS += num2 / den2
			count++
		}
	}
	return sumSSIM / float64(count), sumCS / float64(count)
}

// compareSSIM 走领域入口 CompareImages 验证多通道平均 SSIM。
func compareSSIM(t *testing.T, src, dst image.Image) float64 {
	t.Helper()
	res, err := CompareImages(context.Background(), src, dst, Options{Metrics: MetricSSIM})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return res.SSIM
}

func TestSSIMIdenticalIsOne(t *testing.T) {
	src := gradientImage(64, 48)
	if got := compareSSIM(t, src, src); math.Abs(got-1) > 1e-12 {
		t.Fatalf("ssim = %v, want 1 for identical images", got)
	}
}

func TestSSIMPlaneCSIdenticalIsOne(t *testing.T) {
	src := gradientImage(64, 48)
	p := mustSSIMPair(t, src, src)

	_, cs, err := ssimPlane(context.Background(), p.src[0], p.dst[0], p.width, p.height)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(cs-1) > 1e-12 {
		t.Fatalf("cs = %v, want 1 for identical planes", cs)
	}
}

func TestSSIMSymmetry(t *testing.T) {
	src := gradientImage(64, 48)
	dst := distortImage(src)

	ab := compareSSIM(t, src, dst)
	ba := compareSSIM(t, dst, src)
	if math.Abs(ab-ba) > 1e-12 {
		t.Fatalf("ssim(A,B) = %v, ssim(B,A) = %v, want equal", ab, ba)
	}
}

// 范围：SSIM ∈ [-1, 1]，失真后严格小于 1。
func TestSSIMRange(t *testing.T) {
	src := gradientImage(64, 48)
	dst := distortImage(src)

	got := compareSSIM(t, src, dst)
	if got < -1 || got > 1 {
		t.Fatalf("ssim = %v, want within [-1, 1]", got)
	}
	if got >= 1 {
		t.Fatalf("ssim = %v, want < 1 for distorted image", got)
	}
}

// 亮度整体偏移保持结构，SSIM 应显著高于强噪声。
func TestSSIMBrightnessShiftVersusNoise(t *testing.T) {
	src := gradientImage(64, 48)
	b := src.Bounds()

	shifted := fillNRGBA(b.Dx(), b.Dy(), func(x, y int) color.NRGBA {
		c := src.NRGBAAt(x, y)
		return color.NRGBA{R: clamp8(int(c.R) + 20), G: clamp8(int(c.G) + 20), B: clamp8(int(c.B) + 20), A: 255}
	})
	noisy := fillNRGBA(b.Dx(), b.Dy(), func(x, y int) color.NRGBA {
		c := src.NRGBAAt(x, y)
		n := int((x*73+y*151)%81) - 40
		return color.NRGBA{R: clamp8(int(c.R) + n), G: clamp8(int(c.G) + n), B: clamp8(int(c.B) + n), A: 255}
	})

	shiftSSIM := compareSSIM(t, src, shifted)
	noiseSSIM := compareSSIM(t, src, noisy)

	if shiftSSIM <= 0.5 || shiftSSIM >= 1 {
		t.Fatalf("brightness-shift ssim = %v, want in (0.5, 1)", shiftSSIM)
	}
	if noiseSSIM <= 0 || noiseSSIM >= shiftSSIM {
		t.Fatalf("noise ssim = %v, want in (0, brightness-shift ssim %v)", noiseSSIM, shiftSSIM)
	}
}

// 宽或高小于 11 的图必须被拒绝，返回 ErrImageTooSmall。
func TestSSIMTooSmallRejected(t *testing.T) {
	tests := []struct {
		name   string
		w, h   int
		wantOK bool
	}{
		{"10x10", 10, 10, false},
		{"11x10", 11, 10, false},
		{"10x11", 10, 11, false},
		{"11x11", 11, 11, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := gradientImage(tt.w, tt.h)
			p := mustSSIMPair(t, src, src)
			_, _, err := ssimPlane(context.Background(), p.src[0], p.dst[0], p.width, p.height)
			if tt.wantOK {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, ErrImageTooSmall) {
				t.Fatalf("error = %v, want ErrImageTooSmall", err)
			}
		})
	}
}

// 11×11 恰好只有一个完整窗口：streaming 结果必须等于直接计算。
func TestSSIM11x11Boundary(t *testing.T) {
	src := gradientImage(11, 11)
	dst := distortImage(src)
	p := mustSSIMPair(t, src, dst)

	got, cs, err := ssimPlane(context.Background(), p.src[0], p.dst[0], p.width, p.height)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want, wantCS := naiveSSIMPlane(p.src[0], p.dst[0], p.width, p.height)
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("ssim = %v, want naive %v", got, want)
	}
	if math.Abs(cs-wantCS) > 1e-12 {
		t.Fatalf("cs = %v, want naive %v", cs, wantCS)
	}
}

// streaming 实现与 naive 参考实现在整个图像上数值一致。
func TestSSIMMatchesNaiveReference(t *testing.T) {
	src := gradientImage(64, 48)
	dst := distortImage(src)
	p := mustSSIMPair(t, src, dst)

	for c := 0; c < p.channels; c++ {
		got, cs, err := ssimPlane(context.Background(), p.src[c], p.dst[c], p.width, p.height)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want, wantCS := naiveSSIMPlane(p.src[c], p.dst[c], p.width, p.height)
		if math.Abs(got-want) > 1e-9 {
			t.Fatalf("channel %d ssim = %v, want naive %v", c, got, want)
		}
		if math.Abs(cs-wantCS) > 1e-9 {
			t.Fatalf("channel %d cs = %v, want naive %v", c, cs, wantCS)
		}
	}
}

// 参考值回归：固化 opaque RGB 梯度 + 确定性偏差的 SSIM 常量。
// 常量由 naive 参考实现离线计算得出，防止 streaming 实现漂移。
func TestSSIMReferenceConstant(t *testing.T) {
	src := gradientImage(64, 48)
	dst := distortImage(src)

	got := compareSSIM(t, src, dst)
	// 参考值由 naive 参考实现离线计算固化（容差 1e-5，不要求 bit-identical）
	const want = 0.972806
	if math.Abs(got-want) > 1e-5 {
		t.Fatalf("ssim = %v, want reference %v", got, want)
	}
}

// 多通道平均：只有 B 通道失真时，总 SSIM 是各通道的平均。
func TestSSIMMultiChannelAverage(t *testing.T) {
	src := gradientImage(32, 32)
	dst := fillNRGBA(32, 32, func(x, y int) color.NRGBA {
		c := src.NRGBAAt(x, y)
		return color.NRGBA{R: c.R, G: c.G, B: clamp8(int(c.B) + ((x + y) % 7) - 3), A: 255}
	})

	got := compareSSIM(t, src, dst)

	p := mustSSIMPair(t, src, dst)
	var sum float64
	for c := 0; c < p.channels; c++ {
		v, _, err := ssimPlane(context.Background(), p.src[c], p.dst[c], p.width, p.height)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		sum += v
	}
	if want := sum / float64(p.channels); math.Abs(got-want) > 1e-12 {
		t.Fatalf("ssim = %v, want channel average %v", got, want)
	}
}
