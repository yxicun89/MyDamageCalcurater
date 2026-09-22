package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/services/speed/internal/api"
	"example.com/pokecalc/services/speed/internal/speed"
)

const pokemonPath = "/api/speed/v1/pokemon"

// fakeProvider は順不同の Roster を返す(API が pokemonId の昇順に並べることを確かめるため)。
type fakeProvider struct {
	roster speed.Roster
	err    error
}

func (f fakeProvider) Roster() (speed.Roster, error) {
	return f.roster, f.err
}

func unorderedRoster() speed.Roster {
	return speed.Roster{RegulationID: "example", Pokemon: []speed.Pokemon{
		{PokemonID: "9003-000", NameJa: "テストサンバンメ", Types: []string{"rock", "ground"}, BaseSpeed: 45},
		{PokemonID: "9001-000", NameJa: "テストカソウドリ", Types: []string{"fire", "flying"}, BaseSpeed: 100},
		{PokemonID: "9010-000", NameJa: "テストジュウバンメ", Types: []string{"ghost"}, BaseSpeed: 81},
		{PokemonID: "9002-000", NameJa: "テストニバンメ", Types: []string{"water"}, BaseSpeed: 81},
	}}
}

func newRequest(path string, headers map[string]string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	return request
}

var validHeaders = map[string]string{"X-Device-Id": "test-device", "X-Session-Id": "test-session"}

func serve(deps Dependencies, request *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	New(deps).ServeHTTP(recorder, request)
	return recorder
}

func decodeError(t *testing.T, recorder *httptest.ResponseRecorder) api.Error {
	t.Helper()
	var body api.Error
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not an Error: %v; body=%s", err, recorder.Body.String())
	}
	return body
}

func TestHealth(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"/healthz", "/api/speed/healthz"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			// read model が未設定でもヘルスは 200(ADR-0600 §4)。ヘッダーも不要。
			recorder := serve(Dependencies{}, newRequest(path, nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			if got := strings.TrimSpace(recorder.Body.String()); got != `{"status":"ok"}` {
				t.Errorf("body = %s, want {\"status\":\"ok\"}", got)
			}
		})
	}
}

func TestListPokemonRequiresRequestContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		headers map[string]string
	}{
		{"両方欠落", nil},
		{"X-Device-Id 欠落", map[string]string{"X-Session-Id": "test-session"}},
		{"X-Session-Id 欠落", map[string]string{"X-Device-Id": "test-device"}},
		{"X-Device-Id 空", map[string]string{"X-Device-Id": "", "X-Session-Id": "test-session"}},
		{"X-Session-Id 空", map[string]string{"X-Device-Id": "test-device", "X-Session-Id": ""}},
		{"X-Device-Id 空白だけ", map[string]string{"X-Device-Id": "  ", "X-Session-Id": "test-session"}},
		{"X-Session-Id 空白だけ", map[string]string{"X-Device-Id": "test-device", "X-Session-Id": "  "}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// ヘッダーの検査は read model の有無より先(provider があっても 400)。
			recorder := serve(Dependencies{Pokemon: fakeProvider{roster: unorderedRoster()}}, newRequest(pokemonPath, tt.headers))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
			if body := decodeError(t, recorder); body.Code != api.InvalidRequest {
				t.Errorf("code = %q, want %q", body.Code, api.InvalidRequest)
			}
		})
	}
}

// TestListPokemonNormalizesGeneratedParameterErrors: 生成コードが返すパラメータの 400(同じヘッダーの重複)も
// Error{code: invalid_request} の形にそろえる(balance と同じ)。
func TestListPokemonNormalizesGeneratedParameterErrors(t *testing.T) {
	t.Parallel()

	request := newRequest(pokemonPath, nil)
	request.Header.Add("X-Device-Id", "first-device")
	request.Header.Add("X-Device-Id", "second-device")
	request.Header.Set("X-Session-Id", "test-session")
	recorder := serve(Dependencies{Pokemon: fakeProvider{roster: unorderedRoster()}}, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if body := decodeError(t, recorder); body.Code != api.InvalidRequest {
		t.Errorf("code = %q, want %q", body.Code, api.InvalidRequest)
	}
}

func TestListPokemonWithoutReadModel(t *testing.T) {
	t.Parallel()

	recorder := serve(Dependencies{}, newRequest(pokemonPath, validHeaders))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
	}
	if body := decodeError(t, recorder); body.Code != api.MasterUnavailable {
		t.Errorf("code = %q, want %q", body.Code, api.MasterUnavailable)
	}
}

func TestListPokemonReturnsSortedByPokemonID(t *testing.T) {
	t.Parallel()

	recorder := serve(Dependencies{Pokemon: fakeProvider{roster: unorderedRoster()}}, newRequest(pokemonPath, validHeaders))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var body api.PokemonListResponse
	decoder := json.NewDecoder(strings.NewReader(recorder.Body.String()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		t.Fatalf("body is not a PokemonListResponse: %v; body=%s", err, recorder.Body.String())
	}
	want := api.PokemonListResponse{RegulationId: "example", Pokemon: []api.SpeedPokemon{
		{PokemonId: "9001-000", NameJa: "テストカソウドリ", Types: []string{"fire", "flying"}, BaseSpeed: 100},
		{PokemonId: "9002-000", NameJa: "テストニバンメ", Types: []string{"water"}, BaseSpeed: 81},
		{PokemonId: "9003-000", NameJa: "テストサンバンメ", Types: []string{"rock", "ground"}, BaseSpeed: 45},
		{PokemonId: "9010-000", NameJa: "テストジュウバンメ", Types: []string{"ghost"}, BaseSpeed: 81},
	}}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("body = %+v, want %+v", body, want)
	}
}

func TestListPokemonHidesProviderError(t *testing.T) {
	t.Parallel()

	secret := "open /secret/path/pokemon.json: permission denied"
	recorder := serve(Dependencies{Pokemon: fakeProvider{err: errors.New(secret)}}, newRequest(pokemonPath, validHeaders))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	body := decodeError(t, recorder)
	if body.Code != api.InternalError {
		t.Errorf("code = %q, want %q", body.Code, api.InternalError)
	}
	// 想定外のエラーは固定文言。内部の文言を返さない(ADR-0600 §5)。
	if body.Message != "internal error" {
		t.Errorf("message = %q, want the fixed string \"internal error\"", body.Message)
	}
	if strings.Contains(recorder.Body.String(), "secret") {
		t.Errorf("body leaks the internal error: %s", recorder.Body.String())
	}
}
