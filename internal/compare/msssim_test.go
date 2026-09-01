package compare

import (
	"context"
	"math"
	"testing"
)

// fillPlane 生成 w×h 的确定性平面，第 i 个样本为 float32(i+1)。
func fillPlane(w, h int) []float32 {
	in := make([]float32, w*h)
	for i := range in {
		in[i] = float32(i + 1)
	}
	return in
}

// naiveDownsample2x2 是独立于生产实现的参考下采样：逐输出像素扫描
// 2×2 源块，只累计实际落在图内的源像素，不做 clamp 重复索引，尺寸由
// 自己按 ceil 半除推导，累加走 float64。与生产 downsample2x2 的算法
// 路径刻意不同，避免参考实现与被测实现共享同一处错误。
func naiveDownsample2x2(in []float32, width, height int) []float32 {
	newWidth, newHeight := (width+1)/2, (height+1)/2
	out := make([]float32, newWidth*newHeight)
	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			sum := 0.0
			count := 0
			for dy := 0; dy < 2; dy++ {
				sy := 2*y + dy
				if sy >= height {
					break
				}
				for dx := 0; dx < 2; dx++ {
					sx := 2*x + dx
					if sx >= width {
						break
					}
					sum += float64(in[sy*width+sx])
					count++
				}
			}
			out[y*newWidth+x] = float32(sum / float64(count))
		}
	}
	return out
}

// 奇数宽/高边缘块只平均实际存在的像素：右/下边缘的 1-2 像素块绝不能
// 因重复采样被放大。期望值按公式手工推导（fillPlane 的样本为 1..w·h）。
func TestDownsample2x2(t *testing.T) {
	tests := []struct {
		name         string
		w, h         int
		wantW, wantH int
		want         []float32
	}{
		{"2x2 全四像素平均", 2, 2, 1, 1, []float32{2.5}},
		{"3x2 奇数宽边缘列", 3, 2, 2, 1, []float32{3, 4.5}},
		{"2x3 奇数高边缘行", 2, 3, 1, 2, []float32{2.5, 5.5}},
		{"3x3 右下角单像素块", 3, 3, 2, 2, []float32{3, 4.5, 7.5, 9}},
		{
			"5x5 交替奇偶块", 5, 5, 3, 3,
			[]float32{4, 6, 7.5, 14, 16, 17.5, 21.5, 23.5, 25},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := downsample2x2(fillPlane(tt.w, tt.h), tt.w, tt.h, tt.wantW, tt.wantH)
			if len(got) != len(tt.want) {
				t.Fatalf("输出长度 = %d, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if math.Abs(float64(got[i]-tt.want[i])) > 1e-6 {
					t.Fatalf("out[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// 161×161 是 MS-SSIM 的最小允许短边：金字塔 161→81→41→21→11 每层
// 都是奇数，每一层的奇数边缘块都必须与独立 naive 参考一致。
func TestDownsample2x2Pyramid161(t *testing.T) {
	cur := fillPlane(161, 161)
	w, h := 161, 161
	for _, next := range [][2]int{{81, 81}, {41, 41}, {21, 21}, {11, 11}} {
		got := downsample2x2(cur, w, h, next[0], next[1])
		naive := naiveDownsample2x2(cur, w, h)
		if len(got) != len(naive) {
			t.Fatalf("尺寸 %dx%d: 输出长度 = %d, want %d", w, h, len(got), len(naive))
		}
		for i := range got {
			if math.Abs(float64(got[i]-naive[i])) > 1e-2 {
				t.Fatalf("尺寸 %dx%d: out[%d] = %v, want naive %v", w, h, i, got[i], naive[i])
			}
		}
		cur, w, h = got, next[0], next[1]
	}
}

// 奇数尺寸下生产 MS-SSIM 与 naive 参考实现数值一致：321×257 的金字塔
// 321×257→161×129→81×65→41×33→21×17 每层都是奇数，任何一处奇数边缘
// 语义漂移都会被放大到最终结果。
func TestMSSSIMMatchesNaiveOddDimensions(t *testing.T) {
	src := gradientImage(321, 257)
	dst := distortImage(src)
	p := mustSSIMPair(t, src, dst)

	res, err := CompareImages(context.Background(), src, dst, Options{Metrics: MetricMSSSIM})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var naiveSum float64
	for c := 0; c < p.channels; c++ {
		naiveSum += naiveMSSSIMChannel(p.src[c], p.dst[c], p.width, p.height)
	}
	if want := naiveSum / float64(p.channels); math.Abs(res.MSSSIM-want) > 1e-9 {
		t.Fatalf("MS-SSIM = %v, want naive %v", res.MSSSIM, want)
	}
}

// 321×257 奇数尺寸的完整 MS-SSIM 参考向量：金字塔每层都是奇数，
// 冻结修复后的奇数边缘下采样语义。常量由独立 naive 参考实现离线
// 计算固化（容差 1e-5），运行期不依赖外部工具。
func TestReferenceVectorMSSSIMOddDimensions(t *testing.T) {
	src := gradientImage(321, 257)
	dst := distortImage(src)

	res, err := CompareImages(context.Background(), src, dst, Options{Metrics: allMetrics})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// naive 独立参考实现离线计算：MS-SSIM=0.9927552457
	const wantMSSSIM = 0.992755
	if math.Abs(res.MSSSIM-wantMSSSIM) > 1e-5 {
		t.Fatalf("MS-SSIM = %v, want reference %v", res.MSSSIM, wantMSSSIM)
	}
}
