package master_test

import (
	"errors"
	"fmt"
	"testing"

	"example.com/pokecalc/services/balance/internal/domain"
)

// TB4 仮想敵診断(ADR-0400)。倍率の期待値は同梱の相性表(master.EmbeddedTypeChart)で成り立つ組だけを使う。
// ID はすべて架空(9001-000 / move-9001 / ability-9001 以降)。

func combatant(id string, types []domain.TypeID, ability *domain.Ability, moves ...domain.Move) domain.Combatant {
	return domain.Combatant{PokemonID: id, Types: types, Ability: ability, Moves: moves}
}

func types(ts ...domain.TypeID) []domain.TypeID { return ts }

func threatAbility(id string, effects ...domain.AbilityEffect) *domain.Ability {
	return &domain.Ability{AbilityID: id, Effects: effects}
}

func immuneEffect(attack domain.TypeID) domain.AbilityEffect {
	return domain.AbilityEffect{Kind: domain.AbilityEffectImmune, AttackType: attack}
}

func absorbEffect(attack domain.TypeID) domain.AbilityEffect {
	return domain.AbilityEffect{Kind: domain.AbilityEffectAbsorb, AttackType: attack}
}

func typeFactorEffect(attack domain.TypeID, num, den int64) domain.AbilityEffect {
	return domain.AbilityEffect{Kind: domain.AbilityEffectTypeMultiplier, AttackType: attack, Factor: domain.Effectiveness{Num: num, Den: den}}
}

func superEffectiveFactorEffect(num, den int64) domain.AbilityEffect {
	return domain.AbilityEffect{Kind: domain.AbilityEffectSuperEffectiveMultiplier, Factor: domain.Effectiveness{Num: num, Den: den}}
}

// effectivenessLabel renders an optional multiplier the way the API does ("null" when absent).
func effectivenessLabel(e *domain.Effectiveness) string {
	if e == nil {
		return "null"
	}
	return e.String()
}

var (
	effectivenessOne = domain.Effectiveness{Num: 1, Den: 1}
	effectivenessTwo = domain.Effectiveness{Num: 2, Den: 1}
)

// analyzeThreats calls AnalyzeThreats and checks the ADR-0400 §2 structure on every result:
// threats and matchups in input order, abilityId, safe / superEffective derived from the
// multipliers (false when null), multipliers in lowest terms, and the per-threat counts.
func analyzeThreats(t *testing.T, members, threats []domain.Combatant) domain.ThreatAnalysis {
	t.Helper()
	got, err := domain.AnalyzeThreats(testTypeChart(), members, threats)
	if err != nil {
		t.Fatalf("AnalyzeThreats() error = %v", err)
	}
	if len(got.Threats) != len(threats) {
		t.Fatalf("threats length = %d, want %d", len(got.Threats), len(threats))
	}
	for ti, result := range got.Threats {
		if result.PokemonID != threats[ti].PokemonID {
			t.Errorf("threats[%d].PokemonID = %q, want %q (input order)", ti, result.PokemonID, threats[ti].PokemonID)
		}
		wantAbilityID := ""
		if threats[ti].Ability != nil {
			wantAbilityID = threats[ti].Ability.AbilityID
		}
		if result.AbilityID != wantAbilityID {
			t.Errorf("threats[%d].AbilityID = %q, want %q", ti, result.AbilityID, wantAbilityID)
		}
		if len(result.Matchups) != len(members) {
			t.Fatalf("threats[%d].matchups length = %d, want %d", ti, len(result.Matchups), len(members))
		}
		safe, super := 0, 0
		for mi, matchup := range result.Matchups {
			if matchup.PokemonID != members[mi].PokemonID {
				t.Errorf("threats[%d].matchups[%d].PokemonID = %q, want %q (member order)", ti, mi, matchup.PokemonID, members[mi].PokemonID)
			}
			for name, e := range map[string]*domain.Effectiveness{"incoming": matchup.Incoming, "outgoing": matchup.Outgoing} {
				if e == nil {
					continue
				}
				reduced, err := domain.NewEffectiveness(e.Num, e.Den)
				if err != nil || reduced != *e {
					t.Errorf("threats[%d].matchups[%d].%s = %+v is not an irreducible fraction", ti, mi, name, *e)
				}
			}
			wantSafe := matchup.Incoming != nil && matchup.Incoming.Cmp(effectivenessOne) < 0
			wantSuper := matchup.Outgoing != nil && matchup.Outgoing.Cmp(effectivenessTwo) >= 0
			if matchup.Safe != wantSafe {
				t.Errorf("threats[%d].matchups[%d].Safe = %v with incoming %s, want %v", ti, mi, matchup.Safe, effectivenessLabel(matchup.Incoming), wantSafe)
			}
			if matchup.SuperEffective != wantSuper {
				t.Errorf("threats[%d].matchups[%d].SuperEffective = %v with outgoing %s, want %v", ti, mi, matchup.SuperEffective, effectivenessLabel(matchup.Outgoing), wantSuper)
			}
			if matchup.Safe {
				safe++
			}
			if matchup.SuperEffective {
				super++
			}
		}
		if result.SafeMembers != safe || result.SuperEffectiveMembers != super {
			t.Errorf("threats[%d] counts = safe %d super %d, want %d %d (recounted from matchups)", ti, result.SafeMembers, result.SuperEffectiveMembers, safe, super)
		}
	}
	return got
}

