## 2026-09-24: issue #113(クライアントのcancel伝播)のAPIレーン連携分を修正(API レーン)
Decision: Web/iOSレーンは自レーン分(200ms trailing debounce・`AbortSignal`/`Task` cancel)を完了済み
(PR #174・iOS側の対応コミット)だったが、issue #113の担当レーン欄に「連携: APIレーン（クライアントのcancel伝播）」
とあったため実測で調べたところ、`services/gateway/internal/httpapi/proxy.go` の
`httputil.ReverseProxy.ErrorHandler` に見落としがあった: クライアントが要求を中断すると Go の `http.Server` が
`r.Context()` を `context.Canceled` で終えるが、gateway はこれを「上流に到達できない」と区別せず、
WARN ログを出し 503 `upstream_unavailable` を返していた(実際は上流もgatewayも正常で、クライアントが単に
離脱しただけ)。実測で確認: クライアント中断時のエラーは文字どおり `"context canceled"`、gateway 自身の
`GATEWAY_UPSTREAM_TIMEOUT` が切れたときのエラーは `"net/http: timeout awaiting response headers"` で、
両者は判別可能。`errors.Is(err, context.Canceled)` のときだけ特別扱いし、WARN ログを出さず(Debug に留める。
運用上のノイズ・誤検知を避けるため)、応答も書かない(クライアントは既に居ないので届かない)ように修正。
`calc-svc` 側は元から問題なし: `httpErrorHandler` が `r.Committed` を先にチェックしており、応答コミット後の
書き込み失敗(クライアント切断)は静かに無視される設計だった(確認のみ・変更なし)。
`TestClientCancelIsNotUpstreamUnavailable`(`services/gateway/internal/httpapi/upstream_test.go`)を追加。
wall-clock sleep を使わず、RoundTripper を差し替えて「RoundTrip が呼ばれた印を送ってから context が終わるまで
ブロックする」フェイク版と、実際の `http.Transport`(本番と同じ)がクライアント中断時に本当に
`context.Canceled` を返すことを固定する版の両方を用意。変異テスト(特別扱いの分岐を無効化)で
両方が実際に検知することを確認(確認後 revert)。ADR-0202 §5 に追記・受け入れ条件表に AC-G10 を追加。

**限界(調査で判明。今回は対応しない)**: gateway → calc-svc への `context` のキャンセル伝播そのものは
効く(実測で上流ハンドラの `r.Context().Done()` が即座に発火することを確認)。しかし `services/calc/` の
ハンドラは受け取ったリクエストの `context.Context` を一度も見ておらず、`engine`(`CalcReverse`/`CalcBulk`)も
`context` を受け取らない(絶対ルール2「engine は純粋に保つ」により、キャンセル検査のためだけに `context` を
持ち込む変更はしない判断)。**そのため issue #113 の達成目標「calc-svc CPU消費も止める」は本修正の範囲では
達成しない**: クライアントが中断しても、gateway は静かに応答を打ち切るだけで、calc-svc 側の計算(特に逆算の
総当たり探索)は最後まで完走する。1リクエストあたりの最悪計算量は ADR-0208(件数・範囲の上限)で有界なので、
実害は「無駄な計算がその上限の範囲で起こりうる」程度に留まる。engine への `context` 導入が必要になったら
別 ADR で扱う。
Reason: issue #113。ブラウザ・iOSでの正常な入力操作(型変換中の中間値の破棄)のたびに、gateway が
「上流に到達できない」という誤った警告ログを出し続けると、運用時の異常検知(将来のSLOダッシュボード等)の
ノイズになる。
Impact: `docs/plan.md` の改善要望に issue #113 のAPIレーン連携分を追加。issue #113 の受け入れ条件8項目は
すべてWeb/iOS固有(debounce・AbortSignal・Task cancel等)で gateway 側の条件は無く、「対象外」も engine の
計算式・gateway/calcのtimeout値・横断rate limitを明示的に除外しているため、上記の限界(calc-svc CPUは
止まらない)を残したまま**issue #113 はWeb・iOS・APIのすべてのレーン分が完了したとしてこのPRのマージで
クローズしてよい**と判断。他レーンへの追加対応は無し。
