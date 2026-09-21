# tools 直接依存

公開前の確認用に、Go の直接依存と利用理由を記録する(coding-rules §1)。version の正は
`go.mod`(`tool` ディレクティブを含む)、完全な依存集合の正は `go.sum` とする。この表は
transitive dependency の完全な license report ではない。

| Module | Version | Purpose | License確認 |
|---|---:|---|---|
| `github.com/sqlc-dev/sqlc` | `v1.31.1`(`tool` ディレクティブで固定) | `make gen-sql` が実行する DB クエリ生成ツール(services/pokedex/db) | module同梱`LICENSE`: MIT |

公開用クリーンコピーを作る際は、transitive dependency(sqlc が内部で使う各種パーサ等)を含む
license/security scan を別途実行する。
