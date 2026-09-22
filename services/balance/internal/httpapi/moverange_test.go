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
	"github.com/labstack/echo/v5"
)

// TB6 技範囲チェッカーの HTTP 契約(ADR-0404)。ID と名前はすべて架空。
//
// 期待値を手で数えられるように、既定が ×1 の架空の相性表(mrHTTPChart)を使う:
//
//	fire   → fire ×1/2・water ×1/2・rock ×1/2・grass ×2
//	water  → water ×1/2・grass ×1/2・fire ×2・rock ×2
//	normal → rock ×1/2・ghost ×0
//
// カタログと特性は internal/balance の TB6 テストと同じ構成で、fire 単発なら
// water / water・rock / rock がタイプだけで受けられ、grass と normal は特性で受けられる。

const moveRangePath = "/api/balance/v1/move-range/analyze"

func mrHTTPChart() recHTTPChart {
	c := recHTTPChart{}
	set := func(attack, defense balance.TypeID, m balance.Multiplier) {
		c[[2]balance.TypeID{attack, defense}] = m
	}
	set(balance.TypeFire, balance.TypeFire, balance.MultiplierHalf)
	set(balance.TypeFire, balance.TypeWater, balance.MultiplierHalf)
	set(balance.TypeFire, balance.TypeRock, balance.MultiplierHalf)
	set(balance.TypeFire, balance.TypeGrass, balance.MultiplierDouble)
	set(balance.TypeWater, balance.TypeWater, balance.MultiplierHalf)
	set(balance.TypeWater, balance.TypeGrass, balance.MultiplierHalf)
	set(balance.TypeWater, balance.TypeFire, balance.MultiplierDouble)
	set(balance.TypeWater, balance.TypeRock, balance.MultiplierDouble)
	set(balance.TypeNormal, balance.TypeRock, balance.MultiplierHalf)
	set(balance.TypeNormal, balance.TypeGhost, balance.MultiplierZero)
	return c
}

// mrHTTPCatalog is deliberately not in pokemonId order (ADR-0404 §2 sorts the output).
var mrHTTPCatalog = testCatalog{
	{PokemonID: "9002-000", Types: []balance.TypeID{balance.TypeWater, balance.TypeRock}},
	{PokemonID: "9001-000", NameJa: "テストミズ", Types: []balance.TypeID{balance.TypeWater}, AbilityIDs: []string{"ability-9003"}},
	{PokemonID: "9003-000", NameJa: "テストクサ", Types: []balance.TypeID{balance.TypeGrass}, AbilityIDs: []string{"ability-9002", "ability-9001"}},
	{PokemonID: "9004-000", Types: []balance.TypeID{balance.TypeNormal}, AbilityIDs: []string{"ability-9003"}},
	{PokemonID: "9005-000", NameJa: "テストゴースト", Types: []balance.TypeID{balance.TypeGhost}, AbilityIDs: []string{"ability-9004"}},
	{PokemonID: "9002-001", NameJa: "テストイワ", Types: []balance.TypeID{balance.TypeRock}},
}

var mrHTTPAbilities = testAbilities{
	"ability-9001": fictionalAbility("ability-9001", balance.AbilityEffect{Kind: balance.AbilityEffectImmune, AttackType: balance.TypeFire}),
	"ability-9002": fictionalAbility("ability-9002", balance.AbilityEffect{Kind: balance.AbilityEffectAbsorb, AttackType: balance.TypeFire}),
	"ability-9003": fictionalAbility("ability-9003", balance.AbilityEffect{Kind: balance.AbilityEffectTypeMultiplier, AttackType: balance.TypeFire, Factor: fraction(1, 2)}),
	"ability-9004": fictionalAbility("ability-9004", balance.AbilityEffect{Kind: balance.AbilityEffectTypeMultiplier, AttackType: balance.TypeFire, Factor: fraction(5, 4)}),
}

