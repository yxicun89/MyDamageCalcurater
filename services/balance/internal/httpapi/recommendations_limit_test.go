package httpapi

// issue #298(ADR-0409): recommendations の同時実行数の上限と、超過時の 503 overloaded。
// 超過したリクエストは待たせず即座に 503。上限は Dependencies.MaxConcurrentRecommendations
// (0 以下は既定の DefaultMaxConcurrentRecommendations)。待ち合わせは channel だけで行い、sleep に頼らない。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/services/balance/internal/api"
	"example.com/pokecalc/services/balance/internal/balance"
)

// gatedCatalog blocks every AllPokemon call until release is closed, and reports each entry on entered.
// The recommendations handler calls AllPokemon while it holds a concurrency slot.
type gatedCatalog struct {
	testCatalog
	entered chan struct{}
	release chan struct{}
}

func (g gatedCatalog) AllPokemon() ([]balance.CatalogPokemon, error) {
	g.entered <- struct{}{}
	<-g.release
	return g.testCatalog.AllPokemon()
}

func newGatedCatalog(capacity int) gatedCatalog {
	return gatedCatalog{testCatalog: recCatalog, entered: make(chan struct{}, capacity+8), release: make(chan struct{})}
}

func gatedDependencies(g gatedCatalog, max int) Dependencies {
	deps := recFullDependencies()
	deps.PokemonCatalog = g
	deps.MaxConcurrentRecommendations = max
	return deps
}

// holdSlots starts n requests that block inside AllPokemon and returns once all n are inside.
func holdSlots(t *testing.T, server http.Handler, g gatedCatalog, n int) *sync.WaitGroup {
	t.Helper()
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			if recorder := postRecommendations(t, server, recBody); recorder.Code != http.StatusOK {
				t.Errorf("held request status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
	for range n {
		select {
		case <-g.entered:
		case <-time.After(5 * time.Second):
			t.Fatal("held requests did not reach AllPokemon (the limit is lower than the configured value?)")
		}
	}
	return &wg
}

// postWithin posts recBody and fails (instead of hanging) when the answer takes longer than 5 seconds:
// an over-limit request must be answered at once, not queued behind the closed gate. It opens the gate
// on failure so the held requests and the stray goroutine can finish.
func postWithin(t *testing.T, server http.Handler, g gatedCatalog) *httptest.ResponseRecorder {
	t.Helper()
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- postRecommendations(t, server, recBody) }()
	select {
	case recorder := <-done:
		return recorder
	case <-time.After(5 * time.Second):
		close(g.release)
		t.Fatal("over-limit request was not answered within 5s (it must not wait for a slot)")
		return nil
	}
}

func assertOverloaded(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", recorder.Code, recorder.Body.String())
	}
	var body api.Error
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode Error: %v; body=%s", err, recorder.Body.String())
	}
	if body.Code != api.Overloaded {
		t.Errorf("code = %q, want %q", body.Code, api.Overloaded)
	}
	if body.Message == "" {
		t.Error("message is empty, want a fixed explanation")
	}
	if got := recorder.Header().Get("Retry-After"); got == "" {
		t.Error("Retry-After header is missing, want a number of seconds")
	}
}

func TestRecommendationsRejectsRequestsOverTheConcurrencyLimit(t *testing.T) {
	t.Parallel()

	const limit = 2
	g := newGatedCatalog(limit + 1)
	server := New(gatedDependencies(g, limit))
	wg := holdSlots(t, server, g, limit)

	// The limit is full: the next request is not queued, it answers 503 at once
	// (the catalog gate is still closed, so a queued request would hang here).
	assertOverloaded(t, postWithin(t, server, g))

	close(g.release)
	wg.Wait()

	// Slots are returned after the held requests finish.
	if recorder := postRecommendations(t, server, recBody); recorder.Code != http.StatusOK {
		t.Fatalf("after release: status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRecommendationsDefaultConcurrencyLimitIsFour(t *testing.T) {
	t.Parallel()

	if DefaultMaxConcurrentRecommendations != 4 {
		t.Fatalf("DefaultMaxConcurrentRecommendations = %d, want 4 (issue #298 既定案)", DefaultMaxConcurrentRecommendations)
	}
	for name, configured := range map[string]int{"zero value": 0, "negative": -1} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := newGatedCatalog(DefaultMaxConcurrentRecommendations)
			server := New(gatedDependencies(g, configured))
			wg := holdSlots(t, server, g, DefaultMaxConcurrentRecommendations)
			assertOverloaded(t, postWithin(t, server, g))
			close(g.release)
			wg.Wait()
		})
	}
}

// The slot is released when the request fails after acquiring it (a catalog error answers 500).
func TestRecommendationsReleasesTheSlotOnFailure(t *testing.T) {
	t.Parallel()

	deps := recFullDependencies()
	deps.PokemonCatalog = failingCatalog{err: http.ErrAbortHandler}
	deps.MaxConcurrentRecommendations = 1
	server := New(deps)
	for i := range 3 {
		if recorder := postRecommendations(t, server, recBody); recorder.Code != http.StatusInternalServerError {
			t.Fatalf("request %d: status = %d, want 500 (a leaked slot would answer 503); body=%s", i, recorder.Code, recorder.Body.String())
		}
	}
}

// Only the heavy computation is limited: while every slot is held, validation errors, other endpoints,
// health and metrics keep answering as before.
func TestRecommendationsLimitDoesNotAffectOtherRequests(t *testing.T) {
	t.Parallel()

	g := newGatedCatalog(1)
	server := New(gatedDependencies(g, 1))
	wg := holdSlots(t, server, g, 1)

	if recorder := postRecommendations(t, server, `{"members":[]}`); recorder.Code != http.StatusBadRequest {
		t.Errorf("invalid body while full: status = %d, want 400 (validation runs before the slot is taken)", recorder.Code)
	}
	for _, path := range []string{"/healthz", "/metrics"} {
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Errorf("GET %s while full: status = %d, want 200", path, recorder.Code)
		}
	}
	analyze := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/balance/v1/team-balance/analyze",
		strings.NewReader(`{"members":[{"pokemonId":"9001-000","moveIds":[]}]}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Device-Id", "11111111-1111-4111-8111-111111111111")
	request.Header.Set("X-Session-Id", "22222222-2222-4222-a222-222222222222")
	server.ServeHTTP(analyze, request)
	if analyze.Code == http.StatusServiceUnavailable && strings.Contains(analyze.Body.String(), string(api.Overloaded)) {
		t.Errorf("analyze answered overloaded while recommendations is full: %s", analyze.Body.String())
	}

	close(g.release)
	wg.Wait()
}
