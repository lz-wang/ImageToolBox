package httpapi

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"imagetoolbox/internal/compress"
	"imagetoolbox/internal/convert"
	"imagetoolbox/internal/crop"
	"imagetoolbox/internal/resize"
	"imagetoolbox/internal/watermark"
)

// 请求结构为 Web 专用，与 CLI 命令参数状态完全隔离，
// 保证并发 HTTP 请求之间互不污染。

type CompressRequest struct {
	Quality int `json:"quality"`
}

type ResizeRequest struct {
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Percent string `json:"percent"`
	Mode    string `json:"mode"`
	Anchor  string `json:"anchor"`
	Filter  string `json:"filter"`
}

type CropRequest struct {
	Anchor string `json:"anchor"`
	Width  string `json:"width"`
	Height string `json:"height"`
}

type ConvertRequest struct {
	To         string `json:"to"`
	Quality    int    `json:"quality"`
	Lossless   bool   `json:"lossless"`
	Background string `json:"background"`
}

type WatermarkRequest struct {
	Type     string   `json:"type"` // text（默认）| image
	Text     string   `json:"text"`
	Mode     string   `json:"mode"` // position（默认）| repeat
	Position string   `json:"position"`
	Opacity  *float64 `json:"opacity"`
	Color    *string  `json:"color"`
	FontSize *int     `json:"fontSize"`
	Space    *int     `json:"space"`
	Angle    *int     `json:"angle"`
	Margin   *float64 `json:"margin"`
	Scale    *float64 `json:"scale"`
}

func handleCompress(c *gin.Context) {
	dir, cleanup, err := newRequestDir("itb-compress")
	if err != nil {
		fail(c, http.StatusInternalServerError, "%v", err)
		return
	}
	defer cleanup()

	inputPath, ok := requireFormFile(c, dir, "file")
	if !ok {
		return
	}

	opts, ok := bindOptions[CompressRequest](c)
	if !ok {
		return
	}
	quality := opts.Quality
	if quality == 0 {
		quality = 80
	}

	outputPath := filepath.Join(dir, "output"+filepath.Ext(inputPath))
	result, err := compress.CompressFile(inputPath, outputPath, compress.FileOptions{Quality: quality})
	if err != nil {
		fail(c, http.StatusBadRequest, "压缩失败: %v", err)
		return
	}

	serveImageFile(c, outputPath, result.InputSize, filepath.Base(inputPath))
}

func handleResize(c *gin.Context) {
	dir, cleanup, err := newRequestDir("itb-resize")
	if err != nil {
		fail(c, http.StatusInternalServerError, "%v", err)
		return
	}
	defer cleanup()

	inputPath, ok := requireFormFile(c, dir, "file")
	if !ok {
		return
	}

	opts, ok := bindOptions[ResizeRequest](c)
	if !ok {
		return
	}

	outName := outputFileName(inputPath, "_resized", "")
	outputPath := filepath.Join(dir, outName)
	if err := resize.ResizeFile(inputPath, outputPath, resize.Options{
		Width:   opts.Width,
		Height:  opts.Height,
		Percent: opts.Percent,
		Mode:    resize.Mode(opts.Mode),
		Anchor:  opts.Anchor,
		Filter:  opts.Filter,
	}); err != nil {
		fail(c, http.StatusBadRequest, "调整尺寸失败: %v", err)
		return
	}

	serveImageFile(c, outputPath, fileSize(inputPath), outName)
}

func handleCrop(c *gin.Context) {
	dir, cleanup, err := newRequestDir("itb-crop")
	if err != nil {
		fail(c, http.StatusInternalServerError, "%v", err)
		return
	}
	defer cleanup()

	inputPath, ok := requireFormFile(c, dir, "file")
	if !ok {
		return
	}

	opts, ok := bindOptions[CropRequest](c)
	if !ok {
		return
	}

	outName := outputFileName(inputPath, "_cropped", "")
	outputPath := filepath.Join(dir, outName)
	if _, err := crop.CropFile(inputPath, outputPath, crop.Options{
		Anchor: crop.Anchor(opts.Anchor),
		Width:  opts.Width,
		Height: opts.Height,
	}); err != nil {
		fail(c, http.StatusBadRequest, "裁剪失败: %v", err)
		return
	}

	serveImageFile(c, outputPath, fileSize(inputPath), outName)
}

