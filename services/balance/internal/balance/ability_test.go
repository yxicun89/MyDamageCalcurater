package balance

import (
	"errors"
	"fmt"
	"testing"
)

// TB3 特性による防御相性の変化(ADR-0017)。特性 ID は架空(ability-9001 以降)。

// tb3Chart is a small stub chart. Every pair not listed is neutral.
var tb3Chart = stubTypeChart{
	{TypeGround, TypeFlying}:   MultiplierZero,
	{TypeElectric, TypeGround}: MultiplierZero,
	{TypeNormal, TypeGhost}:    MultiplierZero,
	{TypeWater, TypeFire}:      MultiplierDouble,
	{TypeFire, TypeWater}:      MultiplierHalf,
	{TypeFire, TypeGrass}:      MultiplierDouble,
	{TypeIce, TypeGrass}:       MultiplierDouble,
	{TypeIce, TypeFlying}:      MultiplierDouble,
	{TypeGrass, TypeWater}:     MultiplierDouble,
	{TypeElectric, TypeGrass}:  MultiplierHalf,
	{TypeWater, TypeGrass}:     MultiplierHalf,
	{TypeFire, TypeFire}:       MultiplierHalf,
}

func immuneTo(attack TypeID) AbilityEffect {
	return AbilityEffect{Kind: AbilityEffectImmune, AttackType: attack}
}

func absorbs(attack TypeID) AbilityEffect {
	return AbilityEffect{Kind: AbilityEffectAbsorb, AttackType: attack}
}

func typeMultiplier(attack TypeID, num, den int64) AbilityEffect {
	return AbilityEffect{Kind: AbilityEffectTypeMultiplier, AttackType: attack, Factor: Effectiveness{Num: num, Den: den}}
}

func superEffectiveMultiplier(num, den int64) AbilityEffect {
	return AbilityEffect{Kind: AbilityEffectSuperEffectiveMultiplier, Factor: Effectiveness{Num: num, Den: den}}
}

func ability(id string, effects ...AbilityEffect) *Ability {
	return &Ability{AbilityID: id, Effects: effects}
}

func TestDefenseEffectValues(t *testing.T) {
	t.Parallel()

	// The string values are the API enum values of DefenseEffect (ADR-0017 §3).
	for effect, want := range map[DefenseEffect]string{
		DefenseEffectNone: "none", DefenseEffectImmune: "immune", DefenseEffectAbsorb: "absorb", DefenseEffectMultiplier: "multiplier",
	} {
		if string(effect) != want {
			t.Errorf("effect %q, want %q", effect, want)
		}
	}
	// The kind values are the read model values (ADR-0017 §2).
	for kind, want := range map[AbilityEffectKind]string{
		AbilityEffectImmune: "immune", AbilityEffectAbsorb: "absorb",
		AbilityEffectTypeMultiplier: "type_multiplier", AbilityEffectSuperEffectiveMultiplier: "super_effective_multiplier",
	} {
		if string(kind) != want {
			t.Errorf("kind %q, want %q", kind, want)
		}
	}
}

// TestCalculateDefenseWithoutAbilityFillsEffectiveness: the TB0 function also reports the
// final value (equal to the type matchup), source=type and, for non-zero values, effect=none.
func TestCalculateDefenseWithoutAbilityFillsEffectiveness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		attack  TypeID
		defense []TypeID
		want    Effectiveness
	}{
		{TypeWater, []TypeID{TypeFire}, eff(2, 1)},
		{TypeIce, []TypeID{TypeGrass, TypeFlying}, eff(4, 1)},
		{TypeFire, []TypeID{TypeWater}, eff(1, 2)},
		{TypeElectric, []TypeID{TypeGrass}, eff(1, 2)},
		{TypeNormal, []TypeID{TypeFire}, eff(1, 1)},
	}
	for _, tt := range tests {
		got, err := CalculateDefense(tb3Chart, tt.attack, tt.defense)
		if err != nil {
			t.Fatalf("CalculateDefense(%s, %v) error = %v", tt.attack, tt.defense, err)
		}
		if got.Effectiveness != tt.want || got.Effectiveness != got.Multiplier.Effectiveness() {
			t.Errorf("%s vs %v: effectiveness = %+v (multiplier %s), want %+v", tt.attack, tt.defense, got.Effectiveness, got.Multiplier, tt.want)
		}
		if got.Source != EffectSourceType || got.Effect != DefenseEffectNone {
			t.Errorf("%s vs %v: source=%d effect=%q, want type and none", tt.attack, tt.defense, got.Source, got.Effect)
		}
	}

	immune, err := CalculateDefense(tb3Chart, TypeGround, []TypeID{TypeFlying})
	if err != nil {
		t.Fatalf("CalculateDefense(ground, flying) error = %v", err)
	}
	if immune.Effectiveness != eff(0, 1) || immune.Source != EffectSourceType {
		t.Errorf("type immunity = %+v, want effectiveness 0 and source type", immune)
	}
}

