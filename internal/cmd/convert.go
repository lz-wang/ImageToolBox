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
		Usage:     "Convert an image to another format",
		Category:  categoryImageTransforms,
		ArgsUsage: "<src> <dst>",
		Description: `Convert a JPEG, PNG, or WebP image to another
format.

The output format is determined only by the <dst> file
extension: .jpg / .jpeg / .png / .webp.

CONSTRAINTS:
  <dst> is required; there is no derived output path.
  PNG output is always lossless; --quality and --lossless
  have no effect on PNG.
  --lossless switches WebP to lossless encoding, where
  --quality controls compression effort instead.
  Transparent areas are flattened onto --background when the
  output is JPEG; the background must be an opaque color.

EXAMPLES:
  itb convert photo.png photo.webp
  itb convert photo.png photo.jpg --background "#FFFFFF"
  itb convert -q 85 photo.jpg converted.png`,
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:      "quality",
				Aliases:   []string{"q"},
				Value:     convert.DefaultQuality,
				Usage:     "JPEG/WebP output quality (1-100); compression effort for lossless WebP; ignored for PNG",
				Validator: intRangeValidator("quality", 1, 100),
			},
			&cli.BoolFlag{
				Name:  "lossless",
				Usage: "Use lossless WebP encoding; PNG is always lossless, so this has no extra effect on PNG",
			},
			&cli.StringFlag{
				Name:  "background",
				Value: convert.DefaultBackground,
				Usage: "Background `COLOR` (#RGB/#RRGGBB/#RRGGBBAA) for transparent areas when writing JPEG; must be opaque",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return operationError("convert", runConvert(ctx, cmd))
		},
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
