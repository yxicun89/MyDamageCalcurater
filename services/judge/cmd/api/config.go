package main

import (
	"fmt"
	"strings"
	"time"

	"example.com/pokecalc/services/judge/internal/client"
	"example.com/pokecalc/services/judge/internal/judge"
)

const (
	portEnv     = "PORT"
	defaultPort = "8080"

	pokedexBaseURLEnv      = "JUDGE_POKEDEX_BASE_URL"
	calcBaseURLEnv         = "JUDGE_CALC_BASE_URL"
	upstreamTimeoutEnv     = "JUDGE_UPSTREAM_TIMEOUT"
	defaultUpstreamTimeout = 3 * time.Second

	choiceScarfItemIDEnv = "JUDGE_CHOICE_SCARF_ITEM_ID"
)

// Upstreams holds the upstream clients judge depends on (ADR-0700 §1). A nil field means its
// base URL was not configured: judge still starts, health stays 200, and the judge API (JD1)
// answers 503 (the same posture as speed's read model; ADR-0600 §4).
type Upstreams struct {
	Pokedex *client.Pokedex
	Calc    *client.Calc
}

// portFromEnv returns PORT, defaulting to 8080 when unset or empty.
func portFromEnv(lookup func(string) (string, bool)) string {
	value, ok := lookup(portEnv)
	if !ok || value == "" {
		return defaultPort
	}
	return value
}

// upstreamTimeoutFromEnv reads JUDGE_UPSTREAM_TIMEOUT as a time.Duration (ADR-0700 §1:
// default 3s; judge calls upstreams within a single client request, so the default must
// stay in a range a human can wait out).
func upstreamTimeoutFromEnv(lookup func(string) (string, bool)) (time.Duration, error) {
	value, ok := lookup(upstreamTimeoutEnv)
	if !ok || value == "" {
		return defaultUpstreamTimeout, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s is not a valid duration: %w", upstreamTimeoutEnv, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive: %v", upstreamTimeoutEnv, d)
	}
	return d, nil
}

// upstreamsFromEnv builds the upstream clients from JUDGE_POKEDEX_BASE_URL /
// JUDGE_CALC_BASE_URL and JUDGE_UPSTREAM_TIMEOUT (ADR-0700 §1). An unset/empty base URL
// leaves that client nil and does not fail startup; a configured-but-invalid base URL or
// timeout does (avoids a half-usable service that only main would notice later).
func upstreamsFromEnv(lookup func(string) (string, bool)) (Upstreams, error) {
	timeout, err := upstreamTimeoutFromEnv(lookup)
	if err != nil {
		return Upstreams{}, err
	}

	pokedex, err := optionalClient(lookup, pokedexBaseURLEnv, timeout, client.NewPokedex)
	if err != nil {
		return Upstreams{}, err
	}
	calc, err := optionalClient(lookup, calcBaseURLEnv, timeout, client.NewCalc)
	if err != nil {
		return Upstreams{}, err
	}
	return Upstreams{Pokedex: pokedex, Calc: calc}, nil
}

// choiceScarfItemIDFromEnv reads JUDGE_CHOICE_SCARF_ITEM_ID (ADR-0701 §3): unset, empty, or
// whitespace-only falls back to judge.DefaultChoiceScarfItemID. This is the one seam that lets
// an overlay fix a wrong choice-scarf item ID without a code change (the real master isn't in
// Git; ADR-0002).
func choiceScarfItemIDFromEnv(lookup func(string) (string, bool)) string {
	value, ok := lookup(choiceScarfItemIDEnv)
	if !ok {
		return judge.DefaultChoiceScarfItemID
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return judge.DefaultChoiceScarfItemID
	}
	return value
}

// optionalClient builds a client with the given constructor only when envName is set to a
// non-empty value; otherwise it returns the zero value (nil for the pointer client types
// used here) without error.
func optionalClient[T any](lookup func(string) (string, bool), envName string, timeout time.Duration, build func(client.Config) (T, error)) (T, error) {
	var zero T
	baseURL, ok := lookup(envName)
	if !ok || baseURL == "" {
		return zero, nil
	}
	return build(client.Config{BaseURL: baseURL, Timeout: timeout})
}
