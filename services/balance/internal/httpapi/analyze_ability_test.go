package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"example.com/pokecalc/services/balance/internal/api"
	"example.com/pokecalc/services/balance/internal/balance"
	"example.com/pokecalc/services/balance/internal/master"
	"github.com/labstack/echo/v4"
)

// TB3 特性による防御相性の変化の HTTP 契約(ADR-0017)。特性 ID は架空(ability-9001 以降)。

// tb1DefenseMultipliers are the six TB1 labels. A request without abilityId must only
// produce these (ADR-0017 §3: TB1 の6値はその部分集合).
var tb1DefenseMultipliers = map[api.DefenseMultiplier]bool{"0": true, "1/4": true, "1/2": true, "1": true, "2": true, "4": true}

// defenseMultiplierPattern mirrors the DefenseMultiplier schema pattern.
var defenseMultiplierPattern = regexp.MustCompile(`^(0|[1-9][0-9]*(/([2-9]|[1-9][0-9]+))?)$`)

// fortyCharAbilityID is a valid fictional abilityId of exactly the 40-character limit.
var fortyCharAbilityID = "ability-9099-" + strings.Repeat("a", 27)

// testAbilities is a fictional in-memory AbilityProvider.
type testAbilities map[string]balance.Ability

func (p testAbilities) Ability(abilityID string) (balance.Ability, error) {
	if a, ok := p[abilityID]; ok {
		return a, nil
	}
	return balance.Ability{}, fmt.Errorf("%w: %s", balance.ErrUnknownAbility, abilityID)
}

func fraction(num, den int64) balance.Effectiveness {
	e, err := balance.NewEffectiveness(num, den)
	if err != nil {
		panic(err)
	}
	return e
}

func fictionalAbility(id string, effects ...balance.AbilityEffect) balance.Ability {
	return balance.Ability{AbilityID: id, Effects: effects}
}

// fictionalAbilities follows testdata/abilities.example.json.
var fictionalAbilities = testAbilities{
	"ability-9001":     fictionalAbility("ability-9001", balance.AbilityEffect{Kind: balance.AbilityEffectImmune, AttackType: balance.TypeGround}),
	"ability-9002":     fictionalAbility("ability-9002", balance.AbilityEffect{Kind: balance.AbilityEffectAbsorb, AttackType: balance.TypeWater}),
	"ability-9003":     fictionalAbility("ability-9003", balance.AbilityEffect{Kind: balance.AbilityEffectTypeMultiplier, AttackType: balance.TypeFire, Factor: fraction(1, 2)}),
	"ability-9004":     fictionalAbility("ability-9004", balance.AbilityEffect{Kind: balance.AbilityEffectSuperEffectiveMultiplier, Factor: fraction(3, 4)}),
	"ability-9005":     fictionalAbility("ability-9005"),
	"ability-9006":     fictionalAbility("ability-9006", balance.AbilityEffect{Kind: balance.AbilityEffectTypeMultiplier, AttackType: balance.TypeFire, Factor: fraction(5, 4)}),
	fortyCharAbilityID: fictionalAbility(fortyCharAbilityID, balance.AbilityEffect{Kind: balance.AbilityEffectImmune, AttackType: balance.TypeFire}),
}

// failingAbilities fails in a way other than an unknown abilityId (500 path), or with a
// wrapped ErrUnknownAbility carrying adapter detail (422 message path).
type failingAbilities struct{ err error }

func (f failingAbilities) Ability(string) (balance.Ability, error) { return balance.Ability{}, f.err }

func newAbilityServer() *echo.Echo {
	return New(Dependencies{
		TypeChart:    testTypeChart(),
		PokemonTypes: fictionalPokemonTypes,
		Abilities:    fictionalAbilities,
	})
}

