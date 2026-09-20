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
- fix/codex-workflow-golden の再検討(Codex が次に着手する際)。マージ時は AGENTS.md / CLAUDE.md の統合が必要
- pokecalc-kit-v2/ は反映が未確認の差分があるため削除していない(Sonnet 化・deep-critic 等。採否は人間の判断待ち)

### Next
- Damage Calculator: P1-6 は保留ブランチの内容を踏まえて新規タスクとして扱うか判断が必要
