package httpapi

import (
	"image"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewMultipartContract(t *testing.T) {
	h := New(Config{})
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
