## 2026-09-23: issue #106(手動importとCronJobの同時実行)を実装(データレーン)
Decision: `tools/importer/cronjob.sh`にbusyboxの`flock`(非ブロッキング)を`fetch.mjs`呼び出しより前に追加し、
取得(Node)〜投入(Go)の全工程を1本のロックでアプリ側排他した(ADR-0109)。`concurrencyPolicy: Forbid`は
「同じCronJobが作るJob同士の重複防止」に役割を限定し、手動Job(`make import-k8s`)との排他はflockが担う
(kubernetesのconcurrencyPolicyは異なるJob作成元をまたいで効かないため。ADR-0104 §5のコメントは不正確だった
ので訂正の追記をした)。ロック取得失敗は既存の終了コード規約(ADR-0104 §3)の1(再試行可能)にし、
`cronjob-import.yaml`のpodFailurePolicy・`services/pokedex/cmd/import`(Go CLI)・Makefileは無変更。
環境変数`IMPORT_APP_DIR`・`IMPORT_LOCK_FILE`でテストから差し替え可能にした。
2プロセス同時起動の統合テスト(`cronjob_lock_test.go`)を追加し、Docker(golang:1.27.1-alpine。busybox flock)で
実際に排他が機能することを確認(macOSはflockが無いため自動Skip)。critic PASS。
Reason: issue #106(Codexレビュー)。`make test`/`lint`/`build`/`k8s-render`すべてgreen。
Impact: k3dでの手動確認手順(2プロセス同時起動)をdocs/runbooks/data.md §6に追記したが、実クラスタでの
実行はまだ行っていない(次にk3dクラスタを使う機会に確認)。他レーンへの影響なし。
