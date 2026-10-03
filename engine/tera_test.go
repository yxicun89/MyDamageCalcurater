package engine

import (
	"reflect"
	"testing"
)

// ADR-0224: テラスタル(Individual.TeraType)をオプションの機能として計算に反映する。
// TeraType が空(TypeNone)なら従来どおり(ゴールデン既存ファイルはバイト不変)。指定したときだけ、
// @smogon/calc 0.12.0 の Champions 世代(Generations.get(0))と同じ結果にする(ADR-0224 §1 の表 T1〜T11):
//
//   - 攻撃側のタイプ一致(util.getStabMod。4096 基準の加算):
//     4096 + (元タイプ=技 ? 2048) + (テラス=技 ? 2048)
//   - (てきおうりょく かつ「技のタイプを持つ」? (テラスが元タイプ ? 1024 : 2048))
//     「技のタイプを持つ」はテラス中ならテラスだけを見る(Pokemon.hasType)。
//   - 「そのタイプを持つか」の判定(接地・サイコフィールドの先制技・すなあらし/ゆきの防御補正)はテラスタイプで行う。
//   - 防御側のタイプ相性はテラスを見ない(Champions 世代の oracle の癖。本編 SV とは違う)。
//
// 数値はすべて ctrlInput(base=90。ロール前の d_i = floor(90×(85+i)/100))を手で丸めたもの:
//
//	×1.0 (4096): 76 … 90
//	×1.5 (6144): pokeRound(d_i×6144)
//	×2.0 (8192): d_i×2
//	×2.25(9216): pokeRound(d_i×9216)。i=7 は 82×2.25=184.5 → 184(五捨五超入の「五捨」)
var (
	teraRolls1x   = [16]int{76, 77, 78, 79, 80, 81, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90}
	teraRolls15x  = [16]int{114, 115, 117, 118, 120, 121, 121, 123, 124, 126, 127, 129, 130, 132, 133, 135}
	teraRolls2x   = [16]int{152, 154, 156, 158, 160, 162, 162, 164, 166, 168, 170, 172, 174, 176, 178, 180}
	teraRolls225x = [16]int{171, 173, 175, 178, 180, 182, 182, 184, 187, 189, 191, 193, 196, 198, 200, 202}
)

// AC-1・AC-2: 攻撃側のテラスのタイプ一致(T1〜T5)。防御側はエスパー単で、どの技も等倍。
func TestTeraAttackerSTAB(t *testing.T) {
	cases := []struct {
		name     string
		atkTypes []Type
		tera     Type
		adapt    bool
		moveType Type
		want     [16]int
		wantSTAB bool
	}{
		// 既定(テラス無し)は従来どおり。
		{"テラス無し・元タイプ一致", []Type{TypeWater}, TypeNone, false, TypeWater, teraRolls15x, true},
		{"テラス無し・不一致", []Type{TypeWater}, TypeNone, false, TypeNormal, teraRolls1x, false},
		{"テラス無し・てきおうりょく", []Type{TypeWater}, TypeNone, true, TypeWater, teraRolls2x, true},
		// T1: テラス=元タイプ=技 → ×2.0
		{"T1 テラス=元タイプ=技", []Type{TypeWater}, TypeWater, false, TypeWater, teraRolls2x, true},
		{"T1 複合タイプでテラス=元タイプ=技", []Type{TypeWater, TypeGround}, TypeGround, false, TypeGround, teraRolls2x, true},
		// T2: テラスが別タイプでも元タイプの技は ×1.5 のまま
		{"T2 テラス≠元タイプ・元タイプの技", []Type{TypeWater}, TypeFire, false, TypeWater, teraRolls15x, true},
		{"T2 複合: テラス=もう一方の元タイプ", []Type{TypeWater, TypeGround}, TypeWater, false, TypeGround, teraRolls15x, true},
		// T3: テラスだけが技と一致 → ×1.5
		{"T3 テラスだけ一致", []Type{TypeWater}, TypeNormal, false, TypeNormal, teraRolls15x, true},
		// T4: どれも一致しない → ×1.0
		{"T4 不一致", []Type{TypeWater}, TypeFire, false, TypeNormal, teraRolls1x, false},
		// T5: てきおうりょく
		{"T5 てきおうりょく・テラス=元タイプ=技 → ×2.25", []Type{TypeWater}, TypeWater, true, TypeWater, teraRolls225x, true},
		{"T5 てきおうりょく・テラスだけ一致 → ×2.0", []Type{TypeWater}, TypeNormal, true, TypeNormal, teraRolls2x, true},
		{"T5 てきおうりょく・テラス≠元タイプの元タイプの技 → ×1.5", []Type{TypeWater}, TypeFire, true, TypeWater, teraRolls15x, true},
		{"T5 てきおうりょく・複合でテラス=もう一方の元タイプ → ×1.5", []Type{TypeWater, TypeGround}, TypeWater, true, TypeGround, teraRolls15x, true},
		{"T5 てきおうりょく・不一致 → ×1.0", []Type{TypeWater}, TypeFire, true, TypeNormal, teraRolls1x, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := ctrlInput(c.atkTypes, []Type{TypePsychic}, CategoryPhysical, c.moveType)
			in.Attacker.TeraType = c.tera
			if c.adapt {
				in.Attacker.Ability = testAdaptability()
			}
			r, err := calcDamage(in)
			if err != nil {
				t.Fatal(err)
			}
			if r.Rolls != c.want {
				t.Errorf("rolls = %v\n want %v", r.Rolls, c.want)
			}
			if r.STAB != c.wantSTAB {
				t.Errorf("STAB = %v, want %v", r.STAB, c.wantSTAB)
			}
			if r.Effectiveness != 1.0 {
				t.Errorf("Effectiveness = %v, want 1.0", r.Effectiveness)
			}
			// 攻撃側のテラスは数値に反映するので、攻撃側のテラスの印は付けない(ADR-0224 §3)。
			if r.Unsupported != nil {
				t.Errorf("Unsupported = %v, want nil", r.Unsupported)
			}
		})
	}
}

