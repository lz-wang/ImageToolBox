package httpapi

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"imagetoolbox/internal/compress"
)

// skipIfNoBinaries 在内嵌压缩工具不可用时跳过集成测试。
func skipIfNoBinaries(t *testing.T) {
	t.Helper()
	if _, err := compress.EnsureBinary(compress.PngQuant); err != nil {
		t.Skipf("内嵌压缩二进制不可用，跳过集成测试: %v", err)
	}
}

func decodePNG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	return img
}

func TestResizeEndpoint(t *testing.T) {
	handler := testHandler(t, nil)
	content := testPNG(t, 32, 16)

	t.Run("fit 缩放返回目标尺寸", func(t *testing.T) {
		req := newMultipartRequest(t, http.MethodPost, "/api/v1/resize",
			map[string]string{"options": `{"width":16,"height":8,"mode":"fit","filter":"lanczos"}`},
			formFile{field: "file", filename: "a.png", content: content})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if got := w.Header().Get("Content-Type"); got != "image/png" {
			t.Fatalf("expected image/png, got %s", got)
		}
		if got := w.Header().Get("Content-Disposition"); got != "attachment; filename=\"a_resized.png\"" {
			t.Fatalf("unexpected disposition: %s", got)
		}
		if w.Header().Get("X-ITB-Input-Size") == "" || w.Header().Get("X-ITB-Output-Size") == "" {
			t.Fatal("expected X-ITB size headers")
		}

		img := decodePNG(t, w.Body.Bytes())
		if img.Bounds().Dx() != 16 || img.Bounds().Dy() != 8 {
			t.Fatalf("expected 16x8, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
		}
	})

	t.Run("缺少 file 返回 400", func(t *testing.T) {
		req := newMultipartRequest(t, http.MethodPost, "/api/v1/resize",
			map[string]string{"options": `{"width":16}`})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
		decodeJSONError(t, w.Body.Bytes())
	})

	t.Run("非法参数返回 400", func(t *testing.T) {
		req := newMultipartRequest(t, http.MethodPost, "/api/v1/resize",
			map[string]string{"options": `{"mode":"fit"}`},
			formFile{field: "file", filename: "a.png", content: content})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
		decodeJSONError(t, w.Body.Bytes())
	})
}

func TestCropEndpoint(t *testing.T) {
	handler := testHandler(t, nil)

	t.Run("center 50% 裁剪", func(t *testing.T) {
		req := newMultipartRequest(t, http.MethodPost, "/api/v1/crop",
			map[string]string{"options": `{"anchor":"center","width":"50%","height":"50%"}`},
			formFile{field: "file", filename: "a.png", content: testPNG(t, 20, 10)})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		img := decodePNG(t, w.Body.Bytes())
		if img.Bounds().Dx() != 10 || img.Bounds().Dy() != 5 {
			t.Fatalf("expected 10x5, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
		}
		if got := w.Header().Get("Content-Disposition"); got != "attachment; filename=\"a_cropped.png\"" {
			t.Fatalf("unexpected disposition: %s", got)
		}
	})

	t.Run("缺少 anchor 返回 400", func(t *testing.T) {
		req := newMultipartRequest(t, http.MethodPost, "/api/v1/crop",
			map[string]string{"options": `{"width":"50%","height":"50%"}`},
			formFile{field: "file", filename: "a.png", content: testPNG(t, 4, 4)})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
		decodeJSONError(t, w.Body.Bytes())
	})
}

func TestConvertEndpoint(t *testing.T) {
	handler := testHandler(t, nil)

	t.Run("png 转 webp", func(t *testing.T) {
		req := newMultipartRequest(t, http.MethodPost, "/api/v1/convert",
			map[string]string{"options": `{"to":"webp","quality":80}`},
			formFile{field: "file", filename: "a.png", content: testPNG(t, 8, 8)})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if got := w.Header().Get("Content-Type"); got != "image/webp" {
			t.Fatalf("expected image/webp, got %s", got)
		}
		if got := w.Header().Get("Content-Disposition"); got != "attachment; filename=\"a_converted.webp\"" {
			t.Fatalf("unexpected disposition: %s", got)
		}
	})

	t.Run("缺少 to 返回 400", func(t *testing.T) {
		req := newMultipartRequest(t, http.MethodPost, "/api/v1/convert",
			map[string]string{"options": `{}`},
			formFile{field: "file", filename: "a.png", content: testPNG(t, 4, 4)})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
		decodeJSONError(t, w.Body.Bytes())
	})
}

func TestWatermarkEndpoint(t *testing.T) {
	handler := testHandler(t, nil)

	t.Run("文字水印 position 模式", func(t *testing.T) {
		req := newMultipartRequest(t, http.MethodPost, "/api/v1/watermark",
			map[string]string{"options": `{"type":"text","text":"ITB","mode":"position","position":"bottom-right","opacity":0.5}`},
			formFile{field: "file", filename: "a.png", content: testPNG(t, 64, 64)})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		img := decodePNG(t, w.Body.Bytes())
		if img.Bounds().Dx() != 64 || img.Bounds().Dy() != 64 {
			t.Fatalf("watermark must not change dimensions, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
		}
		if got := w.Header().Get("Content-Disposition"); got != "attachment; filename=\"a_watermarked.png\"" {
			t.Fatalf("unexpected disposition: %s", got)
		}
	})

	t.Run("空文字水印返回 400", func(t *testing.T) {
		req := newMultipartRequest(t, http.MethodPost, "/api/v1/watermark",
			map[string]string{"options": `{"type":"text","text":"","mode":"position"}`},
			formFile{field: "file", filename: "a.png", content: testPNG(t, 64, 64)})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
		decodeJSONError(t, w.Body.Bytes())
	})
}

// TestCompressEndpoint 为集成测试：触发内嵌 pngquant/oxipng 的解压与执行。
func TestCompressEndpoint(t *testing.T) {
	skipIfNoBinaries(t)
	handler := testHandler(t, nil)

	req := newMultipartRequest(t, http.MethodPost, "/api/v1/compress",
		map[string]string{"options": `{"quality":80}`},
		formFile{field: "file", filename: "a.png", content: testPNG(t, 128, 128)})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("expected image/png, got %s", got)
	}
	// 输出仍是合法 PNG
	decodePNG(t, w.Body.Bytes())
}

func TestCompressEndpointInvalidQuality(t *testing.T) {
	skipIfNoBinaries(t)
	handler := testHandler(t, nil)

	req := newMultipartRequest(t, http.MethodPost, "/api/v1/compress",
		map[string]string{"options": `{"quality":0}`},
		formFile{field: "file", filename: "a.png", content: testPNG(t, 8, 8)})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// quality=0 时服务端默认为 80，应成功
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with default quality, got %d: %s", w.Code, w.Body.String())
	}
}
