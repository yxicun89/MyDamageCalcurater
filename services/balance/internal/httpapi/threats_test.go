package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/pokecalc/services/balance/internal/api"
	"example.com/pokecalc/services/balance/internal/balance"
	"example.com/pokecalc/services/balance/internal/master"
	"github.com/labstack/echo/v5"
)

// TB4 仮想敵診断の HTTP 契約(ADR-0400)。ID はすべて架空(9001-000 / move-9001 / ability-9001 以降)。
// 倍率の期待値は同梱の相性表(testTypeChart)で成り立つ組だけを使う。

const threatsPath = "/api/balance/v1/team-balance/threats"

// threatPokemonTypes adds a dragon/flying entry (as in testdata/pokemon-types.example.json) to fictionalPokemonTypes.
var threatPokemonTypes = func() testPokemonTypes {
	p := testPokemonTypes{"9006-001": {balance.TypeDragon, balance.TypeFlying}}
	for id, types := range fictionalPokemonTypes {
		p[id] = types
	}
	return p
}()

// threatMoves adds the remaining example moves and one fictional grass attack to fictionalMoves.
var threatMoves = func() testMoves {
	p := testMoves{
		"move-9008": fictionalMove("move-9008", balance.TypeIce, balance.MoveCategorySpecial),
		"move-9011": fictionalMove("move-9011", balance.TypeDragon, balance.MoveCategorySpecial),
		"move-9012": fictionalMove("move-9012", balance.TypePsychic, balance.MoveCategoryStatus),
		"move-9013": fictionalMove("move-9013", balance.TypeGrass, balance.MoveCategorySpecial),
	}
	for id, move := range fictionalMoves {
		p[id] = move
	}
	return p
}()

func newThreatsServer() *echo.Echo {
	return New(Dependencies{
		TypeChart:    testTypeChart(),
		PokemonTypes: threatPokemonTypes,
		Moves:        threatMoves,
		Abilities:    fictionalAbilities,
	})
}

func postThreats(t *testing.T, server http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, threatsPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Device-Id", "test-device")
	request.Header.Set("X-Session-Id", "test-session")
	server.ServeHTTP(recorder, request)
	return recorder
}

// decodeThreatsResponse decodes into the generated type with unknown fields rejected and checks,
// on every matchup, that the multipliers match the pattern in lowest terms and that safe /
// superEffective and the per-threat counts follow from them (ADR-0400 §2).
func decodeThreatsResponse(t *testing.T, body []byte) api.ThreatsResponse {
	t.Helper()
	var response api.ThreatsResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("decode ThreatsResponse: %v; body=%s", err, body)
	}
	for ti, threat := range response.Threats {
		safe, super := 0, 0
		for mi, matchup := range threat.Matchups {
			wantSafe := false
			if matchup.Incoming != nil {
				wantSafe = parseMultiplier(t, *matchup.Incoming).Cmp(big.NewRat(1, 1)) < 0
			}
			wantSuper := false
			if matchup.Outgoing != nil {
				wantSuper = parseMultiplier(t, *matchup.Outgoing).Cmp(big.NewRat(2, 1)) >= 0
			}
			if matchup.Safe != wantSafe || matchup.SuperEffective != wantSuper {
				t.Errorf("threats[%d].matchups[%d] = %+v: safe/superEffective do not follow the multipliers", ti, mi, matchup)
			}
			if matchup.Safe {
				safe++
			}
			if matchup.SuperEffective {
				super++
			}
		}
		if threat.SafeMembers != safe || threat.SuperEffectiveMembers != super {
			t.Errorf("threats[%d] counts = safe %d super %d, want %d %d", ti, threat.SafeMembers, threat.SuperEffectiveMembers, safe, super)
		}
	}
	return response
}

func matchupLabel(m *api.MatchupMultiplier) string {
	if m == nil {
		return "null"
	}
	return *m
}

// threatsBody is the 200 fixture shared by the body and field-name tests.
//
//	members: m0 9002-000 grass [fire, grass status]; m1 9003-000 water/ground [ground, ice] ability-9004 (super effective x3/4);
//	         m2 9001-000 fire/flying [] ability-9003 (fire x1/2)
//	threats: t0 9005-000 ice [fire, psychic status]; t1 9002-000 grass [] ability-9004;
//	         t2 9004-000 normal/ghost [electric, ground]; t3 9006-001 dragon/flying [dragon, grass] ability-9001 (ground immune)
const threatsBody = `{
	"members":[
		{"pokemonId":"9002-000","moveIds":["move-9001","move-9006"]},
		{"pokemonId":"9003-000","moveIds":["move-9007","move-9008"],"abilityId":"ability-9004"},
		{"pokemonId":"9001-000","moveIds":[],"abilityId":"ability-9003"}
	],
	"threats":[
		{"pokemonId":"9005-000","moveIds":["move-9002","move-9012"]},
		{"pokemonId":"9002-000","moveIds":[],"abilityId":"ability-9004"},
		{"pokemonId":"9004-000","moveIds":["move-9004","move-9007"]},
		{"pokemonId":"9006-001","moveIds":["move-9011","move-9013"],"abilityId":"ability-9001"}
	]
}`