// teraCalc は ctrlInput の数値だけを取り出す(関係のテスト用)。
func teraCalc(t *testing.T, in DamageInput) DamageResult {
	t.Helper()
	r, err := calcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func teraSameNumbers(a, b DamageResult) bool {
	return a.Rolls == b.Rolls && a.KO == b.KO && a.Effectiveness == b.Effectiveness &&
		a.STAB == b.STAB && a.DefenderHP == b.DefenderHP && a.Nullified == b.Nullified
}

// AC-3: 「そのタイプを持つか」の判定はテラスタイプで行う(T6〜T10)。
// 各ケースは「テラス中の個体」と「元のタイプがテラスタイプと同じ個体(テラス無し)」が同じ数値になることで確かめる。
// 技は相性・タイプ一致が両者で同じになるものを選んでいる(元タイプの違いが相性・一致に出ない)。
func TestTeraHasTypeChecks(t *testing.T) {
	cases := []struct {
		name string
		// tera はテラス中の入力、same は「元のタイプがテラスと同じ」入力、diff はテラス無しの元の入力。
		tera, same, diff func() DamageInput
	}{
		{
			name: "T6 攻撃側: テラスひこうは浮く(エレキフィールドの補正が外れる)",
			tera: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypePsychic}, CategoryPhysical, TypeElectric)
				in.Field.Terrain = TerrainElectric
				in.Attacker.TeraType = TypeFlying
				return in
			},
			same: func() DamageInput {
				in := ctrlInput([]Type{TypeFlying}, []Type{TypePsychic}, CategoryPhysical, TypeElectric)
				in.Field.Terrain = TerrainElectric
				return in
			},
			diff: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypePsychic}, CategoryPhysical, TypeElectric)
				in.Field.Terrain = TerrainElectric
				return in
			},
		},
		{
			name: "T6 攻撃側: 元ひこうでもテラス中(ひこう以外)は接地する",
			tera: func() DamageInput {
				in := ctrlInput([]Type{TypeFlying}, []Type{TypePsychic}, CategoryPhysical, TypeElectric)
				in.Field.Terrain = TerrainElectric
				in.Attacker.TeraType = TypeNormal
				return in
			},
			same: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypePsychic}, CategoryPhysical, TypeElectric)
				in.Field.Terrain = TerrainElectric
				return in
			},
			diff: func() DamageInput {
				in := ctrlInput([]Type{TypeFlying}, []Type{TypePsychic}, CategoryPhysical, TypeElectric)
				in.Field.Terrain = TerrainElectric
				return in
			},
		},
		{
			name: "T7 防御側: テラスひこうは浮く(ミストフィールドのドラゴン半減が外れる)",
			tera: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypeNormal}, CategoryPhysical, TypeDragon)
				in.Field.Terrain = TerrainMisty
				in.Defender.TeraType = TypeFlying
				return in
			},
			same: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypeFlying}, CategoryPhysical, TypeDragon)
				in.Field.Terrain = TerrainMisty
				return in
			},
			diff: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypeNormal}, CategoryPhysical, TypeDragon)
				in.Field.Terrain = TerrainMisty
				return in
			},
		},
		{
			name: "T7 防御側: 元ひこうでもテラス中(ひこう以外)は接地する",
			tera: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypeFlying}, CategoryPhysical, TypeDragon)
				in.Field.Terrain = TerrainMisty
				in.Defender.TeraType = TypeNormal
				return in
			},
			same: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypeNormal}, CategoryPhysical, TypeDragon)
				in.Field.Terrain = TerrainMisty
				return in
			},
			diff: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypeFlying}, CategoryPhysical, TypeDragon)
				in.Field.Terrain = TerrainMisty
				return in
			},
		},
		{
			name: "T9 すなあらし: テラスいわの防御側は特防 ×1.5",
			tera: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypeNormal}, CategorySpecial, TypeElectric)
				in.Field.Weather = WeatherSand
				in.Defender.TeraType = TypeRock
				return in
			},
			same: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypeRock}, CategorySpecial, TypeElectric)
				in.Field.Weather = WeatherSand
				return in
			},
			diff: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypeNormal}, CategorySpecial, TypeElectric)
				in.Field.Weather = WeatherSand
				return in
			},
		},
		{
			name: "T9 すなあらし: 元いわでもテラス中(いわ以外)は特防が上がらない",
			tera: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypeRock}, CategorySpecial, TypeElectric)
				in.Field.Weather = WeatherSand
				in.Defender.TeraType = TypeNormal
				return in
			},
			same: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypeNormal}, CategorySpecial, TypeElectric)
				in.Field.Weather = WeatherSand
				return in
			},
			diff: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypeRock}, CategorySpecial, TypeElectric)
				in.Field.Weather = WeatherSand
				return in
			},
		},
		{
			name: "T10 ゆき: テラスこおりの防御側は防御 ×1.5",
			tera: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypeNormal}, CategoryPhysical, TypeElectric)
				in.Field.Weather = WeatherSnow
				in.Defender.TeraType = TypeIce
				return in
			},
			same: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypeIce}, CategoryPhysical, TypeElectric)
				in.Field.Weather = WeatherSnow
				return in
			},
			diff: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypeNormal}, CategoryPhysical, TypeElectric)
				in.Field.Weather = WeatherSnow
				return in
			},
		},
		{
			name: "T10 ゆき: 元こおりでもテラス中(こおり以外)は防御が上がらない",
			tera: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypeIce}, CategoryPhysical, TypeElectric)
				in.Field.Weather = WeatherSnow
				in.Defender.TeraType = TypeNormal
				return in
			},
			same: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypeNormal}, CategoryPhysical, TypeElectric)
				in.Field.Weather = WeatherSnow
				return in
			},
			diff: func() DamageInput {
				in := ctrlInput([]Type{TypeNormal}, []Type{TypeIce}, CategoryPhysical, TypeElectric)
				in.Field.Weather = WeatherSnow
				return in
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, same, diff := teraCalc(t, c.tera()), teraCalc(t, c.same()), teraCalc(t, c.diff())
			if !teraSameNumbers(got, same) {
				t.Errorf("テラス中の数値が「元のタイプがテラスと同じ個体」と違う\n got %+v\nwant %+v", got, same)
			}
			if teraSameNumbers(got, diff) {
				t.Errorf("テラスを付けても数値が変わらない(判定がテラスタイプを見ていない): %v", got.Rolls)
			}
		})
	}
}

