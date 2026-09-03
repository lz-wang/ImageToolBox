package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"
	"imagetoolbox/internal/imageio"
	"imagetoolbox/internal/resize"
)

func newResizeCommand() *cli.Command {
	return &cli.Command{
		Name:      "resize",
		Usage:     "Resize an image",
		Category:  categoryImageTransforms,
		ArgsUsage: "<src> [dst]",
		Description: `Resize a JPEG, PNG, or WebP image.

DEFAULTS:
  If [dst] is omitted, writes <name>_resized.<ext>.

CONSTRAINTS:
  Specify --percent or at least one of --width and --height.
  --percent cannot be combined with --width or --height.
  fit keeps the aspect ratio and allows a single dimension.
  fill requires both --width and --height.
  stretch does not keep the aspect ratio when both dimensions
  are given.

EXAMPLES:
  itb resize --width 1200 photo.jpg
  itb resize --percent 50% photo.jpg half.jpg
  itb resize --width 1200 --height 630 --mode fill --anchor top photo.jpg social.jpg`,
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:      "width",
				Usage:     "Target width in `PIXELS`",
				Validator: positiveIntValidator("width"),
			},
			&cli.IntFlag{
				Name:      "height",
				Usage:     "Target height in `PIXELS`",
				Validator: positiveIntValidator("height"),
			},
			&cli.StringFlag{
				Name:      "percent",
				Usage:     "Scale by `PERCENT`, e.g. 50%",
				Validator: percentRangeValidator("percent", 0),
			},
			&cli.StringFlag{
				Name:      "mode",
				Value:     "fit",
				Usage:     "Resize `MODE`: fit/fill/stretch",
				Validator: enumValidator("mode", "fit", "fill", "stretch"),
			},
			&cli.StringFlag{
				Name:  "anchor",
				Value: "center",
				Usage: "Anchor used by fill `MODE`: left/right/top/bottom/top-left/top-right/bottom-left/bottom-right/center",
				Validator: enumValidator("anchor",
					"left", "right", "top", "bottom",
					"top-left", "top-right", "bottom-left", "bottom-right", "center"),
			},
			&cli.StringFlag{
				Name:      "filter",
				Value:     string(resize.FilterLanczos),
				// 支持列表单一来源于 resize.FilterNames，
				// 与领域层 parseFilter 共享同一份枚举。
				Usage:     "Resampling filter: " + strings.Join(resize.FilterNames(), "/"),
				Validator: enumValidator("filter", resize.FilterNames()...),
			},
		},
		Action: runResize,
	}
}

func runResize(ctx context.Context, cmd *cli.Command) error {
	inputFile, outputPath, err := sourceDestinationArgs(cmd, false)
	if err != nil {
		return err
	}
	if outputPath == "" {
		outputPath = imageio.SuffixedPath(inputFile, "_resized")
	}

	err = resize.ResizeFile(inputFile, outputPath, resize.Options{
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
