package server

import (
	"context"
	"net/http"
	"net/http/httptest"
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
