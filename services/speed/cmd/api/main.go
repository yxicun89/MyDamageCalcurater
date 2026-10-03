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

	"example.com/pokecalc/services/speed/internal/httpapi"
	"example.com/pokecalc/services/speed/internal/httpguard"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 16 * 1024

	// maxInflight は同時に処理する API リクエストの数。超えた分は待たせず 503 overloaded + Retry-After
	// (issue #299・ADR-0801)。
	maxInflight = 32
)

// guard はハンドラ全体の締め切り(writeTimeout - 1 秒)と同時実行の上限。
var guard = httpguard.Config{MaxInflight: maxInflight, Timeout: httpguard.DeadlineFor(writeTimeout)}

func main() {
	port := portFromEnv(os.LookupEnv)

	pokemon, err := pokemonProviderFromEnv(os.LookupEnv)
	if err != nil {
		slog.Error("speed API failed to load pokemon read model", "path", os.Getenv(pokemonPathEnv), "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.New(httpapi.Dependencies{Pokemon: pokemon, Guard: guard}),
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
		slog.Info("speed API listening", "address", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("speed API stopped", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		if err := gracefulShutdown(server, shutdownTimeout); err != nil {
			slog.Warn("speed API shutdown did not finish in time; remaining connections closed", "error", err)
		}
	}
}