func TestThreatsResponseBody(t *testing.T) {
	t.Parallel()

	recorder := postThreats(t, newThreatsServer(), threatsBody)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/json") {
		t.Errorf("Content-Type = %q, want application/json", recorder.Header().Get("Content-Type"))
	}
	response := decodeThreatsResponse(t, recorder.Body.Bytes())
	if len(response.Threats) != 4 {
		t.Fatalf("threats length = %d, want 4; body=%s", len(response.Threats), recorder.Body.String())
	}

	type matchup struct {
		incoming, outgoing string
		safe, super        bool
	}
	want := []struct {
		pokemonID   string
		abilityID   string
		attackTypes []api.TypeId
		matchups    []matchup
		safe, super int
	}{
		{
			// fire vs grass x2 / vs water-ground x1/2 / vs fire-flying x1/2 x ability 1/2 = 1/4.
			// fire vs ice x2; ground x1, ice x1/2 → x1; no moves → null.
			pokemonID: "9005-000", attackTypes: []api.TypeId{api.Fire},
			matchups: []matchup{{"2", "2", false, true}, {"1/2", "1", true, false}, {"1/4", "null", true, false}},
			safe:     2, super: 1,
		},
		{
			// no attack move → incoming null. fire vs grass x2 x3/4 = 3/2; ground x1/2, ice x2 x3/4 = 3/2.
			pokemonID: "9002-000", abilityID: "ability-9004", attackTypes: []api.TypeId{},
			matchups: []matchup{{"null", "3/2", false, false}, {"null", "3/2", false, false}, {"null", "null", false, false}},
			safe:     0, super: 0,
		},
		{
			// electric/ground vs grass x1/2; vs water-ground: electric x0, ground x1 (not super effective, so no x3/4) → x1;
			// vs fire-flying: electric x2, ground x0 → x2. Outgoing vs normal-ghost: fire x1; ground x1, ice x1.
			pokemonID: "9004-000", attackTypes: []api.TypeId{api.Electric, api.Ground},
			matchups: []matchup{{"1/2", "1", true, false}, {"1", "1", false, false}, {"2", "null", false, false}},
			safe:     1, super: 0,
		},
		{
			// grass/dragon vs grass: x1/2, x1 → x1; vs water-ground: grass x4 x3/4 = 3 → x3; vs fire-flying: grass x1/4, dragon x1 → x1.
			// Outgoing vs dragon-flying (ground immune by type): fire x1/2; ground x0, ice x4 → x4.
			pokemonID: "9006-001", abilityID: "ability-9001", attackTypes: []api.TypeId{api.Grass, api.Dragon},
			matchups: []matchup{{"1", "1/2", false, false}, {"3", "4", false, true}, {"1", "null", false, false}},
			safe:     0, super: 1,
		},
	}
	memberIDs := []string{"9002-000", "9003-000", "9001-000"}
	for ti, w := range want {
		got := response.Threats[ti]
		if got.PokemonId != w.pokemonID {
			t.Errorf("threats[%d].pokemonId = %q, want %q (request order)", ti, got.PokemonId, w.pokemonID)
		}
		// A threat without abilityId must not echo one (deref reports a missing key as "<absent>").
		wantAbility := w.abilityID
		if wantAbility == "" {
			wantAbility = "<absent>"
		}
		if deref(got.AbilityId) != wantAbility {
			t.Errorf("threats[%d].abilityId = %q, want %q", ti, deref(got.AbilityId), wantAbility)
		}
		if fmt.Sprint(got.AttackTypes) != fmt.Sprint(w.attackTypes) {
			t.Errorf("threats[%d].attackTypes = %v, want %v", ti, got.AttackTypes, w.attackTypes)
		}
		if len(got.Matchups) != len(memberIDs) {
			t.Fatalf("threats[%d].matchups length = %d, want %d", ti, len(got.Matchups), len(memberIDs))
		}
		for mi, wm := range w.matchups {
			m := got.Matchups[mi]
			if m.PokemonId != memberIDs[mi] {
				t.Errorf("threats[%d].matchups[%d].pokemonId = %q, want %q (member order)", ti, mi, m.PokemonId, memberIDs[mi])
			}
			if matchupLabel(m.Incoming) != wm.incoming || matchupLabel(m.Outgoing) != wm.outgoing || m.Safe != wm.safe || m.SuperEffective != wm.super {
				t.Errorf("threats[%d].matchups[%d] = incoming %s outgoing %s safe %v super %v, want %s %s %v %v", ti, mi,
					matchupLabel(m.Incoming), matchupLabel(m.Outgoing), m.Safe, m.SuperEffective, wm.incoming, wm.outgoing, wm.safe, wm.super)
			}
		}
		if got.SafeMembers != w.safe || got.SuperEffectiveMembers != w.super {
			t.Errorf("threats[%d] = safeMembers %d superEffectiveMembers %d, want %d %d", ti, got.SafeMembers, got.SuperEffectiveMembers, w.safe, w.super)
		}
	}
}

