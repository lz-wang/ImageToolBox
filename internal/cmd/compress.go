package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"imagetoolbox/internal/compress"
)

var (
	inputFile  string
	outputFile string
	quality    int
)

var compressCmd = &cobra.Command{
	Use:   "compress",
	Short: "自动检测并压缩图片",
	Long: `自动检测输入图片的格式（PNG/JPEG），然后执行对应的压缩操作。

无需指定图片类型，程序会通过读取文件头自动判断。`,
	Example: `  itb compress -i photo.png
  itb compress -i photo.jpg -o compressed.jpg -q 90`,
	RunE: runCompress,
}

func init() {
	rootCmd.AddCommand(compressCmd)

	compressCmd.Flags().StringVarP(&inputFile, "input", "i", "", "输入图片文件路径")
	compressCmd.Flags().StringVarP(&outputFile, "output", "o", "", "输出图片文件路径")
	compressCmd.Flags().IntVarP(&quality, "quality", "q", 80, "压缩质量 (1-100)")
}

func runCompress(cmd *cobra.Command, args []string) error {
	if inputFile == "" {
		return fmt.Errorf("必须指定输入文件路径 (-i)")
	}

	outputPath := outputFile
	tmpPath := ""
	if outputPath == "" {
		// 临时文件放在输入文件所在目录，保证 rename 不跨文件系统
		tmp, err := os.CreateTemp(filepath.Dir(inputFile), ".itb-compress-*"+filepath.Ext(inputFile))
		if err != nil {
			return fmt.Errorf("创建临时文件失败: %w", err)
		}
		tmpPath = tmp.Name()
		tmp.Close()
		outputPath = tmpPath
	}

	result, err := compress.CompressFile(inputFile, outputPath, compress.FileOptions{Quality: quality})
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
	}

	fmt.Printf("检测到格式: %s\n", result.Format)
	fmt.Printf("压缩完成: %s (%s → %s)\n", inputFile, formatSize(result.InputSize), formatSize(result.OutputSize))
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
