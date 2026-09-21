// Package master は calc-svc の**暫定の**マスタ境界(ADR-0016)。
//
// データレーンの共通マスタ(services/internal/master。plan.md P2-2a)が main に入ったら、
// Store の実装をそちらに差し替える。httpapi は Store インターフェースにだけ依存する。
//
// 起動時にスナップショット(暫定 JSON スキーマ。services/calc/README.md)と
// タイプ相性表(testdata/golden/typechart.json と同じ schema。ADR-0013 / ADR-0015)を
// メモリに読み込む。フォールバックの既定データは持たない(ADR-0013)。
package master

import (
	"errors"
	"io"

	"example.com/pokecalc/engine"
)

// ロード時の失敗。呼び出し側は errors.Is で判別する。
var (
	// ErrInvalidSnapshot はマスタのスナップショットが暫定スキーマを満たさない
	// (壊れた JSON・未知のフィールド・ID の重複・未知のタイプ/分類/ステータスキー・性格が HP を指す等)。
	ErrInvalidSnapshot = errors.New("マスタのスナップショットが不正")
	// ErrInvalidTypeChart はタイプ相性表のデータが schema を満たさない、または表として不正。
	ErrInvalidTypeChart = errors.New("タイプ相性表のデータが不正")
)

// errNotImplemented は P3-1 の implementer が置き換えるまでのスタブ用。
var errNotImplemented = errors.New("未実装(P3-1)")

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
	// NatureID は性格補正の構造値から性格 ID を引く(ADR-0016)。
	//   - 無補正(n.IsNeutral())は、マスタ中の無補正性格を ID の昇順で並べた最初のもの。
	//   - それ以外は (Plus, Minus) が一致する性格のうち ID の昇順で最初のもの。
	//   - 該当が無ければ ("", false)。
	NatureID(n engine.Nature) (string, bool)
	// TypeChart は検証済みのタイプ相性表を返す。
	TypeChart() engine.TypeChart
}

// Snapshot は検証済みのマスタのスナップショット(暫定スキーマ。services/calc/README.md)。
type Snapshot struct {
	// TODO(P3-1): implementer が中身を決める。
}

// LoadSnapshot はスナップショットを読んで検証する。
// 不正はすべて ErrInvalidSnapshot で包んで返す(部分的なスナップショットは返さない)。
func LoadSnapshot(r io.Reader) (*Snapshot, error) {
	return nil, errNotImplemented
}

// LoadTypeChart は testdata/golden/typechart.json と同じ schema(schemaVersion 1)の相性表を読み、
// engine.NewTypeChart で検証済みの表にする。不正はすべて ErrInvalidTypeChart で包む。
func LoadTypeChart(r io.Reader) (engine.TypeChart, error) {
	return engine.TypeChart{}, errNotImplemented
}

// MemoryStore はスナップショットと相性表をメモリに持つ Store。
type MemoryStore struct {
	// TODO(P3-1): implementer が中身を決める。
}

var _ Store = (*MemoryStore)(nil)

// New はスナップショットと相性表から Store を作る。
// 相性表がゼロ値なら ErrInvalidTypeChart、スナップショットの種族・技・持ち物・特性に
// 相性表に無いタイプが現れたら engine.ErrUnknownType で包んだエラーを返す
// (計算時ではなく起動時に気づくため)。
func New(snapshot *Snapshot, chart engine.TypeChart) (*MemoryStore, error) {
	return nil, errNotImplemented
}

// Species は Store を実装する。
func (s *MemoryStore) Species(key string) (engine.Species, bool) { panic("TODO(P3-1)") }

// Move は Store を実装する。
func (s *MemoryStore) Move(id string) (engine.Move, bool) { panic("TODO(P3-1)") }

// Item は Store を実装する。
func (s *MemoryStore) Item(id string) (engine.Item, bool) { panic("TODO(P3-1)") }

// Ability は Store を実装する。
func (s *MemoryStore) Ability(id string) (engine.Ability, bool) { panic("TODO(P3-1)") }

// Nature は Store を実装する。
func (s *MemoryStore) Nature(id string) (engine.Nature, bool) { panic("TODO(P3-1)") }

// NatureID は Store を実装する。
func (s *MemoryStore) NatureID(n engine.Nature) (string, bool) { panic("TODO(P3-1)") }

// TypeChart は Store を実装する。
func (s *MemoryStore) TypeChart() engine.TypeChart { panic("TODO(P3-1)") }
