package server

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func unzipEntries(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	entries := map[string][]byte{}
	for _, f := range reader.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", f.Name, err)
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read zip entry %s: %v", f.Name, err)
		}
		entries[f.Name] = content
	}
	return entries
}

func TestBatchResizeEndpoint(t *testing.T) {
	handler := testHandler(t, nil)

	req := newMultipartRequest(t, http.MethodPost, "/api/v1/batch/resize",
		map[string]string{"options": `{"width":16,"mode":"fit"}`},
		formFile{field: "files", filename: "a.png", content: testPNG(t, 32, 32)},
		formFile{field: "files", filename: "b.png", content: testPNG(t, 64, 64)})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "application/zip" {
		t.Fatalf("expected application/zip, got %s", got)
	}
	if got := w.Header().Get("X-ITB-Success"); got != "2" {
		t.Fatalf("expected success=2, got %s", got)
	}

	entries := unzipEntries(t, w.Body.Bytes())
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(entries), entries)
	}
	for name, content := range entries {
		img := decodePNG(t, content)
		if img.Bounds().Dx() != 16 {
			t.Fatalf("%s: expected width 16, got %d", name, img.Bounds().Dx())
		}
	}
	if _, ok := entries["a_resized.png"]; !ok {
		t.Fatal("expected a_resized.png in zip")
	}
	if _, ok := entries["b_resized.png"]; !ok {
		t.Fatal("expected b_resized.png in zip")
	}
}

func TestBatchConvertEndpointNaming(t *testing.T) {
	handler := testHandler(t, nil)

	req := newMultipartRequest(t, http.MethodPost, "/api/v1/batch/convert",
		map[string]string{"options": `{"to":"webp","quality":80}`},
		formFile{field: "files", filename: "a.png", content: testPNG(t, 8, 8)})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	entries := unzipEntries(t, w.Body.Bytes())
	if _, ok := entries["a_converted.webp"]; !ok {
		t.Fatalf("expected a_converted.webp, got %v", entries)
	}
}

func TestBatchWatermarkEndpoint(t *testing.T) {
	handler := testHandler(t, nil)

	req := newMultipartRequest(t, http.MethodPost, "/api/v1/batch/watermark",
		map[string]string{"options": `{"type":"text","text":"ITB","mode":"position","position":"center","opacity":0.6}`},
		formFile{field: "files", filename: "a.png", content: testPNG(t, 64, 64)},
		formFile{field: "files", filename: "b.png", content: testPNG(t, 64, 64)})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	entries := unzipEntries(t, w.Body.Bytes())
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestBatchEndpointErrors(t *testing.T) {
	handler := testHandler(t, nil)

	tests := []struct {
		name    string
		target  string
		fields  map[string]string
		files   []formFile
		wantMsg string
	}{
		{
			name:   "缺少 files 字段",
			target: "/api/v1/batch/resize",
			fields: map[string]string{"options": `{"width":16}`},
		},
		{
			name:   "只上传非图片文件",
			target: "/api/v1/batch/resize",
			fields: map[string]string{"options": `{"width":16}`},
			files:  []formFile{{field: "files", filename: "note.txt", content: []byte("hello")}},
		},
		{
			name:   "convert 缺少目标格式",
			target: "/api/v1/batch/convert",
			fields: map[string]string{"options": `{}`},
			files:  []formFile{{field: "files", filename: "a.png", content: testPNG(t, 4, 4)}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newMultipartRequest(t, http.MethodPost, tt.target, tt.fields, tt.files...)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
			decodeJSONError(t, w.Body.Bytes())
		})
	}
}