// AC-3: T8 サイコフィールドの先制技は、テラスで浮く/接地する防御側でも oracle と同じに当たる/当たらない。
func TestTeraPsychicTerrainPriority(t *testing.T) {
	cases := []struct {
		name     string
		defTypes []Type
		tera     Type
		blocked  bool
	}{
		{"テラスひこうの防御側には当たる", []Type{TypeNormal}, TypeFlying, false},
		{"元ひこうでもテラス中(ノーマル)なら当たらない", []Type{TypeFlying}, TypeNormal, true},
		{"元ひこう・テラスひこうは当たる", []Type{TypeFlying}, TypeFlying, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := ctrlInput([]Type{TypeNormal}, c.defTypes, CategoryPhysical, TypeDragon)
			in.Move.Priority = 1
			in.Field.Terrain = TerrainPsychic
			in.Defender.TeraType = c.tera
			r := teraCalc(t, in)
			gotBlocked := r.Nullified == NullifyPsychicTerrain
			if gotBlocked != c.blocked {
				t.Errorf("Nullified = %q, want blocked=%v", r.Nullified, c.blocked)
			}
			if c.blocked != (r.MaxDamage() == 0) {
				t.Errorf("rolls = %v, want blocked=%v", r.Rolls, c.blocked)
			}
		})
	}
}

