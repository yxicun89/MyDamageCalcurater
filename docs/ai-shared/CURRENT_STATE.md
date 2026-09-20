# Current State

## Damage Calculator
Owner: Claude Code
Branch: main
Status: Phase 1 (計算エンジン) 実装中。P1-5 まで完了(main)。P1-6(golden)相当の作業は fix/codex-workflow-golden に保全済み・未マージ
Next: P1-6。保留ブランチ(ADR-0008 の丸め順訂正を含む)を新規タスクとして再検討するか判断してから着手

## Type Balance Checker
Owner: Codex
Branch: 保留中(fix/codex-workflow-golden は main へ未マージ。理由は DECISIONS.md 参照。TB 用のブランチはまだ無い)
Status: 未着手
Next: Codex がレートリミット解除後、feat/codex-tb0-foundation で TB0(基盤・Kustomize・Argo CD Application 定義)を
新規開始する(CODEX_KICKOFF.md どおり。引き継ぎ作業ではない)

## Shared Interfaces
- Pokemon ID: pokedex-svc の `{図鑑番号4桁}-{フォルム3桁}` 形式に準拠
- Type: 18タイプの英語小文字ID(fire, water, ...)。表示名・色は docs/design.md のトークンに準拠
- Type multiplier: 分数ではなく整数表現(claude-review.md 参照)
- balance-svc は pokedex-svc の REST API を呼ぶ。DBには直接繋がない
