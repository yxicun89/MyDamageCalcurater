# services 直接依存(pokedex 追加分)

公開前の確認用に、Go の直接依存と利用理由を記録する(coding-rules §1)。version の正は
`go.mod`、完全な依存集合の正は `go.sum` とする。この表は transitive dependency の完全な
license report ではない(`services/balance/DEPENDENCIES.md` と同じ方針)。

pokedex-svc のスキーマ・migrate(ADR-0100)で追加した分と、P7-1(メトリクス計測。ADR-0406)で
追加した分を記載する(oapi-codegen 系は元々の記載を参照。API レーンの担当)。

| Module | Version | Purpose | License確認 |
|---|---:|---|---|
| `github.com/golang-migrate/migrate/v4` | `v4.20.1` | DB migration の実行(embed した SQL を適用) | module同梱`LICENSE`: MIT |
| `github.com/go-sql-driver/mysql` | `v1.10.1` | MySQL ドライバ(database/sql) | module同梱`LICENSE`: MPL-2.0(ファイル単位のコピーレフト。本体を静的リンクする Go の利用形態では自プロダクトのライセンスに影響しないが、改変して配布する場合は当該ファイルの公開が必要) |
| `gopkg.in/yaml.v3` | `v3.0.1` | sqlc.yaml のテスト内検証(layout_test.go)に使用 | module同梱`LICENSE`: MIT/Apache-2.0 のデュアル |
| `github.com/santhosh-tekuri/jsonschema/v6` | `v6.0.3` | `pokedex export` の出力が balance の JSON Schema(ADR-0402)に合うことのテスト内検証(readmodel_test.go。ADR-0105 §5)。実行時のバイナリには入らない。kin-openapi 経由の indirect を direct にしただけで版は同じ | module同梱`LICENSE`: Apache-2.0 |
| `github.com/prometheus/client_golang` | `v1.24.1` | gateway・pokedex・calc 共通の `GET /metrics`(`services/internal/httpmetrics`。ADR-0406 §1〜3) | module同梱`LICENSE`: Apache-2.0 |

2026-09-22 確認(ADR-0102): 上記のうち移行時点の4件は `go mod tidy` 後も版が変わらず、`proxy.golang.org` の
`@latest` と一致(いずれも既に最新の安定版)。`go` 行は `go.work` と揃えて `1.27.1`(最新の安定版。
`go.dev/dl` で確認)に更新した。継続確認は `make deps-outdated`。

2026-09-24 確認(P7-1): `github.com/prometheus/client_golang` を `go get .../client_golang@v1.24.1`
(確認時点で `proxy.golang.org` の `@latest` と一致)で追加し `go mod tidy` した。

公開用クリーンコピーを作る際は、transitive dependency を含む license/security scan を別途実行する。
