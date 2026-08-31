package compress

import (
	"errors"
	"image"
	"io"
)

// ErrUnsupportedFormat 表示格式不在 compress 支持的集合（PNG/JPEG）内。
var ErrUnsupportedFormat = errors.New("不支持的图片格式")

// DetectFormat 检测图片格式，返回格式名称（jpeg, png 等）
func DetectFormat(r io.ReadSeeker) (string, error) {
	_, format, err := image.DecodeConfig(r)
	if err != nil {
		return "", err
	}
	r.Seek(0, io.SeekStart)
	return format, nil
}
