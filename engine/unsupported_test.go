package engine

import (
	"reflect"
	"sort"
	"testing"
)

// issue #271-b / #270 案 B / ADR-0123: engine が正しく計算できない技の機構・持ち物・特性を持つ入力には、
// 結果に「未対応」の印を付ける。数値は従来どおり(通常の式)で、印は数値を変えない。

func moveMark(id string, r UnsupportedReason) UnsupportedMark {
	return UnsupportedMark{Target: UnsupportedTargetMove, Reason: r, ID: id}
}

func TestAllMoveMechanismsSortedUnique(t *testing.T) {
	all := AllMoveMechanisms()
	if len(all) != 13 {
		t.Fatalf("機構の数 = %d, want 13(ADR-0121 の表)", len(all))
	}
	if !sort.SliceIsSorted(all, func(i, j int) bool { return all[i] < all[j] }) {
		t.Fatalf("機構の一覧が昇順でない: %v", all)
	}
	for i := 1; i < len(all); i++ {
		if all[i] == all[i-1] {
			t.Fatalf("機構 %q が重複", all[i])
		}
	}
	for _, m := range all {
		if !m.Known() {
			t.Errorf("%q が Known でない", m)
		}
	}
	if MoveMechanism("broken").Known() {
		t.Error("未知の値が Known になっている")
	}
	all[0] = "broken"
	if AllMoveMechanisms()[0] == "broken" {
		t.Fatal("AllMoveMechanisms が内部のスライスをそのまま返している")
	}
}

// 常に印を付ける機構(条件で正しくなることを engine が判定できないもの)。
func TestUnsupportedMoveMechanismsAlwaysMarked(t *testing.T) {
	always := []MoveMechanism{
		MechanismAltDefenseStat, MechanismAltOffenseStat, MechanismEffectivenessChange,
		MechanismFixedDamage, MechanismMoveSpecific, MechanismMultiHit, MechanismOHKO,
		MechanismTypeChange, MechanismVariablePower,
	}
	for _, m := range always {
		t.Run(string(m), func(t *testing.T) {
			plain := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
			want, err := calcDamage(plain)
			if err != nil {
				t.Fatal(err)
			}
			if want.Unsupported != nil {
				t.Fatalf("通常の技に印が付いた: %v", want.Unsupported)
			}
			in := plain
			in.Move.Mechanisms = []MoveMechanism{m}
			got, err := calcDamage(in)
			if err != nil {
				t.Fatal(err)
			}
			if got.Rolls != want.Rolls || got.KO != want.KO {
				t.Errorf("印で数値が変わった: %v -> %v", want.Rolls, got.Rolls)
			}
			if w := []UnsupportedMark{moveMark("m", UnsupportedReason(m))}; !reflect.DeepEqual(got.Unsupported, w) {
				t.Errorf("Unsupported = %v, want %v", got.Unsupported, w)
			}
		})
	}
}

// 条件によっては通常の式で正しい機構は、誤るときだけ印を付ける。
func TestUnsupportedConditionalMechanisms(t *testing.T) {
	base := func(cat MoveCategory, m MoveMechanism) DamageInput {
		in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, cat, TypeNormal)
		in.Move.Mechanisms = []MoveMechanism{m}
		return in
	}
	cases := []struct {
		name   string
		in     func() DamageInput
		marked bool
	}{
		{"必ず急所 × 急所なしの入力は誤る", func() DamageInput { return base(CategoryPhysical, MechanismAlwaysCrit) }, true},
		{"必ず急所 × 急所ありの入力は正しい", func() DamageInput {
			in := base(CategoryPhysical, MechanismAlwaysCrit)
			in.Critical = true
			return in
		}, false},
		{"防御ランク無視 × 防御側の防御ランクあり(物理)", func() DamageInput {
			in := base(CategoryPhysical, MechanismIgnoreDefenseRanks)
			in.Defender.Ranks.Def = 1
			return in
		}, true},
		{"防御ランク無視 × 防御側の特防ランクあり(特殊)", func() DamageInput {
			in := base(CategorySpecial, MechanismIgnoreDefenseRanks)
			in.Defender.Ranks.SpD = -2
			return in
		}, true},
		{"防御ランク無視 × 使わない側のランクだけ(物理で特防)", func() DamageInput {
			in := base(CategoryPhysical, MechanismIgnoreDefenseRanks)
			in.Defender.Ranks.SpD = 3
			return in
		}, false},
		{"防御ランク無視 × ランクなし", func() DamageInput { return base(CategoryPhysical, MechanismIgnoreDefenseRanks) }, false},
		{"優先度変化 × サイコフィールド", func() DamageInput {
			in := base(CategoryPhysical, MechanismPriorityChange)
			in.Field.Terrain = TerrainPsychic
			return in
		}, true},
		{"優先度変化 × 他のフィールド", func() DamageInput {
			in := base(CategoryPhysical, MechanismPriorityChange)
			in.Field.Terrain = TerrainGrassy
			return in
		}, false},
		{"場で変わる技 × 天候", func() DamageInput {
			in := base(CategoryPhysical, MechanismFieldSpecific)
			in.Field.Weather = WeatherRain
			return in
		}, true},
		{"場で変わる技 × フィールド", func() DamageInput {
			in := base(CategoryPhysical, MechanismFieldSpecific)
			in.Field.Terrain = TerrainGrassy
			return in
		}, true},
		{"場で変わる技 × 天候もフィールドもなし", func() DamageInput { return base(CategoryPhysical, MechanismFieldSpecific) }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := c.in()
			got, err := calcDamage(in)
			if err != nil {
				t.Fatal(err)
			}
			if c.marked != (len(got.Unsupported) == 1) {
				t.Errorf("Unsupported = %v, want marked=%v", got.Unsupported, c.marked)
			}
			if c.marked {
				w := moveMark("m", UnsupportedReason(in.Move.Mechanisms[0]))
				if got.Unsupported[0] != w {
					t.Errorf("Unsupported[0] = %v, want %v", got.Unsupported[0], w)
				}
			}
		})
	}
}

