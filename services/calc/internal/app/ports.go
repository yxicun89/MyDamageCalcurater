package app

import (
	"context"
	"errors"
	"time"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/calc/internal/domain"
)

// Master は計算に使うマスタの参照口(ADR-0204)。実装は adapters/master.MemoryStore。
// 実装は並行に呼ばれても安全であること。返す値は呼び出し側が書き換えても中身に影響しない(スライスは複製して返す)。
type Master interface {
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
	// MegaRequiredItem はメガシンカ後の種族の持ち物の規則(domain.MegaItemRules。issue #315・ADR-0200 §4 追記)。
	domain.MegaItemRules
	// TypeChart は検証済みのタイプ相性表を返す。
	TypeChart() engine.TypeChart
}

// Operation はイベントの操作の種類。
type Operation string

const (
	OperationCalc    Operation = "calc"
	OperationBulk    Operation = "bulk"
	OperationReverse Operation = "reverse"
)

// Event は計算が成功したことの通知(ADR-0212)。Calc は計算(OperationCalc)のときだけ入る
// (一括・逆算は envelope だけ。比較検討中の候補で「よく使う相手」の集計を汚染しないため。ADR-0212 §7.1)。
type Event struct {
	Meta       Meta
	Operation  Operation
	OccurredAt time.Time
	Calc       *CalcEvent
}

// CalcEvent は計算のイベントの中身(リクエストのまま + 結果)。
type CalcEvent struct {
	Request CalcRequest
	Result  engine.DamageResult
}

// EventPublisher は計算イベントの発行。発行は非同期で応答を一切ブロックしない(CLAUDE.md 絶対ルール5)。
type EventPublisher interface {
	Publish(Event)
}

// Expired は ctx の締め切りを過ぎたかを返す(engine を呼ぶ前に確かめる。ADR-0801)。
type Expired func(ctx context.Context) bool

// ErrDeadlineExceeded は締め切りを過ぎたので計算を始めなかったこと(adapters が 503 にする)。
var ErrDeadlineExceeded = errors.New("処理の締め切りを過ぎた")

// Meta はリクエストを送った端末とセッション(イベントの envelope に載る)。
type Meta struct {
	DeviceID, SessionID string
}

type noopPublisher struct{}

func (noopPublisher) Publish(Event) {}
