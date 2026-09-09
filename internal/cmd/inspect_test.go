package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"imagetoolbox/internal/inspect"
)

// writeInspectPNG 生成最小 PNG fixture。
func writeInspectPNG(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "photo.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return path
}

// captureProcessStdout 重定向进程级 os.Stdout，返回 fn 期间的输出。
// inspect 等 Action 直接写 os.Stdout，ExecuteArgs 的 buffer 捕获不到。
func captureProcessStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	w.Close()
	os.Stdout = orig
	return <-done
}

// runInspectJSON 执行 inspect --format json 并返回解码后的结果。
func runInspectJSON(t *testing.T, args ...string) inspect.Result {
	t.Helper()

	full := append([]string{"itb", "inspect", "--format", "json"}, args...)
	var stderr bytes.Buffer
	output := captureProcessStdout(t, func() {
		if err := ExecuteArgs(context.Background(), "test", full, io.Discard, &stderr); err != nil {
			t.Errorf("inspect %v failed: %v\nstderr: %s", args, err, stderr.String())
		}
	})
	var result inspect.Result
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("decode inspect JSON: %v\n%s", err, output)
	}
	return result
}

// TestInspectDefaultHashesAll 默认（不带 --hash）计算全部四种算法，
// v2 契约形状不变。
func TestInspectDefaultHashesAll(t *testing.T) {
	path := writeInspectPNG(t)
	result := runInspectJSON(t, path)

	if result.Hashes == nil {
		t.Fatal("default must compute hashes")
	}
	for _, field := range []string{result.Hashes.SHA256, result.Hashes.SHA1, result.Hashes.MD5, result.Hashes.CRC32} {
		if field == "" {
			t.Fatalf("default hashes must all be present: %+v", result.Hashes)
		}
	}
}

// TestInspectSelectiveHashes --hash 只计算指定算法，未选中的省略。
func TestInspectSelectiveHashes(t *testing.T) {
	path := writeInspectPNG(t)
	result := runInspectJSON(t, "--hash", "sha256", "--hash", "crc32", path)

	if result.Hashes == nil {
		t.Fatal("hashes missing")
	}
	if result.Hashes.SHA256 == "" || result.Hashes.CRC32 == "" {
		t.Fatalf("selected algorithms must be computed: %+v", result.Hashes)
	}
	if result.Hashes.SHA1 != "" || result.Hashes.MD5 != "" {
		t.Fatalf("unselected algorithms must be omitted: %+v", result.Hashes)
	}
}

// TestInspectNoHashFlagsMutuallyExclusive --no-hash 与 --hash 互斥。
func TestInspectNoHashFlagsMutuallyExclusive(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := ExecuteArgs(context.Background(), "test", []string{
		"itb", "inspect", "--no-hash", "--hash", "sha256", "photo.png",
	}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected mutual exclusion error")
	}
	if !strings.Contains(err.Error(), "only one of") && !strings.Contains(err.Error(), "--hash") && !strings.Contains(err.Error(), "no-hash") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

// TestInspectUnknownHashAlgorithm 未知算法在任何处理之前报错。
func TestInspectUnknownHashAlgorithm(t *testing.T) {
	path := writeInspectPNG(t)
	var stdout, stderr bytes.Buffer
	err := ExecuteArgs(context.Background(), "test", []string{
		"itb", "inspect", "--hash", "sha512", path,
	}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected unknown algorithm error")
	}
	if !strings.Contains(err.Error(), "sha512") {
		t.Fatalf("error should name the algorithm, got: %v", err)
	}
}

// TestInspectPlainRequiresSHA256 plain 输出在 sha256 未计算时报错。
func TestInspectPlainRequiresSHA256(t *testing.T) {
	path := writeInspectPNG(t)

	var stdout, stderr bytes.Buffer
	err := ExecuteArgs(context.Background(), "test", []string{
		"itb", "inspect", "--hash", "crc32", "--format", "plain", path,
	}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("err = %v, want sha256 requirement error", err)
	}

	// --hash sha256 时 plain 输出 sha256
	output := captureProcessStdout(t, func() {
		if err := ExecuteArgs(context.Background(), "test", []string{
			"itb", "inspect", "--hash", "sha256", "--format", "plain", path,
		}, io.Discard, &stderr); err != nil {
			t.Errorf("plain with sha256 failed: %v", err)
		}
	})
	if len(strings.TrimSpace(output)) != 64 {
		t.Fatalf("plain output = %q, want 64-hex sha256", output)
	}
}
