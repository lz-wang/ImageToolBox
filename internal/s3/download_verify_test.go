package s3

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// assertNoLeftovers 断言输出目录没有残留的下载临时文件，
// 且目标文件不存在（失败路径不留下 partial 文件）。
func assertNoLeftovers(t *testing.T, outputDir, outputPath string) {
	t.Helper()

	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Errorf("output file should not exist after failed download, stat err = %v", err)
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("read output dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".itb-download-") {
			t.Errorf("temp file %q left behind after failed download", e.Name())
		}
	}
}

// TestDownloadVerifyMetadataMatch --verify 命中：单次 GET，文件落盘，
// 结果携带计算出的 SHA-256。
func TestDownloadVerifyMetadataMatch(t *testing.T) {
	rec, client := newDownloadObjectServer(t, []byte(helloContent), map[string]string{
		MetadataSHA256Key: helloSHA256,
	})
	output := filepath.Join(t.TempDir(), "hello.txt")

	result, err := Download(context.Background(), client, "hello.txt", output, &DownloadOptions{Verify: true})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if result.SHA256 != helloSHA256 {
		t.Errorf("result.SHA256 = %q, want %q", result.SHA256, helloSHA256)
	}
	assertMethods(t, rec.snapshotMethods(), []string{http.MethodGet})

	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(got) != helloContent {
		t.Errorf("output = %q, want %q", got, helloContent)
	}
}

// TestDownloadVerifyMetadataMismatch --verify 失败：ErrChecksumMismatch，
// 目标路径与临时文件都不留下。
func TestDownloadVerifyMetadataMismatch(t *testing.T) {
	_, client := newDownloadObjectServer(t, []byte(helloContent), map[string]string{
		MetadataSHA256Key: "deadbeef",
	})
	dir := t.TempDir()
	output := filepath.Join(dir, "hello.txt")

	_, err := Download(context.Background(), client, "hello.txt", output, &DownloadOptions{Verify: true})
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("got %v, want ErrChecksumMismatch", err)
	}
	assertNoLeftovers(t, dir, output)
}

// TestDownloadVerifyMissingMetadata 对象缺少 itb-sha256 metadata 时
// 无法校验，直接报错且不落盘。
func TestDownloadVerifyMissingMetadata(t *testing.T) {
	_, client := newDownloadObjectServer(t, []byte(helloContent), nil)
	dir := t.TempDir()
	output := filepath.Join(dir, "hello.txt")

	_, err := Download(context.Background(), client, "hello.txt", output, &DownloadOptions{Verify: true})
	if err == nil {
		t.Fatal("expected error for missing itb-sha256 metadata")
	}
	assertNoLeftovers(t, dir, output)
}

// TestDownloadVerifySHA256 --verify-sha256 提供 provider-neutral 校验。
func TestDownloadVerifySHA256(t *testing.T) {
	t.Run("哈希一致通过", func(t *testing.T) {
		_, client := newDownloadTestServer(t, []byte(helloContent))
		output := filepath.Join(t.TempDir(), "hello.txt")

		result, err := Download(context.Background(), client, "hello.txt", output, &DownloadOptions{VerifySHA256: helloSHA256})
		if err != nil {
			t.Fatalf("Download: %v", err)
		}
		if result.SHA256 != helloSHA256 {
			t.Errorf("result.SHA256 = %q, want %q", result.SHA256, helloSHA256)
		}
	})

	t.Run("哈希不一致报错且不落盘", func(t *testing.T) {
		_, client := newDownloadTestServer(t, []byte(helloContent))
		dir := t.TempDir()
		output := filepath.Join(dir, "hello.txt")

		_, err := Download(context.Background(), client, "hello.txt", output, &DownloadOptions{VerifySHA256: "0000"})
		if err == nil {
			t.Fatal("expected checksum error")
		}
		assertNoLeftovers(t, dir, output)
	})

	t.Run("非法哈希格式报参数错误", func(t *testing.T) {
		_, client := newDownloadTestServer(t, []byte(helloContent))
		dir := t.TempDir()
		output := filepath.Join(dir, "hello.txt")

		_, err := Download(context.Background(), client, "hello.txt", output, &DownloadOptions{VerifySHA256: "not-hex"})
		if err == nil || errors.Is(err, ErrChecksumMismatch) {
			t.Fatalf("expected format error, got %v", err)
		}
		assertNoLeftovers(t, dir, output)
	})

	t.Run("与 --verify 同用时两者都必须通过", func(t *testing.T) {
		_, client := newDownloadObjectServer(t, []byte(helloContent), map[string]string{
			MetadataSHA256Key: helloSHA256,
		})
		dir := t.TempDir()
		output := filepath.Join(dir, "hello.txt")

		if _, err := Download(context.Background(), client, "hello.txt", output, &DownloadOptions{Verify: true, VerifySHA256: helloSHA256}); err != nil {
			t.Fatalf("Download: %v", err)
		}

		output2 := filepath.Join(dir, "hello2.txt")
		_, err := Download(context.Background(), client, "hello.txt", output2, &DownloadOptions{Verify: true, VerifySHA256: "1111"})
		if !errors.Is(err, ErrChecksumMismatch) {
			t.Fatalf("got %v, want ErrChecksumMismatch", err)
		}
		assertNoLeftovers(t, dir, output2)
	})
}

// TestDownloadTruncationLeavesNoFile 传输中断（声明长度大于实际 body）
// 时目标路径与临时文件都不留下。
func TestDownloadTruncationLeavesNoFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		// 声明 1024 字节但只发送 5 字节：handler 返回后 Go 会关闭
		// 连接，客户端读到 unexpected EOF，模拟下载中途断流
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("short"))
	}))
	t.Cleanup(srv.Close)

	cfg := &Config{
		Endpoint: srv.URL, AccessKeyID: "ak", SecretAccessKey: "sk",
		Region: "us-east-1", Bucket: "test-bucket", ForcePathStyle: true,
	}
	client, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	dir := t.TempDir()
	output := filepath.Join(dir, "hello.txt")
	if _, err := Download(context.Background(), client, "hello.txt", output, nil); err == nil {
		t.Fatal("expected error for truncated body")
	}
	assertNoLeftovers(t, dir, output)
}
