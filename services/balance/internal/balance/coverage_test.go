package balance_test

import (
	"errors"
	"fmt"
	"testing"

	"example.com/pokecalc/services/balance/internal/balance"
)

// TB2 攻撃範囲(ADR-0016)。倍率の期待値は同梱の相性表(master.EmbeddedTypeChart、P1-13 のデータ)で
// 成り立つ単タイプ同士の組だけを使う。技 ID は架空(move-9001 以降)。

func attack(id string, t balance.TypeID) balance.Move {
	return balance.Move{MoveID: id, Type: t, Category: balance.MoveCategorySpecial}
}

func physical(id string, t balance.TypeID) balance.Move {
	return balance.Move{MoveID: id, Type: t, Category: balance.MoveCategoryPhysical}
}

func status(id string, t balance.TypeID) balance.Move {
	return balance.Move{MoveID: id, Type: t, Category: balance.MoveCategoryStatus}
}

// multiplierLabel renders an optional multiplier the way the API does ("null" when absent).
func multiplierLabel(m *balance.Multiplier) string {
	if m == nil {
		return "null"
	}
	return m.String()
}

func analyzeCoverage(t *testing.T, members ...balance.CoverageMember) balance.CoverageAnalysis {
	t.Helper()
	got, err := balance.AnalyzeCoverage(testTypeChart(), members)
	if err != nil {
		t.Fatalf("AnalyzeCoverage() error = %v", err)
	}
	if len(got.Members) != len(members) {
		t.Fatalf("members length = %d, want %d", len(got.Members), len(members))
	}
	if len(got.TeamCoverage) != 18 {
		t.Fatalf("teamCoverage length = %d, want 18", len(got.TeamCoverage))
	}
	canonical := balance.AllTypes()
	for i, entry := range got.TeamCoverage {
		if entry.DefenseType != canonical[i] {
			t.Errorf("teamCoverage[%d].defenseType = %q, want %q (canonical order)", i, entry.DefenseType, canonical[i])
		}
	}
	for mi := range got.Members {
		if len(got.Members[mi].Coverage) != 18 {
			t.Fatalf("members[%d].coverage length = %d, want 18", mi, len(got.Members[mi].Coverage))
		}
		for i, entry := range got.Members[mi].Coverage {
			if entry.DefenseType != canonical[i] {
				t.Errorf("members[%d].coverage[%d].defenseType = %q, want %q (canonical order)", mi, i, entry.DefenseType, canonical[i])
			}
		}
	}
	return got
}

func coverageOf(t *testing.T, entries []balance.DefenseCoverage, defense balance.TypeID) balance.DefenseCoverage {
	t.Helper()
	for _, entry := range entries {
		if entry.DefenseType == defense {
			return entry
		}
	}
	t.Fatalf("defense type %q not found in coverage", defense)
	return balance.DefenseCoverage{}
}

func teamCoverageOf(t *testing.T, entries []balance.TeamCoverageEntry, defense balance.TypeID) balance.TeamCoverageEntry {
	t.Helper()
	for _, entry := range entries {
		if entry.DefenseType == defense {
			return entry
		}
	}
	t.Fatalf("defense type %q not found in teamCoverage", defense)
	return balance.TeamCoverageEntry{}
}

