package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// 嵌入 pngquant 二进制
//
//go:embed pngquant
var pngquantBinary []byte

// 写入临时文件
func ensureBinary() (string, error) {

	path := filepath.Join(os.TempDir(), "pngquant")

	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	err := os.WriteFile(path, pngquantBinary, 0755)
	if err != nil {
		return "", err
	}

	return path, nil
}

// 调用 pngquant 压缩 PNG
func compressPNG(input []byte, quality string) ([]byte, error) {

	bin, err := ensureBinary()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(
		bin,
		"--quality", quality,
		"--speed", "1",
		"--strip",
		"--output", "-",
		"-",
	)

	cmd.Stdin = bytes.NewReader(input)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// 生成输出文件名
func generateOutputPath(inputPath string) string {
	ext := filepath.Ext(inputPath)
	base := strings.TrimSuffix(inputPath, ext)
	return base + "_compressed" + ext
}

func main() {
	// 定义命令行参数
	inputPath := flag.String("i", "", "输入图片路径 (必需)")
	outputPath := flag.String("o", "", "输出图片路径 (可选，默认为 input_compressed.png)")
	quality := flag.String("q", "60-80", "压缩质量范围 (例如: 60-80)")
	help := flag.Bool("h", false, "显示帮助信息")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "PNG 图片压缩工具\n\n")
		fmt.Fprintf(os.Stderr, "用法:\n")
		fmt.Fprintf(os.Stderr, "  %s -i <输入图片> [-o <输出图片>] [-q <质量范围>]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "参数:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n示例:\n")
		fmt.Fprintf(os.Stderr, "  %s -i photo.png\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -i photo.png -o compressed.png\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -i photo.png -q 70-90\n", os.Args[0])
	}

	flag.Parse()

	// 显示帮助
	if *help {
		flag.Usage()
		os.Exit(0)
	}

	// 检查必需参数
	if *inputPath == "" {
		fmt.Fprintln(os.Stderr, "错误: 必须指定输入图片路径 (-i)")
		flag.Usage()
		os.Exit(1)
	}

	// 确定输出路径
	outPath := *outputPath
	if outPath == "" {
		outPath = generateOutputPath(*inputPath)
	}

	// 读取输入文件
	data, err := os.ReadFile(*inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 无法读取输入文件: %v\n", err)
		os.Exit(1)
	}

	// 压缩图片
	result, err := compressPNG(data, *quality)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 压缩失败: %v\n", err)
		os.Exit(1)
	}

	// 写入输出文件
	err = os.WriteFile(outPath, result, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 无法写入输出文件: %v\n", err)
		os.Exit(1)
	}

	// 计算压缩率
	originalSize := len(data)
	compressedSize := len(result)
	ratio := float64(compressedSize) / float64(originalSize) * 100

	fmt.Printf("压缩完成!\n")
	fmt.Printf("  原始大小: %d 字节\n", originalSize)
	fmt.Printf("  压缩后大小: %d 字节\n", compressedSize)
	fmt.Printf("  压缩率: %.1f%%\n", ratio)
	fmt.Printf("  输出文件: %s\n", outPath)
}
