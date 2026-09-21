package engine

// タイプ相性(ADR-0013)。表は**データ**、引き方は**ルール**。
//
//   - 引き方のルール(複合タイプ・無効・等倍の省略・TypeNone・未知タイプ)は、
//     実在しないタイプ ID の小さな独立 fixture で確かめる(smallTypeChartData)。
//     engine が「18 タイプ」を知らないこと自体の検証でもある。
//   - 表の中身(火は草に抜群、など)は oracle 由来の fixture 経由で確かめる。
//     期待値は P1-13 以前の TestTypeEffectivenessSingle / Dual と同じ(データ化で結果は変わらない)。

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// --- AC-2: 引き方のルール(独立 fixture)------------------------------------

func TestTypeChartEffectivenessRules(t *testing.T) {
	chart := smallTypeChart(t)
	tests := []struct {
		name     string
		atk      Type
		def      []Type
		num, den int
		mult     float64
	}{
		{"単一・抜群", typeAlpha, []Type{typeBeta}, 4, 2, 2.0},
		{"単一・いまひとつ", typeBeta, []Type{typeGamma}, 1, 2, 0.5},
		{"単一・無効", typeAlpha, []Type{typeDelta}, 0, 2, 0.0},
		{"単一・等倍(行ごと省略)", typeGamma, []Type{typeAlpha}, 2, 2, 1.0},
		{"単一・等倍(組だけ省略)", typeBeta, []Type{typeAlpha}, 2, 2, 1.0},
		{"複合・4倍", typeAlpha, []Type{typeBeta, typeGamma}, 16, 4, 4.0},
		{"複合・1/4倍", typeBeta, []Type{typeBeta, typeGamma}, 1, 4, 0.25},
		{"複合・無効が優先", typeAlpha, []Type{typeDelta, typeBeta}, 0, 4, 0.0},
		{"複合・抜群と半減が打ち消す", typeDelta, []Type{typeBeta, typeAlpha}, 4, 4, 1.0},
		{"複合・両方とも省略", typeGamma, []Type{typeAlpha, typeBeta}, 4, 4, 1.0},
		{"防御タイプなし", typeAlpha, nil, 1, 1, 1.0},
		{"防御タイプが TypeNone だけ", typeAlpha, []Type{TypeNone}, 1, 1, 1.0},
		{"防御タイプの TypeNone は飛ばす", typeAlpha, []Type{typeBeta, TypeNone}, 4, 2, 2.0},
		{"攻撃タイプが TypeNone", TypeNone, []Type{typeBeta}, 1, 1, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := chart.Effectiveness(tt.atk, tt.def)
			if err != nil {
				t.Fatalf("Effectiveness(%q, %v): %v", tt.atk, tt.def, err)
			}
			// 倍率は整数比のまま(float に潰さない)。
			if got.Num != tt.num || got.Den != tt.den {
				t.Errorf("Num/Den = %d/%d, want %d/%d", got.Num, got.Den, tt.num, tt.den)
			}
			if got.Multiplier() != tt.mult {
				t.Errorf("Multiplier() = %v, want %v", got.Multiplier(), tt.mult)
			}
			if got.IsImmune() != (tt.mult == 0) {
				t.Errorf("IsImmune() = %v, want %v", got.IsImmune(), tt.mult == 0)
			}
			if got.IsSuperEffective() != (tt.mult > 1.0) {
				t.Errorf("IsSuperEffective() = %v, want %v", got.IsSuperEffective(), tt.mult > 1.0)
			}
		})
	}
}

