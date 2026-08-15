package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"imagetoolbox/internal/s3"
)

// S3 凭证只从服务端环境变量读取（ITB_S3_*），secret 永不出现在响应中。

type S3UploadRequest struct {
	Key         string `json:"key"`
	Prefix      string `json:"prefix"`
	ContentType string `json:"contentType"`
}

func s3Client(c *gin.Context) (*s3.Client, bool) {
	cfg := &s3.Config{}
	cfg.LoadFromEnv()
	client, err := s3.NewClient(c.Request.Context(), cfg)
	if err != nil {
		fail(c, http.StatusServiceUnavailable, "S3 未配置或配置不完整: %v", err)
		return nil, false
	}
	return client, true
}

// handleS3Status 返回 S3 配置状态；绝不返回 SecretAccessKey。
func handleS3Status(c *gin.Context) {
	cfg := &s3.Config{}
	cfg.LoadFromEnv()
	c.JSON(http.StatusOK, gin.H{
		"configured": cfg.Validate() == nil,
		"endpoint":   cfg.Endpoint,
		"region":     cfg.Region,
		"bucket":     cfg.Bucket,
	})
}

func handleS3List(c *gin.Context) {
	client, ok := s3Client(c)
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

func handleS3Upload(c *gin.Context) {
	client, ok := s3Client(c)
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

	uploadOpts := &s3.UploadOptions{ContentType: opts.ContentType}
	if err := s3.Upload(c.Request.Context(), client, inputPath, key, uploadOpts); err != nil {
		fail(c, http.StatusBadGateway, "上传失败: %v", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"key": key})
}

func handleS3Download(c *gin.Context) {
	client, ok := s3Client(c)
	if !ok {
		return
	}

	key := c.Query("key")
	if key == "" {
		fail(c, http.StatusBadRequest, "缺少参数: key")
		return
	}

	dir, cleanup, err := newRequestDir("itb-s3-download")
	if err != nil {
		fail(c, http.StatusInternalServerError, "%v", err)
		return
	}
	defer cleanup()

	outputPath := filepath.Join(dir, filepath.Base(key))
	if err := s3.Download(c.Request.Context(), client, key, outputPath, nil); err != nil {
		fail(c, http.StatusBadGateway, "下载失败: %v", err)
		return
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		fail(c, http.StatusInternalServerError, "读取下载内容失败: %v", err)
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(key)))
	c.Data(http.StatusOK, http.DetectContentType(data), data)
}

func handleS3Delete(c *gin.Context) {
	client, ok := s3Client(c)
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
