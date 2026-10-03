# docs/decisions/ — 判断の記録(1件1ファイル)

2026-10-03 から、レーン間の判断・提案・依頼・ユーザー回答はここに1件1ファイルで書く(ADR-0170)。
それ以前の記録は凍結アーカイブ [../ai-shared/DECISIONS.md](../ai-shared/DECISIONS.md)(追記・編集しない)。

- 作る: `scripts/new-decision.sh <lane> <slug> [タイトル]` → `docs/decisions/<YYYY-MM-DD>-<lane>-<slug>.md`
  (`<lane>`: data api web ios tb speed judge ops、全レーンにかかるものは shared)
- 未決の一覧: `scripts/list-decisions.sh`(状態が `open`・`未回答`)。全件は `--all`、アーカイブの見出しは `--archive`
- 先頭の項目: `- 日付:`・`- レーン:`(発信 → 宛先)・`- 状態:`(`open` / `未回答` / `決定` / `撤回`)・`- 状況:`・`- 既定案:`
- 本文は `Decision:` / `Reason:` / `Impact:` か箇条書き。既存エントリの本文は編集しない(状態の行だけは発信レーン・宛先レーンが更新してよい)
- 運用の全体は [../ai-shared/COORDINATION.md](../ai-shared/COORDINATION.md)「共有ログの構造」
