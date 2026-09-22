package balance_test

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"example.com/pokecalc/services/balance/internal/balance"
)

// TB5 おすすめタイプと該当ポケモン(ADR-0401)。
//
// 手で数え上げられるように、既定が ×1 の小さな架空の相性表(recChart)を主に使う。
// 同梱の相性表(master.EmbeddedTypeChart)では、独立に組んだ oracle と上位を照合する。
// ID はすべて架空(9001-000 / move-9001 / ability-9001 以降)、名前も架空。

// recChart is a fictional type chart: every matchup not listed is x1.
type recChart map[[2]balance.TypeID]balance.Multiplier

func (c recChart) Matchup(attack, defense balance.TypeID) (balance.Multiplier, error) {
	if m, ok := c[[2]balance.TypeID{attack, defense}]; ok {
		return m, nil
	}
	return balance.MultiplierNormal, nil
}

func (c recChart) set(attack, defense balance.TypeID, m balance.Multiplier) recChart {
	c[[2]balance.TypeID{attack, defense}] = m
	return c
}

// resistAllExcept makes defense resist (x1/2) every attack type except the listed ones.
func (c recChart) resistAllExcept(defense balance.TypeID, except ...balance.TypeID) recChart {
	skip := make(map[balance.TypeID]bool, len(except))
	for _, t := range except {
		skip[t] = true
	}
	for _, attack := range balance.AllTypes() {
		if !skip[attack] {
			c.set(attack, defense, balance.MultiplierHalf)
		}
	}
	return c
}

const (
	half = balance.MultiplierHalf
	dbl  = balance.MultiplierDouble
	zero = balance.MultiplierZero
)

// chartTwoDefenseHoles: a normal member resists everything but fire and ice, so the defense
// holes are [fire, ice]. water takes both at x1/2, rock takes fire at x1/2, ghost is immune to ice.
// grass hits water x2, fighting and ground hit rock x2.
func chartTwoDefenseHoles() recChart {
	return recChart{}.resistAllExcept(balance.TypeNormal, balance.TypeFire, balance.TypeIce).
		set(balance.TypeFire, balance.TypeWater, half).
		set(balance.TypeIce, balance.TypeWater, half).
		set(balance.TypeFire, balance.TypeRock, half).
		set(balance.TypeIce, balance.TypeGhost, zero).
		set(balance.TypeGrass, balance.TypeWater, dbl).
		set(balance.TypeFighting, balance.TypeRock, dbl).
		set(balance.TypeGround, balance.TypeRock, dbl)
}

// chartRockOnly: a normal member resists everything but fire; only rock takes fire below x1.
// There are no x2 matchups, so every weaknesses count is 0. Exactly 18 candidates fill the hole
// (rock and the 17 duals with rock).
func chartRockOnly() recChart {
	return recChart{}.resistAllExcept(balance.TypeNormal, balance.TypeFire).
		set(balance.TypeFire, balance.TypeRock, half)
}

// chartOffense: a fairy member resists everything (no defense hole). A dark member's normal move
// hits rock x1/2, ghost x0, steel x1/2 (steel resists everything but fighting) and fairy x1/2, so the
// offense holes are [rock, ghost, steel, fairy]. fire hits rock and steel x1/2; fighting hits steel x2.
// Nothing hits fairy at x1 or more.
func chartOffense() recChart {
	return recChart{}.resistAllExcept(balance.TypeFairy).
		resistAllExcept(balance.TypeSteel, balance.TypeFighting).
		set(balance.TypeFighting, balance.TypeSteel, dbl).
		set(balance.TypeNormal, balance.TypeRock, half).
		set(balance.TypeNormal, balance.TypeGhost, zero).
		set(balance.TypeFire, balance.TypeRock, half)
}

// chartAbilityOptions: a normal member resists everything but fire and water (holes [fire, water]).
// rock takes fire x1/2, grass takes water x1/2 and fire x2.
func chartAbilityOptions() recChart {
	return recChart{}.resistAllExcept(balance.TypeNormal, balance.TypeFire, balance.TypeWater).
		set(balance.TypeFire, balance.TypeRock, half).
		set(balance.TypeWater, balance.TypeGrass, half).
		set(balance.TypeFire, balance.TypeGrass, dbl)
}

// recAbilities is a fictional in-memory AbilityProvider.
type recAbilities map[string]balance.Ability

func (p recAbilities) Ability(id string) (balance.Ability, error) {
	if a, ok := p[id]; ok {
		return a, nil
	}
	return balance.Ability{}, fmt.Errorf("%w: %s", balance.ErrUnknownAbility, id)
}

type failingRecAbilities struct{ err error }

func (f failingRecAbilities) Ability(string) (balance.Ability, error) {
	return balance.Ability{}, f.err
}

func recAbility(id string, effects ...balance.AbilityEffect) balance.Ability {
	return balance.Ability{AbilityID: id, Effects: effects}
}

func catalogPokemon(id, nameJa string, ts []balance.TypeID, abilityIDs ...string) balance.CatalogPokemon {
	return balance.CatalogPokemon{PokemonID: id, NameJa: nameJa, Types: ts, AbilityIDs: abilityIDs}
}

