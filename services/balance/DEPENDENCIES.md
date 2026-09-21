# balance direct dependencies

公開前の確認用に、Goの直接依存と利用理由を記録する。versionの正は`go.mod`、完全な依存集合の正は
`go.sum`とする。この表はtransitive dependencyの完全なlicense reportではない。

| Module | Version | Purpose | License確認 |
|---|---:|---|---|
| `github.com/labstack/echo/v4` | `v4.15.4` | HTTP adapter/router | module同梱`LICENSE`: MIT |
| `github.com/oapi-codegen/runtime` | `v1.7.0` | OpenAPI生成serverのparameter binding | module同梱`LICENSE`: Apache-2.0 |

公開用クリーンコピーを作る際は、transitive dependencyを含むlicense/security scanを別途実行し、
repository自体のLICENSEはプロジェクト方針が決まるまで追加しない。
