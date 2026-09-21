package balance

import (
	"errors"
	"fmt"
	"testing"
)

// tb1Chart is a small stub chart. Every pair not listed is neutral.
// Fictional members built on it:
//
//	9001-000 grass/flying -> ice x4 (quad_weak)
//	9002-000 water/rock   -> fire x1/4 (quad_resist)
//	9003-000 ghost        -> normal x0 (immune)
//	9004-000 fire         -> water x2 (weak), grass x1/2 (resist)
var tb1Chart = stubTypeChart{
	{TypeIce, TypeGrass}:    MultiplierDouble,
	{TypeIce, TypeFlying}:   MultiplierDouble,
	{TypeFire, TypeWater}:   MultiplierHalf,
	{TypeFire, TypeRock}:    MultiplierHalf,
	{TypeNormal, TypeGhost}: MultiplierZero,
	{TypeWater, TypeFire}:   MultiplierDouble,
	{TypeGrass, TypeFire}:   MultiplierHalf,
}

var (
	memberA = Member{PokemonID: "9001-000", Types: []TypeID{TypeGrass, TypeFlying}}
	memberB = Member{PokemonID: "9002-000", Types: []TypeID{TypeWater, TypeRock}}
	memberC = Member{PokemonID: "9003-000", Types: []TypeID{TypeGhost}}
	memberD = Member{PokemonID: "9004-000", Types: []TypeID{TypeFire}}
)

func TestClassifyMultiplier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		multiplier Multiplier
		want       Category
		wantString string
	}{
		{MultiplierQuad, CategoryQuadWeak, "quad_weak"},
		{MultiplierDouble, CategoryWeak, "weak"},
		{MultiplierNormal, CategoryNeutral, "neutral"},
		{MultiplierHalf, CategoryResist, "resist"},
		{MultiplierQuarter, CategoryQuadResist, "quad_resist"},
		{MultiplierZero, CategoryImmune, "immune"},
	}
	for _, tt := range tests {
		t.Run(tt.multiplier.String(), func(t *testing.T) {
			t.Parallel()
			got, err := ClassifyMultiplier(tt.multiplier)
			if err != nil {
				t.Fatalf("ClassifyMultiplier(%d) error = %v", tt.multiplier, err)
			}
			if got != tt.want {
				t.Errorf("ClassifyMultiplier(%d) = %q, want %q", tt.multiplier, got, tt.want)
			}
			// The string value is the API enum value (DefenseCategory).
			if string(got) != tt.wantString {
				t.Errorf("category string = %q, want %q", got, tt.wantString)
			}
		})
	}
}

func TestClassifyMultiplierRejectsUnrepresentableValues(t *testing.T) {
	t.Parallel()

	for _, m := range []Multiplier{3, 5, 6, 7, 12, 32, 64} {
		if _, err := ClassifyMultiplier(m); !errors.Is(err, ErrInvalidMultiplier) {
			t.Errorf("ClassifyMultiplier(%d) error = %v, want ErrInvalidMultiplier", m, err)
		}
	}
}

// expectedDefense returns the multiplier for member against attack on tb1Chart.
func expectedDefense(member Member, attack TypeID) Multiplier {
	combined := uint16(MultiplierNormal)
	for _, defense := range member.Types {
		matchup := MultiplierNormal
		if m, ok := tb1Chart[[2]TypeID{attack, defense}]; ok {
			matchup = m
		}
		combined = combined * uint16(matchup) / uint16(MultiplierNormal)
	}
	return Multiplier(combined)
}

