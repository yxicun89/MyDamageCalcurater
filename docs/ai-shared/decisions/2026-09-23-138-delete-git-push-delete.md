## 2026-09-23: リモートブランチの `--delete`(git push --delete)も自動承認にする(ユーザー決定)
Decision: Claude Code のユーザー設定ファイル(グローバル)とこのプロジェクトの `.claude/settings.json`(Git管理下)の両方で、
`git push * --delete*`/`git push --delete*` を ask から削除した(広い `Bash(git push *)` の allow がそのまま効くようになる)。
main への直接push・force push・`--mirror`・`--all` は引き続き禁止のまま。`rm` も変更していない。
Reason: ユーザーの言葉「まだ承認出るので許可したい」。PR マージ後のフィーチャーブランチ削除は本セッションの通常フローで
毎回発生しており、確認プロンプトが挟まる運用負荷が大きかったため。
Impact: COORDINATION.md の運用上の注意を更新した。`gh pr merge` に続きリモートブランチ削除も人間が目を通す機会が無くなるため、
PR作成前のテスト・lint・check-publishable・critic PASS確認の重要性は変わらず高いまま。
