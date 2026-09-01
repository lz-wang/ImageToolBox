package resize

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"imagetoolbox/internal/imageio"
)

type Mode string

const (
	ModeFit     Mode = "fit"
	ModeFill    Mode = "fill"
	ModeStretch Mode = "stretch"
)

type Options struct {
	Width   int
	Height  int
	Percent string
	Mode    Mode
	Anchor  string
	Filter  string
}

func ResizeFile(inputPath, outputPath string, opts Options) error {
	if err := imageio.RejectSameFile(inputPath, outputPath); err != nil {
		return err
	}

	// 输入统一走 imageio.OpenStatic：仅 JPEG/PNG/WebP，JPEG EXIF
	// Orientation 烘焙进像素，解码尺寸与 Probe 的逻辑尺寸一致
	img, err := imageio.OpenStatic(inputPath)
	if err != nil {
		return err
	}

	resized, err := Apply(img, opts)
	if err != nil {
		return err
	}

	return imageio.Save(outputPath, resized, imageio.SaveOptions{
		Quality:    100,
		Background: imageioMustColor("#FFFFFF"),
	})
}

func Apply(img image.Image, opts Options) (image.Image, error) {
	plan, err := Resolve(img.Bounds(), opts)
	if err != nil {
		return nil, err
	}

	if opts.Percent != "" {
		// percent 语义是按百分比精确缩放（允许放大，如 200%），不能走
		// imaging.Fit 的"只缩小不放大"包围盒逻辑。
		return imaging.Resize(img, plan.Width, plan.Height, plan.filter), nil
	}

	switch plan.mode {
	case ModeFit:
		if plan.boxWidth == 0 || plan.boxHeight == 0 {
			return imaging.Resize(img, plan.boxWidth, plan.boxHeight, plan.filter), nil
		}
		if plan.Width == img.Bounds().Dx() && plan.Height == img.Bounds().Dy() {
			return imaging.Clone(img), nil
		}
		return imaging.Resize(img, plan.Width, plan.Height, plan.filter), nil
	case ModeFill:
		return imaging.Fill(img, plan.boxWidth, plan.boxHeight, plan.anchor, plan.filter), nil
	case ModeStretch:
		return imaging.Resize(img, plan.boxWidth, plan.boxHeight, plan.filter), nil
	default:
		return nil, fmt.Errorf("unsupported resize mode: %s", plan.mode)
	}
}

// Plan 描述一次 resize 的完整执行参数。Width/Height 是推导后的真实输出
// 尺寸（fit 单边指定时按宽高比补全另一边），供调用方做资源准入检查；
// boxWidth/boxHeight 保留调用方传入的目标框，Apply 据此分派 imaging，
// 与历史行为逐像素一致。
type Plan struct {
	Width  int
	Height int

	boxWidth  int
	boxHeight int
	mode      Mode
	anchor    imaging.Anchor
	filter    imaging.ResampleFilter
}

// Resolve 从源图尺寸推导真实输出尺寸与执行参数。Apply 内部调用同一
// 函数，因此对 Resolve 结果做限制检查等价于对最终输出做限制检查，
// 且不会复制 percent / fit / fill 的推导语义。
func Resolve(bounds image.Rectangle, opts Options) (Plan, error) {
	mode := opts.Mode
	if mode == "" {
		mode = ModeFit
	}

	filter, err := parseFilter(opts.Filter)
	if err != nil {
		return Plan{}, err
	}
	anchor, err := parseAnchor(opts.Anchor)
	if err != nil {
		return Plan{}, err
	}

	if opts.Percent != "" && (opts.Width > 0 || opts.Height > 0) {
		return Plan{}, fmt.Errorf("--percent cannot be used together with --width or --height")
	}

	if opts.Percent == "" && opts.Width <= 0 && opts.Height <= 0 {
		return Plan{}, fmt.Errorf("must provide --percent or at least one of --width/--height")
	}

	width := opts.Width
	height := opts.Height
	if opts.Percent != "" {
		percent, err := parsePercent(opts.Percent)
		if err != nil {
			return Plan{}, err
		}
		width = max(1, int(math.Round(float64(bounds.Dx())*percent/100)))
		height = max(1, int(math.Round(float64(bounds.Dy())*percent/100)))
	}

	resolvedWidth, resolvedHeight := width, height
	switch mode {
	case ModeFit, ModeStretch:
		if width <= 0 && height <= 0 {
			return Plan{}, fmt.Errorf("resize target size is invalid")
		}
		if width <= 0 {
			resolvedWidth = derivedDimension(bounds.Dx(), bounds.Dy(), height)
		} else if height <= 0 {
			resolvedHeight = derivedDimension(bounds.Dy(), bounds.Dx(), width)
		} else if mode == ModeFit && opts.Percent == "" {
			// fit 的 box 只是包围盒：imaging.Fit 只缩小不放大，真实输出
			// 可能小于 box（源图已在 box 内时等于源图尺寸）。percent 的
			// 目标是精确尺寸，不经过包围盒推导。
			resolvedWidth, resolvedHeight = fitResolved(bounds.Dx(), bounds.Dy(), width, height)
		}
	case ModeFill:
		if width <= 0 || height <= 0 {
			return Plan{}, fmt.Errorf("--mode fill requires both --width and --height")
		}
	default:
		return Plan{}, fmt.Errorf("unsupported resize mode: %s", mode)
	}

	return Plan{
		Width:     resolvedWidth,
		Height:    resolvedHeight,
		boxWidth:  width,
		boxHeight: height,
		mode:      mode,
		anchor:    anchor,
		filter:    filter,
	}, nil
}

