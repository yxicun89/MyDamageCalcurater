package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/pokecalc/services/judge/internal/httpapi"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 16 * 1024
)

func main() {
	port := portFromEnv(os.LookupEnv)

	upstreams, err := upstreamsFromEnv(os.LookupEnv)
	if err != nil {
		slog.Error("judge API failed to configure upstreams", "error", err)
		os.Exit(1)
	}
	choiceScarfItemID := choiceScarfItemIDFromEnv(os.LookupEnv)

	server := &http.Server{
		Addr: ":" + port,
		Handler: httpapi.New(httpapi.Dependencies{
			Pokedex:           upstreams.Pokedex,
			Calc:              upstreams.Calc,
			ChoiceScarfItemID: choiceScarfItemID,
		}),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		slog.Info("judge API listening", "address", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("judge API stopped", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("judge API shutdown failed", "error", err)
			os.Exit(1)
		}
	}
}
