package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultLogDir = "logs"

type accessLogWriter struct {
	file   *os.File
	logger *log.Logger
}

func openAccessLog(logDir string) (*accessLogWriter, error) {
	logDir = strings.TrimSpace(logDir)
	if logDir == "" {
		logDir = defaultLogDir
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("create log dir %q: %w", logDir, err)
	}
	path := filepath.Join(logDir, "http-access.log")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open access log %q: %w", path, err)
	}
	mw := io.MultiWriter(os.Stdout, f)
	return &accessLogWriter{
		file:   f,
		logger: log.New(mw, "", log.LstdFlags),
	}, nil
}

func (a *accessLogWriter) Close() error {
	if a == nil || a.file == nil {
		return nil
	}
	return a.file.Close()
}

// responseRecorder wraps http.ResponseWriter for status code; delegates http.Flusher for SSE.
type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func withHTTPAccessLog(next http.Handler, logger *log.Logger) http.Handler {
	if logger == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		st := rec.status
		if st == 0 {
			st = http.StatusOK
		}
		ua := r.Header.Get("User-Agent")
		if ua == "" {
			ua = "-"
		}
		const uaMax = 160
		if len(ua) > uaMax {
			ua = ua[:uaMax-3] + "..."
		}
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" {
			xff = " xff=" + truncateRun(xff, 120)
		}
		logger.Printf("http %s %q %s %d %s ua=%q%s",
			r.Method, r.URL.RequestURI(), r.RemoteAddr, st, time.Since(start).Round(time.Millisecond), ua, xff)
	})
}

func truncateRun(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
