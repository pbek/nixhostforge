package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/omega/nixhostforge/internal/app"
)

func main() {
	configPath := flag.String("config", "", "path to config.toml")
	flag.Parse()

	cfg, err := app.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	svc, err := app.New(cfg)
	if err != nil {
		log.Fatalf("init service: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			log.Printf("close service: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.Port),
		Handler:           svc.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go svc.Run(ctx)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("NixHostForge listening on http://%s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
}
