package engine

// issue #274/#272 の残り / ADR-0216: 一括計算の防御側に、ランク・状態異常を全行一律で上書きする。
//
// 受け入れ条件(ADR-0216 §決定):
//   - BulkInput.DefenderOverride のゼロ値は従来どおり(Ranks=0・Status=none。行・結果とも変わらない)
//   - 上書きはプリセット解決の後・ダメージ計算の前に、全行(全プリセット × 全持ち物 × 全特性)の防御側に当てる。
//     プリセットが決める SP・性格・持ち物・特性は変えない。プリセットの Ranks=0・Status=none より上書きが勝つ
//   - 各行は上書き後の防御側を使った CalcDamage と完全に一致する(独自計算をしない)
//   - 防御側の防御/特防ランクは使う側だけが効く。急所は防御側の正のランクを無視する
//   - 防御側の状態異常は今の式ではダメージを変えない(やけどの半減は攻撃側だけ。ADR-0216 §3)。行の Defender には載る
//   - ランク -6..+6 の外・未知の状態異常は ErrInvalidDefenderOverride(部分的な行を返さない)

import (
	"errors"
	"reflect"
	"testing"
)

// overrideBulk は上書きつきの一括計算を行い、各行が「上書き後の防御側」の CalcDamage と一致することを確かめる。
// 行の並びと SP・性格・持ち物は上書きなしと同じであること(プリセットの値に触れない)も見る。
func overrideBulk(t *testing.T, in BulkInput) BulkResult {
	t.Helper()
	presets := DefaultDefenderPresets(in.Move.Category)
	items := in.ItemVariants
	if len(items) == 0 {
		items = []*Item{nil}
	}
	res, err := calcBulk(in)
	if err != nil {
		t.Fatalf("CalcBulk: %v", err)
	}
	if len(res.Rows) != len(presets)*len(items) {
		t.Fatalf("行数=%d want %d(上書きで行数は変わらない)", len(res.Rows), len(presets)*len(items))
	}
	wantStatus := in.DefenderOverride.Status
	if wantStatus == "" {
		wantStatus = StatusNone
	}
	i := 0
	for _, p := range presets {
		for _, item := range items {
			row := res.Rows[i]
			def := wantDefender(p, in.DefenderSpecies, item)
			def.Ranks = in.DefenderOverride.Ranks
			def.Status = wantStatus
			if !reflect.DeepEqual(row.Defender, def) {
				t.Errorf("rows[%d](%s).Defender=%+v want %+v", i, p.Key, row.Defender, def)
			}
			if want := wantResult(t, in, def); !reflect.DeepEqual(row.Result, want) {
				t.Errorf("rows[%d](%s).Result=%+v want %+v(上書き後の CalcDamage と不一致)", i, p.Key, row.Result, want)
			}
			i++
		}
	}
	return res
}

// ゼロ値・明示の none・ランク0 は従来の行と完全に同じ(既定はバイト同一の前提)。
func TestCalcBulkDefenderOverrideZeroValueKeepsLegacyRows(t *testing.T) {
	for _, cat := range []MoveCategory{CategoryPhysical, CategorySpecial} {
		legacy := checkedBulk(t, bulkInput(cat, TypeWater))
		for _, ov := range []DefenderOverride{{}, {Status: StatusNone}, {Ranks: Ranks{}, Status: StatusNone}} {
			in := bulkInput(cat, TypeWater)
			in.DefenderOverride = ov
			got, err := calcBulk(in)
			if err != nil {
				t.Fatalf("%s %+v: CalcBulk: %v", cat, ov, err)
			}
			if !reflect.DeepEqual(got, legacy) {
				t.Errorf("%s %+v: 上書きのゼロ値で結果が従来と違う", cat, ov)
			}
		}
	}
}

