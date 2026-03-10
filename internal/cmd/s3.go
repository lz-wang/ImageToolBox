package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"imagetoolbox/internal/s3"
)

// s3 公共参数
var (
	s3Endpoint       string
	s3AccessKey      string
	s3SecretKey      string
	s3Region         string
	s3Bucket         string
	s3ForcePathStyle bool
)

// s3 upload 参数
var (
	s3UploadInput       string
	s3UploadKey         string
	s3UploadContentType string
)

// s3 download 参数
var (
	s3DownloadKey    string
	s3DownloadOutput string
)

// s3 delete 参数
var (
	s3DeleteKey   string
	s3DeleteForce bool
)

// s3 list 参数
var (
	s3ListPrefix  string
	s3ListMaxKeys int
	s3ListFormat  string
)

// S3 命令
var s3Cmd = &cobra.Command{
	Use:   "s3",
	Short: "S3 兼容存储操作",
	Long: `S3 兼容存储操作，支持 AWS S3、MinIO、阿里云 OSS、腾讯云 COS 等。

环境变量支持:
  AWS_ACCESS_KEY_ID       Access Key
  AWS_SECRET_ACCESS_KEY   Secret Key
  AWS_REGION              区域
  S3_ENDPOINT             自定义端点`,
}

var s3UploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "上传文件到存储桶",
	Long:  `上传本地文件到 S3 兼容存储桶。`,
	Example: `  # 上传文件
  imagetoolbox s3 upload -i photo.jpg -b my-bucket -e http://localhost:9000

  # 指定对象键名
  imagetoolbox s3 upload -i photo.jpg -b my-bucket -k images/photo.jpg

  # 指定 Content-Type
  imagetoolbox s3 upload -i data.json -b my-bucket --content-type application/json`,
	RunE: runS3Upload,
}

var s3DownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "从存储桶下载文件",
	Long:  `从 S3 兼容存储桶下载文件到本地。`,
	Example: `  # 下载文件
  imagetoolbox s3 download -b my-bucket -k photo.jpg -o ./photo.jpg

  # 使用默认文件名
  imagetoolbox s3 download -b my-bucket -k images/photo.jpg`,
	RunE: runS3Download,
}

var s3DeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "从存储桶删除对象",
	Long:  `从 S3 兼容存储桶删除指定对象。`,
	Example: `  # 删除对象（需要确认）
  imagetoolbox s3 delete -b my-bucket -k photo.jpg

  # 强制删除（不需要确认）
  imagetoolbox s3 delete -b my-bucket -k photo.jpg -f`,
	RunE: runS3Delete,
}

var s3ListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出存储桶中的对象",
	Long:  `列出 S3 兼容存储桶中的对象。`,
	Example: `  # 列出所有对象
  imagetoolbox s3 list -b my-bucket

  # 按前缀过滤
  imagetoolbox s3 list -b my-bucket -p images/

  # JSON 格式输出
  imagetoolbox s3 list -b my-bucket --format json`,
	RunE: runS3List,
}

