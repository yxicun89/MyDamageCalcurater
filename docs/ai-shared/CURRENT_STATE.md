# Current State

## Damage Calculator
Owner: Claude Code
Branch: main
Status: Phase 1 (計算エンジン) 実装中。P1-5 完了。P1-6(golden 照合・ADR-0008 の丸め順訂正)の Codex 作業を main へマージ済み。P1-6 は [~](独立レビュー未実施)
Next: feat/claude-p1-engine を main から切り、P1-6 を critic でレビューして [x] にしてから P1-7 へ

## Type Balance Checker
Owner: Codex
Branch: feat/codex-tb0-foundation(main の 8049702 から作成)
Status: TB0 はタイプ相性データの取得元・契約が不明なためブロック中。実装未着手
Next: pokedex-svc のタイプ相性 API 契約、または MySQL 取り込みデータのエクスポート仕様が確定したら、
同ブランチで TB0(基盤・Kustomize・Argo CD Application 定義)を再開する

## Shared Interfaces
- Pokemon ID: pokedex-svc の `{図鑑番号4桁}-{フォルム3桁}` 形式に準拠
- Type: 18タイプの英語小文字ID(fire, water, ...)。表示名・色は docs/design.md のトークンに準拠
- Type multiplier: 分数ではなく整数表現(claude-review.md 参照)
- balance-svc は pokedex-svc の REST API を呼ぶ。DBには直接繋がない
