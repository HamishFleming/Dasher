package main

import (
	"context"
	"dash-analyzer/internal/api"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		dir = "data"
	}
	a, err := api.New(dir)
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		for {
			fetchCtx, cancel := context.WithTimeout(ctx, 130*time.Second)
			err := a.Refresh(fetchCtx, false)
			cancel()
			if err != nil {
				log.Printf("Restaurant refresh failed; retaining cache: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Hour):
			}
		}
	}()
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		addr = "127.0.0.1:" + port
	}
	srv := &http.Server{Addr: addr, Handler: a.Handler("web"), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 150 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Printf("Go API http://%s (legacy dashboard /web/)", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
