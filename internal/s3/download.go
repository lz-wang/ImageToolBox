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
	// Progress 接收大文件（>5MB）传输提示等进度信息，nil 表示不输出。
	// 进度信息不是执行结果，domain 不直接写 stdout，由 adapter 决定
	// 输出去向（CLI 传 os.Stderr，保持 stdout 只承载正式结果）。
	Progress io.Writer
}

// DownloadResult 下载结果
type DownloadResult struct {
	// Key 下载的对象键
	Key string `json:"key"`

	// OutputPath 本地输出文件路径
	OutputPath string `json:"output_path"`

	// Size 实际写入本地的字节数
	Size int64 `json:"size"`
}

// Get 获取对象内容流（GetObject），调用方负责关闭返回的 Body。
// Download 以及其他需要流式消费对象内容的调用方可复用此函数，
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

// Download 从存储桶下载文件到本地路径。
//
// 本函数不输出任何内容：结果通过 DownloadResult 返回（Size 是实际
// 写入本地的字节数），进度提示写入 opts.Progress，由 adapter
// （CLI/脚本）决定如何呈现。
func Download(ctx context.Context, client *Client, key string, outputPath string, opts *DownloadOptions) (*DownloadResult, error) {
	if key == "" {
		return nil, ErrMissingKey
	}

	// 创建输出目录
	outputDir := filepath.Dir(outputPath)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// 先取对象流，失败时不留下空文件
	out, err := Get(ctx, client, key)
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()

	// 创建输出文件
	file, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// 对象声明的大小只用于进度提示；实际结果以写入字节为准
	var declared int64
	if out.ContentLength != nil {
		declared = *out.ContentLength
	}
	var progress io.Writer
	if opts != nil {
		progress = opts.Progress
	}
	if declared > 5*1024*1024 && progress != nil {
		fmt.Fprintf(progress, "Downloading (%.2f MB)...\n", float64(declared)/(1024*1024))
	}

	// 写入文件
	written, err := io.Copy(file, out.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}
	return &DownloadResult{Key: key, OutputPath: outputPath, Size: written}, nil
}