func TestTypeChartCodeAndNone(t *testing.T) {
	chart := smallTypeChart(t)
	tests := []struct {
		atk, def Type
		want     int
	}{
		{typeAlpha, typeBeta, TypeCodeSuperEffective},
		{typeAlpha, typeDelta, TypeCodeImmune},
		{typeBeta, typeGamma, TypeCodeNotVeryEffective},
		{typeBeta, typeAlpha, TypeCodeNeutral},  // 組の省略は等倍
		{typeGamma, typeDelta, TypeCodeNeutral}, // 行ごとの省略も等倍
		{TypeNone, typeBeta, TypeCodeNeutral},
		{typeAlpha, TypeNone, TypeCodeNeutral},
		{TypeNone, TypeNone, TypeCodeNeutral},
	}
	for _, tt := range tests {
		got, err := chart.Code(tt.atk, tt.def)
		if err != nil {
			t.Fatalf("Code(%q, %q): %v", tt.atk, tt.def, err)
		}
		if got != tt.want {
			t.Errorf("Code(%q, %q) = %d, want %d", tt.atk, tt.def, got, tt.want)
		}
	}

	if !chart.Has(typeAlpha) {
		t.Error("Has(alpha) = false, want true")
	}
	if chart.Has(typeEpsilon) {
		t.Error("Has(epsilon) = true, want false(表に無い)")
	}
	if chart.Has(TypeNone) {
		t.Error("Has(TypeNone) = true, want false")
	}
	if chart.IsZero() {
		t.Error("IsZero() = true, want false(検証済みの表)")
	}
}

// --- AC-5: 未知タイプは等倍にせずエラー -------------------------------------

func TestTypeChartRejectsUnknownType(t *testing.T) {
	chart := smallTypeChart(t)
	tests := []struct {
		name string
		call func() error
	}{
		{"攻撃タイプが未知", func() error { _, err := chart.Effectiveness(typeEpsilon, []Type{typeBeta}); return err }},
		{"防御タイプが未知", func() error { _, err := chart.Effectiveness(typeAlpha, []Type{typeEpsilon}); return err }},
		{"複合の2つ目が未知", func() error { _, err := chart.Effectiveness(typeAlpha, []Type{typeBeta, typeEpsilon}); return err }},
		{"Code の攻撃側が未知", func() error { _, err := chart.Code(typeEpsilon, typeBeta); return err }},
		{"Code の防御側が未知", func() error { _, err := chart.Code(typeAlpha, typeEpsilon); return err }},
		// 実在するタイプ名でも、渡された表に無ければ未知。
		{"別世代のタイプ名でも表に無ければ未知", func() error { _, err := chart.Code(TypeFire, typeBeta); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if !errors.Is(err, ErrUnknownType) {
				t.Fatalf("err = %v, want ErrUnknownType(黙って等倍にしない)", err)
			}
		})
	}
}

// --- AC-4: 表が未設定なら必ず鳴る -------------------------------------------

func TestZeroTypeChartIsMissing(t *testing.T) {
	var zero TypeChart
	if !zero.IsZero() {
		t.Fatal("ゼロ値の IsZero() = false, want true")
	}
	if got := zero.Types(); len(got) != 0 {
		t.Errorf("ゼロ値の Types() = %v, want 空", got)
	}
	if _, err := zero.Effectiveness(typeAlpha, []Type{typeBeta}); !errors.Is(err, ErrTypeChartMissing) {
		t.Errorf("Effectiveness の err = %v, want ErrTypeChartMissing", err)
	}
	// TypeNone だけの問い合わせでも「表が無い」が先に鳴る(等倍を返さない)。
	if _, err := zero.Effectiveness(TypeNone, nil); !errors.Is(err, ErrTypeChartMissing) {
		t.Errorf("TypeNone の Effectiveness の err = %v, want ErrTypeChartMissing", err)
	}
	if _, err := zero.Code(typeAlpha, typeBeta); !errors.Is(err, ErrTypeChartMissing) {
		t.Errorf("Code の err = %v, want ErrTypeChartMissing", err)
	}
}

// --- AC-6: 定義の検証と不変性 ------------------------------------------------