func TestAnalyzeCoverageMemberResults(t *testing.T) {
	t.Parallel()

	// Moves are given electric → fire → (grass status); attackTypes must be canonical: fire, electric.
	member := balance.CoverageMember{PokemonID: "9001-000", Moves: []balance.Move{
		attack("move-9004", balance.TypeElectric),
		physical("move-9002", balance.TypeFire),
		status("move-9006", balance.TypeGrass),
	}}
	got := analyzeCoverage(t, member).Members[0]

	if got.PokemonID != "9001-000" {
		t.Errorf("pokemonId = %q, want 9001-000", got.PokemonID)
	}
	if fmt.Sprint(got.MoveIDs) != fmt.Sprint([]string{"move-9004", "move-9002", "move-9006"}) {
		t.Errorf("moveIds = %v, want the input order including the status move", got.MoveIDs)
	}
	if fmt.Sprint(got.AttackTypes) != fmt.Sprint([]balance.TypeID{balance.TypeFire, balance.TypeElectric}) {
		t.Errorf("attackTypes = %v, want [fire electric] (status excluded, canonical order)", got.AttackTypes)
	}

	// Best of fire and electric against each single defense type (data chart).
	want := []struct {
		defense   balance.TypeID
		best      string
		effective bool
		super     bool
	}{
		{balance.TypeNormal, "1", true, false},
		{balance.TypeFire, "1", true, false}, // fire x1/2, electric x1
		{balance.TypeWater, "2", true, true}, // electric x2
		{balance.TypeElectric, "1", true, false},
		{balance.TypeGrass, "2", true, true}, // fire x2 (the grass status move is not counted)
		{balance.TypeIce, "2", true, true},
		{balance.TypeFighting, "1", true, false},
		{balance.TypePoison, "1", true, false},
		{balance.TypeGround, "1", true, false}, // electric x0, fire x1
		{balance.TypeFlying, "2", true, true},
		{balance.TypePsychic, "1", true, false},
		{balance.TypeBug, "2", true, true},
		{balance.TypeRock, "1", true, false},      // fire x1/2, electric x1
		{balance.TypeGhost, "1", true, false},     //
		{balance.TypeDragon, "1/2", false, false}, // both x1/2: not effective
		{balance.TypeDark, "1", true, false},
		{balance.TypeSteel, "2", true, true},
		{balance.TypeFairy, "1", true, false},
	}
	for i, tt := range want {
		entry := got.Coverage[i]
		if entry.DefenseType != tt.defense {
			t.Fatalf("coverage[%d].defenseType = %q, want %q", i, entry.DefenseType, tt.defense)
		}
		if multiplierLabel(entry.BestMultiplier) != tt.best || entry.Effective != tt.effective || entry.SuperEffective != tt.super {
			t.Errorf("vs %s: best=%s effective=%v super=%v, want %s %v %v",
				tt.defense, multiplierLabel(entry.BestMultiplier), entry.Effective, entry.SuperEffective, tt.best, tt.effective, tt.super)
		}
	}
}

func TestAnalyzeCoverageAttackTypesAreCanonicalAndDistinct(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		moves []balance.Move
		want  []balance.TypeID
	}{
		{
			name: "four types in reverse order",
			moves: []balance.Move{
				attack("move-9010", balance.TypeFairy), physical("move-9005", balance.TypeNormal),
				attack("move-9011", balance.TypeDragon), attack("move-9003", balance.TypeWater),
			},
			want: []balance.TypeID{balance.TypeNormal, balance.TypeWater, balance.TypeDragon, balance.TypeFairy},
		},
		{
			name:  "same type physical and special counted once",
			moves: []balance.Move{attack("move-9001", balance.TypeFire), physical("move-9002", balance.TypeFire)},
			want:  []balance.TypeID{balance.TypeFire},
		},
		{
			name: "four moves of one type",
			moves: []balance.Move{
				attack("move-9001", balance.TypeFire), physical("move-9002", balance.TypeFire),
				attack("move-9012", balance.TypeFire), physical("move-9013", balance.TypeFire),
			},
			want: []balance.TypeID{balance.TypeFire},
		},
		{
			name:  "status move of another type is excluded",
			moves: []balance.Move{status("move-9006", balance.TypeGrass), physical("move-9005", balance.TypeNormal)},
			want:  []balance.TypeID{balance.TypeNormal},
		},
		{
			name:  "status move of the same type does not add anything",
			moves: []balance.Move{status("move-9014", balance.TypeFire), attack("move-9001", balance.TypeFire)},
			want:  []balance.TypeID{balance.TypeFire},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := analyzeCoverage(t, balance.CoverageMember{PokemonID: "9001-000", Moves: tt.moves}).Members[0]
			if fmt.Sprint(got.AttackTypes) != fmt.Sprint(tt.want) {
				t.Errorf("attackTypes = %v, want %v", got.AttackTypes, tt.want)
			}
			if len(got.MoveIDs) != len(tt.moves) {
				t.Errorf("moveIds = %v, want all %d input moves (status included)", got.MoveIDs, len(tt.moves))
			}
		})
	}
}