func TestAnalyzeDefenseMemberResults(t *testing.T) {
	t.Parallel()

	members := []Member{memberA, memberB, memberC, memberD}
	got, err := AnalyzeDefense(tb1Chart, members)
	if err != nil {
		t.Fatalf("AnalyzeDefense() error = %v", err)
	}
	if len(got.Members) != len(members) {
		t.Fatalf("members length = %d, want %d", len(got.Members), len(members))
	}

	for i, member := range members {
		result := got.Members[i]
		if result.PokemonID != member.PokemonID {
			t.Errorf("members[%d].PokemonID = %q, want %q (request order must be kept)", i, result.PokemonID, member.PokemonID)
		}
		if fmt.Sprint(result.Types) != fmt.Sprint(member.Types) {
			t.Errorf("members[%d].Types = %v, want %v", i, result.Types, member.Types)
		}
		if len(result.Defense) != 18 {
			t.Fatalf("members[%d].Defense length = %d, want 18", i, len(result.Defense))
		}
		for j, attack := range AllTypes() {
			entry := result.Defense[j]
			if entry.AttackType != attack {
				t.Errorf("members[%d].Defense[%d].AttackType = %q, want %q (canonical order)", i, j, entry.AttackType, attack)
			}
			want := expectedDefense(member, attack)
			if entry.Result.Multiplier != want {
				t.Errorf("members[%d] vs %s multiplier = %s, want %s", i, attack, entry.Result.Multiplier, want)
			}
			if entry.Result.Source != EffectSourceType {
				t.Errorf("members[%d] vs %s source = %d, want EffectSourceType", i, attack, entry.Result.Source)
			}
			wantCategory, err := ClassifyMultiplier(want)
			if err != nil {
				t.Fatalf("ClassifyMultiplier(%d) error = %v", want, err)
			}
			if entry.Category != wantCategory {
				t.Errorf("members[%d] vs %s category = %q, want %q", i, attack, entry.Category, wantCategory)
			}
		}
	}
}

func TestAnalyzeDefenseRepresentativeCategories(t *testing.T) {
	t.Parallel()

	got, err := AnalyzeDefense(tb1Chart, []Member{memberA, memberB, memberC, memberD})
	if err != nil {
		t.Fatalf("AnalyzeDefense() error = %v", err)
	}
	if len(got.Members) != 4 {
		t.Fatalf("members length = %d, want 4", len(got.Members))
	}

	tests := []struct {
		name       string
		member     int
		attack     TypeID
		multiplier Multiplier
		category   Category
	}{
		{name: "dual type quad weak", member: 0, attack: TypeIce, multiplier: MultiplierQuad, category: CategoryQuadWeak},
		{name: "dual type quarter resist", member: 1, attack: TypeFire, multiplier: MultiplierQuarter, category: CategoryQuadResist},
		{name: "single type immune", member: 2, attack: TypeNormal, multiplier: MultiplierZero, category: CategoryImmune},
		{name: "single type weak", member: 3, attack: TypeWater, multiplier: MultiplierDouble, category: CategoryWeak},
		{name: "single type resist", member: 3, attack: TypeGrass, multiplier: MultiplierHalf, category: CategoryResist},
		{name: "neutral", member: 2, attack: TypeFire, multiplier: MultiplierNormal, category: CategoryNeutral},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entry := findAttack(t, got.Members[tt.member].Defense, tt.attack)
			if entry.Result.Multiplier != tt.multiplier {
				t.Errorf("multiplier = %s, want %s", entry.Result.Multiplier, tt.multiplier)
			}
			if entry.Category != tt.category {
				t.Errorf("category = %q, want %q", entry.Category, tt.category)
			}
			if entry.Result.Source != EffectSourceType {
				t.Errorf("source = %d, want EffectSourceType", entry.Result.Source)
			}
		})
	}
}

func findAttack(t *testing.T, entries []AttackDefense, attack TypeID) AttackDefense {
	t.Helper()
	for _, entry := range entries {
		if entry.AttackType == attack {
			return entry
		}
	}
	t.Fatalf("attack type %q not found in %d entries", attack, len(entries))
	return AttackDefense{}
}

