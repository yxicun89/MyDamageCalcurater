package engine

// issue #272 / ADR-0126: 一括計算の防御側・逆算の相手側に特性の候補を渡す。
//
// 受け入れ条件(ADR-0126 §決定):
//   - BulkInput.DefenderAbilities / ReverseInput.UnknownAbilities が空なら従来どおり(特性なし・行も候補も今までと同じ)
//   - 渡した特性は各行・各候補の計算に使われ、同じ入力の CalcDamage と完全に一致する
//   - 結果が完全に同じになる特性は1つにまとめ(代表=先に渡したもの)、まとめた ID を AbilityIDs に出す
//   - 結果が違う特性は行・候補を分ける(並びは プリセット → 特性 → 持ち物 / 性格クラス → 特性 → 持ち物)
//   - 件数上限・重複・空 ID・種族が持たない特性は ErrInvalidAbilityCandidates

import (
	"errors"
	"reflect"
	"testing"
)

// 効果だけを書く(特性名はコードに書かない)。
func candGroundImmune() Ability {
	return Ability{ID: "cand-ground-immune", NameJa: "テストとくせいA", Effect: &AbilityEffect{DefImmuneTypes: []Type{TypeGround}, Airborne: true}}
}

func candFireResist() Ability {
	return Ability{ID: "cand-fire-resist", NameJa: "テストとくせいB", Effect: &AbilityEffect{DefResistType: map[Type]int{TypeFire: 2048}}}
}

func candNoEffect(id string) Ability {
	return Ability{ID: id, NameJa: "テストとくせいなし"}
}

func abilityIDsOf(as ...Ability) []string {
	out := make([]string, 0, len(as))
	for _, a := range as {
		out = append(out, a.ID)
	}
	return out
}

// issue #272 の再現手順1: 防御側の種族の特性が「地面無効」1つだけのとき、一括計算の全行が 0・immune になり、
// 1対1の CalcDamage と食い違わない。
func TestCalcBulkDefenderAbilityImmunity(t *testing.T) {
	in := bulkInput(CategoryPhysical, TypeGround)
	immune := candGroundImmune()
	in.DefenderSpecies.Abilities = []string{immune.ID}
	in.DefenderAbilities = []Ability{immune}
	res, err := calcBulk(in)
	if err != nil {
		t.Fatalf("CalcBulk: %v", err)
	}
	presets := DefaultDefenderPresets(CategoryPhysical)
	if len(res.Rows) != len(presets) {
		t.Fatalf("行数=%d want %d", len(res.Rows), len(presets))
	}
	for i, row := range res.Rows {
		if row.Result.Nullified != NullifyImmune {
			t.Errorf("rows[%d].Nullified=%q want immune", i, row.Result.Nullified)
		}
		if row.Result.Rolls != ([16]int{}) {
			t.Errorf("rows[%d].Rolls=%v want 全0", i, row.Result.Rolls)
		}
		if row.Ability.ID != immune.ID || !reflect.DeepEqual(row.AbilityIDs, []string{immune.ID}) {
			t.Errorf("rows[%d] Ability=%q AbilityIDs=%v want %q", i, row.Ability.ID, row.AbilityIDs, immune.ID)
		}
		def := wantDefender(presets[i], in.DefenderSpecies, nil)
		def.Ability = immune
		if !reflect.DeepEqual(row.Defender, def) {
			t.Errorf("rows[%d].Defender=%+v want %+v", i, row.Defender, def)
		}
		if want := wantResult(t, in, def); !reflect.DeepEqual(row.Result, want) {
			t.Errorf("rows[%d].Result=%+v want %+v(CalcDamage と不一致)", i, row.Result, want)
		}
	}
}

// 特性を渡さないときは従来どおり(Ability はゼロ値、AbilityIDs は nil)。
func TestCalcBulkNoDefenderAbilitiesKeepsLegacyRows(t *testing.T) {
	in := bulkInput(CategoryPhysical, TypeGround)
	res := checkedBulk(t, in)
	for i, row := range res.Rows {
		if row.Ability != (Ability{}) || row.AbilityIDs != nil {
			t.Errorf("rows[%d] Ability=%+v AbilityIDs=%v want ゼロ値・nil", i, row.Ability, row.AbilityIDs)
		}
	}
	empty := in
	empty.DefenderAbilities = []Ability{}
	got, err := calcBulk(empty)
	if err != nil {
		t.Fatalf("CalcBulk: %v", err)
	}
	if !reflect.DeepEqual(got, res) {
		t.Errorf("空スライスと nil で結果が違う")
	}
}

