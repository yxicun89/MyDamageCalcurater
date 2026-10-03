// Package official は公式ページの販売状況の監視(フェーズ4-3。apps/wishlist/CLAUDE.md §12、docs/phase4-spec.md AC-O*)。
//
//   - Judge・ExtractText(純関数):本文のテキストから決まった語(Terms)を探し、種類がちょうど 1 つのときだけ状態にする。
//     サイト固有の構造は推測しない(語の一致だけ)
//   - ParseRobots(純関数):robots.txt の解析と、パスを取ってよいかの判定
//   - Checker:robots.txt を確かめてから公式ページを取得し、判定する(夜間の CronJob の RefreshAll だけが使う)
//
// 実サイトへのアクセスはテストからは行わない(httptest を使う。仕様 §13)。
package official

import "example.com/pokecalc/apps/wishlist/api/internal/item"

// TermSet は 1 つの状態と、その状態を示す語。
type TermSet struct {
	State item.OfficialState
	Words []string
}

// Terms は判定に使う語(docs/phase4-spec.md の表と同じ。変えるときは spec も変える)。
var Terms = []TermSet{
	{item.OfficialAvailable, []string{"販売中", "在庫あり", "カートに入れる", "購入手続きへ"}},
	{item.OfficialPreorder, []string{"予約受付中", "予約する", "予約受付"}},
	{item.OfficialSoldOut, []string{"在庫切れ", "売り切れ", "SOLD OUT", "在庫なし"}},
	{item.OfficialEnded, []string{"販売終了", "受付終了", "予約受付終了", "販売を終了"}},
}

// Neutral は、Terms の語を含むが意味が逆・別になる語。長い語として先に消し、どの種類にも数えない(根拠にもしない)。
// 例:「販売中止」は「販売中」を、「在庫ありません」は「在庫あり」を、「予約受付前」は「予約受付」を含む。
var Neutral = []string{"販売中止", "在庫ありません", "予約受付前", "予約受付開始前"}

// Result は 1 回の試行の結果(item.OfficialCheck の State・Evidence)。
type Result struct {
	State    item.OfficialState
	Evidence []string
}

// ExtractText は HTML の body のテキスト(body が無ければ文書全体)を返す。script・style・noscript・template の中身は除く。
// 要素の境目には空白を入れる(「在庫」「あり」が別の要素なら「在庫 あり」になり、語として一致しない)。
// 結果は NFKC で正規化し、連続する空白(改行・全角空白を含む)を 1 つの半角空白にして前後を除く。読めない HTML は空。
func ExtractText(html []byte) string {
	return "" // TODO(implementer)
}

// Judge はテキスト(ExtractText の結果)から状態を決める。
//
//   - テキストと語をそれぞれ NFKC・小文字にし、連続する空白を 1 つにしてから比べる(SOLD OUT は「sold out」「ＳＯＬＤ ＯＵＴ」とも一致)
//   - Terms と Neutral の全語を長い順(rune 数。同じ長さは Terms の順、Neutral は後)に探し、見つけた箇所をテキストから消してから次の語を探す
//     (「予約受付終了」は ended として数え、その中の「予約受付」「受付終了」は数えない)
//   - 見つけた種類(Neutral を除く)がちょうど 1 つならその状態、0 なら unknown、2 つ以上なら ambiguous
//   - Evidence は見つけた語(Terms に書いた表記のまま、重複なし)を、テキストに最初に現れた位置の順に最大 3 つ。unknown なら空(nil にしない)
func Judge(text string) Result {
	return Result{} // TODO(implementer)
}