// TestCalculateDefenseWithAbility follows the ADR-0017 §3 order: type matchup → a type
// immunity is final (source=type) → ability effects. immune/absorb give x0 with
// source=ability; type_multiplier applies to its attack type; super_effective_multiplier
// applies only when the type matchup is above x1; an unchanged value keeps source=type
// and effect=none.
func TestCalculateDefenseWithAbility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		attack   TypeID
		defense  []TypeID
		ability  *Ability
		want     Effectiveness
		source   EffectSource
		effect   DefenseEffect
		category Category
	}{
		{name: "nil ability", attack: TypeWater, defense: []TypeID{TypeFire}, ability: nil,
			want: eff(2, 1), source: EffectSourceType, effect: DefenseEffectNone, category: CategoryWeak},
		{name: "ability without effects", attack: TypeWater, defense: []TypeID{TypeFire}, ability: ability("ability-9005"),
			want: eff(2, 1), source: EffectSourceType, effect: DefenseEffectNone, category: CategoryWeak},

		{name: "immune on neutral", attack: TypeGround, defense: []TypeID{TypeFire}, ability: ability("ability-9001", immuneTo(TypeGround)),
			want: eff(0, 1), source: EffectSourceAbility, effect: DefenseEffectImmune, category: CategoryImmune},
		{name: "immune on weakness", attack: TypeWater, defense: []TypeID{TypeFire}, ability: ability("ability-9001", immuneTo(TypeWater)),
			want: eff(0, 1), source: EffectSourceAbility, effect: DefenseEffectImmune, category: CategoryImmune},
		{name: "absorb on weakness", attack: TypeWater, defense: []TypeID{TypeFire}, ability: ability("ability-9002", absorbs(TypeWater)),
			want: eff(0, 1), source: EffectSourceAbility, effect: DefenseEffectAbsorb, category: CategoryImmune},
		{name: "absorb on resistance", attack: TypeElectric, defense: []TypeID{TypeGrass}, ability: ability("ability-9002", absorbs(TypeElectric)),
			want: eff(0, 1), source: EffectSourceAbility, effect: DefenseEffectAbsorb, category: CategoryImmune},
		{name: "immune for another attack type", attack: TypeWater, defense: []TypeID{TypeFire}, ability: ability("ability-9001", immuneTo(TypeGround)),
			want: eff(2, 1), source: EffectSourceType, effect: DefenseEffectNone, category: CategoryWeak},

		{name: "type immunity wins over ability immune", attack: TypeGround, defense: []TypeID{TypeFlying}, ability: ability("ability-9001", immuneTo(TypeGround)),
			want: eff(0, 1), source: EffectSourceType, effect: DefenseEffectNone, category: CategoryImmune},
		{name: "type immunity wins over absorb", attack: TypeElectric, defense: []TypeID{TypeGround}, ability: ability("ability-9002", absorbs(TypeElectric)),
			want: eff(0, 1), source: EffectSourceType, effect: DefenseEffectNone, category: CategoryImmune},
		{name: "type immunity ignores type_multiplier", attack: TypeGround, defense: []TypeID{TypeFire, TypeFlying}, ability: ability("ability-9003", typeMultiplier(TypeGround, 2, 1)),
			want: eff(0, 1), source: EffectSourceType, effect: DefenseEffectNone, category: CategoryImmune},

		{name: "type_multiplier halves a weakness", attack: TypeFire, defense: []TypeID{TypeGrass}, ability: ability("ability-9003", typeMultiplier(TypeFire, 1, 2)),
			want: eff(1, 1), source: EffectSourceAbility, effect: DefenseEffectMultiplier, category: CategoryNeutral},
		{name: "type_multiplier halves a resistance", attack: TypeFire, defense: []TypeID{TypeWater}, ability: ability("ability-9003", typeMultiplier(TypeFire, 1, 2)),
			want: eff(1, 4), source: EffectSourceAbility, effect: DefenseEffectMultiplier, category: CategoryQuadResist},
		{name: "type_multiplier 5/4 on weakness", attack: TypeFire, defense: []TypeID{TypeGrass}, ability: ability("ability-9006", typeMultiplier(TypeFire, 5, 4)),
			want: eff(5, 2), source: EffectSourceAbility, effect: DefenseEffectMultiplier, category: CategoryWeak},
		{name: "type_multiplier 5/4 on neutral", attack: TypeFire, defense: []TypeID{TypeNormal}, ability: ability("ability-9006", typeMultiplier(TypeFire, 5, 4)),
			want: eff(5, 4), source: EffectSourceAbility, effect: DefenseEffectMultiplier, category: CategoryWeak},
		{name: "type_multiplier 5/4 on resistance", attack: TypeFire, defense: []TypeID{TypeWater}, ability: ability("ability-9006", typeMultiplier(TypeFire, 5, 4)),
			want: eff(5, 8), source: EffectSourceAbility, effect: DefenseEffectMultiplier, category: CategoryResist},
		{name: "type_multiplier x2 on neutral", attack: TypeFire, defense: []TypeID{TypeNormal}, ability: ability("ability-9007", typeMultiplier(TypeFire, 2, 1)),
			want: eff(2, 1), source: EffectSourceAbility, effect: DefenseEffectMultiplier, category: CategoryWeak},
		{name: "type_multiplier x2 on weakness reaches quad_weak", attack: TypeFire, defense: []TypeID{TypeGrass}, ability: ability("ability-9007", typeMultiplier(TypeFire, 2, 1)),
			want: eff(4, 1), source: EffectSourceAbility, effect: DefenseEffectMultiplier, category: CategoryQuadWeak},
		{name: "type_multiplier for another attack type", attack: TypeWater, defense: []TypeID{TypeFire}, ability: ability("ability-9003", typeMultiplier(TypeFire, 1, 2)),
			want: eff(2, 1), source: EffectSourceType, effect: DefenseEffectNone, category: CategoryWeak},
		{name: "type_multiplier x1 changes nothing", attack: TypeFire, defense: []TypeID{TypeGrass}, ability: ability("ability-9008", typeMultiplier(TypeFire, 2, 2)),
			want: eff(2, 1), source: EffectSourceType, effect: DefenseEffectNone, category: CategoryWeak},

		{name: "super effective x2 gets 3/4", attack: TypeFire, defense: []TypeID{TypeGrass}, ability: ability("ability-9004", superEffectiveMultiplier(3, 4)),
			want: eff(3, 2), source: EffectSourceAbility, effect: DefenseEffectMultiplier, category: CategoryWeak},
		{name: "super effective x4 gets 3/4", attack: TypeIce, defense: []TypeID{TypeGrass, TypeFlying}, ability: ability("ability-9004", superEffectiveMultiplier(3, 4)),
			want: eff(3, 1), source: EffectSourceAbility, effect: DefenseEffectMultiplier, category: CategoryWeak},
		{name: "super effective multiplier skips neutral", attack: TypeNormal, defense: []TypeID{TypeFire}, ability: ability("ability-9004", superEffectiveMultiplier(3, 4)),
			want: eff(1, 1), source: EffectSourceType, effect: DefenseEffectNone, category: CategoryNeutral},
		{name: "super effective multiplier skips resistance", attack: TypeFire, defense: []TypeID{TypeWater}, ability: ability("ability-9004", superEffectiveMultiplier(3, 4)),
			want: eff(1, 2), source: EffectSourceType, effect: DefenseEffectNone, category: CategoryResist},
		{name: "super effective multiplier skips type immunity", attack: TypeGround, defense: []TypeID{TypeFlying}, ability: ability("ability-9004", superEffectiveMultiplier(3, 4)),
			want: eff(0, 1), source: EffectSourceType, effect: DefenseEffectNone, category: CategoryImmune},
		// fire vs grass/water is x2 × x1/2 = x1 (not above x1), so 3/4 does not apply.
		{name: "super effective multiplier skips a dual type neutral", attack: TypeFire, defense: []TypeID{TypeGrass, TypeWater}, ability: ability("ability-9004", superEffectiveMultiplier(3, 4)),
			want: eff(1, 1), source: EffectSourceType, effect: DefenseEffectNone, category: CategoryNeutral},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := CalculateDefenseWithAbility(tb3Chart, tt.attack, tt.defense, tt.ability)
			if err != nil {
				t.Fatalf("CalculateDefenseWithAbility() error = %v", err)
			}
			typeOnly, err := CalculateDefense(tb3Chart, tt.attack, tt.defense)
			if err != nil {
				t.Fatalf("CalculateDefense() error = %v", err)
			}
			if got.Multiplier != typeOnly.Multiplier {
				t.Errorf("Multiplier = %s, want the type matchup %s (Multiplier stays type-only)", got.Multiplier, typeOnly.Multiplier)
			}
			if got.Effectiveness != tt.want {
				t.Errorf("effectiveness = %s (%+v), want %s", got.Effectiveness, got.Effectiveness, tt.want)
			}
			if got.Source != tt.source {
				t.Errorf("source = %d, want %d", got.Source, tt.source)
			}
			// ADR-0017 §5.1・§5.5: a type immunity is effect=none (source=type).
			if got.Effect != tt.effect {
				t.Errorf("effect = %q, want %q", got.Effect, tt.effect)
			}
			category, err := ClassifyEffectiveness(got.Effectiveness)
			if err != nil || category != tt.category {
				t.Errorf("category = %q, %v, want %q", category, err, tt.category)
			}
		})
	}
}

