package main

import (
	"log"

	"github.com/caarlos0/env/v6"
)

type Config struct {
	User string `env:"USER"`
}

func main() {
	// Объявляем переменную с типом
	var cfg Config
	// Заполняем переменную из env окружения
	err := env.Parse(&cfg)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Current user is %s\n", cfg.User)
}
