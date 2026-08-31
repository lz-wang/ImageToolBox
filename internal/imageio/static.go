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
//  2. imaging.Open(AutoOrientation(true)) 把 JPEG EXIF Orientation
//     烘焙进实际像素。
//
// 解码结果的 image.Bounds() 与 Probe 报告的逻辑 Width/Height 一致，
// 调用方（CLI/HTTP）基于 Probe 做的资源准入与最终输出不再漂移。
func OpenStatic(path string) (image.Image, error) {
	if _, err := DetectFormat(path); err != nil {
		return nil, fmt.Errorf("unsupported input image: %w", err)
	}
	img, err := imaging.Open(path, imaging.AutoOrientation(true))
	if err != nil {
		return nil, fmt.Errorf("open input image: %w", err)
	}
	return img, nil
}