// 結果が違う特性は行を分け、同じになる特性は1行にまとめる。並びはプリセット → 特性 → 持ち物。
func TestCalcBulkSplitsAndMergesAbilities(t *testing.T) {
	immune, a2, a3 := candGroundImmune(), candNoEffect("cand-plain-1"), candNoEffect("cand-plain-2")
	band := testChoiceBand()
	cases := []struct {
		name     string
		moveType Type
		want     [][]string // プリセット1つぶんの特性グループ(代表が先頭)
	}{
		{"地面技: 無効の特性だけ分かれる", TypeGround, [][]string{{immune.ID}, {a2.ID, a3.ID}}},
		{"水技: どの特性でも同じなので1つにまとまる", TypeWater, [][]string{{immune.ID, a2.ID, a3.ID}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := bulkInput(CategoryPhysical, tc.moveType)
			in.DefenderSpecies.Abilities = abilityIDsOf(immune, a2, a3)
			in.DefenderAbilities = []Ability{immune, a2, a3}
			in.ItemVariants = []*Item{nil, band}
			res, err := calcBulk(in)
			if err != nil {
				t.Fatalf("CalcBulk: %v", err)
			}
			byID := map[string]Ability{immune.ID: immune, a2.ID: a2, a3.ID: a3}
			presets := DefaultDefenderPresets(CategoryPhysical)
			if want := len(presets) * len(tc.want) * 2; len(res.Rows) != want {
				t.Fatalf("行数=%d want %d", len(res.Rows), want)
			}
			i := 0
			for _, p := range presets {
				for _, group := range tc.want {
					for _, item := range in.ItemVariants {
						row := res.Rows[i]
						if row.Preset != p.Key || row.Item != item || row.Ability.ID != group[0] || !reflect.DeepEqual(row.AbilityIDs, group) {
							t.Errorf("rows[%d]=%s/%s/%v item=%p want %s/%s/%v item=%p", i, row.Preset, row.Ability.ID, row.AbilityIDs, row.Item, p.Key, group[0], group, item)
						}
						for _, id := range group {
							def := wantDefender(p, in.DefenderSpecies, item)
							def.Ability = byID[id]
							if want := wantResult(t, in, def); !reflect.DeepEqual(row.Result, want) {
								t.Errorf("rows[%d] の結果が特性 %s の CalcDamage と違う", i, id)
							}
						}
						i++
					}
				}
			}
		})
	}
}

