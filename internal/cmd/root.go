package cmd

import (
	"context"
	"io/fs"
	"os"

	"github.com/urfave/cli/v3"
)

// New 构造 itb 根命令。version 由 main 注入，staticFS 为 WebUI
// 静态资源（web/dist），仅 serve 命令使用。
func New(version string, staticFS fs.FS) *cli.Command {
	return &cli.Command{
		Name:  "itb",
		Usage: "图片处理工具箱",
		Description: `一个图片处理 CLI 工具箱，提供压缩、水印、S3 存储操作等功能。

功能:
  - crop: 图片裁剪，基于锚点和百分比保留目标区域
  - resize: 图片缩放，支持 fit/fill/stretch
  - compress: 图片压缩（PNG/JPEG），基于 pngquant、oxipng 和 libjpeg-turbo
  - convert: 图片格式转换，支持 JPEG/PNG/WEBP
  - watermark: 添加文字水印，支持位置和重复平铺两种模式
  - inspect: 检查图片元数据和文件 hash
  - s3: S3 兼容存储操作（上传、下载、删除、列表）`,
		Commands: []*cli.Command{
			newCompressCommand(),
			newResizeCommand(),
			newCropCommand(),
			newConvertCommand(),
			newWatermarkCommand(),
			newInspectCommand(),
			newS3Command(),
			newServeCommand(staticFS),
			newVersionCommand(version),
		},
	}
}

// Execute 运行根命令。
func Execute(ctx context.Context, version string, staticFS fs.FS) error {
	return New(version, staticFS).Run(ctx, os.Args)
}