type wantMatchup struct {
	incoming string
	outgoing string
	safe     bool
	super    bool
}

func assertMatchup(t *testing.T, name string, got domain.ThreatMatchup, want wantMatchup) {
	t.Helper()
	if effectivenessLabel(got.Incoming) != want.incoming || effectivenessLabel(got.Outgoing) != want.outgoing ||
		got.Safe != want.safe || got.SuperEffective != want.super {
		t.Errorf("%s: got incoming %s outgoing %s safe %v super %v, want %s %s %v %v", name,
			effectivenessLabel(got.Incoming), effectivenessLabel(got.Outgoing), got.Safe, got.SuperEffective,
			want.incoming, want.outgoing, want.safe, want.super)
	}
}

// ADR-0400 §3: incoming / outgoing are the largest multiplier over the non-status attack
// types; safe is incoming < 1 and superEffective is outgoing >= 2; null without an attack move.
func TestAnalyzeThreatsMatchups(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		member domain.Combatant
		threat domain.Combatant
		want   wantMatchup
	}{
		{
			name:   "x2 incoming is not safe, x1/2 outgoing is not super effective",
			member: combatant("9002-000", types(domain.TypeGrass), nil, attack("move-9001", domain.TypeFire)),
			threat: combatant("9101-000", types(domain.TypeWater), nil, attack("move-9003", domain.TypeWater), attack("move-9008", domain.TypeIce)),
			want:   wantMatchup{incoming: "2", outgoing: "1/2"},
		},
		{
			name:   "type immunity x0 incoming is safe, x2 outgoing is super effective",
			member: combatant("9003-000", types(domain.TypeWater, domain.TypeGround), nil, physical("move-9007", domain.TypeGround)),
			threat: combatant("9101-000", types(domain.TypeElectric), nil, attack("move-9004", domain.TypeElectric)),
			want:   wantMatchup{incoming: "0", outgoing: "2", safe: true, super: true},
		},
		{
			name:   "x1 is neither safe nor super effective",
			member: combatant("9004-000", types(domain.TypeNormal), nil, physical("move-9005", domain.TypeNormal)),
			threat: combatant("9101-000", types(domain.TypeNormal), nil, physical("move-9005", domain.TypeNormal)),
			want:   wantMatchup{incoming: "1", outgoing: "1"},
		},
		{
			name:   "x1/4 incoming on a dual type is safe",
			member: combatant("9101-000", types(domain.TypeWater, domain.TypeDragon), nil, attack("move-9003", domain.TypeWater)),
			threat: combatant("9102-000", types(domain.TypeFire), nil, attack("move-9001", domain.TypeFire)),
			want:   wantMatchup{incoming: "1/4", outgoing: "2", safe: true, super: true},
		},
		{
			name:   "x1/2 incoming is safe, a member without moves has null outgoing",
			member: combatant("9002-000", types(domain.TypeGrass), nil),
			threat: combatant("9101-000", types(domain.TypeWater), nil, attack("move-9003", domain.TypeWater)),
			want:   wantMatchup{incoming: "1/2", outgoing: "null", safe: true},
		},
		{
			name:   "x4 outgoing on a dual type is super effective, a threat without moves has null incoming",
			member: combatant("9005-000", types(domain.TypeIce), nil, attack("move-9008", domain.TypeIce)),
			threat: combatant("9006-001", types(domain.TypeDragon, domain.TypeFlying), nil),
			want:   wantMatchup{incoming: "null", outgoing: "4", super: true},
		},
		{
			name:   "status moves never count (fire and water status would be x2 / x1/2)",
			member: combatant("9002-000", types(domain.TypeGrass), nil, status("move-9006", domain.TypeFire)),
			threat: combatant("9101-000", types(domain.TypeWater), nil, status("move-9012", domain.TypeFire), status("move-9013", domain.TypeWater)),
			want:   wantMatchup{incoming: "null", outgoing: "null"},
		},
		{
			name:   "a status move beside an attack move is ignored (grass status would be x2)",
			member: combatant("9101-000", types(domain.TypeWater), nil),
			threat: combatant("9102-000", types(domain.TypeWater), nil, status("move-9006", domain.TypeGrass), attack("move-9003", domain.TypeWater)),
			want:   wantMatchup{incoming: "1/2", outgoing: "null", safe: true},
		},
		{
			// poison vs steel/fairy x0, water x1, ground x2.
			name:   "the maximum over attack types: x0 and x1 do not hide x2",
			member: combatant("9006-000", types(domain.TypeSteel, domain.TypeFairy), nil, physical("move-9007", domain.TypeGround)),
			threat: combatant("9101-000", types(domain.TypePoison), nil,
				physical("move-9014", domain.TypePoison), attack("move-9003", domain.TypeWater), physical("move-9007", domain.TypeGround)),
			want: wantMatchup{incoming: "2", outgoing: "2", super: true},
		},
		{
			name:   "x0 outgoing is not super effective",
			member: combatant("9004-000", types(domain.TypeNormal), nil, physical("move-9005", domain.TypeNormal)),
			threat: combatant("9101-000", types(domain.TypeGhost), nil),
			want:   wantMatchup{incoming: "null", outgoing: "0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := analyzeThreats(t, []domain.Combatant{tt.member}, []domain.Combatant{tt.threat})
			assertMatchup(t, tt.name, got.Threats[0].Matchups[0], tt.want)
		})
	}
}

