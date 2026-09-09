package s3

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"imagetoolbox/internal/filehash"
)

// newUploadBodyCaptureServer 在 newUploadTestServer 基础上记录 PUT body
// 全文，用于断言"itb-sha256 与实际上传 body 严格对应"。
func newUploadBodyCaptureServer(t *testing.T) (*requestRecorder, *[]byte, *Client) {
	t.Helper()

	rec := &requestRecorder{}
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			body = raw
			rec.recordPutHeaders(r.Header)
			w.Header().Set("ETag", "\"etag\"")
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		rec.record(r.Method, r.Header.Get("x-amz-meta-itb-sha256"))
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
	return rec, &body, client
}

// leftoverSnapshots 统计 os.TempDir 下当前存在的上传快照路径。
func leftoverSnapshots(t *testing.T) map[string]bool {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(os.TempDir(), "itb-upload-snapshot-*"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	set := make(map[string]bool, len(matches))
	for _, path := range matches {
		set[path] = true
	}
	return set
}

// assertNoNewSnapshots 断言 fn 执行期间没有新增的快照残留
//（不依赖全局 TempDir 的历史状态）。
func assertNoNewSnapshots(t *testing.T, before map[string]bool) {
	t.Helper()

	for path := range leftoverSnapshots(t) {
		if !before[path] {
			t.Errorf("snapshot left behind: %s", path)
		}
	}
}

// TestUploadBodyMatchesSHA256 协议级锁定 Commit 6 的核心正确性保证：
// PUT body 的 SHA-256 与 itb-sha256 metadata 完全一致（两者来自
// 同一次稳定读取）。
func TestUploadBodyMatchesSHA256(t *testing.T) {
	rec, body, client := newUploadBodyCaptureServer(t)
	path := writeUploadFixture(t)

	result, err := Upload(context.Background(), client, path, "hello.txt", nil)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if string(*body) != helloContent {
		t.Errorf("PUT body = %q, want %q", string(*body), helloContent)
	}
	if rec.recordedPutSHA256() != helloSHA256 || result.SHA256 != helloSHA256 {
		t.Errorf("sha256 metadata = %q, result = %q, want %q", rec.recordedPutSHA256(), result.SHA256, helloSHA256)
	}
}

// TestUploadCleansUpSnapshot 上传结束后快照必须被清理。
func TestUploadCleansUpSnapshot(t *testing.T) {
	before := leftoverSnapshots(t)
	_, client := newUploadTestServer(t, nil)
	path := writeUploadFixture(t)

	if _, err := Upload(context.Background(), client, path, "hello.txt", nil); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	assertNoNewSnapshots(t, before)

	// 失败路径同样清理：PUT 一直返回 500
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(failSrv.Close)
	failCfg := &Config{
		Endpoint: failSrv.URL, AccessKeyID: "ak", SecretAccessKey: "sk",
		Region: "us-east-1", Bucket: "test-bucket", ForcePathStyle: true,
	}
	clientFail, err := NewClient(context.Background(), failCfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := Upload(context.Background(), clientFail, path, "hello.txt", nil); err == nil {
		t.Fatal("expected PUT failure")
	}
	assertNoNewSnapshots(t, before)
}

// TestSnapshotSourceDetectsChangedSource 快照期间源文件发生可观察变化
// 时报 ErrSourceChanged 并清理快照。
func TestSnapshotSourceDetectsChangedSource(t *testing.T) {
	before := leftoverSnapshots(t)
	path := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer file.Close()
	initial, err := file.Stat()
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	// "复制期间"内容被替换（size/modtime 变化）
	if err := os.WriteFile(path, []byte("replaced with different content"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	snapshot, digest, err := snapshotSource(path, file, initial)
	if err == nil || !strings.Contains(err.Error(), filehash.ErrSourceChanged.Error()) {
		t.Fatalf("err = %v, want source-changed failure", err)
	}
	if snapshot != "" || digest != "" {
		t.Errorf("failed snapshot must return nothing, got %q / %q", snapshot, digest)
	}
	assertNoNewSnapshots(t, before)
}

// TestSnapshotSourceStableContent 稳定源：快照内容与摘要一一对应。
func TestSnapshotSourceStableContent(t *testing.T) {
	path := writeUploadFixture(t)

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer file.Close()
	initial, err := file.Stat()
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	snapshot, digest, err := snapshotSource(path, file, initial)
	if err != nil {
		t.Fatalf("snapshotSource: %v", err)
	}
	defer os.Remove(snapshot)
	if digest != helloSHA256 {
		t.Errorf("digest = %q, want %q", digest, helloSHA256)
	}
	got, err := os.ReadFile(snapshot)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if string(got) != helloContent {
		t.Errorf("snapshot content = %q, want %q", got, helloContent)
	}
	info, err := os.Stat(snapshot)
	if err != nil {
		t.Fatalf("stat snapshot: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("snapshot mode = %v, want 0600", info.Mode().Perm())
	}
}
