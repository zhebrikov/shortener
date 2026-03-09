package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// compressibleTypes — типы контента, для которых включаем сжатие ответа.
var compressibleTypes = map[string]bool{
	"application/json": true,
	"text/html":        true,
}

// Gzip обрабатывает сжатие: распаковывает тело запроса при Content-Encoding: gzip
// и сжимает ответ при Accept-Encoding: gzip только для application/json и text/html.
func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			gzReader, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer gzReader.Close()
			r.Body = gzReader
		}

		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		buf := &gzipResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			headers:        make(http.Header),
		}
		next.ServeHTTP(buf, r)
		buf.flush()
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	statusCode int
	statusSet  bool
	headers    http.Header
	body       bytes.Buffer
}

func (w *gzipResponseWriter) Header() http.Header {
	return w.headers
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	if w.statusSet {
		return
	}
	w.statusCode = code
	w.statusSet = true
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.statusSet {
		w.WriteHeader(http.StatusOK)
	}
	return w.body.Write(b)
}

func (w *gzipResponseWriter) flush() {
	contentType := w.headers.Get("Content-Type")
	// Берём только базовый тип (до точки с запятой)
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = strings.TrimSpace(contentType[:idx])
	}
	contentType = strings.TrimSpace(strings.ToLower(contentType))

	shouldCompress := compressibleTypes[contentType]

	if shouldCompress {
		// Копируем заголовки в реальный ResponseWriter, добавляем Content-Encoding
		for k, v := range w.headers {
			for _, vv := range v {
				w.ResponseWriter.Header().Add(k, vv)
			}
		}
		w.ResponseWriter.Header().Set("Content-Encoding", "gzip")
		w.ResponseWriter.Header().Del("Content-Length") // размер тела изменился
		w.ResponseWriter.WriteHeader(w.statusCode)

		gz := gzip.NewWriter(w.ResponseWriter)
		_, _ = io.Copy(gz, &w.body)
		_ = gz.Close()
		return
	}

	// Без сжатия: копируем заголовки и тело как есть
	for k, v := range w.headers {
		for _, vv := range v {
			w.ResponseWriter.Header().Add(k, vv)
		}
	}
	w.ResponseWriter.WriteHeader(w.statusCode)
	_, _ = io.Copy(w.ResponseWriter, &w.body)
}