func findSummary(t *testing.T, entries []TeamSummaryEntry, attack TypeID) TeamSummaryEntry {
	t.Helper()
	for _, entry := range entries {
		if entry.AttackType == attack {
			return entry
		}
	}
	t.Fatalf("attack type %q not found in %d summary entries", attack, len(entries))
	return TeamSummaryEntry{}
}

func TestAnalyzeDefenseTeamSummary(t *testing.T) {
	t.Parallel()

	neutral := func(attack TypeID, n int) TeamSummaryEntry {
		return TeamSummaryEntry{AttackType: attack, Neutral: n}
	}
	tests := []struct {
		name    string
		members []Member
		want    []TeamSummaryEntry // non-neutral rows; every other attack type must be all-neutral
	}{
		{
			name:    "four distinct members",
			members: []Member{memberA, memberB, memberC, memberD},
			want: []TeamSummaryEntry{
				{AttackType: TypeIce, Weak: 1, QuadWeak: 1, Neutral: 3},
				{AttackType: TypeFire, Resist: 1, Neutral: 3},
				{AttackType: TypeNormal, Immune: 1, Neutral: 3},
				{AttackType: TypeWater, Weak: 1, Neutral: 3},
				{AttackType: TypeGrass, Resist: 1, Neutral: 3},
			},
		},
		{
			name:    "duplicated pokemonId is counted each time",
			members: []Member{memberA, memberA, memberC},
			want: []TeamSummaryEntry{
				{AttackType: TypeIce, Weak: 2, QuadWeak: 2, Neutral: 1},
				{AttackType: TypeNormal, Immune: 1, Neutral: 2},
			},
		},
		{
			name:    "six members",
			members: []Member{memberA, memberB, memberC, memberD, memberA, memberB},
			want: []TeamSummaryEntry{
				{AttackType: TypeIce, Weak: 2, QuadWeak: 2, Neutral: 4},
				{AttackType: TypeFire, Resist: 2, Neutral: 4},
				{AttackType: TypeNormal, Immune: 1, Neutral: 5},
				{AttackType: TypeWater, Weak: 1, Neutral: 5},
				{AttackType: TypeGrass, Resist: 1, Neutral: 5},
			},
		},
		{
			name:    "single member",
			members: []Member{memberD},
			want: []TeamSummaryEntry{
				{AttackType: TypeWater, Weak: 1},
				{AttackType: TypeGrass, Resist: 1},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := AnalyzeDefense(tb1Chart, tt.members)
			if err != nil {
				t.Fatalf("AnalyzeDefense() error = %v", err)
			}
			if len(got.TeamSummary) != 18 {
				t.Fatalf("TeamSummary length = %d, want 18", len(got.TeamSummary))
			}
			want := make(map[TypeID]TeamSummaryEntry, 18)
			for _, attack := range AllTypes() {
				want[attack] = neutral(attack, len(tt.members))
			}
			for _, entry := range tt.want {
				want[entry.AttackType] = entry
			}
			for i, attack := range AllTypes() {
				entry := got.TeamSummary[i]
				if entry.AttackType != attack {
					t.Errorf("TeamSummary[%d].AttackType = %q, want %q (canonical order)", i, entry.AttackType, attack)
				}
				if entry != want[attack] {
					t.Errorf("TeamSummary[%s] = %+v, want %+v", attack, entry, want[attack])
				}
			}
		})
	}
}

