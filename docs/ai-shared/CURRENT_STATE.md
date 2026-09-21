# Current State

## Damage Calculator
Owner: Claude Code
Branch: feat/claude-p1-engine
Status: Phase 1 と P2-1 は完了。ユーザー決定を docs に反映済み。コーディング規約 v2(docs/coding-rules.md)を起草し、Codex の再確認待ち。Phase 1b の P1-10(防御プリセット再定義)は実装・レビュー修正済みでコミット待ち。任意の外部 Codex レビュー(scripts/codex-review.sh)は未実施(規約 v2 のレビューだけ codex exec で依頼した)。Codex の担当はタイプバランスチェッカー実装で、ダメージ計算のレビュー担当ではない
Next: Phase R(R-1 監査 → R-2 是正 → R-3 check-publishable)→ P1-11 → P1-12 → P2-1b(golden の oracle 切替。先に diff)→ P2-1c(技の使用可否の調査)→ P2-2。人間の確認待ち(plan.md ブロッカー): 観測ダメージの入力と丸めの解釈、LICENSE の方針

## Type Balance Checker
Owner: Codex
Branch: なし(fix/codex-workflow-golden は main へマージ済み・削除済み。TB 用のブランチはまだ無い)
Status: 未着手
Next: Codex がレートリミット解除後、feat/codex-tb0-foundation で TB0(基盤・Kustomize・Argo CD Application 定義)を
新規開始する(CODEX_KICKOFF.md どおり。引き継ぎ作業ではない)

## Shared Interfaces
- Pokemon ID: pokedex-svc の `{図鑑番号4桁}-{フォルム3桁}` 形式に準拠
- Type: 18タイプの英語小文字ID(fire, water, ...)。表示名・色は docs/design.md のトークンに準拠
- Type multiplier: 分数ではなく整数表現(claude-review.md 参照)
- balance-svc は pokedex-svc の REST API を呼ぶ。DBには直接繋がない
