package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"
	"imagetoolbox/internal/compress"
	"imagetoolbox/internal/imageio"
)

func newCompressCommand() *cli.Command {
	return &cli.Command{
		Name:      "compress",
		Usage:     "压缩 PNG/JPEG 图片",
		ArgsUsage: "<src> [dst]",
		Description: `自动检测输入图片的格式（PNG/JPEG），然后执行对应的压缩操作。

无需指定图片类型，程序会通过读取文件头自动判断。

默认保留输入文件，输出到原文件名后加 _compressed 的新文件；
需要覆盖原文件时显式指定 --in-place。

压缩管道:
  PNG:  pngquant → oxipng
  JPEG: djpeg → cjpeg（libjpeg-turbo）

示例:
	  itb compress photo.png
	  itb compress -q 90 photo.jpg compressed.jpg
	  itb compress --in-place photo.jpg`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "in-place",
				Usage: "覆盖输入文件",
			},
			&cli.IntFlag{
				Name:      "quality",
				Aliases:   []string{"q"},
				Value:     compress.DefaultQuality,
				Usage:     "压缩质量 (1-100)",
				Validator: intRangeValidator("quality", 1, 100),
			},
		},
		Action: runCompress,
	}
}

func runCompress(ctx context.Context, cmd *cli.Command) error {
	inputFile, outputPath, err := sourceDestinationArgs(cmd, false)
	if err != nil {
		return err
	}
	if cmd.Bool("in-place") && outputPath != "" {
		return fmt.Errorf("--in-place 不能与 <dst> 同时使用")
	}
	tmpPath := ""
	if cmd.Bool("in-place") {
		// 临时文件放在输入文件所在目录，保证 rename 不跨文件系统
		tmp, err := os.CreateTemp(filepath.Dir(inputFile), ".itb-compress-*"+filepath.Ext(inputFile))
		if err != nil {
			return fmt.Errorf("创建临时文件失败: %w", err)
		}
		tmpPath = tmp.Name()
		tmp.Close()
		outputPath = tmpPath
	} else if outputPath == "" {
		outputPath = imageio.SuffixedPath(inputFile, "_compressed")
	}

	result, err := compress.CompressFile(ctx, inputFile, outputPath, compress.FileOptions{Quality: cmd.Int("quality")})
	if err != nil {
		if tmpPath != "" {
			os.Remove(tmpPath)
		}
		return err
	}

	if tmpPath != "" {
		if err := os.Rename(tmpPath, inputFile); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("覆盖原文件失败: %w", err)
		}
		outputPath = inputFile
	}

	fmt.Printf("检测到格式: %s\n", result.Format)
	fmt.Printf("压缩完成: %s (%s → %s)\n", outputPath, formatSize(result.InputSize), formatSize(result.OutputSize))
	return nil
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}