// TestCalculateDefenseWithAbilityCombinedEffects: several effects of one ability are all applied
// (ADR-0017 §3 "特性の効果を順に掛ける"). The super effective condition uses the type matchup.
func TestCalculateDefenseWithAbilityCombinedEffects(t *testing.T) {
	t.Parallel()

	mixed := ability("ability-9010", immuneTo(TypeElectric), typeMultiplier(TypeIce, 2, 1))
	halfAndSuper := ability("ability-9011", typeMultiplier(TypeFire, 1, 2), superEffectiveMultiplier(3, 4))
	superAndHalf := ability("ability-9012", superEffectiveMultiplier(3, 4), typeMultiplier(TypeFire, 1, 2))
	cancel := ability("ability-9013", typeMultiplier(TypeFire, 2, 1), typeMultiplier(TypeFire, 1, 2))
	immuneAndMultiplier := ability("ability-9014", typeMultiplier(TypeFire, 2, 1), immuneTo(TypeFire))
	absorbAndSuper := ability("ability-9015", absorbs(TypeWater), superEffectiveMultiplier(3, 4))
	twoMultipliers := ability("ability-9016", typeMultiplier(TypeFire, 1, 2), typeMultiplier(TypeFire, 3, 4))

	tests := []struct {
		name    string
		ability *Ability
		attack  TypeID
		defense []TypeID
		want    Effectiveness
		source  EffectSource
		effect  DefenseEffect
	}{
		{"immune part", mixed, TypeElectric, []TypeID{TypeGrass}, eff(0, 1), EffectSourceAbility, DefenseEffectImmune},
		{"multiplier part", mixed, TypeIce, []TypeID{TypeGrass}, eff(4, 1), EffectSourceAbility, DefenseEffectMultiplier},
		{"untouched type", mixed, TypeFire, []TypeID{TypeGrass}, eff(2, 1), EffectSourceType, DefenseEffectNone},
		{"type multiplier then super effective", halfAndSuper, TypeFire, []TypeID{TypeGrass}, eff(3, 4), EffectSourceAbility, DefenseEffectMultiplier},
		{"super effective then type multiplier", superAndHalf, TypeFire, []TypeID{TypeGrass}, eff(3, 4), EffectSourceAbility, DefenseEffectMultiplier},
		{"only super effective applies", halfAndSuper, TypeIce, []TypeID{TypeGrass}, eff(3, 2), EffectSourceAbility, DefenseEffectMultiplier},
		{"only type multiplier applies", halfAndSuper, TypeFire, []TypeID{TypeNormal}, eff(1, 2), EffectSourceAbility, DefenseEffectMultiplier},
		{"multipliers cancel out", cancel, TypeFire, []TypeID{TypeGrass}, eff(2, 1), EffectSourceType, DefenseEffectNone},
		{"immune after a multiplier", immuneAndMultiplier, TypeFire, []TypeID{TypeGrass}, eff(0, 1), EffectSourceAbility, DefenseEffectImmune},
		{"absorb with super effective", absorbAndSuper, TypeWater, []TypeID{TypeFire}, eff(0, 1), EffectSourceAbility, DefenseEffectAbsorb},
		{"super effective part of absorb ability", absorbAndSuper, TypeFire, []TypeID{TypeGrass}, eff(3, 2), EffectSourceAbility, DefenseEffectMultiplier},
		{"two multipliers for one type", twoMultipliers, TypeFire, []TypeID{TypeGrass}, eff(3, 4), EffectSourceAbility, DefenseEffectMultiplier},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := CalculateDefenseWithAbility(tb3Chart, tt.attack, tt.defense, tt.ability)
			if err != nil {
				t.Fatalf("CalculateDefenseWithAbility() error = %v", err)
			}
			if got.Effectiveness != tt.want || got.Source != tt.source || got.Effect != tt.effect {
				t.Errorf("got effectiveness %s source %d effect %q, want %s %d %q", got.Effectiveness, got.Source, got.Effect, tt.want, tt.source, tt.effect)
			}
		})
	}
}

