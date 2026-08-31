package httpapi

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"imagetoolbox/internal/compress"
)

// TestMain 注入仓库根目录（与 main.go 的 embed.FS 结构一致，内含 bins/），
// 供涉及原生压缩工具的集成测试使用；平台二进制缺失时相关测试自行跳过。
func TestMain(m *testing.M) {
	compress.InitBinaries(os.DirFS("../.."))
	os.Exit(m.Run())
}

// 测试夹具：按项目约定就地合成图片（image.NewNRGBA），不提交二进制 fixture。

type formFile struct {
	field    string
	filename string
	content  []byte
}

func testPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return buf.Bytes()
}

// testGIF 生成真实可解码的 GIF；用于验证 Probe 对支持集合外格式的行为。
func testGIF(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewPaletted(image.Rect(0, 0, width, height), color.Palette{color.Gray{0}, color.Gray{255}})
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode test gif: %v", err)
	}
	return buf.Bytes()
}

func decodePNG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	return img
}

// pngHeader 构造只含签名与 IHDR 的 PNG 头：足够 image.DecodeConfig 读出
// 尺寸但不分配像素，用于测试超大图片的准入检查。
func pngHeader(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.Write([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})
	data := make([]byte, 13)
	binary.BigEndian.PutUint32(data[0:4], uint32(width))
	binary.BigEndian.PutUint32(data[4:8], uint32(height))
	data[8] = 8 // bit depth
	data[9] = 6 // color type RGBA
	chunk := append([]byte("IHDR"), data...)
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(data)))
	buf.Write(length[:])
	buf.Write(chunk)
	var crc [4]byte
	binary.BigEndian.PutUint32(crc[:], crc32.ChecksumIEEE(chunk))
	buf.Write(crc[:])
	return buf.Bytes()
}

// newMultipartRequest 构造 multipart/form-data 请求，不监听任何端口。
func newMultipartRequest(t *testing.T, method, target string, fields map[string]string, files ...formFile) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, f := range files {
		fw, err := w.CreateFormFile(f.field, f.filename)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := fw.Write(f.content); err != nil {
			t.Fatalf("write form file: %v", err)
		}
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(method, target, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

// mustNew 构造测试 handler；New 对非法配置返回 error 而不是 panic。
func mustNew(t *testing.T, cfg Config) http.Handler {
	t.Helper()
	h, err := New(cfg)
	if err != nil {
		t.Fatalf("New(%+v): %v", cfg, err)
	}
	return h
}

func decodeJSONError(t *testing.T, body []byte) string {
	t.Helper()
	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode error body %q: %v", body, err)
	}
	if payload.Error.Code == "" || payload.Error.Message == "" {
		t.Fatalf("expected error code and message, got %q", body)
	}
	return payload.Error.Code
}
