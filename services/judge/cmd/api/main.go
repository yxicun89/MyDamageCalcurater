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

	"example.com/pokecalc/services/judge/internal/api"
	"example.com/pokecalc/services/judge/internal/httpapi"
	"example.com/pokecalc/services/judge/internal/httpguard"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 16 * 1024

	// maxInflight は同時に処理する判定リクエストの数。超えた分は待たせず 503 upstream_unavailable +
	// Retry-After(issue #299・ADR-0801)。全体の締め切りは JUDGE_REQUEST_TIMEOUT(ADR-0707)。
	maxInflight = 16
)

// guard は同時実行の上限だけを持つ(締め切りは requestTimeout が担う)。code は契約が宣言済みのもの。
var guard = httpguard.Config{MaxInflight: maxInflight, Code: string(api.UpstreamUnavailable)}

func main() {
	port := portFromEnv(os.LookupEnv)

	upstreams, err := upstreamsFromEnv(os.LookupEnv)
	if err != nil {
		slog.Error("judge API failed to configure upstreams", "error", err)
		os.Exit(1)
	}
	choiceScarfItemID := choiceScarfItemIDFromEnv(os.LookupEnv)
	requestTimeout, err := requestTimeoutFromEnv(os.LookupEnv, writeTimeout)
	if err != nil {
		slog.Error("judge API failed to configure request timeout", "error", err)
		os.Exit(1)
	}

	speedEffects, err := speedEffectsFromEnv(os.LookupEnv, upstreams.Pokedex)
	if err != nil {
		slog.Error("judge API failed to configure speed effects", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr: ":" + port,
		Handler: httpapi.New(httpapi.Dependencies{
			Pokedex:           upstreams.Pokedex,
			Calc:              upstreams.Calc,
			ChoiceScarfItemID: choiceScarfItemID,
			RequestTimeout:    requestTimeout,
			Guard:             guard,
			SpeedEffects:      speedEffects,
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
		if err := gracefulShutdown(server, shutdownTimeout); err != nil {
			slog.Warn("judge API shutdown did not finish in time; remaining connections closed", "error", err)
		}
	}
}
