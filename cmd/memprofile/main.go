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

	"github.com/zhebrikov/shortener/internal/handler"
	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
)

func main() {
	out := flag.String("output", "profiles/heap.pprof", "path to write heap profile")
	links := flag.Int("links", 5000, "number of links to preload")
	ops := flag.Int("ops", 50000, "number of read/create operations after preload")
	flag.Parse()

	if err := os.MkdirAll("profiles", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir profiles: %v\n", err)
		os.Exit(1)
	}

	shortener := service.NewShortener("http://localhost:8080")
	store := storage.NewMemoryStorage()

	for i := 0; i < *links; i++ {
		original := fmt.Sprintf("https://example.com/path/%d", i)
		shortURL, err := shortener.CreateLink(original)
		if err != nil {
			fmt.Fprintf(os.Stderr, "CreateLink: %v\n", err)
			os.Exit(1)
		}
		uuid, err := storage.NewLinkUUID()
		if err != nil {
			fmt.Fprintf(os.Stderr, "NewLinkUUID: %v\n", err)
			os.Exit(1)
		}
		if err := store.WriteStorage(storage.Link{
			UUID:        uuid,
			ShortURL:    shortURL,
			OriginalURL: original,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "WriteStorage: %v\n", err)
			os.Exit(1)
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
		fmt.Fprintln(os.Stderr, "no sample short codes collected")
		os.Exit(1)
	}

	jsonBody, _ := json.Marshal(handler.Input{URL: "https://bench.example/new"})
	plainBody := []byte("https://bench.example/plain")

	for i := 0; i < *ops; i++ {
		switch i % 4 {
		case 0:
			code := codes[i%len(codes)]
			req := httptest.NewRequest(http.MethodGet, "/"+code, nil)
			rr := httptest.NewRecorder()
			handler.GetLink(rr, req, shortener, store, nil)
		case 1:
			req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			handler.CreateLinkJSON(rr, req, shortener, store, nil)
		case 2:
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(plainBody))
			req.Header.Set("Content-Type", "text/plain")
			rr := httptest.NewRecorder()
			handler.CreateLink(rr, req, shortener, store, nil)
		case 3:
			_, _ = shortener.GetLink(codes[0], store)
		}
	}

	runtime.GC()
	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create profile: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()
	if err := pprof.WriteHeapProfile(f); err != nil {
		fmt.Fprintf(os.Stderr, "write heap profile: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("heap profile written to %s\n", *out)
}
