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
	"github.com/labstack/echo/v5"
)

// TB5 おすすめタイプと該当ポケモンの HTTP 契約(ADR-0401)。ID と名前はすべて架空。
//
// 期待値を手で数えられるように、既定が ×1 の架空の相性表(recHTTPChart)を使う:
//
//	normal は fire / water 以外を ×1/2 で受ける(メンバーは normal なので防御の穴は [fire, water])。
//	fire → rock ×1/2、water → grass ×1/2、fire → grass ×2、normal → steel ×1/2。
//	メンバーの技は normal だけ(normal → normal ×1/2・steel ×1/2 なので攻撃範囲の穴は [normal, steel])。
//	normal を ×1 以上で打てるのは fire と water だけ、steel を ×1 以上で打てるのは normal 以外。

const recommendationsPath = "/api/balance/v1/team-balance/recommendations"

// recHTTPChart is a fictional type chart: every matchup not listed is x1.
type recHTTPChart map[[2]balance.TypeID]balance.Multiplier

func (c recHTTPChart) Matchup(attack, defense balance.TypeID) (balance.Multiplier, error) {
	if m, ok := c[[2]balance.TypeID{attack, defense}]; ok {
		return m, nil
	}
	return balance.MultiplierNormal, nil
}

func recommendationsChart() recHTTPChart {
	c := recHTTPChart{}
	for _, attack := range balance.AllTypes() {
		if attack != balance.TypeFire && attack != balance.TypeWater {
			c[[2]balance.TypeID{attack, balance.TypeNormal}] = balance.MultiplierHalf
		}
	}
	c[[2]balance.TypeID{balance.TypeFire, balance.TypeRock}] = balance.MultiplierHalf
	c[[2]balance.TypeID{balance.TypeWater, balance.TypeGrass}] = balance.MultiplierHalf
	c[[2]balance.TypeID{balance.TypeFire, balance.TypeGrass}] = balance.MultiplierDouble
	c[[2]balance.TypeID{balance.TypeNormal, balance.TypeSteel}] = balance.MultiplierHalf
	return c
}

// testCatalog is a fictional in-memory PokemonCatalog.
type testCatalog []balance.CatalogPokemon

func (c testCatalog) AllPokemon() ([]balance.CatalogPokemon, error) {
	out := make([]balance.CatalogPokemon, len(c))
	for i, p := range c {
		out[i] = balance.CatalogPokemon{
			PokemonID:  p.PokemonID,
			NameJa:     p.NameJa,
			Types:      append([]balance.TypeID(nil), p.Types...),
			AbilityIDs: append([]string(nil), p.AbilityIDs...),
		}
	}
	return out, nil
}

// types returns the pokemon type read model view of the same entries.
func (c testCatalog) types() testPokemonTypes {
	p := testPokemonTypes{}
	for _, entry := range c {
		p[entry.PokemonID] = entry.Types
	}
	return p
}

type failingCatalog struct{ err error }

func (f failingCatalog) AllPokemon() ([]balance.CatalogPokemon, error) { return nil, f.err }

var recCatalog = testCatalog{
	{PokemonID: "9001-000", NameJa: "テストノーマル", Types: []balance.TypeID{balance.TypeNormal}, AbilityIDs: []string{}},
	{PokemonID: "9002-000", NameJa: "テストイワ", Types: []balance.TypeID{balance.TypeRock}},
	{PokemonID: "9003-000", Types: []balance.TypeID{balance.TypeRock, balance.TypeFire}},
	{PokemonID: "9002-001", NameJa: "テストヨウガン", Types: []balance.TypeID{balance.TypeFire, balance.TypeRock}},
	{PokemonID: "9004-000", Types: []balance.TypeID{balance.TypeGrass}, AbilityIDs: []string{"ability-9001"}},
	{PokemonID: "9005-000", Types: []balance.TypeID{balance.TypeNormal}, AbilityIDs: []string{"ability-9003"}},
	{PokemonID: "9006-000", NameJa: "テストホノオ", Types: []balance.TypeID{balance.TypeFire}, AbilityIDs: []string{"ability-9002"}},
}

var recAbilities = testAbilities{
	"ability-9001": fictionalAbility("ability-9001", balance.AbilityEffect{Kind: balance.AbilityEffectImmune, AttackType: balance.TypeFire}),
	"ability-9002": fictionalAbility("ability-9002", balance.AbilityEffect{Kind: balance.AbilityEffectAbsorb, AttackType: balance.TypeWater}),
	"ability-9003": fictionalAbility("ability-9003", balance.AbilityEffect{Kind: balance.AbilityEffectTypeMultiplier, AttackType: balance.TypeFire, Factor: fraction(1, 2)}),
}

