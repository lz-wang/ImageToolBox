package cmd

import (
	"io/fs"

	"github.com/spf13/cobra"
)

var (
	// Version 由 main 通过 Execute 传入
	version string
)

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:   "itb",
	Short: "图片处理工具箱",
	Long: `一个图片处理 CLI 工具箱，提供压缩、水印、图床上传等功能。

功能:
  - crop: 图片裁剪，基于锚点和百分比保留目标区域
  - resize: 图片缩放，支持 fit/fill/stretch
  - compress: 图片压缩（PNG/JPEG），基于 pngquant、oxipng 和 libjpeg-turbo
  - convert: 图片格式转换，支持 JPEG/PNG/WEBP
  - watermark: 添加文字水印，支持位置和重复平铺两种模式
  - inspect: 检查图片元数据和文件 hash
  - batch: 批量执行 resize/convert/watermark
  - lsky: 上传图片到 LskyPro 图床`,
	CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
}

// Execute 执行根命令。staticFS 为 WebUI 静态资源（web/dist），
// 仅 serve 命令使用。
func Execute(v string, staticFS fs.FS) error {
	version = v
	webFS = staticFS
	return rootCmd.Execute()
}
