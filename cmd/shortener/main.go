package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/zhebrikov/shortener/internal/handler"
	"github.com/zhebrikov/shortener/internal/service"
)

func main() {
	shortener := service.NewShortener()
	handler := handler.NewShortenerHandler(shortener)

	http.HandleFunc("/", handler.CreateLink)

	port := ":8080"
	fmt.Println("Сервер запущен на http://localhost" + port)

	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal(err)
	}
}
