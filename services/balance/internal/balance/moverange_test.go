package balance_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"example.com/pokecalc/services/balance/internal/balance"
)

// TB6 技範囲チェッカー(ADR-0404)。技 ID だけを入力に、攻撃範囲と「半減以下で受けられる
// 実在ポケモン」を出す。ID・名前はすべて架空(9001-000 / move-9001 / ability-9001 以降)。
//
// 期待値を手で数えられるように、既定が ×1 の架空の相性表(moveRangeChart)を使う。
// 同梱の相性表では、TB2 の1メンバー分の coverage と一致することを照合する(ADR-0404 §3:
// typeChart は AnalyzeCoverage の防御配列と同じ計算。二重実装しない)。

const (
	mrNormal   = balance.TypeNormal
	mrFire     = balance.TypeFire
	mrWater    = balance.TypeWater
	mrElectric = balance.TypeElectric
	mrGrass    = balance.TypeGrass
	mrRock     = balance.TypeRock
	mrGhost    = balance.TypeGhost
)

// moveRangeChart is a fictional type chart (every matchup not listed is x1):
//
//	fire   → fire x1/2, water x1/2, rock x1/2, grass x2
//	water  → water x1/2, grass x1/2, fire x2, rock x2
//	normal → rock x1/2, ghost x0
func moveRangeChart() recChart {
	return recChart{}.
		set(mrFire, mrFire, half).set(mrFire, mrWater, half).set(mrFire, mrRock, half).set(mrFire, mrGrass, dbl).
		set(mrWater, mrWater, half).set(mrWater, mrGrass, half).set(mrWater, mrFire, dbl).set(mrWater, mrRock, dbl).
		set(mrNormal, mrRock, half).set(mrNormal, mrGhost, zero)
}

func mrMove(id string, t balance.TypeID, category balance.MoveCategory) balance.Move {
	return balance.Move{MoveID: id, Type: t, Category: category}
}

var (
	mrFireMove     = mrMove("move-9001", mrFire, balance.MoveCategorySpecial)
	mrFireMove2    = mrMove("move-9002", mrFire, balance.MoveCategoryPhysical)
	mrWaterMove    = mrMove("move-9003", mrWater, balance.MoveCategorySpecial)
	mrElectricMove = mrMove("move-9004", mrElectric, balance.MoveCategorySpecial)
	mrNormalMove   = mrMove("move-9005", mrNormal, balance.MoveCategoryPhysical)
	mrStatusMove   = mrMove("move-9006", mrGrass, balance.MoveCategoryStatus)
	mrStatusMove2  = mrMove("move-9012", mrRock, balance.MoveCategoryStatus)
)

// mrCatalog is a fictional read model catalog, deliberately not in pokemonId order so the
// ADR-0404 §2 ordering (pokemonId ascending) is actually exercised.
var mrCatalog = []balance.CatalogPokemon{
	{PokemonID: "9002-000", Types: []balance.TypeID{mrWater, mrRock}},
	{PokemonID: "9001-000", NameJa: "テストミズ", Types: []balance.TypeID{mrWater}, AbilityIDs: []string{"ability-9003"}},
	{PokemonID: "9003-000", NameJa: "テストクサ", Types: []balance.TypeID{mrGrass}, AbilityIDs: []string{"ability-9002", "ability-9001"}},
	{PokemonID: "9004-000", Types: []balance.TypeID{mrNormal}, AbilityIDs: []string{"ability-9003"}},
	{PokemonID: "9005-000", NameJa: "テストゴースト", Types: []balance.TypeID{mrGhost}, AbilityIDs: []string{"ability-9004"}},
	{PokemonID: "9002-001", NameJa: "テストイワ", Types: []balance.TypeID{mrRock}},
}

// mrAbilities: ability-9001 is immune to fire, ability-9002 absorbs fire, ability-9003 halves
// fire, ability-9004 multiplies fire by 5/4 (which never brings anything to x1/2 or less).
type mrAbilityProvider map[string]balance.Ability

func (p mrAbilityProvider) Ability(abilityID string) (balance.Ability, error) {
	if ability, ok := p[abilityID]; ok {
		return ability, nil
	}
	return balance.Ability{}, &balance.UnknownAbilityError{AbilityID: abilityID}
}

func mrAbility(id string, effects ...balance.AbilityEffect) balance.Ability {
	return balance.Ability{AbilityID: id, Effects: effects}
}