var recMoves = testMoves{
	"move-9001":     fictionalMove("move-9001", balance.TypeFire, balance.MoveCategorySpecial),
	"move-9003":     fictionalMove("move-9003", balance.TypeWater, balance.MoveCategorySpecial),
	"move-9004":     fictionalMove("move-9004", balance.TypeElectric, balance.MoveCategorySpecial),
	"move-9005":     fictionalMove("move-9005", balance.TypeNormal, balance.MoveCategoryPhysical),
	"move-9007":     fictionalMove("move-9007", balance.TypeGround, balance.MoveCategoryPhysical),
	fortyCharMoveID: fictionalMove(fortyCharMoveID, balance.TypeIce, balance.MoveCategorySpecial),
}

func recFullDependencies() Dependencies {
	return Dependencies{
		TypeChart:      recommendationsChart(),
		PokemonTypes:   recCatalog.types(),
		PokemonCatalog: recCatalog,
		Moves:          recMoves,
		Abilities:      recAbilities,
	}
}

func newRecommendationsServer() *echo.Echo {
	return New(recFullDependencies())
}

func postRecommendations(t *testing.T, server http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, recommendationsPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Device-Id", "test-device")
	request.Header.Set("X-Session-Id", "test-session")
	server.ServeHTTP(recorder, request)
	return recorder
}

// decodeRecommendationsResponse decodes into the generated type with unknown fields rejected.
func decodeRecommendationsResponse(t *testing.T, body []byte) api.RecommendationsResponse {
	t.Helper()
	var response api.RecommendationsResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("decode RecommendationsResponse: %v; body=%s", err, body)
	}
	return response
}

func apiTypeLabel(ts []api.TypeId) string {
	parts := make([]string, len(ts))
	for i, t := range ts {
		parts[i] = string(t)
	}
	return strings.Join(parts, "/")
}

// recBody is the shared 200 fixture: one normal member with a normal move, no limit (default 10).
const recBody = `{"members":[{"pokemonId":"9001-000","moveIds":["move-9005"]}]}`

func TestRecommendationsResponseBody(t *testing.T) {
	t.Parallel()

	recorder := postRecommendations(t, newRecommendationsServer(), recBody)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/json") {
		t.Errorf("Content-Type = %q, want application/json", recorder.Header().Get("Content-Type"))
	}
	response := decodeRecommendationsResponse(t, recorder.Body.Bytes())

	if got := apiTypeLabel(response.DefenseHoles); got != "fire/water" {
		t.Errorf("defenseHoles = %s, want fire/water", got)
	}
	if got := apiTypeLabel(response.OffenseHoles); got != "normal/steel" {
		t.Errorf("offenseHoles = %s, want normal/steel", got)
	}

	type pokemon struct {
		id, name, types string
		exact           bool
	}
	want := []struct {
		types, defense, offense string
		weaknesses              int
		pokemon                 []pokemon
	}{
		// defenseCovered + offenseCovered = 3, weaknesses 0 (canonical order).
		{"fire/rock", "fire", "normal/steel", 0, []pokemon{{"9002-001", "テストヨウガン", "fire/rock", true}, {"9003-000", "", "rock/fire", true}}},
		{"water/rock", "fire", "normal/steel", 0, []pokemon{}},
		// 3, weaknesses 1 (fire hits grass x2).
		{"fire/grass", "water", "normal/steel", 1, []pokemon{}},
		{"water/grass", "water", "normal/steel", 1, []pokemon{}},
		// 2, weaknesses 0: single types first, then duals in canonical order.
		// ADR-0401 §8: single types also list the pokemon containing them (exact matches first).
		// fire covers no defense hole, so every fire pokemon stays; rock/fire takes fire x1/4, so it stays under rock.
		{"fire", "", "normal/steel", 0, []pokemon{{"9006-000", "テストホノオ", "fire", true}, {"9002-001", "テストヨウガン", "fire/rock", false}, {"9003-000", "", "rock/fire", false}}},
		{"water", "", "normal/steel", 0, []pokemon{}},
		{"rock", "fire", "steel", 0, []pokemon{{"9002-000", "テストイワ", "rock", true}, {"9002-001", "テストヨウガン", "fire/rock", false}, {"9003-000", "", "rock/fire", false}}},
		{"normal/fire", "", "normal/steel", 0, []pokemon{}},
		{"normal/water", "", "normal/steel", 0, []pokemon{}},
		{"normal/rock", "fire", "steel", 0, []pokemon{}},
	}
	if len(response.Candidates) != len(want) {
		labels := make([]string, len(response.Candidates))
		for i, c := range response.Candidates {
			labels[i] = apiTypeLabel(c.Types)
		}
		t.Fatalf("candidates = %v, want %d (default limit 10)", labels, len(want))
	}
	for i, w := range want {
		c := response.Candidates[i]
		if apiTypeLabel(c.Types) != w.types || apiTypeLabel(c.DefenseCovered) != w.defense || apiTypeLabel(c.OffenseCovered) != w.offense || c.Weaknesses != w.weaknesses {
			t.Errorf("candidates[%d] = %s def=%s off=%s w=%d, want %s def=%s off=%s w=%d", i,
				apiTypeLabel(c.Types), apiTypeLabel(c.DefenseCovered), apiTypeLabel(c.OffenseCovered), c.Weaknesses,
				w.types, w.defense, w.offense, w.weaknesses)
		}
		got := make([]pokemon, len(c.Pokemon))
		for j, p := range c.Pokemon {
			got[j] = pokemon{p.PokemonId, deref(p.NameJa), apiTypeLabel(p.Types), p.ExactMatch}
		}
		wantPokemon := make([]pokemon, len(w.pokemon))
		for j, p := range w.pokemon {
			if p.name == "" {
				p.name = "<absent>"
			}
			wantPokemon[j] = p
		}
		if fmt.Sprint(got) != fmt.Sprint(wantPokemon) {
			t.Errorf("candidates[%d] (%s) pokemon = %v, want %v", i, w.types, got, wantPokemon)
		}
	}

	type option struct{ id, name, ability, multiplier string }
	wantOptions := []struct {
		attack  api.TypeId
		pokemon []option
	}{
		{api.Fire, []option{{"9004-000", "<absent>", "ability-9001", "0"}, {"9005-000", "<absent>", "ability-9003", "1/2"}}},
		{api.Water, []option{{"9006-000", "テストホノオ", "ability-9002", "0"}}},
	}
	if len(response.AbilityOptions) != len(wantOptions) {
		t.Fatalf("abilityOptions = %+v, want one entry per defense hole", response.AbilityOptions)
	}
	for i, w := range wantOptions {
		o := response.AbilityOptions[i]
		got := make([]option, len(o.Pokemon))
		for j, p := range o.Pokemon {
			got[j] = option{p.PokemonId, deref(p.NameJa), p.AbilityId, p.Multiplier}
		}
		if o.AttackType != w.attack || fmt.Sprint(got) != fmt.Sprint(w.pokemon) {
			t.Errorf("abilityOptions[%d] = %s %v, want %s %v", i, o.AttackType, got, w.attack, w.pokemon)
		}
	}
}

