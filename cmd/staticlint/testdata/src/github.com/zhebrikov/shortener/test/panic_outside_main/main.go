package main

func helper() {
	panic("oops") // want "panic must not be called outside main.main"
}

func main() {}