func TestCalculateDefenseWithAbilityMatchesCalculateDefenseWithoutAbility(t *testing.T) {
	t.Parallel()

	for _, attack := range AllTypes() {
		for _, defense := range [][]TypeID{{TypeFire}, {TypeGrass, TypeFlying}, {TypeWater}, {TypeGround}, {TypeNormal, TypeGhost}} {
			want, err := CalculateDefense(tb3Chart, attack, defense)
			if err != nil {
				t.Fatalf("CalculateDefense() error = %v", err)
			}
			for _, a := range []*Ability{nil, ability("ability-9005")} {
				got, err := CalculateDefenseWithAbility(tb3Chart, attack, defense, a)
				if err != nil {
					t.Fatalf("CalculateDefenseWithAbility() error = %v", err)
				}
				if got != want {
					t.Errorf("%s vs %v (ability %v): got %+v, want the CalculateDefense result %+v", attack, defense, a, got, want)
				}
			}
		}
	}
}

func TestCalculateDefenseWithAbilityRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		chart   TypeChartProvider
		attack  TypeID
		defense []TypeID
		ability *Ability
		wantErr error
	}{
		{name: "nil chart", chart: nil, attack: TypeFire, defense: []TypeID{TypeGrass}, ability: ability("ability-9001", immuneTo(TypeGround)), wantErr: ErrNilTypeChart},
		{name: "invalid attack", chart: tb3Chart, attack: "stellar", defense: []TypeID{TypeGrass}, ability: ability("ability-9001", immuneTo(TypeGround)), wantErr: ErrInvalidType},
		{name: "no defense types", chart: tb3Chart, attack: TypeFire, ability: ability("ability-9001", immuneTo(TypeGround)), wantErr: ErrDefenseTypeCount},
		{name: "unknown effect kind", chart: tb3Chart, attack: TypeFire, defense: []TypeID{TypeGrass}, ability: ability("ability-9001", AbilityEffect{Kind: "heal", AttackType: TypeFire}), wantErr: ErrInvalidAbilityEffect},
		{name: "invalid effect attack type for another attack", chart: tb3Chart, attack: TypeFire, defense: []TypeID{TypeGrass}, ability: ability("ability-9001", immuneTo("stellar")), wantErr: ErrInvalidAbilityEffect},
		{name: "type_multiplier with zero denominator", chart: tb3Chart, attack: TypeFire, defense: []TypeID{TypeGrass}, ability: ability("ability-9003", typeMultiplier(TypeFire, 1, 0)), wantErr: ErrInvalidAbilityEffect},
		{name: "super_effective_multiplier with zero denominator", chart: tb3Chart, attack: TypeNormal, defense: []TypeID{TypeGrass}, ability: ability("ability-9004", superEffectiveMultiplier(3, 0)), wantErr: ErrInvalidAbilityEffect},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := CalculateDefenseWithAbility(tt.chart, tt.attack, tt.defense, tt.ability)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want errors.Is(_, %v)", err, tt.wantErr)
			}
		})
	}
}

