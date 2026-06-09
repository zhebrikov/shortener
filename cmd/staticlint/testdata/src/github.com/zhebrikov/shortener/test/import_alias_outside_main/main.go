package main

import (
	mylog "log"
	myos "os"
)

func helper() {
	myos.Exit(1)       // want "os.Exit must not be called outside main.main"
	mylog.Fatal("err") // want "log.Fatal must not be called outside main.main"
}

func main() {}
