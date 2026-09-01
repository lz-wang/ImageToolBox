package cmd

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rotateTestPNG 生成 2×3 的非对称 PNG（每像素颜色不同），并写入临时目录。
func rotateTestPNG(t *testing.T) string {
	t.Helper()

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

	path := filepath.Join(t.TempDir(), "photo.png")
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// captureRotateRun 执行 rotate 并捕获 stdout（Action 使用 fmt.Printf）。
func captureRotateRun(t *testing.T, args ...string) (string, error) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w

	runErr := testApp().Run(context.Background(), append([]string{"itb", "rotate"}, args...))

	os.Stdout = orig
	w.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	return buf.String(), runErr
}

func TestRotateRequiresAngleFlag(t *testing.T) {
	err := runContract("rotate", "photo.png")
	if err == nil || !strings.Contains(err.Error(), "Required flag") {
		t.Fatalf("error = %v, want required flag error", err)
	}
}

func TestRotateSuccess(t *testing.T) {
	src := rotateTestPNG(t)
	dir := filepath.Dir(src)

	decoded := func(t *testing.T, path string) image.Image {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		return img
	}

	t.Run("省略 dst 输出 _rotated", func(t *testing.T) {
		out, err := captureRotateRun(t, "--angle", "90", src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "photo_rotated.png") {
			t.Fatalf("output missing default name: %s", out)
		}
		if _, err := os.Stat(filepath.Join(dir, "photo_rotated.png")); err != nil {
			t.Fatalf("default output not created: %v", err)
		}
	})

	t.Run("显式 dst 与 flag 后置于 operand", func(t *testing.T) {
		dst := filepath.Join(dir, "clockwise.png")
		if _, err := captureRotateRun(t, src, dst, "--angle", "-90"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		img := decoded(t, dst)
		if got := img.Bounds(); got != image.Rect(0, 0, 3, 2) {
			t.Fatalf("bounds = %v, want 3x2", got)
		}
		// 顺时针 90°：src 左下角 (0,2) → dst 左上角 (0,0)
		if _, _, _, a := img.At(0, 0).RGBA(); a != 0xFFFF {
			t.Fatalf("dst(0,0) should be opaque source pixel, got alpha %d", a)
		}
	})

	t.Run("90 度交换宽高且方向为逆时针", func(t *testing.T) {
		dst := filepath.Join(dir, "ccw.png")
		if _, err := captureRotateRun(t, "--angle", "90", src, dst); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		img := decoded(t, dst)
		if got := img.Bounds(); got != image.Rect(0, 0, 3, 2) {
			t.Fatalf("bounds = %v, want 3x2", got)
		}
		// 逆时针 90°：src 右上角 (1,0) [纯蓝] → dst 左上角 (0,0)
		r, g, b, _ := img.At(0, 0).RGBA()
		if r>>8 != 0 || g>>8 != 0 || b>>8 != 255 {
			t.Fatalf("dst(0,0) = (%d,%d,%d), want pure blue from src(1,0)", r>>8, g>>8, b>>8)
		}
	})

	t.Run("任意角度扩大画布且 PNG 保持透明", func(t *testing.T) {
		dst := filepath.Join(dir, "angle45.png")
		if _, err := captureRotateRun(t, "--angle", "45", src, dst); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		img := decoded(t, dst)
		if got := img.Bounds(); got.Dx() <= 2 || got.Dy() <= 3 {
			t.Fatalf("bounds = %v, want expanded canvas", got)
		}
		if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
			t.Fatalf("corner (0,0) alpha = %d, want 0", a)
		}
	})
}
