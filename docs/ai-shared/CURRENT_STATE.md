# Current State

## Damage Calculator
Owner: Claude Code
Branch: feat/claude-p1-engine
Status: Phase 1(計算エンジン)と P2-1(データソース調査)は完了。2026-09-21 のユーザー決定を docs に反映済み(ADR-0002 確定、requirements.md 修正、データ非コミット方針)。決定に伴うエンジン変更は Phase 1b(P1-10 プリセット再定義 → P1-11 表示%の分離 → P1-12 逆算の再設計)として未着手。任意の外部 Codex レビュー(scripts/codex-review.sh)は未実施。Codex の担当はタイプバランスチェッカー実装で、ダメージ計算のレビュー担当ではない
Next: P1-10 → P1-11 → P1-12 → P2-1b(golden の oracle 切替。先に diff)→ P2-2。人間の確認待ち(plan.md ブロッカー): 実機観測%の丸め規則、testdata/golden と非コミット方針の関係、技の使用可否の食い違い、メガ石・フォーム・更新運用

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
