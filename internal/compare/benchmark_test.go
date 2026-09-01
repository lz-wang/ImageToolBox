package compare

import (
	"context"
	"testing"
)

// benchmarkPlanes 预构造 w×h 的梯度比较平面，kernel 循环内不再计平面提取。
func benchmarkPlanes(b *testing.B, w, h int) *testPlanes {
	b.Helper()
	src := gradientImage(w, h)
	dst := distortImage(src)
	return materializePlanes(b, src, dst)
}

// kernel 基准只测单通道指标核（输入平面已提前物化）；完整 compare
// 路径（含逐通道提取与全部指标）见 BenchmarkCompareImages*。
//
// 基准不做 CI 时间门禁（GitHub Runner CPU 波动太大）；真正锁定的
// 是分配契约，见 TestMetricKernelAllocationCountDoesNotScaleWithPixels。
func BenchmarkPSNRKernel1080p(b *testing.B) {
	p := benchmarkPlanes(b, 1920, 1080)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := psnrChannel(context.Background(), p.src[0], p.dst[0], p.width, p.height); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSSIMKernel1080p(b *testing.B) {
	p := benchmarkPlanes(b, 1920, 1080)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := ssimPlane(context.Background(), p.src[0], p.dst[0], p.width, p.height); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMSSSIMKernel1080p(b *testing.B) {
	p := benchmarkPlanes(b, 1920, 1080)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := msSSIMChannel(context.Background(), p.src[0], p.dst[0], p.width, p.height); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMSSSIMKernel4K(b *testing.B) {
	p := benchmarkPlanes(b, 3840, 2160)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := msSSIMChannel(context.Background(), p.src[0], p.dst[0], p.width, p.height); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompareImages* 测完整 compare 路径：逐通道提取 + 全部指标
// （allMetrics），反映真实 `itb compare <src> <dst>` 命令的耗时与分配
// （decode 除外），而不是提前物化平面后的纯指标核。
func BenchmarkCompareImages1080p(b *testing.B) {
	src := gradientImage(1920, 1080)
	dst := distortImage(src)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := CompareImages(context.Background(), src, dst, Options{Metrics: allMetrics}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompareImages4K(b *testing.B) {
	src := gradientImage(3840, 2160)
	dst := distortImage(src)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := CompareImages(context.Background(), src, dst, Options{Metrics: allMetrics}); err != nil {
			b.Fatal(err)
		}
	}
}

// TestMetricKernelAllocationCountDoesNotScaleWithPixels 锁定指标核的
// 分配契约：
//
//   - psnrChannel 全程零分配；
//   - ssimPlane 的分配只与 ring buffer 行数有关，与像素数无关；
//   - msSSIMChannel 每尺度只分配下一层平面，分配次数不随像素数增长。
//
// 平面提取本身按 float32 平面物化输入是设计允许的，完整
// CompareImages 的分配当然随像素数增长，不在本测试范围。
func TestMetricKernelAllocationCountDoesNotScaleWithPixels(t *testing.T) {
	gs := gradientImage(64, 64)
	small := mustSSIMPair(t, gs, distortImage(gs))
	gl := gradientImage(256, 256)
	large := mustSSIMPair(t, gl, distortImage(gl))
	gm := gradientImage(192, 192) // msSSIMChannel 要求 >= 161
	mid := mustSSIMPair(t, gm, distortImage(gm))

	if n := testing.AllocsPerRun(10, func() {
		_, _ = psnrChannel(context.Background(), small.src[0], small.dst[0], small.width, small.height)
	}); n != 0 {
		t.Fatalf("psnrChannel allocs = %v per run, want 0", n)
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
