package httpapi

import (
	"image"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewMultipartContract(t *testing.T) {
	h := New(Config{NoAuth: true})
	tests := []struct {
		name   string
		path   string
		fields map[string]string
		file   formFile
		want   int
	}{
		{name: "health", path: "/api/v1/health", want: http.StatusOK},
		{name: "resize direct fields", path: "/api/v1/resize", fields: map[string]string{"width": "16", "height": "8", "mode": "fit"}, file: formFile{field: "input", filename: "a.png", content: testPNG(t, 32, 16)}, want: http.StatusOK},
		{name: "legacy file rejected", path: "/api/v1/resize", fields: map[string]string{"width": "16"}, file: formFile{field: "file", filename: "a.png", content: testPNG(t, 32, 16)}, want: http.StatusBadRequest},
		{name: "legacy JSON field rejected", path: "/api/v1/resize", fields: map[string]string{"op" + "tions": `{"width":16}`}, file: formFile{field: "input", filename: "a.png", content: testPNG(t, 32, 16)}, want: http.StatusBadRequest},
		{name: "unknown rejected", path: "/api/v1/resize", fields: map[string]string{"width": "16", "widht": "16"}, file: formFile{field: "input", filename: "a.png", content: testPNG(t, 32, 16)}, want: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.name == "health" {
				req = httptest.NewRequest(http.MethodGet, tt.path, nil)
			} else {
				req = newMultipartRequest(t, http.MethodPost, tt.path, tt.fields, tt.file)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tt.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tt.want, w.Body.String())
			}
			if tt.want == http.StatusOK && tt.name != "health" {
				img := decodePNG(t, w.Body.Bytes())
				if img.Bounds() != image.Rect(0, 0, 16, 8) {
					t.Fatalf("bounds = %v", img.Bounds())
				}
			}
		})
	}
}

func TestAuthenticationAndLimits(t *testing.T) {
	input := formFile{field: "input", filename: "a.png", content: testPNG(t, 32, 16)}
	t.Run("health does not require authentication", func(t *testing.T) {
		w := httptest.NewRecorder()
		New(Config{Token: "secret"}).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})
	t.Run("image operations require a bearer token", func(t *testing.T) {
		h := New(Config{Token: "secret"})
		for _, tt := range []struct {
			name  string
			token string
			want  int
		}{
			{name: "missing", want: http.StatusUnauthorized},
			{name: "wrong", token: "wrong", want: http.StatusUnauthorized},
			{name: "correct", token: "secret", want: http.StatusOK},
		} {
			t.Run(tt.name, func(t *testing.T) {
				req := newMultipartRequest(t, http.MethodPost, "/api/v1/resize", map[string]string{"width": "16"}, input)
				if tt.token != "" {
					req.Header.Set("Authorization", "Bearer "+tt.token)
				}
				w := httptest.NewRecorder()
				h.ServeHTTP(w, req)
				if w.Code != tt.want {
					t.Fatalf("status = %d, want %d: %s", w.Code, tt.want, w.Body.String())
				}
			})
		}
	})
	t.Run("oversized dimensions are rejected", func(t *testing.T) {
		h := New(Config{NoAuth: true, MaxDimension: 16})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, newMultipartRequest(t, http.MethodPost, "/api/v1/resize", map[string]string{"width": "16"}, input))
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusRequestEntityTooLarge, w.Body.String())
		}
	})
	t.Run("oversized multipart request is rejected", func(t *testing.T) {
		h := New(Config{NoAuth: true, MaxUpload: 128})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, newMultipartRequest(t, http.MethodPost, "/api/v1/resize", map[string]string{"padding": strings.Repeat("x", 512)}, input))
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusRequestEntityTooLarge, w.Body.String())
		}
	})
}
