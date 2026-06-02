package main

import "log"

func helper() {
	log.Fatal("error") // want "log.Fatal must not be called outside main.main"
}

func main() {}
