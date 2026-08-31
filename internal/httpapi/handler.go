// Package httpapi provides the HTTP adapter for Image Tool Box.
package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"imagetoolbox/internal/compress"
	"imagetoolbox/internal/convert"
	"imagetoolbox/internal/crop"
	"imagetoolbox/internal/imageio"
	"imagetoolbox/internal/inspect"
	"imagetoolbox/internal/resize"
	"imagetoolbox/internal/watermark"
)

const (
	DefaultMaxUpload     int64 = 64 << 20
	DefaultMaxPixels     int64 = 50_000_000
	DefaultMaxDimension        = 16_384
	DefaultMaxConcurrent       = 2
	DefaultTimeout             = 2 * time.Minute
)

// Config configures the trusted remote HTTP API.
type Config struct {
	Token         string
	NoAuth        bool
	MaxUpload     int64
	MaxPixels     int64
	MaxDimension  int
	MaxConcurrent int
	Timeout       time.Duration
}

// New creates the versioned Image Tool Box HTTP API.
func New(cfg Config) http.Handler {
	cfg.normalize()
	sem := make(chan struct{}, cfg.MaxConcurrent)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", health)
	mux.HandleFunc("POST /api/v1/compress", protected(cfg, sem, imageHandler(cfg, compressImage)))
	mux.HandleFunc("POST /api/v1/resize", protected(cfg, sem, imageHandler(cfg, resizeImage)))
	mux.HandleFunc("POST /api/v1/crop", protected(cfg, sem, imageHandler(cfg, cropImage)))
	mux.HandleFunc("POST /api/v1/convert", protected(cfg, sem, imageHandler(cfg, convertImage)))
	mux.HandleFunc("POST /api/v1/watermark", protected(cfg, sem, imageHandler(cfg, watermarkImage)))
	mux.HandleFunc("POST /api/v1/inspect", protected(cfg, sem, inspectHandler(cfg)))
	return mux
}

func (c *Config) normalize() {
	if c.MaxUpload == 0 {
		c.MaxUpload = DefaultMaxUpload
	}
	if c.MaxPixels == 0 {
		c.MaxPixels = DefaultMaxPixels
	}
	if c.MaxDimension == 0 {
		c.MaxDimension = DefaultMaxDimension
	}
	if c.MaxConcurrent == 0 {
		c.MaxConcurrent = DefaultMaxConcurrent
	}
	if c.Timeout == 0 {
		c.Timeout = DefaultTimeout
	}
}

func protected(cfg Config, sem chan struct{}, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !cfg.NoAuth && !authorized(r, cfg.Token) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeError(w, http.StatusUnauthorized, fmt.Errorf("unauthorized"))
			return
		}
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		default:
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, fmt.Errorf("busy"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), cfg.Timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
func authorized(r *http.Request, token string) bool {
	if token == "" {
		return false
	}
	value := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return len(value) == len(token) && subtle.ConstantTimeCompare([]byte(value), []byte(token)) == 1
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type form struct {
	values map[string]string
	files  map[string]string
}
type operation func(form, string) (string, string, int64, error)

func imageHandler(cfg Config, op operation) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dir, err := os.MkdirTemp("", "itb-api-*")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		defer os.RemoveAll(dir)
		f, err := parseMultipart(w, r, dir, cfg)
		if err != nil {
			writeError(w, multipartErrorStatus(err), err)
			return
		}
		if err := admitImage(f.files["input"], cfg); err != nil {
			writeError(w, http.StatusRequestEntityTooLarge, err)
			return
		}
		path, name, inputSize, err := op(f, dir)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		serveFile(w, path, name, inputSize)
	}
}

func multipartErrorStatus(err error) int {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}

