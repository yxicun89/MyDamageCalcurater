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

// TB2 攻撃範囲の HTTP 契約(ADR-0016)。技 ID は架空(move-9001 以降)。

const coveragePath = "/api/balance/v1/team-balance/coverage"

// fortyCharMoveID is a valid fictional moveId of exactly the 40-character limit.
var fortyCharMoveID = "move-9099-" + strings.Repeat("a", 30)

// testMoves is a fictional in-memory MoveProvider.
type testMoves map[string]balance.Move

func (p testMoves) Move(moveID string) (balance.Move, error) {
	if move, ok := p[moveID]; ok {
		return move, nil
	}
	return balance.Move{}, fmt.Errorf("%w: %s", balance.ErrUnknownMove, moveID)
}

func fictionalMove(id string, t balance.TypeID, c balance.MoveCategory) balance.Move {
	return balance.Move{MoveID: id, Type: t, Category: c}
}

var fictionalMoves = testMoves{
	"move-9001":     fictionalMove("move-9001", balance.TypeFire, balance.MoveCategorySpecial),
	"move-9002":     fictionalMove("move-9002", balance.TypeFire, balance.MoveCategoryPhysical),
	"move-9003":     fictionalMove("move-9003", balance.TypeWater, balance.MoveCategorySpecial),
	"move-9004":     fictionalMove("move-9004", balance.TypeElectric, balance.MoveCategorySpecial),
	"move-9005":     fictionalMove("move-9005", balance.TypeNormal, balance.MoveCategoryPhysical),
	"move-9006":     fictionalMove("move-9006", balance.TypeGrass, balance.MoveCategoryStatus),
	"move-9007":     fictionalMove("move-9007", balance.TypeGround, balance.MoveCategoryPhysical),
	fortyCharMoveID: fictionalMove(fortyCharMoveID, balance.TypeIce, balance.MoveCategorySpecial),
}

// failingMoves fails in a way other than an unknown moveId (500 path), or with a
// wrapped ErrUnknownMove carrying adapter detail (422 message path).
type failingMoves struct{ err error }

func (f failingMoves) Move(string) (balance.Move, error) { return balance.Move{}, f.err }

func newCoverageServer() *echo.Echo {
	return New(Dependencies{
		TypeChart:    testTypeChart(),
		PokemonTypes: fictionalPokemonTypes,
		Moves:        fictionalMoves,
	})
}

func postCoverage(t *testing.T, server http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, coveragePath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Device-Id", "test-device")
	request.Header.Set("X-Session-Id", "test-session")
	server.ServeHTTP(recorder, request)
	return recorder
}

func decodeCoverageResponse(t *testing.T, body []byte) api.CoverageResponse {
	t.Helper()
	var response api.CoverageResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("decode CoverageResponse: %v; body=%s", err, body)
	}
	if len(response.TeamCoverage) != 18 {
		t.Fatalf("teamCoverage length = %d, want 18; body=%s", len(response.TeamCoverage), body)
	}
	for i, entry := range response.TeamCoverage {
		if entry.DefenseType != canonicalTypeIDs[i] {
			t.Errorf("teamCoverage[%d].defenseType = %q, want %q", i, entry.DefenseType, canonicalTypeIDs[i])
		}
		if entry.BestMultiplier != nil && !entry.BestMultiplier.Valid() {
			t.Errorf("teamCoverage[%d].bestMultiplier = %q is out of enum", i, *entry.BestMultiplier)
		}
	}
	for mi, member := range response.Members {
		if len(member.Coverage) != 18 {
			t.Fatalf("members[%d].coverage length = %d, want 18", mi, len(member.Coverage))
		}
		for i, entry := range member.Coverage {
			if entry.DefenseType != canonicalTypeIDs[i] {
				t.Errorf("members[%d].coverage[%d].defenseType = %q, want %q", mi, i, entry.DefenseType, canonicalTypeIDs[i])
			}
			if entry.BestMultiplier != nil && !entry.BestMultiplier.Valid() {
				t.Errorf("members[%d].coverage[%d].bestMultiplier = %q is out of enum", mi, i, *entry.BestMultiplier)
			}
		}
	}
	return response
}