// ADR-0400 §3: incoming uses the member's ability, outgoing uses the threat's ability
// (CalculateDefenseWithAbility, ADR-0017). An ability never changes the side's own attacks.
func TestAnalyzeThreatsAppliesDefenderAbility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		member domain.Combatant
		threat domain.Combatant
		want   wantMatchup
	}{
		{
			name:   "member absorb makes x2 into x0 (safe)",
			member: combatant("9101-000", types(domain.TypeFire), threatAbility("ability-9002", absorbEffect(domain.TypeWater))),
			threat: combatant("9102-000", types(domain.TypeWater), nil, attack("move-9003", domain.TypeWater)),
			want:   wantMatchup{incoming: "0", outgoing: "null", safe: true},
		},
		{
			name:   "member immune makes x2 into x0 (safe)",
			member: combatant("9101-000", types(domain.TypeFire), threatAbility("ability-9001", immuneEffect(domain.TypeGround))),
			threat: combatant("9102-000", types(domain.TypeGround), nil, physical("move-9007", domain.TypeGround)),
			want:   wantMatchup{incoming: "0", outgoing: "null", safe: true},
		},
		{
			name:   "member type multiplier x3/4 makes x1 into x3/4 (below neutral, safe)",
			member: combatant("9004-000", types(domain.TypeNormal), threatAbility("ability-9101", typeFactorEffect(domain.TypeFire, 3, 4))),
			threat: combatant("9101-000", types(domain.TypeFire), nil, attack("move-9001", domain.TypeFire)),
			want:   wantMatchup{incoming: "3/4", outgoing: "null", safe: true},
		},
		{
			name:   "member type multiplier x5/4 makes x1 into x5/4 (not safe)",
			member: combatant("9004-000", types(domain.TypeNormal), threatAbility("ability-9006", typeFactorEffect(domain.TypeFire, 5, 4))),
			threat: combatant("9101-000", types(domain.TypeFire), nil, attack("move-9001", domain.TypeFire)),
			want:   wantMatchup{incoming: "5/4", outgoing: "null"},
		},
		{
			name:   "member type multiplier x1/2 makes x2 into exactly x1 (not safe)",
			member: combatant("9002-000", types(domain.TypeGrass), threatAbility("ability-9003", typeFactorEffect(domain.TypeFire, 1, 2))),
			threat: combatant("9101-000", types(domain.TypeFire), nil, attack("move-9001", domain.TypeFire)),
			want:   wantMatchup{incoming: "1", outgoing: "null"},
		},
		{
			name:   "member super effective x3/4 makes x2 into x3/2 (still not safe)",
			member: combatant("9002-000", types(domain.TypeGrass), threatAbility("ability-9004", superEffectiveFactorEffect(3, 4))),
			threat: combatant("9101-000", types(domain.TypeFire), nil, attack("move-9001", domain.TypeFire)),
			want:   wantMatchup{incoming: "3/2", outgoing: "null"},
		},
		{
			name:   "threat super effective x3/4 makes x2 into x3/2 (not super effective)",
			member: combatant("9101-000", types(domain.TypeFire), nil, attack("move-9001", domain.TypeFire)),
			threat: combatant("9002-000", types(domain.TypeGrass), threatAbility("ability-9004", superEffectiveFactorEffect(3, 4))),
			want:   wantMatchup{incoming: "null", outgoing: "3/2"},
		},
		{
			name:   "threat super effective x3/4 makes x4 into x3 (super effective)",
			member: combatant("9101-000", types(domain.TypeFire), nil, attack("move-9001", domain.TypeFire)),
			threat: combatant("9102-000", types(domain.TypeGrass, domain.TypeSteel), threatAbility("ability-9004", superEffectiveFactorEffect(3, 4))),
			want:   wantMatchup{incoming: "null", outgoing: "3", super: true},
		},
		{
			name:   "threat immune makes x2 into x0",
			member: combatant("9003-000", types(domain.TypeWater, domain.TypeGround), nil, physical("move-9007", domain.TypeGround)),
			threat: combatant("9101-000", types(domain.TypeElectric), threatAbility("ability-9001", immuneEffect(domain.TypeGround))),
			want:   wantMatchup{incoming: "null", outgoing: "0"},
		},
		{
			name:   "threat immune to one type: the other attack type is the maximum",
			member: combatant("9003-000", types(domain.TypeWater, domain.TypeGround), nil, physical("move-9007", domain.TypeGround), attack("move-9003", domain.TypeWater)),
			threat: combatant("9101-000", types(domain.TypeElectric), threatAbility("ability-9001", immuneEffect(domain.TypeGround))),
			want:   wantMatchup{incoming: "null", outgoing: "1"},
		},
		{
			name:   "the threat's ability does not change its own attacks (incoming)",
			member: combatant("9002-000", types(domain.TypeGrass), nil),
			threat: combatant("9101-000", types(domain.TypeFire), threatAbility("ability-9003", typeFactorEffect(domain.TypeFire, 1, 2)), attack("move-9001", domain.TypeFire)),
			want:   wantMatchup{incoming: "2", outgoing: "null"},
		},
		{
			name:   "the member's ability does not change its own attacks (outgoing)",
			member: combatant("9101-000", types(domain.TypeWater), threatAbility("ability-9102", typeFactorEffect(domain.TypeWater, 1, 2)), attack("move-9003", domain.TypeWater)),
			threat: combatant("9102-000", types(domain.TypeFire), nil),
			want:   wantMatchup{incoming: "null", outgoing: "2", super: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := analyzeThreats(t, []domain.Combatant{tt.member}, []domain.Combatant{tt.threat})
			assertMatchup(t, tt.name, got.Threats[0].Matchups[0], tt.want)
		})
	}
}

