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

// compareTestImages 在临时目录生成 192×192 的 src/dst PNG（dst 有确定性
// 小幅偏差），满足 MS-SSIM 的 161 像素最小短边。
func compareTestImages(t *testing.T) (srcPath, dstPath string) {
	t.Helper()

	makeImg := func(distort bool) *image.NRGBA {
		img := image.NewNRGBA(image.Rect(0, 0, 192, 192))
		for y := 0; y < 192; y++ {
			for x := 0; x < 192; x++ {
				r := uint8((x * 255) / 191)
				g := uint8((y * 255) / 191)
				b := uint8((x + y) % 256)
				if distort {
					r = uint8(min(int(r)+((x+y)%7)-3, 255))
				}
				img.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: 255})
			}
		}
		return img
	}

	dir := t.TempDir()
	write := func(name string, img image.Image) string {
		path := filepath.Join(dir, name)
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		return path
	}
	return write("src.png", makeImg(false)), write("dst.png", makeImg(true))
}

// captureStdoutRun 执行 compare 并捕获 stdout（Action 使用 fmt.Printf）。
func captureStdoutRun(t *testing.T, args ...string) (string, error) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w

	runErr := testApp().Run(context.Background(), append([]string{"itb", "compare"}, args...))

	os.Stdout = orig
	w.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	return buf.String(), runErr
}

// hasLine 报告输出中是否存在以 prefix 开头的行（"SSIM:" 不会被
// "MS-SSIM:" 行误匹配）。
func hasLine(out, prefix string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func TestCompareMetricSelection(t *testing.T) {
	src, dst := compareTestImages(t)

	tests := []struct {
		name     string
		args     []string
		wantLine []string
		banned   []string
	}{
		{"默认输出 PSNR 与 MS-SSIM", []string{src, dst}, []string{"PSNR:", "MS-SSIM:"}, []string{"SSIM:"}},
		{"仅 PSNR", []string{src, dst, "--psnr"}, []string{"PSNR:"}, []string{"SSIM:", "MS-SSIM:"}},
		{"仅 SSIM", []string{src, dst, "--ssim"}, []string{"SSIM:"}, []string{"PSNR:", "MS-SSIM:"}},
		{"仅 MS-SSIM", []string{src, dst, "--ms-ssim"}, []string{"MS-SSIM:"}, []string{"PSNR:", "SSIM:"}},
		{"全部指标", []string{src, dst, "--psnr", "--ssim", "--ms-ssim"}, []string{"PSNR:", "SSIM:", "MS-SSIM:"}, nil},
		{"flag 前置于 operand", []string{"--ssim", src, dst}, []string{"SSIM:"}, []string{"PSNR:", "MS-SSIM:"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := captureStdoutRun(t, tt.args...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, want := range tt.wantLine {
				if !hasLine(out, want) {
					t.Errorf("output missing %q\n--- got ---\n%s", want, out)
				}
			}
			for _, banned := range tt.banned {
				if hasLine(out, banned) {
					t.Errorf("output should not contain %q\n--- got ---\n%s", banned, out)
				}
			}
		})
	}
}

// 输出顺序固定为 PSNR、SSIM、MS-SSIM，与 flag 出现顺序无关。
func TestCompareOutputOrder(t *testing.T) {
	src, dst := compareTestImages(t)

	out, err := captureStdoutRun(t, src, dst, "--ms-ssim", "--ssim", "--psnr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"PSNR:", "SSIM:", "MS-SSIM:"}
	if len(lines) != len(want) {
		t.Fatalf("got %d output lines, want %d:\n%s", len(lines), len(want), out)
	}
	for i, prefix := range want {
		if !strings.HasPrefix(lines[i], prefix) {
			t.Fatalf("line %d = %q, want prefix %q", i, lines[i], prefix)
		}
	}
}

func TestCompareExplicitAllDisabled(t *testing.T) {
	src, dst := compareTestImages(t)

	_, err := captureStdoutRun(t, src, dst, "--psnr=false")
	if err == nil || !strings.Contains(err.Error(), "至少需要选择一个比较指标") {
		t.Fatalf("error = %v, want explicit metric selection error", err)
	}
}

func TestComparePositionalArgs(t *testing.T) {
	src, dst := compareTestImages(t)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"缺少 dst", []string{src}, "需要提供 <src> <dst>"},
		{"三个路径", []string{src, dst, src}, "需要提供 <src> <dst>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := captureStdoutRun(t, tt.args...)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
