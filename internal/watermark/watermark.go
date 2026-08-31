package watermark

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"math"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/disintegration/imaging"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"imagetoolbox/internal/imageio"
)

// Position defines the watermark position.
type Position string

const (
	BottomRight Position = "bottom-right"
	BottomLeft  Position = "bottom-left"
	TopRight    Position = "top-right"
	TopLeft     Position = "top-left"
	Center      Position = "center"
)

// RepeatOptions options for repeat watermark mode.
type RepeatOptions struct {
	Color          *string
	Space          *int
	Angle          *int
	Opacity        *float64
	FontPath       string
	FontSize       *int
	FontHeightCrop *float64
}

// PositionOptions options for position watermark mode.
type PositionOptions struct {
	Opacity       *float64
	Position      Position
	FontPath      string
	FontSize      *int
	Color         *string
	MarginRatio   *float64
	JPGBackground *color.NRGBA
}

type ImageOptions struct {
	ImagePath     string
	Opacity       *float64
	Position      Position
	ScaleRatio    *float64
	MarginRatio   *float64
	JPGBackground *color.NRGBA
}

// Mode selects the text watermark layout.
type Mode string

const (
	ModePosition Mode = "position"
	ModeRepeat   Mode = "repeat"
)

// Options describes a complete file-level watermark operation. Pointer fields
// preserve the distinction between an omitted option and an explicit zero.
type Options struct {
	Text      string
	ImagePath string
	Mode      Mode
	Position  Position
	Color     string
	FontPath  string
	Opacity   *float64
	FontSize  *int
	Space     *int
	Angle     *int
	Margin    *float64
	Scale     *float64
}

// Domain 参数上限。这些是 watermark 语义的合理范围，而非 VPS 资源配置。
const (
	// MaxFontSize 限制字号上限，防止极端字号触发巨大字形栅格化。
	MaxFontSize = 4096
	// MaxTextRunes 限制水印文字长度；数千字符已覆盖绝大多数场景，
	// 不应只依赖 multipart body 大小限制。
	MaxTextRunes = 4096
)

// Normalize 填充默认值（Mode 空值默认 position）。
func (o *Options) Normalize() {
	if o.Mode == "" {
		o.Mode = ModePosition
	}
}

// Validate 校验 watermark 参数的完整业务规则，CLI 与 HTTP 共用，
// 保证两个入口语义一致。
func (o Options) Validate() error {
	hasText := strings.TrimSpace(o.Text) != ""
	hasImage := strings.TrimSpace(o.ImagePath) != ""
	if hasText == hasImage {
		return errors.New("must provide exactly one of text or image watermark")
	}
	if utf8.RuneCountInString(o.Text) > MaxTextRunes {
		return fmt.Errorf("watermark text must not exceed %d runes", MaxTextRunes)
	}
	switch o.Mode {
	case ModePosition, ModeRepeat:
	default:
		return fmt.Errorf("unsupported watermark mode: %s", o.Mode)
	}
	if hasImage && o.Mode == ModeRepeat {
		return errors.New("image watermark only supports position mode")
	}
	switch o.Position {
	case "", BottomRight, BottomLeft, TopRight, TopLeft, Center:
	default:
		return fmt.Errorf("unsupported watermark position: %s", o.Position)
	}
	if o.Opacity != nil && (*o.Opacity < 0 || *o.Opacity > 1) {
		return fmt.Errorf("opacity must be between 0 and 1: %v", *o.Opacity)
	}
	if o.FontSize != nil && (*o.FontSize < 0 || *o.FontSize > MaxFontSize) {
		return fmt.Errorf("font size must be between 0 and %d: %d", MaxFontSize, *o.FontSize)
	}
	if o.Space != nil && *o.Space < 0 {
		return fmt.Errorf("space must not be negative: %d", *o.Space)
	}
	if o.Angle != nil && (*o.Angle < -360 || *o.Angle > 360) {
		return fmt.Errorf("angle must be between -360 and 360: %d", *o.Angle)
	}
	if o.Margin != nil && *o.Margin < 0 {
		return fmt.Errorf("margin must not be negative: %v", *o.Margin)
	}
	if o.Scale != nil && *o.Scale <= 0 {
		return fmt.Errorf("scale must be greater than 0: %v", *o.Scale)
	}
	if strings.TrimSpace(o.Color) != "" {
		if _, err := imageio.ParseHexColor(o.Color); err != nil {
			return fmt.Errorf("invalid watermark color: %w", err)
		}
	}
	return nil
}

