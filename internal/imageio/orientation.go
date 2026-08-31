package imageio

import (
	"encoding/binary"
	"io"
	"os"
)

// jpegOrientation 从 JPEG 流解析 EXIF Orientation（tag 0x0112），
// 返回 1-8；无法解析（无 EXIF、非 JPEG、格式错误）返回 0，
// 调用方按 1（不旋转）处理。仅 JPEG 携带 EXIF，WebP 的 orientation
// 元数据不处理。
//
// 这是 orientation 的唯一 parser：Probe（报告逻辑尺寸）与 OpenStatic
// （把 orientation 烘焙进像素）共用同一实现，"Probe 逻辑尺寸 ==
// OpenStatic 解码 bounds" 的 invariant 由结构保证，而不是寄望两个
// 独立实现恰好一致。
func jpegOrientation(r io.Reader) int {
	var head [2]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return 0
	}
	if head[0] != 0xFF || head[1] != 0xD8 {
		return 0
	}

	// 逐段扫描 APP1(Exif)；EXIF 必须出现在图像数据(SOS)之前
	for {
		var marker [2]byte
		if _, err := io.ReadFull(r, marker[:]); err != nil {
			return 0
		}
		if marker[0] != 0xFF {
			return 0
		}
		switch m := marker[1]; {
		case m == 0x01 || (m >= 0xD0 && m <= 0xD7):
			// 无长度填充段
			continue
		case m == 0xDA:
			// SOS：图像数据开始，后面没有 EXIF
			return 0
		}

		var lenBuf [2]byte
		if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
			return 0
		}
		segLen := int(lenBuf[0])<<8 | int(lenBuf[1])
		if segLen < 2 {
			return 0
		}

		if marker[1] == 0xE1 {
			data := make([]byte, segLen-2)
			if _, err := io.ReadFull(r, data); err != nil {
				return 0
			}
			if o, ok := exifOrientation(data); ok {
				return o
			}
			continue
		}
		if _, err := io.CopyN(io.Discard, r, int64(segLen-2)); err != nil {
			return 0
		}
	}
}

// exifOrientation 解析 APP1 payload 中的 Orientation。
// 布局："Exif\0\0" + TIFF 头（字节序标记 + 42 + IFD0 偏移）+ IFD0 条目；
// Orientation 是 SHORT 类型、count 为 1，值直接内联在条目的 value 字段。
func exifOrientation(data []byte) (int, bool) {
	// 必须先验证 "Exif\x00\x00" 前缀（6 字节）再取 TIFF 流：
	// APP1 也承载 XMP 等其他数据，直接从偏移 6 解析会把巧合构成
	// 合法 TIFF 头的非 EXIF payload 误读出 orientation。
	if len(data) < 14 || string(data[:6]) != "Exif\x00\x00" {
		return 0, false
	}
	tiff := data[6:]

	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0, false
	}
	if order.Uint16(tiff[2:4]) != 42 {
		return 0, false
	}
	ifd0Offset := order.Uint32(tiff[4:8])
	if int(ifd0Offset)+2 > len(tiff) {
		return 0, false
	}

	entries := int(order.Uint16(tiff[ifd0Offset : ifd0Offset+2]))
	for i := range entries {
		entryOff := int(ifd0Offset) + 2 + i*12
		if entryOff+12 > len(tiff) {
			return 0, false
		}
		entry := tiff[entryOff : entryOff+12]
		if order.Uint16(entry[0:2]) != 0x0112 { // Orientation
			continue
		}
		if order.Uint16(entry[2:4]) != 3 { // SHORT
			return 0, false
		}
		if order.Uint32(entry[4:8]) != 1 {
			return 0, false
		}
		value := int(order.Uint16(entry[8:10]))
		if value < 1 || value > 8 {
			return 0, false
		}
		return value, true
	}
	return 0, false
}

// swapsDimensions 报告 EXIF Orientation 是否属于 90°/270° 旋转族
//（5/6/7/8）：应用旋转后逻辑宽高与物理宽高互换。
func swapsDimensions(orientation int) bool {
	switch orientation {
	case 5, 6, 7, 8:
		return true
	default:
		return false
	}
}

// fileJPEGOrientation 打开文件解析 JPEG EXIF Orientation，无法解析
// （非 JPEG、无 EXIF、IO 失败）返回 0（调用方按 1 处理）。
// Probe 与 OpenStatic 共用，保证两侧读到的 orientation 必然相同。
func fileJPEGOrientation(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	return jpegOrientation(f)
}