func handleConvert(c *gin.Context) {
	dir, cleanup, err := newRequestDir("itb-convert")
	if err != nil {
		fail(c, http.StatusInternalServerError, "%v", err)
		return
	}
	defer cleanup()

	inputPath, ok := requireFormFile(c, dir, "file")
	if !ok {
		return
	}

	opts, ok := bindOptions[ConvertRequest](c)
	if !ok {
		return
	}
	if opts.To == "" {
		fail(c, http.StatusBadRequest, "缺少目标格式 (to)")
		return
	}
	quality := opts.Quality
	if quality == 0 {
		quality = 80
	}

	// DefaultOutputPath 复用 CLI 的命名约定（base_converted.<ext>），输出落在临时目录内
	outputPath, err := convert.DefaultOutputPath(inputPath, opts.To)
	if err != nil {
		fail(c, http.StatusBadRequest, "转换失败: %v", err)
		return
	}
	if err := convert.ConvertFile(inputPath, outputPath, convert.Options{
		To:         opts.To,
		Quality:    quality,
		Lossless:   opts.Lossless,
		Background: opts.Background,
	}); err != nil {
		fail(c, http.StatusBadRequest, "转换失败: %v", err)
		return
	}

	serveImageFile(c, outputPath, fileSize(inputPath), filepath.Base(outputPath))
}

func handleWatermark(c *gin.Context) {
	dir, cleanup, err := newRequestDir("itb-watermark")
	if err != nil {
		fail(c, http.StatusInternalServerError, "%v", err)
		return
	}
	defer cleanup()

	inputPath, ok := requireFormFile(c, dir, "file")
	if !ok {
		return
	}

	wmImagePath, ok := optionalFormFile(c, dir, "watermark")
	if !ok {
		return
	}
	fontPath, ok := optionalFormFile(c, dir, "font")
	if !ok {
		return
	}

	opts, ok := bindOptions[WatermarkRequest](c)
	if !ok {
		return
	}

	outName := outputFileName(inputPath, "_watermarked", "")
	outputPath := filepath.Join(dir, outName)

	if err := processWatermark(inputPath, outputPath, opts, wmImagePath, fontPath); err != nil {
		fail(c, http.StatusBadRequest, "添加水印失败: %v", err)
		return
	}

	serveImageFile(c, outputPath, fileSize(inputPath), outName)
}

// processWatermark 按请求参数为单张图片添加水印。
func processWatermark(inputPath, outputPath string, opts WatermarkRequest, watermarkPath, fontPath string) error {
	mode := opts.Mode
	if mode == "" {
		mode = "position"
	}

	if strings.EqualFold(opts.Type, "image") || watermarkPath != "" {
		if mode != "position" {
			return fmt.Errorf("图片水印仅支持 position 模式")
		}
		_, err := watermark.AddImageWatermark(inputPath, outputPath, &watermark.ImageOptions{
			ImagePath:   watermarkPath,
			Opacity:     opts.Opacity,
			Position:    watermark.Position(opts.Position),
			ScaleRatio:  opts.Scale,
			MarginRatio: opts.Margin,
		})
		return err
	}

	switch mode {
	case "repeat":
		_, err := watermark.AddRepeatWatermark(inputPath, outputPath, opts.Text, &watermark.RepeatOptions{
			Color:    opts.Color,
			Space:    opts.Space,
			Angle:    opts.Angle,
			Opacity:  opts.Opacity,
			FontPath: fontPath,
			FontSize: opts.FontSize,
		})
		return err
	case "position":
		_, err := watermark.AddPositionWatermark(inputPath, outputPath, opts.Text, &watermark.PositionOptions{
			Opacity:     opts.Opacity,
			Position:    watermark.Position(opts.Position),
			FontPath:    fontPath,
			FontSize:    opts.FontSize,
			Color:       opts.Color,
			MarginRatio: opts.Margin,
		})
		return err
	default:
		return fmt.Errorf("不支持的水印模式: %s", mode)
	}
}

// outputFileName 生成 base+suffix+ext 的输出文件名；ext 为空时沿用输入扩展名。
func outputFileName(inputPath, suffix, ext string) string {
	if ext == "" {
		ext = filepath.Ext(inputPath)
	}
	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	return base + suffix + ext
}

func fileSize(path string) int64 {
	stat, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return stat.Size()
}
