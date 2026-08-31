package httpapi

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

// protected 串联 Bearer 认证、并发准入与请求超时上下文。
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

// authorized 只接受 Authorization: Bearer <exact-token>，使用常数时间比较。
// 必须带 "Bearer " 前缀：裸 token 不符合契约，即使值正确也拒绝。
func authorized(r *http.Request, token string) bool {
	if token == "" {
		return false
	}
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	value := strings.TrimPrefix(header, prefix)
	return len(value) == len(token) && subtle.ConstantTimeCompare([]byte(value), []byte(token)) == 1
}

// recoverMiddleware 把 handler panic 转换为稳定的 JSON 500 响应，
// 堆栈只写入日志，绝不返回给客户端。
func recoverMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				logger.Error("handler panic", "panic", rec, "method", r.Method, "path", r.URL.Path, "stack", string(debug.Stack()))
				writeError(w, http.StatusInternalServerError, errors.New("internal server error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
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
