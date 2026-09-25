package engine

// タイプ相性表を「入力」として受け取ることの検証(ADR-0013 §P1-13.2/.3)。
//
//   - CalcDamage は入力の表だけを見る(engine に埋め込まれた表を見ない)
//   - 表が無ければ計算せずに ErrTypeChartMissing
//   - 表に無いタイプは ErrUnknownType(黙って等倍にしない)
//   - CalcBulk / CalcReverse は表を解釈せず DamageInput へ素通しする
//
// 表の中身に依存しないよう、実在しないタイプ ID(alpha/beta/gamma)で組む。
// 統制ケースの素のロールは damage_test.go の ctrlInput と同じ 76..90(等倍・タイプ一致なし)。

import (
	"errors"
	"testing"
)

// ctrlNeutralMin / ctrlNeutralMax は ctrlInput の等倍・タイプ一致なしの下限・上限
// (TestCalcDamageBaseNeutralNoSTAB と同じ値)。相性は d*Num/Den の整数演算で掛かる。
const (
	ctrlNeutralMin = 76
	ctrlNeutralMax = 90
)

// chartForCode は「alpha → beta が code」だけを定めた表を作る(他は等倍)。
func chartForCode(t *testing.T, code int) TypeChart {
	t.Helper()
	data := TypeChartData{Types: []Type{typeAlpha, typeBeta, typeGamma}}
	if code != TypeCodeNeutral {
		data.Effectiveness = map[Type]map[Type]int{typeAlpha: {typeBeta: code}}
	}
	c, err := NewTypeChart(data)
	if err != nil {
		t.Fatalf("NewTypeChart(code=%d): %v", code, err)
	}
	return c
}

// ctrlFictional は「gamma の個体が alpha 技で beta の個体を殴る」統制ケース。
func ctrlFictional() DamageInput {
	return ctrlInput([]Type{typeGamma}, []Type{typeBeta}, CategoryPhysical, typeAlpha)
}

// --- AC-1 / AC-3: 相性は入力の表だけで決まる --------------------------------

func TestCalcDamageUsesSuppliedTypeChart(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		min, max int
		mult     float64
	}{
		{"表が抜群と言えば抜群", TypeCodeSuperEffective, ctrlNeutralMin * 2, ctrlNeutralMax * 2, 2.0},
		{"表が等倍と言えば等倍", TypeCodeNeutral, ctrlNeutralMin, ctrlNeutralMax, 1.0},
		{"表がいまひとつと言えばいまひとつ", TypeCodeNotVeryEffective, ctrlNeutralMin / 2, ctrlNeutralMax / 2, 0.5},
		{"表が無効と言えば無効", TypeCodeImmune, 0, 0, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := ctrlFictional()
			in.TypeChart = chartForCode(t, tt.code)
			r, err := CalcDamage(in)
			if err != nil {
				t.Fatalf("CalcDamage: %v", err)
			}
			if r.Effectiveness != tt.mult {
				t.Errorf("Effectiveness = %v, want %v", r.Effectiveness, tt.mult)
			}
			if r.MinDamage() != tt.min || r.MaxDamage() != tt.max {
				t.Errorf("ロール = %d..%d, want %d..%d", r.MinDamage(), r.MaxDamage(), tt.min, tt.max)
			}
		})
	}
}

// TestCalcDamageSuperEffectiveModifiersFollowChart は、抜群かどうかで効く補正
// (抜群のときだけ効く持ち物・抜群を軽減する特性・半減きのみ)が、埋め込みの表ではなく
// 入力の表の判定に従うことを確かめる(modifiers.go の superEffective)。
func TestCalcDamageSuperEffectiveModifiersFollowChart(t *testing.T) {
	// 抜群のときだけ最終ダメージを上げる持ち物(効果はマスタから解決済みの体で与える)。
	belt := &Item{ID: "testbelt", Effect: &ItemEffect{DamageMod: 4915, OnlySuperEffective: true}}

	superEffective := ctrlFictional()
	superEffective.Attacker.Item = belt
	superEffective.TypeChart = chartForCode(t, TypeCodeSuperEffective)
	withBelt, err := CalcDamage(superEffective)
	if err != nil {
		t.Fatal(err)
	}

	neutral := ctrlFictional()
	neutral.Attacker.Item = belt
	neutral.TypeChart = chartForCode(t, TypeCodeNeutral)
	noBelt, err := CalcDamage(neutral)
	if err != nil {
		t.Fatal(err)
	}

	// 抜群(×2)側は持ち物も乗るので、等倍側の2倍より大きくなる。
	if got, want := withBelt.MaxDamage(), noBelt.MaxDamage()*2; got <= want {
		t.Errorf("抜群時の最大ダメージ = %d、等倍の2倍 = %d。抜群限定の持ち物が入力の表で判定されていない", got, want)
	}
	// 等倍側では抜群限定の補正が乗らない(素の統制値のまま)。
	if noBelt.MaxDamage() != ctrlNeutralMax {
		t.Errorf("等倍時の最大ダメージ = %d, want %d(抜群限定の補正が乗ってはいけない)", noBelt.MaxDamage(), ctrlNeutralMax)
	}
}