// Members without an attack move (no moves at all, or status moves only) have no
// bestMultiplier and are never effective (ADR-0016 §2).
func TestAnalyzeCoverageWithoutAttackMoves(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		moves []balance.Move
	}{
		{name: "no moves", moves: nil},
		{name: "empty moves", moves: []balance.Move{}},
		{name: "status moves only", moves: []balance.Move{status("move-9006", balance.TypeGrass), status("move-9014", balance.TypeFire)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := analyzeCoverage(t, balance.CoverageMember{PokemonID: "9002-000", Moves: tt.moves})
			member := got.Members[0]
			if len(member.AttackTypes) != 0 {
				t.Errorf("attackTypes = %v, want none", member.AttackTypes)
			}
			if len(member.MoveIDs) != len(tt.moves) {
				t.Errorf("moveIds = %v, want %d entries", member.MoveIDs, len(tt.moves))
			}
			for _, entry := range member.Coverage {
				if entry.BestMultiplier != nil || entry.Effective || entry.SuperEffective {
					t.Errorf("vs %s: best=%s effective=%v super=%v, want null false false",
						entry.DefenseType, multiplierLabel(entry.BestMultiplier), entry.Effective, entry.SuperEffective)
				}
			}
			for _, entry := range got.TeamCoverage {
				if entry.BestMultiplier != nil || entry.EffectiveMembers != 0 || entry.SuperEffectiveMembers != 0 {
					t.Errorf("teamCoverage[%s] = best %s effective %d super %d, want null 0 0",
						entry.DefenseType, multiplierLabel(entry.BestMultiplier), entry.EffectiveMembers, entry.SuperEffectiveMembers)
				}
			}
		})
	}
}

// Immune (x0) and not very effective (x1/2) are not effective; x1 is effective but not
// super effective; x2 is both (ADR-0016 §1).
func TestAnalyzeCoverageEffectiveThresholds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		move      balance.Move
		defense   balance.TypeID
		best      string
		effective bool
		super     bool
	}{
		{"normal vs ghost immune", physical("move-9005", balance.TypeNormal), balance.TypeGhost, "0", false, false},
		{"electric vs ground immune", attack("move-9004", balance.TypeElectric), balance.TypeGround, "0", false, false},
		{"ground vs flying immune", physical("move-9007", balance.TypeGround), balance.TypeFlying, "0", false, false},
		{"dragon vs fairy immune", attack("move-9011", balance.TypeDragon), balance.TypeFairy, "0", false, false},
		{"normal vs rock resisted", physical("move-9005", balance.TypeNormal), balance.TypeRock, "1/2", false, false},
		{"fire vs water resisted", attack("move-9001", balance.TypeFire), balance.TypeWater, "1/2", false, false},
		{"normal vs normal neutral", physical("move-9005", balance.TypeNormal), balance.TypeNormal, "1", true, false},
		{"fire vs normal neutral", attack("move-9001", balance.TypeFire), balance.TypeNormal, "1", true, false},
		{"water vs fire super effective", attack("move-9003", balance.TypeWater), balance.TypeFire, "2", true, true},
		{"ground vs electric super effective", physical("move-9007", balance.TypeGround), balance.TypeElectric, "2", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := analyzeCoverage(t, balance.CoverageMember{PokemonID: "9001-000", Moves: []balance.Move{tt.move}})
			entry := coverageOf(t, got.Members[0].Coverage, tt.defense)
			if multiplierLabel(entry.BestMultiplier) != tt.best || entry.Effective != tt.effective || entry.SuperEffective != tt.super {
				t.Errorf("best=%s effective=%v super=%v, want %s %v %v",
					multiplierLabel(entry.BestMultiplier), entry.Effective, entry.SuperEffective, tt.best, tt.effective, tt.super)
			}
			team := teamCoverageOf(t, got.TeamCoverage, tt.defense)
			wantEffective, wantSuper := 0, 0
			if tt.effective {
				wantEffective = 1
			}
			if tt.super {
				wantSuper = 1
			}
			if multiplierLabel(team.BestMultiplier) != tt.best || team.EffectiveMembers != wantEffective || team.SuperEffectiveMembers != wantSuper {
				t.Errorf("team = best %s effective %d super %d, want %s %d %d",
					multiplierLabel(team.BestMultiplier), team.EffectiveMembers, team.SuperEffectiveMembers, tt.best, wantEffective, wantSuper)
			}
		})
	}
}

