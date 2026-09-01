package compare

import (
	"context"
	"fmt"
	"math"
)

// msSSIMScales 是固定尺度数。MS-SSIM 始终是五尺度定义，不因图片小
// 而自动减少——否则同一数值在不同尺寸图片上代表不同的数学定义。
const msSSIMScales = 5

// msSSIMWeights 是 Wang/Simoncelli/Bovik 经典五尺度权重。
var msSSIMWeights = [msSSIMScales]float64{0.0448, 0.2856, 0.3001, 0.2363, 0.1333}

// msSSIMMinDim 是 MS-SSIM 的最小短边：五尺度下第 5 尺度必须仍有
// 完整 11×11 窗口，即短边 >= (11-1)·2⁴+1 = 161。
const msSSIMMinDim = (ssimWindow-1)<<4 + 1

// msSSIM 计算全部活动通道的平均 MS-SSIM（每通道独立计算后取平均）。
func msSSIM(ctx context.Context, p *pixelPlanes) (float64, error) {
	var sum float64
	for c := 0; c < p.channels; c++ {
		v, err := msSSIMChannel(ctx, p.src[c], p.dst[c], p.width, p.height)
		if err != nil {
			return 0, err
		}
		sum += v
	}
	return sum / float64(p.channels), nil
}

// msSSIMChannel 对单通道平面计算五尺度 MS-SSIM：
//
//	MS-SSIM = SSIM_5^w5 · Π_{j=1..4} CS_j^w_j
//
// 实现：
//   - 尺度间用 2×2 均值低通加 2 倍降采样（奇数边缘块只平均实际存在
//     的像素，半除为 ceil，因此 161→81→41→21→11 后第 5 尺度恰好
//     留下一个完整窗口）；
//   - 分数幂之前对每尺度 cs 与最终 ssim 做非负钳制（cs<0 时分数幂
//     会产生 NaN），保证结果 ∈ [0,1]，这与常见 PyTorch 实现一致；
//   - 金字塔只保留当前层和下一层，顺序消费，不保存全部尺度像素；
//     中间平面使用 float32，统计累加使用 float64。
func msSSIMChannel(ctx context.Context, x, y []float32, width, height int) (float64, error) {
	if min(width, height) < msSSIMMinDim {
		return 0, fmt.Errorf("%w: MS-SSIM requires both image dimensions >= %d pixels; use --psnr or --ssim for smaller images (got %dx%d)",
			ErrImageTooSmall, msSSIMMinDim, width, height)
	}

	var csMeans [msSSIMScales - 1]float64
	xs, ys := x, y
	ws, hs := width, height

	for scale := 0; scale < msSSIMScales; scale++ {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		ssimMean, csMean, err := ssimPlane(ctx, xs, ys, ws, hs)
		if err != nil {
			return 0, err
		}
		if scale == msSSIMScales-1 {
			return combineMSSSIM(ssimMean, csMeans), nil
		}
		csMeans[scale] = csMean

		nextW, nextH := (ws+1)/2, (hs+1)/2
		nextX, nextY := downsample2x2(xs, ws, hs, nextW, nextH), downsample2x2(ys, ws, hs, nextW, nextH)
		xs, ys, ws, hs = nextX, nextY, nextW, nextH
	}
	panic("unreachable")
}

// combineMSSSIM 按固定权重组合五尺度结果：幂运算前对 ssim 与 cs 做
// 非负钳制。
func combineMSSSIM(ssim5 float64, csMeans [msSSIMScales - 1]float64) float64 {
	result := math.Pow(max(ssim5, 0), msSSIMWeights[msSSIMScales-1])
	for j := range csMeans {
		result *= math.Pow(max(csMeans[j], 0), msSSIMWeights[j])
	}
	return result
}

// downsample2x2 对平面做 2×2 均值低通加 2 倍降采样。
//
// 奇数宽/高的边缘块只平均实际存在的 1-2 个像素（新尺寸为 ceil 半除），
// 偶数尺寸的行为与标准 2×2 平均完全一致。
func downsample2x2(in []float32, width, height, newWidth, newHeight int) []float32 {
	out := make([]float32, newWidth*newHeight)
	for y := 0; y < newHeight; y++ {
		y0 := 2 * y
		y1 := min(y0+1, height-1)
		yCount := y1 - y0 + 1
		for x := 0; x < newWidth; x++ {
			x0 := 2 * x
			x1 := min(x0+1, width-1)
			xCount := x1 - x0 + 1
			// x0==x1（或 y0==y1）时同一像素被加两次，除以对应计数后
			// 恰好等于单像素（或行/列）平均值。
			sum := in[y0*width+x0] + in[y0*width+x1] + in[y1*width+x0] + in[y1*width+x1]
			out[y*newWidth+x] = sum / float32(xCount*yCount)
		}
	}
	return out
}