// The JSON uses the contract names, an array (never null) for attackTypes, strings for
// multipliers, an explicit null incoming / outgoing without attack moves, and abilityId only when named.
func TestThreatsResponseUsesContractFieldNames(t *testing.T) {
	t.Parallel()

	recorder := postThreats(t, newThreatsServer(), threatsBody)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	var raw struct {
		Threats []map[string]json.RawMessage `json:"threats"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(raw.Threats) != 4 {
		t.Fatalf("threats = %d, want 4; body=%s", len(raw.Threats), recorder.Body.String())
	}
	for _, key := range []string{"pokemonId", "attackTypes", "matchups", "safeMembers", "superEffectiveMembers"} {
		if _, ok := raw.Threats[0][key]; !ok {
			t.Errorf("threat is missing %q", key)
		}
	}
	if _, ok := raw.Threats[0]["abilityId"]; ok {
		t.Errorf("threats[0] has no abilityId in the request, so the key must be omitted; got %s", raw.Threats[0]["abilityId"])
	}
	if got := string(raw.Threats[1]["abilityId"]); got != `"ability-9004"` {
		t.Errorf("threats[1].abilityId = %s, want \"ability-9004\"", got)
	}
	if got := string(raw.Threats[1]["attackTypes"]); got != "[]" {
		t.Errorf("threats[1].attackTypes = %s, want [] (an array, not null)", got)
	}

	var t0, t1 []map[string]json.RawMessage
	if err := json.Unmarshal(raw.Threats[0]["matchups"], &t0); err != nil || len(t0) != 3 {
		t.Fatalf("threats[0].matchups = %s, err = %v", raw.Threats[0]["matchups"], err)
	}
	if err := json.Unmarshal(raw.Threats[1]["matchups"], &t1); err != nil || len(t1) != 3 {
		t.Fatalf("threats[1].matchups = %s, err = %v", raw.Threats[1]["matchups"], err)
	}
	for _, key := range []string{"pokemonId", "incoming", "outgoing", "safe", "superEffective"} {
		if _, ok := t0[0][key]; !ok {
			t.Errorf("matchup is missing %q", key)
		}
	}
	if _, ok := t0[1]["abilityId"]; ok {
		t.Errorf("a matchup has no abilityId field; got %s", t0[1]["abilityId"])
	}
	checks := []struct {
		name string
		got  json.RawMessage
		want string
	}{
		{"threats[0].matchups[0].incoming", t0[0]["incoming"], `"2"`},
		{"threats[0].matchups[2].incoming", t0[2]["incoming"], `"1/4"`},
		{"threats[0].matchups[2].outgoing (member without moves)", t0[2]["outgoing"], `null`},
		{"threats[1].matchups[0].incoming (threat without moves)", t1[0]["incoming"], `null`},
		{"threats[1].matchups[0].outgoing", t1[0]["outgoing"], `"3/2"`},
		{"threats[1].matchups[0].safe", t1[0]["safe"], `false`},
		{"threats[0].matchups[0].superEffective", t0[0]["superEffective"], `true`},
		{"threats[0].safeMembers", raw.Threats[0]["safeMembers"], `2`},
	}
	for _, c := range checks {
		if string(c.got) != c.want {
			t.Errorf("%s = %s, want %s", c.name, c.got, c.want)
		}
	}
}

// ADR-0400 §4: without any moveId the move read model is not needed, and without any
// abilityId the ability read model is not needed.
func TestThreatsWithoutMovesOrAbilitiesNeedsOnlyPokemonTypes(t *testing.T) {
	t.Parallel()

	server := New(Dependencies{TypeChart: testTypeChart(), PokemonTypes: threatPokemonTypes})
	recorder := postThreats(t, server, `{"members":[{"pokemonId":"9002-000","moveIds":[]},{"pokemonId":"9002-000","moveIds":[]}],"threats":[{"pokemonId":"9001-000","moveIds":[]}]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 without the move and ability read models; body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeThreatsResponse(t, recorder.Body.Bytes())
	if len(response.Threats) != 1 || len(response.Threats[0].Matchups) != 2 {
		t.Fatalf("response = %+v, want 1 threat with 2 matchups (duplicated members kept)", response)
	}
	for mi, m := range response.Threats[0].Matchups {
		if m.PokemonId != "9002-000" || m.Incoming != nil || m.Outgoing != nil || m.Safe || m.SuperEffective {
			t.Errorf("matchups[%d] = %+v, want 9002-000 null null false false", mi, m)
		}
	}
	if len(response.Threats[0].AttackTypes) != 0 || response.Threats[0].SafeMembers != 0 || response.Threats[0].SuperEffectiveMembers != 0 {
		t.Errorf("threat = %+v, want no attack types and zero counts", response.Threats[0])
	}
}

func TestThreatsAcceptsBoundaries(t *testing.T) {
	t.Parallel()

	fourMoves := `["move-9001","move-9003","move-9004","` + fortyCharMoveID + `"]`
	entry := `{"pokemonId":"9001-000","moveIds":` + fourMoves + `,"abilityId":"` + fortyCharAbilityID + `"}`
	six := strings.TrimSuffix(strings.Repeat(entry+",", 6), ",")
	one := `{"pokemonId":"9002-000","moveIds":[]}`
	tests := []struct {
		name             string
		body             string
		threats, members int
	}{
		{name: "six members and six threats with four moves and a 40-character abilityId each (duplicates across entries)",
			body: `{"members":[` + six + `],"threats":[` + six + `]}`, threats: 6, members: 6},
		{name: "one member and one threat without moves", body: `{"members":[` + one + `],"threats":[` + one + `]}`, threats: 1, members: 1},
		{name: "the same moveId in a member and a threat", body: `{"members":[{"pokemonId":"9001-000","moveIds":["move-9001"]}],"threats":[{"pokemonId":"9001-000","moveIds":["move-9001"]}]}`, threats: 1, members: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := postThreats(t, newThreatsServer(), tt.body)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
			}
			got := decodeThreatsResponse(t, recorder.Body.Bytes())
			if len(got.Threats) != tt.threats {
				t.Fatalf("threats length = %d, want %d", len(got.Threats), tt.threats)
			}
			for ti, threat := range got.Threats {
				if len(threat.Matchups) != tt.members {
					t.Errorf("threats[%d].matchups length = %d, want %d", ti, len(threat.Matchups), tt.members)
				}
			}
		})
	}
}

