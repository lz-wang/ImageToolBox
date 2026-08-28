// Package server 提供 itb WebUI 的 HTTP 服务。
//
// WebUI 不经由 CLI 子进程，而是直接调用各领域包（resize/convert/crop/
// watermark/compress/inspect），静态资源与 API 共用一个 Handler。
package server

import (
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Server WebUI 服务器，聚合 API 路由与内嵌的前端静态资源。
type Server struct {
	staticFS fs.FS
}

// New 创建 Server。staticFS 为前端构建产物（web/dist）的根目录。
func New(staticFS fs.FS) *Server {
	return &Server{staticFS: staticFS}
}

// Handler 返回完整的 http.Handler，可直接用于 http.Server 或 httptest。
func (s *Server) Handler() http.Handler {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	api := r.Group("/api/v1")
	registerAPIRoutes(api)

	r.NoRoute(s.handleWeb)

	return r
}
