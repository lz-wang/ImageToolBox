// Package compare 提供只读图片质量指标（PSNR/SSIM/MS-SSIM）的纯 Go 实现。
//
// compare 是显式调用的只读分析命令的领域层：不修改任何输入文件，
// 也绝不隐式 resize/crop/pad 尺寸不一致的图片，指标数学定义保持稳定。
package compare

import (
	"context"
	"fmt"
	"image"
)

const (
	// opaqueChannelCount 是两张图片都不含透明度时的活动通道数（R/G/B）。
	opaqueChannelCount = 3

	// alphaChannelCount 是任一图片存在 alpha != 255 时的活动通道数
	//（premultiplied R/G/B + A）。
	alphaChannelCount = 4
)

// pixelPlanes 是一对图片归一化后的比较平面。
//
// 两张图片的样本统一为 0..255 动态范围的行优先 float32 平面：
//
//   - 都不透明时活动通道为 R/G/B；
//   - 任一图片存在 alpha != 255 时活动通道变为 premultiplied R/G/B 加 A。
//     完全透明区域隐藏的 RGB 不再影响结果，而 alpha 丢失仍然会被检测。
//     注意这是 itb 定义的 alpha-aware RGBA 变体，数值不应要求与只比较
//     RGB 的第三方工具逐位一致。
type pixelPlanes struct {
	width    int
	height   int
	channels int

	// src/dst[c] 是第 c 个活动通道的样本，长度均为 width*height。
	src [][]float32
	dst [][]float32
}

// newPixelPlanes 校验两张图片的逻辑尺寸一致并提取活动通道平面。
// 输入应已完成解码（含 JPEG EXIF Orientation 烘焙），这里比较的是
// 应用 Orientation 后的实际视觉像素，而不是文件字节或编码参数。
func newPixelPlanes(ctx context.Context, src, dst image.Image) (*pixelPlanes, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	sb, db := src.Bounds(), dst.Bounds()
	if sb.Dx() != db.Dx() || sb.Dy() != db.Dy() {
		return nil, fmt.Errorf("图片尺寸不一致: src=%dx%d, dst=%dx%d", sb.Dx(), sb.Dy(), db.Dx(), db.Dy())
	}

	channels := opaqueChannelCount
	if hasTransparency(src) || hasTransparency(dst) {
		channels = alphaChannelCount
	}

	p := &pixelPlanes{
		width:    sb.Dx(),
		height:   sb.Dy(),
		channels: channels,
		src:      make([][]float32, channels),
		dst:      make([][]float32, channels),
	}
	for c := 0; c < channels; c++ {
		p.src[c] = make([]float32, p.width*p.height)
		p.dst[c] = make([]float32, p.width*p.height)
	}

	if err := extractChannels(ctx, src, p.src, channels == alphaChannelCount); err != nil {
		return nil, err
	}
	if err := extractChannels(ctx, dst, p.dst, channels == alphaChannelCount); err != nil {
		return nil, err
	}
	return p, nil
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

// extractChannels 把图片的活动通道写入平面（行优先，0..255 动态范围）。
// premultiply 为 true 时 R/G/B 写入 alpha 预乘值，且 A 本身作为第四个
// 通道参与比较。
func extractChannels(ctx context.Context, img image.Image, planes [][]float32, premultiply bool) error {
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
				idx := y*width + x
				if premultiply {
					planes[0][idx] = float32(premultiply8(im.Pix[i], im.Pix[i+3]))
					planes[1][idx] = float32(premultiply8(im.Pix[i+1], im.Pix[i+3]))
					planes[2][idx] = float32(premultiply8(im.Pix[i+2], im.Pix[i+3]))
					planes[3][idx] = float32(im.Pix[i+3])
					continue
				}
				planes[0][idx] = float32(im.Pix[i])
				planes[1][idx] = float32(im.Pix[i+1])
				planes[2][idx] = float32(im.Pix[i+2])
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
				idx := y*width + x
				planes[0][idx] = float32(im.Pix[i])
				planes[1][idx] = float32(im.Pix[i+1])
				planes[2][idx] = float32(im.Pix[i+2])
				if premultiply {
					planes[3][idx] = float32(im.Pix[i+3])
				}
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
			idx := y*width + x
			planes[0][idx] = float32(r >> 8)
			planes[1][idx] = float32(g >> 8)
			planes[2][idx] = float32(bl >> 8)
			if premultiply {
				planes[3][idx] = float32(a >> 8)
			}
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
