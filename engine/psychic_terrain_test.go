package engine

import "testing"

// ADR-0121 §5 / ADR-0123: サイコフィールドでは、優先度が正の攻撃技は接地した防御側に当たらない
// (@smogon/calc 0.12.0 champions.js: move.priority > 0 && field.hasTerrain('Psychic') && isGrounded(defender))。
// 判定の順は oracle と同じで、タイプ由来の無効・特性による無効/吸収が先。

func TestPsychicTerrainBlocksPriorityMoves(t *testing.T) {
	airborne := Ability{ID: "example-airborne", Effect: &AbilityEffect{Airborne: true}}
	immuneNormal := Ability{ID: "example-immune", Effect: &AbilityEffect{DefImmuneTypes: []Type{TypeNormal}}}
	cases := []struct {
		name     string
		defTypes []Type
		defAb    Ability
		terrain  Terrain
		priority int
		category MoveCategory
		want     NullifyKind
		zero     bool
	}{
		{"接地した防御側 × 先制技", []Type{TypePsychic}, Ability{}, TerrainPsychic, 1, CategoryPhysical, NullifyPsychicTerrain, true},
		{"優先度 +2 でも同じ", []Type{TypePsychic}, Ability{}, TerrainPsychic, 2, CategorySpecial, NullifyPsychicTerrain, true},
		{"ひこうの防御側には当たる", []Type{TypeWater, TypeFlying}, Ability{}, TerrainPsychic, 1, CategoryPhysical, NullifyNone, false},
		{"浮く特性の防御側には当たる", []Type{TypePsychic}, airborne, TerrainPsychic, 1, CategoryPhysical, NullifyNone, false},
		{"優先度 0 は当たる", []Type{TypePsychic}, Ability{}, TerrainPsychic, 0, CategoryPhysical, NullifyNone, false},
		{"優先度が負でも当たる", []Type{TypePsychic}, Ability{}, TerrainPsychic, -1, CategoryPhysical, NullifyNone, false},
		{"他のフィールドでは当たる", []Type{TypePsychic}, Ability{}, TerrainElectric, 1, CategoryPhysical, NullifyNone, false},
		{"フィールドなしでは当たる", []Type{TypePsychic}, Ability{}, TerrainNone, 1, CategoryPhysical, NullifyNone, false},
		// タイプ由来の無効が先(Nullified は空のまま)。
		{"ゴーストの防御側はタイプで無効", []Type{TypeGhost}, Ability{}, TerrainPsychic, 1, CategoryPhysical, NullifyNone, true},
		// 特性による無効が先。
		{"特性で無効が先", []Type{TypePsychic}, immuneNormal, TerrainPsychic, 1, CategoryPhysical, NullifyImmune, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := ctrlInput([]Type{TypeWater}, c.defTypes, c.category, TypeNormal)
			in.Defender.Ability = c.defAb
			in.Field.Terrain = c.terrain
			in.Move.Priority = c.priority
			got, err := calcDamage(in)
			if err != nil {
				t.Fatal(err)
			}
			if got.Nullified != c.want {
				t.Errorf("Nullified = %q, want %q", got.Nullified, c.want)
			}
			if c.zero {
				if got.MaxDamage() != 0 || got.KO != (KOChance{}) {
					t.Errorf("ダメージ %v / KO %+v, want 0 / ゼロ値", got.Rolls, got.KO)
				}
				return
			}
			// 当たるときは優先度 0 の同じ入力と同じロール(フィールドの他の補正は変えない)。
			ref := in
			ref.Move.Priority = 0
			want, err := calcDamage(ref)
			if err != nil {
				t.Fatal(err)
			}
			if got.Rolls != want.Rolls || got.MinDamage() == 0 {
				t.Errorf("ロール = %v, want %v(0 でない)", got.Rolls, want.Rolls)
			}
		})
	}
}
