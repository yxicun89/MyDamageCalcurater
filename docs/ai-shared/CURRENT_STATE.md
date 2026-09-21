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
Branch: feat/tb-tb1b-typechart(作業ディレクトリ ~/MyDamageCalcurater-tb。git worktree)
Status: TB0(Argo CD 実同期のみ人間待ち)・TB1(防御タイプバランス。PR #6)は main に統合済み。TB1b(相性表を typechart.json から読み TemporaryTypeChart を削除。ADR-0015)を実装・テスト済み、critic レビュー中
Next: TB1b を critic → PR(squash)で統合 → TB2(攻撃範囲。設計書 §6 TB2。技のデータ・入力形式が未定義なので ADR を先に書く)。未対応の軽微: HTTP で相性表が失敗したときの 500 テスト、typed nil の provider、read model の schema ファイル

## Shared Interfaces
- Pokemon ID: pokedex-svc の `{図鑑番号4桁}-{フォルム3桁}` 形式に準拠
- Type: 18タイプの英語小文字ID(fire, water, ...)。表示名・色は docs/design.md のトークンに準拠
- Type multiplier: 分数ではなく整数表現(claude-review.md 参照)
- サービス境界: damage-calc と balance は兄弟。相互の実行時 API へ直接依存しない
- 共通マスタ: 正本は1つ。`feat/claude-p1-engine` の ADR-0002 にあるコミット済みスナップショット案を候補とし、人間の確認待ち
- balance の type chart: P1-13 の `testdata/golden/typechart.json` をバイト複製して同梱(ADR-0015)。正式マスタ確定後に provider を差し替える
- 共有状態の正本: `origin/main` の `docs/ai-shared/`。未マージの続きは各レーン欄の `Branch` の最新コミット(COORDINATION.md)
- 開発の正本: private の `origin` の `main`(PR でのみ更新。Argo CD が参照)。作業ディレクトリはレーンごとに `~/MyDamageCalcurater`(ダメージ計算)と `~/MyDamageCalcurater-tb`(タイプバランス)
