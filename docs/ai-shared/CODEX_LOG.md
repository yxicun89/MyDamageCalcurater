# Codex Log

(Codex のセッション終了時にここへ追記する。書式は README_AI_SHARED.md 参照)

## 2026-09-21 (Claude Code による代理記録。Codex はレートリミットで操作不可)
- Claude Code が fix/codex-workflow-golden(未コミットだった作業)を確認。engine コア・API 契約に触れるため保留と判断し、
  内容を変えずに 6d86382 として保全。main へはマージしていない。理由と再検討の方針は DECISIONS.md 参照。
- 同日、ユーザー判断で fix/codex-workflow-golden を main へマージ(Claude Code が実施)。AGENTS.md / CLAUDE.md の衝突は両方残して統合。
  Codex は次回 main から自分のブランチを切って作業する。詳細は DECISIONS.md。

## 2026-09-21 (TB0 foundation)
- `feat/codex-tb0-foundation` に TB0 基盤を実装し、`6e8989e` (`feat(balance): add TB0 service foundation`)へコミットした。mainへは未統合。
- 純粋Goの型・整数倍率・単/複合防御相性・`EffectSource`、差し替え可能な`TypeChartProvider`とtemporary adapterを追加。adapterの複製元commit/hashをREADMEへ記録した。
- service-local OpenAPIを正としてmodels・Echo server interface・routeを生成。request context、body上限、JSON/ID検証、501 stubを実装し、server timeout/header上限も設定した。
- Dockerfile、service-local Makefile、Kustomize base/local overlay、manual-syncのArgo CD Application定義、smoke script、テスト戦略を追加した。ルート`go.work`とルート`Makefile`は規約どおり未変更。
- 独立レビュー初回は重要3・軽微2でFAIL。生成serverによる契約同期、HTTP timeout、providerエラー分岐テスト、複製元metadataを修正し、再レビューは修正差分PASS(重大0・軽微0)。
- 検証成功: OpenAPI再生成SHA-256一致、`balance-test`、`balance-lint`(gofmt/go vet)、`balance-build`、`balance-kustomize`、`go test -race ./...`、Docker build、k3d deploy、Pod 1/1 Ready・restart 0、smoke `health=200 analyze=501`。`cmd/api`と`internal/api`は`[no test files]`。
- 未完了ブロッカー: Git remote、配布imageのregistry/repositoryと不変参照、Argo CD Application CRDが未設定で、Git変更→manual sync→Pod更新を検証できない。値を推測せず、`docs/type-balance-test-strategy.md`へ必要な人間判断とともに明記した。このためTB0全体は未完了。
