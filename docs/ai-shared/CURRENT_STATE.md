# Current State

## Damage Calculator
Owner: Claude Code
Branch: feat/claude-p1-engine
Status: Phase 1・P2-1・P1-10・Phase R(R-2-9 の公開用クリーンコピーは公開時に実施)・R-3・P1-13(タイプ相性表のデータ化。ADR-0013)は完了。P1-13 は critic の独立レビューを受け、指摘(ADR 番号の参照・AC-8 のテスト名・ドキュメント)を反映済み。注意: origin/main に協調運用の改訂(docs/ai-shared/COORDINATION.md。マージコーディネーター廃止・各 AI が自分のブランチを統合)があるが、このブランチには未取り込み(main より8コミット遅れ)。作業ディレクトリは ~/MyDamageCalcurater(旧 ~/pokecalc 系はアーカイブ。削除はユーザーの確認待ち)。
Next: origin/main を取り込む(git fetch → merge → make test/lint/check-publishable)→ P1-11(表示%の分離)→ P1-12(逆算の再設計)→ P2-1b → P2-1c → P2-2。人間の確認待ち(plan.md ブロッカー): 観測ダメージの入力と丸めの解釈、公開のタイミング(LICENSE・クリーンコピー)

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
