package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// registerAPIRoutes 注册 /api/v1 下的所有 API 路由。
func registerAPIRoutes(api *gin.RouterGroup) {
	api.GET("/health", handleHealth)

	api.POST("/compress", handleCompress)
	api.POST("/resize", handleResize)
	api.POST("/crop", handleCrop)
	api.POST("/convert", handleConvert)
	api.POST("/watermark", handleWatermark)
	api.POST("/inspect", handleInspect)
}

func handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