// AddFile applies a text or image watermark and owns the operation dispatch.
func AddFile(inputPath, outputPath string, opts Options) error {
	opts.Normalize()
	if err := opts.Validate(); err != nil {
		return err
	}

	if strings.TrimSpace(opts.ImagePath) != "" {
		_, err := AddImageWatermark(inputPath, outputPath, &ImageOptions{
			ImagePath:   opts.ImagePath,
			Opacity:     opts.Opacity,
			Position:    opts.Position,
			ScaleRatio:  opts.Scale,
			MarginRatio: opts.Margin,
		})
		return err
	}

	switch opts.Mode {
	case ModePosition:
		_, err := AddPositionWatermark(inputPath, outputPath, opts.Text, &PositionOptions{
			Opacity:     opts.Opacity,
			Position:    opts.Position,
			FontPath:    opts.FontPath,
			FontSize:    opts.FontSize,
			Color:       &opts.Color,
			MarginRatio: opts.Margin,
		})
		return err
	case ModeRepeat:
		_, err := AddRepeatWatermark(inputPath, outputPath, opts.Text, &RepeatOptions{
			Color:    &opts.Color,
			Space:    opts.Space,
			Angle:    opts.Angle,
			Opacity:  opts.Opacity,
			FontPath: opts.FontPath,
			FontSize: opts.FontSize,
		})
		return err
	default:
		return fmt.Errorf("unsupported watermark mode: %s", opts.Mode)
	}
}

// WatermarkArgs configuration for watermark generation.
type WatermarkArgs struct {
	Mark           string
	Color          string
	Space          int
	Angle          int
	FontFamily     string
	FontHeightCrop float64
	Size           int
	Opacity        float64
}

// Watermarker provides watermark generation and application.
type Watermarker struct {
	args    WatermarkArgs
	markImg image.Image
}

// NewWatermarker creates a Watermarker and pre-generates the mark tile image.
func NewWatermarker(args WatermarkArgs) (*Watermarker, error) {
	if strings.TrimSpace(args.Mark) == "" {
		return nil, errors.New("args.Mark must not be empty")
	}
	wm := &Watermarker{args: args}
	mark, err := wm.generateMark()
	if err != nil {
		return nil, err
	}
	wm.markImg = mark
	if wm.markImg == nil {
		log.Printf("generated mark image is empty; check mark text and font path")
	}
	return wm, nil
}

// Apply overlays the repeated watermark onto the image.
func (w *Watermarker) Apply(im image.Image) (image.Image, error) {
	if w.markImg == nil {
		return nil, errors.New("mark image not generated")
	}

	base := imaging.Clone(im)
	bw := base.Bounds().Dx()
	bh := base.Bounds().Dy()

	mw := w.markImg.Bounds().Dx()
	mh := w.markImg.Bounds().Dy()

	c := int(math.Hypot(float64(bw), float64(bh))) + max(mw, mh)*2
	tiled := image.NewNRGBA(image.Rect(0, 0, c, c))

	y := 0
	rowShift := 0
	for y < c {
		x := -int(float64(mw+w.args.Space) * 0.5 * float64(rowShift))
		rowShift ^= 1
		for x < c {
			pasteWithAlpha(tiled, w.markImg, x, y)
			x += mw + w.args.Space
		}
		y += mh + w.args.Space
	}

	rotated := imaging.Rotate(tiled, float64(w.args.Angle), color.NRGBA{0, 0, 0, 0})

	overlay := image.NewNRGBA(image.Rect(0, 0, bw, bh))
	offX := (bw - rotated.Bounds().Dx()) / 2
	offY := (bh - rotated.Bounds().Dy()) / 2
	pasteWithAlpha(overlay, rotated, offX, offY)

	result := image.NewNRGBA(base.Bounds())
	draw.Draw(result, base.Bounds(), base, image.Point{}, draw.Src)
	draw.Draw(result, overlay.Bounds(), overlay, image.Point{}, draw.Over)

	if sameRGB(base, result) {
		log.Printf("result identical to source; watermark not visible (increase opacity or verify font)")
	}

	return result, nil
}

