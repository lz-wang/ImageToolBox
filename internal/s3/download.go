package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// DownloadOptions 下载选项
type DownloadOptions struct {
	OutputFile string
}

// Get 获取对象内容流（GetObject），调用方负责关闭返回的 Body。
// 供 Web 层把 S3 响应直接流式转发给浏览器，
// 避免"落盘临时文件 + 整文件读入内存"的开销。
func Get(ctx context.Context, client *Client, key string) (*s3.GetObjectOutput, error) {
	if key == "" {
		return nil, ErrMissingKey
	}

	out, err := client.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(client.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, WrapError(err)
	}
	return out, nil
}

// Download 从存储桶下载文件到本地路径
func Download(ctx context.Context, client *Client, key string, outputPath string, opts *DownloadOptions) error {
	if key == "" {
		return ErrMissingKey
	}

	// 创建输出目录
	outputDir := filepath.Dir(outputPath)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// 先取对象流，失败时不留下空文件
	out, err := Get(ctx, client, key)
	if err != nil {
		return err
	}
	defer out.Body.Close()

	// 创建输出文件
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// 获取文件大小
	var size int64
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	if size > 5*1024*1024 {
		fmt.Printf("Downloading (%.2f MB)...\n", float64(size)/(1024*1024))
	}

	// 写入文件
	if _, err := io.Copy(file, out.Body); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	fmt.Printf("Download completed: %s -> %s (%d bytes)\n", key, outputPath, size)
	return nil
}
