package compress

import (
	"context"
	"fmt"
	"os"
)

const DefaultQuality = 80

// FileOptions 文件级压缩选项
type FileOptions struct {
	Quality int // 压缩质量 1-100
}

// Normalize applies the domain defaults shared by every adapter.
func (o *FileOptions) Normalize() {
	if o.Quality == 0 {
		o.Quality = DefaultQuality
	}
}

// Validate verifies file-level compression options.
func (o FileOptions) Validate() error {
	if o.Quality < 1 || o.Quality > 100 {
		return fmt.Errorf("压缩质量必须在 1-100 范围内: %d", o.Quality)
	}
	return nil
}

// Result 文件压缩结果
type Result struct {
	Format     string // 检测到的输入格式（png/jpeg）
	InputSize  int64  // 输入文件字节数
	OutputSize int64  // 输出文件字节数
}

// CompressFile 检测输入图片格式（PNG/JPEG），执行对应的压缩管道并写入 outputPath。
// 供 CLI 与 Web API 共用；原地覆盖（输出回输入路径）由调用方自行处理。
func CompressFile(ctx context.Context, inputPath, outputPath string, opts FileOptions) (Result, error) {
	ctx = commandContext(ctx)
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	opts.Normalize()
	if err := opts.Validate(); err != nil {
		return Result{}, err
	}
	if outputPath == "" {
		return Result{}, fmt.Errorf("必须指定输出文件路径")
	}

	stat, err := os.Stat(inputPath)
	if err != nil {
		return Result{}, fmt.Errorf("无法读取输入文件信息: %w", err)
	}

	f, err := os.Open(inputPath)
	if err != nil {
		return Result{}, fmt.Errorf("无法打开输入文件: %w", err)
	}
	format, err := DetectFormat(f)
	f.Close()
	if err != nil {
		return Result{}, fmt.Errorf("无法检测图片格式: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	switch format {
	case "png":
		err = compressPNGTo(ctx, inputPath, outputPath, opts.Quality)
	case "jpeg":
		err = compressJPEGTo(ctx, inputPath, outputPath, opts.Quality)
	default:
		err = fmt.Errorf("不支持的图片格式: %s", format)
	}
	if err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	outStat, err := os.Stat(outputPath)
	if err != nil {
		return Result{}, fmt.Errorf("无法读取输出文件信息: %w", err)
	}

	return Result{
		Format:     format,
		InputSize:  stat.Size(),
		OutputSize: outStat.Size(),
	}, nil
}

func compressPNGTo(ctx context.Context, inputPath, outputPath string, quality int) error {
	input, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("无法打开输入文件: %w", err)
	}
	defer input.Close()

	output, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("无法创建输出文件: %w", err)
	}
	defer output.Close()

	return CompressPNG(PNGOptions{
		Context:     ctx,
		Quality:     quality,
		OxiPngLevel: 4,
		Input:       input,
		Output:      output,
	})
}

func compressJPEGTo(ctx context.Context, inputPath, outputPath string, quality int) error {
	output, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("无法创建输出文件: %w", err)
	}
	defer output.Close()

	return CompressJPEG(JPEGOptions{
		Context:     ctx,
		Quality:     quality,
		Progressive: true,
		Optimize:    true,
		InputPath:   inputPath,
		Output:      output,
	})
}