// SaveImage saves the image to disk with correct RGBA -> JPEG handling.
func SaveImage(img image.Image, path string, jpgBackground color.NRGBA) error {
	return imageio.Save(path, img, imageio.SaveOptions{
		Quality:    100,
		Background: jpgBackground,
	})
}

// AddRepeatWatermark adds a repeated text watermark and saves the output.
func AddRepeatWatermark(inputPath, outputPath, text string, opts *RepeatOptions) (image.Image, error) {
	var colorVal *string
	var space *int
	var angleVal = DefaultRepeatAngle
	var opacityVal = DefaultOpacity
	var fontSize *int
	var fontHeightCropVal = 1.0
	var fontPath string

	if opts != nil {
		colorVal = opts.Color
		space = opts.Space
		if opts.Angle != nil {
			angleVal = *opts.Angle
		}
		if opts.Opacity != nil {
			opacityVal = *opts.Opacity
		}
		fontSize = opts.FontSize
		if opts.FontHeightCrop != nil {
			fontHeightCropVal = *opts.FontHeightCrop
		}
		fontPath = opts.FontPath
	}

	// 先打开图片以获取尺寸
	im, err := imaging.Open(inputPath)
	if err != nil {
		return nil, err
	}

	// 如果未指定字体大小，则自动计算（与资源规划共用同一公式）
	fontSizeVal := effectiveFontSize(fontSize, im.Bounds().Dx(), im.Bounds().Dy())

	// 如果未指定间距，则根据字体大小自动计算
	spaceVal := effectiveSpace(space, fontSizeVal)

	// 如果未指定颜色，则根据图片亮度自动选择
	var colorStr string
	if colorVal != nil && *colorVal != "" {
		colorStr = *colorVal
	} else {
		rgba := imaging.Clone(im)
		brightness := meanRedChannel(rgba, rgba.Bounds())
		if brightness > 128 {
			colorStr = "#000000" // 亮背景用黑色
		} else {
			colorStr = "#FFFFFF" // 暗背景用白色
		}
	}

	args := WatermarkArgs{
		Mark:           text,
		Color:          colorStr,
		Space:          spaceVal,
		Angle:          angleVal,
		FontFamily:     fontPath,
		FontHeightCrop: fontHeightCropVal,
		Size:           fontSizeVal,
		Opacity:        opacityVal,
	}
	wm, err := NewWatermarker(args)
	if err != nil {
		return nil, err
	}
	marked, err := wm.Apply(im)
	if err != nil {
		return nil, err
	}
	if err := SaveImage(marked, outputPath, color.NRGBA{255, 255, 255, 255}); err != nil {
		return nil, err
	}
	return marked, nil
}

