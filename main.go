package main

import (
	"embed"
	"fmt"
	"os"

	_ "image/jpeg"
	_ "image/png"

	"github.com/spf13/cobra"
	"imagetoolbox/internal/compress"
)

//go:embed bins/**
var binaries embed.FS

var (
	version = "dev"
)

func main() {
	compress.InitBinaries(binaries)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "imagetoolbox",
	Short: "高效的图片压缩工具",
	Long: `一个基于 pngquant、oxipng 和 libjpeg-turbo 的图片压缩 CLI 工具。

支持 PNG 和 JPEG 格式的高效压缩，所有依赖二进制已内嵌，无需外部依赖。`,
}

var compressCmd = &cobra.Command{
	Use:   "compress",
	Short: "自动检测并压缩图片",
	Long: `自动检测输入图片的格式（PNG/JPEG），然后执行对应的压缩操作。

无需指定图片类型，程序会通过读取文件头自动判断。`,
	Example: `  imagetoolbox compress -i photo.png
  imagetoolbox compress -i photo.jpg -o compressed.jpg -q 90`,
	RunE: runCompress,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("imagetoolbox version %s\n", version)
	},
}

var (
	inputFile  string
	outputFile string
	quality    int
)

func init() {
	rootCmd.AddCommand(compressCmd)
	rootCmd.AddCommand(versionCmd)

	compressCmd.Flags().StringVarP(&inputFile, "input", "i", "", "输入图片文件路径")
	compressCmd.Flags().StringVarP(&outputFile, "output", "o", "", "输出图片文件路径")
	compressCmd.Flags().IntVarP(&quality, "quality", "q", 80, "压缩质量 (1-100)")
}

func runCompress(cmd *cobra.Command, args []string) error {
	if inputFile == "" {
		return fmt.Errorf("必须指定输入文件路径 (-i)")
	}

	f, err := os.Open(inputFile)
	if err != nil {
		return fmt.Errorf("无法打开输入文件: %w", err)
	}

	format, err := compress.DetectFormat(f)
	f.Close()
	if err != nil {
		return fmt.Errorf("无法检测图片格式: %w", err)
	}

	fmt.Printf("检测到格式: %s\n", format)

	switch format {
	case "png":
		return compressPNGFile(inputFile, outputFile, quality)
	case "jpeg":
		return compressJPEGFile(inputFile, outputFile, quality)
	default:
		return fmt.Errorf("不支持的图片格式: %s", format)
	}
}

func compressPNGFile(inPath, outPath string, q int) error {
	input, err := os.Open(inPath)
	if err != nil {
		return err
	}
	defer input.Close()

	var output *os.File
	var outputPath string
	var tmpFile *os.File

	if outPath != "" {
		output, err = os.Create(outPath)
		if err != nil {
			return err
		}
		defer output.Close()
		outputPath = outPath
	} else {
		tmpFile, err = os.CreateTemp("", "imagetoolbox-*.png")
		if err != nil {
			return err
		}
		output = tmpFile
		outputPath = inPath
	}

	opts := compress.PNGOptions{
		Quality:     q,
		OxiPngLevel: 4,
		Input:       input,
		Output:      output,
	}

	if err := compress.CompressPNG(opts); err != nil {
		return err
	}

	if tmpFile != nil {
		tmpFile.Close()
		os.Rename(tmpFile.Name(), inPath)
	}

	fmt.Printf("压缩完成: %s\n", outputPath)
	return nil
}

func compressJPEGFile(inPath, outPath string, q int) error {
	var output *os.File
	var outputPath string
	var tmpFile *os.File
	var err error

	if outPath != "" {
		output, err = os.Create(outPath)
		if err != nil {
			return err
		}
		defer output.Close()
		outputPath = outPath
	} else {
		tmpFile, err = os.CreateTemp("", "imagetoolbox-*.jpg")
		if err != nil {
			return err
		}
		output = tmpFile
		outputPath = inPath
	}

	opts := compress.JPEGOptions{
		Quality:     q,
		Progressive: true,
		Optimize:    true,
		InputPath:   inPath,
		Output:      output,
	}

	if err := compress.CompressJPEG(opts); err != nil {
		return err
	}

	if tmpFile != nil {
		tmpFile.Close()
		os.Rename(tmpFile.Name(), inPath)
	}

	fmt.Printf("压缩完成: %s\n", outputPath)
	return nil
}
