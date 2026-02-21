package main

import (
	"bytes"
	"fmt"
	"log"
)

// run выполняет логирование в буфер и возвращает его содержимое (для тестов).
func run() string {
	var buf bytes.Buffer
	var logger *log.Logger
	logger = log.New(&buf, "mylog: ", log.LstdFlags)
	logger.Println("Hello, world!")
	logger.Println("Goodbye")
	return buf.String()
}

func main() {
	fmt.Print(run())
}
