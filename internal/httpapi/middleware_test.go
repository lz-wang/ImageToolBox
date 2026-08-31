package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   int
	}{
		{name: "exact bearer token", header: "Bearer secret", want: http.StatusOK},
		{name: "lowercase scheme", header: "bearer secret", want: http.StatusUnauthorized},
		{name: "scheme only", header: "Bearer", want: http.StatusUnauthorized},
		{name: "basic scheme", header: "Basic secret", want: http.StatusUnauthorized},
		{name: "trailing extra word", header: "Bearer secret extra", want: http.StatusUnauthorized},
		{name: "double space", header: "Bearer  secret", want: http.StatusUnauthorized},
		{name: "missing header", header: "", want: http.StatusUnauthorized},
		{name: "wrong token", header: "Bearer wrong", want: http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := mustNew(t, Config{Token: "secret"})
			req := newMultipartRequest(t, http.MethodPost, "/api/v1/resize",
				map[string]string{"width": "8"},
				formFile{field: "input", filename: "a.png", content: testPNG(t, 16, 8)})
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tt.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tt.want, w.Body.String())
			}
		})
	}
}

func TestConcurrencyLimiter(t *testing.T) {
	cfg := Config{NoAuth: true, MaxConcurrent: 1}
	cfg.Normalize()
	started := make(chan struct{})
	release := make(chan struct{})
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
	})
	h := protected(cfg, make(chan struct{}, 1), next)

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/resize", nil))
		done <- w
	}()
	<-started

	// 第一个请求占住唯一 slot，第二个请求应立即 429。
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, httptest.NewRequest(http.MethodPost, "/api/v1/resize", nil))
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", w2.Code, http.StatusTooManyRequests)
	}
	if got := w2.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After = %q, want 1", got)
	}
	if code := decodeJSONError(t, w2.Body.Bytes()); code != "busy" {
		t.Fatalf("error code = %q, want busy", code)
	}

	close(release)
	if w1 := <-done; w1.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want %d", w1.Code, http.StatusOK)
	}
}

func TestOperationTimeout(t *testing.T) {
	cfg := Config{NoAuth: true, Timeout: 20 * time.Millisecond}
	cfg.Normalize()
	op := func(ctx context.Context, _ form, _ string, _ Config) (string, string, int64, error) {
		<-ctx.Done()
		return "", "", 0, ctx.Err()
	}
	h := protected(cfg, make(chan struct{}, 1), imageHandler(cfg, "test", op))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, newMultipartRequest(t, http.MethodPost, "/api/v1/resize", nil,
		formFile{field: "input", filename: "a.png", content: testPNG(t, 4, 4)}))
	if w.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusGatewayTimeout, w.Body.String())
	}
	if code := decodeJSONError(t, w.Body.Bytes()); code != "timeout" {
		t.Fatalf("error code = %q, want timeout", code)
	}
}

func TestRecoverMiddleware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	panicking := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("boom")
	})
	h := recoverMiddleware(logger, panicking)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/resize", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
	if code := decodeJSONError(t, w.Body.Bytes()); code != "internal_error" {
		t.Fatalf("error code = %q, want internal_error", code)
	}
	// panic 堆栈绝不进入响应体。
	if body := w.Body.String(); len(body) > 512 {
		t.Fatalf("response body unexpectedly large: %d bytes", len(body))
	}
}

func TestRouterErrors(t *testing.T) {
	h := mustNew(t, Config{NoAuth: true})
	t.Run("unknown route returns structured 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		if code := decodeJSONError(t, w.Body.Bytes()); code != "not_found" {
			t.Fatalf("error code = %q, want not_found", code)
		}
	})
	t.Run("wrong method returns structured 405 with Allow", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/resize", nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
		}
		if got := w.Header().Get("Allow"); got != http.MethodPost {
			t.Fatalf("Allow = %q, want POST", got)
		}
		if code := decodeJSONError(t, w.Body.Bytes()); code != "method_not_allowed" {
			t.Fatalf("error code = %q, want method_not_allowed", code)
		}
	})
	t.Run("health rejects non-GET", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/health", nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
		}
	})
}