// ADR-0400 §2: attackTypes are the threat's non-status move types, without duplicates, in canonical order.
func TestAnalyzeThreatsAttackTypesAreCanonicalAndDistinct(t *testing.T) {
	t.Parallel()

	member := combatant("9002-000", types(domain.TypeGrass), nil)
	tests := []struct {
		name  string
		moves []domain.Move
		want  []domain.TypeID
	}{
		{name: "canonical order, status excluded", moves: []domain.Move{
			attack("move-9008", domain.TypeIce), physical("move-9002", domain.TypeFire),
			status("move-9006", domain.TypeGrass), attack("move-9001", domain.TypeFire),
		}, want: []domain.TypeID{domain.TypeFire, domain.TypeIce}},
		{name: "same type twice counts once", moves: []domain.Move{
			attack("move-9011", domain.TypeDragon), attack("move-9003", domain.TypeWater),
			physical("move-9005", domain.TypeNormal), physical("move-9015", domain.TypeWater),
		}, want: []domain.TypeID{domain.TypeNormal, domain.TypeWater, domain.TypeDragon}},
		{name: "no moves", moves: nil, want: []domain.TypeID{}},
		{name: "status only", moves: []domain.Move{status("move-9006", domain.TypeGrass), status("move-9012", domain.TypePsychic)}, want: []domain.TypeID{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			threat := combatant("9101-000", types(domain.TypeNormal), nil, tt.moves...)
			got := analyzeThreats(t, []domain.Combatant{member}, []domain.Combatant{threat})
			if fmt.Sprint(got.Threats[0].AttackTypes) != fmt.Sprint(tt.want) {
				t.Errorf("attackTypes = %v, want %v", got.Threats[0].AttackTypes, tt.want)
			}
		})
	}
}

