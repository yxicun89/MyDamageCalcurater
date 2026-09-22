package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/pokecalc/services/balance/internal/api"
	"example.com/pokecalc/services/balance/internal/master"
)

// A typed nil provider (an interface value whose concrete type is a nil pointer -- e.g. a
// caller that wires up `var m *master.PokemonTypeReadModel` from a config step that never
// assigned it) must be normalized to an unset (nil interface) dependency by New. Without
// that normalization the field compares non-nil (Dependencies.X != nil) and the handler
// calls straight into the nil receiver's method, which panics. Each provider is exercised
// through the endpoint that actually needs it, with every other required dependency
// present, so the only difference from a 200 is the typed-nil one.
func TestNewNormalizesTypedNilProviders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		deps     Dependencies
		path     string
		body     string
		wantCode int
		wantErr  api.ErrorCode
	}{
		{
			name:     "typed nil type chart",
			deps:     Dependencies{TypeChart: (*master.TypeChart)(nil), PokemonTypes: fictionalPokemonTypes},
			path:     analyzePath,
			body:     `{"members":[{"pokemonId":"9001-000"}]}`,
			wantCode: http.StatusInternalServerError,
			wantErr:  api.InternalError,
		},
		{
			name:     "typed nil pokemon types",
			deps:     Dependencies{TypeChart: testTypeChart(), PokemonTypes: (*master.PokemonTypeReadModel)(nil)},
			path:     analyzePath,
			body:     `{"members":[{"pokemonId":"9001-000"}]}`,
			wantCode: http.StatusServiceUnavailable,
			wantErr:  api.MasterUnavailable,
		},
		{
			name:     "typed nil moves",
			deps:     Dependencies{TypeChart: testTypeChart(), PokemonTypes: fictionalPokemonTypes, Moves: (*master.MoveReadModel)(nil)},
			path:     coveragePath,
			body:     `{"members":[{"pokemonId":"9001-000","moveIds":[]}]}`,
			wantCode: http.StatusServiceUnavailable,
			wantErr:  api.MasterUnavailable,
		},
		{
			name:     "typed nil abilities",
			deps:     Dependencies{TypeChart: testTypeChart(), PokemonTypes: fictionalPokemonTypes, Abilities: (*master.AbilityReadModel)(nil)},
			path:     analyzePath,
			body:     `{"members":[{"pokemonId":"9001-000","abilityId":"ability-9001"}]}`,
			wantCode: http.StatusServiceUnavailable,
			wantErr:  api.MasterUnavailable,
		},
		{
			name:     "typed nil pokemon catalog",
			deps:     Dependencies{TypeChart: testTypeChart(), PokemonTypes: fictionalPokemonTypes, PokemonCatalog: (*master.PokemonTypeReadModel)(nil)},
			path:     recommendationsPath,
			body:     `{"members":[{"pokemonId":"9001-000","moveIds":[]}]}`,
			wantCode: http.StatusServiceUnavailable,
			wantErr:  api.MasterUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := New(tt.deps)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Device-Id", "test-device")
			request.Header.Set("X-Session-Id", "test-session")

			server.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.wantCode, recorder.Body.String())
			}
			if got := decodeError(t, recorder.Body.Bytes()); got.Code != tt.wantErr {
				t.Errorf("code = %q, want %q", got.Code, tt.wantErr)
			}
		})
	}
}

// A nil slice/map provider is a usable (empty) value in Go, so it is not normalized away:
// a nil testCatalog is an empty catalog (200), not a missing one (503).
func TestNewKeepsNilSliceProviderAsEmpty(t *testing.T) {
	t.Parallel()

	deps := recFullDependencies()
	deps.PokemonCatalog = testCatalog(nil)
	recorder := postRecommendations(t, New(deps), recBody)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for an empty (nil slice) catalog; body=%s", recorder.Code, recorder.Body.String())
	}
}
