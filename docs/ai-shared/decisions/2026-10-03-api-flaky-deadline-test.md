## 2026-10-03: calc の期限切れテストの不安定は実装側の競合だった(API レーン。issue 538・ADR-0801 追記)
Decision: `httpguard.Expired` が `ctx.Err()` だけを見ていて、締め切り直後に timer が context を終了させる前だと期限切れを取りこぼした。
`ctx.Deadline()` と現在時刻の比較も加えて直した。テストは弱めていない(期待は「期限切れの要求は engine を呼ばない」のまま)。
Reason: `-count=500` で bulk が 200 を返す失敗を再現。実時間・sleep への依存ではなく、実装の競合。
Impact: httpguard の 4 つの複製(services・balance・speed・judge)に同じ変更。他レーンのサービスにも効くが挙動は期限切れ判定が正確になるだけ。
