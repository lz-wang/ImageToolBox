package httpapi

import (
	"errors"
	"image"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"imagetoolbox/internal/compress"
	"imagetoolbox/internal/imageio"
)

func TestNewMultipartContract(t *testing.T) {
	h := mustNew(t, Config{NoAuth: true})
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
				if got := w.Header().Get("X-ITB-Operation"); got != "resize" {
					t.Fatalf("X-ITB-Operation = %q, want resize", got)
				}
				if got := w.Header().Get("X-ITB-Input-Size"); got == "" {
					t.Fatal("X-ITB-Input-Size is empty")
				}
				if got := w.Header().Get("X-ITB-Output-Size"); got == "" {
					t.Fatal("X-ITB-Output-Size is empty")
				}
			}
		})
	}
}

func TestAuthenticationAndLimits(t *testing.T) {
	input := formFile{field: "input", filename: "a.png", content: testPNG(t, 32, 16)}
	t.Run("health does not require authentication", func(t *testing.T) {
		w := httptest.NewRecorder()
		mustNew(t, Config{Token: "secret"}).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})
	t.Run("image operations require a bearer token", func(t *testing.T) {
		h := mustNew(t, Config{Token: "secret"})
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
	t.Run("errors use the stable structured contract", func(t *testing.T) {
		h := mustNew(t, Config{NoAuth: true})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, newMultipartRequest(t, http.MethodPost, "/api/v1/resize", nil))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		if code := decodeJSONError(t, w.Body.Bytes()); code != "missing_input" {
			t.Fatalf("error code = %q, want missing_input", code)
		}
	})
	t.Run("oversized dimensions are rejected", func(t *testing.T) {
		h := mustNew(t, Config{NoAuth: true, MaxDimension: 16})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, newMultipartRequest(t, http.MethodPost, "/api/v1/resize", map[string]string{"width": "16"}, input))
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusRequestEntityTooLarge, w.Body.String())
		}
	})
	t.Run("oversized multipart request is rejected", func(t *testing.T) {
		h := mustNew(t, Config{NoAuth: true, MaxUpload: 128})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, newMultipartRequest(t, http.MethodPost, "/api/v1/resize", map[string]string{"padding": strings.Repeat("x", 512)}, input))
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusRequestEntityTooLarge, w.Body.String())
		}
	})
}

func TestConfigNormalizeAndValidate(t *testing.T) {
	t.Run("normalize fills zero values with defaults", func(t *testing.T) {
		cfg := Config{}
		cfg.Normalize()
		if cfg.MaxUpload != DefaultMaxUpload {
			t.Fatalf("MaxUpload = %d, want %d", cfg.MaxUpload, DefaultMaxUpload)
		}
		if cfg.MaxPixels != DefaultMaxPixels {
			t.Fatalf("MaxPixels = %d, want %d", cfg.MaxPixels, DefaultMaxPixels)
		}
		if cfg.MaxDimension != DefaultMaxDimension {
			t.Fatalf("MaxDimension = %d, want %d", cfg.MaxDimension, DefaultMaxDimension)
		}
		if cfg.MaxConcurrent != DefaultMaxConcurrent {
			t.Fatalf("MaxConcurrent = %d, want %d", cfg.MaxConcurrent, DefaultMaxConcurrent)
		}
		if cfg.Timeout != DefaultTimeout {
			t.Fatalf("Timeout = %v, want %v", cfg.Timeout, DefaultTimeout)
		}
		if cfg.Logger == nil {
			t.Fatal("Logger must default to slog.Default()")
		}
	})
	t.Run("explicit valid limits pass", func(t *testing.T) {
		cfg := Config{MaxUpload: 1 << 20, MaxPixels: 1000, MaxDimension: 64, MaxConcurrent: 1, Timeout: time.Second}
		cfg.Normalize()
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
		if cfg.MaxPixels != 1000 {
			t.Fatalf("Normalize must keep explicit MaxPixels, got %d", cfg.MaxPixels)
		}
	})
	invalid := []struct {
		name string
		cfg  Config
	}{
		{name: "negative max concurrent", cfg: Config{MaxConcurrent: -1}},
		{name: "negative max pixels", cfg: Config{MaxPixels: -1}},
		{name: "negative max dimension", cfg: Config{MaxDimension: -1}},
		{name: "negative max upload", cfg: Config{MaxUpload: -1}},
		{name: "negative timeout", cfg: Config{Timeout: -time.Second}},
		{name: "zero timeout after normalize", cfg: Config{}},
	}
	for _, tt := range invalid[:len(invalid)-1] {
		t.Run(tt.name+" returns error without panic", func(t *testing.T) {
			tt.cfg.Normalize()
			if err := tt.cfg.Validate(); err == nil {
				t.Fatalf("Validate(%+v) = nil, want error", tt.cfg)
			}
			if _, err := New(tt.cfg); err == nil {
				t.Fatalf("New(%+v) = nil error, want error", tt.cfg)
			}
		})
	}
}

