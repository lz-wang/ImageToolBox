package cmd

import (
	"github.com/spf13/cobra"
)

var (
	// Version 由 main 通过 Execute 传入
	version string
)

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:   "imagetoolbox",
	Short: "高效的图片压缩工具",
	Long: `一个基于 pngquant、oxipng 和 libjpeg-turbo 的图片压缩 CLI 工具。

支持 PNG 和 JPEG 格式的高效压缩，所有依赖二进制已内嵌，无需外部依赖。`,
}

// Execute 执行根命令
func Execute(v string) error {
	version = v
	return rootCmd.Execute()
}
