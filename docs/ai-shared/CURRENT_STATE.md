# Current State

## Damage Calculator
Lane: ダメージ計算(どの AI が進めてもよい。COORDINATION.md)
Active: Claude Code
Branch: feat/claude-p1-engine(作業ディレクトリ ~/MyDamageCalcurater)
Status: Phase 1・P2-1・P1-10・Phase R(R-2-9 の公開用クリーンコピーは公開時に実施)・R-3・P1-13(タイプ相性表のデータ化。ADR-0013)・P1-11(表示%の分離)・P1-12(逆算の再設計。ADR-0010 §R)・P2-1b(ゴールデンを @smogon/calc 0.12.0 の Champions へ。ADR-0002 追記)は完了(critic レビュー済み)
Next: P2-1c → P2-2 → P2-3 → P3-1〜3 → P4-1〜7。人間の確認待ち(plan.md ブロッカー): 観測%の丸め方(整数%表示は確認済み)、公開のタイミング(LICENSE・クリーンコピー)、P2-1c の裁定

## Type Balance Checker
Lane: タイプバランス(どの AI が進めてもよい。COORDINATION.md)
Active: Claude Code
Branch: feat/tb-tb1-defense(作業ディレクトリ ~/MyDamageCalcurater-tb。git worktree)。TB0 は feat/codex-tb0-foundation から PR で main に統合
Status: TB0 は Claude Code の critic で独立再レビュー PASS(engine の相性表と単/複合 5832 件一致を確認)、軽微指摘を反映、balance-* / make test・lint・build・check-publishable、k3d smoke(health=200 analyze=501)成功。Argo CD 実同期のみ人間の作業待ち(plan.md「ブロッカー(タイプバランスレーン)」)
Next: TB1(防御タイプバランス)を main から切った feat/tb-tb1-defense で進める(quick-scanner → spec-writer → implementer → critic)

## Shared Interfaces
- Pokemon ID: pokedex-svc の `{図鑑番号4桁}-{フォルム3桁}` 形式に準拠
- Type: 18タイプの英語小文字ID(fire, water, ...)。表示名・色は docs/design.md のトークンに準拠
- Type multiplier: 分数ではなく整数表現(claude-review.md 参照)
- サービス境界: damage-calc と balance は兄弟。相互の実行時 API へ直接依存しない
- 共通マスタ: 正本は1つ。`feat/claude-p1-engine` の ADR-0002 にあるコミット済みスナップショット案を候補とし、人間の確認待ち
- TB0 type chart: `engine/typechart.go` と同じ現行相性を temporary adapter で持つ。正式マスタ確定後に provider を差し替える
- 共有状態の正本: `origin/main` の `docs/ai-shared/`。未マージの続きは各レーン欄の `Branch` の最新コミット(COORDINATION.md)
- 開発の正本: private の `origin` の `main`(PR でのみ更新。Argo CD が参照)。作業ディレクトリはレーンごとに `~/MyDamageCalcurater`(ダメージ計算)と `~/MyDamageCalcurater-tb`(タイプバランス)