// typeLabel renders a type set as "water" or "water/grass".
func typeLabel(ts []balance.TypeID) string {
	parts := make([]string, len(ts))
	for i, t := range ts {
		parts[i] = string(t)
	}
	return strings.Join(parts, "/")
}

func candidateLabels(candidates []balance.TypeCandidate) []string {
	labels := make([]string, len(candidates))
	for i, c := range candidates {
		labels[i] = typeLabel(c.Types)
	}
	return labels
}

func recommend(t *testing.T, chart balance.TypeChartProvider, members []balance.Combatant, catalog []balance.CatalogPokemon, abilities balance.AbilityProvider, limit int) balance.Recommendation {
	t.Helper()
	got, err := balance.RecommendTypes(chart, members, catalog, abilities, limit)
	if err != nil {
		t.Fatalf("RecommendTypes() error = %v", err)
	}
	return got
}

func findCandidate(t *testing.T, got balance.Recommendation, label string) balance.TypeCandidate {
	t.Helper()
	for _, c := range got.Candidates {
		if typeLabel(c.Types) == label {
			return c
		}
	}
	t.Fatalf("candidate %s not found in %v", label, candidateLabels(got.Candidates))
	return balance.TypeCandidate{}
}

var normalMember = []balance.Combatant{combatant("9001-000", types(balance.TypeNormal), nil)}

// ADR-0401 §2: a defense hole is an attack type that no member resists or is immune to
// (resist + immune = 0), with the members' abilities (TB3) applied.
func TestRecommendTypesDefenseHoles(t *testing.T) {
	t.Parallel()

	normal := func(ability *balance.Ability) balance.Combatant {
		return combatant("9001-000", types(balance.TypeNormal), ability)
	}
	allButFire := make([]string, 0, 17)
	for _, attack := range balance.AllTypes() {
		if attack != balance.TypeFire {
			allButFire = append(allButFire, string(attack))
		}
	}
	tests := []struct {
		name    string
		members []balance.Combatant
		want    string
	}{
		{name: "no resistance to fire and ice", members: []balance.Combatant{normal(nil)}, want: "fire/ice"},
		{name: "ability immune to fire", members: []balance.Combatant{normal(threatAbility("ability-9001", immuneEffect(balance.TypeFire)))}, want: "ice"},
		{name: "ability absorbs ice", members: []balance.Combatant{normal(threatAbility("ability-9002", absorbEffect(balance.TypeIce)))}, want: "fire"},
		{name: "ability halves fire", members: []balance.Combatant{normal(threatAbility("ability-9003", typeFactorEffect(balance.TypeFire, 1, 2)))}, want: "ice"},
		{name: "ability x5/4 on fire is not a resistance", members: []balance.Combatant{normal(threatAbility("ability-9006", typeFactorEffect(balance.TypeFire, 5, 4)))}, want: "fire/ice"},
		{name: "super effective x3/4 does not touch neutral hits", members: []balance.Combatant{normal(threatAbility("ability-9004", superEffectiveFactorEffect(3, 4)))}, want: "fire/ice"},
		{name: "ability without effects", members: []balance.Combatant{normal(threatAbility("ability-9005"))}, want: "fire/ice"},
		{name: "a rock member resists fire", members: []balance.Combatant{normal(nil), combatant("9002-000", types(balance.TypeRock), nil)}, want: "ice"},
		{name: "a ghost member is immune to ice", members: []balance.Combatant{normal(nil), combatant("9003-000", types(balance.TypeGhost), nil)}, want: "fire"},
		{name: "a water member resists both", members: []balance.Combatant{normal(nil), combatant("9004-000", types(balance.TypeWater), nil)}, want: ""},
		{name: "a lone rock member only resists fire (its weaknesses do not matter)", members: []balance.Combatant{combatant("9002-000", types(balance.TypeRock), nil)}, want: strings.Join(allButFire, "/")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := recommend(t, chartTwoDefenseHoles(), tt.members, nil, nil, balance.MaxRecommendationLimit)
			if typeLabel(got.DefenseHoles) != tt.want {
				t.Errorf("DefenseHoles = %v, want %s (canonical order)", got.DefenseHoles, tt.want)
			}
			if len(got.OffenseHoles) != 0 {
				t.Errorf("OffenseHoles = %v, want none (no member has a move)", got.OffenseHoles)
			}
		})
	}
}

