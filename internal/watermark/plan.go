package watermark

import (
	"errors"
	"fmt"
	"image"
	"math"
	"strings"
	"unicode/utf8"
)

// 领域默认值。AddImageWatermark / AddRepeatWatermark 与资源规划共用，
// 保证 admission 推导与真实执行一致。
const (
	DefaultOpacity      = 0.5
	DefaultImageScale   = 0.2
	DefaultMarginRatio  = 0.04
	DefaultRepeatAngle  = 30
	DefaultPositionMode = BottomRight
)

// ImagePlan 描述图片水印经 scale 缩放后的 logo 目标尺寸。
type ImagePlan struct {
	TargetWidth  int
	TargetHeight int
}

// ResolveImagePlan 从底图与 logo 尺寸推导缩放后 logo 的目标尺寸。
// AddImageWatermark 实际执行使用同一推导（scaledLogoSize），对计划做
// 限制检查等价于对真实分配做限制检查。
func ResolveImagePlan(baseBounds, logoBounds image.Rectangle, opts Options) (ImagePlan, error) {
	if baseBounds.Dx() <= 0 || baseBounds.Dy() <= 0 {
		return ImagePlan{}, fmt.Errorf("invalid base image bounds: %v", baseBounds)
	}
	if logoBounds.Dx() <= 0 || logoBounds.Dy() <= 0 {
		return ImagePlan{}, fmt.Errorf("invalid watermark image bounds: %v", logoBounds)
	}
	scaleRatio := DefaultImageScale
	if opts.Scale != nil {
		scaleRatio = *opts.Scale
	}
	width, height := scaledLogoSize(baseBounds.Dx(), baseBounds.Dy(), logoBounds.Dx(), logoBounds.Dy(), scaleRatio)
	return ImagePlan{TargetWidth: width, TargetHeight: height}, nil
}

// scaledLogoSize 按"底图短边 × scaleRatio"推导缩放后 logo 尺寸。
func scaledLogoSize(baseW, baseH, logoW, logoH int, scaleRatio float64) (int, int) {
	targetShort := max(1, int(math.Round(float64(min(baseW, baseH))*scaleRatio)))
	logoShort := min(logoW, logoH)
	scale := float64(targetShort) / float64(logoShort)
	targetW := max(1, int(math.Round(float64(logoW)*scale)))
	targetH := max(1, int(math.Round(float64(logoH)*scale)))
	return targetW, targetH
}

// WorkingSet 描述一次水印操作中间画布的尺寸保守上界，供 HTTP 层做
// working-set 内存准入。不追求字节级精确，只保证不低估。
type WorkingSet struct {
	// Output 是底图/输出尺寸：解码图、clone、结果与保存 flatten 都不超过它。
	OutputWidth  int
	OutputHeight int
	// Mark 是文字 repeat 模式的 mark 临时画布，或图片水印缩放后 logo 的
	// 尺寸上界；position 文字模式为 0（直接绘制，不分配整幅画布）。
	MarkWidth  int
	MarkHeight int
	// Tile 是 repeat 模式平铺画布旋转后的尺寸上界；非 repeat 模式为 0。
	TileWidth  int
	TileHeight int
}

// Bytes 返回中间画布内存的保守上界（RGBA，4 字节/像素）。
func (w WorkingSet) Bytes() int64 {
	const bytesPerPixel = 4
	px := func(width, height int) int64 {
		if width <= 0 || height <= 0 {
			return 0
		}
		return int64(width) * int64(height)
	}
	// base 维度最多同时存在 4 份（解码图、clone、结果、保存 flatten）；
	// mark 与 tile 各按 2 份估算（生成/缩放 + opacity clone；平铺 + rotate）。
	total := px(w.OutputWidth, w.OutputHeight)*4 + px(w.MarkWidth, w.MarkHeight)*2 + px(w.TileWidth, w.TileHeight)*2
	return total * bytesPerPixel
}

// ResolveWorkingSet 计算水印操作的中间画布上界。baseBounds 必填；
// 图片水印必须提供 logoBounds。opts 应已通过 Validate。
func ResolveWorkingSet(baseBounds, logoBounds image.Rectangle, opts Options) (WorkingSet, error) {
	if baseBounds.Dx() <= 0 || baseBounds.Dy() <= 0 {
		return WorkingSet{}, fmt.Errorf("invalid base image bounds: %v", baseBounds)
	}
	hasText := strings.TrimSpace(opts.Text) != ""
	hasImage := strings.TrimSpace(opts.ImagePath) != ""
	if hasText == hasImage {
		return WorkingSet{}, errors.New("must provide exactly one of text or image watermark")
	}
	set := WorkingSet{OutputWidth: baseBounds.Dx(), OutputHeight: baseBounds.Dy()}

	if hasImage {
		plan, err := ResolveImagePlan(baseBounds, logoBounds, opts)
		if err != nil {
			return WorkingSet{}, err
		}
		set.MarkWidth, set.MarkHeight = plan.TargetWidth, plan.TargetHeight
		return set, nil
	}

	mode := opts.Mode
	if mode == "" {
		mode = ModePosition
	}
	if mode == ModeRepeat {
		fontSize := effectiveFontSize(opts.FontSize, baseBounds.Dx(), baseBounds.Dy())
		canvas := markCanvasSize(fontSize, opts.Text)
		set.MarkWidth, set.MarkHeight = canvas.X, canvas.Y
		// 平铺画布边长与 Watermarker.Apply 使用同一公式；mark 经裁剪与
		// 缩放只会更小，以画布尺寸为上界。旋转任意角度的包围盒按 √2 放大。
		c := int(math.Hypot(float64(baseBounds.Dx()), float64(baseBounds.Dy()))) + max(canvas.X, canvas.Y)*2
		side := int(math.Ceil(float64(c) * math.Sqrt2))
		set.TileWidth, set.TileHeight = side, side
	}
	return set, nil
}

// effectiveFontSize 与 AddRepeatWatermark/AddPositionWatermark 的自动
// 字号公式一致（0 或未指定 = 按底图短边推导）。
func effectiveFontSize(fontSize *int, baseW, baseH int) int {
	if fontSize != nil && *fontSize > 0 {
		return *fontSize
	}
	return max(min(baseW, baseH)/25, 16)
}

// effectiveSpace 与 AddRepeatWatermark 的自动间距公式一致。
func effectiveSpace(space *int, fontSize int) int {
	if space != nil && *space > 0 {
		return *space
	}
	return fontSize * 2
}

// markCanvasSize 与 generateMark 的临时画布公式一致。
func markCanvasSize(fontSize int, text string) image.Point {
	runes := utf8.RuneCountInString(text)
	return image.Point{
		X: max(200, fontSize*max(4, runes)),
		Y: max(64, int(float64(fontSize)*2.5)),
	}
}
