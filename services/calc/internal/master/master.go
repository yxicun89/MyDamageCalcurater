// Package master は calc-svc のマスタ境界(ADR-0204)。
//
// pokedex-svc の内部 API(GET /internal/pokedex/master)が返すマスタ一式(api.MasterExport。
// api/openapi.yaml)を FromExport(export.go)で検証し、engine の型でメモリに持つ Store(MemoryStore)にする。
// 相性表・種族・技・持ち物・特性は、データレーンの共通マスタ(services/internal/master。DB の行 → engine の型の
// 写像)にそのまま委ねる。値の検証(ID の形式・範囲・組の整合・効果定義)もそちらが行う。
// 性格(natures テーブルはまだ無い。ADR-0204 §5)だけは calc-svc 側で検証する。
//
// 入手元(ファイル・HTTP)は Source(export.go)。httpapi は Store インターフェースにだけ依存する。
package master

import (
	"sort"

	"example.com/pokecalc/engine"
)

// Store は calc-svc が計算に使うマスタの参照口。実装は並行に呼ばれても安全であること。
// 返す値は呼び出し側が書き換えても Store の中身に影響しない(スライスは複製して返す)。
type Store interface {
	// Species は種族キー({図鑑番号4桁}-{フォルム3桁})で種族を引く。
	Species(key string) (engine.Species, bool)
	// Move は技 ID で技を引く。
	Move(id string) (engine.Move, bool)
	// Item は持ち物 ID で持ち物(効果つき)を引く。
	Item(id string) (engine.Item, bool)
	// Ability は特性 ID で特性(効果つき)を引く。
	Ability(id string) (engine.Ability, bool)
	// Nature は性格 ID で性格補正を引く。
	Nature(id string) (engine.Nature, bool)
	// NatureID は性格補正の構造値から性格 ID を引く(ADR-0200)。
	//   - 無補正(n.IsNeutral())は、マスタ中の無補正性格を ID の昇順で並べた最初のもの。
	//   - それ以外は (Plus, Minus) が一致する性格のうち ID の昇順で最初のもの。
	//   - 該当が無ければ ("", false)。
	NatureID(n engine.Nature) (string, bool)
	// TypeChart は検証済みのタイプ相性表を返す。
	TypeChart() engine.TypeChart
}

// MemoryStore はマスタ一式をメモリに持つ Store(ADR-0204 §2)。FromExport(export.go)で作る。
type MemoryStore struct {
	species     map[string]engine.Species
	moves       map[string]engine.Move
	items       map[string]engine.Item
	abilities   map[string]engine.Ability
	natures     map[string]engine.Nature
	chart       engine.TypeChart
	dataVersion string
}

var _ Store = (*MemoryStore)(nil)

// Species は Store を実装する。返すスライスはコピーで、書き換えても Store に影響しない。
func (s *MemoryStore) Species(key string) (engine.Species, bool) {
	sp, ok := s.species[key]
	if !ok {
		return engine.Species{}, false
	}
	sp.Types = append([]engine.Type(nil), sp.Types...)
	sp.Abilities = append([]string(nil), sp.Abilities...)
	return sp, true
}

// Move は Store を実装する。Effect(内部の map を含む)はコピーを返す(呼び出し側が書き換えても
// Store に影響しない、という doc の約束を Effect にも適用する)。
func (s *MemoryStore) Move(id string) (engine.Move, bool) {
	mv, ok := s.moves[id]
	if !ok {
		return engine.Move{}, false
	}
	mv.Effect = copyMoveEffect(mv.Effect)
	return mv, true
}

// Item は Store を実装する。Effect(内部の map を含む)はコピーを返す(呼び出し側が書き換えても
// Store に影響しない、という doc の約束を Effect にも適用する)。
func (s *MemoryStore) Item(id string) (engine.Item, bool) {
	it, ok := s.items[id]
	if !ok {
		return engine.Item{}, false
	}
	it.Effect = copyItemEffect(it.Effect)
	return it, true
}

// Ability は Store を実装する。Effect(内部の map を含む)はコピーを返す。
func (s *MemoryStore) Ability(id string) (engine.Ability, bool) {
	ab, ok := s.abilities[id]
	if !ok {
		return engine.Ability{}, false
	}
	ab.Effect = copyAbilityEffect(ab.Effect)
	return ab, true
}

// copyItemEffect は *engine.ItemEffect のディープコピーを返す(nil は nil のまま)。
func copyItemEffect(e *engine.ItemEffect) *engine.ItemEffect {
	if e == nil {
		return nil
	}
	out := *e
	if e.StatMods != nil {
		out.StatMods = make(map[engine.StatKey]int, len(e.StatMods))
		for k, v := range e.StatMods {
			out.StatMods[k] = v
		}
	}
	return &out
}

// copyMoveEffect は *engine.MoveEffect のディープコピーを返す(nil は nil のまま)。
func copyMoveEffect(e *engine.MoveEffect) *engine.MoveEffect {
	if e == nil {
		return nil
	}
	out := *e
	if e.Stages != nil {
		out.Stages = make(map[engine.StatKey]int, len(e.Stages))
		for k, v := range e.Stages {
			out.Stages[k] = v
		}
	}
	return &out
}

// copyAbilityEffect は *engine.AbilityEffect のディープコピーを返す(nil は nil のまま)。
func copyAbilityEffect(e *engine.AbilityEffect) *engine.AbilityEffect {
	if e == nil {
		return nil
	}
	out := *e
	if e.DefResistType != nil {
		out.DefResistType = make(map[engine.Type]int, len(e.DefResistType))
		for k, v := range e.DefResistType {
			out.DefResistType[k] = v
		}
	}
	if e.DefImmuneTypes != nil {
		out.DefImmuneTypes = make([]engine.Type, len(e.DefImmuneTypes))
		copy(out.DefImmuneTypes, e.DefImmuneTypes)
	}
	if e.DefAbsorbTypes != nil {
		out.DefAbsorbTypes = make(map[engine.Type]engine.AbsorbEffect, len(e.DefAbsorbTypes))
		for k, v := range e.DefAbsorbTypes {
			out.DefAbsorbTypes[k] = v
		}
	}
	return &out
}

// Nature は Store を実装する。
func (s *MemoryStore) Nature(id string) (engine.Nature, bool) {
	n, ok := s.natures[id]
	return n, ok
}

// NatureID は Store を実装する(ADR-0200 の写像規則)。
func (s *MemoryStore) NatureID(n engine.Nature) (string, bool) {
	ids := make([]string, 0, len(s.natures))
	for id := range s.natures {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		m := s.natures[id]
		if (n.IsNeutral() && m.IsNeutral()) || m == n {
			return id, true
		}
	}
	return "", false
}

// TypeChart は Store を実装する。
func (s *MemoryStore) TypeChart() engine.TypeChart { return s.chart }

// DataVersion はマスタ一式の dataVersion(記録とログに使うだけ。計算には影響しない)。
func (s *MemoryStore) DataVersion() string { return s.dataVersion }
