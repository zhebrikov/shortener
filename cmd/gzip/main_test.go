package main

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDefaultHandle(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", defaultHandle)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("defaultHandle: status = %d; want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html" {
		t.Errorf("defaultHandle: Content-Type = %q; want %q", ct, "text/html")
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Hello, world") {
		t.Errorf("defaultHandle: body must contain %q; got %q", "Hello, world", body)
	}
	// 20 раз "Hello, world<br>"
	wantSnippet := strings.Repeat("Hello, world<br>", 20)
	if !strings.Contains(body, wantSnippet) {
		t.Errorf("defaultHandle: body must contain repeated snippet; got len %d", len(body))
	}
}

func TestGzipHandle(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", defaultHandle)
	handler := gzipHandle(mux)

	t.Run("without_accept_encoding", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
		}
		if enc := rec.Header().Get("Content-Encoding"); enc != "" {
			t.Errorf("Content-Encoding = %q; want empty (no compression)", enc)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "Hello, world") {
			t.Errorf("body must contain %q", "Hello, world")
		}
	})

	t.Run("with_accept_encoding_gzip", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
		}
		if enc := rec.Header().Get("Content-Encoding"); enc != "gzip" {
			t.Errorf("Content-Encoding = %q; want gzip", enc)
		}

		gr, err := gzip.NewReader(rec.Body)
		if err != nil {
			t.Fatalf("gzip.NewReader: %v", err)
		}
		defer gr.Close()
		decoded, err := io.ReadAll(gr)
		if err != nil {
			t.Fatalf("gzip read: %v", err)
		}
		if !bytes.Contains(decoded, []byte("Hello, world")) {
			t.Errorf("decoded body must contain Hello, world; got %q", string(decoded))
		}
	})
}

func TestDeflateHandle(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", defaultHandle)
	handler := deflateHandle(mux)

	t.Run("without_accept_encoding", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
		}
		if enc := rec.Header().Get("Content-Encoding"); enc != "" {
			t.Errorf("Content-Encoding = %q; want empty", enc)
		}
		if !strings.Contains(rec.Body.String(), "Hello, world") {
			t.Errorf("body must contain Hello, world")
		}
	})

	t.Run("with_accept_encoding_deflate", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "deflate")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
		}
		if enc := rec.Header().Get("Content-Encoding"); enc != "deflate" {
			t.Errorf("Content-Encoding = %q; want deflate", enc)
		}

		zr, err := zlib.NewReader(rec.Body)
		if err != nil {
			t.Fatalf("zlib.NewReader: %v", err)
		}
		defer zr.Close()
		decoded, err := io.ReadAll(zr)
		if err != nil {
			t.Fatalf("zlib read: %v", err)
		}
		if !bytes.Contains(decoded, []byte("Hello, world")) {
			t.Errorf("decoded body must contain Hello, world; got %q", string(decoded))
		}
	})
}

func TestGzipWriter_Write(t *testing.T) {
	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	w := gzipWriter{ResponseWriter: rec, Writer: gz}

	data := []byte("test data")
	n, err := w.Write(data)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write returned n = %d; want %d", n, len(data))
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	gr, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gr.Close()
	decoded, _ := io.ReadAll(gr)
	if string(decoded) != "test data" {
		t.Errorf("decoded = %q; want %q", decoded, "test data")
	}
}

func TestZlibWriter_Write(t *testing.T) {
	var buf bytes.Buffer
	zl, err := zlib.NewWriterLevel(&buf, flate.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	w := zlibWriter{ResponseWriter: rec, Writer: zl}

	data := []byte("deflate test")
	n, err := w.Write(data)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write returned n = %d; want %d", n, len(data))
	}
	if err := zl.Close(); err != nil {
		t.Fatal(err)
	}

	zr, err := zlib.NewReader(&buf)
	if err != nil {
		t.Fatalf("zlib.NewReader: %v", err)
	}
	defer zr.Close()
	decoded, _ := io.ReadAll(zr)
	if string(decoded) != "deflate test" {
		t.Errorf("decoded = %q; want %q", decoded, "deflate test")
	}
}
