## 2026-09-21: Claude Code から Codex へのレビュー依頼を行わない(ユーザー指示)
Decision: Claude Code は Codex を(codex exec / scripts/codex-review.sh で)レビューに使わない。レビューは critic のみ。ユーザーが別ターミナルで開いている Codex のセッションは、並行作業のため止めず、Claude Code も操作しない。
Reason: ユーザーが「Codex でレビューは止めてほしい。別ターミナルで開いているものは並行作業のため開いたままにしたい」と指示した。
Impact: .claude/skills/phase の手順6と CLAUDE.md のワークフロー表を更新。コーディング規約 v2 の Codex 確認(条件付き承認)は、この指示の前に行ったもの。今後の規約の変更の確認は、Codex が自分のセッションで DECISIONS.md に書く形にする。
