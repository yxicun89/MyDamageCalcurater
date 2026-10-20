package master_test

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"example.com/pokecalc/services/balance/internal/domain"
)

// TB5 おすすめタイプと該当ポケモン(ADR-0401)。
//
// 手で数え上げられるように、既定が ×1 の小さな架空の相性表(recChart)を主に使う。
// 同梱の相性表(master.EmbeddedTypeChart)では、独立に組んだ oracle と上位を照合する。
// ID はすべて架空(9001-000 / move-9001 / ability-9001 以降)、名前も架空。

// recChart is a fictional type chart: every matchup not listed is x1.
type recChart map[[2]domain.TypeID]domain.Multiplier

func (c recChart) Matchup(attack, defense domain.TypeID) (domain.Multiplier, error) {
	if m, ok := c[[2]domain.TypeID{attack, defense}]; ok {
		return m, nil
	}
	return domain.MultiplierNormal, nil
}

func (c recChart) set(attack, defense domain.TypeID, m domain.Multiplier) recChart {
	c[[2]domain.TypeID{attack, defense}] = m
	return c
}

// resistAllExcept makes defense resist (x1/2) every attack type except the listed ones.
func (c recChart) resistAllExcept(defense domain.TypeID, except ...domain.TypeID) recChart {
	skip := make(map[domain.TypeID]bool, len(except))
	for _, t := range except {
		skip[t] = true
	}
	for _, attack := range domain.AllTypes() {
		if !skip[attack] {
			c.set(attack, defense, domain.MultiplierHalf)
		}
	}
	return c
}

const (
	half = domain.MultiplierHalf
	dbl  = domain.MultiplierDouble
	zero = domain.MultiplierZero
)

// chartTwoDefenseHoles: a normal member resists everything but fire and ice, so the defense
// holes are [fire, ice]. water takes both at x1/2, rock takes fire at x1/2, ghost is immune to ice.
// grass hits water x2, fighting and ground hit rock x2.
func chartTwoDefenseHoles() recChart {
	return recChart{}.resistAllExcept(domain.TypeNormal, domain.TypeFire, domain.TypeIce).
		set(domain.TypeFire, domain.TypeWater, half).
		set(domain.TypeIce, domain.TypeWater, half).
		set(domain.TypeFire, domain.TypeRock, half).
		set(domain.TypeIce, domain.TypeGhost, zero).
		set(domain.TypeGrass, domain.TypeWater, dbl).
		set(domain.TypeFighting, domain.TypeRock, dbl).
		set(domain.TypeGround, domain.TypeRock, dbl)
}

// chartRockOnly: a normal member resists everything but fire; only rock takes fire below x1.
// There are no x2 matchups, so every weaknesses count is 0. Exactly 18 candidates fill the hole
// (rock and the 17 duals with rock).
func chartRockOnly() recChart {
	return recChart{}.resistAllExcept(domain.TypeNormal, domain.TypeFire).
		set(domain.TypeFire, domain.TypeRock, half)
}

// chartOffense: a fairy member resists everything (no defense hole). A dark member's normal move
// hits rock x1/2, ghost x0, steel x1/2 (steel resists everything but fighting) and fairy x1/2, so the
// offense holes are [rock, ghost, steel, fairy]. fire hits rock and steel x1/2; fighting hits steel x2.
// Nothing hits fairy at x1 or more.
func chartOffense() recChart {
	return recChart{}.resistAllExcept(domain.TypeFairy).
		resistAllExcept(domain.TypeSteel, domain.TypeFighting).
		set(domain.TypeFighting, domain.TypeSteel, dbl).
		set(domain.TypeNormal, domain.TypeRock, half).
		set(domain.TypeNormal, domain.TypeGhost, zero).
		set(domain.TypeFire, domain.TypeRock, half)
}

// chartAbilityOptions: a normal member resists everything but fire and water (holes [fire, water]).
// rock takes fire x1/2, grass takes water x1/2 and fire x2.
func chartAbilityOptions() recChart {
	return recChart{}.resistAllExcept(domain.TypeNormal, domain.TypeFire, domain.TypeWater).
		set(domain.TypeFire, domain.TypeRock, half).
		set(domain.TypeWater, domain.TypeGrass, half).
		set(domain.TypeFire, domain.TypeGrass, dbl)
}

