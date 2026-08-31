package s3

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// newDownloadTestServer 启动模拟 S3 GET 的 httptest 服务器：
// 始终恰好 1 × GET、0 × HEAD，body 由调用方给定，
// Content-Length 与 body 实际长度一致（用于进度提示断言）。
func newDownloadTestServer(t *testing.T, body []byte) (*requestRecorder, *Client) {
	t.Helper()

	rec := &requestRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Length", strconv.FormatInt(int64(len(body)), 10))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		rec.record(r.Method, "")
	}))
	t.Cleanup(srv.Close)

	cfg := &Config{
		Endpoint:        srv.URL,
		AccessKeyID:     "test-access-key",
		SecretAccessKey: "test-secret-key",
		Region:          "us-east-1",
		Bucket:          "test-bucket",
		ForcePathStyle:  true,
	}
	client, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return rec, client
}

// captureStdout 把进程 stdout 重定向到管道并返回 fn 执行期间的输出，
// 用于锁定 domain 层不向 stdout 打印的契约。
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()

	fn()

	w.Close()
	os.Stdout = orig
	return <-done
}

// TestDownloadSingleGetNoHead 锁定下载的请求特征：恰好 1 × GET、
// 0 × HEAD，结果携带 key / output_path / 实际写入字节数。
func TestDownloadSingleGetNoHead(t *testing.T) {
	rec, client := newDownloadTestServer(t, []byte(helloContent))
	output := filepath.Join(t.TempDir(), "out", "hello.txt")

	var result *DownloadResult
	out := captureStdout(t, func() {
		var err error
		result, err = Download(context.Background(), client, "hello.txt", output, nil)
		if err != nil {
			t.Errorf("Download: %v", err)
		}
	})

	if out != "" {
		t.Errorf("domain must not write to stdout, got %q", out)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Key != "hello.txt" || result.OutputPath != output {
		t.Errorf("result = %+v", result)
	}
	if result.Size != int64(len(helloContent)) {
		t.Errorf("result.Size = %d, want %d", result.Size, len(helloContent))
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

// TestDownloadGetFailureLeavesNoFile GET 失败时不创建输出文件。
func TestDownloadGetFailureLeavesNoFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<Error><Code>NoSuchKey</Code></Error>`))
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
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

	output := filepath.Join(t.TempDir(), "missing.txt")
	if _, err := Download(context.Background(), client, "missing.txt", output, nil); err == nil {
		t.Fatal("expected error for missing object")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Errorf("output file should not exist after failed GET, stat err = %v", err)
	}
}

// TestDownloadLargeFileProgressHint 大对象（>5MB 声明大小）时进度提示
// 写入 Progress writer，而不是 stdout。
func TestDownloadLargeFileProgressHint(t *testing.T) {
	_, client := newDownloadTestServer(t, bytes.Repeat([]byte{0}, 6*1024*1024))

	var progress strings.Builder
	out := captureStdout(t, func() {
		_, err := Download(context.Background(), client, "big.bin", filepath.Join(t.TempDir(), "big.bin"), &DownloadOptions{Progress: &progress})
		if err != nil {
			t.Errorf("Download: %v", err)
		}
	})

	if out != "" {
		t.Errorf("domain must not write to stdout, got %q", out)
	}
	if !strings.Contains(progress.String(), "Downloading (") {
		t.Errorf("progress writer got %q, want download hint", progress.String())
	}
}

// TestDownloadMissingKey 空对象键直接报错且不发起任何请求。
func TestDownloadMissingKey(t *testing.T) {
	rec, client := newDownloadTestServer(t, []byte(helloContent))

	if _, err := Download(context.Background(), client, "", filepath.Join(t.TempDir(), "x"), nil); err == nil {
		t.Error("expected ErrMissingKey")
	}
	assertMethods(t, rec.snapshotMethods(), nil)
}
