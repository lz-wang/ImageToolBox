package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
	"imagetoolbox/internal/imageio"
	"imagetoolbox/internal/watermark"
)

func newWatermarkCommand() *cli.Command {
	return &cli.Command{
		Name:      "watermark",
		Usage:     "Add a text or image watermark",
		Category:  categoryImageTransforms,
		ArgsUsage: "<src> [dst]",
		Description: `Add a text or image watermark to a JPEG, PNG, or
WebP image.

DEFAULTS:
  If [dst] is omitted, writes <name>_watermarked.<ext>.
  The text watermark color and font size are auto-selected
  from the underlying image when not specified.

CONSTRAINTS:
  Exactly one of --text or --image is required.
  Text watermarks support position and repeat modes.
  Image watermarks support position mode only; tiled image
  watermarks are not supported.

EXAMPLES:
  # Position watermark (default: bottom-right, auto color)
  itb watermark -t "Author" photo.jpg

  # Explicit position and opacity
  itb watermark -t "Copyright" --position center --opacity 0.8 photo.png

  # Repeated tiled watermark
  itb watermark -t "WATERMARK" --mode repeat photo.png

  # Image watermark
  itb watermark --image logo.png --scale 0.2 photo.jpg

  # Explicit output path
  itb watermark -t "Author" photo.jpg output.jpg`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:      "mode",
				Aliases:   []string{"m"},
				Value:     "position",
				Usage:     "Watermark `MODE`: position (single placement) / repeat (tiled)",
				Validator: enumValidator("mode", "position", "repeat"),
			},
			&cli.StringFlag{
				Name:        "color",
				Usage:       "Text watermark `COLOR` (#RGB/#RRGGBB/#RRGGBBAA)",
				DefaultText: "auto",
				Validator:   colorValidator("color"),
			},
			&cli.IntFlag{
				Name:        "space",
				Usage:       "Tile spacing in `PIXELS` (text repeat mode only)",
				DefaultText: "auto",
				Validator:   nonNegativeIntValidator("space"),
			},
			&cli.IntFlag{
				Name:      "angle",
				Value:     30,
				Usage:     "Tile rotation `ANGLE` in degrees (text repeat mode only)",
				Validator: intRangeValidator("angle", -360, 360),
			},
			&cli.FloatFlag{
				Name:      "opacity",
				Value:     0.5,
				Usage:     "Watermark opacity, range [0,1]",
				Validator: floatRangeValidator("opacity", 0, 1),
			},
			&cli.StringFlag{
				Name:        "font",
				Usage:       "Text watermark font `FILE`",
				DefaultText: "auto",
			},
			&cli.IntFlag{
				Name:        "font-size",
				Usage:       "Text watermark font size in `PIXELS`",
				DefaultText: "auto",
				Validator:   intRangeValidator("font-size", 0, watermark.MaxFontSize),
			},
			&cli.StringFlag{
				Name:      "position",
				Value:     "bottom-right",
				Usage:     "Watermark `POSITION` (position mode): bottom-right/bottom-left/top-right/top-left/center",
				Validator: enumValidator("position", "bottom-right", "bottom-left", "top-right", "top-left", "center"),
			},
			&cli.FloatFlag{
				Name:      "margin",
				Value:     0.04,
				Usage:     "Margin as a fraction of the source image's shorter side (position mode)",
				Validator: nonNegativeFloatValidator("margin"),
			},
			&cli.FloatFlag{
				Name:      "scale",
				Value:     0.2,
				Usage:     "Image watermark size relative to the source image's shorter side",
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
							Usage:   "Watermark text",
						},
					},
					{
						&cli.StringFlag{
							Name:  "image",
							Usage: "Image watermark `FILE`",
						},
					},
				},
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return operationError("watermark", runWatermark(ctx, cmd))
		},
	}
}

func runWatermark(ctx context.Context, cmd *cli.Command) error {
	inputFile, outputPath, err := sourceDestinationArgs(cmd, false)
	if err != nil {
		return err
	}

	// 生成默认输出路径
	if outputPath == "" {
		outputPath = imageio.SuffixedPath(inputFile, "_watermarked")
	}

	opacity := cmd.Float("opacity")
	space := cmd.Int("space")
	angle := cmd.Int("angle")
	fontSize := cmd.Int("font-size")
	margin := cmd.Float("margin")
	scale := cmd.Float("scale")
	err = watermark.AddFile(inputFile, outputPath, watermark.Options{
		Text:      cmd.String("text"),
		ImagePath: cmd.String("image"),
		Mode:      watermark.Mode(cmd.String("mode")),
		Position:  watermark.Position(cmd.String("position")),
		Color:     cmd.String("color"),
		FontPath:  cmd.String("font"),
		Opacity:   &opacity,
		FontSize:  &fontSize,
		Space:     &space,
		Angle:     &angle,
		Margin:    &margin,
		Scale:     &scale,
	})

	if err != nil {
		return fmt.Errorf("添加水印失败: %w", err)
	}

	fmt.Printf("水印添加完成: %s\n", outputPath)
	return nil
}
