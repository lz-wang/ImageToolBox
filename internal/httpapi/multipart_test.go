package httpapi

import (
	"image"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"imagetoolbox/internal/compress"
)

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

func TestDuplicatePartsRejected(t *testing.T) {
	t.Run("duplicate scalar field", func(t *testing.T) {
		h := mustNew(t, Config{NoAuth: true})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, newRawMultipartRequest(t, "/api/v1/compress",
			rawPart{name: "quality", value: "80"},
			rawPart{name: "quality", value: "90"},
			rawPart{name: "input", isFile: true, filename: "a.png", content: testPNG(t, 8, 8)}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
	})
	t.Run("duplicate file field", func(t *testing.T) {
		h := mustNew(t, Config{NoAuth: true})
		w := httptest.NewRecorder()
		png := testPNG(t, 8, 8)
		h.ServeHTTP(w, newRawMultipartRequest(t, "/api/v1/resize",
			rawPart{name: "input", isFile: true, filename: "a.png", content: png},
			rawPart{name: "input", isFile: true, filename: "b.png", content: png},
			rawPart{name: "width", value: "4"}))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
	})
}

func TestFieldLimits(t *testing.T) {
	t.Run("text field accepts up to its larger limit", func(t *testing.T) {
		h := mustNew(t, Config{NoAuth: true})
		w := httptest.NewRecorder()
		// 8KiB 文本未超 text 专属上限，也不会触发领域 rune 校验失败以外
		// 的错误路径；此处只验证 400 而非 413（字段限制未触发）。
		h.ServeHTTP(w, newMultipartRequest(t, http.MethodPost, "/api/v1/watermark",
			map[string]string{"text": strings.Repeat("x", 8<<10)},
			formFile{field: "input", filename: "a.png", content: testPNG(t, 8, 8)}))
		if w.Code == http.StatusRequestEntityTooLarge {
			t.Fatalf("status = 413, want non-413: %s", w.Body.String())
		}
	})
}
