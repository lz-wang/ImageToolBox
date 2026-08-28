package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
	"imagetoolbox/internal/crop"
)

func newCropCommand() *cli.Command {
	return &cli.Command{
		Name:  "crop",
		Usage: "按锚点和百分比裁剪图片",
		Description: `按指定锚点和百分比裁剪图片。

宽高仅支持百分比，例如 40%。

示例:
  itb crop -i a.jpg --anchor left --width 40%
  itb crop -i a.jpg --anchor right --width 40%
  itb crop -i a.jpg --anchor top-left --width 40% --height 40%
  itb crop -i a.jpg --anchor center --width 40% --height 40%`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "input",
				Aliases:  []string{"i"},
				Usage:    "输入图片文件路径",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "输出图片文件路径（默认在原文件名后加 _cropped）",
			},
			&cli.StringFlag{
				Name:     "anchor",
				Usage:    "裁剪锚点: left/right/top/bottom/top-left/top-right/bottom-left/bottom-right/center",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "width",
				Usage: "裁剪宽度百分比，例如 40%",
			},
			&cli.StringFlag{
				Name:  "height",
				Usage: "裁剪高度百分比，例如 40%",
			},
		},
		Action: runCrop,
	}
}

func runCrop(ctx context.Context, cmd *cli.Command) error {
	inputFile := cmd.String("input")

	outputPath := cmd.String("output")
	if outputPath == "" {
		ext := filepath.Ext(inputFile)
		base := strings.TrimSuffix(filepath.Base(inputFile), ext)
		dir := filepath.Dir(inputFile)
		outputPath = filepath.Join(dir, base+"_cropped"+ext)
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
