package master_test

// 持ち物の役割(攻撃側/防御側)の導出規則(ADR-0175 §1)。
//   - TestItemRoles: ItemEffect の全項目 × 攻撃/防御のテーブル駆動(中立値・修飾子だけ・両方・効果なし・メガストーン)。
//   - TestItemRolesMatchEngineDamage: 同じ表の各行を engine に実際に持たせて、「その側に持たせるとダメージが変わるか
//     未対応の印が付く」ことと役割が一致することを確かめる(規則が計算の意味と食い違わないことの根拠。実装の写しにしない)。
//   - TestItemRolesTableCoversEveryItemEffectField: engine.ItemEffect に項目が増えたら、この表(=規則)の見直しを強制する。

import (
	"reflect"
	"slices"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

var (
	attackerOnly = []master.ItemRole{master.ItemRoleAttacker}
	defenderOnly = []master.ItemRole{master.ItemRoleDefender}
	bothRoles    = []master.ItemRole{master.ItemRoleAttacker, master.ItemRoleDefender}
	noRoles      = []master.ItemRole{}
)

type itemRoleCase struct {
	name        string
	effect      *engine.ItemEffect
	isMegaStone bool
	want        []master.ItemRole
}

// 値の例(4096 基準): 6144 = ×1.5、5324 = ×1.3、4915 = ×1.2、4505 = ×1.1、4096 = ×1.0(中立)。
func itemRoleCases() []itemRoleCase {
	return []itemRoleCase{
		{name: "効果なし(nil)", effect: nil, want: noRoles},
		{name: "効果が空", effect: &engine.ItemEffect{}, want: noRoles},

		// StatMods: 攻撃側は atk(物理)・spa(特殊)、防御側は def(物理)・spd(特殊)を読む(engine の offensiveStatMod / defensiveStatMod)。
		{name: "StatMods atk", effect: &engine.ItemEffect{StatMods: map[engine.StatKey]int{engine.StatAtk: 6144}}, want: attackerOnly},
		{name: "StatMods spa", effect: &engine.ItemEffect{StatMods: map[engine.StatKey]int{engine.StatSpA: 6144}}, want: attackerOnly},
		{name: "StatMods def", effect: &engine.ItemEffect{StatMods: map[engine.StatKey]int{engine.StatDef: 6144}}, want: defenderOnly},
		{name: "StatMods spd", effect: &engine.ItemEffect{StatMods: map[engine.StatKey]int{engine.StatSpD: 6144}}, want: defenderOnly},
		{name: "StatMods def と spd", effect: &engine.ItemEffect{StatMods: map[engine.StatKey]int{engine.StatDef: 6144, engine.StatSpD: 6144}}, want: defenderOnly},
		{name: "StatMods hp はダメージ計算で読まない", effect: &engine.ItemEffect{StatMods: map[engine.StatKey]int{engine.StatHP: 6144}}, want: noRoles},
		{name: "StatMods spe はダメージ計算で読まない", effect: &engine.ItemEffect{StatMods: map[engine.StatKey]int{engine.StatSpe: 6144}}, want: noRoles},
		{name: "StatMods atk が中立(4096)", effect: &engine.ItemEffect{StatMods: map[engine.StatKey]int{engine.StatAtk: 4096}}, want: noRoles},
		{name: "StatMods spd が中立(4096)", effect: &engine.ItemEffect{StatMods: map[engine.StatKey]int{engine.StatSpD: 4096}}, want: noRoles},
		{name: "StatMods atk と spd(両方)", effect: &engine.ItemEffect{StatMods: map[engine.StatKey]int{engine.StatAtk: 6144, engine.StatSpD: 6144}}, want: bothRoles},

		// DamageMod: 攻撃側の最終ダメージ倍率(otherModifiers)。OnlySuperEffective は条件だけ。
		{name: "DamageMod", effect: &engine.ItemEffect{DamageMod: 5324}, want: attackerOnly},
		{name: "DamageMod が中立(4096)", effect: &engine.ItemEffect{DamageMod: 4096}, want: noRoles},
		{name: "DamageMod + OnlySuperEffective", effect: &engine.ItemEffect{DamageMod: 4915, OnlySuperEffective: true}, want: attackerOnly},
		{name: "OnlySuperEffective だけ", effect: &engine.ItemEffect{OnlySuperEffective: true}, want: noRoles},

		// PowerMod: 攻撃側の威力倍率(powerModifier)。PowerCategory は条件だけ。
		{name: "PowerMod", effect: &engine.ItemEffect{PowerMod: 4505}, want: attackerOnly},
		{name: "PowerMod + PowerCategory physical", effect: &engine.ItemEffect{PowerMod: 4505, PowerCategory: engine.CategoryPhysical}, want: attackerOnly},
		{name: "PowerMod + PowerCategory special", effect: &engine.ItemEffect{PowerMod: 4505, PowerCategory: engine.CategorySpecial}, want: attackerOnly},
		{name: "PowerMod が中立(4096)", effect: &engine.ItemEffect{PowerMod: 4096}, want: noRoles},
		{name: "PowerCategory だけ", effect: &engine.ItemEffect{PowerCategory: engine.CategoryPhysical}, want: noRoles},

		// BoostType: 攻撃側のタイプ強化(powerModifier。BoostTypeMod != 0 のときだけ掛かる)。
		{name: "BoostType + BoostTypeMod", effect: &engine.ItemEffect{BoostType: engine.TypeFire, BoostTypeMod: 4915}, want: attackerOnly},
		{name: "BoostType だけ(BoostTypeMod 0)", effect: &engine.ItemEffect{BoostType: engine.TypeFire}, want: noRoles},
		{name: "BoostTypeMod だけ(BoostType なし)", effect: &engine.ItemEffect{BoostTypeMod: 4915}, want: noRoles},
		{name: "BoostType + BoostTypeMod が中立(4096)", effect: &engine.ItemEffect{BoostType: engine.TypeFire, BoostTypeMod: 4096}, want: noRoles},

		// ResistBerryType: 防御側の半減きのみ(otherModifiers)。ノーマルは抜群でなくても掛かる。
		{name: "ResistBerryType", effect: &engine.ItemEffect{ResistBerryType: engine.TypeFire}, want: defenderOnly},
		{name: "ResistBerryType normal", effect: &engine.ItemEffect{ResistBerryType: engine.TypeNormal}, want: defenderOnly},

		// 未対応の印(ADR-0123): その側で持つとダメージが変わるのに計算に入れていない。
		{name: "UnsupportedAttacker", effect: &engine.ItemEffect{UnsupportedAttacker: true}, want: attackerOnly},
		{name: "UnsupportedDefender", effect: &engine.ItemEffect{UnsupportedDefender: true}, want: defenderOnly},
		{name: "UnsupportedAttacker と UnsupportedDefender", effect: &engine.ItemEffect{UnsupportedAttacker: true, UnsupportedDefender: true}, want: bothRoles},

		// 攻撃側の項目と防御側の項目を両方持つ。
		{name: "DamageMod と ResistBerryType", effect: &engine.ItemEffect{DamageMod: 5324, ResistBerryType: engine.TypeFire}, want: bothRoles},
		{name: "PowerMod と UnsupportedDefender", effect: &engine.ItemEffect{PowerMod: 4505, UnsupportedDefender: true}, want: bothRoles},

		// メガストーンは効果があっても常に空(種族の選択で固定される持ち物。ADR-0175 §1・§2)。
		{name: "メガストーン(効果なし)", effect: nil, isMegaStone: true, want: noRoles},
		{name: "メガストーン(効果あり)", effect: &engine.ItemEffect{DamageMod: 5324}, isMegaStone: true, want: noRoles},
		{name: "メガストーン(両方の効果)", effect: &engine.ItemEffect{UnsupportedAttacker: true, UnsupportedDefender: true}, isMegaStone: true, want: noRoles},
	}
}

func TestItemRoles(t *testing.T) {
	for _, tc := range itemRoleCases() {
		t.Run(tc.name, func(t *testing.T) {
			got := master.ItemRoles(tc.effect, tc.isMegaStone)
			if got == nil {
				t.Fatalf("ItemRoles = nil, want nil でない配列 %v(nil は JSON で null になる。空のときも [] を返す)", tc.want)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ItemRoles = %v, want %v", got, tc.want)
			}
		})
	}
}

// ItemRoles は入力の効果を書き換えない(公開 API・キャッシュの値を壊さない)。
func TestItemRolesDoesNotMutateEffect(t *testing.T) {
	effect := &engine.ItemEffect{StatMods: map[engine.StatKey]int{engine.StatAtk: 6144, engine.StatSpD: 6144}, DamageMod: 5324}
	before := *effect
	before.StatMods = map[engine.StatKey]int{engine.StatAtk: 6144, engine.StatSpD: 6144}
	_ = master.ItemRoles(effect, false)
	if !reflect.DeepEqual(*effect, before) {
		t.Errorf("効果が書き換わった: %+v, want %+v", *effect, before)
	}
}

func TestAllItemRolesOrder(t *testing.T) {
	if got := master.AllItemRoles(); !reflect.DeepEqual(got, bothRoles) {
		t.Errorf("AllItemRoles = %v, want %v(ItemRoles の並びと同じ attacker → defender)", got, bothRoles)
	}
}

// --- engine との一致 ---------------------------------------------------------------

// roleProbeChart は確認用の相性表。防御側をくさ・ほのおにして、ほのお技・みず技がそれぞれ抜群になる組を作る
// (OnlySuperEffective・ResistBerryType が掛かる入力を必ず含めるため)。
func roleProbeChart(t *testing.T) engine.TypeChart {
	t.Helper()
	c, err := master.TypeChart(
		[]master.TypeRow{
			{ID: "normal", SortOrder: 1, NameJa: "テストふつう"},
			{ID: "fire", SortOrder: 2, NameJa: "テストほのお"},
			{ID: "water", SortOrder: 3, NameJa: "テストみず"},
			{ID: "grass", SortOrder: 4, NameJa: "テストくさ"},
		},
		[]master.TypeChartRow{
			{AttackType: "fire", DefenseType: "grass", Code: 4},
			{AttackType: "water", DefenseType: "fire", Code: 4},
			{AttackType: "grass", DefenseType: "water", Code: 4},
			{AttackType: "fire", DefenseType: "water", Code: 1},
			{AttackType: "grass", DefenseType: "fire", Code: 1},
		},
	)
	if err != nil {
		t.Fatalf("TypeChart: %v", err)
	}
	return c
}

func probeIndividual(key string, ty engine.Type) engine.Individual {
	return engine.Individual{
		Species: engine.Species{Key: key, DexNo: 9900, NameJa: "テストかくにん", Types: []engine.Type{ty},
			BaseStats: engine.Stats{HP: 100, Atk: 100, Def: 100, SpA: 100, SpD: 100, Spe: 100}},
		Nature: engine.NatureNeutral,
	}
}

// probeInputs は役割の確認に使う入力の組(技のタイプ 4 × 分類 2 × 防御側のタイプ 2)。
func probeInputs(t *testing.T) []engine.DamageInput {
	t.Helper()
	chart := roleProbeChart(t)
	var out []engine.DamageInput
	for _, defType := range []engine.Type{engine.TypeGrass, engine.TypeFire} {
		for _, moveType := range []engine.Type{engine.TypeFire, engine.TypeWater, engine.TypeGrass, engine.TypeNormal} {
			for _, cat := range []engine.MoveCategory{engine.CategoryPhysical, engine.CategorySpecial} {
				out = append(out, engine.DamageInput{
					Format:    engine.FormatSingle,
					Attacker:  probeIndividual("9900-000", engine.TypeNormal),
					Defender:  probeIndividual("9901-000", defType),
					Move:      engine.Move{ID: "testprobe", NameJa: "テストかくにんわざ", Type: moveType, Category: cat, Power: 80},
					TypeChart: chart,
				})
			}
		}
	}
	return out
}

// sideAffectsDamage は、その側に持たせたとき、持たせないときと比べて Rolls が変わるか、その側の持ち物の未対応の印が
// 付く入力が1つでもあるかを返す。
func sideAffectsDamage(t *testing.T, effect *engine.ItemEffect, side master.ItemRole) bool {
	t.Helper()
	item := &engine.Item{ID: "testroleitem", NameJa: "テストやくわり", Effect: effect}
	wantTarget := engine.UnsupportedTargetAttackerItem
	if side == master.ItemRoleDefender {
		wantTarget = engine.UnsupportedTargetDefenderItem
	}
	for _, base := range probeInputs(t) {
		without, err := engine.CalcDamage(base)
		if err != nil {
			t.Fatalf("CalcDamage(持ち物なし): %v", err)
		}
		with := base
		if side == master.ItemRoleAttacker {
			with.Attacker.Item = item
		} else {
			with.Defender.Item = item
		}
		got, err := engine.CalcDamage(with)
		if err != nil {
			t.Fatalf("CalcDamage(%s に持たせる): %v", side, err)
		}
		if got.Rolls != without.Rolls {
			return true
		}
		if slices.ContainsFunc(got.Unsupported, func(m engine.UnsupportedMark) bool { return m.Target == wantTarget }) {
			return true
		}
	}
	return false
}

func TestItemRolesMatchEngineDamage(t *testing.T) {
	for _, tc := range itemRoleCases() {
		if tc.isMegaStone || tc.effect == nil {
			continue // メガストーンは engine の概念ではない(役割は規則で空)。nil は効果なし
		}
		t.Run(tc.name, func(t *testing.T) {
			got := master.ItemRoles(tc.effect, false)
			for _, side := range master.AllItemRoles() {
				affects := sideAffectsDamage(t, tc.effect, side)
				if has := slices.Contains(got, side); has != affects {
					t.Errorf("役割 %s: ItemRoles に含む=%v だが、engine でその側に持たせてダメージ・印が変わる=%v", side, has, affects)
				}
			}
		})
	}
}

// 表が engine.ItemEffect の全項目を少なくとも1回は使うこと。項目が増えたら規則(ADR-0175 §1)と表の見直しが要る。
func TestItemRolesTableCoversEveryItemEffectField(t *testing.T) {
	typ := reflect.TypeOf(engine.ItemEffect{})
	used := map[string]bool{}
	for _, tc := range itemRoleCases() {
		if tc.effect == nil {
			continue
		}
		v := reflect.ValueOf(*tc.effect)
		for i := range typ.NumField() {
			if !v.Field(i).IsZero() {
				used[typ.Field(i).Name] = true
			}
		}
	}
	for i := range typ.NumField() {
		if name := typ.Field(i).Name; !used[name] {
			t.Errorf("engine.ItemEffect.%s を使う行が表に無い(ADR-0175 §1 の規則に足し、表に行を足す)", name)
		}
	}
}
