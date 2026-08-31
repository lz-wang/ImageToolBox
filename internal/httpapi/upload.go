package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// newRequestDir 为单次请求创建独立临时目录，返回清理函数供 defer 调用。
func newRequestDir(prefix string) (string, func(), error) {
	dir, err := os.MkdirTemp("", prefix+"-*")
	if err != nil {
		return "", nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	return dir, func() { os.RemoveAll(dir) }, nil
}

// sanitizeFilename 只保留文件名部分，屏蔽客户端传入的路径分隔符。
func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimLeft(filepath.Base(name), ".")
	if name == "" || name == "/" {
		return ""
	}
	return name
}

// saveFormFile 把 multipart 表单字段保存到 dir 下，返回保存路径。
// 字段缺失时不报错，返回空字符串。
func saveFormFile(c *gin.Context, dir, field string) (string, error) {
	fh, err := c.FormFile(field)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return "", nil
		}
		return "", fmt.Errorf("读取上传字段 %s 失败: %w", field, err)
	}

	name := sanitizeFilename(fh.Filename)
	if name == "" {
		name = field + ".bin"
	}

	src, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("打开上传字段 %s 失败: %w", field, err)
	}
	defer src.Close()

	dst := filepath.Join(dir, name)
	out, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("保存上传文件失败: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, src); err != nil {
		return "", fmt.Errorf("保存上传文件失败: %w", err)
	}
	return dst, nil
}

// requireFormFile 保存必需的上传字段；缺失或失败时已写回 400 响应。
func requireFormFile(c *gin.Context, dir, field string) (string, bool) {
	path, err := saveFormFile(c, dir, field)
	if err != nil {
		fail(c, http.StatusBadRequest, "%v", err)
		return "", false
	}
	if path == "" {
		fail(c, http.StatusBadRequest, "缺少上传字段: %s", field)
		return "", false
	}
	return path, true
}

// optionalFormFile 同 requireFormFile，但字段缺失不算错误。
func optionalFormFile(c *gin.Context, dir, field string) (string, bool) {
	path, err := saveFormFile(c, dir, field)
	if err != nil {
		fail(c, http.StatusBadRequest, "%v", err)
		return "", false
	}
	return path, true
}

// bindOptions 把 multipart 的 options 字段（JSON 字符串）解析为请求结构体；
// 未提供时使用零值，结构体内部自行处理默认值。
func bindOptions[T any](c *gin.Context) (T, bool) {
	var opts T
	raw := c.PostForm("options")
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &opts); err != nil {
			fail(c, http.StatusBadRequest, "options 参数无效: %v", err)
			return opts, false
		}
	}
	return opts, true
}