func coverageLabel(m *api.CoverageMultiplier) string {
	if m == nil {
		return "null"
	}
	return string(*m)
}

func findCoverageEntry(t *testing.T, entries []api.DefenseCoverageEntry, defense api.TypeId) api.DefenseCoverageEntry {
	t.Helper()
	for _, entry := range entries {
		if entry.DefenseType == defense {
			return entry
		}
	}
	t.Fatalf("defense type %q not found", defense)
	return api.DefenseCoverageEntry{}
}

func findTeamCoverageEntry(t *testing.T, entries []api.TeamCoverageEntry, defense api.TypeId) api.TeamCoverageEntry {
	t.Helper()
	for _, entry := range entries {
		if entry.DefenseType == defense {
			return entry
		}
	}
	t.Fatalf("defense type %q not found in teamCoverage", defense)
	return api.TeamCoverageEntry{}
}

func TestCoverageResponseBody(t *testing.T) {
	t.Parallel()

	body := `{"members":[
		{"pokemonId":"9003-000","moveIds":["move-9004","move-9002","move-9001","move-9006"]},
		{"pokemonId":"9002-000","moveIds":[]},
		{"pokemonId":"9001-000","moveIds":["move-9005"]}
	]}`
	recorder := postCoverage(t, newCoverageServer(), body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/json") {
		t.Errorf("Content-Type = %q, want application/json", recorder.Header().Get("Content-Type"))
	}
	response := decodeCoverageResponse(t, recorder.Body.Bytes())
	if len(response.Members) != 3 {
		t.Fatalf("members length = %d, want 3", len(response.Members))
	}

	wantMembers := []struct {
		id          string
		moveIDs     []string
		attackTypes []api.TypeId
	}{
		// electric + fire (twice) + grass status → [fire electric] in canonical order, deduplicated.
		{"9003-000", []string{"move-9004", "move-9002", "move-9001", "move-9006"}, []api.TypeId{api.Fire, api.Electric}},
		{"9002-000", []string{}, []api.TypeId{}},
		{"9001-000", []string{"move-9005"}, []api.TypeId{api.Normal}},
	}
	for i, want := range wantMembers {
		member := response.Members[i]
		if member.PokemonId != want.id {
			t.Errorf("members[%d].pokemonId = %q, want %q", i, member.PokemonId, want.id)
		}
		if fmt.Sprint(member.MoveIds) != fmt.Sprint(want.moveIDs) {
			t.Errorf("members[%d].moveIds = %v, want %v (request order)", i, member.MoveIds, want.moveIDs)
		}
		if fmt.Sprint(member.AttackTypes) != fmt.Sprint(want.attackTypes) {
			t.Errorf("members[%d].attackTypes = %v, want %v", i, member.AttackTypes, want.attackTypes)
		}
	}

	memberTests := []struct {
		name      string
		member    int
		defense   api.TypeId
		best      string
		effective bool
		super     bool
	}{
		{"fire/electric vs water x2", 0, api.Water, "2", true, true},
		{"fire/electric vs grass x2 (grass status ignored)", 0, api.Grass, "2", true, true},
		{"fire/electric vs ground x1 (electric x0)", 0, api.Ground, "1", true, false},
		{"fire/electric vs dragon x1/2", 0, api.Dragon, "1/2", false, false},
		{"no moves vs normal", 1, api.Normal, "null", false, false},
		{"no moves vs fire", 1, api.Fire, "null", false, false},
		{"normal vs ghost x0", 2, api.Ghost, "0", false, false},
		{"normal vs rock x1/2", 2, api.Rock, "1/2", false, false},
		{"normal vs normal x1", 2, api.Normal, "1", true, false},
	}
	for _, tt := range memberTests {
		entry := findCoverageEntry(t, response.Members[tt.member].Coverage, tt.defense)
		if coverageLabel(entry.BestMultiplier) != tt.best || entry.Effective != tt.effective || entry.SuperEffective != tt.super {
			t.Errorf("%s: got best %s effective %v super %v, want %s %v %v", tt.name,
				coverageLabel(entry.BestMultiplier), entry.Effective, entry.SuperEffective, tt.best, tt.effective, tt.super)
		}
	}
	for _, entry := range response.Members[1].Coverage {
		if entry.BestMultiplier != nil || entry.Effective || entry.SuperEffective {
			t.Errorf("member without moves vs %s = %s %v %v, want null false false", entry.DefenseType,
				coverageLabel(entry.BestMultiplier), entry.Effective, entry.SuperEffective)
		}
	}

	teamTests := []struct {
		defense   api.TypeId
		best      string
		effective int
		super     int
	}{
		{api.Water, "2", 2, 1},  // fire/electric x2, normal x1
		{api.Dragon, "1", 1, 0}, // fire/electric x1/2, normal x1
		{api.Ghost, "1", 1, 0},  // fire/electric x1, normal x0
		{api.Steel, "2", 1, 1},  // fire x2, normal x1/2
		{api.Normal, "1", 2, 0},
		{api.Rock, "1", 1, 0}, // electric x1, normal x1/2
	}
	for _, tt := range teamTests {
		entry := findTeamCoverageEntry(t, response.TeamCoverage, tt.defense)
		if coverageLabel(entry.BestMultiplier) != tt.best || entry.EffectiveMembers != tt.effective || entry.SuperEffectiveMembers != tt.super {
			t.Errorf("teamCoverage[%s] = best %s effective %d super %d, want %s %d %d", tt.defense,
				coverageLabel(entry.BestMultiplier), entry.EffectiveMembers, entry.SuperEffectiveMembers, tt.best, tt.effective, tt.super)
		}
	}
}

