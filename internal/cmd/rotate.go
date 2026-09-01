package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
	"imagetoolbox/internal/imageio"
	"imagetoolbox/internal/rotate"
)

func newRotateCommand() *cli.Command {
	return &cli.Command{
		Name:      "rotate",
		Usage:     "Rotate an image",
		Category:  categoryImageTransforms,
		ArgsUsage: "<src> [dst]",
		Description: `旋转图片。省略 [dst] 时输出到原文件名后加 _rotated 的新文件。

规则:
  - 旋转角度: 正数表示逆时针，负数表示顺时针；支持小数角度，
    范围 (-360, 360)，不能为 0
  - 任意角度按需调整输出画布，避免常规角度下裁掉主体内容
  - PNG/WebP 新增区域保持透明，JPEG 使用白色背景
  - 精确 90/180/270 不做插值，其余角度使用双线性插值
  - 支持格式: JPEG / PNG / WebP；JPEG 的 EXIF Orientation 先归一化，
    再执行本次旋转

示例:
	  itb rotate --angle 90 photo.jpg
	  itb rotate --angle -90 photo.jpg clockwise.jpg
	  itb rotate --angle 45 photo.png rotated.png
	  itb rotate --angle 22.5 photo.webp rotated.webp`,
		Flags: []cli.Flag{
			&cli.FloatFlag{
				Name:      "angle",
				Usage:     "旋转角度（度）: 正数逆时针、负数顺时针，支持小数；范围 (-360, 360) 且不能为 0",
				Required:  true,
				Validator: nonZeroOpenRangeFloatValidator("angle", -360, 360),
			},
		},
		Action: runRotate,
	}
}

func runRotate(ctx context.Context, cmd *cli.Command) error {
	inputFile, outputPath, err := sourceDestinationArgs(cmd, false)
	if err != nil {
		return err
	}
	if outputPath == "" {
		outputPath = imageio.SuffixedPath(inputFile, "_rotated")
	}

	if err := rotate.RotateFile(inputFile, outputPath, rotate.Options{
		Angle: cmd.Float("angle"),
	}); err != nil {
		return fmt.Errorf("旋转失败: %w", err)
	}

	fmt.Printf("旋转完成: %s\n", outputPath)
	return nil
}
