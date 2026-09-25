package master

// 技の機構(move_mechanisms。ADR-0121)。
//
// 機構は「威力・分類・タイプから通常の式で計算すると誤る」理由の分類で、1つの技が複数を持つことがある
// (例: 多段かつ威力が変わる)。行が無い技 = 通常の技。変化技は機構を持たない。
// 値の一覧は migration の CHECK(chk_move_mechanisms_mechanism)と一致させる
// (services/pokedex/db の layout テストで確かめる)。
//
// engine の型にはまだ載せない(未対応の印・計算できる機構の実装は D16)。ここでは DB の行が
// 正しい値かを検証するだけにする。

import (
	"fmt"
	"sort"
)

// MoveMechanism は技の機構1つ。
type MoveMechanism string

const (
	// MechanismAltDefenseStat は防御側の参照する能力値が分類と違う(特殊技で防御を参照する 等)。
	MechanismAltDefenseStat MoveMechanism = "alt_defense_stat"
	// MechanismAltOffenseStat は攻撃側の参照する能力値・参照するポケモンが違う(防御で攻撃する・相手の攻撃を使う 等)。
	MechanismAltOffenseStat MoveMechanism = "alt_offense_stat"
	// MechanismAlwaysCrit は必ず急所に当たる。
	MechanismAlwaysCrit MoveMechanism = "always_crit"
	// MechanismEffectivenessChange はタイプ相性の求め方が違う(2タイプの相性を使う・特定タイプへの相性を変える 等)。
	MechanismEffectivenessChange MoveMechanism = "effectiveness_change"
	// MechanismFieldSpecific は天候・フィールドがこの技を名指しして処理を変える(フィールドで威力が変わる 等)。
	MechanismFieldSpecific MoveMechanism = "field_specific"
	// MechanismFixedDamage はダメージを計算式でなく技の処理で決める(レベルと同じ・残り HP の半分・受けたダメージの倍返し 等)。
	MechanismFixedDamage MoveMechanism = "fixed_damage"
	// MechanismIgnoreDefenseRanks は防御側の防御・特防のランク変化を無視する。
	MechanismIgnoreDefenseRanks MoveMechanism = "ignore_defense_ranks"
	// MechanismMoveSpecific は技固有の処理があり、ダメージに効くかをデータから機械的に決められない
	// (技の性質を書き換える処理・当たる前の処理・使う前の処理)。安全側に倒して「通常の技ではない」とする。
	MechanismMoveSpecific MoveMechanism = "move_specific"
	// MechanismMultiHit は複数回当たる。
	MechanismMultiHit MoveMechanism = "multi_hit"
	// MechanismOHKO は一撃必殺。
	MechanismOHKO MoveMechanism = "ohko"
	// MechanismPriorityChange は条件で優先度が変わる(サイコフィールドの先制技の扱いに効く)。
	MechanismPriorityChange MoveMechanism = "priority_change"
	// MechanismTypeChange は条件で技のタイプが変わる。
	MechanismTypeChange MoveMechanism = "type_change"
	// MechanismVariablePower は条件で威力が変わる(重さ・HP・ランク・状態・相手の持ち物 等)。
	// 威力 0 で登録された攻撃技(固定ダメージ・一撃必殺でないもの)もここに入る。
	MechanismVariablePower MoveMechanism = "variable_power"
)

// allMoveMechanisms は機構の一覧(値の昇順)。
var allMoveMechanisms = []MoveMechanism{
	MechanismAltDefenseStat,
	MechanismAltOffenseStat,
	MechanismAlwaysCrit,
	MechanismEffectivenessChange,
	MechanismFieldSpecific,
	MechanismFixedDamage,
	MechanismIgnoreDefenseRanks,
	MechanismMoveSpecific,
	MechanismMultiHit,
	MechanismOHKO,
	MechanismPriorityChange,
	MechanismTypeChange,
	MechanismVariablePower,
}

// AllMoveMechanisms は機構の一覧(値の昇順)のコピーを返す。
func AllMoveMechanisms() []MoveMechanism {
	return append([]MoveMechanism(nil), allMoveMechanisms...)
}

// IsMoveMechanism は s が既知の機構かどうか(大文字小文字を区別する)。
func IsMoveMechanism(s string) bool {
	for _, m := range allMoveMechanisms {
		if string(m) == s {
			return true
		}
	}
	return false
}

// MoveMechanismsOf は技の行の機構を検証し、昇順に並べて返す。通常の技は nil。
// 未知の値・重複・変化技の機構は ErrInvalidRow。
func MoveMechanismsOf(row MoveRow) ([]MoveMechanism, error) {
	if len(row.Mechanisms) == 0 {
		return nil, nil
	}
	if row.Category == "status" {
		return nil, fmt.Errorf("%w: 変化技なのに機構がある: %v", ErrInvalidRow, row.Mechanisms)
	}
	seen := map[string]bool{}
	out := make([]MoveMechanism, 0, len(row.Mechanisms))
	for _, s := range row.Mechanisms {
		if !IsMoveMechanism(s) {
			return nil, fmt.Errorf("%w: 未知の機構: %q", ErrInvalidRow, s)
		}
		if seen[s] {
			return nil, fmt.Errorf("%w: 機構が重複: %q", ErrInvalidRow, s)
		}
		seen[s] = true
		out = append(out, MoveMechanism(s))
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}
