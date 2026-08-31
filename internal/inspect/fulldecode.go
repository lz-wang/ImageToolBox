package inspect

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"os"
)

// applyFullDecode 对 info 执行完整解码并回填 FullDecodeOK / FrameCount /
// Animated / AnimationKnown：
//
//   - GIF：gif.DecodeAll 逐帧解码，FrameCount = len(g.Image)，
//     FrameCount > 1 → Animated；即使单帧 GIF 也能给出确定结论
//   - WebP：image.Decode 解码首帧；Animated 已由 VP8X 头嗅探给出，
//     帧数当前解码器不暴露
//   - JPEG/PNG 等静态格式：image.Decode 整图解码，动画状态恒为已知非动画
//
// 完整解码失败（文件头正常但后半部分损坏）时 FullDecodeOK 置为
// false 并返回错误，由调用方决定 strict 报错还是记 warning。
func applyFullDecode(path string, info *ImageInfo) error {
	info.FullDecodeOK = new(false)

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("打开图片失败: %w", err)
	}
	defer f.Close()

	switch info.Format {
	case "gif":
		g, err := gif.DecodeAll(f)
		if err != nil {
			return err
		}
		info.FrameCount = len(g.Image)
		info.Animated = len(g.Image) > 1
		info.AnimationKnown = true
	case "webp":
		if _, _, err := image.Decode(f); err != nil {
			return err
		}
		// Animated 已由 webpAnimation 嗅探填充；帧数不暴露
		info.AnimationKnown = true
	default:
		if _, _, err := image.Decode(f); err != nil {
			return err
		}
		info.Animated = false
		info.AnimationKnown = true
	}

	info.FullDecodeOK = new(true)
	return nil
}

// webpAnimation 从 RIFF/WEBP 文件头嗅探动画标记：动画 WebP 的第一个
// chunk 必然是 VP8X，其 flags 字节的 bit1（0x02）是 Animation 位；
// 纯 VP8/VP8L 静态图没有 VP8X，同样可以断言非动画。
// known 为 false 表示 header 不足以判断（不是 WebP 或太短）。
func webpAnimation(header []byte) (animated, known bool) {
	// 布局：0-3 "RIFF"、4-7 size、8-11 "WEBP"、12-15 第一个 chunk
	// 四字符、16-19 chunk size、20 起 chunk payload（VP8X 首字节是 flags）
	if len(header) < 21 {
		return false, false
	}
	if !bytes.HasPrefix(header, []byte("RIFF")) || !bytes.Equal(header[8:12], []byte("WEBP")) {
		return false, false
	}
	if !bytes.Equal(header[12:16], []byte("VP8X")) {
		return false, true
	}
	return header[20]&0x02 != 0, true
}
