package master

// 技の機構(move_mechanisms。ADR-0121)。
//
// 機構は「威力・分類・タイプから通常の式で計算すると誤る」理由の分類で、1つの技が複数を持つことがある
// (例: 多段かつ威力が変わる)。行が無い技 = 通常の技。変化技は機構を持たない。
// 値の一覧の正は engine(engine.AllMoveMechanisms。engine が未対応の印を付けるため。ADR-0123)。
// migration の CHECK(chk_move_mechanisms_mechanism)と一致させる(services/pokedex/db の layout テストで確かめる)。
// ここでは DB の行が正しい値かを検証し、engine.Move.Mechanisms に載せる。

import (
	"fmt"
	"slices"

	"example.com/pokecalc/engine"
)

// MoveMechanism は技の機構1つ(engine.MoveMechanism の別名)。
type MoveMechanism = engine.MoveMechanism

const (
	MechanismAltDefenseStat      = engine.MechanismAltDefenseStat
	MechanismAltOffenseStat      = engine.MechanismAltOffenseStat
	MechanismAlwaysCrit          = engine.MechanismAlwaysCrit
	MechanismEffectivenessChange = engine.MechanismEffectivenessChange
	MechanismFieldSpecific       = engine.MechanismFieldSpecific
	MechanismFixedDamage         = engine.MechanismFixedDamage
	MechanismIgnoreDefenseRanks  = engine.MechanismIgnoreDefenseRanks
	MechanismMoveSpecific        = engine.MechanismMoveSpecific
	MechanismMultiHit            = engine.MechanismMultiHit
	MechanismOHKO                = engine.MechanismOHKO
	MechanismPriorityChange      = engine.MechanismPriorityChange
	MechanismTypeChange          = engine.MechanismTypeChange
	MechanismVariablePower       = engine.MechanismVariablePower
)

// AllMoveMechanisms は機構の一覧(値の昇順)のコピーを返す。
func AllMoveMechanisms() []MoveMechanism {
	return engine.AllMoveMechanisms()
}

// IsMoveMechanism は s が既知の機構かどうか(大文字小文字を区別する)。
func IsMoveMechanism(s string) bool {
	return engine.MoveMechanism(s).Known()
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
	slices.Sort(out)
	return out, nil
}
