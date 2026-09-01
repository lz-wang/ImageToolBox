package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
	"imagetoolbox/internal/convert"
)

func newConvertCommand() *cli.Command {
	return &cli.Command{
		Name:      "convert",
		Usage:     "转换图片格式",
		ArgsUsage: "<src> <dst>",
		Description: `转换图片格式。
目标格式由 <dst> 文件扩展名决定。
支持: .jpg / .jpeg / .png / .webp

示例:
	  itb convert photo.png photo.webp
	  itb convert photo.png photo.jpg --background "#FFFFFF"
	  itb convert -q 85 photo.jpg converted.png`,
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:      "quality",
				Aliases:   []string{"q"},
				Value:     convert.DefaultQuality,
				Usage:     "JPEG/WebP 输出质量 (1-100)；WebP 无损模式下表示压缩强度，PNG 忽略该参数",
				Validator: intRangeValidator("quality", 1, 100),
			},
			&cli.BoolFlag{
				Name:  "lossless",
				Usage: "使用 WebP 无损编码；PNG 始终为无损格式，该参数对 PNG 无额外影响",
			},
			&cli.StringFlag{
				Name:  "background",
				Value: convert.DefaultBackground,
				Usage: "输出 JPEG 时透明区域使用的背景色（必须为不透明颜色）",
			},
		},
		Action: runConvert,
	}
}

func runConvert(ctx context.Context, cmd *cli.Command) error {
	inputFile, outputPath, err := sourceDestinationArgs(cmd, true)
	if err != nil {
		return err
	}

	if err := convert.ConvertFile(inputFile, outputPath, convert.Options{
		Quality:    cmd.Int("quality"),
		Lossless:   cmd.Bool("lossless"),
		Background: cmd.String("background"),
	}); err != nil {
		return fmt.Errorf("转换失败: %w", err)
	}

	fmt.Printf("转换完成: %s\n", outputPath)
	return nil
}
