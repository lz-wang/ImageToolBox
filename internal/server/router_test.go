package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func testHandler(t *testing.T, files map[string]string) http.Handler {
	t.Helper()
	mapFS := fstest.MapFS{}
	for name, content := range files {
		mapFS[name] = &fstest.MapFile{Data: []byte(content)}
	}
	return New(mapFS).Handler()
}

func TestHealthRoute(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "health"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			testHandler(t, nil).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
			}
			if got := w.Body.String(); got != `{"status":"ok"}` {
				t.Fatalf("unexpected body: %s", got)
			}
		})
	}
}

func TestWebSPAFallback(t *testing.T) {
	handler := testHandler(t, map[string]string{
		"index.html": "<html>itb</html>",
		"app.js":     "console.log(1)",
	})

	tests := []struct {
		name        string
		path        string
		wantCode    int
		wantContent string
	}{
		{name: "根路径返回 index.html", path: "/", wantCode: 200, wantContent: "<html>itb</html>"},
		{name: "静态资源", path: "/app.js", wantCode: 200, wantContent: "console.log(1)"},
		{name: "未知前端路由回退 index.html", path: "/some/deep/route", wantCode: 200, wantContent: "<html>itb</html>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if w.Code != tt.wantCode {
				t.Fatalf("expected %d, got %d: %s", tt.wantCode, w.Code, w.Body.String())
			}
			if got := w.Body.String(); got != tt.wantContent {
				t.Fatalf("expected body %q, got %q", tt.wantContent, got)
			}
		})
	}
}

func TestAPIUnknownRouteReturnsJSON(t *testing.T) {
	w := httptest.NewRecorder()
	testHandler(t, nil).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if msg := decodeJSONError(t, w.Body.Bytes()); msg == "" {
		t.Fatal("expected JSON error body")
	}
}

func TestBatchRoutesRemoved(t *testing.T) {
	for _, route := range []string{"/api/v1/batch/resize", "/api/v1/batch/convert", "/api/v1/batch/watermark"} {
		t.Run(route, func(t *testing.T) {
			w := httptest.NewRecorder()
			testHandler(t, nil).ServeHTTP(w, httptest.NewRequest(http.MethodPost, route, nil))

			if w.Code != http.StatusNotFound {
				t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestWebWithoutBuildReturnsHint(t *testing.T) {
	w := httptest.NewRecorder()
	testHandler(t, nil).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if body := w.Body.String(); body == "" {
		t.Fatal("expected hint body")
	}
}
