// Package compare 提供只读图片质量指标（PSNR/SSIM/MS-SSIM）的纯 Go 实现。
//
// compare 是显式调用的只读分析命令的领域层：不修改任何输入文件，
// 也绝不隐式 resize/crop/pad 尺寸不一致的图片，指标数学定义保持稳定。
// 包不依赖 urfave/cli，仅暴露 CLI-only 的能力（不进入 HTTP API）。
package compare

import (
	"context"
	"fmt"
	"image"

	"imagetoolbox/internal/imageio"
)

// Metrics 是位掩码形式的指标选择。
type Metrics uint8

const (
	MetricPSNR Metrics = 1 << iota
	MetricSSIM
	MetricMSSSIM
)

// allMetrics 是全部已定义的指标位。
const allMetrics = MetricPSNR | MetricSSIM | MetricMSSSIM

// DefaultMetrics 是未显式选择指标时的默认组合。
const DefaultMetrics = MetricPSNR | MetricMSSSIM

// Options 是 compare 的领域选项，领域层是默认值与最终校验的唯一
// 事实来源。
type Options struct {
	// Metrics 是期望计算的指标位掩码；零值由 Normalize 填充
	// DefaultMetrics。
	Metrics Metrics
}

// Normalize 应用领域默认值：未选择任何指标时使用 DefaultMetrics
// （PSNR + MS-SSIM）。CLI adapter 在用户显式提供指标 flag 却全部
// 关闭（如 --psnr=false）时应在 transport 层直接报错，不依赖这里
// 的零值默认机制。
func (o *Options) Normalize() {
	if o.Metrics == 0 {
		o.Metrics = DefaultMetrics
	}
}

// Validate 校验指标位掩码非空且只包含已定义的指标。
func (o Options) Validate() error {
	if o.Metrics == 0 {
		return fmt.Errorf("compare: 至少需要选择一个比较指标")
	}
	if o.Metrics&^allMetrics != 0 {
		return fmt.Errorf("compare: 未知的指标位: %d", uint8(o.Metrics&^allMetrics))
	}
	return nil
}

// Result 是比较结果，Metrics 标记哪些指标字段有效。
type Result struct {
	Width   int
	Height  int
	Metrics Metrics

	PSNR   float64 // 单位 dB；活动通道完全一致时为 +Inf
	SSIM   float64
	MSSSIM float64
}

// CompareImages 比较两张已解码图片。逻辑尺寸必须完全一致，否则报错；
// 两张图都是只读输入，同一张图与自身比较是合法的 sanity check
// （PSNR 为 +Inf、SSIM/MS-SSIM 为 1）。ctx 用于取消耗时的指标计算。
//
// 内存策略是逐通道"提取-计算-复用"：xs/ys 一对通道平面在全部通道间
// 复用，峰值工作集只有 2×N float32 加上解码图，而不是物化全部 6/8 个
// 平面——24/48MP 摄影原图的指标工作集因此降到约三分之一。
func CompareImages(ctx context.Context, src, dst image.Image, opts Options) (Result, error) {
	opts.Normalize()
	if err := opts.Validate(); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	sb, db := src.Bounds(), dst.Bounds()
	if sb.Dx() != db.Dx() || sb.Dy() != db.Dy() {
		return Result{}, fmt.Errorf("图片尺寸不一致: src=%dx%d, dst=%dx%d", sb.Dx(), sb.Dy(), db.Dx(), db.Dy())
	}
	width, height := sb.Dx(), sb.Dy()

	channels := activeChannelCount(src, dst)
	premultiply := channels == alphaChannelCount

	xs := make([]float32, width*height)
	ys := make([]float32, width*height)

	var psnrSumSq float64
	var ssimSum, msSum float64
	for c := 0; c < channels; c++ {
		if err := extractChannel(ctx, src, xs, c, premultiply); err != nil {
			return Result{}, err
		}
		if err := extractChannel(ctx, dst, ys, c, premultiply); err != nil {
			return Result{}, err
		}
		if opts.Metrics&MetricPSNR != 0 {
			sq, err := psnrChannel(ctx, xs, ys, width, height)
			if err != nil {
				return Result{}, err
			}
			psnrSumSq += sq
		}
		if opts.Metrics&MetricSSIM != 0 {
			v, _, err := ssimPlane(ctx, xs, ys, width, height)
			if err != nil {
				return Result{}, err
			}
			ssimSum += v
		}
		if opts.Metrics&MetricMSSSIM != 0 {
			v, err := msSSIMChannel(ctx, xs, ys, width, height)
			if err != nil {
				return Result{}, err
			}
			msSum += v
		}
	}

	result := Result{Width: width, Height: height, Metrics: opts.Metrics}
	if opts.Metrics&MetricPSNR != 0 {
		result.PSNR = psnrFromSumSq(psnrSumSq, width*height*channels)
	}
	if opts.Metrics&MetricSSIM != 0 {
		result.SSIM = ssimSum / float64(channels)
	}
	if opts.Metrics&MetricMSSSIM != 0 {
		result.MSSSIM = msSum / float64(channels)
	}
	return result, nil
}

// CompareFiles 读取两张图片文件并比较。
//
// 输入统一走 imageio.OpenStatic：严格限定 JPEG/PNG/WebP（拒绝
// GIF/BMP/TIFF），并把 JPEG EXIF Orientation 烘焙进像素——比较的是
// 应用 Orientation 后的实际视觉像素，而不是文件字节、格式或 metadata。
// src 与 dst 都是输入，这里绝不调用 imageio.RejectSameFile。
func CompareFiles(ctx context.Context, srcPath, dstPath string, opts Options) (Result, error) {
	src, err := imageio.OpenStatic(srcPath)
	if err != nil {
		return Result{}, err
	}
	dst, err := imageio.OpenStatic(dstPath)
	if err != nil {
		return Result{}, err
	}
	return CompareImages(ctx, src, dst, opts)
}
