package server

import (
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// registerAPIRoutes 注册 /api/v1 下的所有 API 路由。
func (s *Server) registerAPIRoutes(api *gin.RouterGroup) {
	api.GET("/health", handleHealth)

	api.POST("/compress", handleCompress)
	api.POST("/resize", handleResize)
	api.POST("/crop", handleCrop)
	api.POST("/convert", handleConvert)
	api.POST("/watermark", handleWatermark)
	api.POST("/inspect", handleInspect)

	api.POST("/batch/resize", handleBatchResize)
	api.POST("/batch/convert", handleBatchConvert)
	api.POST("/batch/watermark", handleBatchWatermark)

	// 存储后端：资源式接口，凭证仅从服务端环境变量读取
	api.GET("/s3/status", s.handleS3Status)
	api.GET("/s3/objects", s.handleS3List)
	api.POST("/s3/objects", s.handleS3Upload)
	api.GET("/s3/objects/download", s.handleS3Download)
	api.GET("/s3/objects/info", s.handleS3Stat)
	api.DELETE("/s3/objects", s.handleS3Delete)

	api.POST("/lsky/images", handleLskyUpload)
}

func handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleWeb 提供 SPA 静态资源服务：先按路径查找文件，
// 未命中且非 /api 路径时回退到 index.html，保证前端路由刷新可用。
func (s *Server) handleWeb(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
		return
	}

	name := strings.TrimPrefix(c.Request.URL.Path, "/")
	if name == "" {
		name = "index.html"
	}

	if _, err := fs.Stat(s.staticFS, name); err != nil {
		name = "index.html"
	}

	data, err := fs.ReadFile(s.staticFS, name)
	if err != nil {
		c.String(http.StatusNotFound, "WebUI 未构建：请先运行 make web 并重新编译 itb")
		return
	}

	contentType := mime.TypeByExtension(filepath.Ext(name))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	c.Data(http.StatusOK, contentType, data)
}