// AddPositionWatermark adds a single positioned watermark and saves the output.
func AddPositionWatermark(inputPath, outputPath, text string, opts *PositionOptions) (image.Image, error) {
	var opacityVal = DefaultOpacity
	var marginRatio = DefaultMarginRatio
	var fontPath string
	var pos Position = DefaultPositionMode
	var jpgBg color.NRGBA
	var fontSize *int
	var colorStr *string

	if opts != nil {
		if opts.Opacity != nil {
			opacityVal = *opts.Opacity
		}
		if opts.MarginRatio != nil {
			marginRatio = *opts.MarginRatio
		}
		if opts.FontPath != "" {
			fontPath = opts.FontPath
		}
		if opts.Position != "" {
			pos = opts.Position
		}
		if opts.JPGBackground != nil {
			jpgBg = *opts.JPGBackground
		}
		fontSize = opts.FontSize
		colorStr = opts.Color
	}
	img, err := imaging.Open(inputPath)
	if err != nil {
		return nil, err
	}
	rgba := imaging.Clone(img)

	width := rgba.Bounds().Dx()
	height := rgba.Bounds().Dy()

	// 如果未指定字体大小，则自动计算（与资源规划共用同一公式）
	fontSizeVal := effectiveFontSize(fontSize, width, height)

	face, err := loadFontFaceWithFallback(fontPath, fontSizeVal)
	if err != nil {
		return nil, err
	}

	bounds, _ := font.BoundString(face, text)
	textW := fixedToInt(bounds.Max.X - bounds.Min.X)
	textH := fixedToInt(bounds.Max.Y - bounds.Min.Y)
	if textW <= 0 || textH <= 0 {
		return nil, errors.New("text bounds are empty")
	}

	sample := image.Rect(
		width/2-textW/2,
		height/2-textH/2,
		width/2+textW/2,
		height/2+textH/2,
	).Intersect(rgba.Bounds())
	if sample.Empty() {
		sample = rgba.Bounds()
	}

	brightness := meanRedChannel(rgba, sample)
	alpha := clampInt(int(math.Round(255*opacityVal)), 0, 255)
	outlineAlpha := clampInt(int(math.Round(255*opacityVal*0.6)), 0, 255)

	var fillColor, outlineColor color.NRGBA

	// 如果指定了颜色，则使用指定颜色
	if colorStr != nil && *colorStr != "" {
		parsedColor, err := imageio.ParseHexColor(*colorStr)
		if err != nil {
			return nil, fmt.Errorf("invalid color format: %w", err)
		}
		fillColor = color.NRGBA{R: parsedColor.R, G: parsedColor.G, B: parsedColor.B, A: uint8(alpha)}
		// 描边使用对比色
		if brightness > 128 {
			outlineColor = color.NRGBA{255, 255, 255, uint8(outlineAlpha)}
		} else {
			outlineColor = color.NRGBA{0, 0, 0, uint8(outlineAlpha)}
		}
	} else {
		// 自动选择颜色
		if brightness > 128 {
			fillColor = color.NRGBA{0, 0, 0, uint8(alpha)}
			outlineColor = color.NRGBA{255, 255, 255, uint8(outlineAlpha)}
		} else {
			fillColor = color.NRGBA{255, 255, 255, uint8(alpha)}
			outlineColor = color.NRGBA{0, 0, 0, uint8(outlineAlpha)}
		}
	}

	chosen := calculatePosition(width, height, textW, textH, pos, marginRatio)

	drawTextOutlined(rgba, face, chosen.X, chosen.Y, text, fillColor, outlineColor, 2)

	if jpgBg == (color.NRGBA{}) {
		jpgBg = color.NRGBA{255, 255, 255, 255}
	}
	if err := SaveImage(rgba, outputPath, jpgBg); err != nil {
		return nil, err
	}

	return rgba, nil
}