// 上書きは全行(全プリセット × 全持ち物)に一律で当たり、SP・性格・持ち物はプリセットのまま。
func TestCalcBulkDefenderOverrideAppliesToEveryRow(t *testing.T) {
	for _, cat := range []MoveCategory{CategoryPhysical, CategorySpecial} {
		in := bulkInput(cat, TypeWater)
		in.ItemVariants = []*Item{nil, testChoiceBand()}
		in.DefenderOverride = DefenderOverride{
			Ranks:  Ranks{Atk: 1, Def: 2, SpA: -3, SpD: -1, Spe: 6},
			Status: StatusParalysis,
		}
		overrideBulk(t, in)
	}
}

// 特性の候補(ADR-0126)と併用しても、全特性グループの行に同じ上書きが当たる。
func TestCalcBulkDefenderOverrideWithAbilityCandidates(t *testing.T) {
	immune, plain := candGroundImmune(), candNoEffect("cand-plain-1")
	in := bulkInput(CategoryPhysical, TypeGround)
	in.DefenderSpecies.Abilities = abilityIDsOf(immune, plain)
	in.DefenderAbilities = []Ability{immune, plain}
	in.DefenderOverride = DefenderOverride{Ranks: Ranks{Def: -2}, Status: StatusSleep}
	res, err := calcBulk(in)
	if err != nil {
		t.Fatalf("CalcBulk: %v", err)
	}
	presets := DefaultDefenderPresets(CategoryPhysical)
	if len(res.Rows) != 2*len(presets) {
		t.Fatalf("行数=%d want %d", len(res.Rows), 2*len(presets))
	}
	for i, row := range res.Rows {
		if row.Defender.Ranks != (Ranks{Def: -2}) || row.Defender.Status != StatusSleep {
			t.Errorf("rows[%d](%s/%s) Ranks=%+v Status=%q want Def-2・sleep", i, row.Preset, row.Ability.ID, row.Defender.Ranks, row.Defender.Status)
		}
		if row.Defender.Ability.ID != row.Ability.ID {
			t.Errorf("rows[%d] 上書きで特性が変わった: %q want %q", i, row.Defender.Ability.ID, row.Ability.ID)
		}
		if want := wantResult(t, in, row.Defender); !reflect.DeepEqual(row.Result, want) {
			t.Errorf("rows[%d] Result が上書き後の CalcDamage と不一致", i)
		}
	}
}

// 呼び出し側が渡したカスタムプリセット(PresetKeys で選別)にも同じ上書きが当たる。
func TestCalcBulkDefenderOverrideAppliesToCustomPresets(t *testing.T) {
	in := bulkInput(CategoryPhysical, TypeWater)
	in.Presets = []DefenderPreset{
		{Key: "c1", SP: Stats{HP: 10, Def: 20}, Nature: Nature{Plus: StatDef, Minus: StatAtk}},
		{Key: "c2", SP: Stats{HP: 32}},
	}
	in.PresetKeys = []PresetKey{"c2", "c1"}
	in.DefenderOverride = DefenderOverride{Ranks: Ranks{Def: 3}}
	res, err := calcBulk(in)
	if err != nil {
		t.Fatalf("CalcBulk: %v", err)
	}
	if got := rowKeys(res.Rows); !reflect.DeepEqual(got, []PresetKey{"c2", "c1"}) {
		t.Fatalf("行の順序=%v", got)
	}
	for i, row := range res.Rows {
		p := in.Presets[1-i]
		def := wantDefender(p, in.DefenderSpecies, nil)
		def.Ranks = Ranks{Def: 3}
		if !reflect.DeepEqual(row.Defender, def) {
			t.Errorf("rows[%d].Defender=%+v want %+v", i, row.Defender, def)
		}
		if want := wantResult(t, in, def); !reflect.DeepEqual(row.Result, want) {
			t.Errorf("rows[%d] Result が上書き後の CalcDamage と不一致", i)
		}
	}
}

