package httpapi

import "errors"

// HTTP adapter 准入阶段的哨兵错误。领域层错误（如 imageio.ErrUnsupportedFormat）
// 定义在各 domain 包中，这里只描述 HTTP admission 语义。

var (
	// ErrPayloadTooLarge 表示请求体或单个 multipart 字段超过服务限制。
	ErrPayloadTooLarge = errors.New("payload too large")
	// ErrImageTooLarge 表示图片尺寸或像素数超过服务限制。
	ErrImageTooLarge = errors.New("image too large")
)
