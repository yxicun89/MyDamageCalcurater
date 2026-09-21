package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/pokecalc/services/balance/internal/api"
	"example.com/pokecalc/services/balance/internal/balance"
	"example.com/pokecalc/services/balance/internal/master"
	"github.com/labstack/echo/v4"
)

const analyzePath = "/api/balance/v1/team-balance/analyze"

// testPokemonTypes is a fictional in-memory provider (IDs from 9001-000, ADR-0014).
type testPokemonTypes map[string][]balance.TypeID

func (p testPokemonTypes) PokemonTypes(pokemonID string) ([]balance.TypeID, error) {
	if types, ok := p[pokemonID]; ok {
		return append([]balance.TypeID(nil), types...), nil
	}
	return nil, fmt.Errorf("%w: %s", balance.ErrUnknownPokemon, pokemonID)
}

var fictionalPokemonTypes = testPokemonTypes{
	"9001-000": {balance.TypeFire, balance.TypeFlying},  // rock x4, ground x0, grass/bug x1/4
	"9002-000": {balance.TypeGrass},                     // fire x2, water x1/2
	"9003-000": {balance.TypeWater, balance.TypeGround}, // grass x4, electric x0
	"9004-000": {balance.TypeNormal, balance.TypeGhost},
	"9005-000": {balance.TypeIce},
	"9006-000": {balance.TypeSteel, balance.TypeFairy},
}

func newTestServer() *echo.Echo {
	return New(Dependencies{
		TypeChart:    testTypeChart(),
		PokemonTypes: fictionalPokemonTypes,
	})
}

func postAnalyze(t *testing.T, server http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, analyzePath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Device-Id", "test-device")
	request.Header.Set("X-Session-Id", "test-session")
	server.ServeHTTP(recorder, request)
	return recorder
}

func decodeAnalyzeResponse(t *testing.T, body []byte) api.AnalyzeResponse {
	t.Helper()
	var response api.AnalyzeResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("decode AnalyzeResponse: %v; body=%s", err, body)
	}
	return response
}

func decodeError(t *testing.T, body []byte) api.Error {
	t.Helper()
	var response api.Error
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("decode Error: %v; body=%s", err, body)
	}
	if response.Message == "" {
		t.Errorf("error message must not be empty; body=%s", body)
	}
	return response
}

var canonicalTypeIDs = []api.TypeId{
	api.Normal, api.Fire, api.Water, api.Electric, api.Grass, api.Ice,
	api.Fighting, api.Poison, api.Ground, api.Flying, api.Psychic, api.Bug,
	api.Rock, api.Ghost, api.Dragon, api.Dark, api.Steel, api.Fairy,
}