// 防御側のランクは、技の分類が使う側(物理=防御・特殊=特防)だけが効き、境界 ±6 まで単調に効く。
func TestCalcBulkDefenderRankDirection(t *testing.T) {
	tests := []struct {
		name     string
		category MoveCategory
		ranks    Ranks
		dir      int
	}{
		{"防御+2×物理", CategoryPhysical, Ranks{Def: 2}, -1},
		{"防御+6×物理(境界)", CategoryPhysical, Ranks{Def: 6}, -1},
		{"防御-2×物理", CategoryPhysical, Ranks{Def: -2}, +1},
		{"防御-6×物理(境界)", CategoryPhysical, Ranks{Def: -6}, +1},
		{"特防+2×特殊", CategorySpecial, Ranks{SpD: 2}, -1},
		{"特防-6×特殊(境界)", CategorySpecial, Ranks{SpD: -6}, +1},
		{"特防+6×物理は無関係", CategoryPhysical, Ranks{SpD: 6}, 0},
		{"防御-6×特殊は無関係", CategorySpecial, Ranks{Def: -6}, 0},
		{"防御側の攻撃・特攻・素早さは無関係", CategoryPhysical, Ranks{Atk: 6, SpA: -6, Spe: 6}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := checkedBulk(t, bulkInput(tt.category, TypeWater))
			in := bulkInput(tt.category, TypeWater)
			in.DefenderOverride = DefenderOverride{Ranks: tt.ranks}
			got := overrideBulk(t, in)
			maxDamageCompare(t, tt.name, got, base, tt.dir)
		})
	}
}

// 急所は防御側の正のランクを無視し、負のランクは効く(上書きでも CalcDamage と同じ規則)。
func TestCalcBulkDefenderOverrideCriticalInteraction(t *testing.T) {
	critBase := bulkInput(CategoryPhysical, TypeWater)
	critBase.Critical = true
	base := checkedBulk(t, critBase)

	plus := critBase
	plus.DefenderOverride = DefenderOverride{Ranks: Ranks{Def: 4}}
	maxDamageCompare(t, "急所×防御+4", overrideBulk(t, plus), base, 0)

	minus := critBase
	minus.DefenderOverride = DefenderOverride{Ranks: Ranks{Def: -2}}
	maxDamageCompare(t, "急所×防御-2", overrideBulk(t, minus), base, +1)
}

// 防御側の状態異常は、今の式ではダメージを変えない(やけどの半減は攻撃側の状態だけを見る。ADR-0216 §3)。
// 行の Defender には指定どおり載る(結果の再現・後の拡張のため)。式が防御側の状態を見るようになったら
// このテストは意図して更新する(その ADR で理由を書く)。
func TestCalcBulkDefenderStatusDoesNotChangeDamage(t *testing.T) {
	statuses := []Status{StatusBurn, StatusParalysis, StatusPoison, StatusBadlyPoison, StatusSleep, StatusFreeze}
	for _, cat := range []MoveCategory{CategoryPhysical, CategorySpecial} {
		legacy := checkedBulk(t, bulkInput(cat, TypeWater))
		for _, st := range statuses {
			in := bulkInput(cat, TypeWater)
			in.DefenderOverride = DefenderOverride{Status: st}
			got := overrideBulk(t, in)
			for i := range got.Rows {
				if got.Rows[i].Defender.Status != st {
					t.Errorf("%s/%s rows[%d].Defender.Status=%q", cat, st, i, got.Rows[i].Defender.Status)
				}
				if !reflect.DeepEqual(got.Rows[i].Result, legacy.Rows[i].Result) {
					t.Errorf("%s/%s rows[%d] 防御側の状態異常で結果が変わった(今の式は防御側の状態を見ない)", cat, st, i)
				}
			}
		}
	}
}

