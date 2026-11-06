package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/server"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	if err := server.Serve(ctx); err != nil {
		log.Fatal(err)
	}
}
