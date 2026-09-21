# Current State

## Damage Calculator
Owner: Claude Code
Branch: feat/claude-p1-engine
Status: Phase 1 (計算エンジン) 実装中。P1-8 逆算まで完了(critic 2回(FAIL→FAIL)の指摘反映後、指示された修正をメインが確認して完了。3回目の critic 実行はしていない。任意の外部 Codex レビュー(scripts/codex-review.sh)は未実施。Codex の担当はタイプバランスチェッカー実装で、ダメージ計算のレビュー担当ではない)
Next: P1-9 WASM → Phase 2。人間の確認待ち: hb_boost / hd_boost の定義、表示 % の丸め(いずれも plan.md ブロッカー)

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