// TestAnalyzeDefenseWithAbilities: members keep their abilityId, categories follow the final
// value, and the team summary counts ability immunities and absorptions as immune (ADR-0017 §1・§3).
func TestAnalyzeDefenseWithAbilities(t *testing.T) {
	t.Parallel()

	members := []Member{
		{PokemonID: "9101-000", Types: []TypeID{TypeFire}, Ability: ability("ability-9002", absorbs(TypeWater))},
		{PokemonID: "9102-000", Types: []TypeID{TypeFire}, Ability: ability("ability-9001", immuneTo(TypeGround))},
		{PokemonID: "9103-000", Types: []TypeID{TypeGrass, TypeFlying}, Ability: ability("ability-9004", superEffectiveMultiplier(3, 4))},
		{PokemonID: "9104-000", Types: []TypeID{TypeGrass}, Ability: ability("ability-9007", typeMultiplier(TypeFire, 2, 1))},
		{PokemonID: "9105-000", Types: []TypeID{TypeGrass}},
		{PokemonID: "9106-000", Types: []TypeID{TypeFlying}, Ability: ability("ability-9001", immuneTo(TypeGround))},
	}
	got, err := AnalyzeDefense(tb3Chart, members)
	if err != nil {
		t.Fatalf("AnalyzeDefense() error = %v", err)
	}
	if len(got.Members) != len(members) || len(got.TeamSummary) != 18 {
		t.Fatalf("members=%d summary=%d, want %d and 18", len(got.Members), len(got.TeamSummary), len(members))
	}
	wantAbilityIDs := []string{"ability-9002", "ability-9001", "ability-9004", "ability-9007", "", "ability-9001"}
	for i, want := range wantAbilityIDs {
		if got.Members[i].AbilityID != want {
			t.Errorf("members[%d].AbilityID = %q, want %q", i, got.Members[i].AbilityID, want)
		}
		if got.Members[i].PokemonID != members[i].PokemonID {
			t.Errorf("members[%d].PokemonID = %q, want %q", i, got.Members[i].PokemonID, members[i].PokemonID)
		}
	}

	entries := []struct {
		member   int
		attack   TypeID
		want     Effectiveness
		category Category
		source   EffectSource
		effect   DefenseEffect
	}{
		{0, TypeWater, eff(0, 1), CategoryImmune, EffectSourceAbility, DefenseEffectAbsorb},
		{0, TypeFire, eff(1, 2), CategoryResist, EffectSourceType, DefenseEffectNone},
		{1, TypeGround, eff(0, 1), CategoryImmune, EffectSourceAbility, DefenseEffectImmune},
		{1, TypeWater, eff(2, 1), CategoryWeak, EffectSourceType, DefenseEffectNone},
		{2, TypeIce, eff(3, 1), CategoryWeak, EffectSourceAbility, DefenseEffectMultiplier},
		{2, TypeFire, eff(3, 2), CategoryWeak, EffectSourceAbility, DefenseEffectMultiplier},
		{3, TypeFire, eff(4, 1), CategoryQuadWeak, EffectSourceAbility, DefenseEffectMultiplier},
		{3, TypeWater, eff(1, 2), CategoryResist, EffectSourceType, DefenseEffectNone},
		{4, TypeFire, eff(2, 1), CategoryWeak, EffectSourceType, DefenseEffectNone},
	}
	for _, tt := range entries {
		entry := findAttack(t, got.Members[tt.member].Defense, tt.attack)
		if entry.Result.Effectiveness != tt.want || entry.Category != tt.category || entry.Result.Source != tt.source || entry.Result.Effect != tt.effect {
			t.Errorf("members[%d] vs %s = %s %q source %d effect %q, want %s %q %d %q", tt.member, tt.attack,
				entry.Result.Effectiveness, entry.Category, entry.Result.Source, entry.Result.Effect, tt.want, tt.category, tt.source, tt.effect)
		}
	}
	// 9106 flying is immune to ground by type: category immune, source type.
	if entry := findAttack(t, got.Members[5].Defense, TypeGround); entry.Category != CategoryImmune || entry.Result.Source != EffectSourceType {
		t.Errorf("type immunity with an immune ability = %+v, want immune from type", entry)
	}

	summaries := []TeamSummaryEntry{
		// water: absorb (immune), 9102 fire x2 (weak), 9103 grass/flying x1/2 (resist), 9104 grass x1/2, 9105 grass x1/2, 9106 flying x1.
		{AttackType: TypeWater, Weak: 1, Resist: 3, Immune: 1, Neutral: 1},
		// ground: 9101 x1, 9102 ability immune, 9103 type immune (flying), 9104 x1, 9105 x1, 9106 type immune.
		{AttackType: TypeGround, Immune: 3, Neutral: 3},
		// fire: 9101 x1/2, 9102 x1/2, 9103 x3/2 (weak, not quad), 9104 x4 (quad_weak), 9105 x2, 9106 x1.
		{AttackType: TypeFire, Weak: 3, QuadWeak: 1, Resist: 2, Neutral: 1},
		// ice: 9101 x1, 9102 x1, 9103 x3 (weak, not quad), 9104 x2, 9105 x2, 9106 x2.
		{AttackType: TypeIce, Weak: 4, Neutral: 2},
	}
	for _, want := range summaries {
		if entry := findSummary(t, got.TeamSummary, want.AttackType); entry != want {
			t.Errorf("TeamSummary[%s] = %+v, want %+v", want.AttackType, entry, want)
		}
	}
	assertSummaryMatchesCategories(t, got, len(members))
}

