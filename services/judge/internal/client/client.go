// Package client holds judge's HTTP clients to pokedex-svc and calc-svc (ADR-0700 §2-4).
// judge calls only their public APIs, per request, and folds every upstream failure into
// one of four sentinel errors so the rest of judge never sees upstream detail.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	deviceIDHeader  = "X-Device-Id"
	sessionIDHeader = "X-Session-Id"

	// maxResponseBodyBytes: judge reads at most one species detail or one calc result per
	// call, never a whole master export. 1 MiB is a generous bound that still stops an
	// unbounded read against a misbehaving upstream (ADR-0700 §2).
	maxResponseBodyBytes = 1 << 20
)

// Sentinel errors judge's callers switch on with errors.Is (ADR-0700 §3). The upstream's
// status line, body, and URL never appear in their text; that detail is for logs only.
var (
	ErrUpstreamUnavailable     = errors.New("upstream unavailable")
	ErrUpstreamInvalidResponse = errors.New("upstream response does not match the contract")
	ErrNotFound                = errors.New("not found")
	ErrInvalidRequest          = errors.New("invalid request")
)

// errBodyTooLarge is an internal signal from readLimitedBody; callers fold it into
// ErrUpstreamInvalidResponse before it ever reaches a caller of this package.
var errBodyTooLarge = errors.New("response body exceeds the limit")

// RequestContext carries the caller's device/session identity. judge forwards it as-is and
// never mints its own (CLAUDE.md 技術規約).
type RequestContext struct {
	DeviceID  string
	SessionID string
}

func (rc RequestContext) validate() error {
	if strings.TrimSpace(rc.DeviceID) == "" || strings.TrimSpace(rc.SessionID) == "" {
		return fmt.Errorf("%w: X-Device-Id and X-Session-Id are required", ErrInvalidRequest)
	}
	return nil
}

// Config is the shared constructor input for Pokedex and Calc (ADR-0700 §3).
type Config struct {
	BaseURL string
	Timeout time.Duration
}

// StatBlock is the 6-stat shape shared by SpeciesDetail.baseStats and Individual.sp in the
// root api/openapi.yaml (their meaning is defined there, not here). The json tags matter in
// both directions: Calc.Damage marshals a StatBlock into the request body, and Pokedex.Species
// decodes one out of the response (via statBlockWire, which checks presence field by field).
type StatBlock struct {
	HP  int `json:"hp"`
	Atk int `json:"atk"`
	Def int `json:"def"`
	SpA int `json:"spa"`
	SpD int `json:"spd"`
	Spe int `json:"spe"`
}

// buildHTTPClient validates config (ADR-0700 §3: base URL must be an absolute http/https URL
// with a host and no query; timeout must be positive) and returns the normalized base URL
// (no trailing slash) plus an *http.Client bound to that timeout.
func buildHTTPClient(config Config) (string, *http.Client, error) {
	if config.Timeout <= 0 {
		return "", nil, fmt.Errorf("upstream timeout must be positive: %v", config.Timeout)
	}
	u, err := url.Parse(config.BaseURL)
	if err != nil {
		return "", nil, fmt.Errorf("invalid base URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", nil, fmt.Errorf("base URL must be an absolute http/https URL: %q", config.BaseURL)
	}
	if u.Host == "" {
		return "", nil, fmt.Errorf("base URL must have a host: %q", config.BaseURL)
	}
	if u.RawQuery != "" {
		return "", nil, fmt.Errorf("base URL must not contain a query: %q", config.BaseURL)
	}
	return strings.TrimSuffix(u.String(), "/"), &http.Client{Timeout: config.Timeout}, nil
}

// send validates rc, attaches the forwarded identity headers, and executes req.
// It normalizes the outcome to the ADR-0700 §3 sentinels: a connection failure or the
// client's own timeout becomes ErrUpstreamUnavailable, while the caller's own ctx ending
// (e.g. the judge API handler's client disconnecting) is returned as-is so callers can
// still see context.Canceled / context.DeadlineExceeded via errors.Is.
func send(ctx context.Context, httpClient *http.Client, req *http.Request, rc RequestContext) (*http.Response, error) {
	if err := rc.validate(); err != nil {
		return nil, err
	}
	req.Header.Set(deviceIDHeader, rc.DeviceID)
	req.Header.Set(sessionIDHeader, rc.SessionID)

	resp, err := httpClient.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("request context ended: %w", ctxErr)
		}
		return nil, fmt.Errorf("%w: %v", ErrUpstreamUnavailable, err)
	}

	if statusErr := statusToError(resp.StatusCode); statusErr != nil {
		_ = resp.Body.Close()
		return nil, statusErr
	}
	return resp, nil
}

// statusToError maps an upstream status code to the ADR-0700 §3 table. http.StatusOK maps
// to nil (proceed to decode the body); everything else that isn't 400/404/5xx is a 2xx/3xx
// the contract doesn't describe, so it counts as an invalid response, not a success.
func statusToError(status int) error {
	switch {
	case status == http.StatusOK:
		return nil
	case status == http.StatusNotFound:
		return ErrNotFound
	case status == http.StatusBadRequest:
		return ErrInvalidRequest
	case status >= http.StatusInternalServerError:
		return ErrUpstreamUnavailable
	default:
		return ErrUpstreamInvalidResponse
	}
}

// decodeUpstreamJSON reads resp.Body up to maxResponseBodyBytes and decodes it as JSON into
// out. Any failure (oversized body, broken JSON, wrong shape) becomes ErrUpstreamInvalidResponse.
func decodeUpstreamJSON(resp *http.Response, out any) error {
	defer func() { _ = resp.Body.Close() }()

	data, err := readLimitedBody(resp.Body)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUpstreamInvalidResponse, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%w: %v", ErrUpstreamInvalidResponse, err)
	}
	return nil
}

func readLimitedBody(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxResponseBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxResponseBodyBytes {
		return nil, errBodyTooLarge
	}
	return data, nil
}
