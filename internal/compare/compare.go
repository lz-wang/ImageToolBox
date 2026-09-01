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
func CompareImages(ctx context.Context, src, dst image.Image, opts Options) (Result, error) {
	opts.Normalize()
	if err := opts.Validate(); err != nil {
		return Result{}, err
	}

	planes, err := newPixelPlanes(ctx, src, dst)
	if err != nil {
		return Result{}, err
	}

	result := Result{Width: planes.width, Height: planes.height, Metrics: opts.Metrics}
	if opts.Metrics&MetricPSNR != 0 {
		v, err := psnr(ctx, planes)
		if err != nil {
			return Result{}, err
		}
		result.PSNR = v
	}
	if opts.Metrics&MetricSSIM != 0 {
		v, err := ssim(ctx, planes)
		if err != nil {
			return Result{}, err
		}
		result.SSIM = v
	}
	if opts.Metrics&MetricMSSSIM != 0 {
		v, err := msSSIM(ctx, planes)
		if err != nil {
			return Result{}, err
		}
		result.MSSSIM = v
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
