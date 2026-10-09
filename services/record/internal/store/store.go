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

// MaxFavoritesPerDevice は1端末が持てるお気に入りの上限(契約の createFavorite の description と
// listFavorites の maxItems と同じ値。ADR-0227 §3)。CreateFavorite はこれに達している端末からの
// (重複でない)作成を ErrFavoriteLimitReached で断る。
const MaxFavoritesPerDevice = 100

// ErrNotFound は「その端末がその ID のお気に入りを持っていない」。他端末のものか実在しないかを
// **区別しない**(区別すると存在の有無が漏れる。ADR-0209 §6-2)。httpapi は 404 `not_found` に写す。
var ErrNotFound = errors.New("この端末のお気に入りに無い")

// ErrFavoriteLimitReached は MaxFavoritesPerDevice に達している端末からの作成。httpapi は
// 400 `invalid_input` に写す(ADR-0227 §3)。
var ErrFavoriteLimitReached = errors.New("1端末が持てるお気に入りの上限に達した")

// Favorite は保存済みのお気に入り1件(ADR-0227。record DB の favorites 表の1行)。
//
// Snapshot は httpapi が組み立てる**正規化済みの JSON**({"label":…,"individual":…}。既定値を補い、
// キーの順序が決まった形)で、store は中身を解釈しない(マスタも引かない)。重複の判定は
// Snapshot のバイト列の SHA-256(favorites.snapshot_hash)で行う(ADR-0227 §2)。
// 中身には利用者の自由入力(label)を含むので**ログに出さない**(ADR-0209 §3・AC-L1)。
type Favorite struct {
	ID         int64 // サーバーが発行する(AUTO_INCREMENT)。契約では10進の文字列(FavoriteId)
	SpeciesKey string
	Snapshot   []byte
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// FavoriteOutcome は CreateFavorite の結果の種別(ADR-0227 §2 の冪等性)。
type FavoriteOutcome int

const (
	// FavoriteCreated は新しく作った(httpapi は 201)。
	FavoriteCreated FavoriteOutcome = iota
	// FavoriteExisted は同じ端末に同じ Snapshot の行がすでにあり、新しく作らずに UpdatedAt だけ now に
	// 進めた(httpapi は 200)。上限に達していてもこちらは成功する(行は増えない)。
	FavoriteExisted
)

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

// CalcHistoryCursor は計算履歴の keyset の位置(ADR-0230 §1)。(OccurredAt, EventID) がこの位置より
// 古い(occurred_at DESC, event_id DESC の並びで後ろの)行だけを返すのに使う。
type CalcHistoryCursor struct {
	OccurredAt time.Time
	EventID    string
}

// CalcHistoryQuery は ListCalcHistory の問い合わせ(ADR-0230 §8)。
type CalcHistoryQuery struct {
	Since  time.Time          // occurred_at >= Since(ゼロ値なら下限なし。保持期間の下限)
	Before *CalcHistoryCursor // nil なら先頭(最新)から
	Limit  int
}

// CalcHistoryRow は履歴の1行。Payload は calc_events.payload で、store は中身を解釈しない。
type CalcHistoryRow struct {
	EventID    string
	OccurredAt time.Time
	Payload    []byte
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

	// --- お気に入り(ADR-0227。P5-3c)---------------------------------------------
	// 3つとも先に端末 ID で絞ってから ID・内容を照合する(ADR-0209 §6-2)。墓石(devices.purged_at)は
	// 見ない: 全削除の後に利用者が新しくピン留めしたものは保存する(墓石は計算イベントの再出現を
	// 止めるためのもの。ADR-0227 §6)。

	// ListFavorites はその端末のお気に入りを UpdatedAt の降順(同時刻は ID の降順)で全件返す。
	// 1件も無ければ長さ0のスライスを返す(エラーにしない)。他端末の行は含めない(§6-3)。
	ListFavorites(ctx context.Context, deviceID string) ([]Favorite, error)

	// CreateFavorite はお気に入りを1件作る。ID・CreatedAt・UpdatedAt は store が決め(引数の
	// fav.ID / fav.CreatedAt / fav.UpdatedAt は無視する)、保存後の内容を返す。
	//   - その端末に同じ Snapshot(SHA-256 が一致)の行があれば、作らずにその行の UpdatedAt を now に
	//     進めて返し、FavoriteExisted(上限の判定より先に行う)。同時に同じ要求が2つ来ても行は1つ
	//     (favorites の UNIQUE (device_id, snapshot_hash) で保証する。ADR-0227 §2)。
	//   - そうでなく、その端末がすでに MaxFavoritesPerDevice 件持っていれば ErrFavoriteLimitReached
	//     (件数の確認と挿入は同じトランザクションで行い、上限をすり抜けさせない)。
	//   - 作ったときは CreatedAt == UpdatedAt == now で FavoriteCreated。
	CreateFavorite(ctx context.Context, deviceID string, fav Favorite, now time.Time) (Favorite, FavoriteOutcome, error)

	// DeleteFavorite はその端末のお気に入りを1件消す。その端末が持っていなければ ErrNotFound
	// (2回目の削除も ErrNotFound。他端末の行は消さない)。
	DeleteFavorite(ctx context.Context, deviceID string, favoriteID int64) error

	// --- 計算履歴(ADR-0230)------------------------------------------------------

	// ListCalcHistory はその端末の operation = calc の行を occurred_at DESC, event_id DESC で最大 q.Limit 行返す。
	// devices.purged_at(墓石)以前の行・q.Since より古い行・q.Before 以降(新しい側)の行は含めない。
	// 1行も無ければ長さ0のスライス。読むだけで何も書き換えない。届かなければ ErrUnavailable。
	ListCalcHistory(ctx context.Context, deviceID string, q CalcHistoryQuery) ([]CalcHistoryRow, error)
}