// AC-4: T11 防御側のテラスはタイプ相性に反映しない(oracle の Champions 世代の癖。ADR-0224 §2)。
// 数値は元のタイプのまま、防御側のテラスの印は残る(数値が本編 SV と違いうることの明示。ADR-0160 の印)。
func TestTeraDefenderEffectivenessNotApplied(t *testing.T) {
	cases := []struct {
		name     string
		defTypes []Type
		tera     Type
		moveType Type
		wantEff  float64
	}{
		{"テラスゴーストでもノーマル技は等倍(無効にならない)", []Type{TypePsychic}, TypeGhost, TypeNormal, 1.0},
		{"テラスフェアリーでもドラゴン技は元タイプどおり抜群", []Type{TypeDragon}, TypeFairy, TypeDragon, 2.0},
		{"テラスひこうでもじめん技は当たる", []Type{TypeNormal}, TypeFlying, TypeGround, 1.0},
		{"テラスくさでもほのお技は等倍のまま", []Type{TypeNormal}, TypeGrass, TypeFire, 1.0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plain := ctrlInput([]Type{TypeWater}, c.defTypes, CategoryPhysical, c.moveType)
			in := plain
			in.Defender.TeraType = c.tera
			got, want := teraCalc(t, in), teraCalc(t, plain)
			if got.Effectiveness != c.wantEff || got.Effectiveness != want.Effectiveness {
				t.Errorf("Effectiveness = %v, want %v", got.Effectiveness, c.wantEff)
			}
			if !teraSameNumbers(got, want) {
				t.Errorf("防御側のテラスで数値が変わった\n got %+v\nwant %+v", got, want)
			}
			wantMarks := []UnsupportedMark{teraMark(targetDefenderTera, c.tera)}
			if !reflect.DeepEqual(got.Unsupported, wantMarks) {
				t.Errorf("Unsupported = %v, want %v", got.Unsupported, wantMarks)
			}
		})
	}
}

// AC-5: 対戦形式に依らない(ダブルでも同じ規則。ダブルの壁・全体技とは独立に掛かる)。
func TestTeraSameInDouble(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	in.Attacker.TeraType = TypeNormal
	in.Format = FormatDouble
	in.Move.Target = MoveTargetSingle
	r := teraCalc(t, in)
	if r.Rolls != teraRolls15x || !r.STAB {
		t.Errorf("ダブルの単体技: rolls=%v STAB=%v, want %v true", r.Rolls, r.STAB, teraRolls15x)
	}
	if r.Unsupported != nil {
		t.Errorf("Unsupported = %v, want nil", r.Unsupported)
	}
}

// AC-6: 一括計算は攻撃側のテラスを全行に反映する。プリセットの防御側はテラス無し(印も無い)。
// 「テラスノーマルの水タイプ」と「元からノーマルの個体」はノーマル技で同じ数値になる(接地も同じ)。
func TestTeraBulkPassesAttackerTera(t *testing.T) {
	tera := bulkInput(CategoryPhysical, TypeNormal)
	tera.Attacker.Species.Types = []Type{TypeWater}
	tera.Attacker.TeraType = TypeNormal
	same := bulkInput(CategoryPhysical, TypeNormal)
	same.Attacker.Species.Types = []Type{TypeNormal}
	plain := bulkInput(CategoryPhysical, TypeNormal)
	plain.Attacker.Species.Types = []Type{TypeWater}

	got, err := calcBulk(tera)
	if err != nil {
		t.Fatal(err)
	}
	want, err := calcBulk(same)
	if err != nil {
		t.Fatal(err)
	}
	base, err := calcBulk(plain)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) == 0 || len(got.Rows) != len(want.Rows) {
		t.Fatalf("行数 = %d, want %d(> 0)", len(got.Rows), len(want.Rows))
	}
	for i := range got.Rows {
		g, w := got.Rows[i].Result, want.Rows[i].Result
		if g.Rolls != w.Rolls || g.STAB != w.STAB || !g.STAB {
			t.Errorf("rows[%d](%s): rolls=%v STAB=%v, want %v %v", i, got.Rows[i].Preset, g.Rolls, g.STAB, w.Rolls, w.STAB)
		}
		if g.Rolls == base.Rows[i].Result.Rolls {
			t.Errorf("rows[%d](%s): テラスを付けても数値が変わらない", i, got.Rows[i].Preset)
		}
		if g.Unsupported != nil {
			t.Errorf("rows[%d].Unsupported = %v, want nil(攻撃側のテラスは反映する。プリセットの防御側はテラス無し)", i, g.Unsupported)
		}
	}
}

