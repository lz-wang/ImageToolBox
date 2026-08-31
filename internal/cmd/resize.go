package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
	"imagetoolbox/internal/imageio"
	"imagetoolbox/internal/resize"
)

func newResizeCommand() *cli.Command {
	return &cli.Command{
		Name:  "resize",
		Usage: "调整图片尺寸",
		Description: `调整图片尺寸。

尺寸规则:
  - 必须指定 --percent，或至少指定 --width / --height 之一
  - --percent 不能与 --width / --height 同时使用
  - fit 支持仅指定宽度或高度，并保持宽高比
  - fill 必须同时指定宽度和高度
  - stretch 同时指定宽高时不保持原始宽高比

示例:
  itb resize -i photo.jpg --width 1200
  itb resize -i photo.png --height 800
  itb resize -i photo.jpg --percent 50%
  itb resize -i photo.jpg --width 1200 --height 630 --mode fill --anchor top`,
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
				Usage:   "输出图片 `FILE`（默认在原文件名后加 _resized）",
			},
			&cli.IntFlag{
				Name:      "width",
				Usage:     "目标宽度（像素）",
				Validator: positiveIntValidator("width"),
			},
			&cli.IntFlag{
				Name:      "height",
				Usage:     "目标高度（像素）",
				Validator: positiveIntValidator("height"),
			},
			&cli.StringFlag{
				Name:      "percent",
				Usage:     "按比例缩放，例如 50%",
				Validator: percentRangeValidator("percent", 0),
			},
			&cli.StringFlag{
				Name:      "mode",
				Value:     "fit",
				Usage:     "缩放模式 `MODE`: fit/fill/stretch",
				Validator: enumValidator("mode", "fit", "fill", "stretch"),
			},
			&cli.StringFlag{
				Name:  "anchor",
				Value: "center",
				Usage: "填充模式锚点（fill 模式）: left/right/top/bottom/top-left/top-right/bottom-left/bottom-right/center",
				Validator: enumValidator("anchor",
					"left", "right", "top", "bottom",
					"top-left", "top-right", "bottom-left", "bottom-right", "center"),
			},
			&cli.StringFlag{
				Name:      "filter",
				Value:     "lanczos",
				Usage:     "采样器: nearest/linear/catmullrom/lanczos",
				Validator: enumValidator("filter", "nearest", "linear", "catmullrom", "lanczos"),
			},
		},
		Action: runResize,
	}
}

func runResize(ctx context.Context, cmd *cli.Command) error {
	inputFile := cmd.String("input")

	outputPath := cmd.String("output")
	if outputPath == "" {
		outputPath = imageio.SuffixedPath(inputFile, "_resized")
	}

	err := resize.ResizeFile(inputFile, outputPath, resize.Options{
		Width:   cmd.Int("width"),
		Height:  cmd.Int("height"),
		Percent: cmd.String("percent"),
		Mode:    resize.Mode(cmd.String("mode")),
		Anchor:  cmd.String("anchor"),
		Filter:  cmd.String("filter"),
	})
	if err != nil {
		return fmt.Errorf("调整尺寸失败: %w", err)
	}

	fmt.Printf("调整尺寸完成: %s\n", outputPath)
	return nil
}
