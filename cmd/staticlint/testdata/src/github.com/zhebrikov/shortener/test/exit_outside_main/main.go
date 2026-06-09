package main

import "os"

func helper() {
	os.Exit(1) // want "os.Exit must not be called outside main.main"
}

func main() {}
