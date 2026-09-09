package compress

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"imagetoolbox/internal/filehash"
)

// leftoverTempFiles 列出目录内残留的 itb 安全提交临时文件。
func leftoverTempFiles(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	var leftovers []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".itb-compress-") {
			leftovers = append(leftovers, entry.Name())
		}
	}
	return leftovers
}

// gradientPNG 生成对 pngquant 质量门槛友好的平滑渐变 PNG。
func gradientPNG(t *testing.T, dir, name string) string {
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

func writeJPEGFixture(t *testing.T, dir, name string) string {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			img.Set(x, y, color.NRGBA{R: uint8(x * 4), G: uint8(y * 4), B: 128, A: 255})
		}
	}
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return path
}

// TestCommitOutputFailureLeavesDestinationIntact 确定性覆盖安全提交的
// 失败路径：压缩失败 → 目标保持原状、无临时文件残留。
func TestCommitOutputFailureLeavesDestinationIntact(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "out.png")
	original := []byte("previous content")
	if err := os.WriteFile(destination, original, 0o644); err != nil {
		t.Fatalf("seed destination: %v", err)
	}

	_, err := commitOutput(destination, "png", func(tmp *os.File) error {
		return errors.New("pngquant failed: synthetic pipeline error")
	})
	if err == nil || !strings.Contains(err.Error(), "pngquant failed") {
		t.Fatalf("err = %v, want pipeline error", err)
	}

	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(got) != string(original) {
		t.Fatalf("destination content = %q, want original %q", got, original)
	}
	if leftovers := leftoverTempFiles(t, dir); len(leftovers) != 0 {
		t.Fatalf("temp files left behind: %v", leftovers)
	}
}

// TestCommitOutputFormatValidationFails 输出格式校验失败同样不提交。
func TestCommitOutputFormatValidationFails(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "out.png")

	_, err := commitOutput(destination, "png", func(tmp *os.File) error {
		// 写入 JPEG 内容冒充 PNG 输出
		img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
		return jpeg.Encode(tmp, img, &jpeg.Options{Quality: 80})
	})
	if err == nil || !strings.Contains(err.Error(), "格式校验失败") {
		t.Fatalf("err = %v, want format validation failure", err)
	}
	if _, statErr := os.Stat(destination); !os.IsNotExist(statErr) {
		t.Fatalf("destination must not exist after failed commit, stat err = %v", statErr)
	}
	if leftovers := leftoverTempFiles(t, dir); len(leftovers) != 0 {
		t.Fatalf("temp files left behind: %v", leftovers)
	}
}

// TestCommitOutputSuccess 成功路径：目标写入、无残留。
func TestCommitOutputSuccess(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "out.png")

	info, err := commitOutput(destination, "png", func(tmp *os.File) error {
		img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
		return png.Encode(tmp, img)
	})
	if err != nil {
		t.Fatalf("commitOutput: %v", err)
	}
	if info.Size() == 0 {
		t.Error("committed file must not be empty")
	}
	if _, err := os.Stat(destination); err != nil {
		t.Fatalf("destination missing: %v", err)
	}
	if leftovers := leftoverTempFiles(t, dir); len(leftovers) != 0 {
		t.Fatalf("temp files left behind: %v", leftovers)
	}
}

// requireRealBinaries 守卫需要真实内嵌压缩器的测试：本地未注入
// bins 时优雅跳过，CI（已注入真实压缩器）必须真实执行。
func requireRealBinaries(t *testing.T, types ...BinaryType) {
	t.Helper()

	if len(types) == 0 {
		types = []BinaryType{PngQuant, OxiPng}
	}
	for _, binType := range types {
		if _, err := EnsureBinary(binType); err != nil {
			if strings.Contains(err.Error(), "not initialized") {
				t.Skipf("embedded compressor %s not available in this build", binType)
			}
			t.Fatalf("EnsureBinary(%s): %v", binType, err)
		}
	}
}

// TestNewReportShape 锁定 itb.compress.v1 报告的构造契约（纯函数）。
func TestNewReportShape(t *testing.T) {
	result := Result{
		Format:       "png",
		InputSize:    120000,
		OutputSize:   80000,
		InputSHA256:  "aaa",
		OutputSHA256: "bbb",
		Quality:      80,
		Processor:    ProcessorPNG,
		ElapsedMs:    123,
	}
	report := NewReport("source.png", "output.png", result)
	if report.SchemaVersion != "itb.compress.v1" {
		t.Errorf("schema_version = %q", report.SchemaVersion)
	}
	if report.Input.Path != "source.png" || report.Input.Format != "png" ||
		report.Input.Size != 120000 || report.Input.SHA256 != "aaa" {
		t.Errorf("input = %+v", report.Input)
	}
	if report.Output.Path != "output.png" || report.Output.Format != "png" ||
		report.Output.Size != 80000 || report.Output.SHA256 != "bbb" {
		t.Errorf("output = %+v", report.Output)
	}
	if report.Quality != 80 || report.Processor != "pngquant+oxipng" || report.ElapsedMs != 123 {
		t.Errorf("report = %+v", report)
	}
}