// ADR-0400 §2: threats and matchups keep the request order (duplicated members are kept),
// and safeMembers / superEffectiveMembers count members per threat.
func TestAnalyzeThreatsOrderAndCounts(t *testing.T) {
	t.Parallel()

	members := []domain.Combatant{
		combatant("9002-000", types(domain.TypeGrass), nil, attack("move-9001", domain.TypeFire)),
		combatant("9003-000", types(domain.TypeWater, domain.TypeGround), nil, physical("move-9007", domain.TypeGround)),
		combatant("9002-000", types(domain.TypeGrass), nil, attack("move-9001", domain.TypeFire)),
	}
	threats := []domain.Combatant{
		combatant("9104-000", types(domain.TypeFire), nil, attack("move-9001", domain.TypeFire)),
		combatant("9101-000", types(domain.TypeElectric), threatAbility("ability-9001", immuneEffect(domain.TypeGround)), attack("move-9004", domain.TypeElectric)),
		combatant("9103-000", types(domain.TypeWater), nil),
		combatant("9102-000", types(domain.TypeIce), nil, status("move-9012", domain.TypePsychic)),
	}
	got := analyzeThreats(t, members, threats)

	want := []struct {
		abilityID string
		matchups  []wantMatchup
		safe      int
		super     int
	}{
		{ // fire: grass x2 / fire vs fire x1/2; water-ground x1/2 / ground vs fire x2
			matchups: []wantMatchup{{"2", "1/2", false, false}, {"1/2", "2", true, true}, {"2", "1/2", false, false}},
			safe:     1, super: 1,
		},
		{ // electric with ground immunity: grass x1/2 / fire x1; water-ground x0 / ground x0 by ability
			abilityID: "ability-9001",
			matchups:  []wantMatchup{{"1/2", "1", true, false}, {"0", "0", true, false}, {"1/2", "1", true, false}},
			safe:      3, super: 0,
		},
		{ // water without moves: fire vs water x1/2, ground vs water x1
			matchups: []wantMatchup{{"null", "1/2", false, false}, {"null", "1", false, false}, {"null", "1/2", false, false}},
			safe:     0, super: 0,
		},
		{ // ice with a status move only: fire vs ice x2, ground vs ice x1
			matchups: []wantMatchup{{"null", "2", false, true}, {"null", "1", false, false}, {"null", "2", false, true}},
			safe:     0, super: 2,
		},
	}
	for ti, w := range want {
		result := got.Threats[ti]
		if result.AbilityID != w.abilityID {
			t.Errorf("threats[%d].AbilityID = %q, want %q", ti, result.AbilityID, w.abilityID)
		}
		for mi, wm := range w.matchups {
			assertMatchup(t, fmt.Sprintf("threats[%d].matchups[%d]", ti, mi), result.Matchups[mi], wm)
		}
		if result.SafeMembers != w.safe || result.SuperEffectiveMembers != w.super {
			t.Errorf("threats[%d] = safe %d super %d, want %d %d", ti, result.SafeMembers, result.SuperEffectiveMembers, w.safe, w.super)
		}
	}
}