func mrFraction(num, den int64) balance.Effectiveness {
	e, err := balance.NewEffectiveness(num, den)
	if err != nil {
		panic(err)
	}
	return e
}

var mrAbilities = mrAbilityProvider{
	"ability-9001": mrAbility("ability-9001", balance.AbilityEffect{Kind: balance.AbilityEffectImmune, AttackType: mrFire}),
	"ability-9002": mrAbility("ability-9002", balance.AbilityEffect{Kind: balance.AbilityEffectAbsorb, AttackType: mrFire}),
	"ability-9003": mrAbility("ability-9003", balance.AbilityEffect{Kind: balance.AbilityEffectTypeMultiplier, AttackType: mrFire, Factor: mrFraction(1, 2)}),
	"ability-9004": mrAbility("ability-9004", balance.AbilityEffect{Kind: balance.AbilityEffectTypeMultiplier, AttackType: mrFire, Factor: mrFraction(5, 4)}),
}

// typeLabels joins type IDs for readable failures.
func typeLabels(types []balance.TypeID) string {
	parts := make([]string, len(types))
	for i, t := range types {
		parts[i] = string(t)
	}
	return strings.Join(parts, "/")
}

// ADR-0404 §2: attackTypes は変化技を除いた技のタイプ(重複なし・正準順)。
func TestAnalyzeMoveRangeAttackTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		moves []balance.Move
		want  string
	}{
		{name: "one attack move", moves: []balance.Move{mrFireMove}, want: "fire"},
		{name: "two moves of the same type count once", moves: []balance.Move{mrFireMove, mrFireMove2}, want: "fire"},
		{name: "canonical order, not request order", moves: []balance.Move{mrWaterMove, mrFireMove, mrNormalMove}, want: "normal/fire/water"},
		{name: "status moves are excluded", moves: []balance.Move{mrStatusMove, mrFireMove, mrStatusMove2}, want: "fire"},
		{name: "four attack moves", moves: []balance.Move{mrNormalMove, mrFireMove, mrWaterMove, mrElectricMove}, want: "normal/fire/water/electric"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			analysis, err := balance.AnalyzeMoveRange(moveRangeChart(), tt.moves, mrCatalog, mrAbilities)
			if err != nil {
				t.Fatalf("AnalyzeMoveRange: %v", err)
			}
			if got := typeLabels(analysis.AttackTypes); got != tt.want {
				t.Errorf("attackTypes = %s, want %s", got, tt.want)
			}
		})
	}
}

