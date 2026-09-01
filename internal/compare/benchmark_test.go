package compare

import (
	"context"
	"testing"
)

// benchmarkPlanes 预构造 w×h 的梯度比较平面，指标循环内不再计平面提取。
func benchmarkPlanes(b *testing.B, w, h int) *pixelPlanes {
	b.Helper()
	src := gradientImage(w, h)
	dst := distortImage(src)
	p, err := newPixelPlanes(context.Background(), src, dst)
	if err != nil {
		b.Fatalf("unexpected error: %v", err)
	}
	return p
}

// 基准不做 CI 时间门禁（GitHub Runner CPU 波动太大）；真正锁定的
// 是分配契约，见 TestMetricAllocationsDoNotScaleWithPixels。
func BenchmarkPSNR1080p(b *testing.B) {
	p := benchmarkPlanes(b, 1920, 1080)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := psnr(context.Background(), p); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSSIM1080p(b *testing.B) {
	p := benchmarkPlanes(b, 1920, 1080)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ssim(context.Background(), p); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMSSSIM1080p(b *testing.B) {
	p := benchmarkPlanes(b, 1920, 1080)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := msSSIM(context.Background(), p); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMSSSIM4K(b *testing.B) {
	p := benchmarkPlanes(b, 3840, 2160)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := msSSIM(context.Background(), p); err != nil {
			b.Fatal(err)
		}
	}
}

// TestMetricAllocationsDoNotScaleWithPixels 锁定分配契约：
//
//   - psnr 全程零分配；
//   - ssimPlane 的分配只与 ring buffer 行数有关，与像素数无关；
//   - msSSIMChannel 每尺度只分配下一层平面，分配次数不随像素数增长。
//
// 平面提取本身按 float32 平面物化输入是设计允许的（pixelPlanes），
// 不在本测试范围。
func TestMetricAllocationsDoNotScaleWithPixels(t *testing.T) {
	gs := gradientImage(64, 64)
	small := mustSSIMPair(t, gs, distortImage(gs))
	gl := gradientImage(256, 256)
	large := mustSSIMPair(t, gl, distortImage(gl))
	gm := gradientImage(192, 192) // msSSIMChannel 要求 >= 161
	mid := mustSSIMPair(t, gm, distortImage(gm))

	if n := testing.AllocsPerRun(10, func() {
		_, _ = psnr(context.Background(), small)
	}); n != 0 {
		t.Fatalf("psnr allocs = %v per run, want 0", n)
	}

	smallSSIM := testing.AllocsPerRun(10, func() {
		_, _, _ = ssimPlane(context.Background(), small.src[0], small.dst[0], small.width, small.height)
	})
	largeSSIM := testing.AllocsPerRun(10, func() {
		_, _, _ = ssimPlane(context.Background(), large.src[0], large.dst[0], large.width, large.height)
	})
	if largeSSIM > smallSSIM+8 {
		t.Fatalf("ssimPlane allocs grew with pixels: small = %v, large = %v", smallSSIM, largeSSIM)
	}

	msAllocs := testing.AllocsPerRun(5, func() {
		_, _ = msSSIMChannel(context.Background(), mid.src[0], mid.dst[0], mid.width, mid.height)
	})
	if msAllocs >= 500 {
		t.Fatalf("msSSIMChannel allocs = %v per run on 192x192, want bounded well below per-pixel", msAllocs)
	}
}