// assertSummaryMatchesCategories recounts the ADR-0014 §3 summary from member categories.
func assertSummaryMatchesCategories(t *testing.T, got DefenseAnalysis, memberCount int) {
	t.Helper()
	for i, entry := range got.TeamSummary {
		if entry.Weak+entry.Resist+entry.Immune+entry.Neutral != memberCount {
			t.Errorf("%s: weak+resist+immune+neutral = %d, want %d", entry.AttackType, entry.Weak+entry.Resist+entry.Immune+entry.Neutral, memberCount)
		}
		if entry.QuadWeak > entry.Weak {
			t.Errorf("%s: quadWeak %d > weak %d", entry.AttackType, entry.QuadWeak, entry.Weak)
		}
		want := TeamSummaryEntry{AttackType: entry.AttackType}
		for _, member := range got.Members {
			d := member.Defense[i]
			if d.AttackType != entry.AttackType {
				t.Fatalf("defense[%d] = %s, want %s", i, d.AttackType, entry.AttackType)
			}
			category, err := ClassifyEffectiveness(d.Result.Effectiveness)
			if err != nil || category != d.Category {
				t.Errorf("%s %s: category %q, ClassifyEffectiveness = %q, %v", member.PokemonID, d.AttackType, d.Category, category, err)
			}
			switch d.Category {
			case CategoryQuadWeak:
				want.Weak++
				want.QuadWeak++
			case CategoryWeak:
				want.Weak++
			case CategoryResist, CategoryQuadResist:
				want.Resist++
			case CategoryImmune:
				want.Immune++
			case CategoryNeutral:
				want.Neutral++
			default:
				t.Errorf("%s %s: unknown category %q", member.PokemonID, d.AttackType, d.Category)
			}
		}
		if entry != want {
			t.Errorf("%s: summary %+v disagrees with member categories %+v", entry.AttackType, entry, want)
		}
	}
}

