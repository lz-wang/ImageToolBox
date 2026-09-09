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
		Name:      "crop",
		Usage:     "Crop an image by anchor and percentage",
		Category:  categoryImageTransforms,
		ArgsUsage: "<src> [dst]",
		Description: `Crop a JPEG, PNG, or WebP image by anchor and by
percentage of the source dimensions.

Width and height accept percentages only, in the range
(0,100], e.g. 40%.

DEFAULTS:
  If [dst] is omitted, writes <name>_cropped.<ext>.

CONSTRAINTS:
  --anchor is required.
  left / right require --width and forbid --height.
  top / bottom require --height and forbid --width.
  corners / center require both --width and --height.

EXAMPLES:
  itb crop --anchor left --width 40% a.jpg
  itb crop --anchor right --width 40% a.jpg
  itb crop --anchor top-left --width 40% --height 40% a.jpg
  itb crop --anchor center --width 40% --height 40% a.jpg result.jpg`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "anchor",
				Usage:    "Crop `ANCHOR`: left/right/top/bottom/top-left/top-right/bottom-left/bottom-right/center",
				Required: true,
				Validator: enumValidator("anchor",
					"left", "right", "top", "bottom",
					"top-left", "top-right", "bottom-left", "bottom-right", "center"),
			},
			&cli.StringFlag{
				Name:      "width",
				Usage:     "Crop width as `PERCENT` of the source width, e.g. 40%",
				Validator: percentRangeValidator("width", 100),
			},
			&cli.StringFlag{
				Name:      "height",
				Usage:     "Crop height as `PERCENT` of the source height, e.g. 40%",
				Validator: percentRangeValidator("height", 100),
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return operationError("crop", runCrop(ctx, cmd))
		},
	}
}

func runCrop(ctx context.Context, cmd *cli.Command) error {
	inputFile, outputPath, err := sourceDestinationArgs(cmd, false)
	if err != nil {
		return err
	}
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
