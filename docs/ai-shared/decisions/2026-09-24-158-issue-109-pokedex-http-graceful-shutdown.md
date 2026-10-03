## 2026-09-24: issue #109(pokedex HTTPタイムアウト・graceful shutdown)を main へ統合(データレーン)
Decision: PR #178(`feat/claude-p1-engine` → `main`)をマージした。`newHTTPServer`/`serve`/
`runServe`の3層分離(ADR-0111)。critic PASS(1往復、指摘なし)。
Reason: 独立レビュー PASS・`make test`(953件)/`lint`/`build`/`k8s-render`すべてgreen。
実クラスタでterminationGracePeriodSeconds=30・api-smoke正常応答を確認済み。
Impact: 他レーンへの影響なし。readiness/livenessの改善はissue #107の範囲(今回は対象外)。
