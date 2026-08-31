package imageio

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// buildEXIFAPP1 构造携带 Orientation 的 JPEG APP1(Exif) 段。
func buildEXIFAPP1(orientation uint16) []byte {
	var tiff bytes.Buffer
	tiff.WriteString("II")
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(42))
	_ = binary.Write(&tiff, binary.LittleEndian, uint32(8)) // IFD0 偏移
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(1)) // 条目数
	// Orientation 条目：tag 0x0112, type SHORT(3), count 1, 内联值
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(0x0112))
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(3))
	_ = binary.Write(&tiff, binary.LittleEndian, uint32(1))
	_ = binary.Write(&tiff, binary.LittleEndian, orientation)
	_ = binary.Write(&tiff, binary.LittleEndian, uint16(0)) // value 高位填充
	_ = binary.Write(&tiff, binary.LittleEndian, uint32(0)) // 下一 IFD

	seg := bytes.NewBuffer(nil)
	seg.WriteByte(0xFF)
	seg.WriteByte(0xE1)
	_ = binary.Write(seg, binary.BigEndian, uint16(2+6+tiff.Len()))
	seg.WriteString("Exif\x00\x00")
	seg.Write(tiff.Bytes())
	return seg.Bytes()
}

// writeOrientedJPEG 生成携带指定 EXIF Orientation 的真实 JPEG：
// 物理尺寸 width×height，APP1(Exif) 插在 SOI 之后。
func writeOrientedJPEG(t *testing.T, orientation uint16, width, height int) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var body bytes.Buffer
	if err := jpeg.Encode(&body, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	encoded := body.Bytes()

	var out bytes.Buffer
	out.Write(encoded[:2]) // SOI
	out.Write(buildEXIFAPP1(orientation))
	out.Write(encoded[2:])

	path := filepath.Join(t.TempDir(), "oriented.jpg")
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestJPEGOrientationParsing(t *testing.T) {
	tests := []struct {
		name        string
		orientation uint16
		want        int
	}{
		{"orientation 1", 1, 1},
		{"orientation 6（90° 旋转）", 6, 6},
		{"orientation 8（270° 旋转）", 8, 8},
		{"orientation 3（180° 旋转）", 3, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jpegOrientation(bytes.NewReader(buildSOIAPP1(t, tt.orientation))); got != tt.want {
				t.Errorf("jpegOrientation = %d, want %d", got, tt.want)
			}
		})
	}

	t.Run("非 JPEG 返回 0", func(t *testing.T) {
		if got := jpegOrientation(bytes.NewReader([]byte{0x89, 'P', 'N', 'G'})); got != 0 {
			t.Errorf("jpegOrientation(png) = %d, want 0", got)
		}
	})
	t.Run("无 EXIF 的 JPEG 返回 0", func(t *testing.T) {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
			t.Fatalf("encode: %v", err)
		}
		if got := jpegOrientation(bytes.NewReader(buf.Bytes())); got != 0 {
			t.Errorf("jpegOrientation(no exif) = %d, want 0", got)
		}
	})
	t.Run("非法 orientation 值返回 0", func(t *testing.T) {
		if got := jpegOrientation(bytes.NewReader(buildSOIAPP1(t, 9))); got != 0 {
			t.Errorf("jpegOrientation(9) = %d, want 0", got)
		}
	})
}

// buildSOIAPP1 构造 SOI + APP1 的最小字节流，直接喂给 jpegOrientation。
func buildSOIAPP1(t *testing.T, orientation uint16) []byte {
	t.Helper()

	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	buf.Write(buildEXIFAPP1(orientation))
	return buf.Bytes()
}

// TestProbeLogicalDimensions 锁定 Probe 的逻辑尺寸 invariant：
// orientation 5-8 交换宽高，1-4 保持。
func TestProbeLogicalDimensions(t *testing.T) {
	tests := []struct {
		orientation uint16
		wantW       int
		wantH       int
	}{
		{1, 4, 8},
		{3, 4, 8},
		{6, 8, 4},
		{8, 8, 4},
		{5, 8, 4},
	}
	for _, tt := range tests {
		t.Run(string(rune('0'+tt.orientation)), func(t *testing.T) {
			path := writeOrientedJPEG(t, tt.orientation, 4, 8)
			info, err := Probe(path)
			if err != nil {
				t.Fatalf("Probe: %v", err)
			}
			if info.PhysicalWidth != 4 || info.PhysicalHeight != 8 {
				t.Errorf("physical = %dx%d, want 4x8", info.PhysicalWidth, info.PhysicalHeight)
			}
			if info.Width != tt.wantW || info.Height != tt.wantH {
				t.Errorf("logical = %dx%d, want %dx%d", info.Width, info.Height, tt.wantW, tt.wantH)
			}
			if info.Orientation != int(tt.orientation) {
				t.Errorf("orientation = %d, want %d", info.Orientation, tt.orientation)
			}
		})
	}
}

// TestProbeMatchesOpenStaticBounds 是本轮统一的核心 invariant：
// Probe 的逻辑尺寸必须等于 OpenStatic 解码后的 image.Bounds()，
// 否则资源准入（基于 Probe）与执行结果（基于 decode）会漂移。
func TestProbeMatchesOpenStaticBounds(t *testing.T) {
	for _, orientation := range []uint16{1, 3, 6, 8} {
		t.Run(string(rune('0'+orientation)), func(t *testing.T) {
			path := writeOrientedJPEG(t, orientation, 4, 8)

			info, err := Probe(path)
			if err != nil {
				t.Fatalf("Probe: %v", err)
			}
			img, err := OpenStatic(path)
			if err != nil {
				t.Fatalf("OpenStatic: %v", err)
			}
			bounds := img.Bounds()
			if info.Width != bounds.Dx() || info.Height != bounds.Dy() {
				t.Errorf("probe logical %dx%d != decoded bounds %dx%d (orientation %d)",
					info.Width, info.Height, bounds.Dx(), bounds.Dy(), orientation)
			}
		})
	}
}

// TestOpenStaticRejectsNonStaticFormats GIF 输入被格式契约拒绝，
// 不再静默处理首帧。
func TestOpenStaticRejectsNonStaticFormats(t *testing.T) {
	var buf bytes.Buffer
	palette := color.Palette{color.White, color.Black}
	if err := gif.Encode(&buf, image.NewPaletted(image.Rect(0, 0, 4, 4), palette), nil); err != nil {
		t.Fatalf("encode gif: %v", err)
	}
	path := filepath.Join(t.TempDir(), "a.gif")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if _, err := OpenStatic(path); err == nil {
		t.Fatal("expected gif input to be rejected")
	}
}

// TestOpenStaticAcceptsStaticFormats JPEG/PNG/WebP 均可通过。
func TestOpenStaticAcceptsStaticFormats(t *testing.T) {
	pngPath := filepath.Join(t.TempDir(), "a.png")
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 3, 5))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	if err := os.WriteFile(pngPath, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	img, err := OpenStatic(pngPath)
	if err != nil {
		t.Fatalf("OpenStatic(png): %v", err)
	}
	if img.Bounds() != image.Rect(0, 0, 3, 5) {
		t.Errorf("bounds = %v, want 3x5", img.Bounds())
	}
}
