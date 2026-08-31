package imageio

import (
	"fmt"
	"image"

	"github.com/disintegration/imaging"
)

// OpenStatic 打开受支持的静态图片输入（JPEG/PNG/WebP），是所有
// transform（convert/resize/crop/watermark）的唯一解码入口：
//
//  1. 先用 DetectFormat 做格式契约检查——GIF/BMP/TIFF 等格式一律拒绝。
//     imaging 本身能解码它们，但放行会造成 animated GIF → 静默处理
//     首帧这类语义损失；
//  2. JPEG 的 EXIF Orientation 用与 Probe 相同的 jpegOrientation 解析
//     （fileJPEGOrientation），再由 applyOrientation 烘焙进实际像素。
//
// 不使用 imaging.AutoOrientation：imaging.readOrientation 只检查第一个
// APP1（不是 EXIF 就放弃），而 jpegOrientation 会扫描全部 APP1，两个
// parser 在"XMP APP1 在 EXIF APP1 之前"的 JPEG 上结果不同。共用一个
// parser 后，解码结果的 image.Bounds() 与 Probe 报告的逻辑
// Width/Height 一致才是结构保证。
func OpenStatic(path string) (image.Image, error) {
	format, err := DetectFormat(path)
	if err != nil {
		return nil, fmt.Errorf("unsupported input image: %w", err)
	}
	img, err := imaging.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open input image: %w", err)
	}
	if format == FormatJPEG {
		return applyOrientation(img, fileJPEGOrientation(path)), nil
	}
	return img, nil
}

// applyOrientation 把 EXIF Orientation（1-8）烘焙进像素，其他值原样返回。
// 变换映射与 EXIF 规范一致（EXIF 描述相机旋转，需用反向变换补偿）：
// 2 水平翻转、3 旋转 180°、4 垂直翻转、5 转置、6 顺时针 90°、
// 7 反转置、8 逆时针 90°。
func applyOrientation(img image.Image, orientation int) image.Image {
	switch orientation {
	case 2:
		return imaging.FlipH(img)
	case 3:
		return imaging.Rotate180(img)
	case 4:
		return imaging.FlipV(img)
	case 5:
		return imaging.Transpose(img)
	case 6:
		return imaging.Rotate270(img)
	case 7:
		return imaging.Transverse(img)
	case 8:
		return imaging.Rotate90(img)
	default:
		return img
	}
}
