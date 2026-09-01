package rotate

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"imagetoolbox/internal/imageio"
)

// asymmetricImage 生成 2×3 的非对称图，每个像素颜色不同，
// 用于精确断言 90/180/270 的 coordinate mapping 与旋转方向。
func asymmetricImage() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 3))
	colors := [2][3]color.NRGBA{
		{{R: 255, A: 255}, {R: 255, G: 127, A: 255}, {R: 255, G: 255, A: 255}},
		{{B: 255, A: 255}, {G: 255, A: 255}, {R: 255, B: 255, A: 255}},
	}
	for y := range 3 {
		for x := range 2 {
			img.SetNRGBA(x, y, colors[x][y])
		}
	}
	return img
}

func nrgbaAt(t *testing.T, img image.Image, x, y int) color.NRGBA {
	t.Helper()
	col, ok := img.At(x, y).(color.NRGBA)
	if !ok {
		t.Fatalf("pixel (%d,%d) is not NRGBA: %T", x, y, img.At(x, y))
	}
	return col
}

func TestValidateRejectsInvalidAngles(t *testing.T) {
	tests := []struct {
		name  string
		angle float64
	}{
		{"零度", 0},
		{"正三百六十", 360},
		{"负三百六十", -360},
		{"超上限", 360.5},
		{"超下限", -400},
		{"NaN", math.NaN()},
		{"正 Inf", math.Inf(1)},
		{"负 Inf", math.Inf(-1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{Angle: tt.angle}
			if err := opts.Validate(); err == nil {
				t.Fatalf("Validate(%v) = nil, want error", tt.angle)
			}
			if _, err := Resolve(image.Rect(0, 0, 2, 3), opts); err == nil {
				t.Fatalf("Resolve(%v) = nil error, want error", tt.angle)
			}
			if _, err := Apply(asymmetricImage(), opts); err == nil {
				t.Fatalf("Apply(%v) = nil error, want error", tt.angle)
			}
		})
	}
}

func TestValidateAcceptsBoundaryAngles(t *testing.T) {
	for _, angle := range []float64{0.1, -0.1, 359.9, -359.9, 90, -90, 180, 45.5} {
		if err := (Options{Angle: angle}).Validate(); err != nil {
			t.Fatalf("Validate(%v) = %v, want nil", angle, err)
		}
	}
}

// TestResolveNormalizesAngle：Resolve 把角度归一化到 [0, 360)，
// -90 与 270 等价（都是顺时针 90°），90/270 交换宽高，180 尺寸不变。
func TestResolveNormalizesAngle(t *testing.T) {
	tests := []struct {
		angle     float64
		wantAngle float64
		wantW     int
		wantH     int
	}{
		{angle: 90, wantAngle: 90, wantW: 3, wantH: 2},
		{angle: -90, wantAngle: 270, wantW: 3, wantH: 2},
		{angle: 270, wantAngle: 270, wantW: 3, wantH: 2},
		{angle: -270, wantAngle: 90, wantW: 3, wantH: 2},
		{angle: 180, wantAngle: 180, wantW: 2, wantH: 3},
		{angle: -180, wantAngle: 180, wantW: 2, wantH: 3},
		{angle: -45, wantAngle: 315, wantW: 4, wantH: 4},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("angle_%v", tt.angle), func(t *testing.T) {
			plan, err := Resolve(image.Rect(0, 0, 2, 3), Options{Angle: tt.angle})
			if err != nil {
				t.Fatalf("Resolve(%v): %v", tt.angle, err)
			}
			if plan.Angle != tt.wantAngle || plan.Width != tt.wantW || plan.Height != tt.wantH {
				t.Fatalf("Resolve(%v) = %+v, want angle=%v %dx%d", tt.angle, plan, tt.wantAngle, tt.wantW, tt.wantH)
			}
		})
	}
}

