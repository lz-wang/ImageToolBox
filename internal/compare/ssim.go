package compare

import (
	"context"
	"errors"
	"fmt"
)

// SSIM 固定参数：原始 Wang 推荐配置，也是 TensorFlow 等常见实现使用的
// 标准参数。这些常量是指标定义的一部分，不得调整。
const (
	// ssimWindow 是 SSIM 的高斯窗口边长（11×11）。
	ssimWindow = 11

	// ssimRadius 是窗口半径 (11-1)/2。
	ssimRadius = ssimWindow / 2

	// ssimSigma 是高斯标准差。
	ssimSigma = 1.5

	// ssimK1 / ssimK2 是稳定常数系数。
	ssimK1 = 0.01
	ssimK2 = 0.03

	// ssimL 是动态范围（8 位样本）。
	ssimL = 255.0
)

var (
	// ssimC1 = (K1·L)²，ssimC2 = (K2·L)²。
	ssimC1 = (ssimK1 * ssimL) * (ssimK1 * ssimL)
	ssimC2 = (ssimK2 * ssimL) * (ssimK2 * ssimL)
)

// ErrImageTooSmall 表示图片（或 MS-SSIM 某个尺度）小于指标窗口要求的
// 最小尺寸。指标定义固定，不为小图自动缩窗或减尺度。
var ErrImageTooSmall = errors.New("image too small for metric")

// ssimMoment 是 ring buffer 中逐行维护的五类统计矩。
type ssimMoment int

const (
	momentX ssimMoment = iota
	momentY
	momentX2
	momentY2
	momentXY
	momentCount
)

// ssimPlane 对单通道平面计算窗口平均 SSIM 与 CS（contrast-structure 项）。
//
// 指标是全部活动通道的平均：每个通道独立计算后取平均（聚合在
// CompareImages 的通道循环里完成）。不做自动 downsample，直接针对
// 原始逻辑分辨率计算（不复刻早期 Matlab ssim.m 根据尺度缩图的
// suggested usage）。
//
// 实现为 separable 高斯（1×11 水平 + 11×1 垂直）加 11 行 ring buffer 的
// streaming statistics：μx、μy、E[x²]、E[y²]、E[xy] 只以
// O(width×window) 的行缓冲存在，逐窗口立即算出 SSIM 并累计 sum/count，
// 从不分配 5×image-size 的整幅 float 缓冲。
//
// 只统计完整落入图内的窗口（'valid' 模式），与原始 Wang 实现一致；
// 输出位置为 x∈[5, width-6]、y∈[5, height-6]。普通 SSIM 可能小于 0，
// 不做 max(ssim, 0) 钳制（非负钳制只发生在 MS-SSIM 的幂运算前）。
func ssimPlane(ctx context.Context, x, y []float32, width, height int) (meanSSIM, meanCS float64, err error) {
	if width < ssimWindow || height < ssimWindow {
		return 0, 0, fmt.Errorf("%w: SSIM requires both image dimensions >= %d pixels, got %dx%d",
			ErrImageTooSmall, ssimWindow, width, height)
	}

	kernel := gaussianKernel(ssimWindow, ssimSigma)

	// ring[m][row%ssimWindow] 缓存第 row 行的水平卷积结果（moment m）。
	var ring [momentCount][][]float64
	for m := range ring {
		ring[m] = make([][]float64, ssimWindow)
		for i := range ring[m] {
			ring[m][i] = make([]float64, width)
		}
	}

	outWidth := width - 2*ssimRadius
	outHeight := height - 2*ssimRadius
	count := float64(outWidth * outHeight)
	var sumSSIM, sumCS float64

	// computeHRow 把输入第 row 行的五个统计矩做水平卷积写入 ring。
	computeHRow := func(row int) {
		xr := x[row*width : (row+1)*width]
		yr := y[row*width : (row+1)*width]
		hx := ring[momentX][row%ssimWindow]
		hy := ring[momentY][row%ssimWindow]
		hx2 := ring[momentX2][row%ssimWindow]
		hy2 := ring[momentY2][row%ssimWindow]
		hxy := ring[momentXY][row%ssimWindow]

		for c := ssimRadius; c+ssimRadius < width; c++ {
			var vx, vy, vx2, vy2, vxy float64
			for i := 0; i < ssimWindow; i++ {
				w := kernel[i]
				xv := float64(xr[c-ssimRadius+i])
				yv := float64(yr[c-ssimRadius+i])
				vx += w * xv
				vy += w * yv
				vx2 += w * xv * xv
				vy2 += w * yv * yv
				vxy += w * xv * yv
			}
			hx[c], hy[c], hx2[c], hy2[c], hxy[c] = vx, vy, vx2, vy2, vxy
		}
	}

	// emitRow 在 ring 覆盖输入行 [r-2·radius, r] 后产出输出行 r-radius，
	// 垂直卷积与 SSIM/CS 公式逐像素计算并只累计 sum。
	emitRow := func(r int) {
		ry := r - ssimRadius
		var vrows [ssimWindow][momentCount][]float64
		for j := 0; j < ssimWindow; j++ {
			slot := (ry - ssimRadius + j) % ssimWindow
			for m := 0; m < int(momentCount); m++ {
				vrows[j][m] = ring[m][slot]
			}
		}

		for cx := ssimRadius; cx+ssimRadius < width; cx++ {
			var vx, vy, vx2, vy2, vxy float64
			for j := 0; j < ssimWindow; j++ {
				w := kernel[j]
				vx += w * vrows[j][momentX][cx]
				vy += w * vrows[j][momentY][cx]
				vx2 += w * vrows[j][momentX2][cx]
				vy2 += w * vrows[j][momentY2][cx]
				vxy += w * vrows[j][momentXY][cx]
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
		}
	}

	for row := 0; row < height; row++ {
		if err := ctx.Err(); err != nil {
			return 0, 0, err
		}
		computeHRow(row)
		if row >= ssimWindow-1 {
			emitRow(row)
		}
	}

	return sumSSIM / count, sumCS / count, nil
}
