# Current State

## Damage Calculator
Owner: Claude Code
Branch: feat/claude-p1-engine (worktree: ~/pokecalc)
Status: Phase 1 完了。P2-1 データソース調査完了。`feat/claude-p1-engine` の ADR-0002 は人間の確認待ち(main 未統合)
Next: 同 ADR-0002 の確認事項確定後に P2-2。Codex は damage-calc の未完了作業を実装しない

## Type Balance Checker
Owner: Codex
Branch: feat/codex-tb0-foundation (worktree: ~/pokecalc-codex-tb0)
Status: TB0 基盤を 6e8989e にコミット。型・整数相性コア・EffectSource・temporary provider・service-local OpenAPI生成server・HTTP最小疎通・Docker/Kustomize/Argo CD Application定義・単体テストを実装。独立再レビューは修正差分PASS、Docker build・k3d直接deploy・Pod Ready・smoke(health=200/analyze=501)まで成功。Argo CD実同期は未実施のためTB0全体は未完了
Next: Git repository URL、配布imageのregistry/repositoryと不変tag/digest、ローカルclusterへArgo CDを導入するかを人間が決定後、Applicationのplaceholderを置換してmanual sync→Pod更新を検証する。main取り込みはユーザー指示後にClaude Codeが行う

## Shared Interfaces
- Pokemon ID: pokedex-svc の `{図鑑番号4桁}-{フォルム3桁}` 形式に準拠
- Type: 18タイプの英語小文字ID(fire, water, ...)。表示名・色は docs/design.md のトークンに準拠
- Type multiplier: 分数ではなく整数表現(claude-review.md 参照)
- サービス境界: damage-calc と balance は兄弟。相互の実行時 API へ直接依存しない
- 共通マスタ: 正本は1つ。`feat/claude-p1-engine` の ADR-0002 にあるコミット済みスナップショット案を候補とし、人間の確認待ち
- TB0 type chart: `engine/typechart.go` と同じ現行相性を temporary adapter で持つ。正式マスタ確定後に provider を差し替える
- 共有状態の正本: main worktree の `docs/ai-shared/`。feature branch 内のコピーは現在状態として使わない