// TestResolveWorkingBytes 锁定工作集内存的保守估算公式：正交旋转只分配
// 输出画布（4 × 输出像素数）；任意角度走 imaging.Rotate，输入 NRGBA 源
// 副本与输出画布同时驻留（4 × 输入像素数 + 4 × 输出像素数）。
// HTTP 据此在分配画布之前执行 MaxWorkingBytes 准入。
func TestResolveWorkingBytes(t *testing.T) {
	tests := []struct {
		name   string
		bounds image.Rectangle
		angle  float64
		want   int64
	}{
		{name: "90 度只计输出画布", bounds: image.Rect(0, 0, 32, 16), angle: 90, want: 4 * 16 * 32},
		{name: "180 度只计输出画布", bounds: image.Rect(0, 0, 32, 16), angle: 180, want: 4 * 32 * 16},
		{name: "270 度只计输出画布", bounds: image.Rect(0, 0, 32, 16), angle: 270, want: 4 * 16 * 32},
		{name: "45 度计入输入副本与输出画布", bounds: image.Rect(0, 0, 32, 16), angle: 45, want: 4*32*16 + 4*34*34},
		{name: "负角度归一化后按正交分派", bounds: image.Rect(0, 0, 32, 16), angle: -90, want: 4 * 16 * 32},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := Resolve(tt.bounds, Options{Angle: tt.angle})
			if err != nil {
				t.Fatalf("Resolve(%v, %v): %v", tt.bounds, tt.angle, err)
			}
			if plan.WorkingBytes != tt.want {
				t.Fatalf("WorkingBytes = %d, want %d (plan %+v)", plan.WorkingBytes, tt.want, plan)
			}
		})
	}
}

// TestResolveMatchesApplyBounds 锁定核心 invariant：
// Resolve(bounds).size == Apply(img).Bounds().size。
// rotatedSize 的舍入规则一旦与 imaging 漂移，这里会立即失败。
func TestResolveMatchesApplyBounds(t *testing.T) {
	sizes := []image.Point{
		{X: 2, Y: 3},
		{X: 3, Y: 2},
		{X: 100, Y: 100},
		{X: 101, Y: 99},
		{X: 1920, Y: 1080},
		{X: 1, Y: 1},
	}
	angles := []float64{90, -90, 180, 45, -45, 22.5, 89.5, 30, 0.5, 359.5, 123.456}
	for _, size := range sizes {
		for _, angle := range angles {
			t.Run(fmt.Sprintf("%dx%d_angle_%v", size.X, size.Y, angle), func(t *testing.T) {
				bounds := image.Rect(0, 0, size.X, size.Y)
				img := image.NewNRGBA(bounds)
				plan, err := Resolve(bounds, Options{Angle: angle})
				if err != nil {
					t.Fatalf("Resolve(%dx%d, %v): %v", size.X, size.Y, angle, err)
				}
				out, err := Apply(img, Options{Angle: angle})
				if err != nil {
					t.Fatalf("Apply(%dx%d, %v): %v", size.X, size.Y, angle, err)
				}
				if plan.Width != out.Bounds().Dx() || plan.Height != out.Bounds().Dy() {
					t.Fatalf("Resolve = %dx%d, Apply bounds = %dx%d (input %dx%d, angle %v)",
						plan.Width, plan.Height, out.Bounds().Dx(), out.Bounds().Dy(), size.X, size.Y, angle)
				}
			})
		}
	}
}

