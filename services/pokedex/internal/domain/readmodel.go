package domain

// read model(pokedex export。ADR-0105 §5・ADR-0106 §決定7・ADR-0145)の純粋な部分: 特性の効果を
// 防御側のタイプ相性の効果だけに正規化する。

import (
	"errors"
	"fmt"
	"sort"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

// ErrInvalidExport は出力として成立しない(使用可能集合が空・効果定義が検証を通らない・
// 係数が 1〜16 の比にならない)。
var ErrInvalidExport = errors.New("readmodel: read model として出力できない")

// MaxCatalogAbilityCount は balance の read model が受け付ける abilityIds の上限
// (services/balance/schema/pokemon-types.schema.json の maxItems・loader の maxCatalogAbilityCount と同じ。
// 4件ある種族は slot 4(Showdown の特殊枠)を落とす。ADR-0105 §5・限界2)。
const MaxCatalogAbilityCount = 4

// AbilityTypeEffect は特性の防御側のタイプ相性に関わる効果(ADR-0017 §2。ADR-0105 §5・ADR-0106 §決定7)。
// Numerator/Denominator は immune/absorb の行では意味を持たないので省く
// (abilities.schema.json の factor は minimum 1 で、immune/absorb では存在自体が不正になる)。
type AbilityTypeEffect struct {
	Kind        string `json:"kind"`
	AttackType  string `json:"attackType,omitempty"`
	Numerator   int    `json:"numerator,omitempty"`
	Denominator int    `json:"denominator,omitempty"`
}

// NormalizedAbilityEffects は ability_effects の JSON を防御側のタイプ相性の効果だけに正規化する
// (ADR-0017 §2・ADR-0105 §5)。効果の行が無い・防御側の効果を持たない特性は空配列(null にしない)。
// このバイナリが知らないフィールドは読み捨て、名前ごとの特性数を ignored に足す(ADR-0145)。
func NormalizedAbilityEffects(raw []byte, chart engine.TypeChart, ignored map[string]int) ([]AbilityTypeEffect, error) {
	out := []AbilityTypeEffect{}
	if raw == nil {
		return out, nil
	}
	effect, unknown, err := master.DecodeAbilityEffectIgnoringUnknown(raw, chart)
	if err != nil {
		return nil, fmt.Errorf("%w: 特性の効果定義を読めない: %v", ErrInvalidExport, err)
	}
	for _, name := range unknown {
		ignored[name]++
	}
	for _, t := range sortedImmuneTypes(effect.DefImmuneTypes) {
		out = append(out, AbilityTypeEffect{Kind: "immune", AttackType: string(t)})
	}
	for _, t := range sortedTypeKeys(effect.DefAbsorbTypes) {
		out = append(out, AbilityTypeEffect{Kind: "absorb", AttackType: string(t)})
	}
	for _, t := range sortedTypeKeys(effect.DefResistType) {
		num, den, ok := reduceRatio(effect.DefResistType[t])
		if !ok {
			return nil, fmt.Errorf("%w: DefResistType[%s] の係数が1〜16の比にならない: %d/4096", ErrInvalidExport, t, effect.DefResistType[t])
		}
		out = append(out, AbilityTypeEffect{Kind: "type_multiplier", AttackType: string(t), Numerator: num, Denominator: den})
	}
	if effect.ReduceSuperEffective != 0 {
		num, den, ok := reduceRatio(effect.ReduceSuperEffective)
		if !ok {
			return nil, fmt.Errorf("%w: ReduceSuperEffective の係数が1〜16の比にならない: %d/4096", ErrInvalidExport, effect.ReduceSuperEffective)
		}
		out = append(out, AbilityTypeEffect{Kind: "super_effective_multiplier", Numerator: num, Denominator: den})
	}
	return out, nil
}

// reduceRatio は m/4096 を約分し、分子・分母が 1〜16 に収まるかを返す(近似しない。ADR-0105 §5)。
func reduceRatio(m int) (numerator, denominator int, ok bool) {
	if m <= 0 {
		return 0, 0, false
	}
	g := gcd(m, 4096)
	n, d := m/g, 4096/g
	if n < 1 || n > 16 || d < 1 || d > 16 {
		return 0, 0, false
	}
	return n, d, true
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func sortedTypeKeys[V any](m map[engine.Type]V) []engine.Type {
	out := make([]engine.Type, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// sortedImmuneTypes はタイプ ID 昇順にした DefImmuneTypes のコピーを返す(入力を書き換えない)。
func sortedImmuneTypes(types []engine.Type) []engine.Type {
	out := make([]engine.Type, len(types))
	copy(out, types)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
