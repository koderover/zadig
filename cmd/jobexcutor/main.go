package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/koderover/zadig/pkg/microservice/jobexcutor/excutor"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-ctx.Done()
		stop()
	}()

	if err := excutor.Execute(ctx); err != nil {
		log.Fatal(err)
	}
}
