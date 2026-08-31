package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
	"imagetoolbox/internal/convert"
)

func newConvertCommand() *cli.Command {
	return &cli.Command{
		Name:  "convert",
		Usage: "转换图片格式",
		Description: `转换图片格式。

示例:
  itb convert -i photo.png --to webp
  itb convert -i photo.png --to jpg --background "#FFFFFF"
  itb convert -i photo.jpg --to png -o converted.png`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "input",
				Aliases:  []string{"i"},
				Usage:    "输入图片 `FILE`",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "输出图片 `FILE`（默认在原文件名后加 _converted 并按目标格式换扩展名）",
			},
			&cli.StringFlag{
				Name:      "to",
				Usage:     "目标格式 `FORMAT`: jpg/jpeg/png/webp",
				Required:  true,
				Validator: formatValidator("to"),
			},
			&cli.IntFlag{
				Name:      "quality",
				Aliases:   []string{"q"},
				Value:     convert.DefaultQuality,
				Usage:     "输出质量 (1-100)",
				Validator: intRangeValidator("quality", 1, 100),
			},
			&cli.BoolFlag{
				Name:  "lossless",
				Usage: "使用无损编码（webp/png）",
			},
			&cli.StringFlag{
				Name:  "background",
				Value: convert.DefaultBackground,
				Usage: "透明图转不透明格式时的背景色",
			},
		},
		Action: runConvert,
	}
}

func runConvert(ctx context.Context, cmd *cli.Command) error {
	inputFile := cmd.String("input")
	to := cmd.String("to")

	outputPath := cmd.String("output")
	if outputPath == "" {
		var err error
		outputPath, err = convert.DefaultOutputPath(inputFile, to)
		if err != nil {
			return err
		}
	}

	if err := convert.ConvertFile(inputFile, outputPath, convert.Options{
		To:         to,
		Quality:    cmd.Int("quality"),
		Lossless:   cmd.Bool("lossless"),
		Background: cmd.String("background"),
	}); err != nil {
		return fmt.Errorf("转换失败: %w", err)
	}

	fmt.Printf("转换完成: %s\n", outputPath)
	return nil
}
