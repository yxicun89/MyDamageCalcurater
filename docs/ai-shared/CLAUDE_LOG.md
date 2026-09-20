# Claude Log

## 2026-09-21 (チャットでのレビュー、Claude Code未起動)
### Done
- type_balance_architecture_review.md をレビュー、claude-review.md に記録
- CURRENT_STATE.md / DECISIONS.md の初版を作成

### Changed files
- docs/ai-shared/claude-review.md (new)
- docs/ai-shared/CURRENT_STATE.md (new)
- docs/ai-shared/DECISIONS.md (new)

### Decisions
- claude-review.md 参照

### Open issues
- なし(未決事項は全て決定済み)

### Next
- Claude Code は damage-calc の P1-3 を継続。balance 側には触れない

## 2026-09-21 (Claude Code: 宙に浮いたブランチの解消とブランチ運用ルール導入)
### Done
- fix/codex-workflow-golden を確認して保留判断(engine コア・API 契約に触れるため)。作業は 6d86382 として保全
- pokecalc-ai-shared の内容をリポジトリへ展開(docs/ai-shared/、docs/type-balance-design.md、CODEX_KICKOFF.md、AGENTS.md へ担当範囲を統合)
- AGENTS.md に「Git ブランチ運用」、CLAUDE.md にその要約と docs/ai-shared への参照を追記

### Decisions
- DECISIONS.md の「fix/codex-workflow-golden は保留」「Git ブランチ運用ルールを導入する」を参照

### Open issues
- (解決済み)fix/codex-workflow-golden はマージ済み。ただし engine の丸め順訂正・openapi の level 固定は独立レビュー未実施

### Later (同日)
- ユーザー指示により pokecalc-kit-v2/ と UNBLOCK_AND_MERGE_KICKOFF.md を削除。kit-v2 の差分
  (spec-writer/critic の Sonnet 化、deep-critic、ループ上限2回など)は採用しない判断で、現行の CLAUDE.md の運用を維持
- ユーザー判断で fix/codex-workflow-golden を main へマージ(保留を撤回)。AGENTS.md / CLAUDE.md の衝突を統合し、
  「Claude と Codex の作業を1ブランチで混ぜない」をブランチ運用に追記

### Next
- feat/claude-p1-engine を main から切り、P1-6([~])を critic でレビュー → [x] → P1-7 以降