func init() {
	rootCmd.AddCommand(s3Cmd)

	// 添加子命令
	s3Cmd.AddCommand(s3UploadCmd, s3DownloadCmd, s3DeleteCmd, s3ListCmd)

	// S3 公共参数使用 PersistentFlags（子命令自动继承）
	s3Cmd.PersistentFlags().StringVarP(&s3Endpoint, "endpoint", "e", "", "S3 端点 URL")
	s3Cmd.PersistentFlags().StringVarP(&s3AccessKey, "access-key", "a", "", "Access Key ID（默认从环境变量读取）")
	s3Cmd.PersistentFlags().StringVarP(&s3SecretKey, "secret-key", "s", "", "Secret Access Key（默认从环境变量读取）")
	s3Cmd.PersistentFlags().StringVarP(&s3Region, "region", "r", "us-east-1", "区域")
	s3Cmd.PersistentFlags().StringVarP(&s3Bucket, "bucket", "b", "", "存储桶名称")
	s3Cmd.PersistentFlags().BoolVar(&s3ForcePathStyle, "force-path-style", false, "强制路径样式 URL（MinIO 需要）")
	s3Cmd.MarkPersistentFlagRequired("bucket")

	// S3 upload 参数
	s3UploadCmd.Flags().StringVarP(&s3UploadInput, "input", "i", "", "本地文件路径")
	s3UploadCmd.Flags().StringVarP(&s3UploadKey, "key", "k", "", "对象键名（默认使用文件名）")
	s3UploadCmd.Flags().StringVar(&s3UploadContentType, "content-type", "", "内容类型（自动检测）")
	s3UploadCmd.MarkFlagRequired("input")

	// S3 download 参数
	s3DownloadCmd.Flags().StringVarP(&s3DownloadKey, "key", "k", "", "对象键名")
	s3DownloadCmd.Flags().StringVarP(&s3DownloadOutput, "output", "o", "", "本地输出路径（默认使用对象键名）")
	s3DownloadCmd.MarkFlagRequired("key")

	// S3 delete 参数
	s3DeleteCmd.Flags().StringVarP(&s3DeleteKey, "key", "k", "", "对象键名")
	s3DeleteCmd.Flags().BoolVarP(&s3DeleteForce, "force", "f", false, "强制删除，不确认")
	s3DeleteCmd.MarkFlagRequired("key")

	// S3 list 参数
	s3ListCmd.Flags().StringVarP(&s3ListPrefix, "prefix", "p", "", "对象键前缀")
	s3ListCmd.Flags().IntVar(&s3ListMaxKeys, "max-keys", 1000, "最大返回数量")
	s3ListCmd.Flags().StringVar(&s3ListFormat, "format", "table", "输出格式: table/json/plain")
}

func newS3Client(ctx context.Context) (*s3.Client, error) {
	cfg := &s3.Config{
		Endpoint:        s3Endpoint,
		AccessKeyID:     s3AccessKey,
		SecretAccessKey: s3SecretKey,
		Region:          s3Region,
		Bucket:          s3Bucket,
		ForcePathStyle:  s3ForcePathStyle,
	}
	cfg.LoadFromEnv()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return s3.NewClient(ctx, cfg)
}

func runS3Upload(cmd *cobra.Command, args []string) error {
	if s3UploadInput == "" {
		return fmt.Errorf("必须指定输入文件路径 (-i)")
	}

	client, err := newS3Client(cmd.Context())
	if err != nil {
		return err
	}

	// 默认使用文件名作为对象键
	key := s3UploadKey
	if key == "" {
		key = filepath.Base(s3UploadInput)
	}

	opts := &s3.UploadOptions{
		ContentType: s3UploadContentType,
	}

	return s3.Upload(cmd.Context(), client, s3UploadInput, key, opts)
}

func runS3Download(cmd *cobra.Command, args []string) error {
	if s3DownloadKey == "" {
		return fmt.Errorf("必须指定对象键名 (-k)")
	}

	client, err := newS3Client(cmd.Context())
	if err != nil {
		return err
	}

	// 默认使用对象键名作为本地文件名
	output := s3DownloadOutput
	if output == "" {
		output = filepath.Base(s3DownloadKey)
	}

	return s3.Download(cmd.Context(), client, s3DownloadKey, output, nil)
}

func runS3Delete(cmd *cobra.Command, args []string) error {
	if s3DeleteKey == "" {
		return fmt.Errorf("必须指定对象键名 (-k)")
	}

	// 确认删除
	if !s3DeleteForce {
		fmt.Printf("确定要删除 s3://%s/%s 吗？(y/N): ", s3Bucket, s3DeleteKey)
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" {
			fmt.Println("已取消")
			return nil
		}
	}

	client, err := newS3Client(cmd.Context())
	if err != nil {
		return err
	}

	return s3.Delete(cmd.Context(), client, s3DeleteKey, nil)
}

func runS3List(cmd *cobra.Command, args []string) error {
	client, err := newS3Client(cmd.Context())
	if err != nil {
		return err
	}

	opts := &s3.ListOptions{
		Prefix:  s3ListPrefix,
		MaxKeys: int32(s3ListMaxKeys),
	}

	objects, err := s3.List(cmd.Context(), client, opts)
	if err != nil {
		return err
	}

	fmt.Print(s3.FormatOutput(objects, s3ListFormat))
	return nil
}
