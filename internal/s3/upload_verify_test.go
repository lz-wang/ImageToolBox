package s3

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// objectState 记录模拟 S3 存储的对象状态：PUT 写入、HEAD 回放，
// 用于 --verify 的 PUT → HEAD 往返测试。tamper 允许在 PUT 之后
// 篡改存储状态，模拟校验必须发现的远端不一致。
type objectState struct {
	mu     sync.Mutex
	exists bool
	size   int64
	header http.Header

	methods []string
	tamper  func(h http.Header, size *int64)
}

func (s *objectState) snapshotMethods() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.methods))
	copy(out, s.methods)
	return out
}

// headReplayHeaders 从 PUT 请求头提炼 HEAD 响应头：Content-Type、
// 标准 HTTP 头与 x-amz-meta-*（与真实 S3 的回读行为一致）。
func headReplayHeaders(put http.Header) http.Header {
	out := http.Header{}
	for _, name := range []string{
		"Content-Type", "Cache-Control", "Content-Disposition", "Content-Encoding",
	} {
		if vals := put.Values(name); len(vals) > 0 {
			for _, v := range vals {
				out.Add(name, v)
			}
		}
	}
	for name, vals := range put {
		if strings.HasPrefix(strings.ToLower(name), "x-amz-meta-") {
			for _, v := range vals {
				out.Add(name, v)
			}
		}
	}
	return out
}

// newUploadVerifyTestServer 启动有状态的模拟 S3：PUT 存储对象，
// HEAD 按存储状态回放（含 metadata 与标准头），支持 tamper 篡改。
func newUploadVerifyTestServer(t *testing.T) (*objectState, *Client) {
	t.Helper()

	state := &objectState{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state.mu.Lock()
		state.methods = append(state.methods, r.Method)
		switch r.Method {
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			state.size = int64(len(body))
			state.header = headReplayHeaders(r.Header)
			state.exists = true
			if state.tamper != nil {
				state.tamper(state.header, &state.size)
			}
		case http.MethodHead:
			if !state.exists {
				state.mu.Unlock()
				w.WriteHeader(http.StatusNotFound)
				return
			}
			for name, vals := range state.header {
				for _, v := range vals {
					w.Header().Add(name, v)
				}
			}
			w.Header().Set("Content-Length", strconv.FormatInt(state.size, 10))
			w.Header().Set("ETag", "\"etag\"")
		default:
			state.mu.Unlock()
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		state.mu.Unlock()
		w.WriteHeader(http.StatusOK)
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
	return state, client
}

// TestUploadVerifyPutsThenHeads 请求契约：verify 上传 = PUT → HEAD。
func TestUploadVerifyPutsThenHeads(t *testing.T) {
	state, client := newUploadVerifyTestServer(t)
	path := writeUploadFixture(t)

	if _, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{Verify: true}); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	assertMethods(t, state.snapshotMethods(), []string{http.MethodPut, http.MethodHead})
}

// TestUploadVerifyAfterSkipMiss 请求契约：skip-existing 未命中 + verify
// = HEAD → PUT → HEAD。
func TestUploadVerifyAfterSkipMiss(t *testing.T) {
	state, client := newUploadVerifyTestServer(t)
	path := writeUploadFixture(t)

	result, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{SkipExisting: true, Verify: true})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if result.Skipped {
		t.Fatal("missing remote object must not be skipped")
	}
	assertMethods(t, state.snapshotMethods(), []string{http.MethodHead, http.MethodPut, http.MethodHead})
}

// TestUploadVerifySkipExistingHitStaysSingleHead 请求契约：skip-existing
// 命中时 verify 不追加请求，仍只有 1 × HEAD。
func TestUploadVerifySkipExistingHitStaysSingleHead(t *testing.T) {
	state, client := newUploadVerifyTestServer(t)
	path := writeUploadFixture(t)

	// 预置同名对象
	if _, err := Upload(context.Background(), client, path, "hello.txt", nil); err != nil {
		t.Fatalf("seed upload: %v", err)
	}
	state.mu.Lock()
	state.methods = nil
	state.mu.Unlock()

	result, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{SkipExisting: true, Verify: true})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if !result.Skipped {
		t.Fatal("existing object must be skipped")
	}
	assertMethods(t, state.snapshotMethods(), []string{http.MethodHead})
}