func parseMultipart(w http.ResponseWriter, r *http.Request, dir string, cfg Config) (form, error) {
	r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxUpload)
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return form{}, fmt.Errorf("Content-Type must be multipart/form-data")
	}
	reader, err := r.MultipartReader()
	if err != nil {
		return form{}, err
	}
	f := form{values: map[string]string{}, files: map[string]string{}}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return form{}, err
		}
		name := part.FormName()
		if name == "" {
			return form{}, fmt.Errorf("multipart field name is required")
		}
		if part.FileName() == "" {
			if _, ok := f.values[name]; ok {
				return form{}, fmt.Errorf("duplicate parameter: %s", name)
			}
			data, err := io.ReadAll(part)
			if err != nil {
				return form{}, err
			}
			f.values[name] = string(data)
			continue
		}
		if _, ok := f.files[name]; ok {
			return form{}, fmt.Errorf("duplicate file: %s", name)
		}
		filename := sanitizeFilename(part.FileName())
		if filename == "" {
			filename = name + ".bin"
		}
		path := filepath.Join(dir, filename)
		out, err := os.Create(path)
		if err != nil {
			return form{}, err
		}
		_, copyErr := io.Copy(out, part)
		closeErr := out.Close()
		if copyErr != nil {
			return form{}, copyErr
		}
		if closeErr != nil {
			return form{}, closeErr
		}
		f.files[name] = path
	}
	return f, nil
}

func admitImage(path string, cfg Config) error {
	if path == "" {
		return nil
	}
	info, err := imageio.Probe(path)
	if err != nil {
		return err
	}
	if info.Width > cfg.MaxDimension || info.Height > cfg.MaxDimension || int64(info.Width)*int64(info.Height) > cfg.MaxPixels {
		return fmt.Errorf("image exceeds configured limits")
	}
	return nil
}

func (f form) input(allowed ...string) (string, error) {
	if err := f.allowed(allowed...); err != nil {
		return "", err
	}
	path := f.files["input"]
	if path == "" {
		return "", fmt.Errorf("missing input")
	}
	return path, nil
}
func (f form) allowed(names ...string) error {
	ok := map[string]bool{}
	for _, name := range names {
		ok[name] = true
	}
	for name := range f.values {
		if !ok[name] {
			return fmt.Errorf("unknown parameter: %s", name)
		}
	}
	for name := range f.files {
		if !ok[name] {
			return fmt.Errorf("unknown parameter: %s", name)
		}
	}
	return nil
}
func (f form) int(name string) (int, error) {
	if f.values[name] == "" {
		return 0, nil
	}
	return strconv.Atoi(f.values[name])
}
func (f form) float(name string) (*float64, error) {
	if f.values[name] == "" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(f.values[name], 64)
	return &v, err
}
func (f form) bool(name string) (bool, error) {
	if f.values[name] == "" {
		return false, nil
	}
	return strconv.ParseBool(f.values[name])
}