func AddImageWatermark(inputPath, outputPath string, opts *ImageOptions) (image.Image, error) {
	if opts == nil {
		return nil, errors.New("image watermark options are required")
	}
	if strings.TrimSpace(opts.ImagePath) == "" {
		return nil, errors.New("image watermark path is required")
	}

	opacityVal := DefaultOpacity
	scaleRatio := DefaultImageScale
	marginRatio := DefaultMarginRatio
	pos := DefaultPositionMode
	jpgBg := color.NRGBA{255, 255, 255, 255}

	if opts.Opacity != nil {
		opacityVal = *opts.Opacity
	}
	if opts.ScaleRatio != nil {
		scaleRatio = *opts.ScaleRatio
	}
	if opts.MarginRatio != nil {
		marginRatio = *opts.MarginRatio
	}
	if opts.Position != "" {
		pos = opts.Position
	}
	if opts.JPGBackground != nil {
		jpgBg = *opts.JPGBackground
	}
	if scaleRatio <= 0 {
		return nil, errors.New("scale ratio must be greater than 0")
	}

	base, err := imaging.Open(inputPath)
	if err != nil {
		return nil, err
	}
	logo, err := imaging.Open(opts.ImagePath)
	if err != nil {
		return nil, err
	}

	baseImg := imaging.Clone(base)
	baseW := baseImg.Bounds().Dx()
	baseH := baseImg.Bounds().Dy()
	logoW := logo.Bounds().Dx()
	logoH := logo.Bounds().Dy()
	if logoW <= 0 || logoH <= 0 {
		return nil, errors.New("watermark image dimensions are invalid")
	}

	targetW, targetH := scaledLogoSize(baseW, baseH, logoW, logoH, scaleRatio)

	resizedLogo := imaging.Resize(logo, targetW, targetH, imaging.Lanczos)
	overlay, err := setOpacity(resizedLogo, opacityVal)
	if err != nil {
		return nil, err
	}

	point := calculatePosition(baseW, baseH, targetW, targetH, pos, marginRatio)
	pasteWithAlpha(baseImg, overlay, point.X, point.Y)

	if err := SaveImage(baseImg, outputPath, jpgBg); err != nil {
		return nil, err
	}

	return baseImg, nil
}

func (w *Watermarker) generateMark() (image.Image, error) {
	face, err := loadFontFaceWithFallback(w.args.FontFamily, w.args.Size)
	if err != nil {
		return nil, err
	}
	colorVal, err := imageio.ParseHexColor(w.args.Color)
	if err != nil {
		return nil, err
	}

	// 临时画布尺寸与 markCanvasSize（资源规划）共用同一公式
	size := markCanvasSize(w.args.Size, w.args.Mark)
	canvas := image.NewNRGBA(image.Rect(0, 0, size.X, size.Y))

	d := &font.Drawer{
		Dst:  canvas,
		Src:  image.NewUniform(colorVal),
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.I(0),
			Y: fixed.I(0) + face.Metrics().Ascent,
		},
	}
	d.DrawString(w.args.Mark)

	bbox, ok := tightAlphaBounds(canvas)
	if !ok {
		return nil, nil
	}
	mark := imaging.Crop(canvas, bbox)

	hcrop := w.args.FontHeightCrop
	if hcrop > 0 && hcrop != 1.0 {
		newH := int(math.Max(1, math.Round(float64(w.args.Size)*hcrop)))
		mark = imaging.Resize(mark, mark.Bounds().Dx(), newH, imaging.Lanczos)
	}

	return setOpacity(mark, w.args.Opacity)
}

func loadFontFace(path string, size int) (font.Face, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("font path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fnt, err := opentype.Parse(data)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(fnt, &opentype.FaceOptions{
		Size:    float64(size),
		DPI:     72,
		Hinting: font.HintingFull,
	})
}

func loadFontFaceWithFallback(path string, size int) (font.Face, error) {
	if strings.TrimSpace(path) != "" {
		face, err := loadFontFace(path, size)
		if err == nil {
			return face, nil
		}
		log.Printf("failed to load font %q, falling back to Go Regular: %v", path, err)
	}
	if strings.TrimSpace(path) == "" {
		if arial := firstExistingFontPath([]string{
			"arial.ttf",
			"/Library/Fonts/Arial.ttf",
			"/System/Library/Fonts/Supplemental/Arial.ttf",
			"C:\\Windows\\Fonts\\arial.ttf",
			"/usr/share/fonts/truetype/msttcorefonts/Arial.ttf",
			"/usr/share/fonts/truetype/msttcorefonts/arial.ttf",
		}); arial != "" {
			face, err := loadFontFace(arial, size)
			if err == nil {
				return face, nil
			}
			log.Printf("failed to load fallback Arial font %q, using Go Regular: %v", arial, err)
		}
	}
	fnt, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(fnt, &opentype.FaceOptions{
		Size:    float64(size),
		DPI:     72,
		Hinting: font.HintingFull,
	})
}

