package server

import (
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
)

// fail 以统一的 JSON 结构返回错误，前端从 error 字段读取消息。
func fail(c *gin.Context, status int, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	c.AbortWithStatusJSON(status, gin.H{"error": message})
}

// serveImageFile 把处理产物以图片二进制流返回：
// Content-Disposition 附件下载 + X-ITB-*-Size 头，同一响应即可预览/下载/展示体积。
func serveImageFile(c *gin.Context, outputPath string, inputSize int64, downloadName string) {
	data, err := os.ReadFile(outputPath)
	if err != nil {
		fail(c, http.StatusInternalServerError, "读取处理结果失败: %v", err)
		return
	}

	contentType := mime.TypeByExtension(filepath.Ext(outputPath))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	c.Header("Content-Disposition", contentDisposition(downloadName))
	c.Header("X-ITB-Input-Size", strconv.FormatInt(inputSize, 10))
	c.Header("X-ITB-Output-Size", strconv.FormatInt(int64(len(data)), 10))
	c.Data(http.StatusOK, contentType, data)
}

// contentDisposition 为非 ASCII 文件名提供 RFC 5987 的 UTF-8 编码，
// 避免浏览器或 Fetch 将传统 filename 参数错误解码为乱码。
func contentDisposition(downloadName string) string {
	for _, r := range downloadName {
		if r > 0x7f {
			fallback := "download" + filepath.Ext(downloadName)
			return fmt.Sprintf(
				"attachment; filename=%q; filename*=UTF-8''%s",
				fallback,
				url.PathEscape(downloadName),
			)
		}
	}

	return fmt.Sprintf("attachment; filename=%q", downloadName)
}
