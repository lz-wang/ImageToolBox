package s3

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// TestRetryRecoversFromTransientErrors 前两次 500、第三次成功：
// SDK 标准 retryer（默认 MaxAttempts=3）自动恢复，调用方无感知。
func TestRetryRecoversFromTransientErrors(t *testing.T) {
	var gets atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gets.Add(1) <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `<Error><Code>InternalError</Code></Error>`)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(helloContent)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(helloContent))
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

	output := filepath.Join(t.TempDir(), "out.txt")
	result, err := Download(context.Background(), client, "hello.txt", output, nil)
	if err != nil {
		t.Fatalf("Download should recover via retries: %v", err)
	}
	if result.Size != int64(len(helloContent)) {
		t.Errorf("size = %d", result.Size)
	}
	if got := gets.Load(); got != 3 {
		t.Errorf("GET attempts = %d, want 3", got)
	}
}

// TestRetryMaxAttemptsOneFailsImmediately max-attempts=1：单次 500
// 直接失败，不重试。
func TestRetryMaxAttemptsOneFailsImmediately(t *testing.T) {
	var gets atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gets.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `<Error><Code>InternalError</Code></Error>`)
	}))
	t.Cleanup(srv.Close)

	cfg := &Config{
		Endpoint: srv.URL, AccessKeyID: "ak", SecretAccessKey: "sk",
		Region: "us-east-1", Bucket: "test-bucket", ForcePathStyle: true,
		MaxAttempts: 1,
	}
	client, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	output := filepath.Join(t.TempDir(), "out.txt")
	if _, err := Download(context.Background(), client, "hello.txt", output, nil); err == nil {
		t.Fatal("expected failure with max-attempts=1")
	}
	if got := gets.Load(); got != 1 {
		t.Errorf("GET attempts = %d, want exactly 1", got)
	}
}

// TestResponseHeaderTimeout 响应头延迟超过配置时快速失败。
func TestResponseHeaderTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cfg := &Config{
		Endpoint: srv.URL, AccessKeyID: "ak", SecretAccessKey: "sk",
		Region: "us-east-1", Bucket: "test-bucket", ForcePathStyle: true,
		MaxAttempts:           1,
		ResponseHeaderTimeout: 100 * time.Millisecond,
	}
	client, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	start := time.Now()
	output := filepath.Join(t.TempDir(), "out.txt")
	if _, err := Download(context.Background(), client, "hello.txt", output, nil); err == nil {
		t.Fatal("expected response header timeout")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("elapsed = %v, want ~100ms（超时未生效）", elapsed)
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Errorf("no file may remain, stat err = %v", statErr)
	}
}

// TestOperationTimeoutCancelsDownload 操作超时：下载中途取消，
// 不留临时文件。
func TestOperationTimeoutCancelsDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10485760")
		w.WriteHeader(http.StatusOK)
		// 慢速 body：先写一段然后挂起，让 context 超时打断 io.Copy
		_, _ = w.Write(make([]byte, 1024))
		time.Sleep(3 * time.Second)
	}))
	t.Cleanup(srv.Close)

	cfg := &Config{
		Endpoint: srv.URL, AccessKeyID: "ak", SecretAccessKey: "sk",
		Region: "us-east-1", Bucket: "test-bucket", ForcePathStyle: true,
		MaxAttempts: 1,
	}
	client, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	output := filepath.Join(t.TempDir(), "slow.bin")
	if _, err := Download(ctx, client, "slow.bin", output, nil); err == nil {
		t.Fatal("expected operation timeout")
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Errorf("timeout must not leave a partial file, stat err = %v", statErr)
	}
	// 临时文件清理
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(output), ".itb-download-*"))
	if len(leftovers) != 0 {
		t.Errorf("temp files left behind: %v", leftovers)
	}
}

// TestOperationTimeoutCleansUploadSnapshot 操作超时：上传失败路径
// 同样清理稳定快照。
func TestOperationTimeoutCleansUploadSnapshot(t *testing.T) {
	before := leftoverSnapshots(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cfg := &Config{
		Endpoint: srv.URL, AccessKeyID: "ak", SecretAccessKey: "sk",
		Region: "us-east-1", Bucket: "test-bucket", ForcePathStyle: true,
		MaxAttempts: 1,
	}
	client, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	path := writeUploadFixture(t)
	if _, err := Upload(ctx, client, path, "hello.txt", nil); err == nil {
		t.Fatal("expected operation timeout")
	}
	for snapshot := range leftoverSnapshots(t) {
		if !before[snapshot] {
			t.Errorf("snapshot left behind after timeout: %s", snapshot)
		}
	}
}
