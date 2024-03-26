package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type (
	loggingResponseWriter struct {
		http.ResponseWriter
		wroteHeader bool
		code        int
		bytes       int
		error       string
	}
)

func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}

	if r.code >= 300 {
		r.error = string(b)
	}

	size, err := r.ResponseWriter.Write(b)
	r.bytes += size
	return size, err
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	if !r.wroteHeader {
		r.code = statusCode
		r.wroteHeader = true
		r.ResponseWriter.WriteHeader(statusCode)
	}
}

var Log *zap.Logger = zap.NewNop()

func Initialize(level string, outputPath string) error {
	lvl, err := zap.ParseAtomicLevel(level)

	if err != nil {
		return fmt.Errorf("parse level %s: %w", level, err)
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = lvl
	cfg.OutputPaths = []string{outputPath}
	zl, err := cfg.Build()

	if err != nil {
		return fmt.Errorf("build logger: %w", err)
	}

	Log = zl

	return nil
}

func RequestLogger(h http.Handler) http.Handler {
	logFn := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		var buf bytes.Buffer
		_, err := buf.ReadFrom(r.Body)

		body := ""
		if err == nil {
			body = buf.String()
			var data map[string]interface{}
			err := json.Unmarshal(buf.Bytes(), &data)

			if err == nil {
				if _, ok := data["password"].(string); ok {
					data["password"] = "********"
				}

				newBody, err := json.Marshal(data)
				if err == nil {
					body = string(newBody)
				}
			}
		}

		Log.Info("Got incoming HTTP request",
			zap.String("uri", r.RequestURI),
			zap.String("method", r.Method),
			zap.String("body", body),
		)

		r.Body = io.NopCloser(&buf)

		lw := loggingResponseWriter{
			ResponseWriter: w,
		}

		defer func() {
			duration := time.Since(start)

			Log.Info("Sending HTTP response",
				zap.String("duration", duration.String()),
				zap.Int("status", lw.code),
				zap.Int("size", lw.bytes),
				zap.String("error", lw.error),
			)
		}()

		h.ServeHTTP(&lw, r)
	}

	return http.HandlerFunc(logFn)
}
