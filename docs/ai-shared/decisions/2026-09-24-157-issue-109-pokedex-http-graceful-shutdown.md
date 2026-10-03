## 2026-09-24: issue #109(pokedex HTTPサーバーにタイムアウトとgraceful shutdownを追加)を実装(データレーン)
Decision: pokedex-svcだけがcalc/gateway/balance/judgeの運用契約(明示的なhttp.Server・タイムアウト・
signal.NotifyContextによるgraceful shutdown)から外れていたリスクを解消した。services/balanceと
同じ値(readHeaderTimeout=5s・readTimeout=10s・writeTimeout=15s・idleTimeout=60s・
maxHeaderBytes=16KiB・shutdownTimeout=10s)で`newHTTPServer`/`serve`/`runServe`の3層に分離
(ADR-0111)。既存の`run(args) int`(サブコマンド振り分け)との名前衝突を`runServe`への改名と
`runServeCmd`の新設で解消(calc-svcにはこの衝突が無いため見落としやすい点。spec-writerが発見)。
`deployment.yaml`に`terminationGracePeriodSeconds: 30`を追加し、main.goの`shutdownTimeout`定数
より長いことをハードコードせず不等式でmanifestテストに固定。critic PASS(1往復。指摘なし)。
**実クラスタ(k3d-pokecalc)でpokedexを再ビルド・再デプロイし、terminationGracePeriodSecondsが
実際に30になっていること・api-smokeが正常応答することを確認済み**。
Reason: issue #109(Codexレビュー)。低速・不完全な接続がリソースを無期限に保持しうる可用性リスクと、
Kubernetesのrollout・node drainで処理中リクエストが即座に打ち切られる問題を解消するため。
`make test`(953件)/`lint`/`build`/`k8s-render`すべてgreen。`-race`・`-count=3`でも安定確認済み。
Impact: HTTPパス・公開OpenAPI・DBクエリ・マスタ内容は無変更。readiness/liveness probeの改善は
issue #107の範囲(今回は対象外)。他レーンへの影響なし。
