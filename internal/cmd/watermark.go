package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
	"imagetoolbox/internal/watermark"
)

func newWatermarkCommand() *cli.Command {
	return &cli.Command{
		Name:  "watermark",
		Usage: "添加文字或图片水印",
		Description: `为图片添加文字或图片水印。

文字水印支持两种模式:
  position（默认）  单点位置水印
  repeat            重复平铺水印

图片水印当前仅支持 position 模式。

示例:
  # 位置水印（默认右下角，智能颜色）
  itb watermark -i photo.jpg -t "Author"

  # 指定位置和透明度
  itb watermark -i photo.png -t "Copyright" --position center --opacity 0.8

  # 重复平铺水印
  itb watermark -i photo.png -t "WATERMARK" --mode repeat

  # 图片水印
  itb watermark -i photo.jpg --image logo.png --scale 0.2

  # 指定输出路径
  itb watermark -i photo.jpg -t "Author" -o output.jpg`,
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
				Usage:   "输出图片 `FILE`（默认在原文件名后加 _watermarked）",
			},
			&cli.StringFlag{
				Name:      "mode",
				Aliases:   []string{"m"},
				Value:     "position",
				Usage:     "水印模式 `MODE`: position（位置）/ repeat（重复平铺）",
				Validator: enumValidator("mode", "position", "repeat"),
			},
			&cli.StringFlag{
				Name:  "color",
				Usage: "文字水印颜色；未指定时自动选择",
			},
			&cli.IntFlag{
				Name:      "space",
				Usage:     "平铺间距（仅文字 repeat 模式；0=自动计算）",
				Validator: nonNegativeIntValidator("space"),
			},
			&cli.IntFlag{
				Name:  "angle",
				Value: 30,
				Usage: "旋转角度，单位为度（仅文字 repeat 模式）",
			},
			&cli.FloatFlag{
				Name:      "opacity",
				Value:     0.5,
				Usage:     "水印透明度，范围 0~1",
				Validator: floatRangeValidator("opacity", 0, 1),
			},
			&cli.StringFlag{
				Name:  "font",
				Usage: "文字水印字体 `FILE`；未指定时自动使用可用的默认字体",
			},
			&cli.IntFlag{
				Name:      "font-size",
				Usage:     "文字水印字号（0=自动计算）",
				Validator: nonNegativeIntValidator("font-size"),
			},
			&cli.StringFlag{
				Name:      "position",
				Value:     "bottom-right",
				Usage:     "水印位置（position 模式）: bottom-right/bottom-left/top-right/top-left/center",
				Validator: enumValidator("position", "bottom-right", "bottom-left", "top-right", "top-left", "center"),
			},
			&cli.FloatFlag{
				Name:  "margin",
				Value: 0.04,
				Usage: "边距比例（position 模式）",
			},
			&cli.FloatFlag{
				Name:      "scale",
				Value:     0.2,
				Usage:     "图片水印尺寸比例，相对底图短边",
				Validator: positiveFloatValidator("scale"),
			},
		},
		// --text 与 --image 二选一且必须提供其一，由框架统一校验
		MutuallyExclusiveFlags: []cli.MutuallyExclusiveFlags{
			{
				Required: true,
				Flags: [][]cli.Flag{
					{
						&cli.StringFlag{
							Name:    "text",
							Aliases: []string{"t"},
							Usage:   "水印文字",
						},
					},
					{
						&cli.StringFlag{
							Name:  "image",
							Usage: "图片水印 `FILE`",
						},
					},
				},
			},
		},
		Action: runWatermark,
	}
}

func runWatermark(ctx context.Context, cmd *cli.Command) error {
	inputFile := cmd.String("input")
	text := cmd.String("text")
	imagePath := cmd.String("image")
	mode := cmd.String("mode")

	// 生成默认输出路径
	outputPath := cmd.String("output")
	if outputPath == "" {
		ext := filepath.Ext(inputFile)
		base := strings.TrimSuffix(filepath.Base(inputFile), ext)
		dir := filepath.Dir(inputFile)
		outputPath = filepath.Join(dir, base+"_watermarked"+ext)
	}

	var err error
	switch {
	case imagePath != "":
		if mode != "position" {
			return fmt.Errorf("图片水印仅支持 position 模式")
		}
		opacity := cmd.Float("opacity")
		scale := cmd.Float("scale")
		margin := cmd.Float("margin")
		opts := &watermark.ImageOptions{
			ImagePath:   imagePath,
			Opacity:     &opacity,
			Position:    watermark.Position(cmd.String("position")),
			ScaleRatio:  &scale,
			MarginRatio: &margin,
		}
		_, err = watermark.AddImageWatermark(inputFile, outputPath, opts)

	case text != "":
		opacity := cmd.Float("opacity")
		color := cmd.String("color")
		space := cmd.Int("space")
		angle := cmd.Int("angle")
		fontSize := cmd.Int("font-size")
		margin := cmd.Float("margin")
		switch mode {
		case "repeat":
			opts := &watermark.RepeatOptions{
				Color:          &color,
				Space:          &space,
				Angle:          &angle,
				Opacity:        &opacity,
				FontPath:       cmd.String("font"),
				FontSize:       &fontSize,
				FontHeightCrop: nil,
			}
			_, err = watermark.AddRepeatWatermark(inputFile, outputPath, text, opts)

		case "position":
			opts := &watermark.PositionOptions{
				Opacity:     &opacity,
				Position:    watermark.Position(cmd.String("position")),
				FontPath:    cmd.String("font"),
				FontSize:    &fontSize,
				Color:       &color,
				MarginRatio: &margin,
			}
			_, err = watermark.AddPositionWatermark(inputFile, outputPath, text, opts)

		default:
			return fmt.Errorf("不支持的水印模式: %s（支持: position, repeat）", mode)
		}
	default:
		return fmt.Errorf("必须指定水印文字 (-t) 或图片水印 (--image)")
	}

	if err != nil {
		return fmt.Errorf("添加水印失败: %w", err)
	}

	fmt.Printf("水印添加完成: %s\n", outputPath)
	return nil
}