// --- AC-4: 表が無ければ計算しない -------------------------------------------

func TestCalcDamageRequiresTypeChart(t *testing.T) {
	in := ctrlFictional() // TypeChart はゼロ値
	if _, err := CalcDamage(in); !errors.Is(err, ErrTypeChartMissing) {
		t.Fatalf("err = %v, want ErrTypeChartMissing(表が無いまま計算しない)", err)
	}
	// 変化技(ダメージ0で早期に返る経路)でも同じ。
	status := in
	status.Move.Category = CategoryStatus
	if _, err := CalcDamage(status); !errors.Is(err, ErrTypeChartMissing) {
		t.Errorf("変化技の err = %v, want ErrTypeChartMissing", err)
	}
}

func TestCalcBulkRequiresTypeChart(t *testing.T) {
	in := BulkInput{
		Format:          FormatSingle,
		Attacker:        mkIndiv([]Type{typeGamma}, Stats{Atk: 180, SpA: 180}),
		DefenderSpecies: Species{Key: "0999-000", Types: []Type{typeBeta}, BaseStats: Stats{HP: 100, Atk: 100, Def: 80, SpA: 100, SpD: 80, Spe: 100}},
		Move:            Move{ID: "m", Type: typeAlpha, Category: CategoryPhysical, Power: 100},
	}
	if _, err := CalcBulk(in); !errors.Is(err, ErrTypeChartMissing) {
		t.Fatalf("err = %v, want ErrTypeChartMissing", err)
	}
}

func TestCalcReverseRequiresTypeChart(t *testing.T) {
	in := ReverseInput{
		Format:         FormatSingle,
		Side:           SideDefender,
		Known:          mkIndiv([]Type{typeGamma}, Stats{Atk: 180, SpA: 180}),
		UnknownSpecies: Species{Key: "0997-000", Types: []Type{typeBeta}, BaseStats: Stats{HP: 100, Atk: 100, Def: 80, SpA: 100, SpD: 80, Spe: 100}},
		Move:           Move{ID: "m", Type: typeAlpha, Category: CategoryPhysical, Power: 100},
		Observations:   []Observation{{Percent: 40}},
	}
	if _, err := CalcReverse(in); !errors.Is(err, ErrTypeChartMissing) {
		t.Fatalf("err = %v, want ErrTypeChartMissing", err)
	}
}

// --- AC-5: 表に無いタイプはエラー -------------------------------------------

func TestCalcDamageRejectsUnknownType(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*DamageInput)
	}{
		{"技のタイプが表に無い", func(in *DamageInput) { in.Move.Type = typeEpsilon }},
		{"防御側の種族タイプが表に無い", func(in *DamageInput) {
			in.Defender.Species.Types = []Type{typeEpsilon}
		}},
		{"防御側の2つ目のタイプが表に無い", func(in *DamageInput) {
			in.Defender.Species.Types = []Type{typeBeta, typeEpsilon}
		}},
		{"攻撃側の種族タイプが表に無い", func(in *DamageInput) {
			in.Attacker.Species.Types = []Type{typeEpsilon}
		}},
		{"攻撃側のテラスタイプが表に無い", func(in *DamageInput) { in.Attacker.TeraType = typeEpsilon }},
		{"防御側のテラスタイプが表に無い", func(in *DamageInput) { in.Defender.TeraType = typeEpsilon }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := ctrlFictional()
			in.TypeChart = chartForCode(t, TypeCodeSuperEffective)
			tt.mutate(&in)
			if _, err := CalcDamage(in); !errors.Is(err, ErrUnknownType) {
				t.Fatalf("err = %v, want ErrUnknownType(等倍にフォールバックしない)", err)
			}
		})
	}
}

// --- AC-7: BulkInput / ReverseInput は表を素通しする -------------------------

