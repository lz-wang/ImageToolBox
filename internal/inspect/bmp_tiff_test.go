package inspect

import (
	"bytes"
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

// writeBMPTiffFixture 编码 BMP/TIFF fixture。
func writeBMPTiffFixture(t *testing.T, name string, encode func(io.Writer, image.Image) error) string {
	t.Helper()

	var buf bytes.Buffer
	if err := encode(&buf, image.NewRGBA(image.Rect(0, 0, 6, 4))); err != nil {
		t.Fatalf("encode %s: %v", name, err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// corruptTIFF 构造"magic 正确但内容损坏"的 TIFF。
func corruptTIFF(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "corrupt.tiff")
	data := append([]byte("II*\x00"), bytes.Repeat([]byte{0xFF}, 64)...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// TestInspectBMP BMP 经 x/image decoder 完整进入识别→结构校验链。
func TestInspectBMP(t *testing.T) {
	path := writeBMPTiffFixture(t, "photo.bmp", bmp.Encode)

	result, err := File(path, Options{NoHash: true})
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if !result.Content.Recognized || result.Content.Format != "bmp" {
		t.Fatalf("content = %+v, want bmp", result.Content)
	}
	if result.Content.MIMEType != "image/bmp" {
		t.Errorf("mime = %q", result.Content.MIMEType)
	}
	if !result.Content.DecodeSupported || !result.Content.FullDecodeSupported {
		t.Errorf("bmp must support decode/full-decode: %+v", result.Content)
	}
	if result.Image == nil || result.Image.Format != "bmp" {
		t.Fatalf("image = %+v, want bmp", result.Image)
	}
	if result.Image.Width != 6 || result.Image.Height != 4 {
		t.Errorf("dimensions = %dx%d, want 6x4", result.Image.Width, result.Image.Height)
	}
	if !result.Image.AnimationKnown || result.Image.Animated {
		t.Errorf("bmp animation should be statically known non-animated: %+v", result.Image)
	}

	// full decode 通道
	full, err := File(path, Options{NoHash: true, FullDecode: true})
	if err != nil {
		t.Fatalf("File full decode: %v", err)
	}
	if full.Image.FullDecodeOK == nil || !*full.Image.FullDecodeOK {
		t.Errorf("full_decode_ok = %v, want true", full.Image.FullDecodeOK)
	}
}

// TestInspectTIFF TIFF（含 .tif 别名扩展名）与扩展名匹配断言。
func TestInspectTIFF(t *testing.T) {
	path := writeBMPTiffFixture(t, "scan.tif", func(buf io.Writer, img image.Image) error {
		return tiff.Encode(buf, img, nil)
	})

	result, err := File(path, Options{NoHash: true})
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if !result.Content.Recognized || result.Content.Format != "tiff" {
		t.Fatalf("content = %+v, want tiff", result.Content)
	}
	if result.Content.MIMEType != "image/tiff" {
		t.Errorf("mime = %q", result.Content.MIMEType)
	}
	// .tif 是 tiff 的 alias 扩展名，必须判定为匹配
	if !result.Content.ExtensionMatches {
		t.Errorf(".tif must match tiff: %+v", result.Content)
	}
	if result.Image == nil || result.Image.Width != 6 || result.Image.Height != 4 {
		t.Fatalf("image = %+v, want 6x4", result.Image)
	}
}

// TestInspectCorruptTIFF 结构损坏的 TIFF：内容已识别但结构校验失败，
// 非 strict 带 error 对象，strict 直接报错。
func TestInspectCorruptTIFF(t *testing.T) {
	path := corruptTIFF(t)

	result, err := File(path, Options{NoHash: true})
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if !result.Content.Recognized || result.Content.Format != "tiff" {
		t.Fatalf("content = %+v, want recognized tiff", result.Content)
	}
	if result.Error == nil || result.Error.Code != "decode_config_failed" {
		t.Fatalf("error = %+v, want decode_config_failed", result.Error)
	}

	if _, err := File(path, Options{NoHash: true, Strict: true}); err == nil {
		t.Error("strict mode must fail on structurally broken tiff")
	}
}

// TestInspectPNGExtensionMismatch PNG 内容改名为 .jpg 时
// extension_matches=false，但识别仍以内容为准。
func TestInspectPNGExtensionMismatch(t *testing.T) {
	pngPath := writeTempImage(t, "a.png", image.NewNRGBA(image.Rect(0, 0, 4, 4)))
	renamed := filepath.Join(t.TempDir(), "a.jpg")
	data, err := os.ReadFile(pngPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := os.WriteFile(renamed, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	result, err := File(renamed, Options{NoHash: true})
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if result.Content.Format != "png" {
		t.Fatalf("format = %q, want png（内容优先于扩展名）", result.Content.Format)
	}
	if result.Content.ExtensionMatches {
		t.Error("png content named .jpg must report extension_matches=false")
	}
}

// TestInspectUnrecognizedText 纯文本：未识别 + decode_config_failed
// error 对象；strict 报错。
func TestInspectUnrecognizedText(t *testing.T) {
	path := writeTextFile(t, "notes.txt", "plain textual content, nothing more")

	result, err := File(path, Options{NoHash: true})
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if result.Content.Recognized {
		t.Fatalf("content = %+v, want unrecognized", result.Content)
	}
	if result.Error == nil || !strings.Contains(result.Error.Code, "decode_config_failed") {
		t.Fatalf("error = %+v, want decode_config_failed", result.Error)
	}

	if _, err := File(path, Options{NoHash: true, Strict: true}); err == nil {
		t.Error("strict must fail on unrecognized content")
	}
}

// TestInspectV3ContentObjectInResult 锁定 v3 Result 中的 content 对象
// 序列化形状（JSON 契约）。
func TestInspectV3ContentObjectInResult(t *testing.T) {
	pngPath := writeTempImage(t, "a.png", image.NewNRGBA(image.Rect(0, 0, 2, 2)))

	result, err := File(pngPath, Options{NoHash: true})
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if result.SchemaVersion != "itb.inspect.v3" {
		t.Errorf("schema_version = %q", result.SchemaVersion)
	}
	if result.Content.Format != "png" ||
		result.Content.CanonicalExtension != ".png" ||
		result.Content.MIMEType != "image/png" ||
		!result.Content.Recognized ||
		!result.Content.DecodeSupported ||
		!result.Content.FullDecodeSupported ||
		!result.Content.ExtensionMatches {
		t.Fatalf("content = %+v", result.Content)
	}
}

// TestInspectTruncatedPNGKeepsV2Behavior 截断 PNG（IHDR 完好）的
// 行为与 v2 一致：DecodeConfig 成功，损坏由 full decode 捕获。
func TestInspectTruncatedPNGKeepsV2Behavior(t *testing.T) {
	path := truncatedPNG(t)

	result, err := File(path, Options{NoHash: true})
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if !result.Content.Recognized || result.Content.Format != "png" {
		t.Fatalf("content = %+v", result.Content)
	}
	if result.Image == nil || !result.Image.DecodeConfigOK {
		t.Fatalf("image = %+v, want decode_config_ok（IHDR 完好）", result.Image)
	}
	if result.Error != nil {
		t.Fatalf("header-valid png must not carry error: %+v", result.Error)
	}
}

