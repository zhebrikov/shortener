package main

import (
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type gzipWriter struct {
	http.ResponseWriter
	Writer *gzip.Writer
}

type zlibWriter struct {
	http.ResponseWriter
	Writer *zlib.Writer
}

func (w zlibWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (w gzipWriter) Write(b []byte) (int, error) {
	// w.Writer будет отвечать за gzip-сжатие, поэтому пишем в него
	return w.Writer.Write(b)
}

func defaultHandle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	io.WriteString(w, "<html><body>"+strings.Repeat("Hello, world<br>", 20)+"</body></html>")
}

func gzipHandle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if err != nil {
			io.WriteString(w, err.Error())
			return
		}
		defer gz.Close()

		w.Header().Set("Content-Encoding", "gzip")
		// передаём обработчику страницы переменную типа gzipWriter для вывода данных
		next.ServeHTTP(gzipWriter{ResponseWriter: w, Writer: gz}, r)
	})
}

func deflateHandle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// проверяем, что клиент поддерживает deflate-сжатие
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "deflate") {
			next.ServeHTTP(w, r)
			return
		}
		flatew, err := zlib.NewWriterLevel(w, flate.BestCompression)
		if err != nil {
			io.WriteString(w, err.Error())
			return
		}
		defer flatew.Close()

		w.Header().Set("Content-Encoding", "deflate")
		next.ServeHTTP(zlibWriter{ResponseWriter: w, Writer: flatew}, r)
	})
}

func LengthHandle(w http.ResponseWriter, r *http.Request) {
	var reader io.Reader

	if r.Header.Get(`Content-Encoding`) != `gzip` {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		reader = gz
		defer gz.Close()
	} else {
		reader = r.Body
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "Length: %d", len(body))
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", defaultHandle)
	mux.HandleFunc("/length", LengthHandle)
	if err := http.ListenAndServe(":3123", deflateHandle(mux)); err != nil {
		log.Fatal(err)
	}
}