// Team counts are per member, not per move (design §6: no double counting).
func TestAnalyzeCoverageTeamCountsMembersNotMoves(t *testing.T) {
	t.Parallel()

	got := analyzeCoverage(t,
		balance.CoverageMember{PokemonID: "9001-000", Moves: []balance.Move{
			attack("move-9001", balance.TypeFire), physical("move-9002", balance.TypeFire),
			attack("move-9012", balance.TypeFire), physical("move-9013", balance.TypeFire),
		}},
		balance.CoverageMember{PokemonID: "9003-000", Moves: []balance.Move{attack("move-9003", balance.TypeWater)}},
	)
	tests := []struct {
		defense   balance.TypeID
		best      string
		effective int
		super     int
	}{
		{balance.TypeNormal, "1", 2, 0}, // 4 fire moves + 1 water move, but 2 members
		{balance.TypeGrass, "2", 1, 1},  // fire x2, water x1/2
		{balance.TypeFire, "2", 1, 1},   // fire x1/2, water x2
		{balance.TypeWater, "1/2", 0, 0},
		{balance.TypeDragon, "1/2", 0, 0},
		{balance.TypeGround, "2", 2, 1}, // fire x1, water x2
		{balance.TypeRock, "2", 1, 1},   // fire x1/2, water x2
		{balance.TypeSteel, "2", 2, 1},  // fire x2, water x1
	}
	for _, tt := range tests {
		entry := teamCoverageOf(t, got.TeamCoverage, tt.defense)
		if multiplierLabel(entry.BestMultiplier) != tt.best || entry.EffectiveMembers != tt.effective || entry.SuperEffectiveMembers != tt.super {
			t.Errorf("teamCoverage[%s] = best %s effective %d super %d, want %s %d %d", tt.defense,
				multiplierLabel(entry.BestMultiplier), entry.EffectiveMembers, entry.SuperEffectiveMembers, tt.best, tt.effective, tt.super)
		}
	}
}

