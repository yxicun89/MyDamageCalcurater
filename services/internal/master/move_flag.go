package master

// 技のフラグ(move_flags。ADR-0178)。
//
// 値の一覧の正は engine(engine.AllMoveFlags)。migration の CHECK(chk_move_flags_flag)と一致させる
// (services/pokedex/db の layout テストで確かめる)。ここでは DB の行が正しい値かを検証し、engine.Move.Flags に載せる。

import (
	"fmt"
	"slices"

	"example.com/pokecalc/engine"
)

// MoveFlag は技のフラグ1つ(engine.MoveFlag の別名)。
type MoveFlag = engine.MoveFlag

// AllMoveFlags はフラグの一覧(値の昇順)のコピーを返す。
func AllMoveFlags() []MoveFlag {
	return engine.AllMoveFlags()
}

// IsMoveFlag は s が既知のフラグかどうか(大文字小文字を区別する)。
func IsMoveFlag(s string) bool {
	return engine.MoveFlag(s).Known()
}

// MoveFlagsOf は行のフラグを検証し、昇順の新しいスライスにして返す(フラグが無ければ nil)。
// 未知の値・重複・FlagsKnown が偽なのに値がある は ErrInvalidRow。
func MoveFlagsOf(row MoveRow) ([]engine.MoveFlag, error) {
	if !row.FlagsKnown && len(row.Flags) > 0 {
		return nil, fmt.Errorf("%w: 技 %q のフラグが不明なのに値がある: %v", ErrInvalidRow, row.ID, row.Flags)
	}
	if len(row.Flags) == 0 {
		return nil, nil
	}
	out := make([]engine.MoveFlag, 0, len(row.Flags))
	for _, s := range row.Flags {
		if !IsMoveFlag(s) {
			return nil, fmt.Errorf("%w: 技 %q のフラグが語彙に無い: %q", ErrInvalidRow, row.ID, s)
		}
		f := engine.MoveFlag(s)
		if slices.Contains(out, f) {
			return nil, fmt.Errorf("%w: 技 %q のフラグが重複している: %q", ErrInvalidRow, row.ID, s)
		}
		out = append(out, f)
	}
	slices.Sort(out)
	return out, nil
}