// AC-7: 逆算は既知側のテラスを反映し、探索側(相手)はテラス無しのまま。
//   - 相手が防御側: 既知の攻撃側のテラスが一致補正に効く(印なし)
//   - 相手が攻撃側: 既知の防御側のテラスが「そのタイプを持つか」に効き(すなあらし)、相性には効かない(印あり)
func TestTeraReversePassesKnownTera(t *testing.T) {
	base := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	rev := func(t *testing.T, side ReverseSide, known Individual, other Species, mv Move, f Field) ReverseResult {
		t.Helper()
		r, err := calcReverse(ReverseInput{
			Format: FormatSingle, Side: side, Known: known, UnknownSpecies: other,
			Move: mv, Field: f, Observations: []Observation{{Percent: 40}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Candidates) == 0 {
			t.Fatal("候補が無い")
		}
		return r
	}
	stripMarks := func(r ReverseResult) ReverseResult {
		out := r
		out.Candidates = append([]ReverseCandidate(nil), r.Candidates...)
		for i := range out.Candidates {
			out.Candidates[i].Unsupported = nil
		}
		return out
	}

	t.Run("相手が防御側: 既知の攻撃側のテラス", func(t *testing.T) {
		tera := base.Attacker
		tera.TeraType = TypeNormal
		same := base.Attacker
		same.Species.Types = []Type{TypeNormal}
		got := rev(t, SideDefender, tera, base.Defender.Species, base.Move, Field{})
		want := rev(t, SideDefender, same, base.Defender.Species, base.Move, Field{})
		plain := rev(t, SideDefender, base.Attacker, base.Defender.Species, base.Move, Field{})
		if !reflect.DeepEqual(got, want) {
			t.Errorf("テラスノーマルの結果が元ノーマルの個体と違う\n got %+v\nwant %+v", got, want)
		}
		if reflect.DeepEqual(stripMarks(got), stripMarks(plain)) {
			t.Error("既知の攻撃側のテラスが逆算に効いていない")
		}
		for _, c := range got.Candidates {
			if c.Unsupported != nil {
				t.Errorf("候補 (%s) の印 = %v, want nil", c.NatureClass, c.Unsupported)
			}
		}
	})

	t.Run("相手が攻撃側: 既知の防御側のテラス", func(t *testing.T) {
		sand := Field{Weather: WeatherSand}
		mv := Move{ID: "m", Type: TypeElectric, Category: CategorySpecial, Power: 100}
		known := mkIndiv([]Type{TypeNormal}, Stats{Def: 80, SpD: 80})
		tera := known
		tera.TeraType = TypeRock
		same := known
		same.Species.Types = []Type{TypeRock}
		got := rev(t, SideAttacker, tera, base.Attacker.Species, mv, sand)
		want := rev(t, SideAttacker, same, base.Attacker.Species, mv, sand)
		plain := rev(t, SideAttacker, known, base.Attacker.Species, mv, sand)
		if !reflect.DeepEqual(stripMarks(got), stripMarks(want)) {
			t.Errorf("テラスいわの結果が元いわの個体と違う\n got %+v\nwant %+v", got, want)
		}
		if reflect.DeepEqual(stripMarks(got), stripMarks(plain)) {
			t.Error("既知の防御側のテラスが逆算に効いていない(すなあらしの特防)")
		}
		wantMarks := []UnsupportedMark{teraMark(targetDefenderTera, TypeRock)}
		for _, c := range got.Candidates {
			if !reflect.DeepEqual(c.Unsupported, wantMarks) {
				t.Errorf("候補 (%s) の印 = %v, want %v", c.NatureClass, c.Unsupported, wantMarks)
			}
		}
	})
}