// 威力 0 の攻撃技は、機構が無くても「ダメージ 0 = 倒せない」を正しい結果のように返さない(issue #271 境界値)。
// 変化技は従来どおり印なしの 0(異常系)。
func TestUnsupportedZeroPowerAttack(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategorySpecial, TypeNormal)
	in.Move.Power = 0
	got, err := calcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.MaxDamage() != 0 {
		t.Errorf("威力 0 のダメージ = %d, want 0(数値は従来どおり)", got.MaxDamage())
	}
	if w := []UnsupportedMark{moveMark("m", UnsupportedZeroPower)}; !reflect.DeepEqual(got.Unsupported, w) {
		t.Errorf("Unsupported = %v, want %v", got.Unsupported, w)
	}

	// 機構も持つ威力 0 の技(重さで変わる 等)は、機構の印に加えて zero_power も付ける。
	in.Move.Mechanisms = []MoveMechanism{MechanismVariablePower}
	got, err = calcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	w := []UnsupportedMark{moveMark("m", UnsupportedReason(MechanismVariablePower)), moveMark("m", UnsupportedZeroPower)}
	if !reflect.DeepEqual(got.Unsupported, w) {
		t.Errorf("Unsupported = %v, want %v", got.Unsupported, w)
	}

	status := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryStatus, TypeNormal)
	status.Move.Power = 0
	got, err = calcDamage(status)
	if err != nil {
		t.Fatal(err)
	}
	if got.Unsupported != nil || got.MaxDamage() != 0 {
		t.Errorf("変化技: Unsupported=%v max=%d, want nil 0", got.Unsupported, got.MaxDamage())
	}
}

// 未知の機構(取得元の更新で増えた値が engine に届いた場合)は安全側で印を付ける。
// 複数の機構は値の昇順・重複なしで並べる。
func TestUnsupportedMechanismsOrderAndUnknown(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Move.Mechanisms = []MoveMechanism{MechanismVariablePower, "zz_future", MechanismMultiHit, MechanismMultiHit}
	got, err := calcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	w := []UnsupportedMark{
		moveMark("m", UnsupportedReason(MechanismMultiHit)),
		moveMark("m", UnsupportedReason(MechanismVariablePower)),
		moveMark("m", "zz_future"),
	}
	if !reflect.DeepEqual(got.Unsupported, w) {
		t.Errorf("Unsupported = %v, want %v", got.Unsupported, w)
	}
}