// parseMultiplier parses a DefenseMultiplier and checks it is an irreducible fraction whose
// denominator 1 is written as an integer.
func parseMultiplier(t *testing.T, m api.DefenseMultiplier) *big.Rat {
	t.Helper()
	if !defenseMultiplierPattern.MatchString(m) {
		t.Fatalf("multiplier %q does not match the DefenseMultiplier pattern", m)
	}
	r, ok := new(big.Rat).SetString(m)
	if !ok {
		t.Fatalf("multiplier %q is not a fraction", m)
	}
	if r.RatString() != m {
		t.Errorf("multiplier %q is not in lowest terms (want %q)", m, r.RatString())
	}
	return r
}

// categoryOf applies the ADR-0017 §3 ranges to a parsed multiplier.
func categoryOf(r *big.Rat) api.DefenseCategory {
	quarter, one, four := big.NewRat(1, 4), big.NewRat(1, 1), big.NewRat(4, 1)
	switch {
	case r.Sign() == 0:
		return api.Immune
	case r.Cmp(quarter) <= 0:
		return api.QuadResist
	case r.Cmp(one) < 0:
		return api.Resist
	case r.Cmp(one) == 0:
		return api.Neutral
	case r.Cmp(four) < 0:
		return api.Weak
	default:
		return api.QuadWeak
	}
}

// assertAnalyzeContract checks every entry: pattern and lowest terms, category by range,
// enum values, the source/effect pairing, and the summary recounted from categories.
func assertAnalyzeContract(t *testing.T, response api.AnalyzeResponse, memberCount int) {
	t.Helper()
	if len(response.Members) != memberCount || len(response.TeamSummary) != 18 {
		t.Fatalf("members=%d teamSummary=%d, want %d and 18", len(response.Members), len(response.TeamSummary), memberCount)
	}
	for mi, member := range response.Members {
		if len(member.Defense) != 18 {
			t.Fatalf("members[%d].defense length = %d, want 18", mi, len(member.Defense))
		}
		for ai, entry := range member.Defense {
			if entry.AttackType != canonicalTypeIDs[ai] {
				t.Errorf("members[%d].defense[%d].attackType = %q, want %q", mi, ai, entry.AttackType, canonicalTypeIDs[ai])
			}
			if !entry.Category.Valid() || !entry.Source.Valid() || !entry.Effect.Valid() {
				t.Errorf("members[%d].defense[%d] has out-of-enum values: %+v", mi, ai, entry)
			}
			if want := categoryOf(parseMultiplier(t, entry.Multiplier)); entry.Category != want {
				t.Errorf("members[%d] vs %s: category %q for multiplier %q, want %q", mi, entry.AttackType, entry.Category, entry.Multiplier, want)
			}
			switch entry.Effect {
			case api.EffectImmune, api.EffectAbsorb:
				if entry.Multiplier != "0" {
					t.Errorf("members[%d] vs %s: effect %q with multiplier %q, want 0", mi, entry.AttackType, entry.Effect, entry.Multiplier)
				}
				if entry.Effect == api.EffectAbsorb && entry.Source != api.Ability {
					t.Errorf("members[%d] vs %s: absorb must come from the ability, source %q", mi, entry.AttackType, entry.Source)
				}
			case api.EffectMultiplier:
				if entry.Source != api.Ability {
					t.Errorf("members[%d] vs %s: effect multiplier with source %q, want ability", mi, entry.AttackType, entry.Source)
				}
			case api.EffectNone:
				if entry.Source != api.Type {
					t.Errorf("members[%d] vs %s: effect none with source %q, want type", mi, entry.AttackType, entry.Source)
				}
			}
		}
	}
	for i, entry := range response.TeamSummary {
		if entry.AttackType != canonicalTypeIDs[i] {
			t.Errorf("teamSummary[%d].attackType = %q, want %q", i, entry.AttackType, canonicalTypeIDs[i])
		}
		want := api.TeamSummaryEntry{AttackType: entry.AttackType}
		for _, member := range response.Members {
			switch member.Defense[i].Category {
			case api.QuadWeak:
				want.Weak++
				want.QuadWeak++
			case api.Weak:
				want.Weak++
			case api.Resist, api.QuadResist:
				want.Resist++
			case api.Immune:
				want.Immune++
			case api.Neutral:
				want.Neutral++
			}
		}
		if entry != want {
			t.Errorf("teamSummary[%s] = %+v, disagrees with member categories %+v", entry.AttackType, entry, want)
		}
	}
}

