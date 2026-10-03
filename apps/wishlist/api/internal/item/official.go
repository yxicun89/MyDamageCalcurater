package item

import (
	"context"
	"errors"
	"time"
)

// フェーズ4-3 公式サイトの販売状況の監視(docs/phase4-spec.md AC-O*)。

// OfficialState は公式ページの販売状況(official_status.status・last_result)。
type OfficialState string

const (
	OfficialAvailable OfficialState = "available" // 販売中
	OfficialPreorder  OfficialState = "preorder"  // 予約受付中
	OfficialSoldOut   OfficialState = "soldout"   // 在庫切れ
	OfficialEnded     OfficialState = "ended"     // 販売終了
	OfficialUnknown   OfficialState = "unknown"   // 決まった語が無い(判定できない)
	OfficialAmbiguous OfficialState = "ambiguous" // 別の種類の語が両方ある(判定できない)
	OfficialBlocked   OfficialState = "blocked"   // robots.txt が許していないので取得しない
	OfficialFailed    OfficialState = "failed"    // 取得できなかった
)

// OfficialStates は全状態(API の enum と同じ順)。
var OfficialStates = []OfficialState{
	OfficialAvailable, OfficialPreorder, OfficialSoldOut, OfficialEnded,
	OfficialUnknown, OfficialAmbiguous, OfficialBlocked, OfficialFailed,
}

// Valid は 8 つの状態のどれかか。
func (s OfficialState) Valid() bool {
	for _, v := range OfficialStates {
		if s == v {
			return true
		}
	}
	return false
}

// Judged はページを取得して判定した結果か(failed・blocked 以外の 6 つ)。
func (s OfficialState) Judged() bool {
	return s.Valid() && s != OfficialBlocked && s != OfficialFailed
}

const (
	// MaxOfficialEvidence は根拠として保存する語の数の上限。
	MaxOfficialEvidence = 3
	// MaxOfficialEvidenceLen は根拠の 1 語の上限(rune)。
	MaxOfficialEvidenceLen = 64
)

// OfficialCheck は 1 回の試行の結果(取得 → 判定)。At はその時刻。
type OfficialCheck struct {
	State    OfficialState
	Evidence []string
	At       time.Time
}

// OfficialStatus は保存した販売状況(official_status。1 商品 1 行)。時刻は秒未満を切り捨てた UTC。
//
//   - Status・Evidence・CheckedAt は「最後に判定できた状態」とそれを確かめた時刻。最後の試行が failed・blocked でも、
//     判定済み(Judged)の Status は上書きしない。一度も判定できていなければ Status は failed・blocked(Evidence は空)
//   - ChangedAt・PreviousStatus は、判定済みの Status が別の判定済みの状態に変わったときだけ更新する(初回・failed/blocked からの判定は変化にしない)
//   - LastResult・LastAttemptAt は最後の試行の結果と時刻(failed・blocked を含む)
type OfficialStatus struct {
	Status         OfficialState
	Evidence       []string
	CheckedAt      time.Time
	ChangedAt      *time.Time
	PreviousStatus *OfficialState
	LastResult     OfficialState
	LastAttemptAt  time.Time
}

// MergeOfficial は前回の状態 prev(無ければ nil)に試行の結果 c を重ねた新しい状態を返す(純関数。規則は OfficialStatus の説明)。
// 時刻は秒未満を切り捨てた UTC にする。Evidence は c のものをコピーする(failed・blocked で上書きしないときは prev のもの)。
func MergeOfficial(prev *OfficialStatus, c OfficialCheck) OfficialStatus {
	return OfficialStatus{} // TODO(implementer): docs/phase4-spec.md 4-3 の規則で実装する
}

// WatchTarget は夜間に公式ページを取りに行く商品なら、その URL(source_url)と true を返す。
// WatchOfficial が true で、SourceURL が http(s) の絶対 URL(ホストあり)のときだけ。
func WatchTarget(it Item) (string, bool) {
	return "", false // TODO(implementer)
}

// errNotImplemented はスタブの戻り値(implementer が消す)。
var errNotImplemented = errors.New("item: not implemented")

// OfficialRepository は販売状況の永続化。
type OfficialRepository interface {
	// SaveOfficialCheck は商品の状態を MergeOfficial(前回, c) で置き換え、保存した状態を返す(1 トランザクション)。
	// 商品が無い → ErrNotFound。c.State が 8 つのどれでもない・Evidence が MaxOfficialEvidence 個を超える・
	// 1 語が MaxOfficialEvidenceLen 文字を超える・空の語 → ErrInvalid(どれも何も変えない)。
	// 保存した状態は Repository の ListItems・GetItem の Item.Official に出る。
	SaveOfficialCheck(ctx context.Context, itemID int64, c OfficialCheck) (OfficialStatus, error)
}
