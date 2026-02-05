package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/zhebrikov/shortener/internal/handler"
	"github.com/zhebrikov/shortener/internal/service"
)

func main() {

	url := flag.String("a", "localhost:8080", "url of the server")
	flag.Parse()

	shortener := service.NewShortener(*url)
	handler := handler.NewShortenerHandler(shortener)

	r := chi.NewRouter()
	r.Post("/", handler.CreateLink)
	r.Get("/{shortCode}", handler.GetLink)

	idx := strings.Index(*url, ":")
	if idx == -1 {
		panic("url must contain port")
	}

	port := (*url)[idx:]
	fmt.Println(port)
	fmt.Println("Сервер запущен на http://localhost" + port)

	err := http.ListenAndServe(port, r)
	if err != nil {
		log.Fatal(err)
	}
}