func TestMultipartFileIsolation(t *testing.T) {
	t.Run("identical input and watermark filenames do not collide", func(t *testing.T) {
		h := mustNew(t, Config{NoAuth: true})
		req := newMultipartRequest(t, http.MethodPost, "/api/v1/watermark",
			map[string]string{"scale": "0.5"},
			formFile{field: "input", filename: "logo.png", content: testPNG(t, 32, 16)},
			formFile{field: "image", filename: "logo.png", content: testPNG(t, 8, 8)})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
		}
		img := decodePNG(t, w.Body.Bytes())
		if img.Bounds() != image.Rect(0, 0, 32, 16) {
			t.Fatalf("bounds = %v, want 32x16", img.Bounds())
		}
		if got := w.Header().Get("Content-Disposition"); !strings.Contains(got, "logo_watermarked.png") {
			t.Fatalf("Content-Disposition = %q, want logo_watermarked.png", got)
		}
	})

	t.Run("compress input named output.png keeps input and output distinct", func(t *testing.T) {
		if _, err := compress.EnsureBinary(compress.PngQuant); err != nil {
			t.Skipf("native compression binaries unavailable: %v", err)
		}
		h := mustNew(t, Config{NoAuth: true})
		req := newMultipartRequest(t, http.MethodPost, "/api/v1/compress",
			map[string]string{"quality": "80"},
			formFile{field: "input", filename: "output.png", content: testPNG(t, 64, 64)})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
		}
		// 输出必须是有效 PNG：若输入输出同路径会互相覆盖产生损坏数据。
		decodePNG(t, w.Body.Bytes())
		if got := w.Header().Get("Content-Disposition"); !strings.Contains(got, "output.png") {
			t.Fatalf("Content-Disposition = %q, want output.png", got)
		}
	})

	t.Run("path traversal and windows filenames are sanitized", func(t *testing.T) {
		h := mustNew(t, Config{NoAuth: true})
		for _, filename := range []string{"../../foo.png", `C:\temp\foo.png`} {
			t.Run(filename, func(t *testing.T) {
				req := newMultipartRequest(t, http.MethodPost, "/api/v1/resize",
					map[string]string{"width": "16"},
					formFile{field: "input", filename: filename, content: testPNG(t, 32, 16)})
				w := httptest.NewRecorder()
				h.ServeHTTP(w, req)
				if w.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
				}
				if got := w.Header().Get("Content-Disposition"); !strings.Contains(got, "foo_resized.png") {
					t.Fatalf("Content-Disposition = %q, want foo_resized.png", got)
				}
			})
		}
	})
}

func TestAdmitImageTypedErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.png")
	if err := os.WriteFile(path, testPNG(t, 32, 16), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}
	t.Run("oversized image wraps ErrImageTooLarge", func(t *testing.T) {
		err := admitImage(path, Config{MaxPixels: 100, MaxDimension: 8})
		if !errors.Is(err, ErrImageTooLarge) {
			t.Fatalf("admitImage error = %v, want ErrImageTooLarge", err)
		}
	})
	t.Run("unsupported input wraps imageio.ErrUnsupportedFormat", func(t *testing.T) {
		gifPath := filepath.Join(dir, "input.gif")
		// GIF 能被 image.DecodeConfig 识别，但不在 imageio 支持的编码集合内。
		if err := os.WriteFile(gifPath, testGIF(t, 32, 16), 0o600); err != nil {
			t.Fatalf("write gif: %v", err)
		}
		_, err := imageio.Probe(gifPath)
		if !errors.Is(err, imageio.ErrUnsupportedFormat) {
			t.Fatalf("Probe(gif) error = %v, want imageio.ErrUnsupportedFormat", err)
		}
	})
	t.Run("within limits passes", func(t *testing.T) {
		if err := admitImage(path, Config{MaxPixels: 1000, MaxDimension: 64}); err != nil {
			t.Fatalf("admitImage() = %v, want nil", err)
		}
	})
}
