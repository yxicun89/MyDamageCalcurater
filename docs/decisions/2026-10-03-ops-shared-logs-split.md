# 共有ログを1エントリ=1ファイルに分けた(運用レーン → 全レーン。ADR-0170・ユーザー決定)

- 日付: 2026-10-03
- レーン: ops → 全レーン
- 状態: 決定
- 状況: 全レーンが DECISIONS.md・CURRENT_STATE.md・plan.md の末尾付近を編集し、PR が頻繁に衝突していた
- 既定案: なし(ユーザー決定「構造で解決する」)

Decision: 判断の記録は `docs/decisions/<YYYY-MM-DD>-<lane>-<slug>.md` に1件1ファイル(`scripts/new-decision.sh`)。
`docs/ai-shared/DECISIONS.md` は凍結アーカイブ(本文は不変・`merge=union`)。レーンの状態は `docs/ai-shared/state/<lane>.md`、
`CURRENT_STATE.md` は索引と Shared Interfaces だけ。plan は区画ごとに `docs/plan/<区画>.md`、`docs/plan.md` は索引とマイルストーン表だけ。
Reason: 別レーンの PR が同じ行・同じ末尾を触らなくなり、競合解決が不要になる。起動時に読む量も減る。
Impact: 全レーン。未マージの PR は `git merge origin/main` のときに COORDINATION.md「未マージの PR の移行」の手順で
自分の変更を新しいファイルへ書き直す。CLAUDE.md・AGENTS.md はパスの事実だけを訂正した(人間のレビュー対象)。
