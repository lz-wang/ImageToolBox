package compare

import "math"

// gaussianKernel 生成归一化的一维高斯核（长度 size，必须为奇数）。
//
// 高斯核是 separable 的：11×11 的二维窗口卷积拆成 1×11 水平加 11×1
// 垂直两趟，每输出像素的采样数从 121 降到 22。SSIM 固定
// size=ssimWindow、sigma=ssimSigma。
func gaussianKernel(size int, sigma float64) []float64 {
	if size <= 0 || size%2 == 0 {
		panic("gaussian kernel size must be a positive odd number")
	}

	kernel := make([]float64, size)
	center := float64(size-1) / 2
	var sum float64
	for i := range kernel {
		d := float64(i) - center
		kernel[i] = math.Exp(-(d * d) / (2 * sigma * sigma))
		sum += kernel[i]
	}
	for i := range kernel {
		kernel[i] /= sum
	}
	return kernel
}
