package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"imagetoolbox/internal/lsky"
)

// Token 只从服务端环境变量读取（ITB_LSKY_*），永不出现在响应中。

type LskyUploadRequest struct {
	StrategyID int `json:"strategyId"`
}

func handleLskyUpload(c *gin.Context) {
	dir, cleanup, err := newRequestDir("itb-lsky-upload")
	if err != nil {
		fail(c, http.StatusInternalServerError, "%v", err)
		return
	}
	defer cleanup()

	inputPath, ok := requireFormFile(c, dir, "file")
	if !ok {
		return
	}
	opts, ok := bindOptions[LskyUploadRequest](c)
	if !ok {
		return
	}

	// NewClient 内部从环境变量补全 URL/Token
	client, err := lsky.NewClient(&lsky.Config{})
	if err != nil {
		fail(c, http.StatusServiceUnavailable, "Lsky 未配置: %v", err)
		return
	}

	result, err := lsky.Upload(c.Request.Context(), client, inputPath, &lsky.UploadOptions{
		StrategyID: opts.StrategyID,
	})
	if err != nil {
		fail(c, http.StatusBadGateway, "上传失败: %v", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":     result.Data.Name,
		"url":      result.Data.Links.URL,
		"markdown": result.Data.Links.Markdown,
	})
}
