package s3

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

// helloContent 与其 SHA-256，作为上传 fixture 与 --skip-unchanged
// 比对测试的已知内容
const (
	helloContent = "hello\n"
	helloSHA256  = "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"
)

// requestRecorder 记录模拟服务器收到的请求方法序列与 PUT 携带的
// itb-sha256 metadata，用于协议级断言（HEAD/PUT 次数）。
type requestRecorder struct {
	mu        sync.Mutex
	methods   []string
	putSHA256 string
}

func (r *requestRecorder) record(method, putSHA256 string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.methods = append(r.methods, method)
	if method == http.MethodPut {
		r.putSHA256 = putSHA256
	}
}

func (r *requestRecorder) snapshotMethods() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.methods)
}

func (r *requestRecorder) recordedPutSHA256() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.putSHA256
}

// newUploadTestServer 启动模拟 S3 的 httptest 服务器并返回指向它的
// 客户端。headFn 为 nil 时 HEAD 返回 200，否则由 headFn 决定响应；
// PUT 一律返回 200。
func newUploadTestServer(t *testing.T, headFn func(w http.ResponseWriter)) (*requestRecorder, *Client) {
	t.Helper()

	rec := &requestRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var putSHA256 string
		switch r.Method {
		case http.MethodHead:
			if headFn != nil {
				headFn(w)
			} else {
				w.Header().Set("ETag", "\"etag\"")
				w.WriteHeader(http.StatusOK)
			}
		case http.MethodPut:
			putSHA256 = r.Header.Get("x-amz-meta-itb-sha256")
			w.Header().Set("ETag", "\"etag\"")
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		rec.record(r.Method, putSHA256)
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

// writeUploadFixture 写入内容为 helloContent 的临时文件
func writeUploadFixture(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "hello.txt")
	if err := os.WriteFile(path, []byte(helloContent), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func assertMethods(t *testing.T, got, want []string) {
	t.Helper()

	if !slices.Equal(got, want) {
		t.Errorf("request methods = %v, want %v", got, want)
	}
}

// TestUploadDefaultPutsWithoutHead 锁定默认上传的请求特征：
// 0 × HEAD + 1 × PUT，且 PUT 携带 itb-sha256 metadata。
// 同时锁定 UploadResult 契约与 domain 不写 stdout。
func TestUploadDefaultPutsWithoutHead(t *testing.T) {
	rec, client := newUploadTestServer(t, nil)
	path := writeUploadFixture(t)

	var result *UploadResult
	out := captureStdout(t, func() {
		var err error
		result, err = Upload(context.Background(), client, path, "hello.txt", nil)
		if err != nil {
			t.Errorf("Upload: %v", err)
		}
	})

	if out != "" {
		t.Errorf("domain must not write to stdout, got %q", out)
	}
	if result.Skipped {
		t.Error("default upload must not be skipped")
	}
	if result.Key != "hello.txt" {
		t.Errorf("result.Key = %q, want hello.txt", result.Key)
	}
	if result.Size != int64(len(helloContent)) {
		t.Errorf("result.Size = %d, want %d", result.Size, len(helloContent))
	}
	if result.SHA256 != helloSHA256 {
		t.Errorf("result.SHA256 = %q, want %q", result.SHA256, helloSHA256)
	}
	assertMethods(t, rec.snapshotMethods(), []string{http.MethodPut})

	if got := rec.recordedPutSHA256(); got != helloSHA256 {
		t.Errorf("PUT itb-sha256 = %q, want %q", got, helloSHA256)
	}
}

// TestUploadSkipExistingSkipsBeforeHash 锁定 --skip-existing 命中时的
// 协议特征：仅 1 × HEAD、0 × PUT，且在 hash 之前返回（0 字节本地读取）。
func TestUploadSkipExistingSkipsBeforeHash(t *testing.T) {
	rec, client := newUploadTestServer(t, nil)
	path := writeUploadFixture(t)

	result, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{SkipExisting: true})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if !result.Skipped {
		t.Fatal("expected upload to be skipped")
	}
	if result.Reason == "" {
		t.Error("skipped result must carry a reason")
	}
	assertMethods(t, rec.snapshotMethods(), []string{http.MethodHead})
}

// TestUploadSkipExistingUploadsWhenMissing 锁定 --skip-existing 未命中时
// 恰好 1 × HEAD + 1 × PUT，不出现重复 HEAD。
func TestUploadSkipExistingUploadsWhenMissing(t *testing.T) {
	rec, client := newUploadTestServer(t, func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusNotFound)
	})
	path := writeUploadFixture(t)

	result, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{SkipExisting: true})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if result.Skipped {
		t.Error("missing remote object must not be skipped")
	}
	assertMethods(t, rec.snapshotMethods(), []string{http.MethodHead, http.MethodPut})
}