func TestAnalyzeWithAbilitiesResponseBody(t *testing.T) {
	t.Parallel()

	body := `{"members":[
		{"pokemonId":"9002-000","abilityId":"ability-9002"},
		{"pokemonId":"9001-000","abilityId":"ability-9001"},
		{"pokemonId":"9003-000","abilityId":"ability-9004"},
		{"pokemonId":"9002-000"},
		{"pokemonId":"9001-000","abilityId":"ability-9003"},
		{"pokemonId":"9002-000","abilityId":"ability-9006"}
	]}`
	recorder := postAnalyze(t, newAbilityServer(), body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeAnalyzeResponse(t, recorder.Body.Bytes())
	assertAnalyzeContract(t, response, 6)

	wantMembers := []struct {
		pokemonID string
		abilityID *string
	}{
		{"9002-000", ptr("ability-9002")},
		{"9001-000", ptr("ability-9001")},
		{"9003-000", ptr("ability-9004")},
		{"9002-000", nil},
		{"9001-000", ptr("ability-9003")},
		{"9002-000", ptr("ability-9006")},
	}
	for i, want := range wantMembers {
		got := response.Members[i]
		if got.PokemonId != want.pokemonID {
			t.Errorf("members[%d].pokemonId = %q, want %q", i, got.PokemonId, want.pokemonID)
		}
		if (got.AbilityId == nil) != (want.abilityID == nil) || (got.AbilityId != nil && *got.AbilityId != *want.abilityID) {
			t.Errorf("members[%d].abilityId = %v, want %v", i, deref(got.AbilityId), deref(want.abilityID))
		}
	}

	tests := []struct {
		name       string
		member     int
		attack     api.TypeId
		multiplier api.DefenseMultiplier
		category   api.DefenseCategory
		source     api.EffectSource
		effect     api.DefenseEffect
	}{
		{"absorb water on grass", 0, api.Water, "0", api.Immune, api.Ability, api.EffectAbsorb},
		{"absorb leaves fire", 0, api.Fire, "2", api.Weak, api.Type, api.EffectNone},
		{"absorb leaves electric", 0, api.Electric, "1/2", api.Resist, api.Type, api.EffectNone},
		{"immune ground leaves rock", 1, api.Rock, "4", api.QuadWeak, api.Type, api.EffectNone},
		{"super effective x4 becomes x3", 2, api.Grass, "3", api.Weak, api.Ability, api.EffectMultiplier},
		{"super effective leaves resistance", 2, api.Rock, "1/2", api.Resist, api.Type, api.EffectNone},
		{"super effective leaves neutral dual type", 2, api.Ice, "1", api.Neutral, api.Type, api.EffectNone},
		{"super effective leaves neutral", 2, api.Normal, "1", api.Neutral, api.Type, api.EffectNone},
		{"no ability", 3, api.Water, "1/2", api.Resist, api.Type, api.EffectNone},
		{"type multiplier 1/2 on resistance", 4, api.Fire, "1/4", api.QuadResist, api.Ability, api.EffectMultiplier},
		{"type multiplier leaves water", 4, api.Water, "2", api.Weak, api.Type, api.EffectNone},
		{"type multiplier 5/4 on weakness", 5, api.Fire, "5/2", api.Weak, api.Ability, api.EffectMultiplier},
	}
	for _, tt := range tests {
		entry := findDefenseEntry(t, response.Members[tt.member].Defense, tt.attack)
		if entry.Multiplier != tt.multiplier || entry.Category != tt.category || entry.Source != tt.source || entry.Effect != tt.effect {
			t.Errorf("%s: got %q %q %q %q, want %q %q %q %q", tt.name, entry.Multiplier, entry.Category, entry.Source, entry.Effect,
				tt.multiplier, tt.category, tt.source, tt.effect)
		}
	}

	// Type immunities stay type-derived even when the ability would also make them x0.
	for _, tt := range []struct {
		member int
		attack api.TypeId
	}{{1, api.Ground}, {2, api.Electric}, {4, api.Ground}} {
		entry := findDefenseEntry(t, response.Members[tt.member].Defense, tt.attack)
		if entry.Multiplier != "0" || entry.Category != api.Immune || entry.Source != api.Type {
			t.Errorf("members[%d] vs %s = %+v, want 0 immune from type", tt.member, tt.attack, entry)
		}
		// The effect of a type immunity is undecided in ADR-0017 (none or immune).
		if entry.Effect != api.EffectNone && entry.Effect != api.EffectImmune {
			t.Errorf("members[%d] vs %s: effect %q, want none or immune", tt.member, tt.attack, entry.Effect)
		}
	}

	summaries := []api.TeamSummaryEntry{
		{AttackType: api.Water, Weak: 2, Resist: 2, Immune: 1, Neutral: 1},
		{AttackType: api.Fire, Weak: 3, Resist: 3},
		{AttackType: api.Grass, Weak: 1, Resist: 5},
		{AttackType: api.Ground, Resist: 3, Immune: 2, Neutral: 1},
		{AttackType: api.Electric, Weak: 2, Resist: 3, Immune: 1},
	}
	for _, want := range summaries {
		if got := findSummaryEntry(t, response.TeamSummary, want.AttackType); got != want {
			t.Errorf("teamSummary[%s] = %+v, want %+v", want.AttackType, got, want)
		}
	}
}

func ptr(s string) *string { return &s }

func deref(s *string) string {
	if s == nil {
		return "<absent>"
	}
	return *s
}

func TestAnalyzeAbilityIDIsOmittedWithoutAbility(t *testing.T) {
	t.Parallel()

	recorder := postAnalyze(t, newAbilityServer(), `{"members":[{"pokemonId":"9001-000"},{"pokemonId":"9002-000","abilityId":"ability-9002"}]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	var raw struct {
		Members []map[string]json.RawMessage `json:"members"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil || len(raw.Members) != 2 {
		t.Fatalf("unmarshal: %v; body=%s", err, recorder.Body.String())
	}
	if value, ok := raw.Members[0]["abilityId"]; ok {
		t.Errorf("members[0].abilityId = %s, want the key to be absent (not null)", value)
	}
	if string(raw.Members[1]["abilityId"]) != `"ability-9002"` {
		t.Errorf("members[1].abilityId = %s, want \"ability-9002\"", raw.Members[1]["abilityId"])
	}
	var defense []map[string]json.RawMessage
	if err := json.Unmarshal(raw.Members[1]["defense"], &defense); err != nil || len(defense) != 18 {
		t.Fatalf("defense = %s, err = %v", raw.Members[1]["defense"], err)
	}
	for _, key := range []string{"attackType", "multiplier", "category", "source", "effect"} {
		if _, ok := defense[0][key]; !ok {
			t.Errorf("defense entry is missing %q", key)
		}
	}
	// water (index 2) vs the absorbing member.
	if string(defense[2]["multiplier"]) != `"0"` || string(defense[2]["effect"]) != `"absorb"` || string(defense[2]["source"]) != `"ability"` {
		t.Errorf("water entry = %v, want multiplier \"0\", effect \"absorb\", source \"ability\"", stringify(defense[2]))
	}
}

func stringify(m map[string]json.RawMessage) string {
	var b strings.Builder
	for k, v := range m {
		fmt.Fprintf(&b, "%s=%s ", k, v)
	}
	return b.String()
}

// TestAnalyzeWithoutAbilityIDIsTB1 (ADR-0017 §1・§4): a request without abilityId answers 200
// even without the ability read model, with TB1 values, and the same body either way.
func TestAnalyzeWithoutAbilityIDIsTB1(t *testing.T) {
	t.Parallel()

	body := `{"members":[{"pokemonId":"9001-000"},{"pokemonId":"9002-000"},{"pokemonId":"9003-000"},{"pokemonId":"9004-000"},{"pokemonId":"9005-000"},{"pokemonId":"9006-000"}]}`
	withoutModel := postAnalyze(t, newTestServer(), body)
	if withoutModel.Code != http.StatusOK {
		t.Fatalf("without the ability read model: status = %d, want 200; body=%s", withoutModel.Code, withoutModel.Body.String())
	}
	withModel := postAnalyze(t, newAbilityServer(), body)
	if withModel.Code != http.StatusOK {
		t.Fatalf("with the ability read model: status = %d, want 200; body=%s", withModel.Code, withModel.Body.String())
	}
	if withoutModel.Body.String() != withModel.Body.String() {
		t.Errorf("body depends on the ability read model:\nwithout: %s\nwith:    %s", withoutModel.Body.String(), withModel.Body.String())
	}

	response := decodeAnalyzeResponse(t, withoutModel.Body.Bytes())
	assertAnalyzeContract(t, response, 6)
	for mi, member := range response.Members {
		if member.AbilityId != nil {
			t.Errorf("members[%d].abilityId = %q, want absent", mi, *member.AbilityId)
		}
		for _, entry := range member.Defense {
			if !tb1DefenseMultipliers[entry.Multiplier] {
				t.Errorf("members[%d] vs %s: multiplier %q is not a TB1 value", mi, entry.AttackType, entry.Multiplier)
			}
			if entry.Source != api.Type {
				t.Errorf("members[%d] vs %s: source %q, want type", mi, entry.AttackType, entry.Source)
			}
			if entry.Multiplier == "0" {
				if entry.Effect != api.EffectNone && entry.Effect != api.EffectImmune {
					t.Errorf("members[%d] vs %s: type immunity effect %q, want none or immune", mi, entry.AttackType, entry.Effect)
				}
			} else if entry.Effect != api.EffectNone {
				t.Errorf("members[%d] vs %s: effect %q, want none", mi, entry.AttackType, entry.Effect)
			}
		}
	}
}

// TestAnalyzeAbilityWithoutEffectsMatchesNoAbility: an ability with an empty effects array
// changes nothing but the echoed abilityId.
func TestAnalyzeAbilityWithoutEffectsMatchesNoAbility(t *testing.T) {
	t.Parallel()

	server := newAbilityServer()
	plain := postAnalyze(t, server, `{"members":[{"pokemonId":"9001-000"},{"pokemonId":"9003-000"}]}`)
	withAbility := postAnalyze(t, server, `{"members":[{"pokemonId":"9001-000","abilityId":"ability-9005"},{"pokemonId":"9003-000","abilityId":"ability-9005"}]}`)
	if plain.Code != http.StatusOK || withAbility.Code != http.StatusOK {
		t.Fatalf("status = %d / %d, want 200; body=%s / %s", plain.Code, withAbility.Code, plain.Body.String(), withAbility.Body.String())
	}
	a := decodeAnalyzeResponse(t, plain.Body.Bytes())
	b := decodeAnalyzeResponse(t, withAbility.Body.Bytes())
	for i := range a.Members {
		if b.Members[i].AbilityId == nil || *b.Members[i].AbilityId != "ability-9005" {
			t.Errorf("members[%d].abilityId = %v, want ability-9005", i, deref(b.Members[i].AbilityId))
		}
		if fmt.Sprint(a.Members[i].Defense) != fmt.Sprint(b.Members[i].Defense) {
			t.Errorf("members[%d].defense differs with an effectless ability:\n%v\n%v", i, a.Members[i].Defense, b.Members[i].Defense)
		}
	}
	if fmt.Sprint(a.TeamSummary) != fmt.Sprint(b.TeamSummary) {
		t.Errorf("teamSummary differs with an effectless ability")
	}
}

func TestAnalyzeAbilityBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "40-character abilityId", body: `{"members":[{"pokemonId":"9002-000","abilityId":"` + fortyCharAbilityID + `"}]}`},
		{name: "same abilityId on every member", body: `{"members":[` + strings.TrimSuffix(strings.Repeat(`{"pokemonId":"9001-000","abilityId":"ability-9002"},`, 6), ",") + `]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := postAnalyze(t, newAbilityServer(), tt.body)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
			}
			decodeAnalyzeResponse(t, recorder.Body.Bytes())
		})
	}
}

func TestAnalyzeRejectsMalformedAbilityID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		abilityID string // raw JSON value
	}{
		{name: "empty", abilityID: `""`},
		{name: "uppercase", abilityID: `"Ability-9001"`},
		{name: "underscore", abilityID: `"ability_9001"`},
		{name: "space", abilityID: `"ability 9001"`},
		{name: "surrounding space", abilityID: `" ability-9001"`},
		{name: "leading hyphen", abilityID: `"-ability-9001"`},
		{name: "trailing hyphen", abilityID: `"ability-9001-"`},
		{name: "double hyphen", abilityID: `"ability--9001"`},
		{name: "41 characters", abilityID: `"ability-9099-` + strings.Repeat("a", 28) + `"`},
		{name: "number", abilityID: `9001`},
		{name: "array", abilityID: `["ability-9001"]`},
		{name: "object", abilityID: `{"id":"ability-9001"}`},
	}
	servers := map[string]*echo.Echo{
		"with read models":    newAbilityServer(),
		"without read models": New(Dependencies{TypeChart: testTypeChart()}),
	}
	for serverName, server := range servers {
		for _, tt := range tests {
			t.Run(serverName+"/"+tt.name, func(t *testing.T) {
				t.Parallel()
				body := `{"members":[{"pokemonId":"9001-000"},{"pokemonId":"9002-000","abilityId":` + tt.abilityID + `}]}`
				recorder := postAnalyze(t, server, body)
				if recorder.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want 400 (before 503/422); body=%s", recorder.Code, recorder.Body.String())
				}
				if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.InvalidRequest {
					t.Errorf("code = %q, want invalid_request", got.Code)
				}
			})
		}
	}
}

// TestAnalyzeAbilityValidationOrder (ADR-0017 §4): header (400) → body (400/413) →
// pokemon read model absent, or an abilityId with the ability read model absent (503) →
// unknown_pokemon (422) → unknown_ability (422) → 200.
func TestAnalyzeAbilityValidationOrder(t *testing.T) {
	t.Parallel()

	noAbilities := New(Dependencies{TypeChart: testTypeChart(), PokemonTypes: fictionalPokemonTypes})
	noPokemon := New(Dependencies{TypeChart: testTypeChart(), Abilities: fictionalAbilities})
	full := newAbilityServer()

	tests := []struct {
		name     string
		server   *echo.Echo
		body     string
		wantCode int
		wantErr  api.ErrorCode
		wantMsg  string
	}{
		{name: "abilityId without the ability read model", server: noAbilities,
			body: `{"members":[{"pokemonId":"9001-000"},{"pokemonId":"9002-000","abilityId":"ability-9001"}]}`, wantCode: http.StatusServiceUnavailable, wantErr: api.MasterUnavailable},
		{name: "503 before unknown_pokemon", server: noAbilities,
			body: `{"members":[{"pokemonId":"9999-000","abilityId":"ability-9001"}]}`, wantCode: http.StatusServiceUnavailable, wantErr: api.MasterUnavailable},
		{name: "503 before unknown_ability", server: noAbilities,
			body: `{"members":[{"pokemonId":"9001-000","abilityId":"ability-9999"}]}`, wantCode: http.StatusServiceUnavailable, wantErr: api.MasterUnavailable},
		{name: "pokemon read model absent with abilityId", server: noPokemon,
			body: `{"members":[{"pokemonId":"9001-000","abilityId":"ability-9001"}]}`, wantCode: http.StatusServiceUnavailable, wantErr: api.MasterUnavailable},
		{name: "pokemon read model absent without abilityId", server: noPokemon,
			body: `{"members":[{"pokemonId":"9001-000"}]}`, wantCode: http.StatusServiceUnavailable, wantErr: api.MasterUnavailable},
		{name: "unknown_pokemon before unknown_ability on an earlier member", server: full,
			body: `{"members":[{"pokemonId":"9001-000","abilityId":"ability-9999"},{"pokemonId":"9999-000"}]}`, wantCode: http.StatusUnprocessableEntity, wantErr: api.UnknownPokemon, wantMsg: "unknown pokemonId: 9999-000"},
		{name: "unknown_pokemon before unknown_ability on the same member", server: full,
			body: `{"members":[{"pokemonId":"9999-000","abilityId":"ability-9999"}]}`, wantCode: http.StatusUnprocessableEntity, wantErr: api.UnknownPokemon, wantMsg: "unknown pokemonId: 9999-000"},
		{name: "unknown_ability", server: full,
			body: `{"members":[{"pokemonId":"9001-000","abilityId":"ability-9001"},{"pokemonId":"9002-000","abilityId":"ability-9999"}]}`, wantCode: http.StatusUnprocessableEntity, wantErr: api.UnknownAbility, wantMsg: "unknown abilityId: ability-9999"},
		{name: "unknown_ability is the first unknown in request order", server: full,
			body: `{"members":[{"pokemonId":"9001-000","abilityId":"ability-9998"},{"pokemonId":"9002-000","abilityId":"ability-9999"}]}`, wantCode: http.StatusUnprocessableEntity, wantErr: api.UnknownAbility, wantMsg: "unknown abilityId: ability-9998"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := postAnalyze(t, tt.server, tt.body)
			if recorder.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.wantCode, recorder.Body.String())
			}
			got := decodeError(t, recorder.Body.Bytes())
			if got.Code != tt.wantErr {
				t.Errorf("code = %q, want %q", got.Code, tt.wantErr)
			}
			if tt.wantMsg != "" && got.Message != tt.wantMsg {
				t.Errorf("message = %q, want %q", got.Message, tt.wantMsg)
			}
		})
	}

	// The ability read model absent does not matter without abilityId, and health stays 200.
	if recorder := postAnalyze(t, noAbilities, `{"members":[{"pokemonId":"9001-000"}]}`); recorder.Code != http.StatusOK {
		t.Errorf("no abilityId without the ability read model: status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAnalyzeAbilityRequiresRequestContextFirst(t *testing.T) {
	t.Parallel()

	for name, server := range map[string]*echo.Echo{
		"full":         newAbilityServer(),
		"no abilities": New(Dependencies{TypeChart: testTypeChart(), PokemonTypes: fictionalPokemonTypes}),
	} {
		recorder := postAnalyzeWithoutContext(t, server, `{"members":[{"pokemonId":"9001-000","abilityId":"ability-9999"}]}`)
		if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"missing_request_context"`) {
			t.Errorf("%s: status = %d body=%s, want 400 missing_request_context", name, recorder.Code, recorder.Body.String())
		}
	}
}

