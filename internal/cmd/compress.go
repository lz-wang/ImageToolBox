package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
	"imagetoolbox/internal/compress"
)

func newCompressCommand() *cli.Command {
	return &cli.Command{
		Name:  "compress",
		Usage: "压缩 PNG/JPEG 图片",
		Description: `自动检测输入图片的格式（PNG/JPEG），然后执行对应的压缩操作。

无需指定图片类型，程序会通过读取文件头自动判断。

默认保留输入文件，输出到原文件名后加 _compressed 的新文件；
需要覆盖原文件时显式指定 --in-place。

压缩管道:
  PNG:  pngquant → oxipng
  JPEG: djpeg → cjpeg（libjpeg-turbo）

示例:
  itb compress -i photo.png
  itb compress -i photo.jpg -o compressed.jpg -q 90
  itb compress -i photo.jpg --in-place`,
		// --output 与 --in-place 是互斥的输出目标
		MutuallyExclusiveFlags: []cli.MutuallyExclusiveFlags{
			{
				Flags: [][]cli.Flag{
					{
						&cli.StringFlag{
							Name:    "output",
							Aliases: []string{"o"},
							Usage:   "输出图片 `FILE`（默认在原文件名后加 _compressed）",
						},
					},
					{
						&cli.BoolFlag{
							Name:  "in-place",
							Usage: "覆盖输入文件",
						},
					},
				},
			},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "input",
				Aliases:  []string{"i"},
				Usage:    "输入图片 `FILE`",
				Required: true,
			},
			&cli.IntFlag{
				Name:      "quality",
				Aliases:   []string{"q"},
				Value:     80,
				Usage:     "压缩质量 (1-100)",
				Validator: intRangeValidator("quality", 1, 100),
			},
		},
		Action: runCompress,
	}
}

func runCompress(ctx context.Context, cmd *cli.Command) error {
	inputFile := cmd.String("input")

	outputPath := cmd.String("output")
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
		outputPath = defaultSuffixedPath(inputFile, "_compressed")
	}

	result, err := compress.CompressFile(inputFile, outputPath, compress.FileOptions{Quality: cmd.Int("quality")})
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

// defaultSuffixedPath 生成默认输出路径：同目录、原名加后缀、保留扩展名。
func defaultSuffixedPath(inputPath, suffix string) string {
	ext := filepath.Ext(inputPath)
	base := strings.TrimSuffix(filepath.Base(inputPath), ext)
	return filepath.Join(filepath.Dir(inputPath), base+suffix+ext)
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
