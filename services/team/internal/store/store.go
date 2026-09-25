// Package store は team-svc のデータアクセス層の契約(型とインターフェース)を持つ。
// TiDB への実装(SQL・接続・上限の設定値)は同じパッケージの `tidb.go`(`//go:build tidb` の
// テストは `tidb_test.go`)。httpapi と events はこのインターフェースだけに依存し、単体テストは
// 架空の実装(fake)で差し替える。record-svc の `services/record/internal/store` と同じ流儀。
//
// 規則(ADR-0209 §6・ADR-0213。このインターフェースを実装するときの絶対条件):
//   - すべてのメソッドは deviceID を必ず受け取り、SQL は必ず `WHERE device_id = ?` を持つ。
//     端末 ID の絞り込みを省く経路(全端末を見るメソッド)をこのインターフェースに足さない。
//   - リソース ID(teamID)を受けるメソッドは、**先に端末 ID で絞ってから** ID を照合する。
//     他端末の構築・実在しない構築はどちらも ErrNotFound(呼び出し側が 404 not_found にする。§6-2)。
//   - 端末 ID はヘッダ由来の値だけを渡す(ボディ・クエリ・パスからは受け取らない。ADR-0209 §2)。
//   - team-svc は自分の DB(team DB)にだけ触る(CLAUDE.md 絶対ルール4)。pokedex のマスタを
//     引かないので、speciesKey / moveIDs / itemID / abilityID / natureID の実在は検証しない
//     (ADR-0213 §3)。
//   - **計算イベントの中身を保存するメソッドを足さない**(ADR-0209 §4・ADR-0213 §5)。team-svc が
//     イベントから行うのは devices.last_seen_at の更新だけで、個体・ダメージは読み捨てる。
//   - TiDB に届かない失敗は ErrUnavailable で包んで返す(httpapi が 503 store_unavailable に写す)。
package store

import (
	"context"
	"errors"
	"time"
)

// MaxTeamsPerDevice は1端末が持てる構築の上限(契約の createTeam の description と同じ値。
// ADR-0213 §2)。CreateTeam はこれに達している端末からの作成を ErrTeamLimitReached で断る。
const MaxTeamsPerDevice = 100

// MaxMembersPerTeam / MaxMovesPerMember は構築の形の上限(契約の maxItems と同じ値。
// requirements.md §2・ADR-0213 §3。iOS の TeamLimits と同じ)。
const (
	MaxMembersPerTeam = 6
	MaxMovesPerMember = 4
)

// ErrUnavailable は team DB(TiDB)に届かない・クエリが実行できない失敗。実装はこれで包んで返し、
// httpapi は 503 `store_unavailable`(ADR-0209 §5.3)に写す。DB のエラー文はログにだけ残し、
// クライアントへは出さない(ADR-0105 §2 と同じ扱い)。
var ErrUnavailable = errors.New("team DB に届かない")

// ErrNotFound は「その端末がその ID の構築を持っていない」。他端末のものか実在しないかを
// **区別しない**(区別すると存在の有無が漏れる。ADR-0209 §6-2)。httpapi は 404 `not_found` に写す。
var ErrNotFound = errors.New("この端末の構築に無い")

// ErrTeamLimitReached は MaxTeamsPerDevice に達している端末からの作成。httpapi は
// 400 `invalid_input` に写す(ADR-0213 §2)。
var ErrTeamLimitReached = errors.New("1端末が持てる構築の上限に達した")

// StatBlock は能力ポイント(SP)の6ステータス(api.StatBlock と同じ形。ADR-0213 §3)。
// 値の範囲(各 0..32・合計 66 以下)の検証は httpapi の責務で、store は受けた値をそのまま保存する。
type StatBlock struct {
	HP  int
	Atk int
	Def int
	Spa int
	Spd int
	Spe int
}

// Member は構築の1体。ID(種族・技・持ち物・特性・性格)はマスタに実在するか検証せずそのまま持つ
// (ADR-0213 §3)。パーティの何番目かは Team.Members の並び順そのもので、slot は持たない
// (DB の team_members.slot は保存時に添字から決める)。
type Member struct {
	SpeciesKey string
	// Nickname は未設定なら nil(空文字は httpapi が nil に正規化する)。利用者の自由入力なので
	// **ログに出さない**(ADR-0209 §3)。
	Nickname *string
	// MoveIDs は最大 MaxMovesPerMember 個で、同一メンバー内で重複しない(検証は httpapi)。
	MoveIDs   []string
	ItemID    *string
	AbilityID *string
	NatureID  string
	SP        StatBlock
	// TeraType は api.PokeType の文字列(未設定なら nil)。
	TeraType *string
}

