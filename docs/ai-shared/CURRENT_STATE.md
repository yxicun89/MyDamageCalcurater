# Current State

## Damage Calculator
Owner: Claude Code
Branch: feat/claude-p1-engine
Status: Phase 1 (計算エンジン) 実装中。P1-7 一括計算まで完了(critic 2回目 PASS。Codex レビューは Codex のレートリミットで未実施)
Next: P1-8 逆算 → P1-9 WASM。hb_boost / hd_boost の定義は人間の確認待ち(plan.md ブロッカー)

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