func TestCalcBulkAbilityCandidateErrors(t *testing.T) {
	immune := candGroundImmune()
	cases := []struct {
		name      string
		species   []string
		abilities []Ability
	}{
		{"上限超過", nil, []Ability{candNoEffect("x1"), candNoEffect("x2"), candNoEffect("x3"), candNoEffect("x4")}},
		{"ID の重複", nil, []Ability{immune, immune}},
		{"空の ID", nil, []Ability{{Effect: immune.Effect}}},
		{"種族が持たない特性", []string{"other"}, []Ability{immune}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := bulkInput(CategoryPhysical, TypeGround)
			in.DefenderSpecies.Abilities = tc.species
			in.DefenderAbilities = tc.abilities
			if _, err := calcBulk(in); !errors.Is(err, ErrInvalidAbilityCandidates) {
				t.Fatalf("err=%v want ErrInvalidAbilityCandidates", err)
			}
		})
	}
	// 上限ちょうどは通る。種族の特性一覧が空(不明)なら所属は検査しない。
	in := bulkInput(CategoryPhysical, TypeGround)
	in.DefenderAbilities = []Ability{immune, candNoEffect("x2"), candNoEffect("x3")}
	if _, err := calcBulk(in); err != nil {
		t.Fatalf("上限ちょうど: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 逆算
// ---------------------------------------------------------------------------

// 相手(防御側)の特性で結果が変わるとき、候補を特性で分け、正しい特性の候補だけが完全一致する。
func TestCalcReverseSplitsUnknownAbilities(t *testing.T) {
	resist, plain := candFireResist(), candNoEffect("cand-plain")
	move := Move{ID: "testfire", NameJa: "テストほのお", Type: TypeFire, Category: CategorySpecial, Power: 100}
	truth := revDefender(Stats{HP: MaxSPPerStat, SpD: 20}, NatureNeutral, nil)
	truth.Ability = resist
	obs, err := calcDamage(DamageInput{Format: FormatSingle, Attacker: revStrongSpecialAttacker(), Defender: truth, Move: move})
	if err != nil {
		t.Fatal(err)
	}
	species := revDefenderSpecies()
	species.Abilities = abilityIDsOf(resist, plain)
	in := ReverseInput{
		Format: FormatSingle, Side: SideDefender, Known: revStrongSpecialAttacker(), UnknownSpecies: species,
		Move: move, UnknownAbilities: []Ability{resist, plain},
		Observations: []Observation{{Damage: obs.Rolls[0]}, {Damage: obs.Rolls[15]}},
	}
	res, err := calcReverse(in)
	if err != nil {
		t.Fatalf("CalcReverse: %v", err)
	}
	if len(res.Candidates) != len(reverseClasses)*2 {
		t.Fatalf("候補数=%d want %d(性格クラス × 特性2)", len(res.Candidates), len(reverseClasses)*2)
	}
	top := res.Candidates[0]
	if !top.Exact || top.Ability.ID != resist.ID || !reflect.DeepEqual(top.AbilityIDs, []string{resist.ID}) || top.NatureClass != NatureClassNeutral {
		t.Fatalf("先頭候補=%+v want 軽減特性・無補正の完全一致", top)
	}
	if !spInRanges(20, top.Ranges) {
		t.Errorf("真の SP 20 が先頭候補の範囲 %v に無い", top.Ranges)
	}
	for _, c := range res.Candidates {
		if c.Ability.ID == plain.ID && c.NatureClass == NatureClassNeutral && c.Exact && spInRanges(20, c.Ranges) {
			t.Errorf("特性なしの候補が真の SP を説明してしまう(特性が効いていない): %+v", c)
		}
	}
}

// 特性で結果が変わらなければ1つにまとめ、特性を渡さないときと同じ候補になる(AbilityIDs 以外)。
func TestCalcReverseMergesAbilitiesWithSameResults(t *testing.T) {
	resist, plain := candFireResist(), candNoEffect("cand-plain")
	base := ReverseInput{
		Format: FormatSingle, Side: SideDefender, Known: revStrongAttacker(), UnknownSpecies: revDefenderSpecies(),
		Move: revMove(CategoryPhysical), Observations: []Observation{{Percent: 40}},
	}
	legacy, err := calcReverse(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range legacy.Candidates {
		if c.Ability != (Ability{}) || c.AbilityIDs != nil {
			t.Fatalf("特性なしの候補に特性が付いた: %+v", c)
		}
	}
	in := base
	in.UnknownAbilities = []Ability{plain, resist}
	got, err := calcReverse(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) != len(legacy.Candidates) {
		t.Fatalf("候補数=%d want %d(水技には炎の軽減が効かないので1つにまとまる)", len(got.Candidates), len(legacy.Candidates))
	}
	for i, c := range got.Candidates {
		if c.Ability.ID != plain.ID || !reflect.DeepEqual(c.AbilityIDs, []string{plain.ID, resist.ID}) {
			t.Errorf("候補[%d] Ability=%q AbilityIDs=%v", i, c.Ability.ID, c.AbilityIDs)
		}
		c.Ability, c.AbilityIDs = Ability{}, nil
		if !reflect.DeepEqual(c, legacy.Candidates[i]) {
			t.Errorf("候補[%d]=%+v want %+v", i, c, legacy.Candidates[i])
		}
	}
}

// 受けたダメージから相手(攻撃側)を逆算するときも、相手の特性(攻撃側の補正)が効く。
func TestCalcReverseAttackerSideUsesUnknownAbility(t *testing.T) {
	boost := Ability{ID: "cand-water-boost", NameJa: "テストとくせいC", Effect: &AbilityEffect{OffBoostType: TypeWater, OffBoostTypeMod: 6144}}
	plain := candNoEffect("cand-plain")
	truth := Individual{Species: revAttackerSpecies(), Level: DefaultLevel, SP: Stats{Atk: 12}, Ability: boost, Status: StatusNone}
	known := revDefender(Stats{HP: MaxSPPerStat}, NatureNeutral, nil)
	obs, err := calcDamage(DamageInput{Format: FormatSingle, Attacker: truth, Defender: known, Move: revMove(CategoryPhysical)})
	if err != nil {
		t.Fatal(err)
	}
	res, err := calcReverse(ReverseInput{
		Format: FormatSingle, Side: SideAttacker, Known: known, UnknownSpecies: revAttackerSpecies(),
		Move: revMove(CategoryPhysical), UnknownAbilities: []Ability{plain, boost},
		Observations: []Observation{{Damage: obs.Rolls[0]}, {Damage: obs.Rolls[15]}},
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range res.Candidates {
		if c.Ability.ID == boost.ID && c.NatureClass == NatureClassNeutral {
			found = true
			if !c.Exact || !spInRanges(12, c.Ranges) {
				t.Errorf("強化特性・無補正の候補が真の SP 12 を説明しない: %+v", c)
			}
		}
	}
	if !found {
		t.Fatal("強化特性の候補が無い(特性で候補が分かれていない)")
	}
}

func TestCalcReverseAbilityCandidateErrors(t *testing.T) {
	cases := []struct {
		name      string
		species   []string
		abilities []Ability
	}{
		{"上限超過", nil, []Ability{candNoEffect("x1"), candNoEffect("x2"), candNoEffect("x3"), candNoEffect("x4")}},
		{"ID の重複", nil, []Ability{candNoEffect("x1"), candNoEffect("x1")}},
		{"空の ID", nil, []Ability{{}}},
		{"種族が持たない特性", []string{"other"}, []Ability{candFireResist()}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			species := revDefenderSpecies()
			species.Abilities = tc.species
			_, err := calcReverse(ReverseInput{
				Format: FormatSingle, Side: SideDefender, Known: revStrongAttacker(), UnknownSpecies: species,
				Move: revMove(CategoryPhysical), UnknownAbilities: tc.abilities, Observations: []Observation{{Percent: 40}},
			})
			if !errors.Is(err, ErrInvalidAbilityCandidates) {
				t.Fatalf("err=%v want ErrInvalidAbilityCandidates", err)
			}
		})
	}
}

// 全特性でダメージが 0(無効)なら従来どおり ErrMoveDealsNoDamage。1つでも出れば候補を返す。
func TestCalcReverseAbilityImmunityNoDamage(t *testing.T) {
	ground := Move{ID: "testground", NameJa: "テストじめん", Type: TypeGround, Category: CategoryPhysical, Power: 100}
	in := ReverseInput{
		Format: FormatSingle, Side: SideDefender, Known: revStrongAttacker(), UnknownSpecies: revDefenderSpecies(),
		Move: ground, UnknownAbilities: []Ability{candGroundImmune()}, Observations: []Observation{{Percent: 40}},
	}
	if _, err := calcReverse(in); !errors.Is(err, ErrMoveDealsNoDamage) {
		t.Fatalf("err=%v want ErrMoveDealsNoDamage", err)
	}
	in.UnknownAbilities = []Ability{candGroundImmune(), candNoEffect("cand-plain")}
	res, err := calcReverse(in)
	if err != nil {
		t.Fatalf("CalcReverse: %v", err)
	}
	if res.Candidates[0].Ability.ID != "cand-plain" {
		t.Errorf("先頭候補の特性=%q want cand-plain(無効の特性ではダメージが出ない)", res.Candidates[0].Ability.ID)
	}
}

// revStrongSpecialAttacker は C32・C上昇の既知側攻撃個体。
func revStrongSpecialAttacker() Individual {
	a := revStrongAttacker()
	a.Nature = Nature{Plus: StatSpA, Minus: StatAtk}
	a.SP = Stats{SpA: MaxSPPerStat}
	return a
}

func spInRanges(sp int, rs []SPRange) bool {
	for _, r := range rs {
		if r.Min <= sp && sp <= r.Max {
			return true
		}
	}
	return false
}
