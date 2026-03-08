package compress

import (
	"image"
	"io"
)

// DetectFormat 检测图片格式，返回格式名称（jpeg, png 等）
func DetectFormat(r io.ReadSeeker) (string, error) {
	_, format, err := image.DecodeConfig(r)
	if err != nil {
		return "", err
	}
	r.Seek(0, io.SeekStart)
	return format, nil
}