// ADR-0404 §2: typeChart は18の単防御タイプ(正準順)ごとの最大倍率。
// 期待値は moveRangeChart から手で数えた18件(normal … fairy の順)。
// effective(×1以上)・superEffective(×2)は倍率から決まるので、期待値の倍率から導く。
func TestAnalyzeMoveRangeTypeChart(t *testing.T) {
	t.Parallel()

	const allNeutral = "1 1 1 1 1 1 1 1 1 1 1 1 1 1 1 1 1 1"
	tests := []struct {
		name  string
		moves []balance.Move
		want  string // normal fire water electric grass ice fighting poison ground flying psychic bug rock ghost dragon dark steel fairy
	}{
		{
			// fire: fire x1/2, water x1/2, rock x1/2, grass x2, rest x1.
			name:  "single type move set",
			moves: []balance.Move{mrFireMove},
			want:  "1 1/2 1/2 1 2 1 1 1 1 1 1 1 1/2 1 1 1 1 1",
		},
		{
			// A status move never contributes, so the chart equals the fire-only one.
			name:  "status move does not widen the range",
			moves: []balance.Move{mrFireMove, mrStatusMove},
			want:  "1 1/2 1/2 1 2 1 1 1 1 1 1 1 1/2 1 1 1 1 1",
		},
		{
			// fire+water: fire max(1/2,2)=2, water max(1/2,1/2)=1/2, grass max(2,1/2)=2, rock max(1/2,2)=2.
			name:  "two types take the best of each",
			moves: []balance.Move{mrFireMove, mrWaterMove},
			want:  "1 2 1/2 1 2 1 1 1 1 1 1 1 2 1 1 1 1 1",
		},
		{
			// normal alone: rock x1/2, ghost x0, rest x1.
			name:  "a x0 matchup stays x0 when nothing else hits it",
			moves: []balance.Move{mrNormalMove},
			want:  "1 1 1 1 1 1 1 1 1 1 1 1 1/2 0 1 1 1 1",
		},
		{
			// normal+fire+water+electric: fire max(1,1/2,2,1)=2, water max(1,1/2,1/2,1)=1,
			// grass max(1,2,1/2,1)=2, rock max(1/2,1/2,2,1)=2, ghost max(0,1,1,1)=1.
			name:  "four moves",
			moves: []balance.Move{mrNormalMove, mrFireMove, mrWaterMove, mrElectricMove},
			want:  "1 2 1 1 2 1 1 1 1 1 1 1 2 1 1 1 1 1",
		},
		{
			name:  "electric alone hits everything neutrally in this chart",
			moves: []balance.Move{mrElectricMove},
			want:  allNeutral,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			analysis, err := balance.AnalyzeMoveRange(moveRangeChart(), tt.moves, mrCatalog, mrAbilities)
			if err != nil {
				t.Fatalf("AnalyzeMoveRange: %v", err)
			}
			want := strings.Fields(tt.want)
			if len(want) != 18 {
				t.Fatalf("test fixture has %d expected multipliers, want 18", len(want))
			}
			if len(analysis.TypeChart) != 18 {
				t.Fatalf("typeChart has %d entries, want 18", len(analysis.TypeChart))
			}
			for i, defense := range balance.AllTypes() {
				entry := analysis.TypeChart[i]
				if entry.DefenseType != defense {
					t.Fatalf("typeChart[%d].defenseType = %s, want %s (canonical order)", i, entry.DefenseType, defense)
				}
				if got := entry.BestMultiplier.String(); got != want[i] {
					t.Errorf("typeChart[%s].bestMultiplier = %s, want %s", defense, got, want[i])
				}
				// ADR-0404 §2 / ADR-0016 §1: effective は ×1 以上、superEffective は ×2。
				wantEffective := want[i] != "0" && want[i] != "1/2"
				wantSuper := want[i] == "2"
				if entry.Effective != wantEffective || entry.SuperEffective != wantSuper {
					t.Errorf("typeChart[%s] effective/superEffective = %v/%v, want %v/%v (bestMultiplier %s)",
						defense, entry.Effective, entry.SuperEffective, wantEffective, wantSuper, want[i])
				}
			}
		})
	}
}

// ADR-0404 §3: typeChart の18行は TB2 の1メンバー分の防御配列と同じ計算(二重実装しない)。
// 同梱の相性表で、代表的な技構成について AnalyzeCoverage と突き合わせる。
func TestAnalyzeMoveRangeTypeChartAgreesWithCoverage(t *testing.T) {
	t.Parallel()

	chart := testTypeChart()
	moveSets := [][]balance.Move{
		{mrFireMove},
		{mrFireMove, mrFireMove2},
		{mrFireMove, mrWaterMove},
		{mrNormalMove, mrStatusMove},
		{mrNormalMove, mrFireMove, mrWaterMove, mrElectricMove},
	}
	for _, moves := range moveSets {
		ids := make([]string, len(moves))
		for i, move := range moves {
			ids[i] = move.MoveID
		}
		name := strings.Join(ids, "+")
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			analysis, err := balance.AnalyzeMoveRange(chart, moves, nil, nil)
			if err != nil {
				t.Fatalf("AnalyzeMoveRange: %v", err)
			}
			coverage, err := balance.AnalyzeCoverage(chart, []balance.CoverageMember{{PokemonID: "9001-000", Moves: moves}})
			if err != nil {
				t.Fatalf("AnalyzeCoverage: %v", err)
			}
			member := coverage.Members[0]
			if typeLabels(analysis.AttackTypes) != typeLabels(member.AttackTypes) {
				t.Fatalf("attackTypes = %s, want %s (the TB2 calculation)", typeLabels(analysis.AttackTypes), typeLabels(member.AttackTypes))
			}
			if len(analysis.TypeChart) != len(member.Coverage) {
				t.Fatalf("typeChart has %d entries, want %d", len(analysis.TypeChart), len(member.Coverage))
			}
			for i, want := range member.Coverage {
				got := analysis.TypeChart[i]
				if want.BestMultiplier == nil {
					t.Fatalf("coverage[%d].bestMultiplier is nil, but TB6 rejects a move set without an attack move", i)
				}
				if got.DefenseType != want.DefenseType || got.BestMultiplier != *want.BestMultiplier ||
					got.Effective != want.Effective || got.SuperEffective != want.SuperEffective {
					t.Errorf("typeChart[%s] = %s eff=%v super=%v, want %s eff=%v super=%v",
						want.DefenseType, got.BestMultiplier, got.Effective, got.SuperEffective,
						want.BestMultiplier, want.Effective, want.SuperEffective)
				}
			}
		})
	}
}

