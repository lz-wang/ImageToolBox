package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"imagetoolbox/internal/crop"
)

var (
	cropInputFile  string
	cropOutputFile string
	cropAnchor     string
	cropWidth      string
	cropHeight     string
)

var cropCmd = &cobra.Command{
	Use:   "crop",
	Short: "按锚点和百分比裁剪图片",
	Long: `按指定锚点和百分比裁剪图片。

宽高仅支持百分比，例如 40%。`,
	Example: `  imagetoolbox crop -i a.jpg --anchor left --width 40%
  imagetoolbox crop -i a.jpg --anchor right --width 40%
  imagetoolbox crop -i a.jpg --anchor top-left --width 40% --height 40%
  imagetoolbox crop -i a.jpg --anchor center --width 40% --height 40%`,
	RunE: runCrop,
}

func init() {
	rootCmd.AddCommand(cropCmd)

	cropCmd.Flags().StringVarP(&cropInputFile, "input", "i", "", "输入图片文件路径")
	cropCmd.Flags().StringVarP(&cropOutputFile, "output", "o", "", "输出图片文件路径（默认在原文件名后加 _cropped）")
	cropCmd.Flags().StringVar(&cropAnchor, "anchor", "", "裁剪锚点: left/right/top/bottom/top-left/top-right/bottom-left/bottom-right/center")
	cropCmd.Flags().StringVar(&cropWidth, "width", "", "裁剪宽度百分比，例如 40%")
	cropCmd.Flags().StringVar(&cropHeight, "height", "", "裁剪高度百分比，例如 40%")

	cropCmd.MarkFlagRequired("input")
	cropCmd.MarkFlagRequired("anchor")
}

func runCrop(cmd *cobra.Command, args []string) error {
	if cropInputFile == "" {
		return fmt.Errorf("必须指定输入文件路径 (-i)")
	}
	if cropAnchor == "" {
		return fmt.Errorf("必须指定 --anchor")
	}

	outputPath := cropOutputFile
	if outputPath == "" {
		ext := filepath.Ext(cropInputFile)
		base := strings.TrimSuffix(filepath.Base(cropInputFile), ext)
		dir := filepath.Dir(cropInputFile)
		outputPath = filepath.Join(dir, base+"_cropped"+ext)
	}

	rect, err := crop.CropFile(cropInputFile, outputPath, crop.Options{
		Anchor: crop.Anchor(cropAnchor),
		Width:  cropWidth,
		Height: cropHeight,
	})
	if err != nil {
		return fmt.Errorf("裁剪失败: %w", err)
	}

	fmt.Printf("裁剪完成: %s (%dx%d)\n", outputPath, rect.Dx(), rect.Dy())
	return nil
}
