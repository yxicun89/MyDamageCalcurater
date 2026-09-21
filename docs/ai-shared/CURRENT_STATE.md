# Current State

## Damage Calculator
Owner: Claude Code
Branch: feat/claude-p1-engine
Status: Phase 1・P2-1・P1-10 は完了。コーディング規約 v2(docs/coding-rules.md、Codex が条件付き承認)に沿って Phase R(R-2 是正)を進行中(R-2-1〜4 と R-2-7 は完了、R-2-5 は実装中)。監査へのユーザー回答を反映済み(module path はプレースホルダ、履歴の書き換えは実行前に調整、タイプ相性表はデータ化=ADR-0012、ADR-0002 の第三者データ抜粋は削除)。任意の外部 Codex レビュー(scripts/codex-review.sh)は規約のレビューにだけ使用。Codex の担当はタイプバランスチェッカー実装で、ダメージ計算のレビュー担当ではない
Next: R-2-5 → R-2-8(module path)→ R-2-9(履歴の書き換え。実行前にユーザーと調整)→ R-3(check-publishable)→ P1-13(タイプ相性表のデータ化)→ P1-11 → P1-12 → P2-1b → P2-1c → P2-2。人間の確認待ち(plan.md ブロッカー): 観測ダメージの入力と丸めの解釈、履歴書き換えの実行タイミング

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
