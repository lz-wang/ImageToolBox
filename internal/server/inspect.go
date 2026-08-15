package server

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"imagetoolbox/internal/inspect"
)

// handleInspect 返回结构化的图片元数据 JSON。
// CLI 的 --format table/plain 是表现层关注点，Web API 只返回结构化结果。
func handleInspect(c *gin.Context) {
	dir, cleanup, err := newRequestDir("itb-inspect")
	if err != nil {
		fail(c, http.StatusInternalServerError, "%v", err)
		return
	}
	defer cleanup()

	inputPath, ok := requireFormFile(c, dir, "file")
	if !ok {
		return
	}

	result, err := inspect.File(inputPath, inspect.Options{Detail: true})
	if err != nil {
		fail(c, http.StatusBadRequest, "检查失败: %v", err)
		return
	}

	// 不向浏览器暴露服务端临时路径
	result.File.Path = filepath.Base(result.File.Path)
	result.File.AbsPath = ""

	c.JSON(http.StatusOK, result)
}