// ADR-0400 §3 reuses CalculateDefenseWithAbility: for every attack type and every single / dual
// defense type, incoming and outgoing with one attack type equal its result on the data chart.
func TestAnalyzeThreatsAgreesWithCalculateDefenseWithAbility(t *testing.T) {
	t.Parallel()

	chart := testTypeChart()
	for _, combo := range allDefenseTypeCombos() {
		for _, attackType := range domain.AllTypes() {
			want, err := domain.CalculateDefenseWithAbility(chart, attackType, combo, nil)
			if err != nil {
				t.Fatalf("CalculateDefenseWithAbility(%s, %v) error = %v", attackType, combo, err)
			}
			move := attack("move-9001", attackType)

			incoming, err := domain.AnalyzeThreats(chart,
				[]domain.Combatant{combatant("9001-000", combo, nil)},
				[]domain.Combatant{combatant("9101-000", types(domain.TypeNormal), nil, move)})
			if err != nil || len(incoming.Threats) != 1 || len(incoming.Threats[0].Matchups) != 1 {
				t.Fatalf("incoming %s vs %v: result %+v, error %v", attackType, combo, incoming, err)
			}
			if got := incoming.Threats[0].Matchups[0].Incoming; got == nil || *got != want.Effectiveness {
				t.Errorf("incoming %s vs %v = %s, want %s", attackType, combo, effectivenessLabel(got), want.Effectiveness)
			}

			outgoing, err := domain.AnalyzeThreats(chart,
				[]domain.Combatant{combatant("9001-000", types(domain.TypeNormal), nil, move)},
				[]domain.Combatant{combatant("9101-000", combo, nil)})
			if err != nil || len(outgoing.Threats) != 1 || len(outgoing.Threats[0].Matchups) != 1 {
				t.Fatalf("outgoing %s vs %v: result %+v, error %v", attackType, combo, outgoing, err)
			}
			if got := outgoing.Threats[0].Matchups[0].Outgoing; got == nil || *got != want.Effectiveness {
				t.Errorf("outgoing %s vs %v = %s, want %s", attackType, combo, effectivenessLabel(got), want.Effectiveness)
			}
		}
	}
}

func TestAnalyzeThreatsAcceptsOneToSixEntries(t *testing.T) {
	t.Parallel()

	entry := combatant("9002-000", types(domain.TypeGrass), nil, attack("move-9001", domain.TypeFire))
	list := make([]domain.Combatant, domain.MaxMembers)
	for i := range list {
		list[i] = entry
	}
	// members and threats are checked independently at 1 and 6 entries.
	for _, n := range []int{1, domain.MaxMembers} {
		for _, m := range []int{1, domain.MaxMembers} {
			members := list[:m]
			got := analyzeThreats(t, members, list[:n])
			if len(got.Threats) != n {
				t.Errorf("%d members x %d threats: threats length = %d", m, n, len(got.Threats))
			}
		}
	}
}

// ADR-0400 §2 (1〜6) and §4 (other failures are internal errors).
func TestAnalyzeThreatsRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	one := []domain.Combatant{combatant("9002-000", types(domain.TypeGrass), nil, attack("move-9001", domain.TypeFire))}
	seven := make([]domain.Combatant, domain.MaxMembers+1)
	for i := range seven {
		seven[i] = one[0]
	}
	chartErr := errors.New("chart unavailable")
	tests := []struct {
		name    string
		chart   domain.TypeChartProvider
		members []domain.Combatant
		threats []domain.Combatant
		wantErr error
	}{
		{name: "no members", chart: testTypeChart(), members: nil, threats: one, wantErr: domain.ErrMemberCount},
		{name: "empty members", chart: testTypeChart(), members: []domain.Combatant{}, threats: one, wantErr: domain.ErrMemberCount},
		{name: "seven members", chart: testTypeChart(), members: seven, threats: one, wantErr: domain.ErrMemberCount},
		{name: "no threats", chart: testTypeChart(), members: one, threats: nil, wantErr: domain.ErrThreatCount},
		{name: "empty threats", chart: testTypeChart(), members: one, threats: []domain.Combatant{}, wantErr: domain.ErrThreatCount},
		{name: "seven threats", chart: testTypeChart(), members: one, threats: seven, wantErr: domain.ErrThreatCount},
		{name: "nil chart", chart: nil, members: one, threats: one, wantErr: domain.ErrNilTypeChart},
		{name: "chart error is propagated", chart: errorTypeChart{err: chartErr}, members: one, threats: one, wantErr: chartErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := domain.AnalyzeThreats(tt.chart, tt.members, tt.threats); !errors.Is(err, tt.wantErr) {
				t.Fatalf("AnalyzeThreats() error = %v, want errors.Is(_, %v)", err, tt.wantErr)
			}
		})
	}
}

func TestAnalyzeThreatsRejectsInvalidChartMultiplier(t *testing.T) {
	t.Parallel()

	one := []domain.Combatant{combatant("9002-000", types(domain.TypeGrass), nil, attack("move-9001", domain.TypeFire))}
	if _, err := domain.AnalyzeThreats(errorTypeChart{multiplier: 3}, one, one); err == nil {
		t.Fatal("AnalyzeThreats() with an impossible single-type multiplier: error = nil, want an error")
	}
}

