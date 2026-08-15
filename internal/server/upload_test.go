package server

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "普通文件名", input: "photo.png", want: "photo.png"},
		{name: "unix 路径穿越", input: "../../etc/passwd", want: "passwd"},
		{name: "windows 路径", input: `C:\Users\a.png`, want: "a.png"},
		{name: "隐藏文件", input: ".DS_Store", want: "DS_Store"},
		{name: "空字符串", input: "", want: ""},
		{name: "只有分隔符", input: "/", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeFilename(tt.input); got != tt.want {
				t.Fatalf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSaveFormFile(t *testing.T) {
	t.Run("保存上传文件并保留内容", func(t *testing.T) {
		content := testPNG(t, 2, 2)
		req := newMultipartRequest(t, http.MethodPost, "/", nil, formFile{field: "file", filename: "a.png", content: content})
		c, _ := newTestContext(t, req)

		dir, cleanup, err := newRequestDir("itb-test-upload")
		if err != nil {
			t.Fatalf("newRequestDir: %v", err)
		}
		defer cleanup()

		path, err := saveFormFile(c, dir, "file")
		if err != nil {
			t.Fatalf("saveFormFile: %v", err)
		}
		if path == "" {
			t.Fatal("expected saved path")
		}
		if filepath.Base(path) != "a.png" {
			t.Fatalf("expected sanitized name a.png, got %s", path)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read saved file: %v", err)
		}
		if !bytes.Equal(got, content) {
			t.Fatal("saved content mismatch")
		}
	})

	t.Run("字段缺失返回空路径", func(t *testing.T) {
		req := newMultipartRequest(t, http.MethodPost, "/", nil)
		c, _ := newTestContext(t, req)

		path, err := saveFormFile(c, t.TempDir(), "file")
		if err != nil {
			t.Fatalf("saveFormFile: %v", err)
		}
		if path != "" {
			t.Fatalf("expected empty path, got %s", path)
		}
	})
}

func TestRequireFormFileMissing(t *testing.T) {
	req := newMultipartRequest(t, http.MethodPost, "/", nil)
	c, w := newTestContext(t, req)

	if _, ok := requireFormFile(c, t.TempDir(), "file"); ok {
		t.Fatal("expected ok=false for missing field")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	decodeJSONError(t, w.Body.Bytes())
}

func TestBindOptions(t *testing.T) {
	t.Run("合法 JSON", func(t *testing.T) {
		req := newMultipartRequest(t, http.MethodPost, "/", map[string]string{"options": `{"width":8}`})
		c, _ := newTestContext(t, req)

		opts, ok := bindOptions[ResizeRequest](c)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if opts.Width != 8 {
			t.Fatalf("expected width 8, got %d", opts.Width)
		}
	})

	t.Run("空 options 使用零值", func(t *testing.T) {
		req := newMultipartRequest(t, http.MethodPost, "/", nil)
		c, _ := newTestContext(t, req)

		if _, ok := bindOptions[ResizeRequest](c); !ok {
			t.Fatal("expected ok=true for empty options")
		}
	})

	t.Run("非法 JSON 返回 400", func(t *testing.T) {
		req := newMultipartRequest(t, http.MethodPost, "/", map[string]string{"options": "not-json"})
		c, w := newTestContext(t, req)

		if _, ok := bindOptions[ResizeRequest](c); ok {
			t.Fatal("expected ok=false for invalid JSON")
		}
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
		if msg := decodeJSONError(t, w.Body.Bytes()); !strings.Contains(msg, "options") {
			t.Fatalf("error should mention options, got %q", msg)
		}
	})
}