// derivedDimension 按宽高比从已知的另一边推导目标尺寸，舍入规则与
// imaging.Resize 保持一致（floor(x+0.5)，最小为 1）。
func derivedDimension(srcSide, srcOther, dstOther int) int {
	return max(1, int(math.Floor(float64(srcSide)*float64(dstOther)/float64(srcOther)+0.5)))
}

// fitResolved 镜像 imaging.Fit 的真实输出推导：源图已在包围盒内时输出
// 等于源图尺寸（Fit 直接 Clone）；否则按宽高比贴边计算。单边被截断为 0
// 时与 Fit 内部调用的 Resize 一样按宽高比补全（最小 1）。
func fitResolved(srcW, srcH, boxW, boxH int) (int, int) {
	if srcW <= boxW && srcH <= boxH {
		return srcW, srcH
	}
	srcAspectRatio := float64(srcW) / float64(srcH)
	maxAspectRatio := float64(boxW) / float64(boxH)
	newW, newH := boxW, boxH
	if srcAspectRatio > maxAspectRatio {
		newW = boxW
		newH = int(float64(newW) / srcAspectRatio)
	} else {
		newH = boxH
		newW = int(float64(newH) * srcAspectRatio)
	}
	if newW <= 0 {
		newW = derivedDimension(srcW, srcH, newH)
	}
	if newH <= 0 {
		newH = derivedDimension(srcH, srcW, newW)
	}
	return newW, newH
}

func parsePercent(value string) (float64, error) {
	if !strings.HasSuffix(value, "%") {
		return 0, fmt.Errorf("percent must use %% suffix, for example 50%%")
	}
	numberPart := strings.TrimSuffix(value, "%")
	parsed, err := strconv.ParseFloat(numberPart, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid percent: %s", value)
	}
	if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, fmt.Errorf("percent must be a finite number: %s", value)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("percent must be greater than 0: %s", value)
	}
	return parsed, nil
}

func parseFilter(value string) (imaging.ResampleFilter, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "lanczos":
		return imaging.Lanczos, nil
	case "nearest":
		return imaging.NearestNeighbor, nil
	case "linear":
		return imaging.Linear, nil
	case "catmullrom":
		return imaging.CatmullRom, nil
	default:
		return imaging.Lanczos, fmt.Errorf("unsupported filter: %s", value)
	}
}

func parseAnchor(value string) (imaging.Anchor, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "center":
		return imaging.Center, nil
	case "left":
		return imaging.Left, nil
	case "right":
		return imaging.Right, nil
	case "top":
		return imaging.Top, nil
	case "bottom":
		return imaging.Bottom, nil
	case "top-left":
		return imaging.TopLeft, nil
	case "top-right":
		return imaging.TopRight, nil
	case "bottom-left":
		return imaging.BottomLeft, nil
	case "bottom-right":
		return imaging.BottomRight, nil
	default:
		return imaging.Center, fmt.Errorf("unsupported anchor: %s", value)
	}
}

func imageioMustColor(hex string) color.NRGBA {
	col, err := imageio.ParseHexColor(hex)
	if err != nil {
		panic(err)
	}
	return col
}