// ADR-0401 §2: an offense hole is a single defense type whose team bestMultiplier is below x1
// (TB2 teamCoverage); a party without any move has no offense hole.
func TestRecommendTypesOffenseHoles(t *testing.T) {
	t.Parallel()

	fairyWall := combatant("9001-000", types(balance.TypeFairy), nil)
	tests := []struct {
		name    string
		members []balance.Combatant
		want    string
	}{
		{name: "normal move only", members: []balance.Combatant{fairyWall, combatant("9002-000", types(balance.TypeDark), nil, physical("move-9005", balance.TypeNormal))}, want: "rock/ghost/steel/fairy"},
		{name: "normal and fighting moves on one member", members: []balance.Combatant{fairyWall, combatant("9002-000", types(balance.TypeDark), nil, physical("move-9005", balance.TypeNormal), physical("move-9009", balance.TypeFighting))}, want: "fairy"},
		{name: "normal and fire moves on two members", members: []balance.Combatant{fairyWall, combatant("9002-000", types(balance.TypeDark), nil, physical("move-9005", balance.TypeNormal)), combatant("9003-000", types(balance.TypeDark), nil, attack("move-9001", balance.TypeFire))}, want: "rock/steel/fairy"},
		{name: "a member's own types are not attacks", members: []balance.Combatant{fairyWall, combatant("9002-000", types(balance.TypeFighting), nil, physical("move-9005", balance.TypeNormal))}, want: "rock/ghost/steel/fairy"},
		{name: "no move at all: no offense hole", members: []balance.Combatant{fairyWall, combatant("9002-000", types(balance.TypeDark), nil)}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := recommend(t, chartOffense(), tt.members, nil, nil, balance.MaxRecommendationLimit)
			if typeLabel(got.OffenseHoles) != tt.want {
				t.Errorf("OffenseHoles = %v, want %s (canonical order)", got.OffenseHoles, tt.want)
			}
			if len(got.DefenseHoles) != 0 {
				t.Errorf("DefenseHoles = %v, want none (the fairy member resists everything)", got.DefenseHoles)
			}
		})
	}
}

// ADR-0401 §3: order by defenseCovered + offenseCovered (desc), weaknesses (asc), then the
// canonical order (single types first, duals by first then second type). Hand-counted on
// chartTwoDefenseHoles (see the comment on each group).
func TestRecommendTypesOrderAndCounts(t *testing.T) {
	t.Parallel()

	got := recommend(t, chartTwoDefenseHoles(), normalMember, nil, nil, balance.MaxRecommendationLimit)
	if typeLabel(got.DefenseHoles) != "fire/ice" || len(got.OffenseHoles) != 0 {
		t.Fatalf("holes = %v / %v, want fire/ice and none", got.DefenseHoles, got.OffenseHoles)
	}
	want := []string{
		// total 2, weaknesses 0: grass hits water x2 but normal halves it.
		"normal/water",
		// total 2, weaknesses 1 (grass): the single type first, then duals in canonical order (water/rock is not here).
		"water", "fire/water", "water/electric", "water/grass", "water/ice", "water/fighting", "water/poison",
		"water/ground", "water/flying", "water/psychic", "water/bug", "water/ghost", "water/dragon", "water/dark",
		"water/steel", "water/fairy",
		// total 2, weaknesses 2 (fighting, ground).
		"rock/ghost",
		// total 2, weaknesses 3 (grass, fighting, ground).
		"water/rock",
		// total 1, weaknesses 0: the single ghost comes before every dual.
		"ghost",
	}
	if fmt.Sprint(candidateLabels(got.Candidates)) != fmt.Sprint(want) {
		t.Fatalf("candidates =\n%v\nwant\n%v", candidateLabels(got.Candidates), want)
	}

	counts := []struct {
		label      string
		defense    string
		weaknesses int
	}{
		{"normal/water", "fire/ice", 0},
		{"water", "fire/ice", 1},
		{"water/ghost", "fire/ice", 1},
		{"rock/ghost", "fire/ice", 2},
		{"water/rock", "fire/ice", 3},
		{"ghost", "ice", 0},
	}
	for _, c := range counts {
		candidate := findCandidate(t, got, c.label)
		if typeLabel(candidate.DefenseCovered) != c.defense {
			t.Errorf("%s DefenseCovered = %v, want %s", c.label, candidate.DefenseCovered, c.defense)
		}
		if len(candidate.OffenseCovered) != 0 {
			t.Errorf("%s OffenseCovered = %v, want none", c.label, candidate.OffenseCovered)
		}
		if candidate.Weaknesses != c.weaknesses {
			t.Errorf("%s Weaknesses = %d, want %d", c.label, candidate.Weaknesses, c.weaknesses)
		}
	}
}

