package inspect

import (
	"os"
	"strings"
	"testing"
)

func TestFormatRegistryLookup(t *testing.T) {
	for _, name := range []string{"png", "jpeg", "gif", "webp", "bmp", "tiff", "svg"} {
		spec, ok := LookupFormat(name)
		if !ok {
			t.Fatalf("format %q missing from registry", name)
		}
		if spec.Name != name || spec.CanonicalExtension == "" || spec.MIMEType == "" {
			t.Fatalf("format %q has incomplete spec: %+v", name, spec)
		}
	}
	if _, ok := LookupFormat("heic"); ok {
		t.Fatal("heic must not be in the registry")
	}
}

func TestFormatByExtension(t *testing.T) {
	tests := []struct {
		ext      string
		wantName string
		wantOK   bool
	}{
		{".png", "png", true},
		{".PNG", "png", true},
		{".jpg", "jpeg", true},
		{".jpeg", "jpeg", true},
		{".JPG", "jpeg", true},
		{".gif", "gif", true},
		{".webp", "webp", true},
		{".bmp", "bmp", true},
		{".tiff", "tiff", true},
		{".tif", "tiff", true},
		{".svg", "svg", true},
		{".txt", "", false},
		{".jxl", "", false},
	}
	for _, tt := range tests {
		spec, ok := FormatByExtension(tt.ext)
		if ok != tt.wantOK || (ok && spec.Name != tt.wantName) {
			t.Errorf("FormatByExtension(%q) = %q, %v; want %q, %v", tt.ext, spec.Name, ok, tt.wantName, tt.wantOK)
		}
	}
}

func TestExtensionMatches(t *testing.T) {
	jpeg, _ := LookupFormat("jpeg")
	if !jpeg.ExtensionMatches(".jpg") || !jpeg.ExtensionMatches(".JPEG") || !jpeg.ExtensionMatches(".jpeg") {
		t.Error("jpeg must accept .jpg/.jpeg/.JPEG")
	}
	if jpeg.ExtensionMatches(".png") {
		t.Error("jpeg must not accept .png")
	}
	tiff, _ := LookupFormat("tiff")
	if !tiff.ExtensionMatches(".tif") || !tiff.ExtensionMatches(".tiff") {
		t.Error("tiff must accept .tif/.tiff")
	}
	if jpeg.ExtensionMatches("") {
		t.Error("empty extension must never match")
	}
}

// TestMagicSniff 用真实/最小 magic 头验证光栅格式识别。
func TestMagicSniff(t *testing.T) {
	tests := []struct {
		name      string
		header    []byte
		wantName  string
		wantFound bool
	}{
		{"png", []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00}, "png", true},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00}, "jpeg", true},
		{"gif87a", []byte("GIF87a padding"), "gif", true},
		{"gif89a", []byte("GIF89a padding"), "gif", true},
		{"webp", []byte("RIFF\x24\x00\x00\x00WEBPVP8 "), "webp", true},
		{"bmp", []byte("BM\x36\x00\x00\x00"), "bmp", true},
		{"tiff little endian", []byte("II*\x00\x08\x00"), "tiff", true},
		{"tiff big endian", []byte("MM\x00*\x00\x08"), "tiff", true},
		{"RIFF 但非 webp", []byte("RIFF\x24\x00\x00\x00WAVEfmt "), "", false},
		{"纯文本", []byte("hello world, definitely not an image"), "", false},
		{"空头", nil, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := magicSniff(tt.header)
			if tt.wantFound {
				if spec == nil || spec.Name != tt.wantName {
					t.Fatalf("magicSniff = %+v, want %q", spec, tt.wantName)
				}
				if !spec.DecodeSupported {
					t.Fatalf("raster format %q must be decode-supported", spec.Name)
				}
			} else if spec != nil {
				t.Fatalf("magicSniff = %+v, want nil", spec)
			}
		})
	}
}

// TestRecognizeContentSVG SVG 经流式 XML 解析识别，而非 magic。
func TestRecognizeContentSVG(t *testing.T) {
	path := writeSVG(t, `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="16"></svg>`)
	content := recognizeContent([]byte("<svg"), path)
	if !content.Recognized || content.Format != "svg" {
		t.Fatalf("content = %+v, want recognized svg", content)
	}
	if content.MIMEType != "image/svg+xml" {
		t.Errorf("mime = %q, want image/svg+xml", content.MIMEType)
	}
	if content.DecodeSupported || content.FullDecodeSupported {
		t.Errorf("svg must not be decode/full-decode supported: %+v", content)
	}
	if !content.ExtensionMatches {
		t.Errorf(".svg extension must match: %+v", content)
	}
}

// TestRecognizeContentUnrecognized 非图片内容：recognized=false，
// 各字段保持零值。
func TestRecognizeContentUnrecognized(t *testing.T) {
	path := writeTextFile(t, "not-an-image.txt", "just some text, no markup here at all")

	content := recognizeContent([]byte("just some text, no markup here at all"), path)
	if content.Recognized {
		t.Fatalf("content = %+v, want unrecognized", content)
	}
	if content.Format != "" || content.MIMEType != "" {
		t.Fatalf("unrecognized content must not carry format/mime: %+v", content)
	}
	if content.ExtensionMatches {
		t.Error("extension_matches must be false for unrecognized content")
	}
}

// TestRecognizeContentExtensionMismatch 内容与扩展名不一致时如实报告。
func TestRecognizeContentExtensionMismatch(t *testing.T) {
	path := writeSVG(t, `<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	renamed := strings.TrimSuffix(path, ".svg") + ".png"
	if err := os.Rename(path, renamed); err != nil {
		t.Fatalf("rename: %v", err)
	}

	content := recognizeContent([]byte("<svg"), renamed)
	if !content.Recognized || content.Format != "svg" {
		t.Fatalf("content = %+v", content)
	}
	if content.ExtensionMatches {
		t.Error("svg content named .png must report extension_matches=false")
	}
}