func TestThreatsRejectsInvalidBody(t *testing.T) {
	t.Parallel()

	const valid = `{"pokemonId":"9001-000","moveIds":[]}`
	members := func(entry string) string { return `{"members":[` + entry + `],"threats":[` + valid + `]}` }
	threats := func(entry string) string { return `{"members":[` + valid + `],"threats":[` + entry + `]}` }
	withMoves := func(moveIDs string) string { return `{"pokemonId":"9001-000","moveIds":` + moveIDs + `}` }
	withAbility := func(abilityID string) string {
		return `{"pokemonId":"9001-000","moveIds":[],"abilityId":` + abilityID + `}`
	}
	seven := strings.TrimSuffix(strings.Repeat(valid+",", 7), ",")

	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"members":`},
		{name: "trailing JSON", body: members(valid) + ` {}`},
		{name: "not an object", body: `[]`},
		{name: "missing members", body: `{"threats":[` + valid + `]}`},
		{name: "missing threats", body: `{"members":[` + valid + `]}`},
		{name: "empty members", body: `{"members":[],"threats":[` + valid + `]}`},
		{name: "empty threats", body: `{"members":[` + valid + `],"threats":[]}`},
		{name: "seven members", body: `{"members":[` + seven + `],"threats":[` + valid + `]}`},
		{name: "seven threats", body: `{"members":[` + valid + `],"threats":[` + seven + `]}`},
		{name: "malformed member pokemonId", body: members(`{"pokemonId":"pokemon-a","moveIds":[]}`)},
		{name: "malformed threat pokemonId", body: threats(`{"pokemonId":"9001-00","moveIds":[]}`)},
		{name: "missing threat pokemonId", body: threats(`{"moveIds":[]}`)},
		{name: "missing member moveIds", body: members(`{"pokemonId":"9001-000"}`)},
		{name: "null threat moveIds", body: threats(`{"pokemonId":"9001-000","moveIds":null}`)},
		{name: "five member moveIds", body: members(withMoves(`["move-9001","move-9003","move-9004","move-9005","move-9007"]`))},
		{name: "five threat moveIds", body: threats(withMoves(`["move-9001","move-9003","move-9004","move-9005","move-9007"]`))},
		{name: "duplicate moveId within a member", body: members(withMoves(`["move-9001","move-9003","move-9001"]`))},
		{name: "duplicate moveId within a threat", body: threats(withMoves(`["move-9001","move-9001"]`))},
		{name: "uppercase moveId", body: threats(withMoves(`["Move-9001"]`))},
		{name: "empty moveId", body: members(withMoves(`[""]`))},
		{name: "double hyphen moveId", body: threats(withMoves(`["move--9001"]`))},
		{name: "forty-one-character moveId", body: threats(withMoves(`["` + fortyCharMoveID + `a"]`))},
		{name: "moveId is a number", body: members(withMoves(`[9001]`))},
		{name: "empty member abilityId", body: members(withAbility(`""`))},
		{name: "uppercase threat abilityId", body: threats(withAbility(`"Ability-9001"`))},
		{name: "trailing hyphen abilityId", body: threats(withAbility(`"ability-9001-"`))},
		{name: "forty-one-character abilityId", body: threats(withAbility(`"` + fortyCharAbilityID + `a"`))},
		{name: "abilityId is a number", body: members(withAbility(`9001`))},
		{name: "unknown member property", body: members(`{"pokemonId":"9001-000","moveIds":[],"types":["fire"]}`)},
		{name: "unknown threat property", body: threats(`{"pokemonId":"9001-000","moveIds":[],"teraType":"fire"}`)},
		{name: "unknown top-level property", body: `{"members":[` + valid + `],"threats":[` + valid + `],"format":"single"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := postThreats(t, newThreatsServer(), tt.body)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
			}
			if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.InvalidRequest {
				t.Errorf("code = %q, want invalid_request", got.Code)
			}
		})
	}
}

func TestThreatsRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	body := `{"members":[{"pokemonId":"9001-000","moveIds":["` + strings.Repeat("a", maxAnalyzeBodyBytes) + `"]}],"threats":[]}`
	recorder := postThreats(t, newThreatsServer(), body)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413; body=%s", recorder.Code, recorder.Body.String())
	}
	if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.RequestTooLarge {
		t.Errorf("code = %q, want request_too_large", got.Code)
	}
}

func TestThreatsRequiresRequestContext(t *testing.T) {
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
		{name: "blank session", deviceID: "test-device", sessionID: "\t"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, threatsPath, strings.NewReader(`{"members":[{"pokemonId":"9001-000","moveIds":[]}],"threats":[{"pokemonId":"9002-000","moveIds":[]}]}`))
			request.Header.Set("Content-Type", "application/json")
			if tt.deviceID != "" {
				request.Header.Set("X-Device-Id", tt.deviceID)
			}
			if tt.sessionID != "" {
				request.Header.Set("X-Session-Id", tt.sessionID)
			}
			newThreatsServer().ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
			}
			if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.MissingRequestContext {
				t.Errorf("code = %q, want missing_request_context", got.Code)
			}
		})
	}
}

// ADR-0400 §4: header (400) → body (400/413) → read models (503: pokemon absent, or moves absent
// while any moveId is named, or abilities absent while any abilityId is named) → unknown_pokemon →
// unknown_move → unknown_ability (422; members before threats, request order; the first one) → 200.
func TestThreatsDecisionOrder(t *testing.T) {
	t.Parallel()

	full := Dependencies{TypeChart: testTypeChart(), PokemonTypes: threatPokemonTypes, Moves: threatMoves, Abilities: fictionalAbilities}
	noPokemon := Dependencies{TypeChart: testTypeChart(), Moves: threatMoves, Abilities: fictionalAbilities}
	noMoves := Dependencies{TypeChart: testTypeChart(), PokemonTypes: threatPokemonTypes, Abilities: fictionalAbilities}
	noAbilities := Dependencies{TypeChart: testTypeChart(), PokemonTypes: threatPokemonTypes, Moves: threatMoves}
	none := Dependencies{TypeChart: testTypeChart()}

	body := func(members, threats string) string {
		return `{"members":[` + members + `],"threats":[` + threats + `]}`
	}
	const (
		plain         = `{"pokemonId":"9001-000","moveIds":[]}`
		withMove      = `{"pokemonId":"9001-000","moveIds":["move-9001"]}`
		withAbility   = `{"pokemonId":"9001-000","moveIds":[],"abilityId":"ability-9001"}`
		unknownPoke1  = `{"pokemonId":"9998-000","moveIds":[]}`
		unknownPoke2  = `{"pokemonId":"9999-000","moveIds":[]}`
		unknownMove1  = `{"pokemonId":"9001-000","moveIds":["move-9001","move-9998"]}`
		unknownMove2  = `{"pokemonId":"9001-000","moveIds":["move-9999"]}`
		unknownAbil1  = `{"pokemonId":"9001-000","moveIds":[],"abilityId":"ability-9998"}`
		unknownAbil2  = `{"pokemonId":"9001-000","moveIds":[],"abilityId":"ability-9999"}`
		fiveMoves     = `{"pokemonId":"9001-000","moveIds":["move-9001","move-9003","move-9004","move-9005","move-9007"]}`
		unknownAll    = `{"pokemonId":"9001-000","moveIds":["move-9999"],"abilityId":"ability-9999"}`
		unknownPokeMA = `{"pokemonId":"9999-000","moveIds":["move-9999"],"abilityId":"ability-9999"}`
	)
	oversized := `{"members":[{"pokemonId":"9001-000","moveIds":["` + strings.Repeat("a", maxAnalyzeBodyBytes) + `"]}],"threats":[]}`
	tests := []struct {
		name        string
		deps        Dependencies
		noHeaders   bool
		body        string
		wantStatus  int
		wantCode    api.ErrorCode
		wantMessage string
	}{
		{name: "headers before body and read models", deps: none, noHeaders: true, body: body(fiveMoves, plain), wantStatus: http.StatusBadRequest, wantCode: api.MissingRequestContext},
		{name: "invalid member body before missing read models", deps: none, body: body(fiveMoves, plain), wantStatus: http.StatusBadRequest, wantCode: api.InvalidRequest},
		{name: "invalid threat body before missing read models", deps: none, body: body(plain, fiveMoves), wantStatus: http.StatusBadRequest, wantCode: api.InvalidRequest},
		{name: "oversized body before missing read models", deps: none, body: oversized, wantStatus: http.StatusRequestEntityTooLarge, wantCode: api.RequestTooLarge},

		{name: "pokemon read model missing without moves or abilities", deps: noPokemon, body: body(plain, plain), wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},
		{name: "move read model missing with a member moveId", deps: noMoves, body: body(withMove, plain), wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},
		{name: "move read model missing with a threat moveId", deps: noMoves, body: body(plain, withMove), wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},
		{name: "ability read model missing with a member abilityId", deps: noAbilities, body: body(withAbility, plain), wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},
		{name: "ability read model missing with a threat abilityId", deps: noAbilities, body: body(plain, withAbility), wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},
		{name: "missing move read model before unknown pokemon", deps: noMoves, body: body(unknownPoke1, withMove), wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},
		{name: "missing ability read model before unknown pokemon", deps: noAbilities, body: body(unknownPoke1, withAbility), wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},

		{name: "unknown pokemon before unknown move and ability", deps: full, body: body(unknownAll, unknownPokeMA), wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownPokemon, wantMessage: "unknown pokemonId: 9999-000"},
		{name: "unknown pokemon: members before threats", deps: full, body: body(plain+`,`+unknownPoke1, unknownPoke2), wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownPokemon, wantMessage: "unknown pokemonId: 9998-000"},
		{name: "unknown pokemon: the first threat in request order", deps: full, body: body(plain, plain+`,`+unknownPoke2+`,`+unknownPoke1), wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownPokemon, wantMessage: "unknown pokemonId: 9999-000"},
		{name: "unknown move before unknown ability (ability in an earlier member)", deps: full, body: body(unknownAbil1, unknownMove2), wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownMove, wantMessage: "unknown moveId: move-9999"},
		{name: "unknown move: members before threats", deps: full, body: body(plain+`,`+unknownMove1, unknownMove2), wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownMove, wantMessage: "unknown moveId: move-9998"},
		{name: "unknown move: the first threat in request order", deps: full, body: body(withMove, unknownMove2+`,`+unknownMove1), wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownMove, wantMessage: "unknown moveId: move-9999"},
		{name: "unknown ability: members before threats", deps: full, body: body(plain+`,`+unknownAbil1, unknownAbil2), wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownAbility, wantMessage: "unknown abilityId: ability-9998"},
		{name: "unknown ability: the first threat in request order", deps: full, body: body(withAbility, unknownAbil2+`,`+unknownAbil1), wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownAbility, wantMessage: "unknown abilityId: ability-9999"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, threatsPath, strings.NewReader(tt.body))
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

	// All checks pass: 200 with every read model, and without the ones the request does not need.
	for name, deps := range map[string]Dependencies{
		"all read models":                     full,
		"no moves and no moveId":              noMoves,
		"no abilities and no abilityId":       noAbilities,
		"neither moves nor abilities needed":  {TypeChart: testTypeChart(), PokemonTypes: threatPokemonTypes},
		"moves and abilities named, all read": full,
	} {
		requestBody := body(plain, plain)
		if name == "moves and abilities named, all read" {
			requestBody = body(withMove+`,`+withAbility, withMove+`,`+withAbility)
		}
		if recorder := postThreats(t, New(deps), requestBody); recorder.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200; body=%s", name, recorder.Code, recorder.Body.String())
		}
	}
}