// TestUploadVerifySkipUnchangedHitStaysSingleHead 请求契约：
// skip-unchanged 命中（itb-sha256 一致）时仍只有 1 × HEAD。
func TestUploadVerifySkipUnchangedHitStaysSingleHead(t *testing.T) {
	state, client := newUploadVerifyTestServer(t)
	path := writeUploadFixture(t)

	// 预置同内容对象（写入 itb-sha256）
	if _, err := Upload(context.Background(), client, path, "hello.txt", nil); err != nil {
		t.Fatalf("seed upload: %v", err)
	}
	state.mu.Lock()
	state.methods = nil
	state.mu.Unlock()

	result, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{SkipUnchanged: true, Verify: true})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if !result.Skipped {
		t.Fatal("unchanged object must be skipped")
	}
	assertMethods(t, state.snapshotMethods(), []string{http.MethodHead})
}

// TestUploadVerifyDetectsMismatch 篡改远端状态后 verify 必须报
// ErrVerifyFailed，并指明不一致的字段。
func TestUploadVerifyDetectsMismatch(t *testing.T) {
	tests := []struct {
		name       string
		tamper     func(h http.Header, size *int64)
		wantDetail string
	}{
		{
			name:       "Content-Type 不一致",
			tamper:     func(h http.Header, size *int64) { h.Set("Content-Type", "image/jpeg") },
			wantDetail: "content-type",
		},
		{
			name:       "Content-Length 不一致",
			tamper:     func(h http.Header, size *int64) { *size = *size + 1 },
			wantDetail: "content-length",
		},
		{
			name:       "Cache-Control 丢失",
			tamper:     func(h http.Header, size *int64) { h.Del("Cache-Control") },
			wantDetail: "cache-control",
		},
		{
			name:       "用户 metadata 丢失",
			tamper:     func(h http.Header, size *int64) { h.Del("x-amz-meta-width") },
			wantDetail: "metadata \"width\" missing",
		},
		{
			name:       "itb-sha256 metadata 不一致",
			tamper:     func(h http.Header, size *int64) { h.Set("x-amz-meta-itb-sha256", "deadbeef") },
			wantDetail: "metadata \"itb-sha256\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, client := newUploadVerifyTestServer(t)
			state.mu.Lock()
			state.tamper = tt.tamper
			state.mu.Unlock()
			path := writeUploadFixture(t)

			_, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{
				CacheControl: "no-cache",
				Metadata:     map[string]string{"width": "1920"},
				Verify:       true,
			})
			if !errors.Is(err, ErrVerifyFailed) {
				t.Fatalf("got %v, want ErrVerifyFailed", err)
			}
			if !strings.Contains(err.Error(), tt.wantDetail) {
				t.Errorf("error should mention %q, got: %v", tt.wantDetail, err)
			}
			assertMethods(t, state.snapshotMethods(), []string{http.MethodPut, http.MethodHead})
		})
	}
}

// TestUploadVerifyAcceptsExactRoundTrip 无篡改时完整属性集
//（含标准头与用户 metadata）全部通过校验。
func TestUploadVerifyAcceptsExactRoundTrip(t *testing.T) {
	_, client := newUploadVerifyTestServer(t)
	path := writeUploadFixture(t)

	result, err := Upload(context.Background(), client, path, "hello.txt", &UploadOptions{
		CacheControl:       "no-cache",
		ContentDisposition: "attachment",
		ContentEncoding:    "gzip",
		Metadata:           map[string]string{"width": "1920", "source-sha256": "abc"},
		Verify:             true,
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if result.Skipped {
		t.Fatal("upload must not be skipped")
	}
}
