package inspect

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempImage 把图片编码写入临时文件，返回路径。
func writeTempImage(t *testing.T, name string, img image.Image) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	var buf bytes.Buffer
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		if err := png.Encode(&buf, img); err != nil {
			t.Fatalf("encode png: %v", err)
		}
	default:
		t.Fatalf("unsupported fixture format: %s", name)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// writeAnimatedGIF 生成 n 帧的 GIF 文件。
func writeAnimatedGIF(t *testing.T, frames int) string {
	t.Helper()

	g := &gif.GIF{}
	palette := color.Palette{color.White, color.Black}
	for i := range frames {
		img := image.NewPaletted(image.Rect(0, 0, 4, 4), palette)
		for y := range 4 {
			for x := range 4 {
				if (x+y+i)%2 == 0 {
					img.Set(x, y, color.Black)
				}
			}
		}
		g.Image = append(g.Image, img)
		g.Delay = append(g.Delay, 10)
	}
	g.Config = image.Config{Width: 4, Height: 4, ColorModel: palette}

	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatalf("encode gif: %v", err)
	}
	path := filepath.Join(t.TempDir(), "anim.gif")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// truncatedPNG 构造"header 正常但后半部分损坏"的 PNG：
// 签名 + 带正确 CRC 的 IHDR（DecodeConfig 成功），但没有 IDAT
//（完整 Decode 失败）。
func truncatedPNG(t *testing.T) string {
	t.Helper()

	data := []byte{
		0x00, 0x00, 0x00, 0x04, // width = 4
		0x00, 0x00, 0x00, 0x04, // height = 4
		0x08, 0x02, 0x00, 0x00, 0x00, // 8-bit RGB
	}
	chunk := append([]byte("IHDR"), data...)
	crc := crc32.ChecksumIEEE(chunk)

	file := append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A},
		0x00, 0x00, 0x00, 0x0D)
	file = append(file, chunk...)
	file = append(file, byte(crc>>24), byte(crc>>16), byte(crc>>8), byte(crc))

	path := filepath.Join(t.TempDir(), "truncated.png")
	if err := os.WriteFile(path, file, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestInspectStaticPNGDefaults(t *testing.T) {
	path := writeTempImage(t, "a.png", image.NewNRGBA(image.Rect(0, 0, 8, 6)))

	result, err := File(path, Options{NoHash: true})
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if result.SchemaVersion != SchemaVersion {
		t.Errorf("schema_version = %q, want %q", result.SchemaVersion, SchemaVersion)
	}
	if result.Image == nil {
		t.Fatal("expected image info")
	}
	if result.Image.Width != 8 || result.Image.Height != 6 {
		t.Errorf("dimensions = %dx%d, want 8x6", result.Image.Width, result.Image.Height)
	}
	// 静态格式在 header 阶段即可断言非动画；未开启 full-decode 时
	// FullDecodeOK 为 nil（未尝试）
	if !result.Image.AnimationKnown || result.Image.Animated {
		t.Errorf("static png: animation_known=%v animated=%v, want known/not animated", result.Image.AnimationKnown, result.Image.Animated)
	}
	if result.Image.FullDecodeOK != nil {
		t.Errorf("full_decode_ok = %v, want nil (not attempted)", *result.Image.FullDecodeOK)
	}
}

func TestInspectFullDecodeStaticPNG(t *testing.T) {
	path := writeTempImage(t, "a.png", image.NewNRGBA(image.Rect(0, 0, 8, 6)))

	result, err := File(path, Options{NoHash: true, FullDecode: true})
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if result.Image.FullDecodeOK == nil || !*result.Image.FullDecodeOK {
		t.Fatalf("full_decode_ok = %v, want true", result.Image.FullDecodeOK)
	}
	if result.Image.Animated || !result.Image.AnimationKnown {
		t.Errorf("animated=%v known=%v, want false/true", result.Image.Animated, result.Image.AnimationKnown)
	}
	if result.Image.FrameCount != 0 {
		t.Errorf("frame_count = %d, want 0 for static format", result.Image.FrameCount)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}
}

func TestInspectFullDecodeTruncatedPNG(t *testing.T) {
	path := truncatedPNG(t)

	// 非 strict：返回结果但 full_decode_ok=false 并带 warning
	result, err := File(path, Options{NoHash: true, FullDecode: true})
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if result.Image == nil {
		t.Fatalf("expected image info from valid header, got nil (error: %+v)", result.Error)
	}
	if result.Image.FullDecodeOK == nil || *result.Image.FullDecodeOK {
		t.Fatalf("expected full_decode_ok=false, got %v", result.Image.FullDecodeOK)
	}
	if len(result.Warnings) == 0 {
		t.Error("expected full decode warning")
	}

	// strict：直接报错（preflight 语义）
	if _, err := File(path, Options{NoHash: true, FullDecode: true, Strict: true}); err == nil {
		t.Error("expected strict error for truncated png")
	}
}

func TestInspectGIFAnimation(t *testing.T) {
	t.Run("多帧 GIF 判定 animated 并给出帧数", func(t *testing.T) {
		path := writeAnimatedGIF(t, 3)

		result, err := File(path, Options{NoHash: true, FullDecode: true})
		if err != nil {
			t.Fatalf("File: %v", err)
		}
		if !result.Image.Animated || !result.Image.AnimationKnown {
			t.Errorf("animated=%v known=%v, want true/true", result.Image.Animated, result.Image.AnimationKnown)
		}
		if result.Image.FrameCount != 3 {
			t.Errorf("frame_count = %d, want 3", result.Image.FrameCount)
		}
		if result.Image.FullDecodeOK == nil || !*result.Image.FullDecodeOK {
			t.Error("expected full_decode_ok=true")
		}
	})

	t.Run("单帧 GIF 判定非动画", func(t *testing.T) {
		path := writeAnimatedGIF(t, 1)

		result, err := File(path, Options{NoHash: true, FullDecode: true})
		if err != nil {
			t.Fatalf("File: %v", err)
		}
		if result.Image.Animated || !result.Image.AnimationKnown {
			t.Errorf("animated=%v known=%v, want false/true", result.Image.Animated, result.Image.AnimationKnown)
		}
		if result.Image.FrameCount != 1 {
			t.Errorf("frame_count = %d, want 1", result.Image.FrameCount)
		}
	})

	t.Run("未开启 full-decode 时 GIF 动画状态未知", func(t *testing.T) {
		path := writeAnimatedGIF(t, 3)

		result, err := File(path, Options{NoHash: true})
		if err != nil {
			t.Fatalf("File: %v", err)
		}
		if result.Image.AnimationKnown {
			t.Error("gif animation must be unknown without full decode")
		}
		if result.Image.Animated {
			t.Error("animated must not be asserted when unknown")
		}
	})
}

func TestWebpAnimationSniff(t *testing.T) {
	// 构造最小 RIFF/WEBP 头：RIFF <size> WEBP VP8X <size> <flags>
	webpHeader := func(firstChunk string, flags byte) []byte {
		h := []byte("RIFF")
		h = append(h, 0x24, 0x00, 0x00, 0x00)
		h = append(h, "WEBP"...)
		h = append(h, firstChunk...)
		h = append(h, 0x0A, 0x00, 0x00, 0x00) // chunk size
		h = append(h, flags)
		return append(h, make([]byte, 8)...) // padding 至足够长度
	}

	tests := []struct {
		name          string
		header        []byte
		wantAnimated  bool
		wantKnown     bool
	}{
		{"VP8X 动画位", webpHeader("VP8X", 0x02), true, true},
		{"VP8X 无动画位", webpHeader("VP8X", 0x10), false, true},
		{"纯 VP8 静态图", webpHeader("VP8 ", 0x00), false, true},
		{"纯 VP8L 静态图", webpHeader("VP8L", 0x00), false, true},
		{"太短", []byte("RIFF"), false, false},
		{"非 WebP", []byte("GIF89a whatever padding here --"), false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			animated, known := webpAnimation(tt.header)
			if animated != tt.wantAnimated || known != tt.wantKnown {
				t.Errorf("webpAnimation = (%v, %v), want (%v, %v)", animated, known, tt.wantAnimated, tt.wantKnown)
			}
		})
	}
}
