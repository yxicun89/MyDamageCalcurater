package engine

import "testing"

// 期待値はローカル @smogon/calc 0.10.0/gen9 の統制ケースから取得。
// A/C=200、B/D=100、HP=175、Lv50、IV31、EV0、Serious、威力100。
func TestModifierRoundingRegression(t *testing.T) {
	terrain := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeElectric)
	terrain.Field.Terrain = TerrainElectric
	tests := []struct {
		name        string
		input       DamageInput
		index, want int
	}{
		{"STAB before effectiveness", ctrlInput([]Type{TypeWater}, []Type{TypeRock}, CategoryPhysical, TypeWater), 1, 230},
		{"terrain power", terrain, 15, 116},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calcDamage(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Rolls[tt.index] != tt.want {
				t.Fatalf("roll[%d]=%d want %d", tt.index, got.Rolls[tt.index], tt.want)
			}
		})
	}
}

func TestWeatherBeforeItemRounding(t *testing.T) {
	in := ctrlInput([]Type{TypeWater}, []Type{TypeRock}, CategorySpecial, TypePsychic)
	in.Defender.Species.BaseStats.SpD = 81 // 実数値101 → 砂151 → 特防1.5倍の持ち物226
	in.Move.Power = 108
	in.Field.Weather = WeatherSand
	in.Defender.Item = &Item{Effect: &ItemEffect{StatMods: map[StatKey]int{StatSpD: 6144}}}
	got, err := calcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.MaxDamage() != 44 {
		t.Fatalf("max=%d want 44", got.MaxDamage())
	}
}

func TestCalcDamageRejectsInvalidLevelAndStats(t *testing.T) {
	for _, level := range []int{-1, 1, 49, 51, 100} {
		in := ctrlInput([]Type{TypeWater}, []Type{TypeRock}, CategoryPhysical, TypeNormal)
		in.Attacker.Level = level
		if _, err := calcDamage(in); err == nil {
			t.Errorf("accepted level=%d", level)
		}
	}
	in := ctrlInput([]Type{TypeWater}, []Type{TypeRock}, CategoryPhysical, TypeNormal)
	in.Defender.Species.BaseStats.Def = -20
	if _, err := calcDamage(in); err == nil {
		t.Error("accepted negative base stat")
	}
}

// TestChainModsRoundHalfUp は補正の連結の各ステップが (M*mod + 2048) >> 12(0.5 を切り上げる丸め)で
// あることを固定する(@smogon/calc の chainMods と同じ。issue #303)。切り捨てに変わると golden が無くても落ちる。
func TestChainModsRoundHalfUp(t *testing.T) {
	tests := []struct {
		name string
		mods []int
		want int
	}{
		{"empty is identity", nil, Modifier4096},
		{"4096 is skipped", []int{Modifier4096, 5324}, 5324},
		// 4915×5324/4096 = 6388.54… → 6389(切り捨てなら 6388)
		{"fraction above half rounds up", []int{4915, 5324}, 6389},
		// 2048×6145/4096 = 3072.5 ちょうど → 3073(連結は半分ちょうども切り上げる。pokeRound とは逆)
		{"exact half rounds up", []int{ModifierHalf, 6145}, 3073},
		// 5324×5324/4096 = 6920.16… → 6920
		{"fraction below half rounds down", []int{5324, 5324}, 6920},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := chainMods(tt.mods, finalModBounds); got != tt.want {
				t.Fatalf("chainMods(%v)=%d want %d", tt.mods, got, tt.want)
			}
		})
	}
}

// TestChainModsClampsLikeOracle は連結した補正を @smogon/calc 0.12.0 と同じ範囲にクランプすることを固定する
// (issue #77。oracle は最終補正 41..131072、威力 41..2097152、攻撃・防御の実数値 410..131072)。
// 現在のマスタの補正集合では範囲外に届かないが、補正を足したときに黙って乖離しないようにする。
func TestChainModsClampsLikeOracle(t *testing.T) {
	const x2 = 2 * Modifier4096
	sixDoubles := []int{x2, x2, x2, x2, x2, x2} // 4096×64 = 262144
	tests := []struct {
		name   string
		mods   []int
		bounds modBounds
		want   int
	}{
		{"final upper", sixDoubles, finalModBounds, 131072},
		{"final lower", []int{1}, finalModBounds, 41},
		{"final lower edge kept", []int{41}, finalModBounds, 41},
		{"power upper not reached", sixDoubles, powerModBounds, 262144},
		{"power upper", append(append([]int{}, sixDoubles...), x2, x2, x2, x2), powerModBounds, 2097152},
		{"power lower", []int{1}, powerModBounds, 41},
		{"stat upper", sixDoubles, statModBounds, 131072},
		{"stat lower", []int{409}, statModBounds, 410},
		{"stat lower edge kept", []int{410}, statModBounds, 410},
		{"in range unchanged", []int{6144}, statModBounds, 6144},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := chainMods(tt.mods, tt.bounds); got != tt.want {
				t.Fatalf("chainMods(%v, %+v)=%d want %d", tt.mods, tt.bounds, got, tt.want)
			}
		})
	}
}

// TestChainModsClampAtCallSites は各段階が正しい範囲でクランプすることを計算結果で固定する(issue #77)。
// 統制ケース(実 A200 / B100 / 威力100、等倍・一致なし)の基本式 floor(floor(22×威力×A/B)/50)+2 で期待値を求める。
func TestChainModsClampAtCallSites(t *testing.T) {
	// 攻撃実数値: 補正1 → 下限410 → pokeRound(200,410)=20 → floor(22×100×20/100/50)+2 = 10(クランプ無しなら A=1 で 2)
	atkLow := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	atkLow.Attacker.Item = &Item{Effect: &ItemEffect{StatMods: map[StatKey]int{StatAtk: 1}}}
	// 威力: 補正1 → 下限41 → pokeRound(250,41)=3 → floor(22×3×200/100/50)+2 = 4(クランプ無しなら威力1で 2)
	powLow := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	powLow.Move.Power = 250
	powLow.Attacker.Item = &Item{Effect: &ItemEffect{PowerMod: 1}}
	// 最終補正: ×256 → 上限131072(×32) → 90×32 = 2880(クランプ無しなら 23040)
	finalHigh := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	finalHigh.Attacker.Item = &Item{Effect: &ItemEffect{DamageMod: 256 * Modifier4096}}
	tests := []struct {
		name  string
		input DamageInput
		want  int
	}{
		{"attack stat lower bound", atkLow, 10},
		{"power lower bound", powLow, 4},
		{"final modifier upper bound", finalHigh, 2880},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calcDamage(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Rolls[15] != tt.want {
				t.Fatalf("rolls[15]=%d want %d", got.Rolls[15], tt.want)
			}
		})
	}
}