// 持ち物・特性の「未対応」の印は、効果が効く側(攻撃側 / 防御側)で持っているときだけ付ける。
func TestUnsupportedItemsAndAbilities(t *testing.T) {
	atkOnlyItem := &Item{ID: "atk-item", Effect: &ItemEffect{UnsupportedAttacker: true}}
	defOnlyItem := &Item{ID: "def-item", Effect: &ItemEffect{UnsupportedDefender: true}}
	atkOnlyAbility := Ability{ID: "atk-ability", Effect: &AbilityEffect{UnsupportedAttacker: true}}
	bothAbility := Ability{ID: "both-ability", Effect: &AbilityEffect{UnsupportedAttacker: true, UnsupportedDefender: true}}

	cases := []struct {
		name string
		edit func(*DamageInput)
		want []UnsupportedMark
	}{
		{"攻撃側の攻撃側用の持ち物", func(in *DamageInput) { in.Attacker.Item = atkOnlyItem },
			[]UnsupportedMark{{Target: UnsupportedTargetAttackerItem, Reason: UnsupportedEffect, ID: "atk-item"}}},
		{"防御側の攻撃側用の持ち物は効かない", func(in *DamageInput) { in.Defender.Item = atkOnlyItem }, nil},
		{"防御側の防御側用の持ち物", func(in *DamageInput) { in.Defender.Item = defOnlyItem },
			[]UnsupportedMark{{Target: UnsupportedTargetDefenderItem, Reason: UnsupportedEffect, ID: "def-item"}}},
		{"攻撃側の防御側用の持ち物は効かない", func(in *DamageInput) { in.Attacker.Item = defOnlyItem }, nil},
		{"攻撃側の攻撃側用の特性", func(in *DamageInput) { in.Attacker.Ability = atkOnlyAbility },
			[]UnsupportedMark{{Target: UnsupportedTargetAttackerAbility, Reason: UnsupportedEffect, ID: "atk-ability"}}},
		{"防御側の攻撃側用の特性は効かない", func(in *DamageInput) { in.Defender.Ability = atkOnlyAbility }, nil},
		{"両側で効く特性を防御側が持つ", func(in *DamageInput) { in.Defender.Ability = bothAbility },
			[]UnsupportedMark{{Target: UnsupportedTargetDefenderAbility, Reason: UnsupportedEffect, ID: "both-ability"}}},
		{"全部そろうと 技 → 攻撃側の持ち物 → 攻撃側の特性 → 防御側の持ち物 → 防御側の特性 の順", func(in *DamageInput) {
			in.Move.Mechanisms = []MoveMechanism{MechanismMultiHit}
			in.Attacker.Item, in.Attacker.Ability = atkOnlyItem, bothAbility
			in.Defender.Item, in.Defender.Ability = defOnlyItem, bothAbility
		}, []UnsupportedMark{
			moveMark("m", UnsupportedReason(MechanismMultiHit)),
			{Target: UnsupportedTargetAttackerItem, Reason: UnsupportedEffect, ID: "atk-item"},
			{Target: UnsupportedTargetAttackerAbility, Reason: UnsupportedEffect, ID: "both-ability"},
			{Target: UnsupportedTargetDefenderItem, Reason: UnsupportedEffect, ID: "def-item"},
			{Target: UnsupportedTargetDefenderAbility, Reason: UnsupportedEffect, ID: "both-ability"},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plain := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
			want, err := calcDamage(plain)
			if err != nil {
				t.Fatal(err)
			}
			in := plain
			c.edit(&in)
			got, err := calcDamage(in)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Unsupported, c.want) {
				t.Errorf("Unsupported = %v, want %v", got.Unsupported, c.want)
			}
			if got.Rolls != want.Rolls {
				t.Errorf("印だけの効果で数値が変わった: %v -> %v", want.Rolls, got.Rolls)
			}
		})
	}
}

// 一括計算の各行・逆算の各候補にも同じ印が付く(CalcDamage の合成なので、持ち物の差し替えごとに決まる)。
func TestUnsupportedPropagatesToBulkAndReverse(t *testing.T) {
	marked := &Item{ID: "def-item", Effect: &ItemEffect{UnsupportedDefender: true}}
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Move.Mechanisms = []MoveMechanism{MechanismMultiHit}
	bulk, err := calcBulk(BulkInput{
		Format: FormatSingle, Attacker: in.Attacker, DefenderSpecies: in.Defender.Species, Move: in.Move,
		PresetKeys: []PresetKey{PresetNone}, ItemVariants: []*Item{nil, marked},
	})
	if err != nil {
		t.Fatal(err)
	}
	moveOnly := []UnsupportedMark{moveMark("m", UnsupportedReason(MechanismMultiHit))}
	withItem := append(append([]UnsupportedMark(nil), moveOnly...),
		UnsupportedMark{Target: UnsupportedTargetDefenderItem, Reason: UnsupportedEffect, ID: "def-item"})
	if len(bulk.Rows) != 2 {
		t.Fatalf("行数 = %d, want 2", len(bulk.Rows))
	}
	if !reflect.DeepEqual(bulk.Rows[0].Result.Unsupported, moveOnly) || !reflect.DeepEqual(bulk.Rows[1].Result.Unsupported, withItem) {
		t.Errorf("一括計算の印 = %v / %v, want %v / %v",
			bulk.Rows[0].Result.Unsupported, bulk.Rows[1].Result.Unsupported, moveOnly, withItem)
	}

	rev, err := calcReverse(ReverseInput{
		Format: FormatSingle, Side: SideDefender, Known: in.Attacker, UnknownSpecies: in.Defender.Species,
		Move: in.Move, ItemCandidates: []*Item{nil, marked}, Observations: []Observation{{Percent: 40}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range rev.Candidates {
		want := moveOnly
		if c.ItemID == "def-item" {
			want = withItem
		}
		if !reflect.DeepEqual(c.Unsupported, want) {
			t.Errorf("逆算の候補 (%s, %q) の印 = %v, want %v", c.NatureClass, c.ItemID, c.Unsupported, want)
		}
	}
}
