package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
	"imagetoolbox/internal/crop"
	"imagetoolbox/internal/imageio"
)

func newCropCommand() *cli.Command {
	return &cli.Command{
		Name:  "crop",
		Usage: "按锚点和百分比裁剪图片",
		Description: `按指定锚点和百分比裁剪图片。

宽高仅支持百分比，范围为 (0,100]，例如 40%。

规则:
  - left / right      必须提供 --width，且不能提供 --height
  - top / bottom      必须提供 --height，且不能提供 --width
  - 角点 / center     必须同时提供 --width 和 --height

示例:
  itb crop -i a.jpg --anchor left --width 40%
  itb crop -i a.jpg --anchor right --width 40%
  itb crop -i a.jpg --anchor top-left --width 40% --height 40%
  itb crop -i a.jpg --anchor center --width 40% --height 40%`,
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
				Usage:   "输出图片 `FILE`（默认在原文件名后加 _cropped）",
			},
			&cli.StringFlag{
				Name:     "anchor",
				Usage:    "裁剪锚点: left/right/top/bottom/top-left/top-right/bottom-left/bottom-right/center",
				Required: true,
				Validator: enumValidator("anchor",
					"left", "right", "top", "bottom",
					"top-left", "top-right", "bottom-left", "bottom-right", "center"),
			},
			&cli.StringFlag{
				Name:      "width",
				Usage:     "裁剪宽度百分比，例如 40%",
				Validator: percentRangeValidator("width", 100),
			},
			&cli.StringFlag{
				Name:      "height",
				Usage:     "裁剪高度百分比，例如 40%",
				Validator: percentRangeValidator("height", 100),
			},
		},
		Action: runCrop,
	}
}

func runCrop(ctx context.Context, cmd *cli.Command) error {
	inputFile := cmd.String("input")

	outputPath := cmd.String("output")
	if outputPath == "" {
		outputPath = imageio.SuffixedPath(inputFile, "_cropped")
	}

	rect, err := crop.CropFile(inputFile, outputPath, crop.Options{
		Anchor: crop.Anchor(cmd.String("anchor")),
		Width:  cmd.String("width"),
		Height: cmd.String("height"),
	})
	if err != nil {
		return fmt.Errorf("裁剪失败: %w", err)
	}

	fmt.Printf("裁剪完成: %s (%dx%d)\n", outputPath, rect.Dx(), rect.Dy())
	return nil
}