func TestAnalyzeCoverageTeamSummary(t *testing.T) {
	t.Parallel()

	// A: normal only, B: no moves, C: electric only, A again (duplicated pokemonId counts twice).
	a := balance.CoverageMember{PokemonID: "9001-000", Moves: []balance.Move{physical("move-9005", balance.TypeNormal)}}
	b := balance.CoverageMember{PokemonID: "9002-000"}
	c := balance.CoverageMember{PokemonID: "9003-000", Moves: []balance.Move{attack("move-9004", balance.TypeElectric)}}
	got := analyzeCoverage(t, a, b, c, a)

	wantIDs := []string{"9001-000", "9002-000", "9003-000", "9001-000"}
	for i, id := range wantIDs {
		if got.Members[i].PokemonID != id {
			t.Errorf("members[%d].pokemonId = %q, want %q (input order, duplicates kept)", i, got.Members[i].PokemonID, id)
		}
	}
	tests := []struct {
		defense   balance.TypeID
		best      string
		effective int
		super     int
	}{
		{balance.TypeNormal, "1", 3, 0},   // A x1 twice, C x1
		{balance.TypeGhost, "1", 1, 0},    // A x0 twice, C x1
		{balance.TypeGround, "1", 2, 0},   // A x1 twice, C x0
		{balance.TypeFlying, "2", 3, 1},   // A x1 twice, C x2
		{balance.TypeWater, "2", 3, 1},    // A x1 twice, C x2
		{balance.TypeRock, "1", 1, 0},     // A x1/2 twice, C x1
		{balance.TypeDragon, "1", 2, 0},   // A x1 twice, C x1/2
		{balance.TypeElectric, "1", 2, 0}, // A x1 twice, C x1/2
	}
	for _, tt := range tests {
		entry := teamCoverageOf(t, got.TeamCoverage, tt.defense)
		if multiplierLabel(entry.BestMultiplier) != tt.best || entry.EffectiveMembers != tt.effective || entry.SuperEffectiveMembers != tt.super {
			t.Errorf("teamCoverage[%s] = best %s effective %d super %d, want %s %d %d", tt.defense,
				multiplierLabel(entry.BestMultiplier), entry.EffectiveMembers, entry.SuperEffectiveMembers, tt.best, tt.effective, tt.super)
		}
	}

	// The member without moves keeps null entries even inside a team that has attack moves.
	for _, entry := range got.Members[1].Coverage {
		if entry.BestMultiplier != nil || entry.Effective || entry.SuperEffective {
			t.Errorf("members[1] vs %s = %s %v %v, want null false false", entry.DefenseType,
				multiplierLabel(entry.BestMultiplier), entry.Effective, entry.SuperEffective)
		}
	}
}

func TestAnalyzeCoverageTeamBestCanBeImmuneOrResisted(t *testing.T) {
	t.Parallel()

	// Only one normal attacker (plus a member without moves): the team best vs ghost is x0, vs rock x1/2.
	got := analyzeCoverage(t,
		balance.CoverageMember{PokemonID: "9001-000", Moves: []balance.Move{physical("move-9005", balance.TypeNormal)}},
		balance.CoverageMember{PokemonID: "9002-000", Moves: []balance.Move{status("move-9006", balance.TypeGrass)}},
	)
	for _, tt := range []struct {
		defense balance.TypeID
		best    string
	}{{balance.TypeGhost, "0"}, {balance.TypeRock, "1/2"}, {balance.TypeSteel, "1/2"}, {balance.TypeNormal, "1"}} {
		entry := teamCoverageOf(t, got.TeamCoverage, tt.defense)
		if multiplierLabel(entry.BestMultiplier) != tt.best {
			t.Errorf("teamCoverage[%s].bestMultiplier = %s, want %s", tt.defense, multiplierLabel(entry.BestMultiplier), tt.best)
		}
	}
}