func firstExistingFontPath(candidates []string) string {
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func setOpacity(img image.Image, opacity float64) (image.Image, error) {
	if opacity < 0 || opacity > 1 {
		return nil, errors.New("opacity must be between 0 and 1")
	}
	out := imaging.Clone(img)
	for i := 0; i < len(out.Pix); i += 4 {
		out.Pix[i+3] = uint8(math.Round(float64(out.Pix[i+3]) * opacity))
	}
	return out, nil
}

func tightAlphaBounds(img *image.NRGBA) (image.Rectangle, bool) {
	b := img.Bounds()
	minX, minY := b.Max.X, b.Max.Y
	maxX, maxY := b.Min.X, b.Min.Y
	found := false
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.NRGBAAt(x, y).A != 0 {
				if !found {
					minX, minY = x, y
					maxX, maxY = x, y
					found = true
				} else {
					if x < minX {
						minX = x
					}
					if y < minY {
						minY = y
					}
					if x > maxX {
						maxX = x
					}
					if y > maxY {
						maxY = y
					}
				}
			}
		}
	}
	if !found {
		return image.Rectangle{}, false
	}
	return image.Rect(minX, minY, maxX+1, maxY+1), true
}

func pasteWithAlpha(dst *image.NRGBA, src image.Image, x, y int) {
	r := image.Rect(x, y, x+src.Bounds().Dx(), y+src.Bounds().Dy())
	draw.DrawMask(dst, r, src, src.Bounds().Min, src, src.Bounds().Min, draw.Over)
}

func meanRedChannel(img *image.NRGBA, r image.Rectangle) float64 {
	var sum uint64
	var count uint64
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			pix := img.NRGBAAt(x, y)
			sum += uint64(pix.R)
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return float64(sum) / float64(count)
}

func drawTextOutlined(dst *image.NRGBA, face font.Face, x, y int, text string, fill, outline color.NRGBA, outlineRange int) {
	for dx := -outlineRange; dx <= outlineRange; dx++ {
		for dy := -outlineRange; dy <= outlineRange; dy++ {
			if dx == 0 && dy == 0 {
				continue
			}
			drawTextAt(dst, face, x+dx, y+dy, text, outline)
		}
	}
	drawTextAt(dst, face, x, y, text, fill)
}

func drawTextAt(dst *image.NRGBA, face font.Face, x, y int, text string, col color.NRGBA) {
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(col),
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.I(x),
			Y: fixed.I(y) + face.Metrics().Ascent,
		},
	}
	d.DrawString(text)
}

func sameRGB(a, b image.Image) bool {
	ab := a.Bounds()
	bb := b.Bounds()
	if ab.Dx() != bb.Dx() || ab.Dy() != bb.Dy() {
		return false
	}
	for y := 0; y < ab.Dy(); y++ {
		for x := 0; x < ab.Dx(); x++ {
			ar, ag, abv, _ := a.At(ab.Min.X+x, ab.Min.Y+y).RGBA()
			br, bg, bbv, _ := b.At(bb.Min.X+x, bb.Min.Y+y).RGBA()
			if ar != br || ag != bg || abv != bbv {
				return false
			}
		}
	}
	return true
}

func fixedToInt(v fixed.Int26_6) int {
	return int(math.Ceil(float64(v) / 64.0))
}

func calculatePosition(baseW, baseH, itemW, itemH int, pos Position, marginRatio float64) image.Point {
	margin := int(float64(min(baseW, baseH)) * marginRatio)

	positions := map[Position]image.Point{
		BottomRight: {X: baseW - itemW - margin, Y: baseH - itemH - margin},
		BottomLeft:  {X: margin, Y: baseH - itemH - margin},
		TopRight:    {X: baseW - itemW - margin, Y: margin},
		TopLeft:     {X: margin, Y: margin},
		Center:      {X: (baseW - itemW) / 2, Y: (baseH - itemH) / 2},
	}

	point, ok := positions[pos]
	if !ok {
		point = positions[BottomRight]
	}
	return point
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
