package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"GoDispatch/dispatcher/api"
	"GoDispatch/dispatcher/config"
	"GoDispatch/dispatcher/controller"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create controller
	ctrl, err := controller.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create controller: %v", err)
	}

	// Start the controller
	if err := ctrl.Start(); err != nil {
		log.Fatalf("Failed to start controller: %v", err)
	}
	defer ctrl.Stop()

	// Create and start API server
	server := api.NewServer(cfg, ctrl)
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Failed to start API server: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Graceful shutdown
	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Stop(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	log.Println("Server exited properly")
}