// 防御ランクを無視する機構の技は、上書きで防御ランクが付いても印を付けず、ランクを無視した結果になる
// (ADR-0142 §5・§10。ADR-0123 の「付いた行にだけ印」は段階1で engine が計算するようになり廃止)。
func TestCalcBulkDefenderOverrideMarksIgnoreDefenseRanks(t *testing.T) {
	in := bulkInput(CategoryPhysical, TypeWater)
	in.Move.Mechanisms = []MoveMechanism{MechanismIgnoreDefenseRanks}
	plain := checkedBulk(t, in)
	for _, row := range plain.Rows {
		if len(row.Result.Unsupported) != 0 {
			t.Fatalf("上書きなしで印が付いた: %+v", row.Result.Unsupported)
		}
	}
	in.DefenderOverride = DefenderOverride{Ranks: Ranks{Def: 1}}
	for i, row := range overrideBulk(t, in).Rows {
		if len(row.Result.Unsupported) != 0 {
			t.Errorf("rows[%d] 防御+1 の上書きで印が付いた: %+v", i, row.Result.Unsupported)
		}
		if row.Result.Rolls != plain.Rows[i].Result.Rolls {
			t.Errorf("rows[%d] 防御ランクを無視していない: %v, want %v", i, row.Result.Rolls, plain.Rows[i].Result.Rolls)
		}
	}
}

// 上書きを渡しても入力(プリセット・持ち物のスライス)は書き換えない。
func TestCalcBulkDefenderOverrideDoesNotMutateInput(t *testing.T) {
	in := bulkInput(CategoryPhysical, TypeWater)
	in.Presets = DefenderPresetCatalog()
	in.ItemVariants = []*Item{nil, testChoiceBand()}
	in.DefenderOverride = DefenderOverride{Ranks: Ranks{Def: 1}, Status: StatusBurn}
	before := DefenderPresetCatalog()
	if _, err := calcBulk(in); err != nil {
		t.Fatalf("CalcBulk: %v", err)
	}
	if !reflect.DeepEqual(in.Presets, before) {
		t.Errorf("Presets が書き換えられた")
	}
	if in.DefenderOverride != (DefenderOverride{Ranks: Ranks{Def: 1}, Status: StatusBurn}) {
		t.Errorf("DefenderOverride が書き換えられた: %+v", in.DefenderOverride)
	}
}

// ランク -6..+6 の外・未知の状態異常は ErrInvalidDefenderOverride。部分的な行を返さない。
func TestCalcBulkDefenderOverrideErrors(t *testing.T) {
	tests := []struct {
		name string
		ov   DefenderOverride
	}{
		{"防御+7", DefenderOverride{Ranks: Ranks{Def: 7}}},
		{"特防-7", DefenderOverride{Ranks: Ranks{SpD: -7}}},
		{"攻撃+7(使わない側でも拒否)", DefenderOverride{Ranks: Ranks{Atk: 7}}},
		{"特攻-7", DefenderOverride{Ranks: Ranks{SpA: -7}}},
		{"素早さ+7", DefenderOverride{Ranks: Ranks{Spe: 7}}},
		{"未知の状態異常", DefenderOverride{Status: Status("confusion")}},
		{"大文字の状態異常", DefenderOverride{Status: Status("BURN")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := bulkInput(CategoryPhysical, TypeWater)
			in.DefenderOverride = tt.ov
			res, err := calcBulk(in)
			if !errors.Is(err, ErrInvalidDefenderOverride) {
				t.Fatalf("err=%v want ErrInvalidDefenderOverride", err)
			}
			if len(res.Rows) != 0 {
				t.Errorf("エラーなのに行を返した: %d 行", len(res.Rows))
			}
		})
	}
	// 境界 ±6 は受け付ける。
	in := bulkInput(CategoryPhysical, TypeWater)
	in.DefenderOverride = DefenderOverride{Ranks: Ranks{Atk: -6, Def: 6, SpA: 6, SpD: -6, Spe: -6}, Status: StatusFreeze}
	overrideBulk(t, in)
}
