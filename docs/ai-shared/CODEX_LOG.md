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

## 2026-09-21 (TB0 GitOps / publication readiness)
- ユーザー方針「現在の履歴を直接公開せず、別directoryの公開用クリーンコピーをprivate repositoryへ接続する」に合わせ、balance側の準備を`d7a8bbf`へコミットした。元repositoryへのremote追加・pushは行っていない。
- local overlayと分離したGitOps overlayを追加し、imageはtagでなくdigest固定とした。Applicationはmanual syncのまま、予約済み`.invalid` domainとzero digestを安全なplaceholderに使用する。
- `balance-gitops-template-check` / `balance-gitops-check`を追加。実設定ではplaceholder、可変image tag、embedded credential付きGit URL、query/fragment、自動syncを拒否する。
- private Git/registryのcredential・Secret実値をGitへ入れない手順、R-2-9のクリーンコピー/full検査完了前はremoteへ接続しない手順、multi-platform imageをpushしてdigestを取得する補助script、直接依存とlicense記録を追加した。
- 独立レビュー中にMake変数のshell injectionとcredential URL検査不足を検出して修正。再レビューPASS(重大・重要0、軽微はguard self-testの将来追加余地のみ)。
- 検証成功: `sh -n`、GitOps template check、local/GitOps/Argo Kustomize render、placeholderの意図的拒否、unsafe image値の拒否、`balance-test`、`balance-lint`、`balance-build`、Docker build、k3d smoke `health=200 analyze=501`、diff/publishable手動scan。sandbox内smokeはlocalhost制限で失敗したため、同じcommandを許可済みsandbox外で再実行して成功を確認した。
- 未実施: private repository接続、registry image push、Argo CD導入・実同期。repository URL、registry/digest、credentialが未作成であり、成功扱いにしていない。
