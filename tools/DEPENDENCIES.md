# tools 直接依存

公開前の確認用に、Go の直接依存と利用理由を記録する(coding-rules §1)。version の正は
`go.mod`(`tool` ディレクティブを含む)、完全な依存集合の正は `go.sum` とする。この表は
transitive dependency の完全な license report ではない。

| Module | Version | Purpose | License確認 |
|---|---:|---|---|
| `github.com/sqlc-dev/sqlc` | `v1.31.1`(`tool` ディレクティブで固定) | `make gen-sql` が実行する DB クエリ生成ツール(services/pokedex/db) | module同梱`LICENSE`: MIT |

2026-09-22 確認(ADR-0018): `v1.31.1` は `go mod tidy` 後も変わらず、`proxy.golang.org` の `@latest` と
一致(既に最新の安定版)。`go` 行は `go.work` と揃えて `1.27.1`(最新の安定版。`go.dev/dl` で確認)に更新した。
継続確認は `make deps-outdated`(sqlc の transitive indirect dependency には更新のあるものが含まれるが、
sqlc 自身が上げるまで個別には固定しない)。

公開用クリーンコピーを作る際は、transitive dependency(sqlc が内部で使う各種パーサ等)を含む
license/security scan を別途実行する。
