# services 直接依存(pokedex 追加分)

公開前の確認用に、Go の直接依存と利用理由を記録する(coding-rules §1)。version の正は
`go.mod`、完全な依存集合の正は `go.sum` とする。この表は transitive dependency の完全な
license report ではない(`services/balance/DEPENDENCIES.md` と同じ方針)。

pokedex-svc のスキーマ・migrate(ADR-0015)で追加した分のみを記載する
(oapi-codegen 系は元々の記載を参照)。

| Module | Version | Purpose | License確認 |
|---|---:|---|---|
| `github.com/golang-migrate/migrate/v4` | `v4.20.1` | DB migration の実行(embed した SQL を適用) | module同梱`LICENSE`: MIT |
| `github.com/go-sql-driver/mysql` | `v1.10.1` | MySQL ドライバ(database/sql) | module同梱`LICENSE`: MPL-2.0(ファイル単位のコピーレフト。本体を静的リンクする Go の利用形態では自プロダクトのライセンスに影響しないが、改変して配布する場合は当該ファイルの公開が必要) |
| `gopkg.in/yaml.v3` | `v3.0.1` | sqlc.yaml のテスト内検証(layout_test.go)に使用 | module同梱`LICENSE`: MIT/Apache-2.0 のデュアル |

公開用クリーンコピーを作る際は、transitive dependency を含む license/security scan を別途実行する。
