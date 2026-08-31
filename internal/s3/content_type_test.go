package s3

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// magic fixtures：各格式的最小文件头，内容检测应识别它们而与扩展名无关。
var (
	jpegMagic = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	pngMagic  = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	gifMagic  = []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00")
	webpMagic = append([]byte("RIFF\x24\x00\x00\x00WEBP"), []byte("VP8 \x10\x00\x00\x00")...)
	pdfMagic  = []byte("%PDF-1.7\n1 0 obj\nendobj\n")
	zipMagic  = []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00")
	htmlBody  = []byte("<html><head><title>Error</title></head><body>Service unavailable</body></html>")
	jsonBody  = []byte(`{"error":{"code":"InternalError","message":"boom"}}`)
	svgXML    = []byte(`<?xml version="1.0" encoding="UTF-8"?><svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"></svg>`)
	svgBare   = []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"></svg>`)
	plainText = []byte("just some plain text file")
	// 超过 512 字节嗅探上限的 JSON：只能看到截断前缀，
	// 走 {" 结构前缀启发式
	largeJSON = []byte(`{"errors":[` + string(bytes.Repeat([]byte(`"0123456789012345678901234567890123456789",`), 12)) + `"x"]}`)
	// 二进制且无已知 magic：内容检测应放弃，落到扩展名兜底
	unknownBinary = []byte{0x00, 0x01, 0x02, 0xFE, 0xFF, 0x9A, 0x00, 0x33}
)

// writeContentFixture 把内容写入指定文件名的临时文件，文件名用于
// 扩展名兜底路径的测试。
func writeContentFixture(t *testing.T, name string, content []byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestResolveContentType(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		content  []byte
		explicit string
		want     string
	}{
		{"显式指定优先于内容", "fake.jpg", htmlBody, "image/jpeg", "image/jpeg"},
		{"HTML 错误页改名 .jpg 检测为 text/html", "error.jpg", htmlBody, "", "text/html; charset=utf-8"},
		{"JPEG magic 覆盖任意扩展名", "photo.txt", jpegMagic, "", "image/jpeg"},
		{"PNG magic", "image.dat", pngMagic, "", "image/png"},
		{"GIF magic", "anim.bin", gifMagic, "", "image/gif"},
		{"WebP RIFF magic", "pic.bin", webpMagic, "", "image/webp"},
		{"PDF magic", "doc.bin", pdfMagic, "", "application/pdf"},
		{"ZIP magic", "arch.bin", zipMagic, "", "application/zip"},
		{"JSON body（错误响应体）", "resp.bin", jsonBody, "", "application/json"},
		{"超过嗅探上限的大 JSON 走前缀启发式", "resp.bin", largeJSON, "", "application/json"},
		{"SVG 带 XML 声明", "vector.jpg", svgXML, "", "image/svg+xml"},
		{"SVG 裸 <svg> 根元素", "vector.bin", svgBare, "", "image/svg+xml"},
		{"普通文本保持 text/plain", "notes.bin", plainText, "", "text/plain; charset=utf-8"},
		{"未知二进制回退扩展名 .png", "blob.png", unknownBinary, "", "image/png"},
		{"未知二进制且未知扩展名", "blob.xyzunknown", unknownBinary, "", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeContentFixture(t, tt.fileName, tt.content)
			if got := ResolveContentType(path, tt.explicit); got != tt.want {
				t.Errorf("ResolveContentType(%s) = %q, want %q", tt.fileName, got, tt.want)
			}
		})
	}
}

// TestResolveContentTypeMissingFile 文件不可读时兜底到扩展名表，
// 读取失败不阻断上传。
func TestResolveContentTypeMissingFile(t *testing.T) {
	got := ResolveContentType(filepath.Join(t.TempDir(), "gone.png"), "")
	if got != "image/png" {
		t.Errorf("missing file falls back to extension, got %q", got)
	}
	got = ResolveContentType(filepath.Join(t.TempDir(), "gone"), "")
	if got != "application/octet-stream" {
		t.Errorf("missing file without extension, got %q", got)
	}
}

// TestUploadContentTypeFromContent 协议级断言：HTML 内容改名为 .jpg
// 上传时 PUT 的 Content-Type 是 text/html 而不是 image/jpeg。
func TestUploadContentTypeFromContent(t *testing.T) {
	rec, client := newUploadTestServer(t, nil)
	path := writeContentFixture(t, "error.jpg", htmlBody)

	if _, err := Upload(context.Background(), client, path, "error.jpg", nil); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if got := rec.recordedPutHeaders().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("PUT Content-Type = %q, want text/html（内容优先于扩展名）", got)
	}
	assertMethods(t, rec.snapshotMethods(), []string{http.MethodPut})
}

// TestUploadExplicitContentTypeOverridesContent 显式 --content-type
// 原样生效，不做内容改写。
func TestUploadExplicitContentTypeOverridesContent(t *testing.T) {
	rec, client := newUploadTestServer(t, nil)
	path := writeContentFixture(t, "error.jpg", htmlBody)

	if _, err := Upload(context.Background(), client, path, "error.jpg", &UploadOptions{ContentType: "application/xml"}); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if got := rec.recordedPutHeaders().Get("Content-Type"); got != "application/xml" {
		t.Errorf("PUT Content-Type = %q, want application/xml", got)
	}
}
