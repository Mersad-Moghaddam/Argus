package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"argus/internal/app"
	"argus/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	application, err := app.New(context.Background(), cfg)
	if err != nil {
		log.Fatalf("initialize application: %v", err)
	}

	if err = application.Start(); err != nil {
		log.Fatalf("start application: %v", err)
	}

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)
	<-shutdownSignal

	shutdownCtx, cancel := app.DefaultShutdownContext()
	defer cancel()

	if err = application.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown application: %v", err)
	}
}