func TestAnalyzeResponseBody(t *testing.T) {
	t.Parallel()

	// Request order 9003 then 9001 must be kept in members.
	recorder := postAnalyze(t, newTestServer(), `{"members":[{"pokemonId":"9003-000"},{"pokemonId":"9001-000"}]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeAnalyzeResponse(t, recorder.Body.Bytes())

	if len(response.Members) != 2 {
		t.Fatalf("members length = %d, want 2", len(response.Members))
	}
	wantMembers := []struct {
		id    string
		types []api.TypeId
	}{
		{"9003-000", []api.TypeId{api.Water, api.Ground}},
		{"9001-000", []api.TypeId{api.Fire, api.Flying}},
	}
	for i, want := range wantMembers {
		member := response.Members[i]
		if member.PokemonId != want.id {
			t.Errorf("members[%d].pokemonId = %q, want %q", i, member.PokemonId, want.id)
		}
		if fmt.Sprint(member.Types) != fmt.Sprint(want.types) {
			t.Errorf("members[%d].types = %v, want %v", i, member.Types, want.types)
		}
		if len(member.Defense) != 18 {
			t.Fatalf("members[%d].defense length = %d, want 18", i, len(member.Defense))
		}
		for j, entry := range member.Defense {
			if entry.AttackType != canonicalTypeIDs[j] {
				t.Errorf("members[%d].defense[%d].attackType = %q, want %q", i, j, entry.AttackType, canonicalTypeIDs[j])
			}
			if entry.Source != api.Type {
				t.Errorf("members[%d].defense[%d].source = %q, want type", i, j, entry.Source)
			}
			if !entry.Multiplier.Valid() || !entry.Category.Valid() {
				t.Errorf("members[%d].defense[%d] has out-of-enum values: %+v", i, j, entry)
			}
		}
	}

	tests := []struct {
		name       string
		member     int
		attack     api.TypeId
		multiplier api.DefenseMultiplier
		category   api.DefenseCategory
	}{
		{"water/ground grass x4", 0, api.Grass, api.N4, api.QuadWeak},
		{"water/ground electric x0", 0, api.Electric, api.N0, api.Immune},
		{"water/ground rock x1/2", 0, api.Rock, api.N12, api.Resist},
		{"water/ground normal x1", 0, api.Normal, api.N1, api.Neutral},
		{"fire/flying rock x4", 1, api.Rock, api.N4, api.QuadWeak},
		{"fire/flying ground x0", 1, api.Ground, api.N0, api.Immune},
		{"fire/flying grass x1/4", 1, api.Grass, api.N14, api.QuadResist},
		{"fire/flying water x2", 1, api.Water, api.N2, api.Weak},
		{"fire/flying electric x2", 1, api.Electric, api.N2, api.Weak},
	}
	for _, tt := range tests {
		entry := findDefenseEntry(t, response.Members[tt.member].Defense, tt.attack)
		if entry.Multiplier != tt.multiplier || entry.Category != tt.category {
			t.Errorf("%s: got multiplier %q category %q, want %q %q", tt.name, entry.Multiplier, entry.Category, tt.multiplier, tt.category)
		}
	}

	if len(response.TeamSummary) != 18 {
		t.Fatalf("teamSummary length = %d, want 18", len(response.TeamSummary))
	}
	for i, entry := range response.TeamSummary {
		if entry.AttackType != canonicalTypeIDs[i] {
			t.Errorf("teamSummary[%d].attackType = %q, want %q", i, entry.AttackType, canonicalTypeIDs[i])
		}
		if entry.Weak+entry.Resist+entry.Immune+entry.Neutral != 2 {
			t.Errorf("teamSummary[%s]: weak+resist+immune+neutral = %d, want 2", entry.AttackType, entry.Weak+entry.Resist+entry.Immune+entry.Neutral)
		}
	}
	summaries := []api.TeamSummaryEntry{
		{AttackType: api.Rock, Weak: 1, QuadWeak: 1, Resist: 1},
		{AttackType: api.Electric, Weak: 1, Immune: 1},
		{AttackType: api.Grass, Weak: 1, QuadWeak: 1, Resist: 1},
		{AttackType: api.Ground, Immune: 1, Neutral: 1},
		{AttackType: api.Normal, Neutral: 2},
	}
	for _, want := range summaries {
		got := findSummaryEntry(t, response.TeamSummary, want.AttackType)
		if got != want {
			t.Errorf("teamSummary[%s] = %+v, want %+v", want.AttackType, got, want)
		}
	}
}

func TestAnalyzeResponseUsesContractFieldNames(t *testing.T) {
	t.Parallel()

	recorder := postAnalyze(t, newTestServer(), `{"members":[{"pokemonId":"9001-000"}]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	var raw struct {
		Members []map[string]json.RawMessage `json:"members"`
		Summary []map[string]json.RawMessage `json:"teamSummary"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(raw.Members) != 1 || len(raw.Summary) != 18 {
		t.Fatalf("members=%d teamSummary=%d, want 1 and 18; body=%s", len(raw.Members), len(raw.Summary), recorder.Body.String())
	}
	for _, key := range []string{"pokemonId", "types", "defense"} {
		if _, ok := raw.Members[0][key]; !ok {
			t.Errorf("member is missing %q", key)
		}
	}
	for _, key := range []string{"attackType", "weak", "quadWeak", "resist", "immune", "neutral"} {
		if _, ok := raw.Summary[0][key]; !ok {
			t.Errorf("teamSummary entry is missing %q", key)
		}
	}
	var defense []map[string]json.RawMessage
	if err := json.Unmarshal(raw.Members[0]["defense"], &defense); err != nil || len(defense) != 18 {
		t.Fatalf("defense = %s, err = %v", raw.Members[0]["defense"], err)
	}
	for _, key := range []string{"attackType", "multiplier", "category", "source"} {
		if _, ok := defense[0][key]; !ok {
			t.Errorf("defense entry is missing %q", key)
		}
	}
	// Multipliers are strings, never numbers or floats.
	if string(defense[0]["multiplier"]) != `"1"` {
		t.Errorf("normal vs fire/flying multiplier = %s, want \"1\"", defense[0]["multiplier"])
	}
}

func TestAnalyzeDuplicatedAndSixMembers(t *testing.T) {
	t.Parallel()

	body := `{"members":[{"pokemonId":"9001-000"},{"pokemonId":"9001-000"},{"pokemonId":"9002-000"},{"pokemonId":"9003-000"},{"pokemonId":"9004-000"},{"pokemonId":"9001-000"}]}`
	recorder := postAnalyze(t, newTestServer(), body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeAnalyzeResponse(t, recorder.Body.Bytes())
	if len(response.Members) != 6 {
		t.Fatalf("members length = %d, want 6 (duplicates kept)", len(response.Members))
	}
	wantIDs := []string{"9001-000", "9001-000", "9002-000", "9003-000", "9004-000", "9001-000"}
	for i, id := range wantIDs {
		if response.Members[i].PokemonId != id {
			t.Errorf("members[%d].pokemonId = %q, want %q", i, response.Members[i].PokemonId, id)
		}
	}
	for _, entry := range response.TeamSummary {
		if entry.Weak+entry.Resist+entry.Immune+entry.Neutral != 6 {
			t.Errorf("teamSummary[%s] does not add up to 6: %+v", entry.AttackType, entry)
		}
		if entry.QuadWeak > entry.Weak {
			t.Errorf("teamSummary[%s]: quadWeak > weak: %+v", entry.AttackType, entry)
		}
	}
	// rock: 9001 x4 three times, 9002 grass x1, 9003 water/ground x1/2, 9004 normal/ghost x1.
	if got := findSummaryEntry(t, response.TeamSummary, api.Rock); got != (api.TeamSummaryEntry{AttackType: api.Rock, Weak: 3, QuadWeak: 3, Resist: 1, Neutral: 2}) {
		t.Errorf("teamSummary[rock] = %+v", got)
	}
	// ground: 9001 x0 three times, 9002 x1/2, 9003 x1, 9004 x1.
	if got := findSummaryEntry(t, response.TeamSummary, api.Ground); got != (api.TeamSummaryEntry{AttackType: api.Ground, Resist: 1, Immune: 3, Neutral: 2}) {
		t.Errorf("teamSummary[ground] = %+v", got)
	}
	// normal/fighting/ghost immunities of 9004 are counted as immune, never as resist.
	for _, attack := range []api.TypeId{api.Normal, api.Fighting, api.Ghost} {
		got := findSummaryEntry(t, response.TeamSummary, attack)
		if got.Immune != 1 {
			t.Errorf("teamSummary[%s].immune = %d, want 1: %+v", attack, got.Immune, got)
		}
	}
}

func TestAnalyzeUnknownPokemon(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		wantInMsg string
	}{
		{name: "single unknown", body: `{"members":[{"pokemonId":"9999-000"}]}`, wantInMsg: "9999-000"},
		{name: "unknown after known", body: `{"members":[{"pokemonId":"9001-000"},{"pokemonId":"9001-001"}]}`, wantInMsg: "9001-001"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := postAnalyze(t, newTestServer(), tt.body)
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422; body=%s", recorder.Code, recorder.Body.String())
			}
			got := decodeError(t, recorder.Body.Bytes())
			if got.Code != api.UnknownPokemon {
				t.Errorf("code = %q, want unknown_pokemon", got.Code)
			}
			// ADR-0014 §5.6: the message must name the not-found pokemonId (the client's own input).
			if !strings.Contains(got.Message, tt.wantInMsg) {
				t.Errorf("message = %q, want it to contain %q", got.Message, tt.wantInMsg)
			}
		})
	}
}

// failingPokemonTypes is a fake PokemonTypeProvider that fails in a way other than
// an unknown pokemonId (e.g. the future shared master snapshot being unreadable),
// to exercise the 500 internal_error path (ADR-0014 §5.5).
type failingPokemonTypes struct{ err error }

func (f failingPokemonTypes) PokemonTypes(string) ([]balance.TypeID, error) { return nil, f.err }

func TestAnalyzeUnexpectedPokemonTypesFailureIsInternalError(t *testing.T) {
	t.Parallel()

	server := New(Dependencies{
		TypeChart:    testTypeChart(),
		PokemonTypes: failingPokemonTypes{err: errors.New("read model backend exploded")},
	})
	recorder := postAnalyze(t, server, `{"members":[{"pokemonId":"9001-000"}]}`)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", recorder.Code, recorder.Body.String())
	}
	got := decodeError(t, recorder.Body.Bytes())
	if got.Code != api.InternalError {
		t.Errorf("code = %q, want internal_error", got.Code)
	}
	if got.Message != "internal error" {
		t.Errorf("message = %q, want the fixed text %q (no internal detail)", got.Message, "internal error")
	}
}

func TestAnalyzeNilTypeChartIsInternalError(t *testing.T) {
	t.Parallel()

	server := New(Dependencies{PokemonTypes: fictionalPokemonTypes})
	recorder := postAnalyze(t, server, `{"members":[{"pokemonId":"9001-000"}]}`)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", recorder.Code, recorder.Body.String())
	}
	got := decodeError(t, recorder.Body.Bytes())
	if got.Code != api.InternalError {
		t.Errorf("code = %q, want internal_error", got.Code)
	}
	if got.Message != "internal error" {
		t.Errorf("message = %q, want the fixed text %q", got.Message, "internal error")
	}
}

func TestAnalyzeWithoutPokemonTypesIsMasterUnavailable(t *testing.T) {
	t.Parallel()

	server := New(Dependencies{TypeChart: testTypeChart()})

	recorder := postAnalyze(t, server, `{"members":[{"pokemonId":"9001-000"}]}`)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", recorder.Code, recorder.Body.String())
	}
	if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.MasterUnavailable {
		t.Errorf("code = %q, want master_unavailable", got.Code)
	}

	for _, path := range []string{"/healthz", "/api/balance/healthz"} {
		health := httptest.NewRecorder()
		server.ServeHTTP(health, httptest.NewRequest(http.MethodGet, path, nil))
		if health.Code != http.StatusOK {
			t.Errorf("%s status = %d, want 200 even without the read model", path, health.Code)
		}
	}
}

func TestAnalyzeWithoutPokemonTypesStillRequiresRequestContext(t *testing.T) {
	t.Parallel()

	server := New(Dependencies{TypeChart: testTypeChart()})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, analyzePath, strings.NewReader(`{"members":[{"pokemonId":"9001-000"}]}`))
	request.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"missing_request_context"`) {
		t.Fatalf("status = %d body=%s, want 400 missing_request_context", recorder.Code, recorder.Body.String())
	}
}

func TestAnalyzeWithExampleReadModel(t *testing.T) {
	t.Parallel()

	model, err := master.LoadPokemonTypesFile("../../testdata/pokemon-types.example.json")
	if err != nil || model == nil {
		t.Fatalf("LoadPokemonTypesFile(example) = %v, %v", model, err)
	}
	server := New(Dependencies{TypeChart: testTypeChart(), PokemonTypes: model})
	recorder := postAnalyze(t, server, `{"members":[{"pokemonId":"9001-000"},{"pokemonId":"9002-000"}]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeAnalyzeResponse(t, recorder.Body.Bytes())
	if len(response.Members) != 2 || len(response.TeamSummary) != 18 {
		t.Fatalf("members=%d teamSummary=%d, want 2 and 18", len(response.Members), len(response.TeamSummary))
	}
}

func findDefenseEntry(t *testing.T, entries []api.DefenseEntry, attack api.TypeId) api.DefenseEntry {
	t.Helper()
	for _, entry := range entries {
		if entry.AttackType == attack {
			return entry
		}
	}
	t.Fatalf("attack type %q not found", attack)
	return api.DefenseEntry{}
}

func findSummaryEntry(t *testing.T, entries []api.TeamSummaryEntry, attack api.TypeId) api.TeamSummaryEntry {
	t.Helper()
	for _, entry := range entries {
		if entry.AttackType == attack {
			return entry
		}
	}
	t.Fatalf("attack type %q not found in teamSummary", attack)
	return api.TeamSummaryEntry{}
}

// ADR-0014 §5.4: 不正な request には provider の有無によらず 400/413 を返す(503 より先に判定する)。
func TestAnalyzeWithoutPokemonTypesValidatesBodyFirst(t *testing.T) {
	t.Parallel()

	server := New(Dependencies{TypeChart: testTypeChart()})
	seven := `{"members":[{"pokemonId":"9001-000"},{"pokemonId":"9001-000"},{"pokemonId":"9001-000"},{"pokemonId":"9001-000"},{"pokemonId":"9001-000"},{"pokemonId":"9001-000"},{"pokemonId":"9001-000"}]}`
	tests := []struct {
		name     string
		body     string
		wantCode int
		wantErr  api.ErrorCode
	}{
		{name: "empty party", body: `{"members":[]}`, wantCode: http.StatusBadRequest, wantErr: api.InvalidRequest},
		{name: "seven members", body: seven, wantCode: http.StatusBadRequest, wantErr: api.InvalidRequest},
		{name: "malformed pokemonId", body: `{"members":[{"pokemonId":"pokemon-a"}]}`, wantCode: http.StatusBadRequest, wantErr: api.InvalidRequest},
		{name: "broken JSON", body: `{"members":`, wantCode: http.StatusBadRequest, wantErr: api.InvalidRequest},
		{name: "oversized body", body: `{"members":[{"pokemonId":"` + strings.Repeat("1", maxAnalyzeBodyBytes) + `"}]}`, wantCode: http.StatusRequestEntityTooLarge, wantErr: api.RequestTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := postAnalyze(t, server, tt.body)
			if recorder.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d (not 503); body=%s", recorder.Code, tt.wantCode, recorder.Body.String())
			}
			if got := decodeError(t, recorder.Body.Bytes()); got.Code != tt.wantErr {
				t.Errorf("code = %q, want %q", got.Code, tt.wantErr)
			}
		})
	}
}

// ADR-0014 §5.6: 422 の message は ID から組み立て、adapter のエラー文言(内部情報を含みうる)を返さない。
func TestAnalyzeUnknownPokemonMessageDoesNotLeakAdapterDetail(t *testing.T) {
	t.Parallel()

	server := New(Dependencies{
		TypeChart:    testTypeChart(),
		PokemonTypes: failingPokemonTypes{err: fmt.Errorf("%w: 9999-000 (read from /secret/path.json)", balance.ErrUnknownPokemon)},
	})
	recorder := postAnalyze(t, server, `{"members":[{"pokemonId":"9999-000"}]}`)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body=%s", recorder.Code, recorder.Body.String())
	}
	got := decodeError(t, recorder.Body.Bytes())
	if got.Message != "unknown pokemonId: 9999-000" {
		t.Errorf("message = %q, want %q", got.Message, "unknown pokemonId: 9999-000")
	}
}

// testTypeChart は同梱の相性表(P1-13 のデータ)を返す。読めないのはテスト環境の不備なので panic する。
func testTypeChart() *master.TypeChart {
	chart, err := master.EmbeddedTypeChart()
	if err != nil {
		panic(err)
	}
	return chart
}