// TestUploadSkipUnchangedSkipsWhenHashMatches 锁定 --skip-unchanged 命中时
// 恰好 1 × HEAD、0 × PUT。
func TestUploadSkipUnchangedSkipsWhenHashMatches(t *testing.T) {
	rec, client := newUploadTestServer(t, func(w http.ResponseWriter) {
		w.Header().Set("x-amz-meta-itb-sha256", helloSHA256)
		w.Header().Set("ETag", "\"etag\"")
		w.WriteHeader(http.StatusOK)
	})
	path := writeUploadFixture(t)

	result, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{SkipUnchanged: true})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if !result.Skipped {
		t.Fatal("expected upload to be skipped")
	}
	assertMethods(t, rec.snapshotMethods(), []string{http.MethodHead})
}

// TestUploadSkipUnchangedUploadsWhenHashDiffers 锁定 --skip-unchanged
// 未命中时恰好 1 × HEAD + 1 × PUT。
func TestUploadSkipUnchangedUploadsWhenHashDiffers(t *testing.T) {
	rec, client := newUploadTestServer(t, func(w http.ResponseWriter) {
		w.Header().Set("x-amz-meta-itb-sha256", "different-hash")
		w.Header().Set("ETag", "\"etag\"")
		w.WriteHeader(http.StatusOK)
	})
	path := writeUploadFixture(t)

	result, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{SkipUnchanged: true})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if result.Skipped {
		t.Error("changed content must not be skipped")
	}
	assertMethods(t, rec.snapshotMethods(), []string{http.MethodHead, http.MethodPut})
}

// TestUploadMissingInputFile 输入文件不存在时立即报错且不发起任何请求。
func TestUploadMissingInputFile(t *testing.T) {
	rec, client := newUploadTestServer(t, nil)

	_, err := Upload(context.Background(), client, filepath.Join(t.TempDir(), "nope.bin"), "nope.bin", nil)
	if err == nil {
		t.Error("expected error for missing file")
	}
	assertMethods(t, rec.snapshotMethods(), nil)
}

func TestReaderSHA256(t *testing.T) {
	got, err := readerSHA256(strings.NewReader(helloContent))
	if err != nil {
		t.Fatalf("readerSHA256: %v", err)
	}
	if got != helloSHA256 {
		t.Errorf("readerSHA256 = %q, want %q", got, helloSHA256)
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestReaderSHA256ReadError(t *testing.T) {
	if _, err := readerSHA256(errorReader{}); err == nil {
		t.Error("expected error from failing reader")
	}
}

func TestIsUnchanged(t *testing.T) {
	newRemote := func(hash string, hasMetadata bool) *StatInfo {
		info := &StatInfo{Key: "k", Size: 1, ETag: "\"e\""}
		if hasMetadata {
			info.Metadata = map[string]string{MetadataSHA256Key: hash}
		}
		return info
	}

	tests := []struct {
		name        string
		remote      *StatInfo
		localSHA256 string
		want        bool
	}{
		{
			name:        "远端不存在时不判定 unchanged",
			remote:      nil,
			localSHA256: "aaa",
		},
		{
			name:        "远端缺少 metadata 时不判定 unchanged",
			remote:      newRemote("", false),
			localSHA256: "aaa",
		},
		{
			name:        "哈希一致时判定 unchanged",
			remote:      newRemote("aaa", true),
			localSHA256: "aaa",
			want:        true,
		},
		{
			name:        "哈希不一致时不判定 unchanged",
			remote:      newRemote("bbb", true),
			localSHA256: "aaa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUnchanged(tt.remote, tt.localSHA256); got != tt.want {
				t.Errorf("isUnchanged = %v, want %v", got, tt.want)
			}
		})
	}
}