func TestAnalyzeDefenseRejectsInvalidAbilityEffect(t *testing.T) {
	t.Parallel()

	members := []Member{memberA, {PokemonID: "9101-000", Types: []TypeID{TypeFire}, Ability: ability("ability-9001", AbilityEffect{Kind: "heal"})}}
	if _, err := AnalyzeDefense(tb3Chart, members); !errors.Is(err, ErrInvalidAbilityEffect) {
		t.Fatalf("AnalyzeDefense() error = %v, want ErrInvalidAbilityEffect", err)
	}
}

type stubAbilities map[string]Ability

func (s stubAbilities) Ability(id string) (Ability, error) {
	if a, ok := s[id]; ok {
		return a, nil
	}
	return Ability{}, fmt.Errorf("%w: %s (read from /secret/abilities.json)", ErrUnknownAbility, id)
}

type failingAbilities struct{ err error }

func (f failingAbilities) Ability(string) (Ability, error) { return Ability{}, f.err }

func TestResolveAbility(t *testing.T) {
	t.Parallel()

	provider := stubAbilities{
		"ability-9001": {AbilityID: "ability-9001", Effects: []AbilityEffect{immuneTo(TypeGround)}},
		// A provider that normalizes IDs must not change the requested ID (same rule as ResolveMoves).
		"ability-9002": {AbilityID: "ABILITY-9002", Effects: []AbilityEffect{absorbs(TypeWater)}},
		"ability-9005": {AbilityID: "ability-9005"},
	}
	for _, tt := range []struct {
		id      string
		effects []AbilityEffect
	}{
		{"ability-9001", []AbilityEffect{immuneTo(TypeGround)}},
		{"ability-9002", []AbilityEffect{absorbs(TypeWater)}},
		{"ability-9005", nil},
	} {
		got, err := ResolveAbility(provider, tt.id)
		if err != nil {
			t.Fatalf("ResolveAbility(%s) error = %v", tt.id, err)
		}
		if got.AbilityID != tt.id {
			t.Errorf("ResolveAbility(%s).AbilityID = %q, want the requested ID", tt.id, got.AbilityID)
		}
		if fmt.Sprint(got.Effects) != fmt.Sprint(tt.effects) {
			t.Errorf("ResolveAbility(%s).Effects = %+v, want %+v", tt.id, got.Effects, tt.effects)
		}
	}
}