var mrHTTPMoves = testMoves{
	"move-9001":     fictionalMove("move-9001", balance.TypeFire, balance.MoveCategorySpecial),
	"move-9002":     fictionalMove("move-9002", balance.TypeFire, balance.MoveCategoryPhysical),
	"move-9003":     fictionalMove("move-9003", balance.TypeWater, balance.MoveCategorySpecial),
	"move-9004":     fictionalMove("move-9004", balance.TypeElectric, balance.MoveCategorySpecial),
	"move-9005":     fictionalMove("move-9005", balance.TypeNormal, balance.MoveCategoryPhysical),
	"move-9006":     fictionalMove("move-9006", balance.TypeGrass, balance.MoveCategoryStatus),
	"move-9012":     fictionalMove("move-9012", balance.TypeRock, balance.MoveCategoryStatus),
	fortyCharMoveID: fictionalMove(fortyCharMoveID, balance.TypeElectric, balance.MoveCategorySpecial),
}

func mrFullDependencies() Dependencies {
	return Dependencies{
		TypeChart:      mrHTTPChart(),
		PokemonTypes:   mrHTTPCatalog.types(),
		PokemonCatalog: mrHTTPCatalog,
		Moves:          mrHTTPMoves,
		Abilities:      mrHTTPAbilities,
	}
}

func newMoveRangeServer() *echo.Echo { return New(mrFullDependencies()) }

func postMoveRange(t *testing.T, server http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, moveRangePath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Device-Id", "test-device")
	request.Header.Set("X-Session-Id", "test-session")
	server.ServeHTTP(recorder, request)
	return recorder
}

// decodeMoveRangeResponse decodes into the generated type with unknown fields rejected.
func decodeMoveRangeResponse(t *testing.T, body []byte) api.MoveRangeResponse {
	t.Helper()
	var response api.MoveRangeResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("decode MoveRangeResponse: %v; body=%s", err, body)
	}
	return response
}

// mrBody is the shared 200 fixture: a single fire move.
const mrBody = `{"moveIds":["move-9001"]}`

func TestMoveRangeResponseBody(t *testing.T) {
	t.Parallel()

	recorder := postMoveRange(t, newMoveRangeServer(), mrBody)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/json") {
		t.Errorf("Content-Type = %q, want application/json", recorder.Header().Get("Content-Type"))
	}
	response := decodeMoveRangeResponse(t, recorder.Body.Bytes())

	if got := apiTypeLabel(response.AttackTypes); got != "fire" {
		t.Errorf("attackTypes = %s, want fire", got)
	}

	// fire: fire ×1/2, water ×1/2, rock ×1/2, grass ×2, それ以外 ×1(normal … fairy の順)。
	wantChart := strings.Fields("1 1/2 1/2 1 2 1 1 1 1 1 1 1 1/2 1 1 1 1 1")
	if len(response.TypeChart) != 18 {
		t.Fatalf("typeChart has %d entries, want 18", len(response.TypeChart))
	}
	for i, defense := range balance.AllTypes() {
		entry := response.TypeChart[i]
		if string(entry.DefenseType) != string(defense) {
			t.Fatalf("typeChart[%d].defenseType = %s, want %s (canonical order)", i, entry.DefenseType, defense)
		}
		if string(entry.BestMultiplier) != wantChart[i] {
			t.Errorf("typeChart[%s].bestMultiplier = %s, want %s", defense, entry.BestMultiplier, wantChart[i])
		}
		wantEffective := wantChart[i] != "0" && wantChart[i] != "1/2"
		if entry.Effective != wantEffective || entry.SuperEffective != (wantChart[i] == "2") {
			t.Errorf("typeChart[%s] effective/superEffective = %v/%v, want %v/%v",
				defense, entry.Effective, entry.SuperEffective, wantEffective, wantChart[i] == "2")
		}
	}

	type walled struct{ id, name, types, multiplier string }
	wantWalled := []walled{
		{"9001-000", "テストミズ", "water", "1/2"},
		{"9002-000", "<absent>", "water/rock", "1/4"},
		{"9002-001", "テストイワ", "rock", "1/2"},
	}
	gotWalled := make([]walled, len(response.WalledBy))
	for i, p := range response.WalledBy {
		gotWalled[i] = walled{p.PokemonId, deref(p.NameJa), apiTypeLabel(p.Types), string(p.BestMultiplier)}
	}
	if fmt.Sprint(gotWalled) != fmt.Sprint(wantWalled) {
		t.Errorf("walledBy = %v, want %v", gotWalled, wantWalled)
	}

	type walledAbility struct{ id, name, ability, multiplier string }
	wantAbility := []walledAbility{
		{"9003-000", "テストクサ", "ability-9001", "0"},
		{"9003-000", "テストクサ", "ability-9002", "0"},
		{"9004-000", "<absent>", "ability-9003", "1/2"},
	}
	gotAbility := make([]walledAbility, len(response.WalledByAbility))
	for i, p := range response.WalledByAbility {
		gotAbility[i] = walledAbility{p.PokemonId, deref(p.NameJa), p.AbilityId, string(p.BestMultiplier)}
	}
	if fmt.Sprint(gotAbility) != fmt.Sprint(wantAbility) {
		t.Errorf("walledByAbility = %v, want %v", gotAbility, wantAbility)
	}
}

