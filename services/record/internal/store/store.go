// Package store は record-svc のデータアクセス層の契約(型とインターフェース)を持つ。
// TiDB への実装(SQL・接続・上限の設定値)は同じパッケージの `tidb.go`(`//go:build tidb` の
// テストは `tidb_test.go`)。httpapi と events はこのインターフェースだけに依存し、単体テストは
// 架空の実装(fake)で差し替える。
//
// 規則(ADR-0209 §6。このインターフェースを実装するときの絶対条件):
//   - すべてのメソッドは deviceID を必ず受け取り、SQL は必ず `WHERE device_id = ?` を持つ。
//     端末 ID の絞り込みを省く経路(全端末を見るメソッド)をこのインターフェースに足さない。
//   - 端末 ID はヘッダ由来の値だけを渡す(ボディ・クエリからは受け取らない。ADR-0209 §2)。
//   - record-svc は自分の DB(record DB)にだけ触る(CLAUDE.md 絶対ルール4)。
//   - TiDB に届かない失敗は ErrUnavailable で包んで返す(httpapi が 503 store_unavailable に写す)。
package store

import (
	"context"
	"errors"
	"time"
)

// ErrUnavailable は record DB(TiDB)に届かない・クエリが実行できない失敗。実装はこれで包んで返し、
// httpapi は 503 `store_unavailable`(ADR-0209 §5.3)に写す。DB のエラー文はログにだけ残し、
// クライアントへは出さない(ADR-0105 §2 と同じ扱い)。
var ErrUnavailable = errors.New("record DB に届かない")

// FrequentOpponent は「よく使う相手」1件(ADR-0209 §3 #2)。api.FrequentOpponent に写す。
type FrequentOpponent struct {
	SpeciesKey       string
	Score            float64 // 頻度 × 時間減衰(半減期は設定値。生イベントの保持期間90日より短い)
	Count            int     // 減衰前の件数
	LastCalculatedAt time.Time
}

// Deleted は1回の削除で消した行数(api.RecordDeletionResult.Deleted と同じ内訳)。
type Deleted struct {
	CalcEvents int
	Aggregates int
	Favorites  int
}

// PurgeResult は PurgeDevice の結果。
type PurgeResult struct {
	// PurgedAt は墓石の時刻。PurgeDevice を呼ぶたびに現在時刻へ更新する(ADR-0209 §5.2 AC-P1b)。
	PurgedAt time.Time
	// Deleted はこの呼び出しで消した行数(冪等なので2回目は全て 0)。
	Deleted Deleted
	// Remaining が true なら1回の上限に達して残りがある(httpapi は status: partial を返す)。
	Remaining bool
}

// SaveOutcome は SaveCalcEvent の結果の種別。
type SaveOutcome int

const (
	// Stored は保存した(集計にも反映した)。
	Stored SaveOutcome = iota
	// Duplicate は同じ EventID をすでに保存済みで、何もしなかった(at-least-once の再配送。ADR-0212 §6)。
	// 集計も devices.last_seen_at も二重に進めない。
	Duplicate
	// Tombstoned は OccurredAt <= devices.purged_at なので保存しなかった(ADR-0209 §7)。
	// 呼び出し側はメッセージを ack して捨てる(再配送のループにしない)。
	Tombstoned
)

// CalcEvent は JetStream から受けた1件の計算イベントを record DB の行にする形
// (services/internal/calcevents.Event からの写像)。
type CalcEvent struct {
	// EventID は重複排除キー。JetStream のストリームシーケンス由来で、同じメッセージの再配送でも
	// 同じ値になること(受信時刻・ランダム値を混ぜない)。calc_events に一意制約を張る。
	EventID string

	DeviceID   string
	SessionID  string
	Operation  string    // calcevents.Operation*(calc / calcBulk / calcReverse)
	OccurredAt time.Time // calc-svc の時計。受信時刻ではない(ADR-0209 §7)

	// DefenderSpeciesKey は「よく使う相手」の集計キー。Operation が calc 以外(Detail の無い
	// calcBulk / calcReverse)では空文字で、集計には寄与しない。
	DefenderSpeciesKey string

	// Payload は calc_events に保存する本体(計算時点の個体スナップショット等)の JSON。
	// record-svc はこの中身をログに出さない(ADR-0209 §3・coding-rules §1)。
	Payload []byte
}

// Store は record-svc が record DB に対して行う操作のすべて。
type Store interface {
	// TouchDevice は devices.last_seen_at を now へ更新する(行が無ければ作る)。
	// 保存済みの last_seen_at から24時間経っていなければ**書かない**(ADR-0209 §4 AC-R5)。
	TouchDevice(ctx context.Context, deviceID string, now time.Time) error

	// FrequentOpponents はその端末の集計を Score の降順(同点は SpeciesKey の昇順)で
	// 最大 limit 件返す。記録が無ければ長さ0のスライスを返す(エラーにしない)。
	FrequentOpponents(ctx context.Context, deviceID string, limit int) ([]FrequentOpponent, error)

	// PurgeDevice はその端末のデータを ADR-0209 §5.2 の順序で消す:
	//  (1) devices.purged_at を now に更新し purge_journal に追記 → (2) calc_events → (3) 集計 → (4) favorites。
	// 1回で消す行数には上限があり、達したら PurgeResult.Remaining = true で返す(残りは次の呼び出し)。
	// すでに何も無い端末でもエラーにせず、Deleted が全て 0 の結果を返す。
	PurgeDevice(ctx context.Context, deviceID string, now time.Time) (PurgeResult, error)

	// SaveCalcEvent は1件の計算イベントを保存し、集計と devices.last_seen_at(24時間規則つき)へ
	// 反映する。冪等であること: 同じ EventID を2回渡しても行・集計・last_seen_at が二重にならず、
	// 2回目は Duplicate を返す(ADR-0212 §6 の at-least-once 配送)。
	// ev.OccurredAt <= devices.purged_at のときは何も保存せず Tombstoned を返す(ADR-0209 §7)。
	SaveCalcEvent(ctx context.Context, ev CalcEvent) (SaveOutcome, error)
}