// recAbilities is a fictional in-memory AbilityProvider.
type recAbilities map[string]domain.Ability

func (p recAbilities) Ability(id string) (domain.Ability, error) {
	if a, ok := p[id]; ok {
		return a, nil
	}
	return domain.Ability{}, fmt.Errorf("%w: %s", domain.ErrUnknownAbility, id)
}

type failingRecAbilities struct{ err error }

func (f failingRecAbilities) Ability(string) (domain.Ability, error) {
	return domain.Ability{}, f.err
}

func recAbility(id string, effects ...domain.AbilityEffect) domain.Ability {
	return domain.Ability{AbilityID: id, Effects: effects}
}

func catalogPokemon(id, nameJa string, ts []domain.TypeID, abilityIDs ...string) domain.CatalogPokemon {
	return domain.CatalogPokemon{PokemonID: id, NameJa: nameJa, Types: ts, AbilityIDs: abilityIDs}
}

// typeLabel renders a type set as "water" or "water/grass".
func typeLabel(ts []domain.TypeID) string {
	parts := make([]string, len(ts))
	for i, t := range ts {
		parts[i] = string(t)
	}
	return strings.Join(parts, "/")
}

func candidateLabels(candidates []domain.TypeCandidate) []string {
	labels := make([]string, len(candidates))
	for i, c := range candidates {
		labels[i] = typeLabel(c.Types)
	}
	return labels
}

func recommend(t *testing.T, chart domain.TypeChartProvider, members []domain.Combatant, catalog []domain.CatalogPokemon, abilities domain.AbilityProvider, limit int) domain.Recommendation {
	t.Helper()
	got, err := domain.RecommendTypes(chart, members, catalog, abilities, limit)
	if err != nil {
		t.Fatalf("RecommendTypes() error = %v", err)
	}
	return got
}

func findCandidate(t *testing.T, got domain.Recommendation, label string) domain.TypeCandidate {
	t.Helper()
	for _, c := range got.Candidates {
		if typeLabel(c.Types) == label {
			return c
		}
	}
	t.Fatalf("candidate %s not found in %v", label, candidateLabels(got.Candidates))
	return domain.TypeCandidate{}
}

var normalMember = []domain.Combatant{combatant("9001-000", types(domain.TypeNormal), nil)}

