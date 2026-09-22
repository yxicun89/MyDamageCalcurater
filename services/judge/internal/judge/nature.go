package judge

import (
	"errors"

	"example.com/pokecalc/engine"
)

// ErrUnknownNature は natureId が性格の一覧に無いことを表す(httpapi で 422 unknown_nature に
// 畳む。ADR-0701 §6)。黙って無補正に倒さない ― 性格補正を取り違えた判定は外からは正しく
// 見えるまま間違うため。
var ErrUnknownNature = errors.New("nature is not in the nature list")

// Nature は judge が使う性格 1 件(ADR-0701 §4)。Plus/Minus は internal/client.Nature の文字列を
// engine.StatKey に変換したもの(client は engine に依存しない)。無補正は Plus == Minus(空文字
// 同士を含む)。
type Nature struct {
	ID    string
	Plus  engine.StatKey
	Minus engine.StatKey
}

// NatureTable は natureId から engine.Nature を引く表(ADR-0701 §4)。1 リクエストにつき 1 回
// 取った一覧を attacker と defender の両方に使い回すために作る。
type NatureTable struct {
	byID map[string]Nature
}

// NewNatureTable は性格の一覧から NatureTable を作る。
func NewNatureTable(natures []Nature) NatureTable {
	byID := make(map[string]Nature, len(natures))
	for _, n := range natures {
		byID[n.ID] = n
	}
	return NatureTable{byID: byID}
}

// Lookup は natureId を engine.Nature に解決する。一覧に無い ID(空文字を含む)は
// ErrUnknownNature。
func (t NatureTable) Lookup(id string) (engine.Nature, error) {
	nature, ok := t.byID[id]
	if !ok {
		return engine.Nature{}, ErrUnknownNature
	}
	return engine.Nature{Plus: nature.Plus, Minus: nature.Minus}, nil
}
