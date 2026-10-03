package master

// 技の対象(moves.target。ADR-0136)。値は Showdown の MoveTarget(@smogon/calc 0.12.0 と同じ 15 種)の
// 文字列のまま持つ。migration の CHECK(chk_moves_target)と一致させる(services/pokedex/db の layout テストで確かめる)。

import (
	"fmt"
	"slices"
)

// MoveTarget は技の対象。
type MoveTarget string

// allMoveTargets は値の昇順。
var allMoveTargets = []MoveTarget{
	"adjacentAlly", "adjacentAllyOrSelf", "adjacentFoe", "all", "allAdjacent", "allAdjacentFoes",
	"allies", "allySide", "allyTeam", "any", "foeSide", "normal", "randomNormal", "scripted", "self",
}

// AllMoveTargets は対象の一覧(昇順)のコピーを返す。
func AllMoveTargets() []MoveTarget {
	return slices.Clone(allMoveTargets)
}

// IsMoveTarget は s が既知の対象かどうか(大文字小文字を区別する)。
func IsMoveTarget(s string) bool {
	return slices.Contains(allMoveTargets, MoveTarget(s))
}

// IsSpread は複数の相手に当たる全体技か。@smogon/calc の全体技補正の条件と同じ集合。
func (t MoveTarget) IsSpread() bool {
	return t == "allAdjacent" || t == "allAdjacentFoes"
}

// MoveTargetOf は技の行の対象を検証して返す。空は不明として通す。未知の値は ErrInvalidRow。
func MoveTargetOf(row MoveRow) (MoveTarget, error) {
	if row.Target == "" {
		return "", nil
	}
	if !IsMoveTarget(row.Target) {
		return "", fmt.Errorf("%w: 未知の技の対象: %q", ErrInvalidRow, row.Target)
	}
	return MoveTarget(row.Target), nil
}
