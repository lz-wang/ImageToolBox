package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func testHandler(t *testing.T, _ map[string]string) http.Handler {
	t.Helper()
	return New(Config{})
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
