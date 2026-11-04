package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"icinidok/api/internal/app"
)

func main() {
	cfg := app.LoadConfig()
	store, err := app.OpenStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("veritabanı açılamadı: %v", err)
	}
	defer store.Close()

	server := app.New(cfg, store)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			server.Sweep()
			if err := store.Cleanup(time.Now()); err != nil {
				log.Printf("cleanup: %v", err)
			}
		}
	}()

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("içini dök. %s üzerinde dinliyor (web: %s, db: %s)", cfg.Addr, cfg.WebDir, cfg.DBPath)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("sunucu: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("kapatma: %v", err)
	}
}