// stackedSuperEffectiveAbility multiplies a super effective hit by 16 sixteen times (2 x 16^16
// does not fit in int64). ADR-0017 §5.6 / ADR-0400 §4: never wrapped, always an error.
func stackedSuperEffectiveAbility() *domain.Ability {
	effects := make([]domain.AbilityEffect, 16)
	for i := range effects {
		effects[i] = superEffectiveFactorEffect(16, 1)
	}
	return threatAbility("ability-9101", effects...)
}

func TestAnalyzeThreatsMultiplierOverflow(t *testing.T) {
	t.Parallel()

	fireAttacker := func(id string) domain.Combatant {
		return combatant(id, types(domain.TypeFire), nil, attack("move-9001", domain.TypeFire))
	}
	overflowing := combatant("9002-000", types(domain.TypeGrass), stackedSuperEffectiveAbility())
	tests := []struct {
		name    string
		members []domain.Combatant
		threats []domain.Combatant
	}{
		{name: "incoming (member ability)", members: []domain.Combatant{overflowing}, threats: []domain.Combatant{fireAttacker("9101-000")}},
		{name: "outgoing (threat ability)", members: []domain.Combatant{fireAttacker("9101-000")}, threats: []domain.Combatant{overflowing}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := domain.AnalyzeThreats(testTypeChart(), tt.members, tt.threats); !errors.Is(err, domain.ErrEffectivenessOverflow) {
				t.Fatalf("AnalyzeThreats() error = %v, want ErrEffectivenessOverflow", err)
			}
		})
	}
}

// ADR-0400 §6.1: the core validates every combatant's moves exactly like AnalyzeCoverage
// (move count, duplicate moveId, move category, attack move type). A provider that returns
// an invalid classification is an error, not silently excluded (ADR-0016 §6).
func TestAnalyzeThreatsRejectsInvalidMoves(t *testing.T) {
	t.Parallel()

	valid := combatant("9002-000", types(domain.TypeGrass), nil, attack("move-9001", domain.TypeFire))
	fiveMoves := combatant("9002-000", types(domain.TypeGrass), nil,
		attack("move-9001", domain.TypeFire), attack("move-9003", domain.TypeWater), attack("move-9004", domain.TypeElectric),
		physical("move-9005", domain.TypeNormal), status("move-9006", domain.TypeGrass))
	duplicateMoves := combatant("9002-000", types(domain.TypeGrass), nil, attack("move-9001", domain.TypeFire), attack("move-9001", domain.TypeFire))
	invalidCategory := combatant("9002-000", types(domain.TypeGrass), nil, domain.Move{MoveID: "move-9001", Type: domain.TypeFire, Category: "other"})
	invalidAttackType := combatant("9002-000", types(domain.TypeGrass), nil, attack("move-9001", "stellar"))

	tests := []struct {
		name    string
		members []domain.Combatant
		threats []domain.Combatant
		wantErr error
	}{
		{name: "five moves on a member", members: []domain.Combatant{fiveMoves}, threats: []domain.Combatant{valid}, wantErr: domain.ErrMoveCount},
		{name: "five moves on a threat", members: []domain.Combatant{valid}, threats: []domain.Combatant{fiveMoves}, wantErr: domain.ErrMoveCount},
		{name: "duplicate moveId on a member", members: []domain.Combatant{duplicateMoves}, threats: []domain.Combatant{valid}, wantErr: domain.ErrDuplicateMove},
		{name: "duplicate moveId on a threat", members: []domain.Combatant{valid}, threats: []domain.Combatant{duplicateMoves}, wantErr: domain.ErrDuplicateMove},
		{name: "invalid move category on a member", members: []domain.Combatant{invalidCategory}, threats: []domain.Combatant{valid}, wantErr: domain.ErrInvalidMoveCategory},
		{name: "invalid move category on a threat", members: []domain.Combatant{valid}, threats: []domain.Combatant{invalidCategory}, wantErr: domain.ErrInvalidMoveCategory},
		{name: "invalid attack move type on a member", members: []domain.Combatant{invalidAttackType}, threats: []domain.Combatant{valid}, wantErr: domain.ErrInvalidType},
		{name: "invalid attack move type on a threat", members: []domain.Combatant{valid}, threats: []domain.Combatant{invalidAttackType}, wantErr: domain.ErrInvalidType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := domain.AnalyzeThreats(testTypeChart(), tt.members, tt.threats); !errors.Is(err, tt.wantErr) {
				t.Fatalf("AnalyzeThreats() error = %v, want errors.Is(_, %v)", err, tt.wantErr)
			}
		})
	}
}

