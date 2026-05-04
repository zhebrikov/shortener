// Command gophermart runs the Gophermart loyalty HTTP API server.
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, defaultRunDeps()); err != nil {
		log.Fatal(err)
	}
}
