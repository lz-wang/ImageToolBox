// Package server 提供 itb WebUI 的 HTTP 服务。
//
// WebUI 不经由 CLI 子进程，而是直接调用各领域包（resize/convert/crop/
// watermark/compress/inspect/batch/s3/lsky），静态资源与 API 共用一个 Handler。
package server

import (
	"context"
	"io/fs"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"imagetoolbox/internal/s3"
)

// Server WebUI 服务器，聚合 API 路由与内嵌的前端静态资源。
type Server struct {
	staticFS fs.FS

	// s3Client 在 Server 生命周期内只创建一次，所有 S3 handler 共享
	// 同一个 http.Client/Transport 与连接池（MaxIdleConns 等才真正生效）。
	s3Mu     sync.Mutex
	s3Client *s3.Client
}

// New 创建 Server。staticFS 为前端构建产物（web/dist）的根目录。
func New(staticFS fs.FS) *Server {
	return &Server{staticFS: staticFS}
}

// sharedS3Client 返回复用的 S3 客户端；未配置时返回错误。
// 创建失败不缓存，下一个请求会重试，避免瞬时失败被固化。
func (s *Server) sharedS3Client(ctx context.Context) (*s3.Client, error) {
	s.s3Mu.Lock()
	defer s.s3Mu.Unlock()

	if s.s3Client != nil {
		return s.s3Client, nil
	}

	cfg := &s3.Config{}
	cfg.LoadFromEnv()
	client, err := s3.NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	s.s3Client = client
	return client, nil
}

// Handler 返回完整的 http.Handler，可直接用于 http.Server 或 httptest。
func (s *Server) Handler() http.Handler {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	api := r.Group("/api/v1")
	s.registerAPIRoutes(api)

	r.NoRoute(s.handleWeb)

	return r
}