// TestExactRotationMapping 用 2×3 非对称图精确断言正交旋转的像素位置：
// +90 逆时针（src 右上角 → dst 左上角），-90 顺时针（src 左下角 → dst 左上角）。
// 方向错误不会被单纯的宽高断言发现，只有逐像素映射能锁定。
func TestExactRotationMapping(t *testing.T) {
	src := asymmetricImage()
	srcAt := func(x, y int) color.NRGBA { return nrgbaAt(t, src, x, y) }

	t.Run("90 度逆时针", func(t *testing.T) {
		out, err := Apply(src, Options{Angle: 90})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if got := out.Bounds(); got != image.Rect(0, 0, 3, 2) {
			t.Fatalf("bounds = %v, want 3x2", got)
		}
		// dst(x, y) = src(1-y, x)
		for y := range 2 {
			for x := range 3 {
				if got, want := nrgbaAt(t, out, x, y), srcAt(1-y, x); got != want {
					t.Fatalf("out(%d,%d) = %v, want src(1-%d,%d) = %v", x, y, got, y, x, want)
				}
			}
		}
	})

	t.Run("-90 度顺时针", func(t *testing.T) {
		out, err := Apply(src, Options{Angle: -90})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if got := out.Bounds(); got != image.Rect(0, 0, 3, 2) {
			t.Fatalf("bounds = %v, want 3x2", got)
		}
		// dst(x, y) = src(y, 2-x)
		for y := range 2 {
			for x := range 3 {
				if got, want := nrgbaAt(t, out, x, y), srcAt(y, 2-x); got != want {
					t.Fatalf("out(%d,%d) = %v, want src(%d,2-%d) = %v", x, y, got, y, x, want)
				}
			}
		}
	})

	t.Run("180 度", func(t *testing.T) {
		out, err := Apply(src, Options{Angle: 180})
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if got := out.Bounds(); got != image.Rect(0, 0, 2, 3) {
			t.Fatalf("bounds = %v, want 2x3", got)
		}
		// dst(x, y) = src(1-x, 2-y)
		for y := range 3 {
			for x := range 2 {
				if got, want := nrgbaAt(t, out, x, y), srcAt(1-x, 2-y); got != want {
					t.Fatalf("out(%d,%d) = %v, want src(1-%d,2-%d) = %v", x, y, got, x, y, want)
				}
			}
		}
	})
}

// 任意角度扩大画布，且四个角（旋转后未被源图覆盖的区域）保持透明。
func TestArbitraryAngleExpandsCanvasWithTransparentCorners(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	for y := range 100 {
		for x := range 100 {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 255 / 99), G: uint8(y * 255 / 99), B: 128, A: 255})
		}
	}

	plan, err := Resolve(img.Bounds(), Options{Angle: 45})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if plan.Width <= 100 || plan.Height <= 100 {
		t.Fatalf("Resolve(45°) = %dx%d, want expanded canvas", plan.Width, plan.Height)
	}

	out, err := Apply(img, Options{Angle: 45})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	bounds := out.Bounds()
	corners := []image.Point{
		{bounds.Min.X, bounds.Min.Y},
		{bounds.Max.X - 1, bounds.Min.Y},
		{bounds.Min.X, bounds.Max.Y - 1},
		{bounds.Max.X - 1, bounds.Max.Y - 1},
	}
	for _, c := range corners {
		if _, _, _, a := out.At(c.X, c.Y).RGBA(); a != 0 {
			t.Fatalf("corner %v alpha = %d, want 0", c, a)
		}
	}
}

func TestRotateFileSamePathRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.png")
	var buf bytes.Buffer
	if err := png.Encode(&buf, asymmetricImage()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	err := RotateFile(path, path, Options{Angle: 90})
	if !errors.Is(err, imageio.ErrSameFile) {
		t.Fatalf("RotateFile(same path) error = %v, want ErrSameFile", err)
	}
}