// Team は保存済みの構築1件。ID と時刻はサーバー(store)が決める。
type Team struct {
	ID        string
	Name      string
	Members   []Member
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Deleted は1回の全削除で消した行数(api.TeamDeletionResult.Deleted と同じ内訳)。
type Deleted struct {
	Teams       int
	TeamMembers int
}

// PurgeResult は PurgeDevice の結果(record-svc の store.PurgeResult と同じ形)。
type PurgeResult struct {
	// PurgedAt は墓石の時刻。PurgeDevice を呼ぶたびに現在時刻へ更新する(ADR-0209 §5.2 AC-P1b)。
	PurgedAt time.Time
	// Deleted はこの呼び出しで消した行数(冪等なので2回目は全て 0)。
	Deleted Deleted
	// Remaining が true なら1回の上限に達して残りがある(httpapi は status: partial を返す)。
	Remaining bool
}

// TouchOutcome は TouchDeviceFromEvent の結果の種別(ADR-0209 §4・§7)。
type TouchOutcome int

const (
	// Touched は devices.last_seen_at を進めた。
	Touched TouchOutcome = iota
	// Skipped は直近の last_seen_at から24時間以内だったので書かなかった(AC-R5)。
	// 同じイベントの再配送(at-least-once。ADR-0212 §6)も必ずこちらになるので、
	// team-svc は event_id の重複排除表を持たなくても冪等になる(ADR-0213 §5)。
	Skipped
	// Tombstoned は occurredAt <= devices.purged_at なので何もしなかった(ADR-0209 §7)。
	// 呼び出し側はメッセージを ack して捨てる(再配送のループにしない)。
	Tombstoned
)

// Store は team-svc が team DB に対して行う操作のすべて。
//
// **このインターフェースにメソッドを足すときの条件**(ADR-0213 §5): 計算イベントの中身
// (個体・技・ダメージ)を受け取る・保存するメソッドを足さないこと。team-svc がイベントから
// 行ってよいのは devices.last_seen_at の更新だけで、それ以外は record-svc の担当。
type Store interface {
	// TouchDevice は HTTP 要求を受けたときに devices.last_seen_at を now へ更新する(行が無ければ作る)。
	// 保存済みの last_seen_at から24時間経っていなければ**書かない**(ADR-0209 §4 AC-R5)。
	TouchDevice(ctx context.Context, deviceID string, now time.Time) error

	// TouchDeviceFromEvent は JetStream の計算イベントを受けたときの devices.last_seen_at の更新
	// (ADR-0209 §4。AC-R2d)。24時間規則は TouchDevice と同じで、基準時刻はイベントの
	// occurredAt(calc-svc の時計。受信時刻ではない)。occurredAt <= devices.purged_at なら
	// 何もせず Tombstoned を返す(ADR-0209 §7)。
	// **イベントの中身は受け取らない**(引数が端末 ID と時刻だけであること自体が ADR-0209 §4 の
	// 「team-svc は計算の中身を保存しない」の実装上の保証。ADR-0213 §5)。
	TouchDeviceFromEvent(ctx context.Context, deviceID string, occurredAt time.Time) (TouchOutcome, error)

	// ListTeams はその端末の構築を UpdatedAt の降順(同時刻は ID の昇順)で全件返す。
	// 1件も無ければ長さ0のスライスを返す(エラーにしない)。他端末の構築は含めない(§6-3)。
	ListTeams(ctx context.Context, deviceID string) ([]Team, error)

	// GetTeam はその端末の構築を1件返す。その端末が持っていなければ ErrNotFound(§6-2)。
	GetTeam(ctx context.Context, deviceID, teamID string) (Team, error)

	// CreateTeam は構築を1件作る。ID(UUID)・CreatedAt・UpdatedAt は store が決め(引数の
	// t.ID / t.CreatedAt / t.UpdatedAt は無視する)、保存後の内容を返す。
	// その端末がすでに MaxTeamsPerDevice 件持っていれば ErrTeamLimitReached(件数の確認と
	// 挿入は同じトランザクションで行い、上限をすり抜けさせない)。
	CreateTeam(ctx context.Context, deviceID string, t Team, now time.Time) (Team, error)

	// UpdateTeam はその端末の構築 teamID の名前とメンバー全体を t の内容で置き換え、UpdatedAt を
	// now にして返す(部分更新はしない。ADR-0213 §2)。その端末が持っていなければ ErrNotFound。
	UpdateTeam(ctx context.Context, deviceID, teamID string, t Team, now time.Time) (Team, error)

	// DeleteTeam はその端末の構築を1件消す(team_members も一緒に消す)。その端末が持っていなければ
	// ErrNotFound(2回目の削除も ErrNotFound。ADR-0213 §2)。
	DeleteTeam(ctx context.Context, deviceID, teamID string) error

	// PurgeDevice はその端末のデータを ADR-0209 §5.2 の順序で消す:
	//  (1) devices.purged_at を now に更新し purge_journal に追記 → (2) team_members → (3) teams。
	// 1回で消す行数には上限があり、達したら PurgeResult.Remaining = true で返す(残りは次の呼び出し)。
	// すでに何も無い端末でもエラーにせず、Deleted が全て 0 の結果を返す(404 にしない。§5.2)。
	PurgeDevice(ctx context.Context, deviceID string, now time.Time) (PurgeResult, error)
}