func TestNewTypeChartRejectsInvalidData(t *testing.T) {
	tests := []struct {
		name string
		data TypeChartData
	}{
		{"Types が空", TypeChartData{}},
		{"Types が nil で Effectiveness だけある", TypeChartData{
			Effectiveness: map[Type]map[Type]int{typeAlpha: {typeBeta: TypeCodeSuperEffective}},
		}},
		{"Types が重複", TypeChartData{Types: []Type{typeAlpha, typeAlpha}}},
		{"Types に TypeNone", TypeChartData{Types: []Type{typeAlpha, TypeNone}}},
		{"攻撃側のキーが Types に無い", TypeChartData{
			Types:         []Type{typeAlpha},
			Effectiveness: map[Type]map[Type]int{typeEpsilon: {typeAlpha: TypeCodeSuperEffective}},
		}},
		{"防御側のキーが Types に無い", TypeChartData{
			Types:         []Type{typeAlpha},
			Effectiveness: map[Type]map[Type]int{typeAlpha: {typeEpsilon: TypeCodeSuperEffective}},
		}},
		{"キーが TypeNone", TypeChartData{
			Types:         []Type{typeAlpha},
			Effectiveness: map[Type]map[Type]int{TypeNone: {typeAlpha: TypeCodeNeutral}},
		}},
		{"コードが 3(定義に無い倍率)", TypeChartData{
			Types:         []Type{typeAlpha},
			Effectiveness: map[Type]map[Type]int{typeAlpha: {typeAlpha: 3}},
		}},
		{"コードが負", TypeChartData{
			Types:         []Type{typeAlpha},
			Effectiveness: map[Type]map[Type]int{typeAlpha: {typeAlpha: -1}},
		}},
		{"コードが 8(未知の倍率)", TypeChartData{
			Types:         []Type{typeAlpha},
			Effectiveness: map[Type]map[Type]int{typeAlpha: {typeAlpha: 8}},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTypeChart(tt.data)
			if !errors.Is(err, ErrInvalidTypeChart) {
				t.Fatalf("err = %v, want ErrInvalidTypeChart", err)
			}
			if !got.IsZero() {
				t.Error("失敗時はゼロ値の表を返すこと(半端な表を使わせない)")
			}
		})
	}
}

func TestTypeChartIsImmutable(t *testing.T) {
	data := smallTypeChartData()
	chart, err := NewTypeChart(data)
	if err != nil {
		t.Fatalf("NewTypeChart: %v", err)
	}
	before, err := chart.Code(typeAlpha, typeBeta)
	if err != nil {
		t.Fatal(err)
	}

	// 渡した素データを後から壊しても、検証済みの表は変わらない。
	data.Effectiveness[typeAlpha][typeBeta] = TypeCodeImmune
	data.Effectiveness[typeGamma] = map[Type]int{typeAlpha: TypeCodeSuperEffective}
	data.Types[0] = typeEpsilon

	after, err := chart.Code(typeAlpha, typeBeta)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Errorf("素データの書き換えで表が変わった: %d → %d", before, after)
	}
	if c, err := chart.Code(typeGamma, typeAlpha); err != nil || c != TypeCodeNeutral {
		t.Errorf("Code(gamma, alpha) = %d, %v; want %d, nil", c, err, TypeCodeNeutral)
	}

	// Types() も呼び出し側に壊されない。
	got := chart.Types()
	if len(got) != len(smallTypeChartData().Types) {
		t.Fatalf("Types() = %v", got)
	}
	got[0] = typeEpsilon
	if again := chart.Types(); again[0] == typeEpsilon {
		t.Error("Types() の返り値を書き換えると表が変わる(コピーを返すこと)")
	}
}

// --- AC-3: 表の中身(oracle 由来の fixture)。期待値は P1-13 以前と同じ -------