// ADR-0400 §4 / ADR-0014 §5.6: 422 messages are built from the request ID, never from adapter detail.
func TestThreatsUnknownMessagesDoNotLeakAdapterDetail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		deps        Dependencies
		body        string
		wantCode    api.ErrorCode
		wantMessage string
	}{
		{
			name:        "unknown pokemon",
			deps:        Dependencies{TypeChart: testTypeChart(), PokemonTypes: failingPokemonTypes{err: fmt.Errorf("%w: 9999-000 (read from /secret/pokemon.json)", balance.ErrUnknownPokemon)}},
			body:        `{"members":[{"pokemonId":"9999-000","moveIds":[]}],"threats":[{"pokemonId":"9999-000","moveIds":[]}]}`,
			wantCode:    api.UnknownPokemon,
			wantMessage: "unknown pokemonId: 9999-000",
		},
		{
			name:        "unknown move",
			deps:        Dependencies{TypeChart: testTypeChart(), PokemonTypes: threatPokemonTypes, Moves: failingMoves{err: fmt.Errorf("%w: move-9999 (read from /secret/moves.json)", balance.ErrUnknownMove)}},
			body:        `{"members":[{"pokemonId":"9001-000","moveIds":[]}],"threats":[{"pokemonId":"9002-000","moveIds":["move-9999"]}]}`,
			wantCode:    api.UnknownMove,
			wantMessage: "unknown moveId: move-9999",
		},
		{
			name:        "unknown ability",
			deps:        Dependencies{TypeChart: testTypeChart(), PokemonTypes: threatPokemonTypes, Abilities: failingAbilities{err: fmt.Errorf("%w: ability-9999 (read from /secret/abilities.json)", balance.ErrUnknownAbility)}},
			body:        `{"members":[{"pokemonId":"9001-000","moveIds":[]}],"threats":[{"pokemonId":"9002-000","moveIds":[],"abilityId":"ability-9999"}]}`,
			wantCode:    api.UnknownAbility,
			wantMessage: "unknown abilityId: ability-9999",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := postThreats(t, New(tt.deps), tt.body)
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422; body=%s", recorder.Code, recorder.Body.String())
			}
			got := decodeError(t, recorder.Body.Bytes())
			if got.Code != tt.wantCode || got.Message != tt.wantMessage {
				t.Errorf("error = %+v, want %s %q", got, tt.wantCode, tt.wantMessage)
			}
		})
	}
}

