package compare

import (
	"context"
	"math"
)

// psnrDataRange 是 PSNR 的固定动态范围上限（8 位样本）。
const psnrDataRange = 255.0

// psnrChannel 累加单通道全部样本的平方差（Σ(x-y)²）。平方差以
// float64 累加，逐行遍历不产生 per-pixel 分配。
func psnrChannel(ctx context.Context, x, y []float32, width, height int) (float64, error) {
	var sum float64
	for row := 0; row < height; row++ {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		base := row * width
		for i := 0; i < width; i++ {
			d := float64(x[base+i]) - float64(y[base+i])
			sum += d * d
		}
	}
	return sum, nil
}

// psnrFromSumSq 由全部活动通道的平方差和计算 PSNR（单位 dB）：
//
//	MSE  = sumSq / N     —— N 是所有通道的全部样本数
//	PSNR = 10 * log10(255^2 / MSE)
//
// MSE == 0（活动通道完全一致）时返回 +Inf，不用 99.99 之类的有限数值
// 人为替代。
func psnrFromSumSq(sumSq float64, samples int) float64 {
	mse := sumSq / float64(samples)
	if mse == 0 {
		return math.Inf(1)
	}
	return 10 * math.Log10(psnrDataRange*psnrDataRange/mse)
}
