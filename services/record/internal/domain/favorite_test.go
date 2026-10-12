package domain

// SP・レベル・ランクの上限の検証が domain にあり、engine の定数を正にしていることの検査(ADR-0254。G6)。
// 応答の code・文言は adapters/httpapi のテスト(favorites_test.go ほか)が HTTP 越しに固定している。

import (
	"errors"
	"testing"

	"example.com/pokecalc/engine"
)

// testEnums はどの値も契約の列挙に含まれる扱いにする(列挙の判定は adapters の責務なので、ここでは範囲だけを見る)。
var testEnums = Enums{
	Status: anyValue, Type: anyValue, Format: anyValue, Weather: anyValue, Terrain: anyValue,
	StatusNone: "none", WeatherNone: "none", TerrainNone: "none",
}

func anyValue(string) bool { return true }

func ptr[T any](v T) *T { return &v }

func individual(sp SPInput, level *int, ranks *RankInput) *IndividualInput {
	return &IndividualInput{SpeciesKey: ptr("9001-000"), NatureID: ptr("n"), Level: level, Sp: &sp, Ranks: ranks}
}

func sp(hp, atk int) SPInput {
	return SPInput{Hp: ptr(hp), Atk: ptr(atk), Def: ptr(0), Spa: ptr(0), Spd: ptr(0), Spe: ptr(0)}
}

// spTotal は合計が total になるよう、1能力の上限まで前から詰める。
func spTotal(total int) SPInput {
	var v [6]int
	for i := range v {
		v[i] = min(total, engine.MaxSPPerStat)
		total -= v[i]
	}
	return SPInput{Hp: ptr(v[0]), Atk: ptr(v[1]), Def: ptr(v[2]), Spa: ptr(v[3]), Spd: ptr(v[4]), Spe: ptr(v[5])}
}

func TestNormalizeIndividualLimits(t *testing.T) {
	tests := []struct {
		name string
		in   *IndividualInput
		ok   bool
	}{
		{"1能力の上限ちょうど", individual(sp(engine.MaxSPPerStat, 0), nil, nil), true},
		{"1能力の上限を超える", individual(sp(engine.MaxSPPerStat+1, 0), nil, nil), false},
		{"負の SP", individual(sp(-1, 0), nil, nil), false},
		{"合計の上限ちょうど", individual(spTotal(engine.MaxSPTotal), nil, nil), true},
		{"合計の上限を超える", individual(spTotal(engine.MaxSPTotal+1), nil, nil), false},
		{"レベルは既定のみ", individual(sp(0, 0), ptr(engine.DefaultLevel), nil), true},
		{"既定以外のレベル", individual(sp(0, 0), ptr(engine.DefaultLevel-1), nil), false},
		{"ランク +6", individual(sp(0, 0), nil, &RankInput{Atk: ptr(6)}), true},
		{"ランク -7", individual(sp(0, 0), nil, &RankInput{Spe: ptr(-7)}), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeIndividual(tt.in, testEnums)
			if tt.ok {
				if err != nil {
					t.Fatalf("通るはずが失敗: %v", err)
				}
				if got.Level != engine.DefaultLevel {
					t.Errorf("level = %d, want %d", got.Level, engine.DefaultLevel)
				}
				return
			}
			var de *Error
			if !errors.As(err, &de) || de.Code != CodeInvalidInput {
				t.Fatalf("invalid_input の domain.Error のはずが %v", err)
			}
		})
	}
}