// TestTypeChartFixtureMatchesKnownMatchups は、データ化しても相性の答えが
// 変わらないことを確かめる。期待値は P1-13 以前の TestTypeEffectivenessSingle /
// TestTypeEffectivenessDual と同一(**弱めない・変えない**。CLAUDE.md 絶対ルール6)。
func TestTypeChartFixtureMatchesKnownMatchups(t *testing.T) {
	chart := mustTypeChart(t)
	tests := []struct {
		atk  Type
		def  []Type
		want float64
	}{
		// 単一
		{TypeFire, []Type{TypeGrass}, 2.0},
		{TypeFire, []Type{TypeWater}, 0.5},
		{TypeNormal, []Type{TypeGhost}, 0.0},
		{TypeGround, []Type{TypeFlying}, 0.0},
		{TypeElectric, []Type{TypeGround}, 0.0},
		{TypeDragon, []Type{TypeFairy}, 0.0},
		{TypeWater, []Type{TypeFire}, 2.0},
		{TypeNormal, []Type{TypePsychic}, 1.0},
		// 複合
		{TypeWater, []Type{TypeRock, TypeGround}, 4.0}, // 水×2×2
		{TypeFire, []Type{TypeWater, TypeRock}, 0.25},  // 火 vs 水0.5×岩0.5
		{TypeGrass, []Type{TypeWater, TypeGround}, 4.0},
		{TypeIce, []Type{TypeDragon, TypeGround}, 4.0},
		{TypeNormal, []Type{TypeRock, TypeGhost}, 0.0}, // ゴースト無効が優先
		{TypeFighting, []Type{TypeNormal, TypeFlying}, 1.0},
	}
	for _, tt := range tests {
		got, err := chart.Effectiveness(tt.atk, tt.def)
		if err != nil {
			t.Fatalf("Effectiveness(%s, %v): %v", tt.atk, tt.def, err)
		}
		if got.Multiplier() != tt.want {
			t.Errorf("%s vs %v = %v want %v", tt.atk, tt.def, got.Multiplier(), tt.want)
		}
	}
}

// TestTypeChartFixtureCoversRuleTypes は、engine のルールが名指しで参照するタイプ
// (天候・フィールド・半減きのみ)が表に載っていることを確かめる。
// マスタ側からこれらが消えたら、ルールは動くのに相性だけ引けない状態になる。
func TestTypeChartFixtureCoversRuleTypes(t *testing.T) {
	chart := mustTypeChart(t)
	// engine/modifiers.go のルールが参照するタイプ(機構=コード側。ADR-0013 §2)。
	ruleTypes := []Type{
		TypeFire, TypeWater, TypeElectric, TypeGrass, TypeIce,
		TypeRock, TypePsychic, TypeDragon, TypeNormal,
	}
	for _, ty := range ruleTypes {
		if !chart.Has(ty) {
			t.Errorf("ルールが参照するタイプ %q が表に無い", ty)
		}
	}
	if n := len(chart.Types()); n != typeChartTypeCount {
		t.Errorf("表のタイプ数 = %d, want %d", n, typeChartTypeCount)
	}
}

// --- AC-1: engine のコードに表が残っていないこと -----------------------------

// TestNoTypeChartTableInEngineSource は、相性表(18×18 の値)が engine の
// コードに書き戻されていないことを確かめる(ADR-0013 §決定1)。
//
// 判定の根拠: 機構(天候・フィールド・半減きのみ)が名指しするタイプだけがコードに残る。
// 下の9タイプはどのルールも名指ししないので、非テストのソースに出てきたら
// それは相性表(かそれに類する一覧)がコードに入ったということ。
func TestNoTypeChartTableInEngineSource(t *testing.T) {
	// ルールが参照しないタイプの定数名。types.go(定数の定義そのもの)は対象外。
	forbidden := []string{
		"TypeFighting", "TypePoison", "TypeGround", "TypeFlying", "TypeBug",
		"TypeGhost", "TypeDark", "TypeSteel", "TypeFairy",
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "types.go" {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(src)
		for _, id := range forbidden {
			if strings.Contains(text, id) {
				t.Errorf("%s が %s を参照している。タイプ相性表は入力(TypeChart)から引くこと(ADR-0013)", name, id)
			}
		}
		// 表を前提にしたパッケージ関数は残さない(暗黙の表に戻る道を塞ぐ)。
		if strings.Contains(text, "func TypeEffectiveness(") {
			t.Errorf("%s に TypeEffectiveness が残っている。TypeChart.Effectiveness に置き換えること", name)
		}
	}
}
