## 2026-09-23: 承認省略の設定(git push / gh pr create / gh pr merge を自動承認、rm は据え置き。ユーザー決定)
Decision: Claude Code のユーザー設定ファイル(全レーン共通のグローバル設定)で `git push`・`gh pr create`・`gh pr merge` を確認なし(allow)にした。
一方、main への直接 push・force push・`--mirror`・`--all` は引き続き禁止(deny)、リモートブランチの `--delete` は引き続き確認が要る(ask)。
`rm` は変更していない(ユーザーが「何を破壊するか分からなくて怖い」ため明示的に据え置きを希望。auto mode の既定判断のまま)。
Reason: ユーザーの言葉「ローカルでのmain直接マージは良くないけどpushとpr mergeは許可した方が承認する手間省けるから許可するルールにしたい」
「rmにかんしては何を破壊するか分からなくて怖いので承認します」。承認の手間を減らしつつ、破壊的操作(直接 push・force push・rm)は従来どおり止める/確認する。
Impact: **運用上の注意(COORDINATION.md に追記済み)**: `gh pr merge` の直前にユーザーが目を通す機会が無くなるため、PR を作る前に
テスト・lint・check-publishable・独立レビュー(critic)PASS を自分で確認することが、これまで以上に唯一の安全網になる。
ローカルで `git merge` して `origin/main` へ直接 push することは、main への直接 push を拒否する deny ルールで従来どおり止まる。
main への統合は必ず PR(`gh pr create` → `gh pr merge`)を経由する。