// ADR-0400 §4: every other failure is 500 with the fixed text, including a multiplier overflow.
func TestThreatsInternalErrors(t *testing.T) {
	t.Parallel()

	// Both sides have attack moves, so the multipliers are always computed.
	const defaultBody = `{"members":[{"pokemonId":"9002-000","moveIds":["move-9001"],"abilityId":"ability-9001"}],"threats":[{"pokemonId":"9001-000","moveIds":["move-9001"],"abilityId":"ability-9001"}]}`
	tests := []struct {
		name string
		deps Dependencies
		body string
	}{
		{name: "pokemon provider failure", deps: Dependencies{TypeChart: testTypeChart(), PokemonTypes: failingPokemonTypes{err: errors.New("pokemon backend exploded at /secret/pokemon.json")}, Moves: threatMoves, Abilities: fictionalAbilities}},
		{name: "move provider failure", deps: Dependencies{TypeChart: testTypeChart(), PokemonTypes: threatPokemonTypes, Moves: failingMoves{err: errors.New("move backend exploded at /secret/moves.json")}, Abilities: fictionalAbilities}},
		{name: "ability provider failure", deps: Dependencies{TypeChart: testTypeChart(), PokemonTypes: threatPokemonTypes, Moves: threatMoves, Abilities: failingAbilities{err: errors.New("ability backend exploded at /secret/abilities.json")}}},
		{name: "nil type chart", deps: Dependencies{PokemonTypes: threatPokemonTypes, Moves: threatMoves, Abilities: fictionalAbilities}},
		{name: "invalid ability effect from provider", deps: Dependencies{TypeChart: testTypeChart(), PokemonTypes: threatPokemonTypes, Moves: threatMoves, Abilities: testAbilities{
			"ability-9001": fictionalAbility("ability-9001", balance.AbilityEffect{Kind: "heal", AttackType: balance.TypeFire}),
		}}},
		{
			// 9002-000 grass takes fire x2; sixteen stacked super effective 16/1 overflow int64 (member ability, incoming).
			name: "multiplier overflow on incoming",
			deps: Dependencies{TypeChart: testTypeChart(), PokemonTypes: threatPokemonTypes, Moves: threatMoves, Abilities: testAbilities{
				"ability-9001": fictionalAbility("ability-9001", stackedSuperEffective(16)...),
			}},
			body: `{"members":[{"pokemonId":"9002-000","moveIds":[],"abilityId":"ability-9001"}],"threats":[{"pokemonId":"9001-000","moveIds":["move-9001"]}]}`,
		},
		{
			// The same ability on the threat (outgoing).
			name: "multiplier overflow on outgoing",
			deps: Dependencies{TypeChart: testTypeChart(), PokemonTypes: threatPokemonTypes, Moves: threatMoves, Abilities: testAbilities{
				"ability-9001": fictionalAbility("ability-9001", stackedSuperEffective(16)...),
			}},
			body: `{"members":[{"pokemonId":"9001-000","moveIds":["move-9001"]}],"threats":[{"pokemonId":"9002-000","moveIds":[],"abilityId":"ability-9001"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := tt.body
			if body == "" {
				body = defaultBody
			}
			recorder := postThreats(t, New(tt.deps), body)
			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500; body=%s", recorder.Code, recorder.Body.String())
			}
			got := decodeError(t, recorder.Body.Bytes())
			if got.Code != api.InternalError || got.Message != "internal error" {
				t.Errorf("error = %+v, want internal_error with the fixed text %q", got, "internal error")
			}
		})
	}
}

// Adding the threats endpoint does not change health, analyze or coverage.
func TestThreatsDoesNotAffectOtherEndpoints(t *testing.T) {
	t.Parallel()

	server := newThreatsServer()
	for _, path := range []string{"/healthz", "/api/balance/healthz"} {
		health := httptest.NewRecorder()
		server.ServeHTTP(health, httptest.NewRequest(http.MethodGet, path, nil))
		if health.Code != http.StatusOK {
			t.Errorf("%s status = %d, want 200", path, health.Code)
		}
	}
	if recorder := postAnalyze(t, server, `{"members":[{"pokemonId":"9001-000"}]}`); recorder.Code != http.StatusOK {
		t.Errorf("analyze status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder := postCoverage(t, server, `{"members":[{"pokemonId":"9001-000","moveIds":["move-9001"]}]}`); recorder.Code != http.StatusOK {
		t.Errorf("coverage status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	// A threats-only field is still unknown to analyze and coverage.
	if recorder := postCoverage(t, server, `{"members":[{"pokemonId":"9001-000","moveIds":[]}],"threats":[]}`); recorder.Code != http.StatusBadRequest {
		t.Errorf("coverage with threats: status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
	}
}

// ADR-0400 §6.1: a provider that returns an invalid move classification is a 500, not a
// silent exclusion (mirrors the core AnalyzeThreats validation, ADR-0016 §6).
func TestThreatsInvalidMoveCategoryFromProviderIsInternalError(t *testing.T) {
	t.Parallel()

	deps := Dependencies{
		TypeChart:    testTypeChart(),
		PokemonTypes: threatPokemonTypes,
		Moves:        testMoves{"move-9001": {MoveID: "move-9001", Type: balance.TypeFire, Category: "other"}},
		Abilities:    fictionalAbilities,
	}
	recorder := postThreats(t, New(deps), `{"members":[{"pokemonId":"9001-000","moveIds":["move-9001"]}],"threats":[{"pokemonId":"9002-000","moveIds":[]}]}`)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", recorder.Code, recorder.Body.String())
	}
	if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.InternalError || got.Message != "internal error" {
		t.Errorf("error = %+v, want internal_error with the fixed text", got)
	}
}

// ADR-0400 §6.4: request abilityId: null decodes the same as an omitted field (valid, no
// ability, response abilityId key omitted), for both a member and a threat.
func TestThreatsNullAbilityIdIsTreatedAsOmitted(t *testing.T) {
	t.Parallel()

	body := `{"members":[{"pokemonId":"9001-000","moveIds":[],"abilityId":null}],"threats":[{"pokemonId":"9002-000","moveIds":[],"abilityId":null}]}`
	recorder := postThreats(t, newThreatsServer(), body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	var raw struct {
		Threats []map[string]json.RawMessage `json:"threats"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(raw.Threats) != 1 {
		t.Fatalf("threats = %d, want 1; body=%s", len(raw.Threats), recorder.Body.String())
	}
	if _, ok := raw.Threats[0]["abilityId"]; ok {
		t.Errorf("threats[0] has an explicit null abilityId in the request, so the key must be omitted; got %s", raw.Threats[0]["abilityId"])
	}
}

// The smoke fixture (scripts/smoke.sh) holds with the example read models.
func TestThreatsWithExampleReadModels(t *testing.T) {
	t.Parallel()

	pokemon, err := master.LoadPokemonTypesFile("../../testdata/pokemon-types.example.json")
	if err != nil || pokemon == nil {
		t.Fatalf("LoadPokemonTypesFile(example) = %v, %v", pokemon, err)
	}
	moves, err := master.LoadMovesFile("../../testdata/moves.example.json")
	if err != nil || moves == nil {
		t.Fatalf("LoadMovesFile(example) = %v, %v", moves, err)
	}
	abilities, err := master.LoadAbilitiesFile("../../testdata/abilities.example.json")
	if err != nil || abilities == nil {
		t.Fatalf("LoadAbilitiesFile(example) = %v, %v", abilities, err)
	}
	server := New(Dependencies{TypeChart: testTypeChart(), PokemonTypes: pokemon, Moves: moves, Abilities: abilities})

	// 9002-000 grass with a fire move and 9003-000 water/ground with ability-9004 and no moves,
	// against 9005-000 ice with an ice move: incoming x2 and x1, outgoing x2 and null.
	recorder := postThreats(t, server, `{"members":[{"pokemonId":"9002-000","moveIds":["move-9001"]},{"pokemonId":"9003-000","moveIds":[],"abilityId":"ability-9004"}],"threats":[{"pokemonId":"9005-000","moveIds":["move-9008"]}]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	for _, key := range []string{`"threats"`, `"matchups"`, `"incoming":"2"`, `"outgoing":"2"`, `"incoming":"1"`, `"outgoing":null`, `"safeMembers":0`, `"superEffectiveMembers":1`} {
		if !strings.Contains(recorder.Body.String(), key) {
			t.Errorf("body is missing %s; body=%s", key, recorder.Body.String())
		}
	}
	response := decodeThreatsResponse(t, recorder.Body.Bytes())
	if len(response.Threats) != 1 || fmt.Sprint(response.Threats[0].AttackTypes) != fmt.Sprint([]api.TypeId{api.Ice}) {
		t.Errorf("threats = %+v, want one ice attacker", response.Threats)
	}

	unknown := postThreats(t, server, `{"members":[{"pokemonId":"9002-000","moveIds":[]}],"threats":[{"pokemonId":"9005-000","moveIds":["move-9999"]}]}`)
	if unknown.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown move: status = %d, want 422; body=%s", unknown.Code, unknown.Body.String())
	}
	if got := decodeError(t, unknown.Body.Bytes()); got.Code != api.UnknownMove || got.Message != "unknown moveId: move-9999" {
		t.Errorf("unknown move: error = %+v", got)
	}
}
