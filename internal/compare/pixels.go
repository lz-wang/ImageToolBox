package compare

import (
	"context"
	"image"
)

const (
	// opaqueChannelCount 是两张图片都不含透明度时的活动通道数（R/G/B）。
	opaqueChannelCount = 3

	// alphaChannelCount 是任一图片存在 alpha != 255 时的活动通道数
	//（premultiplied R/G/B + A）。
	alphaChannelCount = 4
)

// activeChannelCount 决定两张图片比较时使用的活动通道数。
//
// 两张图片的样本统一为 0..255 动态范围的行优先 float32 平面：
//
//   - 都不透明时活动通道为 R/G/B；
//   - 任一图片存在 alpha != 255 时活动通道变为 premultiplied R/G/B 加 A。
//     完全透明区域隐藏的 RGB 不再影响结果，而 alpha 丢失仍然会被检测。
//     注意这是 itb 定义的 alpha-aware RGBA 变体，数值不应要求与只比较
//     RGB 的第三方工具逐位一致。
func activeChannelCount(src, dst image.Image) int {
	if hasTransparency(src) || hasTransparency(dst) {
		return alphaChannelCount
	}
	return opaqueChannelCount
}

// hasTransparency 报告图片是否存在 alpha != 255 的像素。
func hasTransparency(img image.Image) bool {
	switch im := img.(type) {
	case *image.NRGBA:
		for i := 3; i < len(im.Pix); i += 4 {
			if im.Pix[i] != 0xFF {
				return true
			}
		}
		return false
	case *image.RGBA:
		for i := 3; i < len(im.Pix); i += 4 {
			if im.Pix[i] != 0xFF {
				return true
			}
		}
		return false
	case *image.YCbCr, *image.Gray, *image.Gray16, *image.CMYK:
		// 这些类型没有 alpha 通道，天然不透明；直接返回避免对大图做
		// 一次完整的 At() 遍历（JPEG 解码结果就是 *image.YCbCr）。
		_ = im
		return false
	}

	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0xFFFF {
				return true
			}
		}
	}
	return false
}

// extractChannel 把图片第 c 个活动通道写入 plane（行优先，0..255 动态
// 范围）。premultiply 为 true 时 R/G/B（c < 3）写入 alpha 预乘值，且 A
// 本身作为第四个通道（c == 3）参与比较。
//
// 调用方逐通道调用并复用同一 plane，比较的峰值工作集只是一对通道
// 平面，而不是全部 6/8 个平面的物化。
func extractChannel(ctx context.Context, img image.Image, plane []float32, c int, premultiply bool) error {
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()

	switch im := img.(type) {
	case *image.NRGBA:
		for y := 0; y < height; y++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			off := im.PixOffset(b.Min.X, b.Min.Y+y)
			for x := 0; x < width; x++ {
				i := off + x*4
				var v uint8
				if c < 3 {
					if premultiply {
						v = premultiply8(im.Pix[i+c], im.Pix[i+3])
					} else {
						v = im.Pix[i+c]
					}
				} else {
					v = im.Pix[i+3]
				}
				plane[y*width+x] = float32(v)
			}
		}
		return nil

	case *image.RGBA:
		// RGBA 的存储本身就是 alpha 预乘的：不透明像素（A=255）的
		// 预乘值等于原值，直接复制即可。
		for y := 0; y < height; y++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			off := im.PixOffset(b.Min.X, b.Min.Y+y)
			for x := 0; x < width; x++ {
				i := off + x*4
				var v uint8
				if c < 3 {
					v = im.Pix[i+c]
				} else {
					v = im.Pix[i+3]
				}
				plane[y*width+x] = float32(v)
			}
		}
		return nil
	}

	// 通用路径：color.Color.RGBA() 对任意实现统一返回 16 位 alpha 预乘
	// 值；不透明时预乘值等于原值，与上面的快速路径一致。
	for y := 0; y < height; y++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		for x := 0; x < width; x++ {
			r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			var v uint32
			switch c {
			case 0:
				v = r
			case 1:
				v = g
			case 2:
				v = bl
			default:
				v = a
			}
			plane[y*width+x] = float32(v >> 8)
		}
	}
	return nil
}

// premultiply8 计算 8 位 alpha 预乘值。舍入行为与标准库 color.NRGBA.RGBA()
// 完全一致（16 位中间量、截断除法），保证快速路径与通用路径等价。
func premultiply8(c, a uint8) uint8 {
	v := uint32(c)
	v |= v << 8
	v *= uint32(a)
	v /= 0xFF
	return uint8(v >> 8)
}