// walledPair is a readable form of a WalledByPokemon for table comparison.
type walledPair struct{ id, name, types, multiplier string }

func walledPairsOf(pokemon []balance.WalledByPokemon) []walledPair {
	out := make([]walledPair, len(pokemon))
	for i, p := range pokemon {
		out[i] = walledPair{p.PokemonID, p.NameJa, typeLabels(p.Types), p.BestMultiplier.String()}
	}
	return out
}

// abilityPair is a readable form of a WalledByAbilityPokemon.
type abilityPair struct{ id, name, ability, multiplier string }

func abilityPairsOf(pokemon []balance.WalledByAbilityPokemon) []abilityPair {
	out := make([]abilityPair, len(pokemon))
	for i, p := range pokemon {
		out[i] = abilityPair{p.PokemonID, p.NameJa, p.AbilityID, p.BestMultiplier.String()}
	}
	return out
}

// ADR-0404 §2: walledBy は、実在ポケモンの実際のタイプ(単/複合)に対する技構成の最大倍率
// (CalculateDefense。特性は考えない)が ×1/2 以下のもの。pokemonId 昇順、nameJa は read model にあるときだけ。
// walledByAbility は、タイプだけでは ×1/2 以下にならないが特性で ×1/2 以下になるものの別枠。
// pokemonId 昇順 → abilityId 昇順。無効(×0)・吸収も含む。
func TestAnalyzeMoveRangeWalledBy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		moves       []balance.Move
		wantWalled  []walledPair
		wantAbility []abilityPair
	}{
		{
			// fire: water x1/2, water/rock x1/4, rock x1/2. grass x2, normal x1, ghost x1 は受けられない。
			// 特性の別枠: grass は ability-9001(無効)・ability-9002(吸収)の2組、normal は ability-9003(×1/2)。
			// water は タイプだけで ×1/2 なので別枠に入らない。ghost の ability-9004 は ×5/4 で届かない。
			name:  "single fire move",
			moves: []balance.Move{mrFireMove},
			wantWalled: []walledPair{
				{"9001-000", "テストミズ", "water", "1/2"},
				{"9002-000", "", "water/rock", "1/4"},
				{"9002-001", "テストイワ", "rock", "1/2"},
			},
			wantAbility: []abilityPair{
				{"9003-000", "テストクサ", "ability-9001", "0"},
				{"9003-000", "テストクサ", "ability-9002", "0"},
				{"9004-000", "", "ability-9003", "1/2"},
			},
		},
		{
			// A status move never widens the range, so the result equals the fire-only one.
			name:  "a status move does not change who walls the set",
			moves: []balance.Move{mrFireMove, mrStatusMove},
			wantWalled: []walledPair{
				{"9001-000", "テストミズ", "water", "1/2"},
				{"9002-000", "", "water/rock", "1/4"},
				{"9002-001", "テストイワ", "rock", "1/2"},
			},
			wantAbility: []abilityPair{
				{"9003-000", "テストクサ", "ability-9001", "0"},
				{"9003-000", "テストクサ", "ability-9002", "0"},
				{"9004-000", "", "ability-9003", "1/2"},
			},
		},
		{
			// fire+water: water max(1/2,1/2)=1/2 だけが残る。water/rock は water が ×1/2×2=×1 で外れ、
			// rock は water ×2、grass は fire ×2。特性の別枠では、grass が fire を無効・吸収しても
			// 残る water ×1/2 が最大になるので、倍率は ×0 ではなく ×1/2 になる。
			name:       "two types leave only the pokemon that resists both",
			moves:      []balance.Move{mrFireMove, mrWaterMove},
			wantWalled: []walledPair{{"9001-000", "テストミズ", "water", "1/2"}},
			wantAbility: []abilityPair{
				{"9003-000", "テストクサ", "ability-9001", "1/2"},
				{"9003-000", "テストクサ", "ability-9002", "1/2"},
			},
		},
		{
			// normal: rock x1/2、water/rock x1×1/2=x1/2、ghost x0。無効(×0)も基準を満たす。
			// 特性は fire にしか効かないので別枠は空。
			name:  "an immunity counts as walled",
			moves: []balance.Move{mrNormalMove},
			wantWalled: []walledPair{
				{"9002-000", "", "water/rock", "1/2"},
				{"9002-001", "テストイワ", "rock", "1/2"},
				{"9005-000", "テストゴースト", "ghost", "0"},
			},
			wantAbility: nil,
		},
		{
			// fire+normal: water は normal ×1 で外れ、water/rock max(1/4,1/2)=1/2、rock max(1/2,1/2)=1/2、
			// ghost は max(fire 1, normal 0)=1。特性の別枠は、最大倍率が全攻撃タイプにわたることの確認:
			// grass は ability-9001 で fire が ×0 になっても normal が ×1 のまま残るので入らない。
			name:  "the multiplier is the maximum over every attack type",
			moves: []balance.Move{mrFireMove, mrNormalMove},
			wantWalled: []walledPair{
				{"9002-000", "", "water/rock", "1/2"},
				{"9002-001", "テストイワ", "rock", "1/2"},
			},
			wantAbility: nil,
		},
		{
			// electric alone is x1 against everything in this chart: nobody walls it by types,
			// and no ability in the fixture touches electric.
			name:        "nobody walls a neutral move set",
			moves:       []balance.Move{mrElectricMove},
			wantWalled:  nil,
			wantAbility: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			analysis, err := balance.AnalyzeMoveRange(moveRangeChart(), tt.moves, mrCatalog, mrAbilities)
			if err != nil {
				t.Fatalf("AnalyzeMoveRange: %v", err)
			}
			if got, want := walledPairsOf(analysis.WalledBy), tt.wantWalled; fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("walledBy = %v, want %v", got, want)
			}
			if got, want := abilityPairsOf(analysis.WalledByAbility), tt.wantAbility; fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("walledByAbility = %v, want %v", got, want)
			}
		})
	}
}