// ADR-0401 §2: a defense hole is an attack type that no member resists or is immune to
// (resist + immune = 0), with the members' abilities (TB3) applied.
func TestRecommendTypesDefenseHoles(t *testing.T) {
	t.Parallel()

	normal := func(ability *domain.Ability) domain.Combatant {
		return combatant("9001-000", types(domain.TypeNormal), ability)
	}
	allButFire := make([]string, 0, 17)
	for _, attack := range domain.AllTypes() {
		if attack != domain.TypeFire {
			allButFire = append(allButFire, string(attack))
		}
	}
	tests := []struct {
		name    string
		members []domain.Combatant
		want    string
	}{
		{name: "no resistance to fire and ice", members: []domain.Combatant{normal(nil)}, want: "fire/ice"},
		{name: "ability immune to fire", members: []domain.Combatant{normal(threatAbility("ability-9001", immuneEffect(domain.TypeFire)))}, want: "ice"},
		{name: "ability absorbs ice", members: []domain.Combatant{normal(threatAbility("ability-9002", absorbEffect(domain.TypeIce)))}, want: "fire"},
		{name: "ability halves fire", members: []domain.Combatant{normal(threatAbility("ability-9003", typeFactorEffect(domain.TypeFire, 1, 2)))}, want: "ice"},
		{name: "ability x5/4 on fire is not a resistance", members: []domain.Combatant{normal(threatAbility("ability-9006", typeFactorEffect(domain.TypeFire, 5, 4)))}, want: "fire/ice"},
		{name: "super effective x3/4 does not touch neutral hits", members: []domain.Combatant{normal(threatAbility("ability-9004", superEffectiveFactorEffect(3, 4)))}, want: "fire/ice"},
		{name: "ability without effects", members: []domain.Combatant{normal(threatAbility("ability-9005"))}, want: "fire/ice"},
		{name: "a rock member resists fire", members: []domain.Combatant{normal(nil), combatant("9002-000", types(domain.TypeRock), nil)}, want: "ice"},
		{name: "a ghost member is immune to ice", members: []domain.Combatant{normal(nil), combatant("9003-000", types(domain.TypeGhost), nil)}, want: "fire"},
		{name: "a water member resists both", members: []domain.Combatant{normal(nil), combatant("9004-000", types(domain.TypeWater), nil)}, want: ""},
		{name: "a lone rock member only resists fire (its weaknesses do not matter)", members: []domain.Combatant{combatant("9002-000", types(domain.TypeRock), nil)}, want: strings.Join(allButFire, "/")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := recommend(t, chartTwoDefenseHoles(), tt.members, nil, nil, domain.MaxRecommendationLimit)
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

	fairyWall := combatant("9001-000", types(domain.TypeFairy), nil)
	tests := []struct {
		name    string
		members []domain.Combatant
		want    string
	}{
		{name: "normal move only", members: []domain.Combatant{fairyWall, combatant("9002-000", types(domain.TypeDark), nil, physical("move-9005", domain.TypeNormal))}, want: "rock/ghost/steel/fairy"},
		{name: "normal and fighting moves on one member", members: []domain.Combatant{fairyWall, combatant("9002-000", types(domain.TypeDark), nil, physical("move-9005", domain.TypeNormal), physical("move-9009", domain.TypeFighting))}, want: "fairy"},
		{name: "normal and fire moves on two members", members: []domain.Combatant{fairyWall, combatant("9002-000", types(domain.TypeDark), nil, physical("move-9005", domain.TypeNormal)), combatant("9003-000", types(domain.TypeDark), nil, attack("move-9001", domain.TypeFire))}, want: "rock/steel/fairy"},
		{name: "a member's own types are not attacks", members: []domain.Combatant{fairyWall, combatant("9002-000", types(domain.TypeFighting), nil, physical("move-9005", domain.TypeNormal))}, want: "rock/ghost/steel/fairy"},
		{name: "no move at all: no offense hole", members: []domain.Combatant{fairyWall, combatant("9002-000", types(domain.TypeDark), nil)}, want: ""},
		// ADR-0401 §7.1: status moves are not attacks, so a party with status moves only has no offense hole either.
		{name: "status moves only: no offense hole", members: []domain.Combatant{fairyWall, combatant("9002-000", types(domain.TypeDark), nil, status("move-9012", domain.TypePsychic))}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := recommend(t, chartOffense(), tt.members, nil, nil, domain.MaxRecommendationLimit)
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

	got := recommend(t, chartTwoDefenseHoles(), normalMember, nil, nil, domain.MaxRecommendationLimit)
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

	members := []domain.Combatant{
		combatant("9001-000", types(domain.TypeFairy), nil),
		combatant("9002-000", types(domain.TypeDark), nil, physical("move-9005", domain.TypeNormal)),
	}
	got := recommend(t, chartOffense(), members, nil, nil, domain.MaxRecommendationLimit)
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

	got := recommend(t, chartRockOnly(), normalMember, nil, nil, domain.MaxRecommendationLimit)
	want := []string{
		"rock", "normal/rock", "fire/rock", "water/rock", "electric/rock", "grass/rock", "ice/rock",
		"fighting/rock", "poison/rock", "ground/rock", "flying/rock", "psychic/rock", "bug/rock",
		"rock/ghost", "rock/dragon", "rock/dark", "rock/steel", "rock/fairy",
	}
	if fmt.Sprint(candidateLabels(got.Candidates)) != fmt.Sprint(want) {
		t.Fatalf("candidates =\n%v\nwant\n%v (18: only rock and its duals fill the fire hole)", candidateLabels(got.Candidates), want)
	}

	// No hole at all: no candidate.
	noHole := recommend(t, chartRockOnly(), []domain.Combatant{
		combatant("9001-000", types(domain.TypeNormal), nil),
		combatant("9002-000", types(domain.TypeRock), nil),
	}, nil, nil, domain.MaxRecommendationLimit)
	if len(noHole.DefenseHoles) != 0 || len(noHole.OffenseHoles) != 0 || len(noHole.Candidates) != 0 {
		t.Errorf("no hole: got %+v, want no holes and no candidates", noHole)
	}
}

// ADR-0401 §3: at most limit candidates (1..20), the first ones of the order.
func TestRecommendTypesLimit(t *testing.T) {
	t.Parallel()

	full := recommend(t, chartTwoDefenseHoles(), normalMember, nil, nil, domain.MaxRecommendationLimit)
	fullLabels := candidateLabels(full.Candidates)
	if len(fullLabels) != domain.MaxRecommendationLimit {
		t.Fatalf("limit 20: %d candidates, want 20", len(fullLabels))
	}
	for _, limit := range []int{domain.MinRecommendationLimit, 2, domain.DefaultRecommendationLimit, 19} {
		got := recommend(t, chartTwoDefenseHoles(), normalMember, nil, nil, limit)
		if fmt.Sprint(candidateLabels(got.Candidates)) != fmt.Sprint(fullLabels[:limit]) {
			t.Errorf("limit %d: candidates = %v, want %v", limit, candidateLabels(got.Candidates), fullLabels[:limit])
		}
	}
	if domain.DefaultRecommendationLimit != 10 || domain.MinRecommendationLimit != 1 || domain.MaxRecommendationLimit != 20 {
		t.Errorf("limits = %d/%d/%d, want default 10, 1..20 (ADR-0401 §3)",
			domain.DefaultRecommendationLimit, domain.MinRecommendationLimit, domain.MaxRecommendationLimit)
	}
}

// ADR-0401 §4: each candidate lists every catalog pokemon whose type set equals the candidate's
// (in any order), pokemonId ascending, with nameJa when present and the read model type order.
func TestRecommendTypesCandidatePokemon(t *testing.T) {
	t.Parallel()

	catalog := []domain.CatalogPokemon{
		catalogPokemon("9006-000", "", types(domain.TypeRock, domain.TypeFire)),
		catalogPokemon("9005-000", "テストイワ", types(domain.TypeRock), "ability-9001"),
		catalogPokemon("9004-000", "テストノーマル", types(domain.TypeNormal)),
		catalogPokemon("9003-000", "テストユウレイイワ", types(domain.TypeRock, domain.TypeGhost)),
		catalogPokemon("9002-000", "", types(domain.TypeGhost, domain.TypeRock)),
		catalogPokemon("9001-000", "", types(domain.TypeRock)),
	}
	got := recommend(t, chartRockOnly(), normalMember, catalog, nil, domain.MaxRecommendationLimit)

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
		"ability-9001": recAbility("ability-9001", immuneEffect(domain.TypeFire)),
		"ability-9002": recAbility("ability-9002", absorbEffect(domain.TypeWater)),
		"ability-9003": recAbility("ability-9003", typeFactorEffect(domain.TypeFire, 1, 2)),
		"ability-9004": recAbility("ability-9004", superEffectiveFactorEffect(3, 4)),
		"ability-9005": recAbility("ability-9005"),
		"ability-9006": recAbility("ability-9006", typeFactorEffect(domain.TypeFire, 5, 4)),
	}
	catalog := []domain.CatalogPokemon{
		// grass: fire x2 x1/2 = x1 is not below x1.
		catalogPokemon("9001-000", "", types(domain.TypeGrass), "ability-9003"),
		// grass: fire x2 -> immune. water x1/2 by type alone, so not listed for water.
		catalogPokemon("9002-000", "テストクサ", types(domain.TypeGrass), "ability-9001"),
		// normal: fire x1 x1/2 = x1/2; water absorbed.
		catalogPokemon("9003-000", "", types(domain.TypeNormal), "ability-9003", "ability-9002"),
		// rock: fire x1/2 by type alone, so the immunity does not make it an ability option.
		catalogPokemon("9004-000", "", types(domain.TypeRock), "ability-9001"),
		// grass: fire x2 x3/4 = x3/2.
		catalogPokemon("9005-000", "", types(domain.TypeGrass), "ability-9004"),
		catalogPokemon("9006-000", "", types(domain.TypeNormal)),
		// x1 x5/4 and an ability without effects.
		catalogPokemon("9008-000", "", types(domain.TypeNormal), "ability-9005", "ability-9006"),
		// Two abilities of one pokemon both fill fire: two pairs.
		catalogPokemon("9009-000", "テストノーマル", types(domain.TypeNormal), "ability-9003", "ability-9001"),
		catalogPokemon("9007-000", "", types(domain.TypeNormal), "ability-9005"),
	}
	got := recommend(t, chartAbilityOptions(), normalMember, catalog, abilities, domain.MaxRecommendationLimit)
	if typeLabel(got.DefenseHoles) != "fire/water" {
		t.Fatalf("DefenseHoles = %v, want fire/water", got.DefenseHoles)
	}
	if len(got.AbilityOptions) != 2 {
		t.Fatalf("AbilityOptions = %+v, want one entry per defense hole (fire, water)", got.AbilityOptions)
	}

	type pair struct{ id, name, ability, multiplier string }
	wantOptions := []struct {
		attack domain.TypeID
		want   []pair
	}{
		{domain.TypeFire, []pair{
			{"9002-000", "テストクサ", "ability-9001", "0"},
			{"9003-000", "", "ability-9003", "1/2"},
			{"9009-000", "テストノーマル", "ability-9001", "0"},
			{"9009-000", "テストノーマル", "ability-9003", "1/2"},
		}},
		{domain.TypeWater, []pair{{"9003-000", "", "ability-9002", "0"}}},
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
		"ability-9001": recAbility("ability-9001", immuneEffect(domain.TypeFire)),
	}
	catalog := []domain.CatalogPokemon{
		// ability-9999 is not in the ability read model; ability-9001 still applies.
		catalogPokemon("9001-000", "", types(domain.TypeGrass), "ability-9999", "ability-9001"),
		// Every abilityId unknown: no ability option for this pokemon, and no error.
		catalogPokemon("9002-000", "", types(domain.TypeGrass), "ability-9998"),
	}
	got := recommend(t, chartAbilityOptions(), normalMember, catalog, abilities, domain.DefaultRecommendationLimit)
	if len(got.AbilityOptions) != 2 {
		t.Fatalf("AbilityOptions = %+v, want one entry per defense hole (fire, water)", got.AbilityOptions)
	}
	fire := got.AbilityOptions[0]
	if fire.AttackType != domain.TypeFire {
		t.Fatalf("AbilityOptions[0].AttackType = %s, want fire", fire.AttackType)
	}
	if len(fire.Pokemon) != 1 || fire.Pokemon[0].PokemonID != "9001-000" || fire.Pokemon[0].AbilityID != "ability-9001" {
		t.Errorf("AbilityOptions[fire].Pokemon = %+v, want only 9001-000/ability-9001 (ability-9999 skipped, 9002-000 has no known ability)", fire.Pokemon)
	}

	// A provider failure other than ErrUnknownAbility still propagates as an error.
	_, err := domain.RecommendTypes(chartAbilityOptions(), normalMember, catalog, failingRecAbilities{err: errors.New("ability backend exploded")}, domain.DefaultRecommendationLimit)
	if err == nil || errors.Is(err, domain.ErrUnknownAbility) {
		t.Errorf("RecommendTypes() error = %v, want a propagated non-ErrUnknownAbility failure", err)
	}
}

// ADR-0401 §6: without the ability read model, AbilityOptions is empty (not an error), and the
// rest of the result does not change.
func TestRecommendTypesWithoutAbilityProvider(t *testing.T) {
	t.Parallel()

	catalog := []domain.CatalogPokemon{catalogPokemon("9002-000", "", types(domain.TypeGrass), "ability-9001")}
	abilities := recAbilities{"ability-9001": recAbility("ability-9001", immuneEffect(domain.TypeFire))}
	with := recommend(t, chartAbilityOptions(), normalMember, catalog, abilities, domain.DefaultRecommendationLimit)
	without := recommend(t, chartAbilityOptions(), normalMember, catalog, nil, domain.DefaultRecommendationLimit)
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
	seven := make([]domain.Combatant, 7)
	for i := range seven {
		seven[i] = combatant(fmt.Sprintf("900%d-000", i+1), types(domain.TypeNormal), nil)
	}
	tests := []struct {
		name      string
		chart     domain.TypeChartProvider
		members   []domain.Combatant
		catalog   []domain.CatalogPokemon
		abilities domain.AbilityProvider
		limit     int
		wantErr   error
	}{
		{name: "no member", chart: chartTwoDefenseHoles(), members: nil, limit: 10, wantErr: domain.ErrMemberCount},
		{name: "seven members", chart: chartTwoDefenseHoles(), members: seven, limit: 10, wantErr: domain.ErrMemberCount},
		{name: "limit 0", chart: chartTwoDefenseHoles(), members: normalMember, limit: 0, wantErr: domain.ErrRecommendationLimit},
		{name: "limit 21", chart: chartTwoDefenseHoles(), members: normalMember, limit: 21, wantErr: domain.ErrRecommendationLimit},
		{name: "negative limit", chart: chartTwoDefenseHoles(), members: normalMember, limit: -1, wantErr: domain.ErrRecommendationLimit},
		{name: "nil chart", chart: nil, members: normalMember, limit: 10, wantErr: domain.ErrNilTypeChart},
		{name: "chart failure is propagated", chart: errorTypeChart{err: sentinel}, members: normalMember, limit: 10, wantErr: sentinel},
		{name: "invalid member type", chart: chartTwoDefenseHoles(), members: []domain.Combatant{combatant("9001-000", types("stellar"), nil)}, limit: 10, wantErr: domain.ErrInvalidType},
		{name: "invalid member ability effect", chart: chartTwoDefenseHoles(), members: []domain.Combatant{combatant("9001-000", types(domain.TypeNormal), threatAbility("ability-9001", domain.AbilityEffect{Kind: "heal", AttackType: domain.TypeFire}))}, limit: 10, wantErr: domain.ErrInvalidAbilityEffect},
		{name: "invalid move category", chart: chartTwoDefenseHoles(), members: []domain.Combatant{combatant("9001-000", types(domain.TypeNormal), nil, domain.Move{MoveID: "move-9001", Type: domain.TypeFire, Category: "other"})}, limit: 10, wantErr: domain.ErrInvalidMoveCategory},
		{
			name: "ability provider failure for a catalog pokemon is propagated", chart: chartAbilityOptions(), members: normalMember,
			catalog:   []domain.CatalogPokemon{catalogPokemon("9002-000", "", types(domain.TypeGrass), "ability-9001")},
			abilities: failingRecAbilities{err: abilitySentinel}, limit: 10, wantErr: abilitySentinel,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.RecommendTypes(tt.chart, tt.members, tt.catalog, tt.abilities, tt.limit)
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
func canonicalRank(combo []domain.TypeID) int {
	position := make(map[domain.TypeID]int, 18)
	for i, t := range domain.AllTypes() {
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
	teams := map[string][]domain.Combatant{
		"grass without moves": {combatant("9002-000", types(domain.TypeGrass), nil)},
		"three members with moves and an ability": {
			combatant("9001-000", types(domain.TypeFire, domain.TypeFlying), nil, attack("move-9001", domain.TypeFire)),
			combatant("9003-000", types(domain.TypeWater, domain.TypeGround), threatAbility("ability-9001", immuneEffect(domain.TypeGrass)), physical("move-9007", domain.TypeGround), attack("move-9008", domain.TypeIce)),
			combatant("9005-000", types(domain.TypeIce), nil, status("move-9012", domain.TypePsychic)),
		},
		"normal move only": {combatant("9004-000", types(domain.TypeNormal), nil, physical("move-9005", domain.TypeNormal))},
		// ADR-0422: three grass members are lopsided against fire, ice, poison, flying and bug.
		"three grass members are skewed": {
			combatant("9002-000", types(domain.TypeGrass), nil),
			combatant("9006-000", types(domain.TypeGrass), nil),
			combatant("9007-000", types(domain.TypeGrass), nil),
		},
	}
	for name, members := range teams {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := recommend(t, chart, members, nil, nil, domain.MaxRecommendationLimit)

			// Skewed attack types (ADR-0422 §1), counted independently from the per-member effectiveness.
			var skewed []domain.TypeID
			for _, a := range domain.AllTypes() {
				weak, resistOrImmune := 0, 0
				for _, m := range members {
					r, err := domain.CalculateDefenseWithAbility(chart, a, m.Types, m.Ability)
					if err != nil {
						t.Fatal(err)
					}
					switch {
					case r.Effectiveness.Cmp(domain.Effectiveness{Num: 2, Den: 1}) >= 0:
						weak++
					case r.Effectiveness.Cmp(domain.Effectiveness{Num: 1, Den: 1}) < 0:
						resistOrImmune++
					}
				}
				if weak-resistOrImmune >= 2 {
					skewed = append(skewed, a)
				}
			}
			if typeLabel(got.SkewedAttackTypes) != typeLabel(skewed) {
				t.Fatalf("SkewedAttackTypes = %v, want %v", got.SkewedAttackTypes, skewed)
			}

			// Holes.
			var defenseHoles, offenseHoles []domain.TypeID
			hasAttack := false
			for _, m := range members {
				for _, mv := range m.Moves {
					if mv.Category != domain.MoveCategoryStatus {
						hasAttack = true
					}
				}
			}
			for _, a := range domain.AllTypes() {
				covered := false
				for _, m := range members {
					r, err := domain.CalculateDefenseWithAbility(chart, a, m.Types, m.Ability)
					if err != nil {
						t.Fatal(err)
					}
					if r.Effectiveness.Cmp(domain.Effectiveness{Num: 1, Den: 1}) < 0 {
						covered = true
					}
				}
				if !covered {
					defenseHoles = append(defenseHoles, a)
				}
			}
			if hasAttack {
				for _, d := range domain.AllTypes() {
					best := -1
					for _, m := range members {
						for _, mv := range m.Moves {
							if mv.Category == domain.MoveCategoryStatus {
								continue
							}
							mul, err := chart.Matchup(mv.Type, d)
							if err != nil {
								t.Fatal(err)
							}
							best = max(best, int(mul))
						}
					}
					if best < int(domain.MultiplierNormal) {
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
				weakToSkewed := false
				for _, a := range skewed {
					r, err := domain.CalculateDefense(chart, a, combo)
					if err != nil {
						t.Fatal(err)
					}
					if r.Multiplier >= domain.MultiplierDouble {
						weakToSkewed = true
					}
				}
				if weakToSkewed {
					continue
				}
				var defense, offense []domain.TypeID
				for _, a := range defenseHoles {
					r, err := domain.CalculateDefense(chart, a, combo)
					if err != nil {
						t.Fatal(err)
					}
					if r.Multiplier < domain.MultiplierNormal {
						defense = append(defense, a)
					}
				}
				for _, d := range offenseHoles {
					for _, own := range combo {
						mul, err := chart.Matchup(own, d)
						if err != nil {
							t.Fatal(err)
						}
						if mul >= domain.MultiplierNormal {
							offense = append(offense, d)
							break
						}
					}
				}
				weaknesses := 0
				for _, a := range domain.AllTypes() {
					r, err := domain.CalculateDefense(chart, a, combo)
					if err != nil {
						t.Fatal(err)
					}
					if r.Multiplier >= domain.MultiplierDouble {
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
			if len(oracle) > domain.MaxRecommendationLimit {
				oracle = oracle[:domain.MaxRecommendationLimit]
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
	chart := chartRockOnly().set(domain.TypeFire, domain.TypeGrass, dbl)
	catalog := []domain.CatalogPokemon{
		catalogPokemon("9001-000", "", types(domain.TypeRock)),
		catalogPokemon("9002-000", "", types(domain.TypeRock, domain.TypeGrass)),
		catalogPokemon("9003-000", "", types(domain.TypeGhost, domain.TypeRock)),
	}
	got := recommend(t, chart, normalMember, catalog, nil, domain.MaxRecommendationLimit)
	candidate := findCandidate(t, got, "rock")
	ids := make([]string, len(candidate.Pokemon))
	for i, p := range candidate.Pokemon {
		ids[i] = p.PokemonID
	}
	if fmt.Sprint(ids) != fmt.Sprint([]string{"9001-000", "9003-000"}) {
		t.Errorf("rock pokemon = %v, want [9001-000 9003-000] (rock/grass loses the fire hole)", ids)
	}
}

// brokenColumnChart resists every matchup (x1/2) except that any matchup against one defense type fails.
type brokenColumnChart struct{ broken domain.TypeID }

func (c brokenColumnChart) Matchup(attack, defense domain.TypeID) (domain.Multiplier, error) {
	if defense == c.broken {
		return 0, errors.New("broken column")
	}
	return domain.MultiplierHalf, nil
}

// A broken type chart is always an error (as in AnalyzeDefense), even when an earlier member
// already resists every attack type: the holes are derived from the full team aggregation, so the
// water member's broken column is always evaluated (a per-type loop that stops early would not).
func TestRecommendTypesBrokenChartIsAlwaysAnError(t *testing.T) {
	t.Parallel()

	chart := brokenColumnChart{broken: domain.TypeWater}
	members := []domain.Combatant{
		combatant("9001-000", types(domain.TypeGrass), nil),
		combatant("9002-000", types(domain.TypeWater), nil),
	}
	if _, err := domain.RecommendTypes(chart, members, nil, nil, domain.MaxRecommendationLimit); err == nil {
		t.Fatal("err = nil, want the chart failure")
	}
}

// ADR-0422 §1: an attack type is skewed when weak - (resist + immune) >= 2. A candidate that takes
// any skewed attack type at x2 or more is left out (it would add a weakness to an already skewed one).
//
// chartSkewedFire: a normal member resists everything but fire and ice; fire hits normal x2 and
// rock takes fire at x1/2. Only grass takes ice below x1 (and fire x2), so with rock in the party the
// only defense hole is ice and every candidate (grass and its duals) is weak to fire.
func chartSkewedFire() recChart {
	return recChart{}.resistAllExcept(domain.TypeNormal, domain.TypeFire, domain.TypeIce).
		set(domain.TypeFire, domain.TypeNormal, dbl).
		set(domain.TypeFire, domain.TypeRock, half).
		set(domain.TypeIce, domain.TypeGrass, half).
		set(domain.TypeFire, domain.TypeGrass, dbl)
}

func normalsAndRock(normals int) []domain.Combatant {
	members := []domain.Combatant{combatant("9100-000", types(domain.TypeRock), nil)}
	for i := 0; i < normals; i++ {
		members = append(members, combatant(fmt.Sprintf("910%d-000", i+1), types(domain.TypeNormal), nil))
	}
	return members
}

func TestRecommendTypesLeavesOutCandidatesWeakToSkewedAttackType(t *testing.T) {
	t.Parallel()

	// 3 normals weak to fire, 1 rock resists it: 3 - 1 = 2 -> fire is skewed.
	got := recommend(t, chartSkewedFire(), normalsAndRock(3), nil, nil, domain.MaxRecommendationLimit)
	if typeLabel(got.SkewedAttackTypes) != "fire" {
		t.Fatalf("SkewedAttackTypes = %v, want [fire]", got.SkewedAttackTypes)
	}
	if typeLabel(got.DefenseHoles) != "ice" {
		t.Fatalf("DefenseHoles = %v, want [ice]", got.DefenseHoles)
	}
	// grass and its duals are weak to fire, except grass/rock where rock halves fire back to x1.
	if fmt.Sprint(candidateLabels(got.Candidates)) != "[grass/rock]" {
		t.Fatalf("candidates = %v, want only [grass/rock]", candidateLabels(got.Candidates))
	}
}

func TestRecommendTypesKeepsWeakCandidatesWhenResistanceBalancesWeakness(t *testing.T) {
	t.Parallel()

	// 2 normals weak to fire, 1 rock resists it: 2 - 1 = 1 < 2 -> not skewed, grass stays.
	got := recommend(t, chartSkewedFire(), normalsAndRock(2), nil, nil, domain.MaxRecommendationLimit)
	if len(got.SkewedAttackTypes) != 0 {
		t.Fatalf("SkewedAttackTypes = %v, want none (2 weak - 1 resist)", got.SkewedAttackTypes)
	}
	if typeLabel(findCandidate(t, got, "grass").DefenseCovered) != "ice" {
		t.Errorf("grass should be a candidate while nothing is skewed")
	}
}
