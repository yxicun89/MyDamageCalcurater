// Package calcevents は calc-svc が NATS JetStream へ発行し、record-svc(P5-3)・
// team-svc(P5-4)が購読するイベントのワイヤフォーマットを持つ(ADR-0212 §7)。
//
// ストリーム名は "CALC_EVENTS"、subject は "calc.events.<device_id>"(ADR-0212 §4)。
// 配送は at-least-once(重複排除は設定しない。ADR-0212 §6)。消費側は同じイベントを
// 2回受け取っても安全なように(冪等に)実装すること。
package calcevents

import (
	"time"

	"example.com/pokecalc/services/internal/api"
)

// StreamName は calc-svc が発行する JetStream のストリーム名(ADR-0212 §4)。
const StreamName = "CALC_EVENTS"

// SubjectPrefix と Subject は発行 subject の組み立てに使う(ADR-0212 §4)。
// device_id は gateway が正準形(8-4-4-4-12)を検証済みの UUID で "." を含まない
// (ADR-0202 §4)前提だが、calc-svc 自身は形式を検証しない(ADR-0212 §4)。
const SubjectPrefix = "calc.events."

// Subject は device_id から発行先の subject 文字列を組み立てる。
func Subject(deviceID string) string {
	return SubjectPrefix + deviceID
}

// SchemaVersion はワイヤフォーマットの版(1始まり)。calc-svc と record-svc/team-svc は
// 独立にデプロイされ、最長7日分のイベントがストリームに滞留しうる(ADR-0212 §4)ため、
// 構造を変えるときはこの値で分岐できるようにしておく。
const SchemaVersion = 1

// Operation の取りうる値(ADR-0212 §7.1)。
const (
	OperationCalc    = "calc"
	OperationBulk    = "calcBulk"
	OperationReverse = "calcReverse"
)

// Event は1回の計算 API 呼び出しに対応する(ADR-0209 §3 #6・ADR-0212 §7)。
type Event struct {
	SchemaVersion int       `json:"schemaVersion"`
	DeviceID      string    `json:"deviceId"`
	SessionID     string    `json:"sessionId"`
	Operation     string    `json:"operation"`  // OperationCalc | OperationBulk | OperationReverse
	OccurredAt    time.Time `json:"occurredAt"` // calc-svc が計算した時刻(ADR-0209 §7。受信時刻ではない)

	// Detail は Operation == OperationCalc のときだけ入る(ADR-0212 §7.1)。team-svc は
	// Detail を一切読まない(devices.last_seen_at の更新に使うのは envelope だけ。ADR-0209 §4)。
	Detail *CalcDetail `json:"detail,omitempty"`
}

// CalcDetail は POST /api/calc(1件の攻撃側 vs 防御側)の内容(requirements.md §6 の
// calc_events 列)。フィールドは api.CalcRequest・api.CalcResult からそのまま埋める
// (ADR-0212 §7。api/openapi.yaml の破壊的変更がこの構造を黙って変えうることを許容する
// 判断で、その代わり contract_test.go(コンパイル時チェック)と golden_test.go(埋め込み型の
// 内部フィールドまで検知する JSON リテラル比較)で検知する)。
type CalcDetail struct {
	Format   string         `json:"format"`
	Attacker api.Individual `json:"attacker"`
	Defender api.Individual `json:"defender"`
	// MoveID は req.MoveId(CalcRequest のトップレベル)から取る。api.Individual.MoveId ではない
	// (calc-svc の resolveIndividual は Individual.MoveId を読まない)。
	MoveID  string           `json:"moveId"`
	Field   *api.FieldState  `json:"field,omitempty"`
	Options *api.CalcOptions `json:"options,omitempty"`

	// MinPercent/MaxPercent は api.CalcResult.MinPercent/MaxPercent(表示%。丸め後の値)を
	// そのまま写す。MinPercent は切り捨て・MaxPercent は四捨五入で丸め方向が異なる(ADR-0010 §3)。
	MinPercent float64 `json:"minPercent"`
	MaxPercent float64 `json:"maxPercent"`

	// ViaRecommendation は今の calc-svc の入力に対応するフィールドが無いため常に false
	// (ADR-0212 §7。推薦機能が calc API 呼び出しにその情報を持たせるようになったら埋める)。
	ViaRecommendation bool `json:"viaRecommendation"`
}
