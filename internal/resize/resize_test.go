package resize

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/disintegration/imaging"
	"imagetoolbox/internal/imageio"
)

func TestApplyPercent(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	got, err := Apply(img, Options{Percent: "50%"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Bounds().Dx() != 100 || got.Bounds().Dy() != 50 {
		t.Fatalf("got %dx%d, want 100x50", got.Bounds().Dx(), got.Bounds().Dy())
	}
}

func TestApplyWidthOnly(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	got, err := Apply(img, Options{Width: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Bounds().Dx() != 50 || got.Bounds().Dy() != 25 {
		t.Fatalf("got %dx%d, want 50x25", got.Bounds().Dx(), got.Bounds().Dy())
	}
}

func TestApplyStretch(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	got, err := Apply(img, Options{Width: 50, Height: 50, Mode: ModeStretch})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Bounds().Dx() != 50 || got.Bounds().Dy() != 50 {
		t.Fatalf("got %dx%d, want 50x50", got.Bounds().Dx(), got.Bounds().Dy())
	}
}

func TestApplyFillUsesAnchor(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.NRGBA{255, 0, 0, 255})
		}
		for x := 100; x < 200; x++ {
			img.Set(x, y, color.NRGBA{0, 0, 255, 255})
		}
	}

	left, err := Apply(img, Options{Width: 50, Height: 50, Mode: ModeFill, Anchor: "left"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	right, err := Apply(img, Options{Width: 50, Height: 50, Mode: ModeFill, Anchor: "right"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lr, _, _, _ := left.At(5, 25).RGBA()
	rr, _, bb, _ := right.At(45, 25).RGBA()
	if lr == 0 {
		t.Fatal("expected left-anchored image to keep red area")
	}
	if rr != 0 || bb == 0 {
		t.Fatal("expected right-anchored image to keep blue area")
	}
}

func TestValidateOptions(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	_, err := Apply(img, Options{Percent: "50%", Width: 50})
	if err == nil {
		t.Fatal("expected conflict error")
	}

	_, err = Apply(img, Options{Mode: ModeFill, Width: 50})
	if err == nil {
		t.Fatal("expected fill validation error")
	}
}

func TestDefaultResizeContract(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	got, err := Apply(img, Options{Width: 50})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got.Bounds() != image.Rect(0, 0, 50, 25) {
		t.Fatalf("default fit dimensions = %v", got.Bounds())
	}
}

// TestApplyPercentUpscale 验证 percent 语义是按百分比精确缩放：
// 200% 必须真正放大，而不是被 fit 的"只缩小不放大"逻辑吞掉。
func TestApplyPercentUpscale(t *testing.T) {
	for _, tt := range []struct {
		percent string
		srcW    int
		srcH    int
		wantW   int
		wantH   int
	}{
		{percent: "200%", srcW: 100, srcH: 100, wantW: 200, wantH: 200},
		{percent: "150%", srcW: 200, srcH: 100, wantW: 300, wantH: 150},
		{percent: "50%", srcW: 100, srcH: 100, wantW: 50, wantH: 50},
	} {
		t.Run(tt.percent, func(t *testing.T) {
			img := image.NewNRGBA(image.Rect(0, 0, tt.srcW, tt.srcH))
			got, err := Apply(img, Options{Percent: tt.percent})
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if got.Bounds().Dx() != tt.wantW || got.Bounds().Dy() != tt.wantH {
				t.Fatalf("Apply(percent=%s) = %dx%d, want %dx%d", tt.percent, got.Bounds().Dx(), got.Bounds().Dy(), tt.wantW, tt.wantH)
			}
		})
	}
}

// sameFilter 比较 ResampleFilter：结构体含函数字段无法直接比较，
// 用 Support 加采样点上的 Kernel 输出锁定具体过滤器核。
// NearestNeighbor 没有核函数（Kernel 为 nil，imaging 内部特判），
// 双方都为 nil 时视为相等。
func sameFilter(a, b imaging.ResampleFilter) bool {
	if a.Support != b.Support {
		return false
	}
	if (a.Kernel == nil) != (b.Kernel == nil) {
		return false
	}
	if a.Kernel == nil {
		return true
	}
	for _, x := range []float64{0, 0.25, 0.5, 1, 1.5} {
		if math.Abs(a.Kernel(x)-b.Kernel(x)) > 1e-12 {
			return false
		}
	}
	return true
}

// TestParseFilter 锁定 filter 名称到 imaging.ResampleFilter 的成功映射，
// 防止某个名称被静默改映射到其他核。
func TestParseFilter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  imaging.ResampleFilter
	}{
		{name: "default", input: "", want: imaging.Lanczos},
		{name: "nearest", input: "nearest", want: imaging.NearestNeighbor},
		{name: "linear", input: "linear", want: imaging.Linear},
		{name: "mitchell", input: "mitchell", want: imaging.MitchellNetravali},
		{name: "catmullrom", input: "catmullrom", want: imaging.CatmullRom},
		{name: "lanczos", input: "lanczos", want: imaging.Lanczos},
		{name: "case insensitive", input: "MITCHELL", want: imaging.MitchellNetravali},
		{name: "trim spaces", input: " mitchell ", want: imaging.MitchellNetravali},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFilter(tt.input)
			if err != nil {
				t.Fatalf("parseFilter(%q): %v", tt.input, err)
			}
			if !sameFilter(got, tt.want) {
				t.Fatalf("parseFilter(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

// TestFilterNamesMatchesParseFilter 锁定 FilterNames 是过滤器的单一
// 事实来源：列表无空名、无重复、顺序即 CLI help 渲染顺序，且每个
// 名称都能被 parseFilter 接受。CLI validator/help 从 FilterNames
// 派生，此处失败意味着支持集合本身发生了漂移。
func TestFilterNamesMatchesParseFilter(t *testing.T) {
	want := []string{"nearest", "linear", "mitchell", "catmullrom", "lanczos"}
	got := FilterNames()
	if !slices.Equal(got, want) {
		t.Fatalf("FilterNames() = %v, want %v", got, want)
	}
	for _, name := range got {
		if _, err := parseFilter(name); err != nil {
			t.Fatalf("parseFilter(%q) rejected but listed in FilterNames: %v", name, err)
		}
	}
}

// TestApplyMitchellFilter 行为冒烟：mitchell 在 Apply 全链路可用。
func TestApplyMitchellFilter(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 4))
	got, err := Apply(img, Options{Percent: "200%", Filter: "mitchell"})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got.Bounds().Dx() != 16 || got.Bounds().Dy() != 8 {
		t.Fatalf("Apply(mitchell 200%%) = %dx%d, want 16x8", got.Bounds().Dx(), got.Bounds().Dy())
	}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name   string
		bounds image.Rectangle
		opts   Options
		width  int
		height int
	}{
		// fit 单边指定时推导另一边，Resolve 结果等于真实输出尺寸。
		{name: "width only keeps aspect ratio", bounds: image.Rect(0, 0, 4000, 2000), opts: Options{Width: 1000}, width: 1000, height: 500},
		{name: "height only keeps aspect ratio", bounds: image.Rect(0, 0, 4000, 2000), opts: Options{Height: 500}, width: 1000, height: 500},
		{name: "stretch single side", bounds: image.Rect(0, 0, 4000, 2000), opts: Options{Width: 1000, Mode: ModeStretch}, width: 1000, height: 500},
		{name: "percent", bounds: image.Rect(0, 0, 100, 100), opts: Options{Percent: "50%"}, width: 50, height: 50},
		{name: "percent upscale", bounds: image.Rect(0, 0, 100, 100), opts: Options{Percent: "1000000%"}, width: 1_000_000, height: 1_000_000},
		{name: "explicit box", bounds: image.Rect(0, 0, 4000, 2000), opts: Options{Width: 100, Height: 50}, width: 100, height: 50},
		// fit 的 box 只是包围盒：源图在 box 内时输出等于源图尺寸，
		// 源图更大时按宽高比贴边缩小（与 imaging.Fit 完全一致）。
		{name: "fit bounding box larger than source", bounds: image.Rect(0, 0, 1000, 100), opts: Options{Width: 20000, Height: 100}, width: 1000, height: 100},
		{name: "fit box within both sides clones source", bounds: image.Rect(0, 0, 100, 100), opts: Options{Width: 500, Height: 500}, width: 100, height: 100},
		{name: "fit downscale keeps aspect", bounds: image.Rect(0, 0, 1000, 100), opts: Options{Width: 200, Height: 200}, width: 200, height: 20},
		{name: "stretch box is exact", bounds: image.Rect(0, 0, 1000, 100), opts: Options{Width: 20000, Height: 100, Mode: ModeStretch}, width: 20000, height: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := Resolve(tt.bounds, tt.opts)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if plan.Width != tt.width || plan.Height != tt.height {
				t.Fatalf("Resolve() = %dx%d, want %dx%d", plan.Width, plan.Height, tt.width, tt.height)
			}
		})
	}
	t.Run("resolve matches apply output", func(t *testing.T) {
		img := image.NewNRGBA(image.Rect(0, 0, 200, 100))
		for _, opts := range []Options{
			{Width: 50},
			{Height: 25},
			{Percent: "33%"},
			{Percent: "200%"},
			{Width: 50, Height: 50, Mode: ModeStretch},
			// fit 放大 box：真实输出等于源图（Fit 直接 Clone）。
			{Width: 500, Height: 500},
			{Width: 500, Height: 500, Mode: ModeFit},
			// fit 缩小 box：按宽高比贴边。
			{Width: 80, Height: 80},
		} {
			plan, err := Resolve(img.Bounds(), opts)
			if err != nil {
				t.Fatalf("Resolve(%+v) error = %v", opts, err)
			}
			got, err := Apply(img, opts)
			if err != nil {
				t.Fatalf("Apply(%+v) error = %v", opts, err)
			}
			if got.Bounds().Dx() != plan.Width || got.Bounds().Dy() != plan.Height {
				t.Fatalf("Apply(%+v) = %dx%d, Resolve planned %dx%d", opts, got.Bounds().Dx(), got.Bounds().Dy(), plan.Width, plan.Height)
			}
		}
	})
	t.Run("invalid options return errors", func(t *testing.T) {
		bounds := image.Rect(0, 0, 10, 10)
		for _, opts := range []Options{
			{},
			{Percent: "50%", Width: 10},
			{Percent: "NaN%"},
			{Percent: "Inf%"},
			{Mode: ModeFill, Width: 10},
			{Mode: "bogus"},
			{Width: 10, Anchor: "bogus"},
			{Width: 10, Filter: "bogus"},
		} {
			if _, err := Resolve(bounds, opts); err == nil {
				t.Fatalf("Resolve(%+v) = nil error, want error", opts)
			}
		}
	})
}

// buildOrientedJPEGFixture 生成携带 EXIF Orientation=6 的 JPEG：
// 物理尺寸 4×8，应用旋转后的逻辑尺寸为 8×4。
func buildOrientedJPEGFixture(t *testing.T) string {
	t.Helper()

	// TIFF: II + 42 + IFD0 偏移 8 + 1 条目（Orientation=6, SHORT, count 1）
	var tiff bytes.Buffer
	tiff.WriteString("II")
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(42))
	_ = binary.Write(&tiff, binary.LittleEndian, uint32(8))
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(1))
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(0x0112))
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(3))
	_ = binary.Write(&tiff, binary.LittleEndian, uint32(1))
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(6))
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(0))
	_ = binary.Write(&tiff, binary.LittleEndian, uint32(0))

	var body bytes.Buffer
	if err := jpeg.Encode(&body, image.NewRGBA(image.Rect(0, 0, 4, 8)), nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	encoded := body.Bytes()

	var out bytes.Buffer
	out.Write(encoded[:2]) // SOI
	out.WriteByte(0xFF)
	out.WriteByte(0xE1)
	_ = binary.Write(&out, binary.BigEndian, uint16(2+6+tiff.Len()))
	out.WriteString("Exif\x00\x00")
	out.Write(tiff.Bytes())
	out.Write(encoded[2:])

	path := filepath.Join(t.TempDir(), "oriented.jpg")
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// TestResizeFileOrientedJPEG 锁定统一后的 orientation 语义：
// 计划推导与解码都基于应用 EXIF 旋转后的逻辑尺寸。
// 物理 4×8 / Orientation 6 → 逻辑 8×4，fit 包围盒 100×100 内
// 的源图输出保持 8×4，而不是物理的 4×8。
func TestResizeFileOrientedJPEG(t *testing.T) {
	input := buildOrientedJPEGFixture(t)
	output := filepath.Join(t.TempDir(), "resized.png")

	if err := ResizeFile(input, output, Options{Width: 100, Height: 100, Mode: ModeFit}); err != nil {
		t.Fatalf("ResizeFile: %v", err)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if w, h := decoded.Bounds().Dx(), decoded.Bounds().Dy(); w != 8 || h != 4 {
		t.Fatalf("output = %dx%d, want 8x4 (logical, orientation applied)", w, h)
	}
}

// TestResizeFileRejectsGIF GIF 输入被统一格式契约拒绝，
// 不再经 imaging.Open 静默处理首帧。
func TestResizeFileRejectsGIF(t *testing.T) {
	var buf bytes.Buffer
	palette := color.Palette{color.White, color.Black}
	if err := gif.Encode(&buf, image.NewPaletted(image.Rect(0, 0, 4, 4), palette), nil); err != nil {
		t.Fatalf("encode gif: %v", err)
	}
	input := filepath.Join(t.TempDir(), "a.gif")
	if err := os.WriteFile(input, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if err := ResizeFile(input, filepath.Join(t.TempDir(), "out.png"), Options{Width: 2}); err == nil {
		t.Fatal("expected gif input to be rejected")
	}
}

func TestResizeFileRejectsSameFile(t *testing.T) {
	input := filepath.Join(t.TempDir(), "input.png")
	if err := os.WriteFile(input, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := ResizeFile(input, input, Options{Width: 1})
	if !errors.Is(err, imageio.ErrSameFile) {
		t.Fatalf("ResizeFile() error = %v, want imageio.ErrSameFile", err)
	}
}