// ADR-0400 §6.2: a nil chart is rejected whatever the presence of attack moves on either
// side (same as AnalyzeCoverage, ADR-0016 §6.3).
func TestAnalyzeThreatsRejectsNilChartRegardlessOfAttackMoves(t *testing.T) {
	t.Parallel()

	noMoves := combatant("9002-000", types(domain.TypeGrass), nil)
	statusOnly := combatant("9002-000", types(domain.TypeGrass), nil, status("move-9006", domain.TypeGrass))

	tests := []struct {
		name    string
		members []domain.Combatant
		threats []domain.Combatant
	}{
		{name: "no attack moves on either side", members: []domain.Combatant{noMoves}, threats: []domain.Combatant{noMoves}},
		{name: "status move only on either side", members: []domain.Combatant{statusOnly}, threats: []domain.Combatant{statusOnly}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := domain.AnalyzeThreats(nil, tt.members, tt.threats); !errors.Is(err, domain.ErrNilTypeChart) {
				t.Fatalf("AnalyzeThreats(nil chart) error = %v, want ErrNilTypeChart", err)
			}
		})
	}
}

// ADR-0400 §6.3: an invalid ability effect is an error even for the side whose ability is
// never exercised through a matchup in this pairing (its owner has no attack move, and an
// ability never changes its own side's attacks either, so CalculateDefenseWithAbility is
// never called with it here).
func TestAnalyzeThreatsValidatesAbilityEffectsRegardlessOfAttackMoves(t *testing.T) {
	t.Parallel()

	invalidAbility := threatAbility("ability-9101", domain.AbilityEffect{Kind: "heal", AttackType: domain.TypeFire})
	noMoves := func(id string, ability *domain.Ability) domain.Combatant {
		return combatant(id, types(domain.TypeGrass), ability)
	}
	plain := noMoves("9002-000", nil)

	tests := []struct {
		name    string
		members []domain.Combatant
		threats []domain.Combatant
	}{
		{
			name:    "member ability invalid, threat has no attack move",
			members: []domain.Combatant{noMoves("9002-000", invalidAbility)},
			threats: []domain.Combatant{plain},
		},
		{
			name:    "threat ability invalid, member has no attack move",
			members: []domain.Combatant{plain},
			threats: []domain.Combatant{noMoves("9101-000", invalidAbility)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := domain.AnalyzeThreats(testTypeChart(), tt.members, tt.threats); !errors.Is(err, domain.ErrInvalidAbilityEffect) {
				t.Fatalf("AnalyzeThreats() error = %v, want ErrInvalidAbilityEffect", err)
			}
		})
	}
}

// ADR-0400 §6.7: every combatant's types are validated even when the other side has no attack move.
func TestAnalyzeThreatsValidatesTypesRegardlessOfAttackMoves(t *testing.T) {
	t.Parallel()

	noMoves := combatant("9002-000", types(domain.TypeGrass), nil)
	tests := []struct {
		name string
		bad  domain.Combatant
		want error
	}{
		{name: "unknown type", bad: combatant("9101-000", types("stellar"), nil), want: domain.ErrInvalidType},
		{name: "three types", bad: combatant("9101-000", types(domain.TypeFire, domain.TypeWater, domain.TypeGrass), nil), want: domain.ErrDefenseTypeCount},
		{name: "no type", bad: combatant("9101-000", types(), nil), want: domain.ErrDefenseTypeCount},
		{name: "duplicate type", bad: combatant("9101-000", types(domain.TypeFire, domain.TypeFire), nil), want: domain.ErrDuplicateDefenseType},
	}
	for _, tt := range tests {
		t.Run(tt.name+" as member", func(t *testing.T) {
			t.Parallel()
			_, err := domain.AnalyzeThreats(testTypeChart(), []domain.Combatant{tt.bad}, []domain.Combatant{noMoves})
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
		t.Run(tt.name+" as threat", func(t *testing.T) {
			t.Parallel()
			_, err := domain.AnalyzeThreats(testTypeChart(), []domain.Combatant{noMoves}, []domain.Combatant{tt.bad})
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}
