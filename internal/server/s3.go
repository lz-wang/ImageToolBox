package server

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gin-gonic/gin"
	"imagetoolbox/internal/s3"
)

// S3 凭证只从服务端环境变量读取（ITB_S3_*），secret 永不出现在响应中。

type S3UploadRequest struct {
	Key           string `json:"key"`
	Prefix        string `json:"prefix"`
	ContentType   string `json:"contentType"`
	SkipExisting  bool   `json:"skipExisting"`
	SkipUnchanged bool   `json:"skipUnchanged"`
}

// requireS3Client 取 Server 生命周期内复用的 S3 客户端；
// 未配置或配置不完整时已写回 503 响应。
func (s *Server) requireS3Client(c *gin.Context) (*s3.Client, bool) {
	client, err := s.sharedS3Client(c.Request.Context())
	if err != nil {
		fail(c, http.StatusServiceUnavailable, "S3 未配置或配置不完整: %v", err)
		return nil, false
	}
	return client, true
}

// handleS3Status 返回 S3 配置状态；绝不返回 SecretAccessKey。
func (s *Server) handleS3Status(c *gin.Context) {
	cfg := &s3.Config{}
	cfg.LoadFromEnv()
	c.JSON(http.StatusOK, gin.H{
		"configured": cfg.Validate() == nil,
		"endpoint":   cfg.Endpoint,
		"region":     cfg.Region,
		"bucket":     cfg.Bucket,
	})
}

func (s *Server) handleS3List(c *gin.Context) {
	client, ok := s.requireS3Client(c)
	if !ok {
		return
	}

	opts := &s3.ListOptions{Prefix: c.Query("prefix")}
	if raw := c.Query("max-keys"); raw != "" {
		if maxKeys, err := strconv.ParseInt(raw, 10, 32); err == nil && maxKeys > 0 {
			opts.MaxKeys = int32(maxKeys)
		}
	}

	objects, err := s3.List(c.Request.Context(), client, opts)
	if err != nil {
		fail(c, http.StatusBadGateway, "列取对象失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"objects": objects})
}

func (s *Server) handleS3Upload(c *gin.Context) {
	client, ok := s.requireS3Client(c)
	if !ok {
		return
	}

	dir, cleanup, err := newRequestDir("itb-s3-upload")
	if err != nil {
		fail(c, http.StatusInternalServerError, "%v", err)
		return
	}
	defer cleanup()

	inputPath, ok := requireFormFile(c, dir, "file")
	if !ok {
		return
	}
	opts, ok := bindOptions[S3UploadRequest](c)
	if !ok {
		return
	}

	key := opts.Key
	if key == "" {
		key = filepath.Base(inputPath)
		if prefix := filepath.ToSlash(filepath.Clean(opts.Prefix)); prefix != "" && prefix != "." {
			key = fmt.Sprintf("%s/%s", prefix, key)
		}
	}

	uploadOpts := &s3.UploadOptions{
		ContentType:   opts.ContentType,
		SkipExisting:  opts.SkipExisting,
		SkipUnchanged: opts.SkipUnchanged,
	}
	result, err := s3.Upload(c.Request.Context(), client, inputPath, key, uploadOpts)
	if err != nil {
		fail(c, http.StatusBadGateway, "上传失败: %v", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"key": key, "skipped": result.Skipped, "reason": result.Reason})
}

// handleS3Stat 返回单个对象的完整元数据（HeadObject）。
// 仅供前端"查看详情"时调用；列表页必须继续使用 ListObjectsV2 的结果，
// 避免对每个对象额外发一次 HEAD 造成 N+1 请求。
func (s *Server) handleS3Stat(c *gin.Context) {
	client, ok := s.requireS3Client(c)
	if !ok {
		return
	}

	key := c.Query("key")
	if key == "" {
		fail(c, http.StatusBadRequest, "缺少参数: key")
		return
	}

	info, err := s3.Stat(c.Request.Context(), client, key)
	if err != nil {
		if errors.Is(err, s3.ErrObjectNotFound) {
			fail(c, http.StatusNotFound, "对象不存在: %s", key)
			return
		}
		fail(c, http.StatusBadGateway, "查询对象元数据失败: %v", err)
		return
	}

	c.JSON(http.StatusOK, info)
}

// handleS3Download 把对象内容从 S3 直接流式转发给浏览器：
// GetObject → io.Copy(c.Writer)，不落盘临时文件、不整文件读入内存。
func (s *Server) handleS3Download(c *gin.Context) {
	client, ok := s.requireS3Client(c)
	if !ok {
		return
	}

	key := c.Query("key")
	if key == "" {
		fail(c, http.StatusBadRequest, "缺少参数: key")
		return
	}

	out, err := s3.Get(c.Request.Context(), client, key)
	if err != nil {
		if errors.Is(err, s3.ErrObjectNotFound) {
			fail(c, http.StatusNotFound, "对象不存在: %s", key)
			return
		}
		fail(c, http.StatusBadGateway, "下载失败: %v", err)
		return
	}
	defer out.Body.Close()

	name := filepath.Base(key)

	contentType := aws.ToString(out.ContentType)
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(name))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	c.Header("Content-Disposition", contentDisposition(name))
	c.Header("Content-Type", contentType)
	if out.ContentLength != nil {
		c.Header("Content-Length", strconv.FormatInt(*out.ContentLength, 10))
	}
	c.Status(http.StatusOK)

	if _, err := io.Copy(c.Writer, out.Body); err != nil {
		// 响应头已写出，无法再改状态码；记录错误并中断传输，
		// 浏览器侧表现为下载中断（客户端取消时也走这里，属正常路径）。
		_ = c.Error(fmt.Errorf("streaming object body: %w", err))
		c.Abort()
	}
}

func (s *Server) handleS3Delete(c *gin.Context) {
	client, ok := s.requireS3Client(c)
	if !ok {
		return
	}

	key := c.Query("key")
	if key == "" {
		fail(c, http.StatusBadRequest, "缺少参数: key")
		return
	}

	if err := s3.Delete(c.Request.Context(), client, key, nil); err != nil {
		fail(c, http.StatusBadGateway, "删除失败: %v", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": key})
}
