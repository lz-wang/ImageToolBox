package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// compressFixturePNG 生成用于 CLI 测试的 PNG fixture（平滑渐变，
// 对 pngquant 质量门槛友好）。
func compressFixturePNG(t *testing.T, dir, name string) string {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			img.Set(x, y, color.NRGBA{R: uint8(x * 4), G: uint8(y * 4), B: uint8((x + y) * 2), A: 255})
		}
	}
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return path
}

// skipIfNoCompressors 真实压缩器未注入（本地无 bins）时跳过测试。
// --format json 模式下失败已被包装为机器错误（ErrReported），
// 需要从 stdout 的 itb.error.v1 文档中识别该情况。
func skipIfNoCompressors(t *testing.T, err error, stdout string) {
	t.Helper()

	if err == nil {
		return
	}
	if strings.Contains(err.Error(), "not initialized") {
		t.Skip("embedded compressors not available in this build")
	}
	if errors.Is(err, ErrReported) && strings.Contains(stdout, "not initialized") {
		t.Skip("embedded compressors not available in this build")
	}
}

// TestCompressJSONOutputContract 锁定 compress --format json 的 stdout 契约：
// 成功时恰为一份 itb.compress.v1 文档。
func TestCompressJSONOutputContract(t *testing.T) {
	dir := t.TempDir()
	input := compressFixturePNG(t, dir, "source.png")
	output := filepath.Join(dir, "out.png")

	var execOut, stderr bytes.Buffer
	var runErr error
	outputText := captureProcessStdout(t, func() {
		runErr = ExecuteArgs(context.Background(), "test", []string{
			"itb", "compress", "--format", "json", input, output,
		}, &execOut, &stderr)
	})
	skipIfNoCompressors(t, runErr, execOut.String())
	if runErr != nil {
		t.Fatalf("compress failed: %v\nstdout: %s\nstderr: %s", runErr, outputText, stderr.String())
	}

	var report struct {
		SchemaVersion string `json:"schema_version"`
		Input         struct {
			Path   string `json:"path"`
			Format string `json:"format"`
			Size   int64  `json:"size"`
			SHA256 string `json:"sha256"`
		} `json:"input"`
		Output struct {
			Path   string `json:"path"`
			Format string `json:"format"`
			Size   int64  `json:"size"`
			SHA256 string `json:"sha256"`
		} `json:"output"`
		Quality   int    `json:"quality"`
		Processor string `json:"processor"`
		ElapsedMs int64  `json:"elapsed_ms"`
	}
	if err := json.Unmarshal([]byte(outputText), &report); err != nil {
		t.Fatalf("decode compress JSON: %v\n%s", err, outputText)
	}
	if report.SchemaVersion != "itb.compress.v1" {
		t.Errorf("schema_version = %q", report.SchemaVersion)
	}
	if report.Input.Format != "png" || report.Output.Format != "png" {
		t.Errorf("formats = %q/%q", report.Input.Format, report.Output.Format)
	}
	if len(report.Input.SHA256) != 64 || len(report.Output.SHA256) != 64 {
		t.Errorf("sha256 lengths = %d/%d, want 64", len(report.Input.SHA256), len(report.Output.SHA256))
	}
	if report.Input.SHA256 == report.Output.SHA256 {
		t.Error("input and output hashes must differ for lossy png pipeline")
	}
	if report.Quality != 80 {
		t.Errorf("quality = %d, want default 80", report.Quality)
	}
	if report.Processor != "pngquant+oxipng" {
		t.Errorf("processor = %q", report.Processor)
	}
	if report.Output.Path != output {
		t.Errorf("output path = %q, want %q", report.Output.Path, output)
	}
	if report.ElapsedMs < 0 {
		t.Errorf("elapsed_ms = %d", report.ElapsedMs)
	}
}

// TestCompressJSONErrorNoPartialDestination 失败时目标不产生 partial 内容
//（安全提交契约的 CLI 级验证）。
func TestCompressJSONErrorNoPartialDestination(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "broken.png")
	broken := append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, bytes.Repeat([]byte{0xFF}, 128)...)
	if err := os.WriteFile(input, broken, 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}
	destination := filepath.Join(dir, "existing.png")
	if err := os.WriteFile(destination, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("seed destination: %v", err)
	}

	var stderr bytes.Buffer
	captureProcessStdout(t, func() {
		err := ExecuteArgs(context.Background(), "test", []string{
			"itb", "compress", "--format", "json", input, destination,
		}, io.Discard, &stderr)
		if err == nil {
			t.Error("expected failure for broken png")
		}
	})

	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(got) != "keep me" {
		t.Fatalf("destination content = %q, want original", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".itb-compress-") {
			t.Errorf("temp file left behind: %s", entry.Name())
		}
	}
}