func compressImage(f form, dir string) (string, string, int64, error) {
	input, err := f.input("input", "quality")
	if err != nil {
		return "", "", 0, err
	}
	quality, err := f.int("quality")
	if err != nil {
		return "", "", 0, err
	}
	output := filepath.Join(dir, "output"+filepath.Ext(input))
	result, err := compress.CompressFile(input, output, compress.FileOptions{Quality: quality})
	return output, filepath.Base(input), result.InputSize, err
}
func resizeImage(f form, dir string) (string, string, int64, error) {
	input, err := f.input("input", "width", "height", "percent", "mode", "anchor", "filter")
	if err != nil {
		return "", "", 0, err
	}
	width, err := f.int("width")
	if err != nil {
		return "", "", 0, err
	}
	height, err := f.int("height")
	if err != nil {
		return "", "", 0, err
	}
	name := imageio.SuffixedName(input, "_resized", "")
	output := filepath.Join(dir, name)
	err = resize.ResizeFile(input, output, resize.Options{Width: width, Height: height, Percent: f.values["percent"], Mode: resize.Mode(f.values["mode"]), Anchor: f.values["anchor"], Filter: f.values["filter"]})
	return output, name, fileSize(input), err
}
func cropImage(f form, dir string) (string, string, int64, error) {
	input, err := f.input("input", "anchor", "width", "height")
	if err != nil {
		return "", "", 0, err
	}
	name := imageio.SuffixedName(input, "_cropped", "")
	output := filepath.Join(dir, name)
	_, err = crop.CropFile(input, output, crop.Options{Anchor: crop.Anchor(f.values["anchor"]), Width: f.values["width"], Height: f.values["height"]})
	return output, name, fileSize(input), err
}
func convertImage(f form, dir string) (string, string, int64, error) {
	input, err := f.input("input", "to", "quality", "lossless", "background")
	if err != nil {
		return "", "", 0, err
	}
	quality, err := f.int("quality")
	if err != nil {
		return "", "", 0, err
	}
	lossless, err := f.bool("lossless")
	if err != nil {
		return "", "", 0, err
	}
	output, err := convert.DefaultOutputPath(input, f.values["to"])
	if err != nil {
		return "", "", 0, err
	}
	err = convert.ConvertFile(input, output, convert.Options{To: f.values["to"], Quality: quality, Lossless: lossless, Background: f.values["background"]})
	return output, filepath.Base(output), fileSize(input), err
}
func watermarkImage(f form, dir string) (string, string, int64, error) {
	input, err := f.input("input", "text", "image", "mode", "color", "space", "angle", "opacity", "font", "font-size", "position", "margin", "scale")
	if err != nil {
		return "", "", 0, err
	}
	space, err := f.int("space")
	if err != nil {
		return "", "", 0, err
	}
	angle, err := f.int("angle")
	if err != nil {
		return "", "", 0, err
	}
	fontSize, err := f.int("font-size")
	if err != nil {
		return "", "", 0, err
	}
	opacity, err := f.float("opacity")
	if err != nil {
		return "", "", 0, err
	}
	margin, err := f.float("margin")
	if err != nil {
		return "", "", 0, err
	}
	scale, err := f.float("scale")
	if err != nil {
		return "", "", 0, err
	}
	name := imageio.SuffixedName(input, "_watermarked", "")
	output := filepath.Join(dir, name)
	err = watermark.AddFile(input, output, watermark.Options{Text: f.values["text"], ImagePath: f.files["image"], Mode: watermark.Mode(f.values["mode"]), Position: watermark.Position(f.values["position"]), Color: f.values["color"], FontPath: f.files["font"], Opacity: opacity, FontSize: intPtr(f.values["font-size"], fontSize), Space: intPtr(f.values["space"], space), Angle: intPtr(f.values["angle"], angle), Margin: margin, Scale: scale})
	return output, name, fileSize(input), err
}
func intPtr(raw string, value int) *int {
	if raw == "" {
		return nil
	}
	return &value
}

func inspectHandler(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dir, err := os.MkdirTemp("", "itb-api-*")
		if err != nil {
			writeError(w, 500, err)
			return
		}
		defer os.RemoveAll(dir)
		f, err := parseMultipart(w, r, dir, cfg)
		if err != nil {
			writeError(w, multipartErrorStatus(err), err)
			return
		}
		input, err := f.input("input", "detail", "no-detail", "no-hash", "strict")
		if err != nil {
			writeError(w, 400, err)
			return
		}
		if err := admitImage(input, cfg); err != nil {
			writeError(w, http.StatusRequestEntityTooLarge, err)
			return
		}
		detail, err := f.bool("detail")
		if err != nil {
			writeError(w, 400, err)
			return
		}
		noDetail, err := f.bool("no-detail")
		if err != nil {
			writeError(w, 400, err)
			return
		}
		noHash, err := f.bool("no-hash")
		if err != nil {
			writeError(w, 400, err)
			return
		}
		strict, err := f.bool("strict")
		if err != nil {
			writeError(w, 400, err)
			return
		}
		if !detail {
			detail = true
		}
		if noDetail {
			detail = false
		}
		result, err := inspect.File(input, inspect.Options{Detail: detail, NoHash: noHash, Strict: strict})
		if err != nil {
			writeError(w, 400, err)
			return
		}
		result.File.Path = filepath.Base(result.File.Path)
		result.File.AbsPath = ""
		writeJSON(w, 200, result)
	}
}

func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	return strings.TrimLeft(filepath.Base(name), ".")
}
func fileSize(path string) int64 {
	stat, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return stat.Size()
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
func serveFile(w http.ResponseWriter, path, name string, inputSize int64) {
	data, err := os.ReadFile(path)
	if err != nil {
		writeError(w, 500, err)
		return
	}
	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", contentDisposition(name))
	w.Header().Set("X-ITB-Input-Size", strconv.FormatInt(inputSize, 10))
	w.Header().Set("X-ITB-Output-Size", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}
func contentDisposition(name string) string {
	for _, r := range name {
		if r > 0x7f {
			return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", "download"+filepath.Ext(name), url.PathEscape(name))
		}
	}
	return fmt.Sprintf("attachment; filename=%q", name)
}