// ADR-0401 §3: offenseCovered counts the offense holes that the candidate's own types hit at x1 or more.
func TestRecommendTypesOffenseCovered(t *testing.T) {
	t.Parallel()

	members := []balance.Combatant{
		combatant("9001-000", types(balance.TypeFairy), nil),
		combatant("9002-000", types(balance.TypeDark), nil, physical("move-9005", balance.TypeNormal)),
	}
	got := recommend(t, chartOffense(), members, nil, nil, balance.MaxRecommendationLimit)
	if typeLabel(got.OffenseHoles) != "rock/ghost/steel/fairy" || len(got.DefenseHoles) != 0 {
		t.Fatalf("holes = %v / %v, want none and rock/ghost/steel/fairy", got.DefenseHoles, got.OffenseHoles)
	}
	want := []string{
		// total 3 (rock, ghost, steel), weaknesses 0.
		"fighting", "normal/fighting", "fire/fighting", "water/fighting", "electric/fighting", "grass/fighting",
		"ice/fighting", "fighting/poison", "fighting/ground", "fighting/flying", "fighting/psychic", "fighting/bug",
		"fighting/rock", "fighting/ghost", "fighting/dragon", "fighting/dark", "fighting/fairy",
		// total 3, weaknesses 1 (fighting hits steel x2).
		"fighting/steel",
		// total 2 (rock, ghost), weaknesses 0, single types in canonical order.
		"water", "electric",
	}
	if fmt.Sprint(candidateLabels(got.Candidates)) != fmt.Sprint(want) {
		t.Fatalf("candidates =\n%v\nwant\n%v", candidateLabels(got.Candidates), want)
	}
	for _, c := range []struct{ label, offense string }{
		{"fighting", "rock/ghost/steel"},
		{"normal/fighting", "rock/ghost/steel"},
		{"fighting/steel", "rock/ghost/steel"},
		{"water", "rock/ghost"},
	} {
		candidate := findCandidate(t, got, c.label)
		if typeLabel(candidate.OffenseCovered) != c.offense {
			t.Errorf("%s OffenseCovered = %v, want %s", c.label, candidate.OffenseCovered, c.offense)
		}
		if len(candidate.DefenseCovered) != 0 {
			t.Errorf("%s DefenseCovered = %v, want none", c.label, candidate.DefenseCovered)
		}
	}
}

// ADR-0401 §3: a candidate that fills no hole is left out; fewer candidates than the limit are all returned.
func TestRecommendTypesLeavesOutCandidatesWithoutHoles(t *testing.T) {
	t.Parallel()

	got := recommend(t, chartRockOnly(), normalMember, nil, nil, balance.MaxRecommendationLimit)
	want := []string{
		"rock", "normal/rock", "fire/rock", "water/rock", "electric/rock", "grass/rock", "ice/rock",
		"fighting/rock", "poison/rock", "ground/rock", "flying/rock", "psychic/rock", "bug/rock",
		"rock/ghost", "rock/dragon", "rock/dark", "rock/steel", "rock/fairy",
	}
	if fmt.Sprint(candidateLabels(got.Candidates)) != fmt.Sprint(want) {
		t.Fatalf("candidates =\n%v\nwant\n%v (18: only rock and its duals fill the fire hole)", candidateLabels(got.Candidates), want)
	}

	// No hole at all: no candidate.
	noHole := recommend(t, chartRockOnly(), []balance.Combatant{
		combatant("9001-000", types(balance.TypeNormal), nil),
		combatant("9002-000", types(balance.TypeRock), nil),
	}, nil, nil, balance.MaxRecommendationLimit)
	if len(noHole.DefenseHoles) != 0 || len(noHole.OffenseHoles) != 0 || len(noHole.Candidates) != 0 {
		t.Errorf("no hole: got %+v, want no holes and no candidates", noHole)
	}
}

// ADR-0401 §3: at most limit candidates (1..20), the first ones of the order.
func TestRecommendTypesLimit(t *testing.T) {
	t.Parallel()

	full := recommend(t, chartTwoDefenseHoles(), normalMember, nil, nil, balance.MaxRecommendationLimit)
	fullLabels := candidateLabels(full.Candidates)
	if len(fullLabels) != balance.MaxRecommendationLimit {
		t.Fatalf("limit 20: %d candidates, want 20", len(fullLabels))
	}
	for _, limit := range []int{balance.MinRecommendationLimit, 2, balance.DefaultRecommendationLimit, 19} {
		got := recommend(t, chartTwoDefenseHoles(), normalMember, nil, nil, limit)
		if fmt.Sprint(candidateLabels(got.Candidates)) != fmt.Sprint(fullLabels[:limit]) {
			t.Errorf("limit %d: candidates = %v, want %v", limit, candidateLabels(got.Candidates), fullLabels[:limit])
		}
	}
	if balance.DefaultRecommendationLimit != 10 || balance.MinRecommendationLimit != 1 || balance.MaxRecommendationLimit != 20 {
		t.Errorf("limits = %d/%d/%d, want default 10, 1..20 (ADR-0401 §3)",
			balance.DefaultRecommendationLimit, balance.MinRecommendationLimit, balance.MaxRecommendationLimit)
	}
}