// The JSON must use the contract names, arrays (never null) for moveIds/attackTypes,
// strings for multipliers, and an explicit null bestMultiplier when there is no attack move.
func TestCoverageResponseUsesContractFieldNames(t *testing.T) {
	t.Parallel()

	recorder := postCoverage(t, newCoverageServer(), `{"members":[{"pokemonId":"9002-000","moveIds":[]},{"pokemonId":"9001-000","moveIds":["move-9005"]}]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	var raw struct {
		Members []map[string]json.RawMessage `json:"members"`
		Team    []map[string]json.RawMessage `json:"teamCoverage"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(raw.Members) != 2 || len(raw.Team) != 18 {
		t.Fatalf("members=%d teamCoverage=%d, want 2 and 18; body=%s", len(raw.Members), len(raw.Team), recorder.Body.String())
	}
	for _, key := range []string{"pokemonId", "moveIds", "attackTypes", "coverage"} {
		if _, ok := raw.Members[0][key]; !ok {
			t.Errorf("member is missing %q", key)
		}
	}
	for _, key := range []string{"moveIds", "attackTypes"} {
		if got := string(raw.Members[0][key]); got != "[]" {
			t.Errorf("members[0].%s = %s, want [] (an array, not null)", key, got)
		}
	}
	for _, key := range []string{"defenseType", "bestMultiplier", "effectiveMembers", "superEffectiveMembers"} {
		if _, ok := raw.Team[0][key]; !ok {
			t.Errorf("teamCoverage entry is missing %q", key)
		}
	}

	var empty, normal []map[string]json.RawMessage
	if err := json.Unmarshal(raw.Members[0]["coverage"], &empty); err != nil || len(empty) != 18 {
		t.Fatalf("members[0].coverage = %s, err = %v", raw.Members[0]["coverage"], err)
	}
	if err := json.Unmarshal(raw.Members[1]["coverage"], &normal); err != nil || len(normal) != 18 {
		t.Fatalf("members[1].coverage = %s, err = %v", raw.Members[1]["coverage"], err)
	}
	for _, key := range []string{"defenseType", "bestMultiplier", "effective", "superEffective"} {
		if _, ok := empty[0][key]; !ok {
			t.Errorf("coverage entry is missing %q", key)
		}
	}
	if got := string(empty[0]["bestMultiplier"]); got != "null" {
		t.Errorf("member without moves: bestMultiplier = %s, want null", got)
	}
	if got := string(empty[0]["effective"]); got != "false" {
		t.Errorf("member without moves: effective = %s, want false", got)
	}
	// Multipliers are strings, never numbers. normal vs normal (index 0) is x1, vs ghost (index 13) x0.
	if got := string(normal[0]["bestMultiplier"]); got != `"1"` {
		t.Errorf("normal vs normal bestMultiplier = %s, want \"1\"", got)
	}
	if got := string(normal[13]["bestMultiplier"]); got != `"0"` {
		t.Errorf("normal vs ghost bestMultiplier = %s, want \"0\"", got)
	}
	if got := string(raw.Team[13]["bestMultiplier"]); got != `"0"` {
		t.Errorf("teamCoverage[ghost].bestMultiplier = %s, want \"0\"", got)
	}
}

func TestCoverageTeamWithoutAttackMoves(t *testing.T) {
	t.Parallel()

	recorder := postCoverage(t, newCoverageServer(), `{"members":[{"pokemonId":"9001-000","moveIds":["move-9006"]},{"pokemonId":"9002-000","moveIds":[]}]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeCoverageResponse(t, recorder.Body.Bytes())
	if len(response.Members[0].AttackTypes) != 0 {
		t.Errorf("status-only member attackTypes = %v, want none", response.Members[0].AttackTypes)
	}
	if fmt.Sprint(response.Members[0].MoveIds) != "[move-9006]" {
		t.Errorf("status-only member moveIds = %v, want [move-9006] (status moves are still echoed)", response.Members[0].MoveIds)
	}
	for _, entry := range response.TeamCoverage {
		if entry.BestMultiplier != nil || entry.EffectiveMembers != 0 || entry.SuperEffectiveMembers != 0 {
			t.Errorf("teamCoverage[%s] = best %s effective %d super %d, want null 0 0", entry.DefenseType,
				coverageLabel(entry.BestMultiplier), entry.EffectiveMembers, entry.SuperEffectiveMembers)
		}
	}
}

// Team counts are per member: four fire moves on one member still count once.
func TestCoverageDoesNotDoubleCountSameTypeMoves(t *testing.T) {
	t.Parallel()

	recorder := postCoverage(t, newCoverageServer(), `{"members":[{"pokemonId":"9001-000","moveIds":["move-9001","move-9002"]},{"pokemonId":"9002-000","moveIds":["move-9003"]}]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeCoverageResponse(t, recorder.Body.Bytes())
	if fmt.Sprint(response.Members[0].AttackTypes) != fmt.Sprint([]api.TypeId{api.Fire}) {
		t.Errorf("attackTypes = %v, want [fire] once", response.Members[0].AttackTypes)
	}
	for _, tt := range []struct {
		defense   api.TypeId
		effective int
		super     int
	}{{api.Normal, 2, 0}, {api.Grass, 1, 1}, {api.Fire, 1, 1}, {api.Water, 0, 0}} {
		entry := findTeamCoverageEntry(t, response.TeamCoverage, tt.defense)
		if entry.EffectiveMembers != tt.effective || entry.SuperEffectiveMembers != tt.super {
			t.Errorf("teamCoverage[%s] = effective %d super %d, want %d %d", tt.defense, entry.EffectiveMembers, entry.SuperEffectiveMembers, tt.effective, tt.super)
		}
	}
}

func TestCoverageAcceptsBoundaries(t *testing.T) {
	t.Parallel()

	fourMoves := `["move-9001","move-9003","move-9004","move-9005"]`
	sixMembers := `{"members":[` + strings.TrimSuffix(strings.Repeat(`{"pokemonId":"9001-000","moveIds":`+fourMoves+`},`, 6), ",") + `]}`
	tests := []struct {
		name    string
		body    string
		members int
	}{
		{name: "six members with four moves each (duplicated pokemonId and moves across members)", body: sixMembers, members: 6},
		{name: "forty-character moveId", body: `{"members":[{"pokemonId":"9001-000","moveIds":["` + fortyCharMoveID + `"]}]}`, members: 1},
		{name: "one member without moves", body: `{"members":[{"pokemonId":"9001-000","moveIds":[]}]}`, members: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := postCoverage(t, newCoverageServer(), tt.body)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
			}
			if got := decodeCoverageResponse(t, recorder.Body.Bytes()); len(got.Members) != tt.members {
				t.Errorf("members length = %d, want %d", len(got.Members), tt.members)
			}
		})
	}
}

func TestCoverageRejectsInvalidBody(t *testing.T) {
	t.Parallel()

	member := func(moveIDs string) string {
		return `{"members":[{"pokemonId":"9001-000","moveIds":` + moveIDs + `}]}`
	}
	seven := `{"members":[` + strings.TrimSuffix(strings.Repeat(`{"pokemonId":"9001-000","moveIds":[]},`, 7), ",") + `]}`
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"members":`},
		{name: "trailing JSON", body: member(`["move-9001"]`) + ` {}`},
		{name: "not an object", body: `[]`},
		{name: "empty party", body: `{"members":[]}`},
		{name: "missing members", body: `{}`},
		{name: "seven members", body: seven},
		{name: "malformed pokemonId", body: `{"members":[{"pokemonId":"pokemon-a","moveIds":[]}]}`},
		{name: "missing pokemonId", body: `{"members":[{"moveIds":["move-9001"]}]}`},
		{name: "five moveIds", body: member(`["move-9001","move-9003","move-9004","move-9005","move-9007"]`)},
		{name: "duplicate moveId within a member", body: member(`["move-9001","move-9001"]`)},
		{name: "duplicate moveId after others", body: member(`["move-9001","move-9003","move-9001"]`)},
		{name: "empty moveId", body: member(`[""]`)},
		{name: "uppercase moveId", body: member(`["Move-9001"]`)},
		{name: "underscore in moveId", body: member(`["move_9001"]`)},
		{name: "space in moveId", body: member(`["move 9001"]`)},
		{name: "leading hyphen", body: member(`["-move-9001"]`)},
		{name: "trailing hyphen", body: member(`["move-9001-"]`)},
		{name: "double hyphen", body: member(`["move--9001"]`)},
		{name: "forty-one-character moveId", body: member(`["` + fortyCharMoveID + `a"]`)},
		{name: "moveIds is a string", body: member(`"move-9001"`)},
		{name: "moveId is a number", body: member(`[9001]`)},
		{name: "unknown member property", body: `{"members":[{"pokemonId":"9001-000","moveIds":[],"types":["fire"]}]}`},
		{name: "unknown top-level property", body: `{"members":[{"pokemonId":"9001-000","moveIds":[]}],"format":"single"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := postCoverage(t, newCoverageServer(), tt.body)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
			}
			if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.InvalidRequest {
				t.Errorf("code = %q, want invalid_request", got.Code)
			}
		})
	}
}

func TestCoverageRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	body := `{"members":[{"pokemonId":"9001-000","moveIds":["` + strings.Repeat("a", maxAnalyzeBodyBytes) + `"]}]}`
	recorder := postCoverage(t, newCoverageServer(), body)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413; body=%s", recorder.Code, recorder.Body.String())
	}
	if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.RequestTooLarge {
		t.Errorf("code = %q, want request_too_large", got.Code)
	}
}

func TestCoverageRequiresRequestContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		deviceID  string
		sessionID string
	}{
		{name: "missing both"},
		{name: "missing device", sessionID: "test-session"},
		{name: "missing session", deviceID: "test-device"},
		{name: "blank device", deviceID: "   ", sessionID: "test-session"},
		{name: "blank session", deviceID: "test-device", sessionID: "  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, coveragePath, strings.NewReader(`{"members":[{"pokemonId":"9001-000","moveIds":["move-9001"]}]}`))
			request.Header.Set("Content-Type", "application/json")
			if tt.deviceID != "" {
				request.Header.Set("X-Device-Id", tt.deviceID)
			}
			if tt.sessionID != "" {
				request.Header.Set("X-Session-Id", tt.sessionID)
			}
			newCoverageServer().ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
			}
			if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.MissingRequestContext {
				t.Errorf("code = %q, want missing_request_context", got.Code)
			}
		})
	}
}