// TestAnalyzeDefenseSummaryInvariants checks the ADR-0014 §3 definitions on every
// attack type against the member-level results.
func TestAnalyzeDefenseSummaryInvariants(t *testing.T) {
	t.Parallel()

	parties := [][]Member{
		{memberA},
		{memberA, memberB, memberC, memberD},
		{memberA, memberA, memberA, memberA, memberA, memberA},
		{memberA, memberB, memberC, memberD, memberC, memberB},
	}
	for pi, party := range parties {
		got, err := AnalyzeDefense(tb1Chart, party)
		if err != nil {
			t.Fatalf("party %d: AnalyzeDefense() error = %v", pi, err)
		}
		if len(got.TeamSummary) != 18 || len(got.Members) != len(party) {
			t.Fatalf("party %d: summary=%d members=%d, want 18 and %d", pi, len(got.TeamSummary), len(got.Members), len(party))
		}
		for i, entry := range got.TeamSummary {
			if entry.Weak+entry.Resist+entry.Immune+entry.Neutral != len(party) {
				t.Errorf("party %d %s: weak+resist+immune+neutral = %d, want %d", pi, entry.AttackType, entry.Weak+entry.Resist+entry.Immune+entry.Neutral, len(party))
			}
			if entry.QuadWeak > entry.Weak {
				t.Errorf("party %d %s: quadWeak %d > weak %d", pi, entry.AttackType, entry.QuadWeak, entry.Weak)
			}
			var weak, quadWeak, resist, immune, neutral int
			for _, member := range got.Members {
				switch member.Defense[i].Result.Multiplier {
				case MultiplierQuad:
					weak++
					quadWeak++
				case MultiplierDouble:
					weak++
				case MultiplierNormal:
					neutral++
				case MultiplierHalf, MultiplierQuarter:
					resist++
				case MultiplierZero:
					immune++
				}
			}
			want := TeamSummaryEntry{AttackType: entry.AttackType, Weak: weak, QuadWeak: quadWeak, Resist: resist, Immune: immune, Neutral: neutral}
			if entry != want {
				t.Errorf("party %d %s: summary %+v disagrees with member results %+v", pi, entry.AttackType, entry, want)
			}
		}
	}
}

func TestAnalyzeDefenseRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	seven := []Member{memberA, memberB, memberC, memberD, memberA, memberB, memberC}
	tests := []struct {
		name    string
		chart   TypeChartProvider
		members []Member
		wantErr error
	}{
		{name: "no members", chart: tb1Chart, members: nil, wantErr: ErrMemberCount},
		{name: "empty members", chart: tb1Chart, members: []Member{}, wantErr: ErrMemberCount},
		{name: "seven members", chart: tb1Chart, members: seven, wantErr: ErrMemberCount},
		{name: "nil chart", chart: nil, members: []Member{memberA}, wantErr: ErrNilTypeChart},
		{name: "invalid type", chart: tb1Chart, members: []Member{{PokemonID: "9001-000", Types: []TypeID{"stellar"}}}, wantErr: ErrInvalidType},
		{name: "invalid second member type", chart: tb1Chart, members: []Member{memberA, {PokemonID: "9005-000", Types: []TypeID{TypeFire, "Fire"}}}, wantErr: ErrInvalidType},
		{name: "no types", chart: tb1Chart, members: []Member{{PokemonID: "9001-000"}}, wantErr: ErrDefenseTypeCount},
		{name: "three types", chart: tb1Chart, members: []Member{{PokemonID: "9001-000", Types: []TypeID{TypeFire, TypeWater, TypeGrass}}}, wantErr: ErrDefenseTypeCount},
		{name: "duplicate types", chart: tb1Chart, members: []Member{{PokemonID: "9001-000", Types: []TypeID{TypeFire, TypeFire}}}, wantErr: ErrDuplicateDefenseType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := AnalyzeDefense(tt.chart, tt.members)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("AnalyzeDefense() error = %v, want errors.Is(_, %v)", err, tt.wantErr)
			}
		})
	}
}

func TestAnalyzeDefensePropagatesChartErrors(t *testing.T) {
	t.Parallel()

	providerErr := errors.New("chart unavailable")
	_, err := AnalyzeDefense(failingTypeChart{err: providerErr}, []Member{memberA})
	if !errors.Is(err, providerErr) {
		t.Fatalf("AnalyzeDefense() error = %v, want errors.Is(_, %v)", err, providerErr)
	}
}