func TestCalcBulkPassesTypeChartThrough(t *testing.T) {
	newIn := func(code int) BulkInput {
		return BulkInput{
			Format:          FormatSingle,
			Attacker:        mkIndiv([]Type{typeGamma}, Stats{Atk: 180, SpA: 180}),
			DefenderSpecies: Species{Key: "0999-000", Types: []Type{typeBeta}, BaseStats: Stats{HP: 100, Atk: 100, Def: 80, SpA: 100, SpD: 80, Spe: 100}},
			Move:            Move{ID: "m", Type: typeAlpha, Category: CategoryPhysical, Power: 100},
			TypeChart:       chartForCode(t, code),
		}
	}
	superEffective, err := CalcBulk(newIn(TypeCodeSuperEffective))
	if err != nil {
		t.Fatalf("CalcBulk(抜群): %v", err)
	}
	neutral, err := CalcBulk(newIn(TypeCodeNeutral))
	if err != nil {
		t.Fatalf("CalcBulk(等倍): %v", err)
	}
	if len(superEffective.Rows) == 0 || len(superEffective.Rows) != len(neutral.Rows) {
		t.Fatalf("行数 = %d / %d", len(superEffective.Rows), len(neutral.Rows))
	}
	for i := range superEffective.Rows {
		se, ne := superEffective.Rows[i], neutral.Rows[i]
		if se.Result.Effectiveness != 2.0 || ne.Result.Effectiveness != 1.0 {
			t.Errorf("行 %d の相性 = %v / %v, want 2.0 / 1.0(表が素通しされていない)", i, se.Result.Effectiveness, ne.Result.Effectiveness)
		}
		if se.Result.MaxDamage() != ne.Result.MaxDamage()*2 {
			t.Errorf("行 %d の最大ダメージ = %d, want %d", i, se.Result.MaxDamage(), ne.Result.MaxDamage()*2)
		}
	}

	// 表に無いタイプは、行の計算まで届いてエラーになる(CalcBulk が自分で等倍にしない)。
	unknown := newIn(TypeCodeSuperEffective)
	unknown.DefenderSpecies.Types = []Type{typeEpsilon}
	if _, err := CalcBulk(unknown); !errors.Is(err, ErrUnknownType) {
		t.Errorf("err = %v, want ErrUnknownType", err)
	}
}

func TestCalcReversePassesTypeChartThrough(t *testing.T) {
	newIn := func(code int) ReverseInput {
		return ReverseInput{
			Format:         FormatSingle,
			Side:           SideDefender,
			Known:          mkIndiv([]Type{typeGamma}, Stats{Atk: 180, SpA: 180}),
			UnknownSpecies: Species{Key: "0997-000", Types: []Type{typeBeta}, BaseStats: Stats{HP: 100, Atk: 100, Def: 80, SpA: 100, SpD: 80, Spe: 100}},
			Move:           Move{ID: "m", Type: typeAlpha, Category: CategoryPhysical, Power: 100},
			Observations:   []Observation{{Percent: 40}},
			TypeChart:      chartForCode(t, code),
		}
	}
	superEffective, err := CalcReverse(newIn(TypeCodeSuperEffective))
	if err != nil {
		t.Fatalf("CalcReverse(抜群): %v", err)
	}
	neutral, err := CalcReverse(newIn(TypeCodeNeutral))
	if err != nil {
		t.Fatalf("CalcReverse(等倍): %v", err)
	}
	if n := len(superEffective.Candidates); n == 0 || n != len(neutral.Candidates) {
		t.Fatalf("候補数 = %d / %d(型カタログは表に依らない)", n, len(neutral.Candidates))
	}
	// 同じ観測でも、表が違えば格子点の想定ダメージ幅が変わる。
	// どの候補も同じなら、表は CalcDamage まで届いていない。
	differs := false
	for i, se := range superEffective.Candidates {
		ne := neutral.Candidates[i]
		if se.MinPercentTenths != ne.MinPercentTenths || se.MaxPercentTenths != ne.MaxPercentTenths {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("抜群と等倍で候補の想定ダメージ幅が一つも変わらない。表が DamageInput へ素通しされていない")
	}

	// 表に無いタイプは格子の計算まで届いてエラーになる。
	unknown := newIn(TypeCodeSuperEffective)
	unknown.UnknownSpecies.Types = []Type{typeEpsilon}
	if _, err := CalcReverse(unknown); !errors.Is(err, ErrUnknownType) {
		t.Errorf("err = %v, want ErrUnknownType", err)
	}
}