// TestCompressFileReport 端到端（真实管线）：报告字段与实际文件一致。
func TestCompressFileReport(t *testing.T) {
	requireRealBinaries(t)

	dir := t.TempDir()
	input := gradientPNG(t, dir, "photo.png")
	output := filepath.Join(dir, "photo_out.png")

	result, err := CompressFile(context.Background(), input, output, FileOptions{})
	if err != nil {
		t.Fatalf("CompressFile: %v", err)
	}

	if result.Format != "png" || result.Processor != ProcessorPNG {
		t.Fatalf("format/processor = %s/%s", result.Format, result.Processor)
	}
	if result.Quality != DefaultQuality {
		t.Errorf("quality = %d, want %d", result.Quality, DefaultQuality)
	}
	if result.ElapsedMs < 0 {
		t.Errorf("elapsed_ms = %d, must be >= 0", result.ElapsedMs)
	}

	// 摘要与磁盘内容一致
	inputSum, err := filehash.SumFile(input, []filehash.Algorithm{filehash.SHA256})
	if err != nil {
		t.Fatalf("hash input: %v", err)
	}
	outputSum, err := filehash.SumFile(output, []filehash.Algorithm{filehash.SHA256})
	if err != nil {
		t.Fatalf("hash output: %v", err)
	}
	if result.InputSHA256 != inputSum.Digests[filehash.SHA256] {
		t.Errorf("input sha256 mismatch: %q vs %q", result.InputSHA256, inputSum.Digests[filehash.SHA256])
	}
	if result.OutputSHA256 != outputSum.Digests[filehash.SHA256] {
		t.Errorf("output sha256 mismatch: %q vs %q", result.OutputSHA256, outputSum.Digests[filehash.SHA256])
	}
	if result.OutputSize != outputSum.BytesRead {
		t.Errorf("output size = %d, want %d", result.OutputSize, outputSum.BytesRead)
	}

	report := NewReport(input, output, result)
	if report.SchemaVersion != CompressSchemaVersion {
		t.Errorf("schema_version = %q, want %q", report.SchemaVersion, CompressSchemaVersion)
	}
	if report.Input.Path != input || report.Output.Path != output {
		t.Errorf("report paths = %q / %q", report.Input.Path, report.Output.Path)
	}
	if report.Input.SHA256 == "" || report.Output.SHA256 == "" {
		t.Errorf("report hashes must be populated: %+v", report)
	}

	if leftovers := leftoverTempFiles(t, dir); len(leftovers) != 0 {
		t.Fatalf("temp files left behind: %v", leftovers)
	}
}

// TestCompressFileJPEGProcessor JPEG 报告锁定 djpeg+cjpeg 处理器名。
func TestCompressFileJPEGProcessor(t *testing.T) {
	requireRealBinaries(t, DJpeg, CJpeg)

	dir := t.TempDir()
	input := writeJPEGFixture(t, dir, "photo.jpg")
	output := filepath.Join(dir, "photo_out.jpg")

	result, err := CompressFile(context.Background(), input, output, FileOptions{Quality: 75})
	if err != nil {
		t.Fatalf("CompressFile: %v", err)
	}
	if result.Format != "jpeg" || result.Processor != ProcessorJPEG {
		t.Fatalf("format/processor = %s/%s, want jpeg/%s", result.Format, result.Processor, ProcessorJPEG)
	}
	if result.Quality != 75 {
		t.Errorf("quality = %d, want 75", result.Quality)
	}
}

// TestCompressFileFailureKeepsExistingDestination 管线中途失败：
// 已存在的目标不被破坏，无临时残留。
func TestCompressFileFailureKeepsExistingDestination(t *testing.T) {
	dir := t.TempDir()
	// PNG magic + 非法 body：通过格式检测，但 pngquant 管线失败
	input := filepath.Join(dir, "broken.png")
	broken := append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, bytes.Repeat([]byte{0xFF}, 128)...)
	if err := os.WriteFile(input, broken, 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	destination := filepath.Join(dir, "out.png")
	original := []byte("untouched")
	if err := os.WriteFile(destination, original, 0o644); err != nil {
		t.Fatalf("seed destination: %v", err)
	}

	if _, err := CompressFile(context.Background(), input, destination, FileOptions{}); err == nil {
		t.Fatal("expected pipeline failure for broken png")
	}

	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(got) != string(original) {
		t.Fatalf("destination content = %q, want %q", got, original)
	}
	if leftovers := leftoverTempFiles(t, dir); len(leftovers) != 0 {
		t.Fatalf("temp files left behind: %v", leftovers)
	}
}