// TestAnalyzeCoverageAgreesWithChartOracle runs every single attack type and every pair of
// attack types (as one member each, six members per call) against the data chart, and checks
// each entry against an oracle computed from Matchup, and each team entry recomputed from the
// member entries.
func TestAnalyzeCoverageAgreesWithChartOracle(t *testing.T) {
	t.Parallel()

	chart := testTypeChart()
	types := balance.AllTypes()
	var sets [][]balance.TypeID
	for i, first := range types {
		sets = append(sets, []balance.TypeID{first})
		for _, second := range types[i+1:] {
			sets = append(sets, []balance.TypeID{first, second})
		}
	}
	if len(sets) != 171 {
		t.Fatalf("attack type sets = %d, want 171", len(sets))
	}

	for start := 0; start < len(sets); start += balance.MaxMembers {
		end := min(start+balance.MaxMembers, len(sets))
		members := make([]balance.CoverageMember, 0, end-start)
		for i, set := range sets[start:end] {
			moves := make([]balance.Move, len(set))
			for j, attackType := range set {
				moves[j] = attack(fmt.Sprintf("move-%d-%d", 9001+start+i, j), attackType)
			}
			members = append(members, balance.CoverageMember{PokemonID: fmt.Sprintf("9%03d-000", start+i+1), Moves: moves})
		}
		got, err := balance.AnalyzeCoverage(chart, members)
		if err != nil {
			t.Fatalf("AnalyzeCoverage(%v) error = %v", members, err)
		}
		if len(got.Members) != len(members) || len(got.TeamCoverage) != 18 {
			t.Fatalf("members=%d teamCoverage=%d, want %d and 18", len(got.Members), len(got.TeamCoverage), len(members))
		}

		for di, defense := range types {
			var teamBest *balance.Multiplier
			effective, super := 0, 0
			for mi, set := range sets[start:end] {
				if len(got.Members[mi].Coverage) != 18 {
					t.Fatalf("%v: coverage length = %d, want 18", set, len(got.Members[mi].Coverage))
				}
				best := balance.MultiplierZero
				for _, attackType := range set {
					m, err := chart.Matchup(attackType, defense)
					if err != nil {
						t.Fatalf("Matchup(%s, %s) error = %v", attackType, defense, err)
					}
					best = max(best, m)
				}
				entry := got.Members[mi].Coverage[di]
				wantEffective := best >= balance.MultiplierNormal
				wantSuper := best == balance.MultiplierDouble
				if entry.DefenseType != defense || entry.BestMultiplier == nil || *entry.BestMultiplier != best ||
					entry.Effective != wantEffective || entry.SuperEffective != wantSuper {
					t.Errorf("%v vs %s = %+v (best %s), want best %s effective %v super %v",
						set, defense, entry, multiplierLabel(entry.BestMultiplier), best, wantEffective, wantSuper)
				}
				if teamBest == nil || best > *teamBest {
					b := best
					teamBest = &b
				}
				if wantEffective {
					effective++
				}
				if wantSuper {
					super++
				}
			}
			team := got.TeamCoverage[di]
			if team.DefenseType != defense || multiplierLabel(team.BestMultiplier) != multiplierLabel(teamBest) ||
				team.EffectiveMembers != effective || team.SuperEffectiveMembers != super {
				t.Errorf("teamCoverage[%s] = best %s effective %d super %d, want %s %d %d", defense,
					multiplierLabel(team.BestMultiplier), team.EffectiveMembers, team.SuperEffectiveMembers,
					multiplierLabel(teamBest), effective, super)
			}
			if team.SuperEffectiveMembers > team.EffectiveMembers || team.EffectiveMembers > len(members) {
				t.Errorf("teamCoverage[%s]: superEffective %d, effective %d, members %d violate super <= effective <= members",
					defense, team.SuperEffectiveMembers, team.EffectiveMembers, len(members))
			}
		}
	}
}

func TestAnalyzeCoverageAcceptsOneToSixMembers(t *testing.T) {
	t.Parallel()

	member := balance.CoverageMember{PokemonID: "9001-000", Moves: []balance.Move{attack("move-9001", balance.TypeFire)}}
	for _, n := range []int{1, balance.MaxMembers} {
		members := make([]balance.CoverageMember, n)
		for i := range members {
			members[i] = member
		}
		got := analyzeCoverage(t, members...)
		if entry := teamCoverageOf(t, got.TeamCoverage, balance.TypeGrass); entry.EffectiveMembers != n || entry.SuperEffectiveMembers != n {
			t.Errorf("%d members: teamCoverage[grass] = %+v, want %d effective and super effective", n, entry, n)
		}
	}
}

// errorTypeChart fails every lookup, or returns a multiplier impossible for one single type.
type errorTypeChart struct {
	multiplier balance.Multiplier
	err        error
}

func (c errorTypeChart) Matchup(balance.TypeID, balance.TypeID) (balance.Multiplier, error) {
	return c.multiplier, c.err
}

func TestAnalyzeCoverageRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	fire := attack("move-9001", balance.TypeFire)
	one := []balance.CoverageMember{{PokemonID: "9001-000", Moves: []balance.Move{fire}}}
	seven := make([]balance.CoverageMember, balance.MaxMembers+1)
	for i := range seven {
		seven[i] = one[0]
	}
	chartErr := errors.New("chart unavailable")
	tests := []struct {
		name    string
		chart   balance.TypeChartProvider
		members []balance.CoverageMember
		wantErr error
	}{
		{name: "no members", chart: testTypeChart(), members: nil, wantErr: balance.ErrMemberCount},
		{name: "seven members", chart: testTypeChart(), members: seven, wantErr: balance.ErrMemberCount},
		{name: "five moves", chart: testTypeChart(), members: []balance.CoverageMember{{PokemonID: "9001-000", Moves: []balance.Move{
			fire, attack("move-9003", balance.TypeWater), attack("move-9004", balance.TypeElectric),
			physical("move-9005", balance.TypeNormal), status("move-9006", balance.TypeGrass),
		}}}, wantErr: balance.ErrMoveCount},
		{name: "duplicate moveId in a member", chart: testTypeChart(), members: []balance.CoverageMember{{PokemonID: "9001-000", Moves: []balance.Move{fire, fire}}}, wantErr: balance.ErrDuplicateMove},
		{name: "invalid attack move type", chart: testTypeChart(), members: []balance.CoverageMember{{PokemonID: "9001-000", Moves: []balance.Move{attack("move-9001", "stellar")}}}, wantErr: balance.ErrInvalidType},
		{name: "invalid move category", chart: testTypeChart(), members: []balance.CoverageMember{{PokemonID: "9001-000", Moves: []balance.Move{{MoveID: "move-9001", Type: balance.TypeFire, Category: "other"}}}}, wantErr: balance.ErrInvalidMoveCategory},
		{name: "empty move category", chart: testTypeChart(), members: []balance.CoverageMember{{PokemonID: "9001-000", Moves: []balance.Move{{MoveID: "move-9001", Type: balance.TypeFire}}}}, wantErr: balance.ErrInvalidMoveCategory},
		{name: "nil chart", chart: nil, members: one, wantErr: balance.ErrNilTypeChart},
		{name: "chart error is propagated", chart: errorTypeChart{err: chartErr}, members: one, wantErr: chartErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := balance.AnalyzeCoverage(tt.chart, tt.members); !errors.Is(err, tt.wantErr) {
				t.Fatalf("AnalyzeCoverage() error = %v, want errors.Is(_, %v)", err, tt.wantErr)
			}
		})
	}
}

func TestAnalyzeCoverageRejectsInvalidChartMultiplier(t *testing.T) {
	t.Parallel()

	members := []balance.CoverageMember{{PokemonID: "9001-000", Moves: []balance.Move{attack("move-9001", balance.TypeFire)}}}
	for _, m := range []balance.Multiplier{balance.MultiplierQuarter, balance.MultiplierQuad, 3, 255} {
		if _, err := balance.AnalyzeCoverage(errorTypeChart{multiplier: m}, members); err == nil {
			t.Errorf("AnalyzeCoverage() with chart multiplier %d: error = nil, want error (not a single-type multiplier)", m)
		}
	}
}

// The same moveId in different members is fine; only duplicates within one member are rejected.
func TestAnalyzeCoverageSameMoveAcrossMembers(t *testing.T) {
	t.Parallel()

	fire := attack("move-9001", balance.TypeFire)
	got := analyzeCoverage(t,
		balance.CoverageMember{PokemonID: "9001-000", Moves: []balance.Move{fire}},
		balance.CoverageMember{PokemonID: "9002-000", Moves: []balance.Move{fire}},
	)
	if entry := teamCoverageOf(t, got.TeamCoverage, balance.TypeGrass); entry.SuperEffectiveMembers != 2 {
		t.Errorf("teamCoverage[grass].superEffectiveMembers = %d, want 2", entry.SuperEffectiveMembers)
	}
}
