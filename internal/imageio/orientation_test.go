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
	// 覆盖全部 orientation 值：1-4 保持宽高，5-8 交换，
	// 逐一锁定 applyOrientation 与 Probe 的宽高语义一致。
	for _, orientation := range []uint16{1, 2, 3, 4, 5, 6, 7, 8} {
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

// TestExifOrientationRequiresExifHeader 锁定 parser 健壮性：APP1 payload
// 必须以 "Exif\x00\x00" 开头才被当作 EXIF 解析，防止其他 APP1 数据
// 在偏移 6 处巧合构成合法 TIFF 头而被误读出 orientation。
func TestExifOrientationRequiresExifHeader(t *testing.T) {
	var fakeTIFF bytes.Buffer
	fakeTIFF.WriteString("II")
	_ = binary.Write(&fakeTIFF, binary.LittleEndian, uint16(42))
	_ = binary.Write(&fakeTIFF, binary.LittleEndian, uint32(8)) // IFD0 偏移
	_ = binary.Write(&fakeTIFF, binary.LittleEndian, uint16(1)) // 条目数
	_ = binary.Write(&fakeTIFF, binary.LittleEndian, uint16(0x0112))
	_ = binary.Write(&fakeTIFF, binary.LittleEndian, uint16(3))
	_ = binary.Write(&fakeTIFF, binary.LittleEndian, uint32(1))
	_ = binary.Write(&fakeTIFF, binary.LittleEndian, uint16(6)) // orientation 6
	_ = binary.Write(&fakeTIFF, binary.LittleEndian, uint16(0))
	_ = binary.Write(&fakeTIFF, binary.LittleEndian, uint32(0))

	payload := append([]byte("NOTEXI"), fakeTIFF.Bytes()...)

	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	buf.Write(buildArbitraryAPP1(payload))
	if got := jpegOrientation(bytes.NewReader(buf.Bytes())); got != 0 {
		t.Errorf("jpegOrientation(非 Exif 头 APP1) = %d, want 0（不得把巧合的 TIFF 结构当 EXIF）", got)
	}
}

// buildArbitraryAPP1 构造携带任意 payload 的 APP1 段（非 EXIF，如 XMP）。
func buildArbitraryAPP1(payload []byte) []byte {
	seg := bytes.NewBuffer(nil)
	seg.WriteByte(0xFF)
	seg.WriteByte(0xE1)
	_ = binary.Write(seg, binary.BigEndian, uint16(2+len(payload)))
	seg.Write(payload)
	return seg.Bytes()
}

// writeJPEGWithPrefixAPP1 生成在 SOI 与 EXIF APP1 之间插入一个前置
// APP1 段（如 XMP）的真实 JPEG，物理尺寸 width×height。
func writeJPEGWithPrefixAPP1(t *testing.T, prefixPayload []byte, orientation uint16, width, height int) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var body bytes.Buffer
	if err := jpeg.Encode(&body, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	encoded := body.Bytes()

	var out bytes.Buffer
	out.Write(encoded[:2]) // SOI
	out.Write(buildArbitraryAPP1(prefixPayload))
	out.Write(buildEXIFAPP1(orientation))
	out.Write(encoded[2:])

	path := filepath.Join(t.TempDir(), "prefixed.jpg")
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// TestProbeMatchesOpenStaticBoundsAfterNonEXIFAPP1 锁定 multi-APP1 边界：
// EXIF APP1 之前存在非 EXIF APP1（XMP 等）时，Probe 与 OpenStatic
// 仍必须报告一致的 orientation 与逻辑尺寸。
//
// imaging.readOrientation 只检查第一个 APP1：不是 EXIF 就直接放弃，
// 而 jpegOrientation 会继续扫描后续 APP1。若 OpenStatic 依赖
// imaging.AutoOrientation，这类文件会让两侧读到的 orientation 不同，
// invariant 失效——OpenStatic 必须与 Probe 使用同一个 parser。
func TestProbeMatchesOpenStaticBoundsAfterNonEXIFAPP1(t *testing.T) {
	xmpPayload := append(
		[]byte("http://ns.adobe.com/xap/1.0/\x00"),
		[]byte(`<x:xmpmeta xmlns:x="adobe:ns:meta/"></x:xmpmeta>`)...,
	)

	tests := []struct {
		name   string
		prefix []byte
	}{
		{"XMP APP1 后跟 EXIF APP1", xmpPayload},
		{"非 EXIF APP1 后跟 EXIF APP1", []byte("arbitrary-extended-metadata")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeJPEGWithPrefixAPP1(t, tt.prefix, 6, 4, 8)

			info, err := Probe(path)
			if err != nil {
				t.Fatalf("Probe: %v", err)
			}
			if info.Orientation != 6 {
				t.Fatalf("Probe orientation = %d, want 6（parser 应跳过前置非 EXIF APP1 继续扫描）", info.Orientation)
			}

			img, err := OpenStatic(path)
			if err != nil {
				t.Fatalf("OpenStatic: %v", err)
			}
			bounds := img.Bounds()
			if info.Width != bounds.Dx() || info.Height != bounds.Dy() {
				t.Errorf("probe logical %dx%d != decoded bounds %dx%d：OpenStatic 与 Probe 必须使用同一 orientation parser",
					info.Width, info.Height, bounds.Dx(), bounds.Dy())
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