// ADR-0404 §4.5 / ADR-0401 §7.2: カタログの abilityIds に特性 read model が知らない ID があっても、
// その特性だけ飛ばして 500 にしない。既知の特性(ability-9001。fire を無効化)は walledByAbility に残る。
func TestAnalyzeMoveRangeSkipsUnknownCatalogAbility(t *testing.T) {
	t.Parallel()

	catalog := []balance.CatalogPokemon{
		{PokemonID: "9010-000", NameJa: "テストミステリー", Types: []balance.TypeID{mrGrass}, AbilityIDs: []string{"ability-9999", "ability-9001"}},
	}
	analysis, err := balance.AnalyzeMoveRange(moveRangeChart(), []balance.Move{mrFireMove}, catalog, mrAbilities)
	if err != nil {
		t.Fatalf("AnalyzeMoveRange: %v", err)
	}
	got := abilityPairsOf(analysis.WalledByAbility)
	want := []abilityPair{{"9010-000", "テストミステリー", "ability-9001", "0"}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("walledByAbility = %v, want %v (unknown ability-9999 skipped, not an error)", got, want)
	}
}

// ADR-0404 §2: 特性の read model が無ければ walledByAbility は空(エラーにしない)。
// ほかの結果は特性の有無で変わらない。
func TestAnalyzeMoveRangeWithoutAbilityProvider(t *testing.T) {
	t.Parallel()

	moves := []balance.Move{mrFireMove}
	without, err := balance.AnalyzeMoveRange(moveRangeChart(), moves, mrCatalog, nil)
	if err != nil {
		t.Fatalf("AnalyzeMoveRange without abilities: %v", err)
	}
	if len(without.WalledByAbility) != 0 {
		t.Errorf("walledByAbility = %v, want empty without an ability provider", abilityPairsOf(without.WalledByAbility))
	}

	with, err := balance.AnalyzeMoveRange(moveRangeChart(), moves, mrCatalog, mrAbilities)
	if err != nil {
		t.Fatalf("AnalyzeMoveRange with abilities: %v", err)
	}
	if typeLabels(without.AttackTypes) != typeLabels(with.AttackTypes) {
		t.Errorf("attackTypes changed with the ability provider: %s vs %s", typeLabels(without.AttackTypes), typeLabels(with.AttackTypes))
	}
	if fmt.Sprint(without.TypeChart) != fmt.Sprint(with.TypeChart) {
		t.Errorf("typeChart changed with the ability provider")
	}
	if fmt.Sprint(walledPairsOf(without.WalledBy)) != fmt.Sprint(walledPairsOf(with.WalledBy)) {
		t.Errorf("walledBy = %v, want the same as with the ability provider %v",
			walledPairsOf(without.WalledBy), walledPairsOf(with.WalledBy))
	}
}

