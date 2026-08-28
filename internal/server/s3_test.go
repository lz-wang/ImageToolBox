package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// configureFakeS3 设置一组可通过校验的假 S3 环境变量（不发真实请求）。
func configureFakeS3(t *testing.T) {
	t.Helper()
	t.Setenv("ITB_S3_ENDPOINT", "http://127.0.0.1:9000")
	t.Setenv("ITB_S3_ACCESS_KEY_ID", "itb-test")
	t.Setenv("ITB_S3_SECRET_ACCESS_KEY", "itb-test")
	t.Setenv("ITB_S3_BUCKET", "itb-test")
}

func TestSharedS3ClientReused(t *testing.T) {
	configureFakeS3(t)

	s := New(nil)
	first, err := s.sharedS3Client(context.Background())
	if err != nil {
		t.Fatalf("sharedS3Client: %v", err)
	}
	second, err := s.sharedS3Client(context.Background())
	if err != nil {
		t.Fatalf("sharedS3Client: %v", err)
	}
	if first != second {
		t.Error("expected the same client instance to be reused across requests")
	}
}

func TestSharedS3ClientConcurrent(t *testing.T) {
	configureFakeS3(t)

	s := New(nil)

	var wg sync.WaitGroup
	results := make([]interface{}, 8)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client, err := s.sharedS3Client(context.Background())
			if err != nil {
				t.Errorf("sharedS3Client: %v", err)
				return
			}
			results[i] = client
		}()
	}
	wg.Wait()

	for i := 1; i < len(results); i++ {
		if results[i] == nil || results[0] == nil {
			t.Fatal("expected non-nil clients")
		}
		if results[i] != results[0] {
			t.Fatalf("concurrent calls returned different client instances (%d)", i)
		}
	}
}

func TestSharedS3ClientUnconfiguredNotCached(t *testing.T) {
	// 清空环境变量模拟未配置；失败不应被缓存，配置后下一次调用应成功
	t.Setenv("ITB_S3_ENDPOINT", "")
	t.Setenv("ITB_S3_ACCESS_KEY_ID", "")
	t.Setenv("ITB_S3_SECRET_ACCESS_KEY", "")
	t.Setenv("ITB_S3_BUCKET", "")

	s := New(nil)
	if _, err := s.sharedS3Client(context.Background()); err == nil {
		t.Fatal("expected error when S3 is not configured")
	}

	configureFakeS3(t)
	if _, err := s.sharedS3Client(context.Background()); err != nil {
		t.Fatalf("expected success after configuration, got: %v", err)
	}
}

func TestS3RoutesWithoutConfigReturn503(t *testing.T) {
	t.Setenv("ITB_S3_ENDPOINT", "")
	t.Setenv("ITB_S3_ACCESS_KEY_ID", "")
	t.Setenv("ITB_S3_SECRET_ACCESS_KEY", "")
	t.Setenv("ITB_S3_BUCKET", "")

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/v1/s3/objects"},
		{name: "stat", method: http.MethodGet, path: "/api/v1/s3/objects/info?key=foo"},
		{name: "delete", method: http.MethodDelete, path: "/api/v1/s3/objects?key=foo"},
		{name: "download", method: http.MethodGet, path: "/api/v1/s3/objects/download?key=foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			testHandler(t, nil).ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))

			if w.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
			}
			decodeJSONError(t, w.Body.Bytes())
		})
	}
}

// fakeS3Server 启动一个最小 S3 兼容 HTTP 服务，按 key 返回内容。
func fakeS3Server(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func configureS3Endpoint(t *testing.T, endpoint string) {
	t.Helper()
	t.Setenv("ITB_S3_ENDPOINT", endpoint)
	t.Setenv("ITB_S3_ACCESS_KEY_ID", "itb-test")
	t.Setenv("ITB_S3_SECRET_ACCESS_KEY", "itb-test")
	t.Setenv("ITB_S3_BUCKET", "itb-test")
}

func TestS3DownloadStreamsObjectBody(t *testing.T) {
	const body = "PNGDATA!"
	srv := fakeS3Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write([]byte(body))
	})
	configureS3Endpoint(t, srv.URL)

	w := httptest.NewRecorder()
	testHandler(t, nil).ServeHTTP(w, httptest.NewRequest(
		http.MethodGet, "/api/v1/s3/objects/download?key=images/foo.png", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != body {
		t.Errorf("body = %q, want %q", got, body)
	}
	if got := w.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", got)
	}
	if got := w.Header().Get("Content-Length"); got != strconv.Itoa(len(body)) {
		t.Errorf("Content-Length = %q, want %d", got, len(body))
	}
	if got := w.Header().Get("Content-Disposition"); !strings.Contains(got, "foo.png") {
		t.Errorf("Content-Disposition = %q, want foo.png attachment", got)
	}
}

func TestS3DownloadObjectNotFound(t *testing.T) {
	srv := fakeS3Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<Error><Code>NoSuchKey</Code><Message>The specified key does not exist.</Message><Key>images/foo.png</Key></Error>`))
	})
	configureS3Endpoint(t, srv.URL)

	w := httptest.NewRecorder()
	testHandler(t, nil).ServeHTTP(w, httptest.NewRequest(
		http.MethodGet, "/api/v1/s3/objects/download?key=images/foo.png", nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	decodeJSONError(t, w.Body.Bytes())
}