// Field names, omitted nameJa, and empty arrays written as [] (never null).
func TestRecommendationsResponseUsesContractFieldNames(t *testing.T) {
	t.Parallel()

	recorder := postRecommendations(t, newRecommendationsServer(), recBody)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"defenseHoles", "offenseHoles", "candidates", "abilityOptions"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("response is missing %q", key)
		}
	}
	var candidates []map[string]json.RawMessage
	if err := json.Unmarshal(raw["candidates"], &candidates); err != nil || len(candidates) < 5 {
		t.Fatalf("candidates = %s, err = %v", raw["candidates"], err)
	}
	for _, key := range []string{"types", "defenseCovered", "offenseCovered", "weaknesses", "pokemon"} {
		if _, ok := candidates[0][key]; !ok {
			t.Errorf("candidate is missing %q", key)
		}
	}
	if got := string(candidates[1]["pokemon"]); got != "[]" {
		t.Errorf("candidates[1].pokemon = %s, want [] (an array, not null)", got)
	}
	if got := string(candidates[4]["defenseCovered"]); got != "[]" {
		t.Errorf("candidates[4].defenseCovered = %s, want []", got)
	}
	var firstRock []map[string]json.RawMessage
	if err := json.Unmarshal(candidates[0]["pokemon"], &firstRock); err != nil || len(firstRock) != 2 {
		t.Fatalf("candidates[0].pokemon = %s, err = %v", candidates[0]["pokemon"], err)
	}
	if got := string(firstRock[0]["nameJa"]); got != `"テストヨウガン"` {
		t.Errorf("candidates[0].pokemon[0].nameJa = %s, want \"テストヨウガン\"", got)
	}
	if _, ok := firstRock[1]["nameJa"]; ok {
		t.Errorf("candidates[0].pokemon[1] has no nameJa in the read model, so the key must be omitted; got %s", firstRock[1]["nameJa"])
	}
	if got := string(firstRock[1]["types"]); got != `["rock","fire"]` {
		t.Errorf("candidates[0].pokemon[1].types = %s, want the read model order [\"rock\",\"fire\"]", got)
	}

	// No move at all: offenseHoles is [] and no candidate has offense coverage.
	noMoves := postRecommendations(t, newRecommendationsServer(), `{"members":[{"pokemonId":"9001-000","moveIds":[]}]}`)
	if noMoves.Code != http.StatusOK {
		t.Fatalf("no moves: status = %d, want 200; body=%s", noMoves.Code, noMoves.Body.String())
	}
	var rawNoMoves map[string]json.RawMessage
	if err := json.Unmarshal(noMoves.Body.Bytes(), &rawNoMoves); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := string(rawNoMoves["offenseHoles"]); got != "[]" {
		t.Errorf("offenseHoles without moves = %s, want []", got)
	}
}

