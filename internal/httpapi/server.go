// Package httpapi 提供 itb HTTP API 服务。
//
// API 不经由 CLI 子进程，而是直接调用各领域包（resize/convert/crop/
// watermark/compress/inspect）。
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Config configures the HTTP adapter. Zero values use safe defaults.
type Config struct{}

// New creates the HTTP API handler.
func New(Config) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	registerAPIRoutes(r.Group("/api/v1"))
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
	})
	return r
}
