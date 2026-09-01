// Package rotate 提供任意角度的图像旋转领域能力。
//
// 契约：
//   - 正角度 = 逆时针，负角度 = 顺时针（与 watermark 的 imaging.Rotate 语义一致）；
//   - 精确 90/180/270 走无插值的离散旋转路径，其余角度为 bilinear 插值；
//   - 任意角度会自动扩大画布以完整容纳旋转后的图像，未覆盖像素透明；
//   - Resolve 推导的输出尺寸与 Apply 的真实输出 bounds 恒等（测试锁定），
//     HTTP 据此在分配画布之前完成资源准入。
package rotate

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/disintegration/imaging"
	"imagetoolbox/internal/imageio"
)

// Options 是 rotate 的领域选项。Angle 单位为度：正数逆时针、负数顺时针，
// 允许小数；必须在 (-360, 360) 开区间内且不能为 0——0/±360 不会改变像素，
// 只会引入一次没有意义的有损重编码。
type Options struct {
	Angle float64
}

// Plan 描述一次旋转的执行参数：归一化到 [0, 360) 的角度与输出画布尺寸。
type Plan struct {
	Angle  float64
	Width  int
	Height int
}

// Validate 在角度归一化之前校验用户参数：拒绝 NaN、±Inf、0 与 (-360, 360)
// 之外的值。
func (o Options) Validate() error {
	if math.IsNaN(o.Angle) || math.IsInf(o.Angle, 0) {
		return fmt.Errorf("angle 必须是有限数值（当前: %v）", o.Angle)
	}
	if o.Angle == 0 {
		return fmt.Errorf("angle 不能为 0（当前: %v）", o.Angle)
	}
	if o.Angle >= 360 || o.Angle <= -360 {
		return fmt.Errorf("angle 必须在 (-360, 360) 范围内且不能为 0（当前: %v）", o.Angle)
	}
	return nil
}

// Resolve 从源图尺寸推导旋转后的输出尺寸。Apply 内部调用同一函数，
// 因此对 Resolve 结果做限制检查等价于对最终输出做限制检查。
// 用户参数校验发生在角度归一化之前。
func Resolve(bounds image.Rectangle, opts Options) (Plan, error) {
	if err := opts.Validate(); err != nil {
		return Plan{}, err
	}

	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return Plan{}, fmt.Errorf("图片尺寸无效: %dx%d", width, height)
	}

	angle := normalizeAngle(opts.Angle)
	var outWidth, outHeight int
	switch angle {
	case 90, 270:
		outWidth, outHeight = height, width
	case 180:
		outWidth, outHeight = width, height
	default:
		outWidth, outHeight = rotatedSize(width, height, angle)
	}
	return Plan{Angle: angle, Width: outWidth, Height: outHeight}, nil
}

// Apply 对已解码图片执行旋转。精确 90/180/270 显式分派无插值的离散路径，
// 把"正交旋转不得插值"固化为领域层可读契约；其余角度交给 imaging.Rotate
// 做双线性插值，未覆盖区域使用透明背景（最终是否铺底由 imageio.Save 按
// 目标格式决定：PNG/WebP 保持透明，JPEG 铺白色）。
func Apply(img image.Image, opts Options) (image.Image, error) {
	plan, err := Resolve(img.Bounds(), opts)
	if err != nil {
		return nil, err
	}

	switch plan.Angle {
	case 90:
		return imaging.Rotate90(img), nil
	case 180:
		return imaging.Rotate180(img), nil
	case 270:
		return imaging.Rotate270(img), nil
	default:
		return imaging.Rotate(img, plan.Angle, color.NRGBA{}), nil
	}
}

// RotateFile 读取输入文件、旋转并保存，与 resize/crop 共用同一 transform
// 链路：RejectSameFile → imageio.OpenStatic → Apply → imageio.Save。
// 输入统一走 OpenStatic：严格限定 JPEG/PNG/WebP，JPEG EXIF Orientation
// 先烘焙进像素，用户旋转作用在归一化后的逻辑像素上。
func RotateFile(inputPath, outputPath string, opts Options) error {
	if err := imageio.RejectSameFile(inputPath, outputPath); err != nil {
		return err
	}

	img, err := imageio.OpenStatic(inputPath)
	if err != nil {
		return fmt.Errorf("打开输入图片失败: %w", err)
	}

	rotated, err := Apply(img, opts)
	if err != nil {
		return err
	}

	if err := imageio.Save(outputPath, rotated, imageio.SaveOptions{
		Quality:    100,
		Background: imageioMustColor("#FFFFFF"),
	}); err != nil {
		return fmt.Errorf("保存旋转结果失败: %w", err)
	}
	return nil
}

// normalizeAngle 把角度折叠到 [0, 360)，与 imaging.Rotate 内部的归一化
// 公式逐字节一致（-90 → 270，即顺时针 90°）。
func normalizeAngle(angle float64) float64 {
	return angle - math.Floor(angle/360)*360
}

// rotatePoint 与 imaging v1.6.2 transform.go 的同名私有函数一致：
// 围绕原点逆时针旋转。
func rotatePoint(x, y, sin, cos float64) (float64, float64) {
	return x*cos - y*sin, x*sin + y*cos
}

// rotatedSize 复刻 imaging v1.6.2 rotatedSize 的尺寸推导语义：旋转
// (0,0)/(w-1,0)/(w-1,h-1)/(0,h-1) 四个角点取包围盒，分数部分大于 0.1 时
// 进位。公式必须与库保持一致，任何偏差都会破坏 "Resolve 尺寸 == Apply
// 输出 bounds" 的 invariant（测试锁定），升级 imaging 后如有漂移测试会
// 立即暴露。
func rotatedSize(w, h int, angle float64) (int, int) {
	if w <= 0 || h <= 0 {
		return 0, 0
	}

	sin, cos := math.Sincos(math.Pi * angle / 180)
	x1, y1 := rotatePoint(float64(w-1), 0, sin, cos)
	x2, y2 := rotatePoint(float64(w-1), float64(h-1), sin, cos)
	x3, y3 := rotatePoint(0, float64(h-1), sin, cos)

	minx := math.Min(x1, math.Min(x2, math.Min(x3, 0)))
	maxx := math.Max(x1, math.Max(x2, math.Max(x3, 0)))
	miny := math.Min(y1, math.Min(y2, math.Min(y3, 0)))
	maxy := math.Max(y1, math.Max(y2, math.Max(y3, 0)))

	neww := maxx - minx + 1
	if neww-math.Floor(neww) > 0.1 {
		neww++
	}
	newh := maxy - miny + 1
	if newh-math.Floor(newh) > 0.1 {
		newh++
	}

	return int(neww), int(newh)
}

func imageioMustColor(hex string) color.NRGBA {
	col, err := imageio.ParseHexColor(hex)
	if err != nil {
		panic(err)
	}
	return col
}
