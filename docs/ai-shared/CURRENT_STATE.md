# Current State

## Damage Calculator
Lane: ダメージ計算(どの AI が進めてもよい。COORDINATION.md)
Active: Claude Code
Branch: feat/claude-p1-engine(作業ディレクトリ ~/MyDamageCalcurater)
Status: P1-13・P1-11 は完了(ブランチに commit・push 済み、main 未統合)。P1-12(逆算の再設計)は実装済み・critic レビュー中
Next: P1-12 をコミット → PR で main に統合 → P2-1b → P2-1c → P2-2(詳細は Branch の最新コミットの CURRENT_STATE)

## Type Balance Checker
Lane: タイプバランス(どの AI が進めてもよい。COORDINATION.md)
Active: なし
Branch: feat/codex-tb0-foundation(作業ディレクトリ ~/MyDamageCalcurater-tb。git worktree)
Status: TB0 基盤6e8989eに加え、GitOps・公開準備をd7a8bbfへコミット。localと分離したdigest固定GitOps overlay、安全なplaceholder/credential検査、private repository/registry手順、multi-platform image push補助、直接依存license記録を追加。独立レビューPASS、test/lint/build/Kustomize/Docker build/smoke成功。Argo CD実同期は未実施のためTB0全体は未完了
Next: (1) TB0 をこのレーンの手順で検証(services/balance のテスト全件・make lint・make check-publishable)し、独立レビューの結果を確認して、PR で main に統合する(go.work の use と Makefile の include の1行を含める)。(2) 統合後、main から feat/tb-tb1-<名前> を切り、docs/type-balance-design.md の TB1 へ進む。旧 Next にあった「Claude 側の未コミット変更を待って push 保留」は解消済み(Claude 側はブランチに commit・push 済み)

## Shared Interfaces
- Pokemon ID: pokedex-svc の `{図鑑番号4桁}-{フォルム3桁}` 形式に準拠
- Type: 18タイプの英語小文字ID(fire, water, ...)。表示名・色は docs/design.md のトークンに準拠
- Type multiplier: 分数ではなく整数表現(claude-review.md 参照)
- サービス境界: damage-calc と balance は兄弟。相互の実行時 API へ直接依存しない
- 共通マスタ: 正本は1つ。`feat/claude-p1-engine` の ADR-0002 にあるコミット済みスナップショット案を候補とし、人間の確認待ち
- TB0 type chart: `engine/typechart.go` と同じ現行相性を temporary adapter で持つ。正式マスタ確定後に provider を差し替える
- 共有状態の正本: `origin/main` の `docs/ai-shared/`。未マージの続きは各レーン欄の `Branch` の最新コミット(COORDINATION.md)
- 開発の正本: private の `origin` の `main`(PR でのみ更新。Argo CD が参照)。作業ディレクトリはレーンごとに `~/MyDamageCalcurater`(ダメージ計算)と `~/MyDamageCalcurater-tb`(タイプバランス)