// ADR-0401 §4: each candidate lists every catalog pokemon whose type set equals the candidate's
// (in any order), pokemonId ascending, with nameJa when present and the read model type order.
func TestRecommendTypesCandidatePokemon(t *testing.T) {
	t.Parallel()

	catalog := []balance.CatalogPokemon{
		catalogPokemon("9006-000", "", types(balance.TypeRock, balance.TypeFire)),
		catalogPokemon("9005-000", "テストイワ", types(balance.TypeRock), "ability-9001"),
		catalogPokemon("9004-000", "テストノーマル", types(balance.TypeNormal)),
		catalogPokemon("9003-000", "テストユウレイイワ", types(balance.TypeRock, balance.TypeGhost)),
		catalogPokemon("9002-000", "", types(balance.TypeGhost, balance.TypeRock)),
		catalogPokemon("9001-000", "", types(balance.TypeRock)),
	}
	got := recommend(t, chartRockOnly(), normalMember, catalog, nil, balance.MaxRecommendationLimit)

	type pokemon struct {
		id, name, types string
		exact           bool
	}
	// ADR-0401 §8: a single-type candidate also lists the pokemon that contain its type (exact matches
	// first, then pokemonId); a dual-type candidate lists exact matches only.
	tests := []struct {
		label string
		want  []pokemon
	}{
		{"rock", []pokemon{
			{"9001-000", "", "rock", true}, {"9005-000", "テストイワ", "rock", true},
			{"9002-000", "", "ghost/rock", false}, {"9003-000", "テストユウレイイワ", "rock/ghost", false}, {"9006-000", "", "rock/fire", false},
		}},
		{"rock/ghost", []pokemon{{"9002-000", "", "ghost/rock", true}, {"9003-000", "テストユウレイイワ", "rock/ghost", true}}},
		{"fire/rock", []pokemon{{"9006-000", "", "rock/fire", true}}},
		{"normal/rock", []pokemon{}},
	}
	for _, tt := range tests {
		candidate := findCandidate(t, got, tt.label)
		gotPokemon := make([]pokemon, len(candidate.Pokemon))
		for i, p := range candidate.Pokemon {
			gotPokemon[i] = pokemon{p.PokemonID, p.NameJa, typeLabel(p.Types), p.ExactMatch}
		}
		if fmt.Sprint(gotPokemon) != fmt.Sprint(tt.want) {
			t.Errorf("%s pokemon = %v, want %v", tt.label, gotPokemon, tt.want)
		}
	}
	// The normal-only pokemon matches no candidate (normal fills no hole).
	for _, candidate := range got.Candidates {
		for _, p := range candidate.Pokemon {
			if p.PokemonID == "9004-000" {
				t.Errorf("9004-000 (normal) listed under %s", typeLabel(candidate.Types))
			}
		}
	}
}

// ADR-0401 §4: per defense hole (canonical order), the pokemon whose abilityIds include an ability
// that takes the hole below x1 while their types alone do not, as (pokemon, ability) pairs with the
// final multiplier, pokemonId ascending.
func TestRecommendTypesAbilityOptions(t *testing.T) {
	t.Parallel()

	abilities := recAbilities{
		"ability-9001": recAbility("ability-9001", immuneEffect(balance.TypeFire)),
		"ability-9002": recAbility("ability-9002", absorbEffect(balance.TypeWater)),
		"ability-9003": recAbility("ability-9003", typeFactorEffect(balance.TypeFire, 1, 2)),
		"ability-9004": recAbility("ability-9004", superEffectiveFactorEffect(3, 4)),
		"ability-9005": recAbility("ability-9005"),
		"ability-9006": recAbility("ability-9006", typeFactorEffect(balance.TypeFire, 5, 4)),
	}
	catalog := []balance.CatalogPokemon{
		// grass: fire x2 x1/2 = x1 is not below x1.
		catalogPokemon("9001-000", "", types(balance.TypeGrass), "ability-9003"),
		// grass: fire x2 -> immune. water x1/2 by type alone, so not listed for water.
		catalogPokemon("9002-000", "テストクサ", types(balance.TypeGrass), "ability-9001"),
		// normal: fire x1 x1/2 = x1/2; water absorbed.
		catalogPokemon("9003-000", "", types(balance.TypeNormal), "ability-9003", "ability-9002"),
		// rock: fire x1/2 by type alone, so the immunity does not make it an ability option.
		catalogPokemon("9004-000", "", types(balance.TypeRock), "ability-9001"),
		// grass: fire x2 x3/4 = x3/2.
		catalogPokemon("9005-000", "", types(balance.TypeGrass), "ability-9004"),
		catalogPokemon("9006-000", "", types(balance.TypeNormal)),
		// x1 x5/4 and an ability without effects.
		catalogPokemon("9008-000", "", types(balance.TypeNormal), "ability-9005", "ability-9006"),
		// Two abilities of one pokemon both fill fire: two pairs.
		catalogPokemon("9009-000", "テストノーマル", types(balance.TypeNormal), "ability-9003", "ability-9001"),
		catalogPokemon("9007-000", "", types(balance.TypeNormal), "ability-9005"),
	}
	got := recommend(t, chartAbilityOptions(), normalMember, catalog, abilities, balance.MaxRecommendationLimit)
	if typeLabel(got.DefenseHoles) != "fire/water" {
		t.Fatalf("DefenseHoles = %v, want fire/water", got.DefenseHoles)
	}
	if len(got.AbilityOptions) != 2 {
		t.Fatalf("AbilityOptions = %+v, want one entry per defense hole (fire, water)", got.AbilityOptions)
	}

	type pair struct{ id, name, ability, multiplier string }
	wantOptions := []struct {
		attack balance.TypeID
		want   []pair
	}{
		{balance.TypeFire, []pair{
			{"9002-000", "テストクサ", "ability-9001", "0"},
			{"9003-000", "", "ability-9003", "1/2"},
			{"9009-000", "テストノーマル", "ability-9001", "0"},
			{"9009-000", "テストノーマル", "ability-9003", "1/2"},
		}},
		{balance.TypeWater, []pair{{"9003-000", "", "ability-9002", "0"}}},
	}
	for i, w := range wantOptions {
		option := got.AbilityOptions[i]
		if option.AttackType != w.attack {
			t.Errorf("AbilityOptions[%d].AttackType = %s, want %s (canonical order of the holes)", i, option.AttackType, w.attack)
		}
		pairs := make([]pair, len(option.Pokemon))
		for j, p := range option.Pokemon {
			pairs[j] = pair{p.PokemonID, p.NameJa, p.AbilityID, p.Multiplier.String()}
			if j > 0 && option.Pokemon[j-1].PokemonID > p.PokemonID {
				t.Errorf("AbilityOptions[%d] pokemon not in pokemonId ascending order: %s after %s", i, p.PokemonID, option.Pokemon[j-1].PokemonID)
			}
		}
		// The order of two abilities of the same pokemon is not fixed by ADR-0401; compare as sets.
		sort.Slice(pairs, func(a, b int) bool { return fmt.Sprint(pairs[a]) < fmt.Sprint(pairs[b]) })
		if fmt.Sprint(pairs) != fmt.Sprint(w.want) {
			t.Errorf("AbilityOptions[%d] (%s) = %v, want %v", i, w.attack, pairs, w.want)
		}
	}
}