// Field names, omitted nameJa, empty arrays as [] (never null), and a bestMultiplier that is
// never null in typeChart (ADR-0404 §2: a move set without an attack move is rejected).
func TestMoveRangeResponseUsesContractFieldNames(t *testing.T) {
	t.Parallel()

	recorder := postMoveRange(t, newMoveRangeServer(), mrBody)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"attackTypes", "typeChart", "walledBy", "walledByAbility"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("response is missing %q", key)
		}
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(raw["typeChart"], &entries); err != nil || len(entries) != 18 {
		t.Fatalf("typeChart = %s, err = %v", raw["typeChart"], err)
	}
	for _, key := range []string{"defenseType", "bestMultiplier", "effective", "superEffective"} {
		if _, ok := entries[0][key]; !ok {
			t.Errorf("typeChart entry is missing %q", key)
		}
	}
	for i, entry := range entries {
		if string(entry["bestMultiplier"]) == "null" {
			t.Errorf("typeChart[%d].bestMultiplier is null, want a value", i)
		}
	}

	var walled []map[string]json.RawMessage
	if err := json.Unmarshal(raw["walledBy"], &walled); err != nil || len(walled) != 3 {
		t.Fatalf("walledBy = %s, err = %v", raw["walledBy"], err)
	}
	if got := string(walled[0]["nameJa"]); got != `"テストミズ"` {
		t.Errorf("walledBy[0].nameJa = %s, want \"テストミズ\"", got)
	}
	if _, ok := walled[1]["nameJa"]; ok {
		t.Errorf("walledBy[1] has no nameJa in the read model, so the key must be omitted; got %s", walled[1]["nameJa"])
	}
	if got := string(walled[1]["types"]); got != `["water","rock"]` {
		t.Errorf("walledBy[1].types = %s, want the read model order [\"water\",\"rock\"]", got)
	}

	// move-9004 (electric) is x1 against everything in this chart, and no ability touches electric.
	empty := postMoveRange(t, newMoveRangeServer(), `{"moveIds":["move-9004"]}`)
	if empty.Code != http.StatusOK {
		t.Fatalf("electric: status = %d, want 200; body=%s", empty.Code, empty.Body.String())
	}
	var rawEmpty map[string]json.RawMessage
	if err := json.Unmarshal(empty.Body.Bytes(), &rawEmpty); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"walledBy", "walledByAbility"} {
		if got := string(rawEmpty[key]); got != "[]" {
			t.Errorf("%s = %s, want [] (an array, not null)", key, got)
		}
	}
}