// ADR-0404 §2: 空のカタログでも計算できる(walledBy / walledByAbility が空になるだけ)。
func TestAnalyzeMoveRangeEmptyCatalog(t *testing.T) {
	t.Parallel()

	analysis, err := balance.AnalyzeMoveRange(moveRangeChart(), []balance.Move{mrFireMove}, nil, mrAbilities)
	if err != nil {
		t.Fatalf("AnalyzeMoveRange: %v", err)
	}
	if len(analysis.WalledBy) != 0 || len(analysis.WalledByAbility) != 0 {
		t.Errorf("walledBy / walledByAbility = %v / %v, want empty for an empty catalog",
			walledPairsOf(analysis.WalledBy), abilityPairsOf(analysis.WalledByAbility))
	}
	if len(analysis.TypeChart) != 18 || typeLabels(analysis.AttackTypes) != "fire" {
		t.Errorf("attackTypes/typeChart = %s / %d entries, want fire / 18", typeLabels(analysis.AttackTypes), len(analysis.TypeChart))
	}
}

// ADR-0404 §2: 件数(1〜4)・重複・全件変化技は入力エラー。ほかの内部的な失敗も伝播する。
func TestAnalyzeMoveRangeInputErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		chart balance.TypeChartProvider
		moves []balance.Move
		want  error
	}{
		{name: "no move", chart: moveRangeChart(), moves: nil, want: balance.ErrMoveRangeMoveCount},
		{name: "empty move set", chart: moveRangeChart(), moves: []balance.Move{}, want: balance.ErrMoveRangeMoveCount},
		{
			name:  "five moves",
			chart: moveRangeChart(),
			moves: []balance.Move{mrNormalMove, mrFireMove, mrFireMove2, mrWaterMove, mrElectricMove},
			want:  balance.ErrMoveRangeMoveCount,
		},
		{
			name:  "duplicate moveId",
			chart: moveRangeChart(),
			moves: []balance.Move{mrFireMove, mrFireMove},
			want:  balance.ErrDuplicateMove,
		},
		{
			name:  "only status moves",
			chart: moveRangeChart(),
			moves: []balance.Move{mrStatusMove, mrStatusMove2},
			want:  balance.ErrMoveRangeNoAttackMove,
		},
		{
			name:  "one status move",
			chart: moveRangeChart(),
			moves: []balance.Move{mrStatusMove},
			want:  balance.ErrMoveRangeNoAttackMove,
		},
		{name: "nil type chart", chart: nil, moves: []balance.Move{mrFireMove}, want: balance.ErrNilTypeChart},
		{
			name:  "invalid move category",
			chart: moveRangeChart(),
			moves: []balance.Move{{MoveID: "move-9001", Type: mrFire, Category: "other"}},
			want:  balance.ErrInvalidMoveCategory,
		},
		{
			name:  "invalid attack type",
			chart: moveRangeChart(),
			moves: []balance.Move{{MoveID: "move-9001", Type: "plasma", Category: balance.MoveCategorySpecial}},
			want:  balance.ErrInvalidType,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := balance.AnalyzeMoveRange(tt.chart, tt.moves, mrCatalog, mrAbilities)
			if !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

// 相性表・特性の provider の失敗は 呼び出し元に伝わる(HTTP では 500)。
func TestAnalyzeMoveRangePropagatesProviderFailures(t *testing.T) {
	t.Parallel()

	chartErr := errors.New("type chart backend exploded")
	if _, err := balance.AnalyzeMoveRange(mrFailingChart{err: chartErr}, []balance.Move{mrFireMove}, mrCatalog, mrAbilities); !errors.Is(err, chartErr) {
		t.Errorf("type chart failure: error = %v, want it to wrap %v", err, chartErr)
	}

	abilityErr := errors.New("ability backend exploded")
	if _, err := balance.AnalyzeMoveRange(moveRangeChart(), []balance.Move{mrFireMove}, mrCatalog, mrFailingAbilities{err: abilityErr}); !errors.Is(err, abilityErr) {
		t.Errorf("ability failure: error = %v, want it to wrap %v", err, abilityErr)
	}
}

type mrFailingChart struct{ err error }

func (c mrFailingChart) Matchup(balance.TypeID, balance.TypeID) (balance.Multiplier, error) {
	return 0, c.err
}

type mrFailingAbilities struct{ err error }

func (a mrFailingAbilities) Ability(string) (balance.Ability, error) {
	return balance.Ability{}, a.err
}
