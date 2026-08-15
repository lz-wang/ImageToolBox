package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInspectEndpoint(t *testing.T) {
	handler := testHandler(t, nil)

	req := newMultipartRequest(t, http.MethodPost, "/api/v1/inspect", nil,
		formFile{field: "file", filename: "photo.png", content: testPNG(t, 40, 20)})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var payload struct {
		SchemaVersion string `json:"schema_version"`
		File          struct {
			Path     string `json:"path"`
			AbsPath  string `json:"abs_path"`
			Name     string `json:"name"`
			SizeByte int64  `json:"size_bytes"`
		} `json:"file"`
		Image struct {
			Format string `json:"format"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"image"`
		Hashes struct {
			SHA256 string `json:"sha256"`
		} `json:"hashes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode inspect json: %v", err)
	}

	if payload.SchemaVersion == "" {
		t.Fatal("expected schema_version")
	}
	if payload.File.Name != "photo.png" {
		t.Fatalf("expected name photo.png, got %s", payload.File.Name)
	}
	// 不向浏览器暴露服务端临时路径
	if payload.File.Path != "photo.png" {
		t.Fatalf("path should be basename only, got %q", payload.File.Path)
	}
	if payload.File.AbsPath != "" {
		t.Fatalf("abs_path must be empty, got %q", payload.File.AbsPath)
	}
	if payload.Image.Format != "png" || payload.Image.Width != 40 || payload.Image.Height != 20 {
		t.Fatalf("unexpected image info: %+v", payload.Image)
	}
	if len(payload.Hashes.SHA256) != 64 {
		t.Fatalf("expected sha256 hash, got %q", payload.Hashes.SHA256)
	}
}

func TestInspectEndpointMissingFile(t *testing.T) {
	handler := testHandler(t, nil)

	req := newMultipartRequest(t, http.MethodPost, "/api/v1/inspect", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	decodeJSONError(t, w.Body.Bytes())
}
