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

	"example.com/pokecalc/services/balance/internal/httpapi"
	"example.com/pokecalc/services/balance/internal/master"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 16 * 1024
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	pokemonTypes, err := pokemonTypeProviderFromEnv(os.LookupEnv)
	if err != nil {
		slog.Error("balance API failed to load pokemon type read model", "path", os.Getenv(pokemonTypesPathEnv), "error", err)
		os.Exit(1)
	}

	moves, err := moveProviderFromEnv(os.LookupEnv)
	if err != nil {
		slog.Error("balance API failed to load move read model", "path", os.Getenv(movesPathEnv), "error", err)
		os.Exit(1)
	}

	abilities, err := abilityProviderFromEnv(os.LookupEnv)
	if err != nil {
		slog.Error("balance API failed to load ability read model", "path", os.Getenv(abilitiesPathEnv), "error", err)
		os.Exit(1)
	}

	typeChart, err := master.EmbeddedTypeChart()
	if err != nil {
		slog.Error("balance API failed to load the type chart", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.New(httpapi.Dependencies{TypeChart: typeChart, PokemonTypes: pokemonTypes, Moves: moves, Abilities: abilities}),
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
		slog.Info("balance API listening", "address", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("balance API stopped", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("balance API shutdown failed", "error", err)
			os.Exit(1)
		}
	}
}