// ADR-0016 §4: header (400) → body (400/413) → either read model absent (503) →
// unknown pokemonId (422) → unknown moveId (422) → 200.
func TestCoverageDecisionOrder(t *testing.T) {
	t.Parallel()

	full := Dependencies{TypeChart: testTypeChart(), PokemonTypes: fictionalPokemonTypes, Moves: fictionalMoves}
	noPokemon := Dependencies{TypeChart: testTypeChart(), Moves: fictionalMoves}
	noMoves := Dependencies{TypeChart: testTypeChart(), PokemonTypes: fictionalPokemonTypes}
	none := Dependencies{TypeChart: testTypeChart()}

	const (
		valid          = `{"members":[{"pokemonId":"9001-000","moveIds":["move-9001"]}]}`
		fiveMoves      = `{"members":[{"pokemonId":"9001-000","moveIds":["move-9001","move-9003","move-9004","move-9005","move-9007"]}]}`
		unknownBoth    = `{"members":[{"pokemonId":"9001-000","moveIds":["move-9999"]},{"pokemonId":"9999-000","moveIds":["move-9001"]}]}`
		unknownMove    = `{"members":[{"pokemonId":"9001-000","moveIds":["move-9001"]},{"pokemonId":"9002-000","moveIds":["move-9003","move-9998"]}]}`
		unknownPokemon = `{"members":[{"pokemonId":"9999-000","moveIds":["move-9001"]}]}`
	)
	oversized := `{"members":[{"pokemonId":"9001-000","moveIds":["` + strings.Repeat("a", maxAnalyzeBodyBytes) + `"]}]}`
	tests := []struct {
		name        string
		deps        Dependencies
		noHeaders   bool
		body        string
		wantStatus  int
		wantCode    api.ErrorCode
		wantMessage string
	}{
		{name: "headers before body and providers", deps: none, noHeaders: true, body: fiveMoves, wantStatus: http.StatusBadRequest, wantCode: api.MissingRequestContext},
		{name: "invalid body before missing read models", deps: none, body: fiveMoves, wantStatus: http.StatusBadRequest, wantCode: api.InvalidRequest},
		{name: "oversized body before missing read models", deps: none, body: oversized, wantStatus: http.StatusRequestEntityTooLarge, wantCode: api.RequestTooLarge},
		{name: "invalid body with only the move read model missing", deps: noMoves, body: fiveMoves, wantStatus: http.StatusBadRequest, wantCode: api.InvalidRequest},
		{name: "both read models missing", deps: none, body: valid, wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},
		{name: "pokemon read model missing", deps: noPokemon, body: valid, wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},
		{name: "move read model missing", deps: noMoves, body: valid, wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},
		{name: "missing move read model before unknown pokemon", deps: noMoves, body: unknownPokemon, wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},
		{name: "missing pokemon read model before unknown move", deps: noPokemon, body: unknownMove, wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},
		{name: "unknown pokemon before unknown move", deps: full, body: unknownBoth, wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownPokemon, wantMessage: "unknown pokemonId: 9999-000"},
		{name: "unknown move", deps: full, body: unknownMove, wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownMove, wantMessage: "unknown moveId: move-9998"},
		{name: "unknown move as the only move", deps: full, body: `{"members":[{"pokemonId":"9001-000","moveIds":["move-9999"]}]}`, wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownMove, wantMessage: "unknown moveId: move-9999"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, coveragePath, strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			if !tt.noHeaders {
				request.Header.Set("X-Device-Id", "test-device")
				request.Header.Set("X-Session-Id", "test-session")
			}
			New(tt.deps).ServeHTTP(recorder, request)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			got := decodeError(t, recorder.Body.Bytes())
			if got.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", got.Code, tt.wantCode)
			}
			if tt.wantMessage != "" && got.Message != tt.wantMessage {
				t.Errorf("message = %q, want %q", got.Message, tt.wantMessage)
			}
		})
	}

	recorder := postCoverage(t, New(full), valid)
	if recorder.Code != http.StatusOK {
		t.Fatalf("all checks passed: status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
}

// Health stays 200 and analyze is unaffected when only the move read model is missing.
func TestCoverageMissingMovesDoesNotAffectHealthOrAnalyze(t *testing.T) {
	t.Parallel()

	server := New(Dependencies{TypeChart: testTypeChart(), PokemonTypes: fictionalPokemonTypes})
	for _, path := range []string{"/healthz", "/api/balance/healthz"} {
		health := httptest.NewRecorder()
		server.ServeHTTP(health, httptest.NewRequest(http.MethodGet, path, nil))
		if health.Code != http.StatusOK {
			t.Errorf("%s status = %d, want 200 without the move read model", path, health.Code)
		}
	}
	if recorder := postAnalyze(t, server, `{"members":[{"pokemonId":"9001-000"}]}`); recorder.Code != http.StatusOK {
		t.Errorf("analyze status = %d, want 200 without the move read model; body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder := postCoverage(t, server, `{"members":[{"pokemonId":"9001-000","moveIds":["move-9001"]}]}`); recorder.Code != http.StatusServiceUnavailable {
		t.Errorf("coverage status = %d, want 503; body=%s", recorder.Code, recorder.Body.String())
	}
}

// ADR-0016 §4 / ADR-0014 §5.6: the 422 message is built from the ID, never from adapter detail.
func TestCoverageUnknownMoveMessageDoesNotLeakAdapterDetail(t *testing.T) {
	t.Parallel()

	server := New(Dependencies{
		TypeChart:    testTypeChart(),
		PokemonTypes: fictionalPokemonTypes,
		Moves:        failingMoves{err: fmt.Errorf("%w: move-9999 (read from /secret/moves.json)", balance.ErrUnknownMove)},
	})
	recorder := postCoverage(t, server, `{"members":[{"pokemonId":"9001-000","moveIds":["move-9999"]}]}`)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body=%s", recorder.Code, recorder.Body.String())
	}
	got := decodeError(t, recorder.Body.Bytes())
	if got.Code != api.UnknownMove {
		t.Errorf("code = %q, want unknown_move", got.Code)
	}
	if got.Message != "unknown moveId: move-9999" {
		t.Errorf("message = %q, want %q", got.Message, "unknown moveId: move-9999")
	}
}

// ADR-0014 §5.5 (applied to TB2): unexpected internal failures answer 500 with a fixed message.
func TestCoverageInternalErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		deps Dependencies
	}{
		{name: "move provider failure", deps: Dependencies{TypeChart: testTypeChart(), PokemonTypes: fictionalPokemonTypes, Moves: failingMoves{err: errors.New("move backend exploded at /secret/moves.json")}}},
		{name: "pokemon provider failure", deps: Dependencies{TypeChart: testTypeChart(), PokemonTypes: failingPokemonTypes{err: errors.New("pokemon backend exploded")}, Moves: fictionalMoves}},
		{name: "nil type chart", deps: Dependencies{PokemonTypes: fictionalPokemonTypes, Moves: fictionalMoves}},
		{name: "move provider returns an invalid category", deps: Dependencies{TypeChart: testTypeChart(), PokemonTypes: fictionalPokemonTypes, Moves: testMoves{
			"move-9001": fictionalMove("move-9001", balance.TypeFire, "other"),
		}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := postCoverage(t, New(tt.deps), `{"members":[{"pokemonId":"9001-000","moveIds":["move-9001"]}]}`)
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
		})
	}
}

