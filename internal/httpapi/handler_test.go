package httpapi

import (
	"encoding/json"
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
		if cfg.MaxWorkingBytes != DefaultMaxWorkingBytes {
			t.Fatalf("MaxWorkingBytes = %d, want %d", cfg.MaxWorkingBytes, DefaultMaxWorkingBytes)
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
		{name: "negative max working bytes", cfg: Config{MaxWorkingBytes: -1}},
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

func TestOperationAdmission(t *testing.T) {
	post := func(t *testing.T, path string, fields map[string]string, files ...formFile) *httptest.ResponseRecorder {
		t.Helper()
		h := mustNew(t, Config{NoAuth: true})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, newMultipartRequest(t, http.MethodPost, path, fields, files...))
		return w
	}
	input := func(width, height int) formFile {
		return formFile{field: "input", filename: "a.png", content: testPNG(t, width, height)}
	}

	t.Run("resize target beyond limits is rejected", func(t *testing.T) {
		w := post(t, "/api/v1/resize", map[string]string{"width": "100000", "height": "100000"}, input(1, 1))
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusRequestEntityTooLarge, w.Body.String())
		}
		if code := decodeJSONError(t, w.Body.Bytes()); code != "image_too_large" {
			t.Fatalf("error code = %q, want image_too_large", code)
		}
	})
	t.Run("percent upscale beyond limits is rejected", func(t *testing.T) {
		w := post(t, "/api/v1/resize", map[string]string{"percent": "1000000%"}, input(100, 100))
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusRequestEntityTooLarge, w.Body.String())
		}
	})
	t.Run("normal resize still succeeds", func(t *testing.T) {
		w := post(t, "/api/v1/resize", map[string]string{"width": "16"}, input(32, 16))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
		}
		if got := decodePNG(t, w.Body.Bytes()).Bounds(); got != image.Rect(0, 0, 16, 8) {
			t.Fatalf("bounds = %v, want 16x8", got)
		}
	})
	t.Run("huge watermark image is rejected", func(t *testing.T) {
		logo := formFile{field: "image", filename: "logo.png", content: pngHeader(t, 100000, 100000)}
		w := post(t, "/api/v1/watermark", map[string]string{"scale": "0.2"}, input(16, 16), logo)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusRequestEntityTooLarge, w.Body.String())
		}
		if code := decodeJSONError(t, w.Body.Bytes()); code != "image_too_large" {
			t.Fatalf("error code = %q, want image_too_large", code)
		}
	})
	t.Run("huge scale on small logo is rejected before allocation", func(t *testing.T) {
		logo := formFile{field: "image", filename: "logo.png", content: testPNG(t, 8, 8)}
		w := post(t, "/api/v1/watermark", map[string]string{"scale": "1000000"}, input(16, 16), logo)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusRequestEntityTooLarge, w.Body.String())
		}
		if code := decodeJSONError(t, w.Body.Bytes()); code != "image_too_large" {
			t.Fatalf("error code = %q, want image_too_large", code)
		}
	})
	t.Run("repeat watermark with huge font size exceeds working set", func(t *testing.T) {
		// font-size=4096 时 mark 画布为 16384×10240，仅 RGBA 两份已超过
		// 默认 512 MiB working set 上限，必须在分配前拒绝。
		w := post(t, "/api/v1/watermark", map[string]string{"text": "水", "mode": "repeat", "font-size": "4096"}, input(16, 16))
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusRequestEntityTooLarge, w.Body.String())
		}
		if code := decodeJSONError(t, w.Body.Bytes()); code != "image_too_large" {
			t.Fatalf("error code = %q, want image_too_large", code)
		}
	})
	t.Run("normal repeat watermark stays within working set", func(t *testing.T) {
		w := post(t, "/api/v1/watermark", map[string]string{"text": "mark", "mode": "repeat"}, input(32, 16))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
		}
		if got := decodePNG(t, w.Body.Bytes()).Bounds(); got != image.Rect(0, 0, 32, 16) {
			t.Fatalf("bounds = %v, want 32x16", got)
		}
	})
	t.Run("watermark parameters are validated", func(t *testing.T) {
		for name, fields := range map[string]map[string]string{
			"opacity=-1":     {"text": "mark", "opacity": "-1"},
			"position":       {"text": "mark", "position": "middle"},
			"font-size huge": {"text": "mark", "font-size": "1000000"},
			"angle":          {"text": "mark", "mode": "repeat", "angle": "100000"},
			"scale":          {"image": "", "scale": "0"},
		} {
			t.Run(name, func(t *testing.T) {
				w := post(t, "/api/v1/watermark", fields, input(16, 16))
				if w.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
				}
			})
		}
	})
	t.Run("oversized watermark text is rejected", func(t *testing.T) {
		w := post(t, "/api/v1/watermark", map[string]string{"text": strings.Repeat("x", 20000)}, input(16, 16))
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusRequestEntityTooLarge, w.Body.String())
		}
		if code := decodeJSONError(t, w.Body.Bytes()); code != "payload_too_large" {
			t.Fatalf("error code = %q, want payload_too_large", code)
		}
		// 5000 ASCII 字符未超 16KiB 字段上限，但超过领域 rune 上限。
		w = post(t, "/api/v1/watermark", map[string]string{"text": strings.Repeat("x", 5000)}, input(16, 16))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
	})
	t.Run("oversized scalar field is rejected", func(t *testing.T) {
		w := post(t, "/api/v1/resize", map[string]string{"anchor": strings.Repeat("x", 5000)}, input(16, 16))
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusRequestEntityTooLarge, w.Body.String())
		}
	})
}