func TestRotateFilePNGAndWebP(t *testing.T) {
	dir := t.TempDir()
	writePNG := func(name string) string {
		path := filepath.Join(dir, name)
		var buf bytes.Buffer
		if err := png.Encode(&buf, asymmetricImage()); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	t.Run("PNG 90 度交换宽高", func(t *testing.T) {
		input := writePNG("a.png")
		output := filepath.Join(dir, "a_rotated.png")
		if err := RotateFile(input, output, Options{Angle: 90}); err != nil {
			t.Fatalf("RotateFile: %v", err)
		}
		img, err := imageio.OpenStatic(output)
		if err != nil {
			t.Fatalf("open output: %v", err)
		}
		if got := img.Bounds(); got != image.Rect(0, 0, 3, 2) {
			t.Fatalf("bounds = %v, want 3x2", got)
		}
	})

	t.Run("PNG 45 度四角透明", func(t *testing.T) {
		input := writePNG("b.png")
		output := filepath.Join(dir, "b_rotated.png")
		if err := RotateFile(input, output, Options{Angle: 45}); err != nil {
			t.Fatalf("RotateFile: %v", err)
		}
		img, err := imageio.OpenStatic(output)
		if err != nil {
			t.Fatalf("open output: %v", err)
		}
		if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
			t.Fatalf("corner (0,0) alpha = %d, want 0 (PNG must keep transparency)", a)
		}
	})

	t.Run("WebP 读写", func(t *testing.T) {
		pngPath := writePNG("c.png")
		webpPath := filepath.Join(dir, "c.webp")
		img, err := imageio.OpenStatic(pngPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := imageio.Save(webpPath, img, imageio.SaveOptions{Quality: 100}); err != nil {
			t.Fatalf("save webp: %v", err)
		}
		output := filepath.Join(dir, "c_rotated.webp")
		if err := RotateFile(webpPath, output, Options{Angle: 90}); err != nil {
			t.Fatalf("RotateFile(webp): %v", err)
		}
		decoded, err := imageio.OpenStatic(output)
		if err != nil {
			t.Fatalf("open webp output: %v", err)
		}
		if got := decoded.Bounds(); got != image.Rect(0, 0, 3, 2) {
			t.Fatalf("bounds = %v, want 3x2", got)
		}
	})
}

// TestRotateFileJPEGEXIFAppliedBeforeRotation：JPEG EXIF Orientation
// 必须先归一化再执行用户旋转。物理 3×2（顶行白、底行黑）+ Orientation 6
// → 逻辑 2×3（左列黑、右列白），再逆时针 90° → 输出 3×2 顶行白、底行黑，
// 与物理排列一致；若先对物理像素旋转再解释 EXIF，输出方向就会颠倒。
func TestRotateFileJPEGEXIFAppliedBeforeRotation(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "oriented.jpg")
	if err := os.WriteFile(input, orientedJPEG(t, 6, 3, 2), 0o600); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(dir, "oriented_rotated.jpg")
	if err := RotateFile(input, output, Options{Angle: 90}); err != nil {
		t.Fatalf("RotateFile: %v", err)
	}

	img, err := imageio.OpenStatic(output)
	if err != nil {
		t.Fatalf("open output: %v", err)
	}
	if got := img.Bounds(); got != image.Rect(0, 0, 3, 2) {
		t.Fatalf("bounds = %v, want 3x2 (logical 2x3 rotated 90°)", got)
	}
	rowLuma := func(y int) float64 {
		sum := 0.0
		for x := range 3 {
			r, g, b, _ := img.At(x, y).RGBA()
			sum += (float64(r>>8)*0.299 + float64(g>>8)*0.587 + float64(b>>8)*0.114) / 3
		}
		return sum
	}
	if top, bottom := rowLuma(0), rowLuma(1); top < 200 || bottom > 50 {
		t.Fatalf("row luminance after EXIF+rotate: top=%.1f bottom=%.1f, want top bright / bottom dark", top, bottom)
	}
}

// orientedJPEG 生成携带指定 EXIF Orientation 的真实 JPEG：
// APP1(Exif) 插在 SOI 之后，顶行白、底行黑。
func orientedJPEG(t *testing.T, orientation uint16, width, height int) []byte {
	t.Helper()

	phys := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			v := uint8(255)
			if y == height-1 {
				v = 0
			}
			phys.SetNRGBA(x, y, color.NRGBA{R: v, G: v, B: v, A: 255})
		}
	}

	var tiff bytes.Buffer
	tiff.WriteString("II")
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(42))
	_ = binary.Write(&tiff, binary.LittleEndian, uint32(8))
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(1))
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(0x0112))
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(3))
	_ = binary.Write(&tiff, binary.LittleEndian, uint32(1))
	_ = binary.Write(&tiff, binary.LittleEndian, orientation)
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(0))
	_ = binary.Write(&tiff, binary.LittleEndian, uint32(0))

	var body bytes.Buffer
	if err := jpeg.Encode(&body, phys, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	encoded := body.Bytes()

	var out bytes.Buffer
	out.Write(encoded[:2])
	out.WriteByte(0xFF)
	out.WriteByte(0xE1)
	_ = binary.Write(&out, binary.BigEndian, uint16(2+6+tiff.Len()))
	out.WriteString("Exif\x00\x00")
	out.Write(tiff.Bytes())
	out.Write(encoded[2:])
	return out.Bytes()
}
