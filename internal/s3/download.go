package s3

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// DownloadOptions 下载选项
type DownloadOptions struct {
	// Verify 为 true 时读取对象 itb-sha256 用户 metadata，边下载边计算
	// SHA-256 并在结束时比对（io.MultiWriter，不二次读取本地文件）；
	// 对象缺少该 metadata 时直接报错。
	Verify bool

	// VerifySHA256 指定期望的十六进制 SHA-256（与 Verify 可同时使用），
	// 提供独立于对象 metadata 的 provider-neutral 完整性校验。
	VerifySHA256 string

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

	// SHA256 下载内容的 SHA-256；仅启用校验选项时计算
	SHA256 string `json:"sha256,omitempty"`
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
// 内容先写入输出目录下的临时文件，成功后 rename 到目标路径：
// 下载中断、写盘失败或校验不通过时删除临时文件，目标路径不会留下
// partial 文件。
//
// 校验在流式下载中完成（io.MultiWriter(file, sha256)），不进行
// 第二遍本地文件读取；哈希不一致返回 ErrChecksumMismatch。
//
// 本函数不输出任何内容：结果通过 DownloadResult 返回（Size 是实际
// 写入本地的字节数），进度提示写入 opts.Progress，由 adapter
// （CLI/脚本）决定如何呈现。
func Download(ctx context.Context, client *Client, key string, outputPath string, opts *DownloadOptions) (*DownloadResult, error) {
	if key == "" {
		return nil, ErrMissingKey
	}
	if opts != nil && opts.VerifySHA256 != "" {
		if _, err := hex.DecodeString(opts.VerifySHA256); err != nil {
			return nil, fmt.Errorf("--verify-sha256 must be a hex SHA-256 digest: %w", err)
		}
	}

	// 创建输出目录
	outputDir := filepath.Dir(outputPath)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// 先取对象流，失败时不创建任何文件
	out, err := Get(ctx, client, key)
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()

	// --verify 依赖对象自带的 itb-sha256 metadata；缺失时无法校验
	var metadataSHA256 string
	if opts != nil && opts.Verify {
		metadataSHA256 = strings.ToLower(out.Metadata[MetadataSHA256Key])
		if metadataSHA256 == "" {
			return nil, fmt.Errorf("verify: object %q has no %s metadata (uploaded by another tool?)", key, MetadataSHA256Key)
		}
	}

	// 写入同目录临时文件，成功后 rename：失败路径一律清理临时文件，
	// 目标路径不留下 partial 内容
	tmp, err := os.CreateTemp(outputDir, ".itb-download-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			tmp.Close()
			os.Remove(tmpPath)
		}
	}()

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

	// 边下载边计算哈希，不二次读取本地文件
	hasher := sha256.New()
	writers := []io.Writer{tmp}
	if opts != nil && (opts.Verify || opts.VerifySHA256 != "") {
		writers = append(writers, hasher)
	}

	written, err := io.Copy(io.MultiWriter(writers...), out.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// 校验失败时临时文件由 defer 清理，目标路径不留下 partial 内容
	var computed string
	if len(writers) > 1 {
		computed = hex.EncodeToString(hasher.Sum(nil))
		if metadataSHA256 != "" && computed != metadataSHA256 {
			return nil, fmt.Errorf("%w: object %q content is %s, metadata says %s", ErrChecksumMismatch, key, computed, metadataSHA256)
		}
		if opts != nil && opts.VerifySHA256 != "" && computed != strings.ToLower(opts.VerifySHA256) {
			return nil, fmt.Errorf("%w: object %q content is %s, expected %s", ErrChecksumMismatch, key, computed, strings.ToLower(opts.VerifySHA256))
		}
	}

	if err := tmp.Chmod(0o644); err != nil {
		return nil, fmt.Errorf("failed to set file mode: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("failed to close output file: %w", err)
	}
	if err := os.Rename(tmpPath, outputPath); err != nil {
		return nil, fmt.Errorf("failed to finalize output file: %w", err)
	}
	committed = true

	return &DownloadResult{Key: key, OutputPath: outputPath, Size: written, SHA256: computed}, nil
}
