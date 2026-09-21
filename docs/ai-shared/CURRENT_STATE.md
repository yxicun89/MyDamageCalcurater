# Current State

## Damage Calculator
Owner: Claude Code
Branch: feat/claude-p1-engine(作業ディレクトリ ~/MyDamageCalcurater)
Status: Phase 1・P2-1・P1-10・Phase R(R-2-9 の公開用クリーンコピーは公開時に実施)・R-3・P1-13(タイプ相性表のデータ化。ADR-0013)・P1-11(表示%の分離。ADR-0010 §3)は完了し、origin/main(協調運用の改訂 COORDINATION.md ほか)を取り込み済み。P1-13 は critic の独立レビューを受け、指摘を反映済み。旧 ~/pokecalc 系はアーカイブ(削除はユーザーの確認待ち)。
Next: main への統合(COORDINATION.md の手順: make test / lint / check-publishable の後に push。push はユーザーの指示があれば)→ P1-12(逆算の再設計)→ P2-1b → P2-1c → P2-2。人間の確認待ち(plan.md ブロッカー): 観測ダメージの入力と丸めの解釈、公開のタイミング(LICENSE・クリーンコピー)

## Type Balance Checker
Owner: Codex
Branch: feat/codex-tb0-foundation (worktree: ~/pokecalc-codex-tb0)
Status: TB0 基盤6e8989eに加え、GitOps・公開準備をd7a8bbfへコミット。localと分離したdigest固定GitOps overlay、安全なplaceholder/credential検査、private repository/registry手順、multi-platform image push補助、直接依存license記録を追加。独立レビューPASS、test/lint/build/Kustomize/Docker build/smoke成功。Argo CD実同期は未実施のためTB0全体は未完了
Next: ユーザー作成のprivate remoteはHTTPSで到達可能・空であることを確認済み(URL自体は個人accountを含むため文書へ記録しない)。Claude Code側worktreeに未コミットのengine変更が多数あるため、欠落を避けてpushは保留。Claude作業のcommit・検証完了後、Codex branchを通常手順で統合した単一source commitから、依頼されたAIが公開前full検査済みclean copyを作成してremoteへpushする。作成時はこの欄と担当log、clean copy側共有状態へ相対path・source/clean commit・検査・push状態・新しい開発正本を記録する。その後registry digest反映、version固定Argo CD・credential、manual sync→Pod更新を検証する

## Shared Interfaces
- Pokemon ID: pokedex-svc の `{図鑑番号4桁}-{フォルム3桁}` 形式に準拠
- Type: 18タイプの英語小文字ID(fire, water, ...)。表示名・色は docs/design.md のトークンに準拠
- Type multiplier: 分数ではなく整数表現(claude-review.md 参照)
- サービス境界: damage-calc と balance は兄弟。相互の実行時 API へ直接依存しない
- 共通マスタ: 正本は1つ。`feat/claude-p1-engine` の ADR-0002 にあるコミット済みスナップショット案を候補とし、人間の確認待ち
- TB0 type chart: `engine/typechart.go` と同じ現行相性を temporary adapter で持つ。正式マスタ確定後に provider を差し替える
- 共有状態の正本: main worktree の `docs/ai-shared/`。feature branch 内のコピーは現在状態として使わない
- 開発の正本: private の `origin`(履歴を消毒したクリーンコピー)の `main`。作業ディレクトリは Claude Code が `~/MyDamageCalcurater`、Codex が `~/MyDamageCalcurater-codex`(同じリポジトリの worktree)。統合・止まるときの作法は `docs/ai-shared/COORDINATION.md`。旧ディレクトリ(`~/pokecalc*`)はアーカイブで、以後そこで実装しない