// ADR-0404 §2: 変化技は攻撃範囲に入らない。攻撃技が1つでもあれば 200。
func TestMoveRangeIgnoresStatusMoves(t *testing.T) {
	t.Parallel()

	withStatus := postMoveRange(t, newMoveRangeServer(), `{"moveIds":["move-9001","move-9006"]}`)
	if withStatus.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", withStatus.Code, withStatus.Body.String())
	}
	only := postMoveRange(t, newMoveRangeServer(), mrBody)
	if only.Code != http.StatusOK {
		t.Fatalf("fire only: status = %d, want 200; body=%s", only.Code, only.Body.String())
	}
	if withStatus.Body.String() != only.Body.String() {
		t.Errorf("a status move changed the result:\nwith status %s\nfire only   %s", withStatus.Body.String(), only.Body.String())
	}
}

// ADR-0404 §2: 全件が変化技の入力は 400 invalid_request(「技範囲が無い」を返さない)。
func TestMoveRangeRejectsStatusOnlyMoveSet(t *testing.T) {
	t.Parallel()

	for _, body := range []string{`{"moveIds":["move-9006"]}`, `{"moveIds":["move-9006","move-9012"]}`} {
		recorder := postMoveRange(t, newMoveRangeServer(), body)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400; body=%s", body, recorder.Code, recorder.Body.String())
			continue
		}
		if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.InvalidRequest {
			t.Errorf("%s: code = %q, want invalid_request", body, got.Code)
		}
	}
}

// ADR-0404 §2: 特性の read model が無くても walledByAbility を空にして 200(503 にしない)。
// ほかの結果は変わらない。
func TestMoveRangeWithoutAbilityReadModel(t *testing.T) {
	t.Parallel()

	deps := mrFullDependencies()
	deps.Abilities = nil
	recorder := postMoveRange(t, New(deps), mrBody)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 without the ability read model; body=%s", recorder.Code, recorder.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := string(raw["walledByAbility"]); got != "[]" {
		t.Errorf("walledByAbility = %s, want []", got)
	}

	with := postMoveRange(t, newMoveRangeServer(), mrBody)
	if with.Code != http.StatusOK {
		t.Fatalf("with abilities: status = %d; body=%s", with.Code, with.Body.String())
	}
	a := decodeMoveRangeResponse(t, recorder.Body.Bytes())
	b := decodeMoveRangeResponse(t, with.Body.Bytes())
	a.WalledByAbility, b.WalledByAbility = nil, nil
	aJSON, errA := json.Marshal(a)
	bJSON, errB := json.Marshal(b)
	if errA != nil || errB != nil || string(aJSON) != string(bJSON) {
		t.Errorf("the rest of the response depends on the ability read model:\nwithout %s\nwith    %s", aJSON, bJSON)
	}
}

// TB6 は pokemonId を受け取らないので、型の provider(PokemonTypes)は要らない。
// 必要なのは技の read model と一覧のカタログだけ(ADR-0404 §2)。
func TestMoveRangeNeedsNoPokemonTypeProvider(t *testing.T) {
	t.Parallel()

	deps := mrFullDependencies()
	deps.PokemonTypes = nil
	recorder := postMoveRange(t, New(deps), mrBody)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 without the pokemon type provider; body=%s", recorder.Code, recorder.Body.String())
	}
	decodeMoveRangeResponse(t, recorder.Body.Bytes())
}

