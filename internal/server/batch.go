package server

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"imagetoolbox/internal/batch"
	"imagetoolbox/internal/convert"
	"imagetoolbox/internal/resize"
)

func handleBatchResize(c *gin.Context) {
	dir, cleanup, err := newRequestDir("itb-batch")
	if err != nil {
		fail(c, http.StatusInternalServerError, "%v", err)
		return
	}
	defer cleanup()

	opts, ok := bindOptions[ResizeRequest](c)
	if !ok {
		return
	}

	inputDir, outputDir, ok := prepareBatchDirs(c, dir)
	if !ok {
		return
	}
	if !saveBatchFiles(c, inputDir) {
		return
	}

	runBatch(c, dir, inputDir, outputDir, batchSuffixRelPath("_resized"), func(in, out string) error {
		return resize.ResizeFile(in, out, resize.Options{
			Width:   opts.Width,
			Height:  opts.Height,
			Percent: opts.Percent,
			Mode:    resize.Mode(opts.Mode),
			Anchor:  opts.Anchor,
			Filter:  opts.Filter,
		})
	})
}

func handleBatchConvert(c *gin.Context) {
	dir, cleanup, err := newRequestDir("itb-batch")
	if err != nil {
		fail(c, http.StatusInternalServerError, "%v", err)
		return
	}
	defer cleanup()

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

	inputDir, outputDir, ok := prepareBatchDirs(c, dir)
	if !ok {
		return
	}
	if !saveBatchFiles(c, inputDir) {
		return
	}

	targetExt := "." + formatExt(opts.To)
	outputRel := func(rel string) string {
		base := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
		return filepath.Join(filepath.Dir(rel), base+"_converted"+targetExt)
	}

	runBatch(c, dir, inputDir, outputDir, outputRel, func(in, out string) error {
		return convert.ConvertFile(in, out, convert.Options{
			To:         opts.To,
			Quality:    quality,
			Lossless:   opts.Lossless,
			Background: opts.Background,
		})
	})
}

func handleBatchWatermark(c *gin.Context) {
	dir, cleanup, err := newRequestDir("itb-batch")
	if err != nil {
		fail(c, http.StatusInternalServerError, "%v", err)
		return
	}
	defer cleanup()

	// 水印图/字体放在 input/ 之外，避免被批处理扫描到
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

	inputDir, outputDir, ok := prepareBatchDirs(c, dir)
	if !ok {
		return
	}
	if !saveBatchFiles(c, inputDir) {
		return
	}

	runBatch(c, dir, inputDir, outputDir, batchSuffixRelPath("_watermarked"), watermarkProcessor(opts, wmImagePath, fontPath))
}

// watermarkProcessor 按请求构造水印处理函数，复用单图的 processWatermark。
func watermarkProcessor(opts WatermarkRequest, wmImagePath, fontPath string) func(inputPath, outputPath string) error {
	return func(in, out string) error {
		return processWatermark(in, out, opts, wmImagePath, fontPath)
	}
}

// prepareBatchDirs 创建 <dir>/input 与 <dir>/output。
func prepareBatchDirs(c *gin.Context, dir string) (string, string, bool) {
	inputDir := filepath.Join(dir, "input")
	outputDir := filepath.Join(dir, "output")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		fail(c, http.StatusInternalServerError, "创建输入目录失败: %v", err)
		return "", "", false
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fail(c, http.StatusInternalServerError, "创建输出目录失败: %v", err)
		return "", "", false
	}
	return inputDir, outputDir, true
}

// saveBatchFiles 把 files[] 字段的全部文件保存到 inputDir。
func saveBatchFiles(c *gin.Context, inputDir string) bool {
	form, err := c.MultipartForm()
	if err != nil {
		fail(c, http.StatusBadRequest, "读取上传内容失败: %v", err)
		return false
	}
	files := form.File["files"]
	if len(files) == 0 {
		fail(c, http.StatusBadRequest, "缺少上传字段: files")
		return false
	}

	for _, fh := range files {
		name := sanitizeFilename(fh.Filename)
		if name == "" {
			continue
		}
		src, err := fh.Open()
		if err != nil {
			fail(c, http.StatusBadRequest, "读取上传文件失败: %v", err)
			return false
		}
		dst, err := os.Create(filepath.Join(inputDir, name))
		if err != nil {
			src.Close()
			fail(c, http.StatusInternalServerError, "保存上传文件失败: %v", err)
			return false
		}
		_, copyErr := io.Copy(dst, src)
		src.Close()
		dst.Close()
		if copyErr != nil {
			fail(c, http.StatusInternalServerError, "保存上传文件失败: %v", copyErr)
			return false
		}
	}
	return true
}

// runBatch 执行批处理并把 output/ 打包为 zip 返回；
// 部分失败时仍返回成功部分的 zip，统计通过 X-ITB-* 头带出。
func runBatch(c *gin.Context, dir, inputDir, outputDir string, outputRel batch.OutputPathFunc, processor batch.Processor) {
	result, err := batch.Process(
		batch.Options{InputDir: inputDir, OutputDir: outputDir},
		outputRel,
		processor,
	)

	if result.Success == 0 {
		msg := "没有可处理的图片文件"
		if err != nil {
			msg = err.Error()
		}
		if len(result.Errors) > 0 {
			msg = fmt.Sprintf("%s: %v", result.Errors[0].Path, result.Errors[0].Err)
		}
		fail(c, http.StatusBadRequest, "批处理失败: %s（成功 0 / 失败 %d）", msg, result.Failed)
		return
	}

	zipPath := filepath.Join(dir, "result.zip")
	if err := zipDir(outputDir, zipPath); err != nil {
		fail(c, http.StatusInternalServerError, "打包结果失败: %v", err)
		return
	}

	data, err := os.ReadFile(zipPath)
	if err != nil {
		fail(c, http.StatusInternalServerError, "读取打包结果失败: %v", err)
		return
	}

	c.Header("Content-Disposition", "attachment; filename=itb-batch-result.zip")
	c.Header("X-ITB-Success", strconv.Itoa(result.Success))
	c.Header("X-ITB-Skipped", strconv.Itoa(result.Skipped))
	c.Header("X-ITB-Failed", strconv.Itoa(result.Failed))
	c.Data(http.StatusOK, "application/zip", data)
}

// batchSuffixRelPath 输出文件名 = 原名 + 后缀，保持与 CLI 一致的命名约定。
func batchSuffixRelPath(suffix string) batch.OutputPathFunc {
	return func(rel string) string {
		base := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
		return filepath.Join(filepath.Dir(rel), base+suffix+filepath.Ext(rel))
	}
}

// formatExt 归一化目标格式扩展名，与 CLI 的 convert 命名保持一致。
func formatExt(to string) string {
	switch strings.ToLower(strings.TrimSpace(strings.TrimPrefix(to, "."))) {
	case "jpg", "jpeg":
		return "jpeg"
	default:
		return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(to, ".")))
	}
}

// zipDir 把目录内容打包为 zip（保留相对路径结构）。
func zipDir(srcDir, zipPath string) error {
	out, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer out.Close()

	w := zip.NewWriter(out)
	walkErr := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		entry, err := w.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(entry, src)
		return err
	})
	if closeErr := w.Close(); walkErr == nil {
		walkErr = closeErr
	}
	return walkErr
}