// ADR-0401 §2: the members' abilities are applied to the defense holes.
func TestRecommendationsAppliesMemberAbility(t *testing.T) {
	t.Parallel()

	recorder := postRecommendations(t, newRecommendationsServer(), `{"members":[{"pokemonId":"9001-000","moveIds":[],"abilityId":"ability-9001"}]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeRecommendationsResponse(t, recorder.Body.Bytes())
	if got := apiTypeLabel(response.DefenseHoles); got != "water" {
		t.Errorf("defenseHoles = %s, want water (ability-9001 makes the member immune to fire)", got)
	}
	if len(response.AbilityOptions) != 1 || response.AbilityOptions[0].AttackType != api.Water {
		t.Errorf("abilityOptions = %+v, want one entry for water", response.AbilityOptions)
	}
}

// ADR-0401 §3: limit 1..20, default 10; 0 and 21 are 400.
func TestRecommendationsLimit(t *testing.T) {
	t.Parallel()

	labels := func(r api.RecommendationsResponse) []string {
		out := make([]string, len(r.Candidates))
		for i, c := range r.Candidates {
			out[i] = apiTypeLabel(c.Types)
		}
		return out
	}
	withLimit := func(limit string) string {
		return `{"members":[{"pokemonId":"9001-000","moveIds":["move-9005"]}],"limit":` + limit + `}`
	}

	full := postRecommendations(t, newRecommendationsServer(), withLimit("20"))
	if full.Code != http.StatusOK {
		t.Fatalf("limit 20: status = %d, want 200; body=%s", full.Code, full.Body.String())
	}
	fullLabels := labels(decodeRecommendationsResponse(t, full.Body.Bytes()))
	if len(fullLabels) != 20 {
		t.Fatalf("limit 20: %d candidates, want 20", len(fullLabels))
	}
	for _, tt := range []struct {
		body string
		want int
	}{
		{withLimit("1"), 1},
		{withLimit("10"), 10},
		{withLimit("19"), 19},
		{recBody, 10},
	} {
		recorder := postRecommendations(t, newRecommendationsServer(), tt.body)
		if recorder.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200; body=%s", tt.body, recorder.Code, recorder.Body.String())
			continue
		}
		got := labels(decodeRecommendationsResponse(t, recorder.Body.Bytes()))
		if fmt.Sprint(got) != fmt.Sprint(fullLabels[:tt.want]) {
			t.Errorf("%s: candidates = %v, want %v", tt.body, got, fullLabels[:tt.want])
		}
	}

	for _, limit := range []string{"0", "21", "-1", "1.5", `"5"`} {
		recorder := postRecommendations(t, newRecommendationsServer(), withLimit(limit))
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("limit %s: status = %d, want 400; body=%s", limit, recorder.Code, recorder.Body.String())
			continue
		}
		if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.InvalidRequest {
			t.Errorf("limit %s: code = %q, want invalid_request", limit, got.Code)
		}
	}
}

// ADR-0401 §7.5: an explicit "limit": null is the same as the field being omitted (default 10).
func TestRecommendationsLimitNullIsDefault(t *testing.T) {
	t.Parallel()

	withNull := postRecommendations(t, newRecommendationsServer(), `{"members":[{"pokemonId":"9001-000","moveIds":["move-9005"]}],"limit":null}`)
	if withNull.Code != http.StatusOK {
		t.Fatalf("limit null: status = %d, want 200; body=%s", withNull.Code, withNull.Body.String())
	}
	omitted := postRecommendations(t, newRecommendationsServer(), recBody)
	if omitted.Code != http.StatusOK {
		t.Fatalf("omitted limit: status = %d, want 200; body=%s", omitted.Code, omitted.Body.String())
	}
	if withNull.Body.String() != omitted.Body.String() {
		t.Errorf("limit null response differs from omitted limit:\nnull    %s\nomitted %s", withNull.Body.String(), omitted.Body.String())
	}
}

// ADR-0401 §6: without the ability read model (and no abilityId) the response is 200 with
// abilityOptions [], and everything else is unchanged.
func TestRecommendationsWithoutAbilityReadModel(t *testing.T) {
	t.Parallel()

	deps := recFullDependencies()
	deps.Abilities = nil
	recorder := postRecommendations(t, New(deps), recBody)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 without the ability read model; body=%s", recorder.Code, recorder.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := string(raw["abilityOptions"]); got != "[]" {
		t.Errorf("abilityOptions = %s, want []", got)
	}

	with := postRecommendations(t, newRecommendationsServer(), recBody)
	if with.Code != http.StatusOK {
		t.Fatalf("with abilities: status = %d; body=%s", with.Code, with.Body.String())
	}
	a := decodeRecommendationsResponse(t, recorder.Body.Bytes())
	b := decodeRecommendationsResponse(t, with.Body.Bytes())
	a.AbilityOptions, b.AbilityOptions = nil, nil
	aJSON, errA := json.Marshal(a)
	bJSON, errB := json.Marshal(b)
	if errA != nil || errB != nil || string(aJSON) != string(bJSON) {
		t.Errorf("holes/candidates depend on the ability read model:\nwithout %s\nwith    %s", aJSON, bJSON)
	}
}

// Without any moveId the move read model is not needed (TB4 と同じ流儀、ADR-0401 §6).
func TestRecommendationsWithoutMovesNeedsNoMoveReadModel(t *testing.T) {
	t.Parallel()

	deps := recFullDependencies()
	deps.Moves = nil
	recorder := postRecommendations(t, New(deps), `{"members":[{"pokemonId":"9001-000","moveIds":[]},{"pokemonId":"9001-000","moveIds":[]}]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 without the move read model; body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeRecommendationsResponse(t, recorder.Body.Bytes())
	if len(response.OffenseHoles) != 0 || apiTypeLabel(response.DefenseHoles) != "fire/water" {
		t.Errorf("holes = %v / %v, want fire/water and none", response.DefenseHoles, response.OffenseHoles)
	}
}

func TestRecommendationsAcceptsBoundaries(t *testing.T) {
	t.Parallel()

	fourMoves := `["move-9001","move-9003","move-9004","` + fortyCharMoveID + `"]`
	entry := `{"pokemonId":"9001-000","moveIds":` + fourMoves + `,"abilityId":"` + fortyCharAbilityID + `"}`
	six := strings.TrimSuffix(strings.Repeat(entry+",", 6), ",")
	deps := recFullDependencies()
	abilities := testAbilities{fortyCharAbilityID: fictionalAbility(fortyCharAbilityID)}
	for id, a := range recAbilities {
		abilities[id] = a
	}
	deps.Abilities = abilities
	for name, body := range map[string]string{
		"six members with four moves and a 40-character abilityId each": `{"members":[` + six + `],"limit":20}`,
		"one member without moves, limit 1":                             `{"members":[{"pokemonId":"9002-000","moveIds":[]}],"limit":1}`,
		"null abilityId is the same as omitted":                         `{"members":[{"pokemonId":"9001-000","moveIds":[],"abilityId":null}]}`,
	} {
		recorder := postRecommendations(t, New(deps), body)
		if recorder.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200; body=%s", name, recorder.Code, recorder.Body.String())
			continue
		}
		decodeRecommendationsResponse(t, recorder.Body.Bytes())
	}
}

func TestRecommendationsRejectsInvalidBody(t *testing.T) {
	t.Parallel()

	const valid = `{"pokemonId":"9001-000","moveIds":[]}`
	members := func(entry string) string { return `{"members":[` + entry + `]}` }
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
		{name: "missing members", body: `{"limit":10}`},
		{name: "empty members", body: `{"members":[]}`},
		{name: "seven members", body: members(seven)},
		{name: "malformed pokemonId", body: members(`{"pokemonId":"9001-00","moveIds":[]}`)},
		{name: "missing pokemonId", body: members(`{"moveIds":[]}`)},
		{name: "missing moveIds", body: members(`{"pokemonId":"9001-000"}`)},
		{name: "null moveIds", body: members(`{"pokemonId":"9001-000","moveIds":null}`)},
		{name: "five moveIds", body: members(withMoves(`["move-9001","move-9003","move-9004","move-9005","move-9007"]`))},
		{name: "duplicate moveId within a member", body: members(withMoves(`["move-9001","move-9001"]`))},
		{name: "uppercase moveId", body: members(withMoves(`["Move-9001"]`))},
		{name: "forty-one-character moveId", body: members(withMoves(`["` + fortyCharMoveID + `a"]`))},
		{name: "empty abilityId", body: members(withAbility(`""`))},
		{name: "uppercase abilityId", body: members(withAbility(`"Ability-9001"`))},
		{name: "forty-one-character abilityId", body: members(withAbility(`"` + fortyCharAbilityID + `a"`))},
		{name: "limit 0", body: `{"members":[` + valid + `],"limit":0}`},
		{name: "limit 21", body: `{"members":[` + valid + `],"limit":21}`},
		{name: "limit is a string", body: `{"members":[` + valid + `],"limit":"10"}`},
		{name: "unknown member property", body: members(`{"pokemonId":"9001-000","moveIds":[],"types":["fire"]}`)},
		{name: "unknown top-level property", body: `{"members":[` + valid + `],"threats":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := postRecommendations(t, newRecommendationsServer(), tt.body)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
			}
			if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.InvalidRequest {
				t.Errorf("code = %q, want invalid_request", got.Code)
			}
		})
	}
}

func TestRecommendationsRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	body := `{"members":[{"pokemonId":"9001-000","moveIds":["` + strings.Repeat("a", maxAnalyzeBodyBytes) + `"]}]}`
	recorder := postRecommendations(t, newRecommendationsServer(), body)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413; body=%s", recorder.Code, recorder.Body.String())
	}
	if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.RequestTooLarge {
		t.Errorf("code = %q, want request_too_large", got.Code)
	}
}

func TestRecommendationsRequiresRequestContext(t *testing.T) {
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
			request := httptest.NewRequest(http.MethodPost, recommendationsPath, strings.NewReader(recBody))
			request.Header.Set("Content-Type", "application/json")
			if tt.deviceID != "" {
				request.Header.Set("X-Device-Id", tt.deviceID)
			}
			if tt.sessionID != "" {
				request.Header.Set("X-Session-Id", tt.sessionID)
			}
			newRecommendationsServer().ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
			}
			if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.MissingRequestContext {
				t.Errorf("code = %q, want missing_request_context", got.Code)
			}
		})
	}
}

// ADR-0401 §6 (TB4 と同じ流儀): header (400) → body (400/413) → read models (503: pokemon types or
// catalog absent, moves absent while any moveId is named, abilities absent while any abilityId is
// named) → unknown_pokemon → unknown_move → unknown_ability (422; request order, the first one) → 200.
func TestRecommendationsDecisionOrder(t *testing.T) {
	t.Parallel()

	full := recFullDependencies()
	without := func(mutate func(*Dependencies)) Dependencies {
		d := recFullDependencies()
		mutate(&d)
		return d
	}
	noPokemon := without(func(d *Dependencies) { d.PokemonTypes = nil })
	noCatalog := without(func(d *Dependencies) { d.PokemonCatalog = nil })
	noMoves := without(func(d *Dependencies) { d.Moves = nil })
	noAbilities := without(func(d *Dependencies) { d.Abilities = nil })
	none := Dependencies{TypeChart: recommendationsChart()}

	body := func(members ...string) string { return `{"members":[` + strings.Join(members, ",") + `]}` }
	const (
		plain        = `{"pokemonId":"9001-000","moveIds":[]}`
		withMove     = `{"pokemonId":"9001-000","moveIds":["move-9005"]}`
		withAbility  = `{"pokemonId":"9001-000","moveIds":[],"abilityId":"ability-9001"}`
		unknownPoke1 = `{"pokemonId":"9998-000","moveIds":[]}`
		unknownPoke2 = `{"pokemonId":"9999-000","moveIds":[]}`
		unknownMove1 = `{"pokemonId":"9001-000","moveIds":["move-9005","move-9998"]}`
		unknownMove2 = `{"pokemonId":"9001-000","moveIds":["move-9999"]}`
		unknownAbil1 = `{"pokemonId":"9001-000","moveIds":[],"abilityId":"ability-9998"}`
		unknownAbil2 = `{"pokemonId":"9001-000","moveIds":[],"abilityId":"ability-9999"}`
		unknownAll   = `{"pokemonId":"9999-000","moveIds":["move-9999"],"abilityId":"ability-9999"}`
		fiveMoves    = `{"pokemonId":"9001-000","moveIds":["move-9001","move-9003","move-9004","move-9005","move-9007"]}`
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
		{name: "headers before body and read models", deps: none, noHeaders: true, body: body(fiveMoves), wantStatus: http.StatusBadRequest, wantCode: api.MissingRequestContext},
		{name: "invalid body before missing read models", deps: none, body: body(fiveMoves), wantStatus: http.StatusBadRequest, wantCode: api.InvalidRequest},
		{name: "invalid limit before missing read models", deps: none, body: `{"members":[` + plain + `],"limit":21}`, wantStatus: http.StatusBadRequest, wantCode: api.InvalidRequest},
		{name: "oversized body before missing read models", deps: none, body: oversized, wantStatus: http.StatusRequestEntityTooLarge, wantCode: api.RequestTooLarge},

		{name: "pokemon read model missing", deps: noPokemon, body: body(plain), wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},
		{name: "pokemon catalog missing", deps: noCatalog, body: body(plain), wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},
		{name: "move read model missing with a moveId", deps: noMoves, body: body(plain, withMove), wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},
		{name: "ability read model missing with an abilityId", deps: noAbilities, body: body(plain, withAbility), wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},
		{name: "missing move read model before unknown pokemon", deps: noMoves, body: body(unknownPoke1, withMove), wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},
		{name: "missing ability read model before unknown pokemon", deps: noAbilities, body: body(unknownPoke1, withAbility), wantStatus: http.StatusServiceUnavailable, wantCode: api.MasterUnavailable},

		{name: "unknown pokemon before unknown move and ability", deps: full, body: body(unknownAll), wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownPokemon, wantMessage: "unknown pokemonId: 9999-000"},
		{name: "unknown pokemon: the first in request order", deps: full, body: body(plain, unknownPoke2, unknownPoke1), wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownPokemon, wantMessage: "unknown pokemonId: 9999-000"},
		{name: "unknown pokemon in a later member before an unknown move in an earlier one", deps: full, body: body(unknownMove2, unknownPoke1), wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownPokemon, wantMessage: "unknown pokemonId: 9998-000"},
		{name: "unknown move before unknown ability (ability in an earlier member)", deps: full, body: body(unknownAbil1, unknownMove2), wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownMove, wantMessage: "unknown moveId: move-9999"},
		{name: "unknown move: the first in request order", deps: full, body: body(withMove, unknownMove1, unknownMove2), wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownMove, wantMessage: "unknown moveId: move-9998"},
		{name: "unknown ability: the first in request order", deps: full, body: body(withAbility, unknownAbil2, unknownAbil1), wantStatus: http.StatusUnprocessableEntity, wantCode: api.UnknownAbility, wantMessage: "unknown abilityId: ability-9999"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, recommendationsPath, strings.NewReader(tt.body))
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
	for name, tc := range map[string]struct {
		deps Dependencies
		body string
	}{
		"all read models":                   {full, body(withMove, withAbility)},
		"no moves and no moveId":            {noMoves, body(plain, withAbility)},
		"no abilities and no abilityId":     {noAbilities, body(withMove, plain)},
		"neither moves nor abilities named": {Dependencies{TypeChart: recommendationsChart(), PokemonTypes: recCatalog.types(), PokemonCatalog: recCatalog}, body(plain)},
	} {
		if recorder := postRecommendations(t, New(tc.deps), tc.body); recorder.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200; body=%s", name, recorder.Code, recorder.Body.String())
		}
	}
}

// 422 messages are built from the request ID, never from adapter detail (ADR-0014 §5.6).
func TestRecommendationsUnknownMessagesDoNotLeakAdapterDetail(t *testing.T) {
	t.Parallel()

	withDeps := func(mutate func(*Dependencies)) Dependencies {
		d := recFullDependencies()
		mutate(&d)
		return d
	}
	tests := []struct {
		name        string
		deps        Dependencies
		body        string
		wantCode    api.ErrorCode
		wantMessage string
	}{
		{
			name: "unknown pokemon",
			deps: withDeps(func(d *Dependencies) {
				d.PokemonTypes = failingPokemonTypes{err: fmt.Errorf("%w: 9999-000 (read from /secret/pokemon.json)", balance.ErrUnknownPokemon)}
			}),
			body: `{"members":[{"pokemonId":"9999-000","moveIds":[]}]}`, wantCode: api.UnknownPokemon, wantMessage: "unknown pokemonId: 9999-000",
		},
		{
			name: "unknown move",
			deps: withDeps(func(d *Dependencies) {
				d.Moves = failingMoves{err: fmt.Errorf("%w: move-9999 (read from /secret/moves.json)", balance.ErrUnknownMove)}
			}),
			body: `{"members":[{"pokemonId":"9001-000","moveIds":["move-9999"]}]}`, wantCode: api.UnknownMove, wantMessage: "unknown moveId: move-9999",
		},
		{
			name: "unknown ability",
			deps: withDeps(func(d *Dependencies) {
				d.Abilities = failingAbilities{err: fmt.Errorf("%w: ability-9999 (read from /secret/abilities.json)", balance.ErrUnknownAbility)}
			}),
			body: `{"members":[{"pokemonId":"9001-000","moveIds":[],"abilityId":"ability-9999"}]}`, wantCode: api.UnknownAbility, wantMessage: "unknown abilityId: ability-9999",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := postRecommendations(t, New(tt.deps), tt.body)
			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422; body=%s", recorder.Code, recorder.Body.String())
			}
			if got := decodeError(t, recorder.Body.Bytes()); got.Code != tt.wantCode || got.Message != tt.wantMessage {
				t.Errorf("error = %+v, want %s %q", got, tt.wantCode, tt.wantMessage)
			}
		})
	}
}

// Every other failure is 500 with the fixed text.
func TestRecommendationsInternalErrors(t *testing.T) {
	t.Parallel()

	withDeps := func(mutate func(*Dependencies)) Dependencies {
		d := recFullDependencies()
		mutate(&d)
		return d
	}
	const withAbilityBody = `{"members":[{"pokemonId":"9001-000","moveIds":["move-9005"],"abilityId":"ability-9003"}]}`
	tests := []struct {
		name string
		deps Dependencies
		body string
	}{
		{name: "pokemon provider failure", deps: withDeps(func(d *Dependencies) {
			d.PokemonTypes = failingPokemonTypes{err: errors.New("pokemon backend exploded at /secret/pokemon.json")}
		})},
		{name: "catalog failure", deps: withDeps(func(d *Dependencies) {
			d.PokemonCatalog = failingCatalog{err: errors.New("catalog backend exploded at /secret/pokemon.json")}
		})},
		{name: "move provider failure", deps: withDeps(func(d *Dependencies) {
			d.Moves = failingMoves{err: errors.New("move backend exploded at /secret/moves.json")}
		})},
		{name: "ability provider failure for a member", deps: withDeps(func(d *Dependencies) {
			d.Abilities = failingAbilities{err: errors.New("ability backend exploded at /secret/abilities.json")}
		}), body: withAbilityBody},
		{name: "ability provider failure for a catalog pokemon", deps: withDeps(func(d *Dependencies) {
			d.Abilities = failingAbilities{err: errors.New("ability backend exploded at /secret/abilities.json")}
		}), body: recBody},
		{name: "nil type chart", deps: withDeps(func(d *Dependencies) { d.TypeChart = nil })},
		{name: "invalid member ability effect from provider", deps: withDeps(func(d *Dependencies) {
			d.Abilities = testAbilities{"ability-9003": fictionalAbility("ability-9003", balance.AbilityEffect{Kind: "heal", AttackType: balance.TypeFire})}
		}), body: withAbilityBody},
		{name: "invalid move category from provider", deps: withDeps(func(d *Dependencies) {
			d.Moves = testMoves{"move-9005": {MoveID: "move-9005", Type: balance.TypeNormal, Category: "other"}}
		})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := tt.body
			if body == "" {
				body = recBody
			}
			recorder := postRecommendations(t, New(tt.deps), body)
			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500; body=%s", recorder.Code, recorder.Body.String())
			}
			if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.InternalError || got.Message != "internal error" {
				t.Errorf("error = %+v, want internal_error with the fixed text %q", got, "internal error")
			}
		})
	}
}

// Adding recommendations does not change health, analyze, coverage or threats.
func TestRecommendationsDoesNotAffectOtherEndpoints(t *testing.T) {
	t.Parallel()

	server := New(Dependencies{
		TypeChart:      testTypeChart(),
		PokemonTypes:   threatPokemonTypes,
		PokemonCatalog: recCatalog,
		Moves:          threatMoves,
		Abilities:      fictionalAbilities,
	})
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
	if recorder := postThreats(t, server, `{"members":[{"pokemonId":"9001-000","moveIds":[]}],"threats":[{"pokemonId":"9002-000","moveIds":[]}]}`); recorder.Code != http.StatusOK {
		t.Errorf("threats status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	// A recommendations-only field is still unknown to threats.
	if recorder := postThreats(t, server, `{"members":[{"pokemonId":"9001-000","moveIds":[]}],"threats":[{"pokemonId":"9002-000","moveIds":[]}],"limit":10}`); recorder.Code != http.StatusBadRequest {
		t.Errorf("threats with limit: status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
	}
}

// The smoke fixture (scripts/smoke.sh) holds with the example read models and the bundled chart:
// 9005-000 (ice) alone. ability-9002 (absorb water) of 9001-000 and ability-9001 (immune to ground)
// of 9006-000 fill holes; steel/fairy (9006-000) is among the first 10 candidates.
func TestRecommendationsWithExampleReadModels(t *testing.T) {
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
	server := New(Dependencies{TypeChart: testTypeChart(), PokemonTypes: pokemon, PokemonCatalog: pokemon, Moves: moves, Abilities: abilities})

	recorder := postRecommendations(t, server, `{"members":[{"pokemonId":"9005-000","moveIds":[]}]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	for _, key := range []string{`"defenseHoles"`, `"offenseHoles":[]`, `"candidates"`, `"abilityOptions"`,
		`"types":["steel","fairy"]`, `"nameJa":"テストメタル"`, `"abilityId":"ability-9002"`, `"abilityId":"ability-9001"`, `"multiplier":"0"`} {
		if !strings.Contains(recorder.Body.String(), key) {
			t.Errorf("body is missing %s; body=%s", key, recorder.Body.String())
		}
	}
	response := decodeRecommendationsResponse(t, recorder.Body.Bytes())
	if len(response.DefenseHoles) != 17 || len(response.Candidates) != 10 {
		t.Errorf("defenseHoles = %d, candidates = %d, want 17 (ice resists only ice) and 10", len(response.DefenseHoles), len(response.Candidates))
	}
}
