package engine

import "testing"

// issue #231 / ADR-0116: フィールド(地形)の補正は接地している側にだけ掛かる。
// 威力を上げる3つ(エレキ・グラス・サイコ)は攻撃側の接地、ミストのドラゴン半減は防御側の接地を見る
// (@smogon/calc 0.12.0 champions.js の isGrounded(attacker/defender))。
//
// 期待値は「同じ入力の威力を補正後の値にした、地形なしの結果」と同じロールかどうかで見る
// (pokeRound(100, 5325) = 130、pokeRound(100, 2048) = 50)。数値を手で持たないので、
// 他の補正段階の変更で壊れない。

// airborneAbility は「浮いている」だけを持つ特性の効果(マスタの効果定義の形。名前には依存しない)。
func airborneAbility() Ability {
	return Ability{ID: "example-airborne", Effect: &AbilityEffect{Airborne: true}}
}

func withPower(in DamageInput, power int) DamageInput {
	in.Field.Terrain = TerrainNone
	in.Move.Power = power
	return in
}

func TestTerrainAppliesOnlyToGroundedSide(t *testing.T) {
	const boosted, halved, unchanged = 130, 50, 100
	cases := []struct {
		name     string
		atkTypes []Type
		defTypes []Type
		atkAir   bool
		defAir   bool
		terrain  Terrain
		moveType Type
		want     int // 地形なしで同じロールになる威力
	}{
		{"接地した攻撃側 × エレキ", []Type{TypeWater}, []Type{TypePsychic}, false, false, TerrainElectric, TypeElectric, boosted},
		{"接地した攻撃側 × グラス", []Type{TypeWater}, []Type{TypePsychic}, false, false, TerrainGrassy, TypeGrass, boosted},
		{"接地した攻撃側 × サイコ", []Type{TypeWater}, []Type{TypeNormal}, false, false, TerrainPsychic, TypePsychic, boosted},
		{"接地した防御側 × ミスト × ドラゴン", []Type{TypeWater}, []Type{TypePsychic}, false, false, TerrainMisty, TypeDragon, halved},

		{"ひこう単タイプの攻撃側 × エレキ", []Type{TypeFlying}, []Type{TypePsychic}, false, false, TerrainElectric, TypeElectric, unchanged},
		{"ひこう複合(2番目)の攻撃側 × グラス", []Type{TypeWater, TypeFlying}, []Type{TypePsychic}, false, false, TerrainGrassy, TypeGrass, unchanged},
		{"ひこう複合(1番目)の攻撃側 × サイコ", []Type{TypeFlying, TypeWater}, []Type{TypeNormal}, false, false, TerrainPsychic, TypePsychic, unchanged},
		{"浮く特性の攻撃側 × エレキ", []Type{TypeWater}, []Type{TypePsychic}, true, false, TerrainElectric, TypeElectric, unchanged},
		{"浮く特性の攻撃側 × サイコ", []Type{TypeWater}, []Type{TypeNormal}, true, false, TerrainPsychic, TypePsychic, unchanged},

		{"ひこうの防御側 × ミスト × ドラゴン", []Type{TypeWater}, []Type{TypeWater, TypeFlying}, false, false, TerrainMisty, TypeDragon, unchanged},
		{"浮く特性の防御側 × ミスト × ドラゴン", []Type{TypeWater}, []Type{TypePsychic}, false, true, TerrainMisty, TypeDragon, unchanged},

		// 反対側の接地は見ない。
		{"ひこうの防御側でも攻撃側が接地ならエレキ強化", []Type{TypeWater}, []Type{TypeFlying}, false, false, TerrainElectric, TypeElectric, boosted},
		{"浮く特性の防御側でも攻撃側が接地ならグラス強化", []Type{TypeWater}, []Type{TypePsychic}, false, true, TerrainGrassy, TypeGrass, boosted},
		{"ひこうの攻撃側でも防御側が接地ならミスト半減", []Type{TypeFlying}, []Type{TypePsychic}, false, false, TerrainMisty, TypeDragon, halved},
		{"浮く特性の攻撃側でも防御側が接地ならミスト半減", []Type{TypeWater}, []Type{TypePsychic}, true, false, TerrainMisty, TypeDragon, halved},

		// 地形なし・対象外タイプは変化なし。
		{"地形なしのひこう", []Type{TypeFlying}, []Type{TypeFlying}, true, true, TerrainNone, TypeElectric, unchanged},
		{"エレキで対象外タイプ", []Type{TypeWater}, []Type{TypePsychic}, false, false, TerrainElectric, TypeFire, unchanged},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := ctrlInput(c.atkTypes, c.defTypes, CategorySpecial, c.moveType)
			in.Field.Terrain = c.terrain
			if c.atkAir {
				in.Attacker.Ability = airborneAbility()
			}
			if c.defAir {
				in.Defender.Ability = airborneAbility()
			}
			got, err := calcDamage(in)
			if err != nil {
				t.Fatal(err)
			}
			want, err := calcDamage(withPower(in, c.want))
			if err != nil {
				t.Fatal(err)
			}
			if got.Rolls != want.Rolls {
				t.Errorf("rolls=%v, want 地形なし・威力%d の %v", got.Rolls, c.want, want.Rolls)
			}
		})
	}
}

func TestIsGrounded(t *testing.T) {
	cases := []struct {
		name    string
		types   []Type
		ability *AbilityEffect
		want    bool
	}{
		{"特性なしの非ひこう", []Type{TypeNormal}, nil, true},
		{"ひこう単", []Type{TypeFlying}, nil, false},
		{"ひこう複合", []Type{TypeSteel, TypeFlying}, nil, false},
		{"浮く特性", []Type{TypeGhost}, &AbilityEffect{Airborne: true}, false},
		// 地面技を無効にするだけでは浮いていない(接地判定は Airborne だけを見る)。
		{"地面無効だが Airborne なし", []Type{TypeNormal}, &AbilityEffect{DefImmuneTypes: []Type{TypeGround}}, true},
		// 地面技を吸収する特性(どしょく)は接地している。
		{"地面吸収", []Type{TypeNormal}, &AbilityEffect{DefAbsorbTypes: map[Type]AbsorbEffect{TypeGround: {}}}, true},
		{"補正のみの特性", []Type{TypeNormal}, &AbilityEffect{StabMod: ModifierAdaptability}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := Individual{Species: Species{Types: c.types}, Ability: Ability{Effect: c.ability}}
			if got := isGrounded(in); got != c.want {
				t.Errorf("isGrounded=%v want %v", got, c.want)
			}
		})
	}
}
