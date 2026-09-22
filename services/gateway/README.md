# gateway

クライアント(Web / iOS)の唯一の入口。`/api/calc`・`/api/pokedex`・`/assets` を各上流へ転送し、`/api/*` の
`X-Device-Id` / `X-Session-Id`(UUID)を検証し、CORS に答える。設計は
[ADR-0202](../../docs/adr/0202-gateway-routing-and-headers.md)。契約は `api/openapi.yaml`(gateway 経由のパス)。

## 起動

| 環境変数 | 必須 | 意味 |
|---|---|---|
| `GATEWAY_ADDR` | いいえ(既定 `:8080`) | 待ち受けアドレス |
| `GATEWAY_CALC_URL` | はい | calc-svc の基底 URL(http / https) |
| `GATEWAY_POKEDEX_URL` | いいえ | pokedex-svc の基底 URL。未設定なら `/api/pokedex/*` は 503 `upstream_unavailable` |
| `GATEWAY_ASSETS_URL` | いいえ | 画像配信(MinIO)の基底 URL。未設定なら `/assets/*` は 404 `not_found` |
| `GATEWAY_CORS_ALLOWED_ORIGINS` | いいえ | カンマ区切りの許可オリジン(完全一致)。空なら CORS ヘッダを付けない。`*` は起動エラー |
| `GATEWAY_UPSTREAM_TIMEOUT` | いいえ(既定 `10s`) | 上流の応答ヘッダを待つ上限(Go の duration)。超えたら 503 `upstream_unavailable` |

設定が不正なら非ゼロで終了する。

```sh
cd services
# calc-svc を別に起動しておく(services/calc/README.md)
GATEWAY_ADDR=:8081 \
GATEWAY_CALC_URL=http://localhost:8080 \
GATEWAY_CORS_ALLOWED_ORIGINS=http://localhost:5173 \
go run ./gateway/cmd/gateway
```

## ルーティング

| パス | 転送先 | ヘッダ検証 |
|---|---|---|
| `/api/calc`、`/api/calc/*` | calc-svc | あり |
| `/api/pokedex/*` | pokedex-svc | あり |
| `/assets/*`(GET / HEAD) | assets の上流 | なし(`<img>` はヘッダを送れない) |
| `GET /healthz` | gateway 自身(`200 {"status":"ok"}`。openapi には載せない) | なし |
| それ以外(`/api/balance` を含む。ADR-0012) | なし(404 `not_found`) | ― |

- ヘッダの欠落・空は 400 `missing_header`、UUID でない値(正準形 8-4-4-4-12 以外)・重複は 400 `invalid_header`。
- 上流に接続できない・タイムアウトは 503 `upstream_unavailable`。上流の応答(4xx / 5xx を含む)は書き換えずに返す。
- CORS のプリフライト(OPTIONS)は 204 で、上流には送らない。

## k3d で動かす(ADR-0203)

```
make up                 # クラスタが無ければ作る(既存の pokecalc クラスタを使う)
make api-k3d-deploy     # calc・gateway のイメージをビルドして k3d に載せる
make api-smoke          # gateway 経由のスモーク(http://localhost:8080。API_URL で上書き)
```

- `api-k3d-deploy` は、データレーンの `mysql-auth` Secret が無いクラスタでは calc・gateway だけ(`deploy/k8s/overlays/local-api`)を、
  ある(`make up` 済み)クラスタでは `deploy/k8s/overlays/local` を丸ごと適用する。
- `/api/pokedex/*` は pokedex-svc(P2-3)ができるまで上流が未設定なので **503 `upstream_unavailable`**。pokedex-svc を載せて
  `GATEWAY_POKEDEX_URL` を設定したら **200** に変わる(`services/gateway/scripts/smoke.sh` の期待値と
  `TestManifestGatewayLocalConfig` を一緒に変える)。
- k3d を使わない開発ループは `make dev`(calc-svc と gateway をローカルで起動。`DEV_GATEWAY_PORT` / `DEV_CALC_PORT` で変更可)。
