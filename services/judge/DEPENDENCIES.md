# judge direct dependencies

公開前の確認用に、Goの直接依存と利用理由を記録する。versionの正は`go.mod`、完全な依存集合の正は
`go.sum`とする。この表はtransitive dependencyの完全なlicense reportではない(balance・speed と同じ方針)。

| Module | Version | Purpose | License確認 |
|---|---:|---|---|
| `example.com/pokecalc/engine` | (replace `../../engine`) | 実数値・ランク補正の式。リポジトリ内モジュール | リポジトリ自身 |
| `github.com/labstack/echo/v5` | `v5.3.1` | HTTP adapter/router | module同梱`LICENSE`: MIT |
| `github.com/oapi-codegen/runtime` | `v1.7.0` | OpenAPI生成serverのparameter binding | module同梱`LICENSE`: Apache-2.0 |
| `github.com/prometheus/client_golang` | `v1.24.1` | `GET /metrics`(`internal/httpmetrics`。ADR-0406 §1〜3。`services/internal/httpmetrics` の複製) | module同梱`LICENSE`: Apache-2.0 |

2026-09-24 確認(P7-1): `github.com/prometheus/client_golang` を `go get .../client_golang@v1.24.1`
(確認時点で `proxy.golang.org` の `@latest` と一致)で追加し `go mod tidy` した。

公開用クリーンコピーを作る際は、transitive dependencyを含むlicense/security scanを別途実行し、
repository自体のLICENSEはプロジェクト方針が決まるまで追加しない(balance・speed と同じ)。
