package engine

// 技の機構の中身(ADR-0142)。取得元のフィールドの値だけで決まる機構を、技名の分岐なしに計算するための入力。
// 中身があるときだけ計算して「未対応」の印(ADR-0123)を外す。機構があって中身が無いときは従来どおり通常の式 + 印。

import (
	"errors"
	"fmt"
	"slices"
)

// MaxMultiHits は多段技の回数の上限(取得元の最大。確定数の畳み込みの計算量の上限でもある)。
const MaxMultiHits = 10

// ErrInvalidMechanismParams は技の機構の中身が不正(機構と対応しない・値域外・変化技の中身)。
var ErrInvalidMechanismParams = errors.New("技の機構の中身が不正")

// MultiHit は多段技の回数。Min == Max は固定回数。
type MultiHit struct{ Min, Max int }

// FixedDamage は固定ダメージ。Level は攻撃側のレベル、Value はその値。どちらか一方だけ。
type FixedDamage struct {
	Level bool
	Value int
}

// OHKO は一撃必殺。ImmuneType は TypeNone(なし)か、そのタイプの相手に効かないタイプ。
type OHKO struct{ ImmuneType Type }

// OffensePokemon は攻撃に使う能力値を持つポケモン。
type OffensePokemon string

const (
	// OffensePokemonAttacker は攻撃側(既定と同じ)。
	OffensePokemonAttacker OffensePokemon = "attacker"
	// OffensePokemonDefender は防御側(相手の攻撃で攻撃する技)。
	OffensePokemonDefender OffensePokemon = "defender"
)

// MechanismParams は技の機構の中身。ゼロ値は「中身なし」。
type MechanismParams struct {
	MultiHit       *MultiHit
	FixedDamage    *FixedDamage
	OHKO           *OHKO
	OffenseStat    StatKey        // "" は分類どおり(物理 atk / 特殊 spa)
	OffensePokemon OffensePokemon // "" と attacker は攻撃側、defender は防御側の能力値とランク
	DefenseStat    StatKey        // "" は分類どおり(物理 def / 特殊 spd)
}

// hasOffenseContent は alt_offense_stat の中身があるか。
func (p MechanismParams) hasOffenseContent() bool {
	return p.OffenseStat != "" || p.OffensePokemon != ""
}

// hasDefenseContent は alt_defense_stat の中身があるか。
func (p MechanismParams) hasDefenseContent() bool { return p.DefenseStat != "" }

// isEmpty は中身が何も無いか。
func (p MechanismParams) isEmpty() bool {
	return p.MultiHit == nil && p.FixedDamage == nil && p.OHKO == nil && !p.hasOffenseContent() && !p.hasDefenseContent()
}

// usableStat は攻撃・防御に使える能力値(HP 以外)か。
func usableStat(k StatKey) bool {
	return slices.Contains(rankStatKeys, k)
}

// ValidateParams は機構の中身が機構と対応し、値域に収まることを確かめる(ADR-0142 §2)。
func (m Move) ValidateParams(chart TypeChart) error {
	p := m.Params
	if p.isEmpty() {
		return nil
	}
	bad := func(format string, args ...any) error {
		return fmt.Errorf("%w: 技 %q: %s", ErrInvalidMechanismParams, m.ID, fmt.Sprintf(format, args...))
	}
	if m.Category == CategoryStatus {
		return bad("変化技に中身がある")
	}
	has := func(mech MoveMechanism) bool { return slices.Contains(m.Mechanisms, mech) }
	if mh := p.MultiHit; mh != nil {
		if !has(MechanismMultiHit) {
			return bad("MultiHit に対応する機構 multi_hit が無い")
		}
		if mh.Min < 1 || mh.Min > mh.Max || mh.Max < 2 || mh.Max > MaxMultiHits {
			return bad("MultiHit は 1 <= Min <= Max・2 <= Max <= %d: %+v", MaxMultiHits, *mh)
		}
	}
	if fd := p.FixedDamage; fd != nil {
		if !has(MechanismFixedDamage) {
			return bad("FixedDamage に対応する機構 fixed_damage が無い")
		}
		if fd.Value < 0 || fd.Level == (fd.Value > 0) {
			return bad("FixedDamage は Level と Value > 0 のちょうど一方: %+v", *fd)
		}
	}
	if o := p.OHKO; o != nil {
		if !has(MechanismOHKO) {
			return bad("OHKO に対応する機構 ohko が無い")
		}
		if err := chart.requireKnown("一撃必殺の無効タイプ", o.ImmuneType); err != nil {
			return err
		}
	}
	if p.hasOffenseContent() {
		if !has(MechanismAltOffenseStat) {
			return bad("OffenseStat / OffensePokemon に対応する機構 alt_offense_stat が無い")
		}
		if p.OffenseStat != "" && !usableStat(p.OffenseStat) {
			return bad("OffenseStat が不正: %q", p.OffenseStat)
		}
		switch p.OffensePokemon {
		case "", OffensePokemonAttacker, OffensePokemonDefender:
		default:
			return bad("OffensePokemon が未知: %q", p.OffensePokemon)
		}
	}
	if p.hasDefenseContent() {
		if !has(MechanismAltDefenseStat) {
			return bad("DefenseStat に対応する機構 alt_defense_stat が無い")
		}
		if !usableStat(p.DefenseStat) {
			return bad("DefenseStat が不正: %q", p.DefenseStat)
		}
	}
	return nil
}

// multiHitCount はこの入力での技1回の使用で当たる回数(多段でなければ 1)。固定回数はその値。範囲は
// 状態の回数 State.Hits の指定があればそれ、無ければ攻撃側の特性の効果 MaxMultiHit(スキルリンク)があれば最大、無ければ 最小 + 1(oracle と同じ。ADR-0142 §3)。
func multiHitCount(in DamageInput) int {
	mh := in.Move.Params.MultiHit
	if mh == nil {
		return 1
	}
	if mh.Min == mh.Max {
		return mh.Min
	}
	if in.State.Hits != 0 {
		return in.State.Hits // 指定は最大回数の特性に勝つ(oracle の options.hits。ADR-0144)
	}
	if ae := in.Attacker.Ability.Effect; ae != nil && ae.MaxMultiHit {
		return mh.Max
	}
	return mh.Min + 1
}

// altStatMarks は目標の探索・配分が扱わない(参照する能力が技と合わない)機構の印(ADR-0142 §6。段階2)。
func altStatMarks(m Move) []UnsupportedMark {
	var marks []UnsupportedMark
	if m.Category == CategoryStatus {
		return nil
	}
	if m.Params.hasOffenseContent() {
		marks = append(marks, UnsupportedMark{Target: UnsupportedTargetMove, Reason: UnsupportedReason(MechanismAltOffenseStat), ID: m.ID})
	}
	if m.Params.hasDefenseContent() {
		marks = append(marks, UnsupportedMark{Target: UnsupportedTargetMove, Reason: UnsupportedReason(MechanismAltDefenseStat), ID: m.ID})
	}
	return marks
}