// ADR-0404 §4.5 / ADR-0401 §7.2: カタログの abilityIds に特性 read model が知らない ID があっても
// 500 にならず、その特性だけ飛ばして 200 を返す。
func TestMoveRangeSkipsUnknownCatalogAbility(t *testing.T) {
	t.Parallel()

	deps := mrFullDependencies()
	deps.PokemonCatalog = testCatalog{
		{PokemonID: "9010-000", NameJa: "テストミステリー", Types: []balance.TypeID{balance.TypeGrass}, AbilityIDs: []string{"ability-9999", "ability-9001"}},
	}
	recorder := postMoveRange(t, New(deps), `{"moveIds":["move-9001"]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeMoveRangeResponse(t, recorder.Body.Bytes())
	if len(response.WalledByAbility) != 1 || response.WalledByAbility[0].AbilityId != "ability-9001" {
		t.Errorf("walledByAbility = %+v, want one entry for ability-9001 (ability-9999 skipped, not an error)", response.WalledByAbility)
	}
}

func TestMoveRangeAcceptsBoundaries(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"one move":                             `{"moveIds":["move-9001"]}`,
		"four moves":                           `{"moveIds":["move-9005","move-9001","move-9003","move-9004"]}`,
		"a forty-character moveId":             `{"moveIds":["` + fortyCharMoveID + `"]}`,
		"three attack moves and a status move": `{"moveIds":["move-9001","move-9003","move-9005","move-9006"]}`,
	} {
		recorder := postMoveRange(t, newMoveRangeServer(), body)
		if recorder.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200; body=%s", name, recorder.Code, recorder.Body.String())
			continue
		}
		decodeMoveRangeResponse(t, recorder.Body.Bytes())
	}
}

func TestMoveRangeRejectsInvalidBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"moveIds":`},
		{name: "trailing JSON", body: mrBody + ` {}`},
		{name: "not an object", body: `[]`},
		{name: "missing moveIds", body: `{}`},
		{name: "null moveIds", body: `{"moveIds":null}`},
		{name: "empty moveIds", body: `{"moveIds":[]}`},
		{name: "moveIds is not an array", body: `{"moveIds":"move-9001"}`},
		{name: "five moveIds", body: `{"moveIds":["move-9001","move-9002","move-9003","move-9004","move-9005"]}`},
		{name: "duplicate moveId", body: `{"moveIds":["move-9001","move-9001"]}`},
		{name: "empty moveId", body: `{"moveIds":[""]}`},
		{name: "uppercase moveId", body: `{"moveIds":["Move-9001"]}`},
		{name: "underscore in moveId", body: `{"moveIds":["move_9001"]}`},
		{name: "trailing hyphen in moveId", body: `{"moveIds":["move-9001-"]}`},
		{name: "forty-one-character moveId", body: `{"moveIds":["` + fortyCharMoveID + `a"]}`},
		{name: "unknown top-level property", body: `{"moveIds":["move-9001"],"members":[]}`},
		{name: "members instead of moveIds", body: `{"members":[{"pokemonId":"9001-000","moveIds":["move-9001"]}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := postMoveRange(t, newMoveRangeServer(), tt.body)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
			}
			if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.InvalidRequest {
				t.Errorf("code = %q, want invalid_request", got.Code)
			}
		})
	}
}

func TestMoveRangeRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	body := `{"moveIds":["` + strings.Repeat("a", maxAnalyzeBodyBytes) + `"]}`
	recorder := postMoveRange(t, newMoveRangeServer(), body)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413; body=%s", recorder.Code, recorder.Body.String())
	}
	if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.RequestTooLarge {
		t.Errorf("code = %q, want request_too_large", got.Code)
	}
}

func TestMoveRangeRequiresRequestContext(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ name, deviceID, sessionID string }{
		{name: "missing both"},
		{name: "missing device", sessionID: "test-session"},
		{name: "missing session", deviceID: "test-device"},
		{name: "blank device", deviceID: "   ", sessionID: "test-session"},
		{name: "blank session", deviceID: "test-device", sessionID: "\t"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, moveRangePath, strings.NewReader(mrBody))
			request.Header.Set("Content-Type", "application/json")
			if tt.deviceID != "" {
				request.Header.Set("X-Device-Id", tt.deviceID)
			}
			if tt.sessionID != "" {
				request.Header.Set("X-Session-Id", tt.sessionID)
			}
			newMoveRangeServer().ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
			}
			if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.MissingRequestContext {
				t.Errorf("code = %q, want missing_request_context", got.Code)
			}
		})
	}
}

// ADR-0404 §2 の判定順: ヘッダー(400)→ body(400/413)→ 技の read model 未設定(503)→
// moveId の解決(422 unknown_move)→ ポケモンのカタログ未設定(503)→ 200。
func TestMoveRangeDecisionOrder(t *testing.T) {
	t.Parallel()

	without := func(mutate func(*Dependencies)) Dependencies {
		d := mrFullDependencies()
		mutate(&d)
		return d
	}
	full := mrFullDependencies()
	noMoves := without(func(d *Dependencies) { d.Moves = nil })
	noCatalog := without(func(d *Dependencies) { d.PokemonCatalog = nil })
	none := Dependencies{TypeChart: mrHTTPChart()}

	oversized := `{"moveIds":["` + strings.Repeat("a", maxAnalyzeBodyBytes) + `"]}`
	tests := []struct {
		name        string
		deps        Dependencies
		noHeaders   bool
		body        string
		wantStatus  int
		wantCode    api.ErrorCode
		wantMessage string
	}{
		{name: "headers before body and read models", deps: none, noHeaders: true, body: `{"moveIds":[]}`, wantStatus: http.StatusBadRequest, wantCode: api.MissingRequestContext},
		{name: "invalid body before missing read models", deps: none, body: `{"moveIds":[]}`, wantStatus: http.StatusBadRequest, wantCode: api.InvalidRequest},
		{name: "duplicate moveId before missing read models", deps: none, body: `{"moveIds":["move-9001","move-9001"]}`, wantStatus: http.StatusBadRequest, wantCode: api.InvalidRequest},
		{name: "oversized body before missing read models", deps: none, body: oversized, wantStatus: http.StatusRequestEntityTooLarge, wantCode: api.RequestTooLarge},

		{name: "move read model missing", deps: noMoves, body: mrBody, wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},
		{name: "move read model missing before unknown move", deps: noMoves, body: `{"moveIds":["move-9999"]}`, wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},
		{name: "every read model missing", deps: none, body: mrBody, wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},

		{name: "unknown move", deps: full, body: `{"moveIds":["move-9999"]}`, wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownMove, wantMessage: "unknown moveId: move-9999"},
		{name: "unknown move: the first in request order", deps: full, body: `{"moveIds":["move-9001","move-9998","move-9999"]}`, wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownMove, wantMessage: "unknown moveId: move-9998"},
		{name: "unknown move before the missing catalog", deps: noCatalog, body: `{"moveIds":["move-9999"]}`, wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownMove, wantMessage: "unknown moveId: move-9999"},

		{name: "pokemon catalog missing", deps: noCatalog, body: mrBody, wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, moveRangePath, strings.NewReader(tt.body))
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

	// All checks pass with only what TB6 needs: the chart, the moves and the catalog.
	minimal := Dependencies{TypeChart: mrHTTPChart(), PokemonCatalog: mrHTTPCatalog, Moves: mrHTTPMoves}
	if recorder := postMoveRange(t, New(minimal), mrBody); recorder.Code != http.StatusOK {
		t.Errorf("minimal dependencies: status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
}

// 422 の message は request の ID だけから作る(adapter の詳細を漏らさない。ADR-0014 §5.6)。
func TestMoveRangeUnknownMessageDoesNotLeakAdapterDetail(t *testing.T) {
	t.Parallel()

	deps := mrFullDependencies()
	deps.Moves = failingMoves{err: fmt.Errorf("%w: move-9999 (read from /secret/moves.json)", balance.ErrUnknownMove)}
	recorder := postMoveRange(t, New(deps), `{"moveIds":["move-9999"]}`)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body=%s", recorder.Code, recorder.Body.String())
	}
	if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.UnknownMove || got.Message != "unknown moveId: move-9999" {
		t.Errorf("error = %+v, want unknown_move with %q", got, "unknown moveId: move-9999")
	}
}

// ADR-0404 §2: それ以外の内部エラーは 500 固定文言。
func TestMoveRangeInternalErrors(t *testing.T) {
	t.Parallel()

	withDeps := func(mutate func(*Dependencies)) Dependencies {
		d := mrFullDependencies()
		mutate(&d)
		return d
	}
	tests := []struct {
		name string
		deps Dependencies
	}{
		{name: "nil type chart", deps: withDeps(func(d *Dependencies) { d.TypeChart = nil })},
		{name: "type chart matchup failure", deps: withDeps(func(d *Dependencies) {
			d.TypeChart = failingChart{err: errors.New("type chart backend exploded")}
		})},
		{name: "catalog failure", deps: withDeps(func(d *Dependencies) {
			d.PokemonCatalog = failingCatalog{err: errors.New("catalog backend exploded at /secret/pokemon.json")}
		})},
		{name: "move provider failure", deps: withDeps(func(d *Dependencies) {
			d.Moves = failingMoves{err: errors.New("move backend exploded at /secret/moves.json")}
		})},
		{name: "ability provider failure for a catalog pokemon", deps: withDeps(func(d *Dependencies) {
			d.Abilities = failingAbilities{err: errors.New("ability backend exploded at /secret/abilities.json")}
		})},
		{name: "invalid move category from the provider", deps: withDeps(func(d *Dependencies) {
			d.Moves = testMoves{"move-9001": {MoveID: "move-9001", Type: balance.TypeFire, Category: "other"}}
		})},
		{name: "invalid move type from the provider", deps: withDeps(func(d *Dependencies) {
			d.Moves = testMoves{"move-9001": {MoveID: "move-9001", Type: "plasma", Category: balance.MoveCategorySpecial}}
		})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := postMoveRange(t, New(tt.deps), mrBody)
			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500; body=%s", recorder.Code, recorder.Body.String())
			}
			if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.InternalError || got.Message != "internal error" {
				t.Errorf("error = %+v, want internal_error with the fixed text %q", got, "internal error")
			}
		})
	}
}

// move-range を足しても health・analyze・coverage は変わらない。
func TestMoveRangeDoesNotAffectOtherEndpoints(t *testing.T) {
	t.Parallel()

	server := newMoveRangeServer()
	for _, path := range []string{"/healthz", "/api/balance/healthz"} {
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", path, recorder.Code)
		}
	}

	analyze := httptest.NewRecorder()
	analyzeRequest := httptest.NewRequest(http.MethodPost, "/api/balance/v1/team-balance/analyze", strings.NewReader(`{"members":[{"pokemonId":"9001-000"}]}`))
	analyzeRequest.Header.Set("Content-Type", "application/json")
	analyzeRequest.Header.Set("X-Device-Id", "test-device")
	analyzeRequest.Header.Set("X-Session-Id", "test-session")
	server.ServeHTTP(analyze, analyzeRequest)
	if analyze.Code != http.StatusOK {
		t.Errorf("analyze: status = %d, want 200; body=%s", analyze.Code, analyze.Body.String())
	}

	coverage := httptest.NewRecorder()
	coverageRequest := httptest.NewRequest(http.MethodPost, "/api/balance/v1/team-balance/coverage", strings.NewReader(`{"members":[{"pokemonId":"9001-000","moveIds":["move-9001"]}]}`))
	coverageRequest.Header.Set("Content-Type", "application/json")
	coverageRequest.Header.Set("X-Device-Id", "test-device")
	coverageRequest.Header.Set("X-Session-Id", "test-session")
	server.ServeHTTP(coverage, coverageRequest)
	if coverage.Code != http.StatusOK {
		t.Errorf("coverage: status = %d, want 200; body=%s", coverage.Code, coverage.Body.String())
	}
}