func TestCoverageWithExampleReadModels(t *testing.T) {
	t.Parallel()

	pokemon, err := master.LoadPokemonTypesFile("../../testdata/pokemon-types.example.json")
	if err != nil || pokemon == nil {
		t.Fatalf("LoadPokemonTypesFile(example) = %v, %v", pokemon, err)
	}
	moves, err := master.LoadMovesFile("../../testdata/moves.example.json")
	if err != nil || moves == nil {
		t.Fatalf("LoadMovesFile(example) = %v, %v", moves, err)
	}
	server := New(Dependencies{TypeChart: testTypeChart(), PokemonTypes: pokemon, Moves: moves})
	recorder := postCoverage(t, server, `{"members":[{"pokemonId":"9001-000","moveIds":["move-9001","move-9002","move-9006"]},{"pokemonId":"9002-000","moveIds":["move-9005"]}]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeCoverageResponse(t, recorder.Body.Bytes())
	if len(response.Members) != 2 {
		t.Fatalf("members = %d, want 2", len(response.Members))
	}
	if fmt.Sprint(response.Members[0].AttackTypes) != fmt.Sprint([]api.TypeId{api.Fire}) {
		t.Errorf("members[0].attackTypes = %v, want [fire]", response.Members[0].AttackTypes)
	}
	if got := findCoverageEntry(t, response.Members[1].Coverage, api.Ghost); coverageLabel(got.BestMultiplier) != "0" {
		t.Errorf("members[1] vs ghost = %s, want 0", coverageLabel(got.BestMultiplier))
	}
}
