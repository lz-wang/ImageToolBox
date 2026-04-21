package crop

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"imagetoolbox/internal/watermark"
)

type Anchor string

const (
	AnchorLeft        Anchor = "left"
	AnchorRight       Anchor = "right"
	AnchorTop         Anchor = "top"
	AnchorBottom      Anchor = "bottom"
	AnchorTopLeft     Anchor = "top-left"
	AnchorTopRight    Anchor = "top-right"
	AnchorBottomLeft  Anchor = "bottom-left"
	AnchorBottomRight Anchor = "bottom-right"
	AnchorCenter      Anchor = "center"
)

type Options struct {
	Anchor Anchor
	Width  string
	Height string
}

func CropFile(inputPath, outputPath string, opts Options) (image.Rectangle, error) {
	img, err := imaging.Open(inputPath)
	if err != nil {
		return image.Rectangle{}, fmt.Errorf("打开输入图片失败: %w", err)
	}

	rect, err := ComputeRect(img.Bounds().Dx(), img.Bounds().Dy(), opts)
	if err != nil {
		return image.Rectangle{}, err
	}

	cropped := imaging.Crop(img, rect)
	if err := watermark.SaveImage(cropped, outputPath, color.NRGBA{255, 255, 255, 255}); err != nil {
		return image.Rectangle{}, fmt.Errorf("保存裁剪结果失败: %w", err)
	}

	return rect, nil
}

func ValidateAndComputeRect(inputPath string, opts Options) (image.Rectangle, error) {
	img, err := imaging.Open(inputPath)
	if err != nil {
		return image.Rectangle{}, fmt.Errorf("打开输入图片失败: %w", err)
	}

	return ComputeRect(img.Bounds().Dx(), img.Bounds().Dy(), opts)
}

func ComputeRect(imgWidth, imgHeight int, opts Options) (image.Rectangle, error) {
	if imgWidth <= 0 || imgHeight <= 0 {
		return image.Rectangle{}, fmt.Errorf("图片尺寸无效: %dx%d", imgWidth, imgHeight)
	}

	widthPct, heightPct, err := validateOptions(opts)
	if err != nil {
		return image.Rectangle{}, err
	}

	fullWidth := 100.0
	fullHeight := 100.0

	if widthPct == nil {
		widthPct = &fullWidth
	}
	if heightPct == nil {
		heightPct = &fullHeight
	}

	cropWidth := percentToPixels(imgWidth, *widthPct)
	cropHeight := percentToPixels(imgHeight, *heightPct)

	switch opts.Anchor {
	case AnchorLeft:
		return image.Rect(0, 0, cropWidth, imgHeight), nil
	case AnchorRight:
		return image.Rect(imgWidth-cropWidth, 0, imgWidth, imgHeight), nil
	case AnchorTop:
		return image.Rect(0, 0, imgWidth, cropHeight), nil
	case AnchorBottom:
		return image.Rect(0, imgHeight-cropHeight, imgWidth, imgHeight), nil
	case AnchorTopLeft:
		return image.Rect(0, 0, cropWidth, cropHeight), nil
	case AnchorTopRight:
		return image.Rect(imgWidth-cropWidth, 0, imgWidth, cropHeight), nil
	case AnchorBottomLeft:
		return image.Rect(0, imgHeight-cropHeight, cropWidth, imgHeight), nil
	case AnchorBottomRight:
		return image.Rect(imgWidth-cropWidth, imgHeight-cropHeight, imgWidth, imgHeight), nil
	case AnchorCenter:
		x0 := (imgWidth - cropWidth) / 2
		y0 := (imgHeight - cropHeight) / 2
		return image.Rect(x0, y0, x0+cropWidth, y0+cropHeight), nil
	default:
		return image.Rectangle{}, fmt.Errorf("不支持的 anchor: %s", opts.Anchor)
	}
}

func validateOptions(opts Options) (*float64, *float64, error) {
	if opts.Anchor == "" {
		return nil, nil, fmt.Errorf("必须指定 --anchor")
	}

	var (
		widthPct  *float64
		heightPct *float64
		err       error
	)

	if opts.Width != "" {
		parsed, parseErr := parsePercent(opts.Width)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("--width 无效: %w", parseErr)
		}
		widthPct = &parsed
	}

	if opts.Height != "" {
		parsed, parseErr := parsePercent(opts.Height)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("--height 无效: %w", parseErr)
		}
		heightPct = &parsed
	}

	switch opts.Anchor {
	case AnchorLeft, AnchorRight:
		if widthPct == nil || heightPct != nil {
			err = fmt.Errorf("%s 裁剪必须提供 --width，且不能提供 --height", opts.Anchor)
		}
	case AnchorTop, AnchorBottom:
		if heightPct == nil || widthPct != nil {
			err = fmt.Errorf("%s 裁剪必须提供 --height，且不能提供 --width", opts.Anchor)
		}
	case AnchorTopLeft, AnchorTopRight, AnchorBottomLeft, AnchorBottomRight, AnchorCenter:
		if widthPct == nil || heightPct == nil {
			err = fmt.Errorf("%s 裁剪必须同时提供 --width 和 --height", opts.Anchor)
		}
	default:
		err = fmt.Errorf("不支持的 anchor: %s", opts.Anchor)
	}

	if err != nil {
		return nil, nil, err
	}

	return widthPct, heightPct, nil
}

func parsePercent(value string) (float64, error) {
	if !strings.HasSuffix(value, "%") {
		return 0, fmt.Errorf("仅支持百分比格式，例如 40%%")
	}

	numberPart := strings.TrimSuffix(value, "%")
	parsed, err := strconv.ParseFloat(numberPart, 64)
	if err != nil {
		return 0, fmt.Errorf("无法解析百分比: %s", value)
	}
	if parsed <= 0 || parsed > 100 {
		return 0, fmt.Errorf("百分比必须在 (0,100] 范围内: %s", value)
	}
	return parsed, nil
}

func percentToPixels(total int, percent float64) int {
	pixels := int(math.Round(float64(total) * percent / 100))
	if pixels < 1 {
		return 1
	}
	if pixels > total {
		return total
	}
	return pixels
}