// ADR-0401 §7.2: a catalog abilityId the ability read model does not know (an export
// inconsistency) is skipped for that one ability only; the pokemon's other abilities, and
// every other pokemon, are still considered. Any other provider failure still propagates.
func TestRecommendTypesSkipsUnknownCatalogAbility(t *testing.T) {
	t.Parallel()

	abilities := recAbilities{
		"ability-9001": recAbility("ability-9001", immuneEffect(balance.TypeFire)),
	}
	catalog := []balance.CatalogPokemon{
		// ability-9999 is not in the ability read model; ability-9001 still applies.
		catalogPokemon("9001-000", "", types(balance.TypeGrass), "ability-9999", "ability-9001"),
		// Every abilityId unknown: no ability option for this pokemon, and no error.
		catalogPokemon("9002-000", "", types(balance.TypeGrass), "ability-9998"),
	}
	got := recommend(t, chartAbilityOptions(), normalMember, catalog, abilities, balance.DefaultRecommendationLimit)
	if len(got.AbilityOptions) != 2 {
		t.Fatalf("AbilityOptions = %+v, want one entry per defense hole (fire, water)", got.AbilityOptions)
	}
	fire := got.AbilityOptions[0]
	if fire.AttackType != balance.TypeFire {
		t.Fatalf("AbilityOptions[0].AttackType = %s, want fire", fire.AttackType)
	}
	if len(fire.Pokemon) != 1 || fire.Pokemon[0].PokemonID != "9001-000" || fire.Pokemon[0].AbilityID != "ability-9001" {
		t.Errorf("AbilityOptions[fire].Pokemon = %+v, want only 9001-000/ability-9001 (ability-9999 skipped, 9002-000 has no known ability)", fire.Pokemon)
	}

	// A provider failure other than ErrUnknownAbility still propagates as an error.
	_, err := balance.RecommendTypes(chartAbilityOptions(), normalMember, catalog, failingRecAbilities{err: errors.New("ability backend exploded")}, balance.DefaultRecommendationLimit)
	if err == nil || errors.Is(err, balance.ErrUnknownAbility) {
		t.Errorf("RecommendTypes() error = %v, want a propagated non-ErrUnknownAbility failure", err)
	}
}

// ADR-0401 §6: without the ability read model, AbilityOptions is empty (not an error), and the
// rest of the result does not change.
func TestRecommendTypesWithoutAbilityProvider(t *testing.T) {
	t.Parallel()

	catalog := []balance.CatalogPokemon{catalogPokemon("9002-000", "", types(balance.TypeGrass), "ability-9001")}
	abilities := recAbilities{"ability-9001": recAbility("ability-9001", immuneEffect(balance.TypeFire))}
	with := recommend(t, chartAbilityOptions(), normalMember, catalog, abilities, balance.DefaultRecommendationLimit)
	without := recommend(t, chartAbilityOptions(), normalMember, catalog, nil, balance.DefaultRecommendationLimit)
	if len(without.AbilityOptions) != 0 {
		t.Errorf("AbilityOptions without a provider = %+v, want empty", without.AbilityOptions)
	}
	if len(with.AbilityOptions) == 0 {
		t.Errorf("AbilityOptions with a provider = %+v, want entries", with.AbilityOptions)
	}
	if fmt.Sprint(with.DefenseHoles, with.OffenseHoles, with.Candidates) != fmt.Sprint(without.DefenseHoles, without.OffenseHoles, without.Candidates) {
		t.Errorf("holes and candidates depend on the ability provider:\nwith    %+v\nwithout %+v", with, without)
	}
}

func TestRecommendTypesRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("chart backend failed")
	abilitySentinel := errors.New("ability backend failed")
	seven := make([]balance.Combatant, 7)
	for i := range seven {
		seven[i] = combatant(fmt.Sprintf("900%d-000", i+1), types(balance.TypeNormal), nil)
	}
	tests := []struct {
		name      string
		chart     balance.TypeChartProvider
		members   []balance.Combatant
		catalog   []balance.CatalogPokemon
		abilities balance.AbilityProvider
		limit     int
		wantErr   error
	}{
		{name: "no member", chart: chartTwoDefenseHoles(), members: nil, limit: 10, wantErr: balance.ErrMemberCount},
		{name: "seven members", chart: chartTwoDefenseHoles(), members: seven, limit: 10, wantErr: balance.ErrMemberCount},
		{name: "limit 0", chart: chartTwoDefenseHoles(), members: normalMember, limit: 0, wantErr: balance.ErrRecommendationLimit},
		{name: "limit 21", chart: chartTwoDefenseHoles(), members: normalMember, limit: 21, wantErr: balance.ErrRecommendationLimit},
		{name: "negative limit", chart: chartTwoDefenseHoles(), members: normalMember, limit: -1, wantErr: balance.ErrRecommendationLimit},
		{name: "nil chart", chart: nil, members: normalMember, limit: 10, wantErr: balance.ErrNilTypeChart},
		{name: "chart failure is propagated", chart: errorTypeChart{err: sentinel}, members: normalMember, limit: 10, wantErr: sentinel},
		{name: "invalid member type", chart: chartTwoDefenseHoles(), members: []balance.Combatant{combatant("9001-000", types("stellar"), nil)}, limit: 10, wantErr: balance.ErrInvalidType},
		{name: "invalid member ability effect", chart: chartTwoDefenseHoles(), members: []balance.Combatant{combatant("9001-000", types(balance.TypeNormal), threatAbility("ability-9001", balance.AbilityEffect{Kind: "heal", AttackType: balance.TypeFire}))}, limit: 10, wantErr: balance.ErrInvalidAbilityEffect},
		{name: "invalid move category", chart: chartTwoDefenseHoles(), members: []balance.Combatant{combatant("9001-000", types(balance.TypeNormal), nil, balance.Move{MoveID: "move-9001", Type: balance.TypeFire, Category: "other"})}, limit: 10, wantErr: balance.ErrInvalidMoveCategory},
		{
			name: "ability provider failure for a catalog pokemon is propagated", chart: chartAbilityOptions(), members: normalMember,
			catalog:   []balance.CatalogPokemon{catalogPokemon("9002-000", "", types(balance.TypeGrass), "ability-9001")},
			abilities: failingRecAbilities{err: abilitySentinel}, limit: 10, wantErr: abilitySentinel,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := balance.RecommendTypes(tt.chart, tt.members, tt.catalog, tt.abilities, tt.limit)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("RecommendTypes() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// oracleCandidate is the test's own brute-force view of one candidate (ADR-0401 §3).
type oracleCandidate struct {
	label             string
	defense, offense  string
	total, weaknesses int
	rank              int // canonicalRank of the type set
}

// canonicalRank is the ADR-0401 §3 tie-breaker: every single type (canonical order) before every
// dual type, duals by their first then second type (both in canonical order).
func canonicalRank(combo []balance.TypeID) int {
	position := make(map[balance.TypeID]int, 18)
	for i, t := range balance.AllTypes() {
		position[t] = i
	}
	if len(combo) == 1 {
		return position[combo[0]]
	}
	return 18 + position[combo[0]]*18 + position[combo[1]]
}

// TestRecommendTypesAgreesWithOracleOnDataChart compares the first 20 candidates with an
// independent brute force over the 171 combinations on the bundled data chart, using only
// CalculateDefenseWithAbility / CalculateDefense / Matchup.
func TestRecommendTypesAgreesWithOracleOnDataChart(t *testing.T) {
	t.Parallel()

	chart := testTypeChart()
	teams := map[string][]balance.Combatant{
		"grass without moves": {combatant("9002-000", types(balance.TypeGrass), nil)},
		"three members with moves and an ability": {
			combatant("9001-000", types(balance.TypeFire, balance.TypeFlying), nil, attack("move-9001", balance.TypeFire)),
			combatant("9003-000", types(balance.TypeWater, balance.TypeGround), threatAbility("ability-9001", immuneEffect(balance.TypeGrass)), physical("move-9007", balance.TypeGround), attack("move-9008", balance.TypeIce)),
			combatant("9005-000", types(balance.TypeIce), nil, status("move-9012", balance.TypePsychic)),
		},
		"normal move only": {combatant("9004-000", types(balance.TypeNormal), nil, physical("move-9005", balance.TypeNormal))},
	}
	for name, members := range teams {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := recommend(t, chart, members, nil, nil, balance.MaxRecommendationLimit)

			// Holes.
			var defenseHoles, offenseHoles []balance.TypeID
			hasAttack := false
			for _, m := range members {
				for _, mv := range m.Moves {
					if mv.Category != balance.MoveCategoryStatus {
						hasAttack = true
					}
				}
			}
			for _, a := range balance.AllTypes() {
				covered := false
				for _, m := range members {
					r, err := balance.CalculateDefenseWithAbility(chart, a, m.Types, m.Ability)
					if err != nil {
						t.Fatal(err)
					}
					if r.Effectiveness.Cmp(balance.Effectiveness{Num: 1, Den: 1}) < 0 {
						covered = true
					}
				}
				if !covered {
					defenseHoles = append(defenseHoles, a)
				}
			}
			if hasAttack {
				for _, d := range balance.AllTypes() {
					best := -1
					for _, m := range members {
						for _, mv := range m.Moves {
							if mv.Category == balance.MoveCategoryStatus {
								continue
							}
							mul, err := chart.Matchup(mv.Type, d)
							if err != nil {
								t.Fatal(err)
							}
							best = max(best, int(mul))
						}
					}
					if best < int(balance.MultiplierNormal) {
						offenseHoles = append(offenseHoles, d)
					}
				}
			}
			if typeLabel(got.DefenseHoles) != typeLabel(defenseHoles) || typeLabel(got.OffenseHoles) != typeLabel(offenseHoles) {
				t.Fatalf("holes = %v / %v, want %v / %v", got.DefenseHoles, got.OffenseHoles, defenseHoles, offenseHoles)
			}

			// Candidates.
			var oracle []oracleCandidate
			for _, combo := range allDefenseTypeCombos() {
				var defense, offense []balance.TypeID
				for _, a := range defenseHoles {
					r, err := balance.CalculateDefense(chart, a, combo)
					if err != nil {
						t.Fatal(err)
					}
					if r.Multiplier < balance.MultiplierNormal {
						defense = append(defense, a)
					}
				}
				for _, d := range offenseHoles {
					for _, own := range combo {
						mul, err := chart.Matchup(own, d)
						if err != nil {
							t.Fatal(err)
						}
						if mul >= balance.MultiplierNormal {
							offense = append(offense, d)
							break
						}
					}
				}
				weaknesses := 0
				for _, a := range balance.AllTypes() {
					r, err := balance.CalculateDefense(chart, a, combo)
					if err != nil {
						t.Fatal(err)
					}
					if r.Multiplier >= balance.MultiplierDouble {
						weaknesses++
					}
				}
				total := len(defense) + len(offense)
				if total == 0 {
					continue
				}
				oracle = append(oracle, oracleCandidate{typeLabel(combo), typeLabel(defense), typeLabel(offense), total, weaknesses, canonicalRank(combo)})
			}
			sort.Slice(oracle, func(i, j int) bool {
				a, b := oracle[i], oracle[j]
				if a.total != b.total {
					return a.total > b.total
				}
				if a.weaknesses != b.weaknesses {
					return a.weaknesses < b.weaknesses
				}
				return a.rank < b.rank
			})
			if len(oracle) > balance.MaxRecommendationLimit {
				oracle = oracle[:balance.MaxRecommendationLimit]
			}
			if len(got.Candidates) != len(oracle) {
				t.Fatalf("candidates = %d %v, want %d", len(got.Candidates), candidateLabels(got.Candidates), len(oracle))
			}
			for i, want := range oracle {
				c := got.Candidates[i]
				gotView := oracleCandidate{typeLabel(c.Types), typeLabel(c.DefenseCovered), typeLabel(c.OffenseCovered), len(c.DefenseCovered) + len(c.OffenseCovered), c.Weaknesses, canonicalRank(c.Types)}
				if gotView != want {
					t.Errorf("candidates[%d] = %+v, want %+v", i, gotView, want)
				}
			}
		})
	}
}

// ADR-0401 §8: a pokemon that contains the single candidate type is left out when its other type
// stops it from taking one of the candidate's covered defense holes below x1.
func TestRecommendTypesSingleCandidateExcludesPokemonThatLoseTheHole(t *testing.T) {
	t.Parallel()

	// The hole is fire (normalMember). rock takes fire x1/2; grass takes fire x2, so rock/grass is back to x1.
	chart := chartRockOnly().set(balance.TypeFire, balance.TypeGrass, dbl)
	catalog := []balance.CatalogPokemon{
		catalogPokemon("9001-000", "", types(balance.TypeRock)),
		catalogPokemon("9002-000", "", types(balance.TypeRock, balance.TypeGrass)),
		catalogPokemon("9003-000", "", types(balance.TypeGhost, balance.TypeRock)),
	}
	got := recommend(t, chart, normalMember, catalog, nil, balance.MaxRecommendationLimit)
	candidate := findCandidate(t, got, "rock")
	ids := make([]string, len(candidate.Pokemon))
	for i, p := range candidate.Pokemon {
		ids[i] = p.PokemonID
	}
	if fmt.Sprint(ids) != fmt.Sprint([]string{"9001-000", "9003-000"}) {
		t.Errorf("rock pokemon = %v, want [9001-000 9003-000] (rock/grass loses the fire hole)", ids)
	}
}