func postAnalyzeWithoutContext(t *testing.T, server http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, analyzePath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recorder, request)
	return recorder
}

// ADR-0017 §4 / ADR-0014 §5.6: the 422 message is built from the ID, never from adapter detail.
func TestAnalyzeUnknownAbilityMessageDoesNotLeakAdapterDetail(t *testing.T) {
	t.Parallel()

	server := New(Dependencies{
		TypeChart:    testTypeChart(),
		PokemonTypes: fictionalPokemonTypes,
		Abilities:    failingAbilities{err: fmt.Errorf("%w: ability-9999 (read from /secret/path.json)", balance.ErrUnknownAbility)},
	})
	recorder := postAnalyze(t, server, `{"members":[{"pokemonId":"9001-000","abilityId":"ability-9999"}]}`)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body=%s", recorder.Code, recorder.Body.String())
	}
	got := decodeError(t, recorder.Body.Bytes())
	if got.Code != api.UnknownAbility || got.Message != "unknown abilityId: ability-9999" {
		t.Errorf("error = %+v, want unknown_ability %q", got, "unknown abilityId: ability-9999")
	}
}

func TestAnalyzeAbilityInternalErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		abilities balance.AbilityProvider
	}{
		{name: "provider failure", abilities: failingAbilities{err: errors.New("ability backend exploded at /secret/path.json")}},
		{name: "invalid effect from provider", abilities: testAbilities{
			"ability-9001": fictionalAbility("ability-9001", balance.AbilityEffect{Kind: "heal", AttackType: balance.TypeFire}),
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := New(Dependencies{TypeChart: testTypeChart(), PokemonTypes: fictionalPokemonTypes, Abilities: tt.abilities})
			recorder := postAnalyze(t, server, `{"members":[{"pokemonId":"9001-000","abilityId":"ability-9001"}]}`)
			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500; body=%s", recorder.Code, recorder.Body.String())
			}
			got := decodeError(t, recorder.Body.Bytes())
			if got.Code != api.InternalError || got.Message != "internal error" {
				t.Errorf("error = %+v, want internal_error with the fixed text %q", got, "internal error")
			}
		})
	}

	// A broken ability provider does not affect requests without abilityId.
	server := New(Dependencies{TypeChart: testTypeChart(), PokemonTypes: fictionalPokemonTypes, Abilities: failingAbilities{err: errors.New("down")}})
	if recorder := postAnalyze(t, server, `{"members":[{"pokemonId":"9001-000"}]}`); recorder.Code != http.StatusOK {
		t.Errorf("no abilityId with a failing ability provider: status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
}

// TestCoverageRejectsAbilityID: coverage does not change in TB3 (ADR-0017 §4).
func TestCoverageRejectsAbilityID(t *testing.T) {
	t.Parallel()

	server := New(Dependencies{TypeChart: testTypeChart(), PokemonTypes: fictionalPokemonTypes, Moves: fictionalMoves, Abilities: fictionalAbilities})
	recorder := postCoverage(t, server, `{"members":[{"pokemonId":"9001-000","moveIds":["move-9001"],"abilityId":"ability-9001"}]}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
	}
	if got := decodeError(t, recorder.Body.Bytes()); got.Code != api.InvalidRequest {
		t.Errorf("code = %q, want invalid_request", got.Code)
	}
}

func TestAnalyzeWithExampleAbilityReadModel(t *testing.T) {
	t.Parallel()

	pokemon, err := master.LoadPokemonTypesFile("../../testdata/pokemon-types.example.json")
	if err != nil || pokemon == nil {
		t.Fatalf("LoadPokemonTypesFile(example) = %v, %v", pokemon, err)
	}
	abilities, err := master.LoadAbilitiesFile("../../testdata/abilities.example.json")
	if err != nil || abilities == nil {
		t.Fatalf("LoadAbilitiesFile(example) = %v, %v", abilities, err)
	}
	server := New(Dependencies{TypeChart: testTypeChart(), PokemonTypes: pokemon, Abilities: abilities})
	// 9002-000 grass with ability-9002 (absorb water) and 9003-000 water/ground with ability-9004 (super effective x3/4).
	recorder := postAnalyze(t, server, `{"members":[{"pokemonId":"9002-000","abilityId":"ability-9002"},{"pokemonId":"9003-000","abilityId":"ability-9004"}]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeAnalyzeResponse(t, recorder.Body.Bytes())
	assertAnalyzeContract(t, response, 2)
	if entry := findDefenseEntry(t, response.Members[0].Defense, api.Water); entry.Multiplier != "0" || entry.Effect != api.EffectAbsorb {
		t.Errorf("9002-000 vs water = %+v, want 0 absorb", entry)
	}
	if entry := findDefenseEntry(t, response.Members[1].Defense, api.Grass); entry.Multiplier != "3" || entry.Effect != api.EffectMultiplier {
		t.Errorf("9003-000 vs grass = %+v, want 3 multiplier", entry)
	}
}
