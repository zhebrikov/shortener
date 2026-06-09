package main

import (
	"errors"
	"log"
)

func helper() error {
	return errors.New("fail")
}

func main() {
	if err := helper(); err != nil {
		log.Fatal(err)
	}
}
