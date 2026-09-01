package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"imagetoolbox/internal/compare"
)

func newCompareCommand() *cli.Command {
	return &cli.Command{
		Name:      "compare",
		Usage:     "比较两张图片的客观质量指标",
		ArgsUsage: "<src> <dst>",
		Description: `只读比较两张图片的客观质量指标（PSNR / SSIM / MS-SSIM）。

- <src> 与 <dst> 都是只读输入，命令不修改任何文件；同一文件自我比较
  是合法的 sanity check（PSNR 为 +Inf、SSIM / MS-SSIM 为 1）
- 支持格式: JPEG / PNG / WebP；JPEG 的 EXIF Orientation 已归一化，
  比较的是实际视觉像素
- 两张图片的逻辑尺寸必须完全一致，不会隐式 resize / crop / pad
- 未指定任何指标 flag 时默认计算 PSNR 和 MS-SSIM；一旦指定任意
  指标 flag，只计算显式选择的指标
- SSIM 要求两边均 >= 11×11；MS-SSIM 固定五尺度，要求短边 >= 161

示例:
	  itb compare original.jpg compressed.jpg
	  itb compare original.jpg compressed.jpg --ssim
	  itb compare original.png original.webp --psnr --ssim --ms-ssim`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "psnr",
				Usage: "计算 PSNR（峰值信噪比，单位 dB；完全一致为 +Inf）",
			},
			&cli.BoolFlag{
				Name:  "ssim",
				Usage: "计算 SSIM（结构相似性，11×11 高斯窗口、sigma 1.5 的标准参数）",
			},
			&cli.BoolFlag{
				Name:  "ms-ssim",
				Usage: "计算 MS-SSIM（固定五尺度，短边需 >= 161 像素）",
			},
		},
		Action: runCompare,
	}
}

// resolveCompareMetrics 解析指标 flag 的 transport 语义：
//
//   - 没有任何指标 flag 被 IsSet 时返回 0，交给 domain Normalize
//     得到默认的 PSNR + MS-SSIM；
//   - 只要任一指标 flag 显式提供（含 =false），则只组合显式为 true
//     的指标；全部显式关闭时直接报错，绝不让 domain 的零值默认机制
//     偷偷恢复默认指标。
func resolveCompareMetrics(cmd *cli.Command) (compare.Metrics, error) {
	psnrSet := cmd.IsSet("psnr")
	ssimSet := cmd.IsSet("ssim")
	msSSIMSet := cmd.IsSet("ms-ssim")
	if !psnrSet && !ssimSet && !msSSIMSet {
		return 0, nil
	}

	var metrics compare.Metrics
	if cmd.Bool("psnr") {
		metrics |= compare.MetricPSNR
	}
	if cmd.Bool("ssim") {
		metrics |= compare.MetricSSIM
	}
	if cmd.Bool("ms-ssim") {
		metrics |= compare.MetricMSSSIM
	}
	if metrics == 0 {
		return 0, fmt.Errorf("至少需要选择一个比较指标")
	}
	return metrics, nil
}

func runCompare(ctx context.Context, cmd *cli.Command) error {
	src, dst, err := sourceDestinationArgs(cmd, true)
	if err != nil {
		return err
	}

	metrics, err := resolveCompareMetrics(cmd)
	if err != nil {
		return err
	}

	result, err := compare.CompareFiles(ctx, src, dst, compare.Options{Metrics: metrics})
	if err != nil {
		return fmt.Errorf("比较失败: %w", err)
	}

	// 输出顺序固定为 PSNR、SSIM、MS-SSIM，与 flag 出现顺序无关。
	if result.Metrics&compare.MetricPSNR != 0 {
		fmt.Printf("PSNR: %.6f dB\n", result.PSNR)
	}
	if result.Metrics&compare.MetricSSIM != 0 {
		fmt.Printf("SSIM: %.6f\n", result.SSIM)
	}
	if result.Metrics&compare.MetricMSSSIM != 0 {
		fmt.Printf("MS-SSIM: %.6f\n", result.MSSSIM)
	}
	return nil
}