func TestInspectSemantics(t *testing.T) {
	post := func(t *testing.T, fields map[string]string, file formFile) *httptest.ResponseRecorder {
		t.Helper()
		h := mustNew(t, Config{NoAuth: true})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, newMultipartRequest(t, http.MethodPost, "/api/v1/inspect", fields, file))
		return w
	}
	t.Run("gif inspect succeeds", func(t *testing.T) {
		w := post(t, nil, formFile{field: "input", filename: "a.gif", content: testGIF(t, 32, 16)})
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
		}
		var result struct {
			Image *struct {
				Format string `json:"format"`
				Width  int    `json:"width"`
				Height int    `json:"height"`
			} `json:"image"`
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if result.Image == nil || result.Image.Format != "gif" || result.Image.Width != 32 {
			t.Fatalf("image = %+v, want gif 32x16", result.Image)
		}
		if len(result.Error) > 0 && string(result.Error) != "null" {
			t.Fatalf("unexpected error field: %s", result.Error)
		}
	})
	t.Run("invalid image with strict=false returns metadata and error", func(t *testing.T) {
		w := post(t, nil, formFile{field: "input", filename: "junk.bin", content: []byte("not an image")})
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
		}
		var result struct {
			File struct {
				Name string `json:"name"`
			} `json:"file"`
			Image *json.RawMessage `json:"image"`
			Error *struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if result.File.Name != "junk.bin" {
			t.Fatalf("file.name = %q, want junk.bin", result.File.Name)
		}
		if result.Image != nil {
			t.Fatalf("image = %s, want absent", *result.Image)
		}
		if result.Error == nil || result.Error.Code != "decode_config_failed" {
			t.Fatalf("error = %+v, want decode_config_failed", result.Error)
		}
	})
	t.Run("invalid image with strict=true is rejected", func(t *testing.T) {
		w := post(t, map[string]string{"strict": "true"}, formFile{field: "input", filename: "junk.bin", content: []byte("not an image")})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
	})
	t.Run("oversized image is rejected", func(t *testing.T) {
		w := post(t, map[string]string{"no-hash": "true"}, formFile{field: "input", filename: "huge.png", content: pngHeader(t, 100000, 100000)})
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusRequestEntityTooLarge, w.Body.String())
		}
		if code := decodeJSONError(t, w.Body.Bytes()); code != "image_too_large" {
			t.Fatalf("error code = %q, want image_too_large", code)
		}
	})
}

