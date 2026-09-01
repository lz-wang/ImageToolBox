package compare

import (
	"context"
	"math"
)

// psnrDataRange 是 PSNR 的固定动态范围上限（8 位样本）。
const psnrDataRange = 255.0

// psnr 基于全部活动通道样本的全局 MSE 计算 PSNR（单位 dB）：
//
//	MSE  = (1/N) * Σ (x_i - y_i)^2     —— N 是所有通道的全部样本数
//	PSNR = 10 * log10(255^2 / MSE)
//
// MSE == 0（活动通道完全一致）时返回 +Inf，不用 99.99 之类的有限数值
// 人为替代。平方差以 float64 累加，逐通道行遍历不产生 per-pixel 分配。
func psnr(ctx context.Context, p *pixelPlanes) (float64, error) {
	samples := p.width * p.height
	var sum float64
	for c := 0; c < p.channels; c++ {
		src, dst := p.src[c], p.dst[c]
		for row := 0; row < p.height; row++ {
			if err := ctx.Err(); err != nil {
				return 0, err
			}
			base := row * p.width
			for x := 0; x < p.width; x++ {
				d := float64(src[base+x]) - float64(dst[base+x])
				sum += d * d
			}
		}
	}

	mse := sum / float64(samples*p.channels)
	if mse == 0 {
		return math.Inf(1), nil
	}
	return 10 * math.Log10(psnrDataRange*psnrDataRange/mse), nil
}
