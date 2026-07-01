// Command memprofile exercises hot paths and writes a heap profile for pprof analysis.
//
// Usage:
//
//	go run ./cmd/memprofile -output profiles/base.pprof
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/zhebrikov/shortener/internal/asyncdelete"
	"github.com/zhebrikov/shortener/internal/handler"
	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	out := flag.String("output", "profiles/heap.pprof", "path to write heap profile")
	links := flag.Int("links", 5000, "number of links to preload")
	ops := flag.Int("ops", 50000, "number of read/create operations after preload")
	flag.Parse()

	if err := os.MkdirAll("profiles", 0o755); err != nil {
		return fmt.Errorf("mkdir profiles: %w", err)
	}

	shortener := service.NewShortener("http://localhost:8080")
	store := storage.NewMemoryStorage()

	for i := 0; i < *links; i++ {
		original := fmt.Sprintf("https://example.com/path/%d", i)
		shortURL, err := shortener.CreateLink(original)
		if err != nil {
			return fmt.Errorf("CreateLink: %w", err)
		}
		uuid, err := storage.NewLinkUUID()
		if err != nil {
			return fmt.Errorf("NewLinkUUID: %w", err)
		}
		if err := store.WriteStorage(storage.Link{
			UUID:        uuid,
			ShortURL:    shortURL,
			OriginalURL: original,
		}); err != nil {
			return fmt.Errorf("WriteStorage: %w", err)
		}
	}

	var codes []string
	for i := 0; i < *links; i++ {
		if i%(max(*links/8, 1)) != 0 {
			continue
		}
		original := fmt.Sprintf("https://example.com/path/%d", i)
		shortURL, err := shortener.CreateLink(original)
		if err != nil {
			continue
		}
		codes = append(codes, storage.ShortCodeFromURL(shortURL))
	}
	if len(codes) == 0 {
		return fmt.Errorf("no sample short codes collected")
	}

	jsonBody, _ := json.Marshal(handler.Input{URL: "https://bench.example/new"})
	plainBody := []byte("https://bench.example/plain")
	h := handler.NewShortenerHandler(shortener, store, asyncdelete.NewWorker(store), nil)

	for i := 0; i < *ops; i++ {
		switch i % 4 {
		case 0:
			code := codes[i%len(codes)]
			req := httptest.NewRequest(http.MethodGet, "/"+code, nil)
			rr := httptest.NewRecorder()
			handler.GetLink(rr, req, h)
		case 1:
			req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			handler.CreateLinkJSON(rr, req, h)
		case 2:
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(plainBody))
			req.Header.Set("Content-Type", "text/plain")
			rr := httptest.NewRecorder()
			handler.CreateLink(rr, req, h)
		case 3:
			_, _ = shortener.GetLink(codes[0], store)
		}
	}

	runtime.GC()
	f, err := os.Create(*out)
	if err != nil {
		return fmt.Errorf("create profile: %w", err)
	}
	defer f.Close()
	if err := pprof.WriteHeapProfile(f); err != nil {
		return fmt.Errorf("write heap profile: %w", err)
	}
	fmt.Printf("heap profile written to %s\n", *out)
	return nil
}
