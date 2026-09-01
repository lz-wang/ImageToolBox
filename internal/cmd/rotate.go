package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
	"imagetoolbox/internal/imageio"
	"imagetoolbox/internal/rotate"
)

func newRotateCommand() *cli.Command {
	return &cli.Command{
		Name:      "rotate",
		Usage:     "Rotate an image",
		Category:  categoryImageTransforms,
		ArgsUsage: "<src> [dst]",
		Description: `Rotate a JPEG, PNG, or WebP image by an arbitrary
angle.

DEFAULTS:
  If [dst] is omitted, writes <name>_rotated.<ext>.

CONSTRAINTS:
  Positive angles rotate counter-clockwise; negative angles
  rotate clockwise. Decimal angles are supported.
  --angle is required, must be in (-360, 360), and cannot be 0.
  Arbitrary angles resize the output canvas to the rotated
  bounding box; uncovered pixels stay transparent before
  encoding, and JPEG output flattens them onto white.
  Exact 90/180/270 rotations are interpolation-free; other
  angles use bilinear interpolation.
  JPEG EXIF Orientation is normalized before rotating.

EXAMPLES:
  itb rotate --angle 90 photo.jpg
  itb rotate --angle -90 photo.jpg clockwise.jpg
  itb rotate --angle 45 photo.png rotated.png
  itb rotate --angle 22.5 photo.webp rotated.webp`,
		Flags: []cli.Flag{
			&cli.FloatFlag{
				Name:      "angle",
				Usage:     "Rotation `ANGLE` in degrees: positive = counter-clockwise, negative = clockwise; range (-360, 360), cannot be 0",
				Required:  true,
				Validator: nonZeroOpenRangeFloatValidator("angle", -360, 360),
			},
		},
		Action: runRotate,
	}
}

func runRotate(ctx context.Context, cmd *cli.Command) error {
	inputFile, outputPath, err := sourceDestinationArgs(cmd, false)
	if err != nil {
		return err
	}
	if outputPath == "" {
		outputPath = imageio.SuffixedPath(inputFile, "_rotated")
	}

	if err := rotate.RotateFile(inputFile, outputPath, rotate.Options{
		Angle: cmd.Float("angle"),
	}); err != nil {
		return fmt.Errorf("旋转失败: %w", err)
	}

	fmt.Printf("旋转完成: %s\n", outputPath)
	return nil
}
