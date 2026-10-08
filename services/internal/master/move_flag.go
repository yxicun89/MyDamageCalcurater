package master

// 技のフラグ(move_flags。ADR-0178)。
//
// 値の一覧の正は engine(engine.AllMoveFlags)。migration の CHECK(chk_move_flags_flag)と一致させる
// (services/pokedex/db の layout テストで確かめる)。ここでは DB の行が正しい値かを検証し、engine.Move.Flags に載せる。
//
// TODO(ADR-0178 実装): spec-writer のスタブ。Move での検証・写像(MoveRow.Flags / FlagsKnown → engine.Move)は実装者が書く。

import "example.com/pokecalc/engine"

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
