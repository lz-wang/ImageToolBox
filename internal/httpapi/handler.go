// Package httpapi provides the HTTP adapter for Image Tool Box.
package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
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
	Logger        *slog.Logger
	MaxUpload     int64
	MaxPixels     int64
	MaxDimension  int
	MaxConcurrent int
	Timeout       time.Duration
}

// New creates the versioned Image Tool Box HTTP API. It returns an error when
// the configuration is unusable instead of panicking on invalid limits.
func New(cfg Config) (http.Handler, error) {
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return newHandler(cfg), nil
}

func newHandler(cfg Config) http.Handler {
	sem := make(chan struct{}, cfg.MaxConcurrent)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", health)
	mux.HandleFunc("POST /api/v1/compress", protected(cfg, sem, imageHandler(cfg, "compress", compressImage)))
	mux.HandleFunc("POST /api/v1/resize", protected(cfg, sem, imageHandler(cfg, "resize", resizeImage)))
	mux.HandleFunc("POST /api/v1/crop", protected(cfg, sem, imageHandler(cfg, "crop", cropImage)))
	mux.HandleFunc("POST /api/v1/convert", protected(cfg, sem, imageHandler(cfg, "convert", convertImage)))
	mux.HandleFunc("POST /api/v1/watermark", protected(cfg, sem, imageHandler(cfg, "watermark", watermarkImage)))
	mux.HandleFunc("POST /api/v1/inspect", protected(cfg, sem, inspectHandler(cfg)))
	return accessLog(cfg, mux)
}

// Normalize fills zero-valued fields with service defaults.
func (c *Config) Normalize() {
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
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
}

// Validate reports whether the configuration holds usable service limits.
// It runs after Normalize, so zero values have already become defaults and
// only genuinely invalid (negative or otherwise unusable) values remain.
func (c Config) Validate() error {
	if c.MaxUpload <= 0 {
		return fmt.Errorf("max upload must be greater than 0")
	}
	if c.MaxPixels <= 0 {
		return fmt.Errorf("max pixels must be greater than 0")
	}
	if c.MaxDimension <= 0 {
		return fmt.Errorf("max dimension must be greater than 0")
	}
	if c.MaxConcurrent <= 0 {
		return fmt.Errorf("max concurrent must be greater than 0")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than 0")
	}
	return nil
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
type operation func(context.Context, form, string) (string, string, int64, error)

func imageHandler(cfg Config, operationName string, op operation) http.HandlerFunc {
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
		if err := r.Context().Err(); err != nil {
			writeError(w, http.StatusGatewayTimeout, err)
			return
		}
		path, name, inputSize, err := op(r.Context(), f, dir)
		if err != nil {
			writeError(w, operationErrorStatus(err), err)
			return
		}
		if err := r.Context().Err(); err != nil {
			writeError(w, http.StatusGatewayTimeout, err)
			return
		}
		serveFile(w, r, path, name, inputSize, operationName)
	}
}

func operationErrorStatus(err error) int {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout
	}
	if strings.Contains(err.Error(), "不支持") {
		return http.StatusUnsupportedMediaType
	}
	return http.StatusBadRequest
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
		return fmt.Errorf("%w: %dx%d exceeds configured limits", ErrImageTooLarge, info.Width, info.Height)
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

func compressImage(ctx context.Context, f form, dir string) (string, string, int64, error) {
	input, err := f.input("input", "quality")
	if err != nil {
		return "", "", 0, err
	}
	quality, err := f.int("quality")
	if err != nil {
		return "", "", 0, err
	}
	output := filepath.Join(dir, "output"+filepath.Ext(input))
	result, err := compress.CompressFile(ctx, input, output, compress.FileOptions{Quality: quality})
	return output, filepath.Base(input), result.InputSize, err
}
func resizeImage(_ context.Context, f form, dir string) (string, string, int64, error) {
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
func cropImage(_ context.Context, f form, dir string) (string, string, int64, error) {
	input, err := f.input("input", "anchor", "width", "height")
	if err != nil {
		return "", "", 0, err
	}
	name := imageio.SuffixedName(input, "_cropped", "")
	output := filepath.Join(dir, name)
	_, err = crop.CropFile(input, output, crop.Options{Anchor: crop.Anchor(f.values["anchor"]), Width: f.values["width"], Height: f.values["height"]})
	return output, name, fileSize(input), err
}
func convertImage(_ context.Context, f form, dir string) (string, string, int64, error) {
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
func watermarkImage(_ context.Context, f form, dir string) (string, string, int64, error) {
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
		if err := r.Context().Err(); err != nil {
			writeError(w, http.StatusGatewayTimeout, err)
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
			writeError(w, operationErrorStatus(err), err)
			return
		}
		if err := r.Context().Err(); err != nil {
			writeError(w, http.StatusGatewayTimeout, err)
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
	code, message := errorResponse(status, err)
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func errorResponse(status int, err error) (string, string) {
	switch status {
	case http.StatusBadRequest:
		if strings.Contains(err.Error(), "missing input") {
			return "missing_input", "input is required"
		}
		return "invalid_argument", err.Error()
	case http.StatusUnauthorized:
		return "unauthorized", "valid bearer token is required"
	case http.StatusRequestEntityTooLarge:
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return "payload_too_large", "request exceeds the configured upload limit"
		}
		return "image_too_large", "image exceeds configured limits"
	case http.StatusUnsupportedMediaType:
		return "unsupported_format", "unsupported image format"
	case http.StatusTooManyRequests:
		return "busy", "too many image operations in progress"
	case http.StatusGatewayTimeout:
		return "timeout", "image operation timed out"
	default:
		return "internal_error", "internal server error"
	}
}
func serveFile(w http.ResponseWriter, r *http.Request, path, name string, inputSize int64, operationName string) {
	f, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		buffer := make([]byte, 512)
		n, readErr := f.Read(buffer)
		if readErr != nil && readErr != io.EOF {
			writeError(w, http.StatusInternalServerError, readErr)
			return
		}
		contentType = http.DetectContentType(buffer[:n])
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", contentDisposition(name))
	w.Header().Set("X-ITB-Input-Size", strconv.FormatInt(inputSize, 10))
	w.Header().Set("X-ITB-Output-Size", strconv.FormatInt(stat.Size(), 10))
	w.Header().Set("X-ITB-Operation", operationName)
	http.ServeContent(w, r, name, stat.ModTime(), f)
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *loggingResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytes += int64(n)
	return n, err
}
func accessLog(cfg Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &loggingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		inputBytes := r.ContentLength
		if inputBytes < 0 {
			inputBytes = 0
		}
		cfg.Logger.Info("http_request", "method", r.Method, "path", r.URL.Path, "status", status, "duration_ms", time.Since(start).Milliseconds(), "input_bytes", inputBytes, "output_bytes", recorder.bytes, "remote_addr", r.RemoteAddr)
	})
}
func contentDisposition(name string) string {
	for _, r := range name {
		if r > 0x7f {
			return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", "download"+filepath.Ext(name), url.PathEscape(name))
		}
	}
	return fmt.Sprintf("attachment; filename=%q", name)
}