func TestResolveAbilityErrors(t *testing.T) {
	t.Parallel()

	if _, err := ResolveAbility(nil, "ability-9001"); !errors.Is(err, ErrNilAbilities) {
		t.Errorf("ResolveAbility(nil) error = %v, want ErrNilAbilities", err)
	}

	_, err := ResolveAbility(stubAbilities{}, "ability-9999")
	if !errors.Is(err, ErrUnknownAbility) {
		t.Fatalf("unknown: error = %v, want errors.Is(_, ErrUnknownAbility)", err)
	}
	var unknown *UnknownAbilityError
	if !errors.As(err, &unknown) {
		t.Fatalf("unknown: error = %T %v, want *UnknownAbilityError", err, err)
	}
	if unknown.AbilityID != "ability-9999" {
		t.Errorf("UnknownAbilityError.AbilityID = %q, want ability-9999", unknown.AbilityID)
	}
	// The adapter detail (e.g. a file path) is dropped: only the ID remains.
	if err.Error() != "unknown ability: ability-9999" {
		t.Errorf("Error() = %q, want %q", err.Error(), "unknown ability: ability-9999")
	}

	providerErr := errors.New("ability backend exploded")
	_, err = ResolveAbility(failingAbilities{err: providerErr}, "ability-9001")
	if !errors.Is(err, providerErr) {
		t.Errorf("provider failure: error = %v, want errors.Is(_, providerErr)", err)
	}
	if errors.Is(err, ErrUnknownAbility) || errors.As(err, &unknown) {
		t.Errorf("provider failure must not become unknown_ability: %v", err)
	}
}

func TestUnknownAbilityErrorUnwrapsOnlyErrUnknownAbility(t *testing.T) {
	t.Parallel()

	err := &UnknownAbilityError{AbilityID: "ability-9999"}
	if err.Unwrap() != ErrUnknownAbility {
		t.Errorf("Unwrap() = %v, want ErrUnknownAbility", err.Unwrap())
	}
	if !errors.Is(err, ErrUnknownAbility) {
		t.Error("errors.Is(_, ErrUnknownAbility) = false")
	}
	for _, other := range []error{ErrUnknownPokemon, ErrUnknownMove, ErrNilAbilities, ErrInvalidAbilityEffect} {
		if errors.Is(err, other) {
			t.Errorf("errors.Is(_, %v) = true, want false", other)
		}
	}
	if err.Error() != "unknown ability: ability-9999" {
		t.Errorf("Error() = %q", err.Error())
	}
}

func TestCalculateDefenseWithAbilityOverflowIsAnError(t *testing.T) {
	t.Parallel()

	// ADR-0017 §5.3 allows repeated effects; stacking 16 × (16/1) must not wrap around silently.
	effects := make([]AbilityEffect, 16)
	for i := range effects {
		effects[i] = AbilityEffect{Kind: AbilityEffectSuperEffectiveMultiplier, Factor: eff(16, 1)}
	}
	stacked := &Ability{AbilityID: "ability-9999", Effects: effects}
	_, err := CalculateDefenseWithAbility(tb3Chart, TypeGrass, []TypeID{TypeWater}, stacked)
	if !errors.Is(err, ErrEffectivenessOverflow) {
		t.Fatalf("err = %v, want ErrEffectivenessOverflow", err)
	}
}

func TestValidateAbilityEffectRejectsZeroFactor(t *testing.T) {
	t.Parallel()

	for _, effect := range []AbilityEffect{
		{Kind: AbilityEffectTypeMultiplier, AttackType: TypeFire, Factor: Effectiveness{Num: 0, Den: 1}},
		{Kind: AbilityEffectSuperEffectiveMultiplier, Factor: Effectiveness{Num: 0, Den: 1}},
	} {
		if err := validateAbilityEffect(effect); !errors.Is(err, ErrInvalidAbilityEffect) {
			t.Errorf("%s with factor 0: err = %v, want ErrInvalidAbilityEffect (factors are 1..16 ratios)", effect.Kind, err)
		}
	}
}

func TestUnreducedFactorOfOneChangesNothingOnNeutral(t *testing.T) {
	t.Parallel()

	// A provider may hand over 2/2; on a neutral matchup it must stay x1, source=type, effect=none (§5.3).
	two := &Ability{AbilityID: "ability-9998", Effects: []AbilityEffect{{Kind: AbilityEffectTypeMultiplier, AttackType: TypeFire, Factor: Effectiveness{Num: 2, Den: 2}}}}
	got, err := CalculateDefenseWithAbility(tb3Chart, TypeFire, []TypeID{TypeNormal}, two)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if got.Effectiveness != eff(1, 1) || got.Source != EffectSourceType || got.Effect != DefenseEffectNone {
		t.Errorf("result = %+v, want 1 with source=type effect=none", got)
	}
}
