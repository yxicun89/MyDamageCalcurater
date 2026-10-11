package domain

// engine を呼ぶ前の入力の検証(invalid_input・invalid_observation・no_observation)。

import (
	"fmt"

	"example.com/pokecalc/engine"
)

// ValidateIndividual は engine.Individual.Validate の失敗を invalid_input にする(engine/wasmapi と同じ)。
func ValidateIndividual(label string, in engine.Individual) error {
	if err := in.Validate(); err != nil {
		return Errorf(CodeInvalidInput, "%s の入力が不正: %v", label, err)
	}
	return nil
}

// MegaItemRules はメガシンカ後の種族の持ち物の規則を引く口(マスタが持つ。使う側のここで宣言する)。
type MegaItemRules interface {
	// MegaRequiredItem はメガシンカ後の種族(isMega)の requiredItemId を返す。
	// メガでない種族・未知の種族は ("", false)。メガで requiredItemId が無ければ ("", true)。
	MegaRequiredItem(speciesKey string) (requiredItemID string, isMega bool)
}

// CheckMegaItem はメガシンカ後の種族の持ち物規則(issue #315・ADR-0200 §4 追記)を検証する。
// メガ種族は requiredItemId の持ち物か持ち物なし(nil)だけを受け付け、別の持ち物は invalid_input。
// メガでない種族は何も検査しない。label は入力の場所(例 "攻撃側"・"itemVariants[1]")。
func CheckMegaItem(rules MegaItemRules, label string, species engine.Species, item *engine.Item) error {
	if item == nil {
		return nil
	}
	required, isMega := rules.MegaRequiredItem(species.Key)
	if !isMega || item.ID == required {
		return nil
	}
	if required == "" {
		return Errorf(CodeInvalidInput, "%s: メガシンカ後の種族 %q には持ち物を持たせられない(指定: %q)",
			label, species.Key, item.ID)
	}
	return Errorf(CodeInvalidInput, "%s: メガシンカ後の種族 %q には持ち物 %q を持たせられない(持てるのはメガストーン %q だけ)",
		label, species.Key, item.ID, required)
}

// CheckMegaItems は持ち物候補の各要素に CheckMegaItem を当てる(bulk の itemVariants・reverse の itemCandidates)。
func CheckMegaItems(rules MegaItemRules, label string, species engine.Species, items []*engine.Item) error {
	for i, it := range items {
		if err := CheckMegaItem(rules, fmt.Sprintf("%s[%d]", label, i), species, it); err != nil {
			return err
		}
	}
	return nil
}

// Observation は逆算の観測 1 件の指定(percent・percentTenths・damage のうちちょうど 1 つ)。
type Observation struct {
	Percent, PercentTenths, Damage *int
	Note                           *string
}

// ConvertObservations は観測を検証して engine.Observation に写す(ADR-0200: キーの有無で数える)。
// engine.CalcReverse 自体は値の有無を区別できない(0 を未指定とみなす)ため、ここで独立に検証する。
func ConvertObservations(obs []Observation) ([]engine.Observation, error) {
	if len(obs) == 0 {
		return nil, Errorf(CodeNoObservation, "観測が1件も無い")
	}
	out := make([]engine.Observation, 0, len(obs))
	for i, o := range obs {
		n := 0
		if o.Percent != nil {
			n++
		}
		if o.PercentTenths != nil {
			n++
		}
		if o.Damage != nil {
			n++
		}
		switch {
		case n != 1:
			return nil, Errorf(CodeInvalidObservation, "観測 %d は percent・percentTenths・damage のうちちょうど1つを指定すること", i+1)
		case o.Percent != nil && (*o.Percent < 1 || *o.Percent > 100):
			return nil, Errorf(CodeInvalidObservation, "観測 %d の percent は 1..100 の範囲外: %d", i+1, *o.Percent)
		case o.PercentTenths != nil && (*o.PercentTenths < 1 || *o.PercentTenths > 1000):
			return nil, Errorf(CodeInvalidObservation, "観測 %d の percentTenths は 1..1000 の範囲外: %d", i+1, *o.PercentTenths)
		case o.Damage != nil && *o.Damage < 1:
			return nil, Errorf(CodeInvalidObservation, "観測 %d の damage は正でなければならない: %d", i+1, *o.Damage)
		}
		e := engine.Observation{}
		if o.Percent != nil {
			e.Percent = *o.Percent
		}
		if o.PercentTenths != nil {
			e.PercentTenths = *o.PercentTenths
		}
		if o.Damage != nil {
			e.Damage = *o.Damage
		}
		if o.Note != nil {
			e.Note = *o.Note
		}
		out = append(out, e)
	}
	return out, nil
}

// BattleState は対戦の状態の指定(ADR-0144 §6)。省略した項目は nil。
type BattleState struct {
	AttackerCurrentHP, DefenderCurrentHP, Hits *int
}

// BattleStateFrom は対戦の状態を engine.BattleState にする。nil は従来どおり(ゼロ値)。
// engine の 0 は「満タン・既定」なので、指定された 0 や負は engine に渡さず invalid_input にする
// (黙って満タンと取り違えない)。最大 HP を超える・多段でない技の回数などの値域は engine が見る(ErrInvalidBattleState)。
func BattleStateFrom(b *BattleState) (engine.BattleState, error) {
	var s engine.BattleState
	if b == nil {
		return s, nil
	}
	for _, f := range []struct {
		name string
		src  *int
		dst  *int
	}{
		{"battleState.attackerCurrentHp", b.AttackerCurrentHP, &s.AttackerCurrentHP},
		{"battleState.defenderCurrentHp", b.DefenderCurrentHP, &s.DefenderCurrentHP},
		{"battleState.hits", b.Hits, &s.Hits},
	} {
		if f.src == nil {
			continue
		}
		if *f.src < 1 {
			return engine.BattleState{}, Errorf(CodeInvalidInput, "%s は 1 以上でなければならない: %d", f.name, *f.src)
		}
		*f.dst = *f.src
	}
	return s, nil
}