func TestErrorStatusMapping(t *testing.T) {
	post := func(t *testing.T, path string, fields map[string]string, file formFile) (*httptest.ResponseRecorder, string) {
		t.Helper()
		h := mustNew(t, Config{NoAuth: true})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, newMultipartRequest(t, http.MethodPost, path, fields, file))
		return w, decodeJSONError(t, w.Body.Bytes())
	}
	t.Run("gif transform input is unsupported format", func(t *testing.T) {
		w, code := post(t, "/api/v1/resize", map[string]string{"width": "16"}, formFile{field: "input", filename: "a.gif", content: testGIF(t, 32, 16)})
		if w.Code != http.StatusUnsupportedMediaType {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusUnsupportedMediaType, w.Body.String())
		}
		if code != "unsupported_format" {
			t.Fatalf("error code = %q, want unsupported_format", code)
		}
	})
	t.Run("junk transform input is unsupported format", func(t *testing.T) {
		w, code := post(t, "/api/v1/resize", map[string]string{"width": "16"}, formFile{field: "input", filename: "a.bin", content: []byte("junk")})
		if w.Code != http.StatusUnsupportedMediaType {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusUnsupportedMediaType, w.Body.String())
		}
		if code != "unsupported_format" {
			t.Fatalf("error code = %q, want unsupported_format", code)
		}
	})
	t.Run("gif compress input is unsupported format", func(t *testing.T) {
		w, code := post(t, "/api/v1/compress", nil, formFile{field: "input", filename: "a.gif", content: testGIF(t, 32, 16)})
		if w.Code != http.StatusUnsupportedMediaType {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusUnsupportedMediaType, w.Body.String())
		}
		if code != "unsupported_format" {
			t.Fatalf("error code = %q, want unsupported_format", code)
		}
	})
	t.Run("oversized transform input is image too large", func(t *testing.T) {
		w, code := post(t, "/api/v1/resize", map[string]string{"width": "16"}, formFile{field: "input", filename: "huge.png", content: pngHeader(t, 100000, 100000)})
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusRequestEntityTooLarge, w.Body.String())
		}
		if code != "image_too_large" {
			t.Fatalf("error code = %q, want image_too_large", code)
		}
	})
	t.Run("parameter error stays invalid argument", func(t *testing.T) {
		w, code := post(t, "/api/v1/resize", map[string]string{"width": "abc"}, formFile{field: "input", filename: "a.png", content: testPNG(t, 8, 8)})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
		if code != "invalid_argument" {
			t.Fatalf("error code = %q, want invalid_argument", code)
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
	t.Run("undecodable input wraps imageio.ErrUnsupportedFormat", func(t *testing.T) {
		junkPath := filepath.Join(dir, "input.bin")
		if err := os.WriteFile(junkPath, []byte("this is not an image"), 0o600); err != nil {
			t.Fatalf("write junk: %v", err)
		}
		_, err := imageio.Probe(junkPath)
		if !errors.Is(err, imageio.ErrUnsupportedFormat) {
			t.Fatalf("Probe(junk) error = %v, want imageio.ErrUnsupportedFormat", err)
		}
	})
	t.Run("probe reports raw formats without encode normalization", func(t *testing.T) {
		gifPath := filepath.Join(dir, "input.gif")
		// GIF 能被 image.DecodeConfig 识别；Probe 只报告识别结果，
		// 不做受支持编码集合的归一化（那是 NormalizeFormat 的职责）。
		if err := os.WriteFile(gifPath, testGIF(t, 32, 16), 0o600); err != nil {
			t.Fatalf("write gif: %v", err)
		}
		info, err := imageio.Probe(gifPath)
		if err != nil {
			t.Fatalf("Probe(gif) error = %v", err)
		}
		if info.Format != "gif" || info.Width != 32 || info.Height != 16 {
			t.Fatalf("Probe(gif) = %+v, want format=gif 32x16", info)
		}
	})
	t.Run("within limits passes", func(t *testing.T) {
		if err := admitImage(path, Config{MaxPixels: 1000, MaxDimension: 64}); err != nil {
			t.Fatalf("admitImage() = %v, want nil", err)
		}
	})
}

func TestEndpointSuccess(t *testing.T) {
	input := formFile{field: "input", filename: "a.png", content: testPNG(t, 32, 16)}
	post := func(t *testing.T, path string, fields map[string]string, files ...formFile) *httptest.ResponseRecorder {
		t.Helper()
		h := mustNew(t, Config{NoAuth: true})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, newMultipartRequest(t, http.MethodPost, path, fields, files...))
		return w
	}
	t.Run("compress", func(t *testing.T) {
		if _, err := compress.EnsureBinary(compress.PngQuant); err != nil {
			t.Skipf("native compression binaries unavailable: %v", err)
		}
		w := post(t, "/api/v1/compress", map[string]string{"quality": "80"}, input)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); ct != "image/png" {
			t.Fatalf("Content-Type = %q, want image/png", ct)
		}
	})
	t.Run("resize", func(t *testing.T) {
		w := post(t, "/api/v1/resize", map[string]string{"width": "16"}, input)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
		}
		if got := decodePNG(t, w.Body.Bytes()).Bounds(); got != image.Rect(0, 0, 16, 8) {
			t.Fatalf("bounds = %v, want 16x8", got)
		}
	})
	t.Run("crop", func(t *testing.T) {
		w := post(t, "/api/v1/crop", map[string]string{"anchor": "center", "width": "50%", "height": "50%"}, input)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
		}
		if got := decodePNG(t, w.Body.Bytes()).Bounds(); got != image.Rect(0, 0, 16, 8) {
			t.Fatalf("bounds = %v, want 16x8", got)
		}
	})
	t.Run("convert", func(t *testing.T) {
		w := post(t, "/api/v1/convert", map[string]string{"to": "webp", "quality": "90"}, input)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); ct != "image/webp" {
			t.Fatalf("Content-Type = %q, want image/webp", ct)
		}
		if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "a_converted.webp") {
			t.Fatalf("Content-Disposition = %q, want a_converted.webp", cd)
		}
	})
	t.Run("watermark text", func(t *testing.T) {
		w := post(t, "/api/v1/watermark", map[string]string{"text": "DRAFT", "position": "center", "opacity": "0.5"}, input)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
		}
		if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "a_watermarked.png") {
			t.Fatalf("Content-Disposition = %q, want a_watermarked.png", cd)
		}
	})
	t.Run("watermark image", func(t *testing.T) {
		logo := formFile{field: "image", filename: "logo.png", content: testPNG(t, 8, 8)}
		w := post(t, "/api/v1/watermark", map[string]string{"scale": "0.5"}, input, logo)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
		}
		if got := decodePNG(t, w.Body.Bytes()).Bounds(); got != image.Rect(0, 0, 32, 16) {
			t.Fatalf("bounds = %v, want 32x16", got)
		}
	})
	t.Run("inspect", func(t *testing.T) {
		w := post(t, "/api/v1/inspect", map[string]string{"no-hash": "true"}, input)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
		}
		var result struct {
			SchemaVersion string `json:"schema_version"`
			Image         *struct {
				Width int `json:"width"`
			} `json:"image"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if result.SchemaVersion != "itb.inspect.v1" || result.Image == nil || result.Image.Width != 32 {
			t.Fatalf("result = %+v, want schema itb.inspect.v1 with width 32", result)
		}
	})
}

func TestUnknownFieldsRejected(t *testing.T) {
	for _, path := range []string{
		"/api/v1/compress",
		"/api/v1/resize",
		"/api/v1/crop",
		"/api/v1/convert",
		"/api/v1/watermark",
		"/api/v1/inspect",
	} {
		t.Run(path, func(t *testing.T) {
			h := mustNew(t, Config{NoAuth: true})
			w := httptest.NewRecorder()
			input := formFile{field: "input", filename: "a.png", content: testPNG(t, 8, 8)}
			h.ServeHTTP(w, newMultipartRequest(t, http.MethodPost, path, map[string]string{"foo": "bar"}, input))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
			}
			if code := decodeJSONError(t, w.Body.Bytes()); code != "invalid_argument" {
				t.Fatalf("error code = %q, want invalid_argument", code)
			}
		})
	}
}

func TestValidationBoundaries(t *testing.T) {
	post := func(t *testing.T, path string, fields map[string]string, files ...formFile) *httptest.ResponseRecorder {
		t.Helper()
		h := mustNew(t, Config{NoAuth: true})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, newMultipartRequest(t, http.MethodPost, path, fields, files...))
		return w
	}
	input := formFile{field: "input", filename: "a.png", content: testPNG(t, 16, 16)}
	t.Run("non-finite floats are rejected", func(t *testing.T) {
		for name, fields := range map[string]map[string]string{
			"opacity NaN":        {"text": "mark", "opacity": "NaN"},
			"opacity Infinity":   {"text": "mark", "opacity": "Infinity"},
			"margin NaN":         {"text": "mark", "margin": "NaN"},
			"scale Inf with img": {"scale": "+Inf"},
		} {
			t.Run(name, func(t *testing.T) {
				files := []formFile{input}
				if fields["scale"] != "" {
					files = append(files, formFile{field: "image", filename: "logo.png", content: testPNG(t, 4, 4)})
				}
				w := post(t, "/api/v1/watermark", fields, files...)
				if w.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
				}
			})
		}
	})
	t.Run("non-finite percent is rejected", func(t *testing.T) {
		for _, value := range []string{"NaN%", "Inf%", "-Infinity%"} {
			t.Run(value, func(t *testing.T) {
				w := post(t, "/api/v1/resize", map[string]string{"percent": value}, input)
				if w.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
				}
			})
		}
	})
	t.Run("scalar and file sharing one field name is a duplicate", func(t *testing.T) {
		h := mustNew(t, Config{NoAuth: true})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, newRawMultipartRequest(t, "/api/v1/resize",
			rawPart{name: "input", value: "x"},
			rawPart{name: "input", isFile: true, filename: "a.png", content: testPNG(t, 8, 8)},
			rawPart{name: "width", value: "4"}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
	})
}

func TestStreamingHeaders(t *testing.T) {
	h := mustNew(t, Config{NoAuth: true})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, newMultipartRequest(t, http.MethodPost, "/api/v1/resize",
		map[string]string{"width": "16"},
		formFile{field: "input", filename: "a.png", content: testPNG(t, 32, 16)}))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}
	headers := map[string]string{
		"Content-Type":        "image/png",
		"Content-Disposition": "a_resized.png",
		"Content-Length":      "",
		"X-ITB-Input-Size":    "",
		"X-ITB-Output-Size":   "",
		"X-ITB-Operation":     "resize",
	}
	for name, want := range headers {
		got := w.Header().Get(name)
		if want != "" && !strings.Contains(got, want) {
			t.Fatalf("%s = %q, want to contain %q", name, got, want)
		}
		if got == "" {
			t.Fatalf("%s header is empty", name)
		}
	}
}
