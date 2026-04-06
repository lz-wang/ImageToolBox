package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"imagetoolbox/internal/lsky"
)

var (
	lskyURL   string
	lskyToken string
)

var (
	lskyUploadInput      string
	lskyUploadStrategyID int
	lskyUploadOutput     string
)

var lskyCmd = &cobra.Command{
	Use:   "lsky",
	Short: "LskyPro 图床操作",
	Long: `LskyPro 图床操作，当前支持上传图片。

环境变量支持:
  LSKY_URL    LskyPro 服务地址（支持根地址或 /api/v1）
  LSKY_TOKEN  API Token`,
}

var lskyUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "上传图片到 LskyPro",
	Long:  `上传本地图片到 LskyPro 图床。`,
	Example: `  # 使用环境变量上传
  imagetoolbox lsky upload -i photo.jpg

  # 显式指定地址和 Token
  imagetoolbox lsky upload -i photo.jpg --url https://img.example.com --token your-token

  # 指定存储策略
  imagetoolbox lsky upload -i photo.jpg --strategy 2

  # 以 JSON 输出完整响应
  imagetoolbox lsky upload -i photo.jpg --output json

  # 输出 URL
  imagetoolbox lsky upload -i photo.jpg --output url`,
	RunE: runLskyUpload,
}

func init() {
	rootCmd.AddCommand(lskyCmd)

	lskyCmd.PersistentFlags().StringVar(&lskyURL, "url", "", "LskyPro 服务地址（支持根地址或 /api/v1）")
	lskyCmd.PersistentFlags().StringVar(&lskyToken, "token", "", "LskyPro API Token（默认从环境变量读取）")

	lskyCmd.AddCommand(lskyUploadCmd)

	lskyUploadCmd.Flags().StringVarP(&lskyUploadInput, "input", "i", "", "本地图片路径")
	lskyUploadCmd.Flags().IntVarP(&lskyUploadStrategyID, "strategy", "s", 0, "存储策略 ID")
	lskyUploadCmd.Flags().StringVarP(&lskyUploadOutput, "output", "o", "markdown", "输出格式: markdown/url/json")
	lskyUploadCmd.MarkFlagRequired("input")
}

func runLskyUpload(cmd *cobra.Command, args []string) error {
	if lskyUploadInput == "" {
		return fmt.Errorf("必须指定输入文件路径 (-i)")
	}

	client, err := lsky.NewClient(&lsky.Config{
		BaseURL: lskyURL,
		Token:   lskyToken,
	})
	if err != nil {
		return err
	}

	result, err := lsky.Upload(cmd.Context(), client, lskyUploadInput, &lsky.UploadOptions{
		StrategyID: lskyUploadStrategyID,
	})
	if err != nil {
		return err
	}

	switch lskyUploadOutput {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	case "markdown":
		fmt.Fprintln(os.Stdout, result.Data.Links.Markdown)
		return nil
	case "url":
		fmt.Fprintln(os.Stdout, result.Data.Links.URL)
		return nil
	default:
		return fmt.Errorf("不支持的输出格式: %s（支持: markdown, url, json）", lskyUploadOutput)
	}
}
