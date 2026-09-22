# gateway

クライアント(Web / iOS)の唯一の入口。`/api/calc`・`/api/pokedex`・`/assets` を各上流へ転送し、`/api/*` の
`X-Device-Id` / `X-Session-Id`(UUID)を検証し、CORS に答える。設計は
[ADR-0202](../../docs/adr/0202-gateway-routing-and-headers.md)。契約は `api/openapi.yaml`(gateway 経由のパス)。

## 起動

| 環境変数 | 必須 | 意味 |
|---|---|---|
| `GATEWAY_ADDR` | いいえ(既定 `:8080`) | 待ち受けアドレス |
| `GATEWAY_CALC_URL` | はい | calc-svc の基底 URL(http / https) |
| `GATEWAY_POKEDEX_URL` | いいえ | pokedex-svc の基底 URL。base の既定は `http://pokedex`(pokedex-svc の Service。ADR-0206)。未設定なら `/api/pokedex/*` は 503 `upstream_unavailable` |
| `GATEWAY_ASSETS_URL` | いいえ | 画像配信(MinIO)の基底 URL。未設定なら `/assets/*` は 404 `not_found` |
| `GATEWAY_WEB_URL` | いいえ | Web の静的配信(nginx)の基底 URL(ADR-0205)。設定時は `/api`・`/assets`・`/healthz`・`/internal` のどれにも当たらない GET / HEAD を転送する。未設定なら従来どおりそれらのパスは 404 `not_found` |
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
| それ以外(`/api/balance` を含む。ADR-0012) | `GATEWAY_WEB_URL` 未設定なら 404 `not_found`。設定時は GET / HEAD を Web へ転送(ADR-0205) | なし |

- ヘッダの欠落・空は 400 `missing_header`、UUID でない値(正準形 8-4-4-4-12 以外)・重複は 400 `invalid_header`。
- 上流に接続できない・タイムアウトは 503 `upstream_unavailable`。上流の応答(4xx / 5xx を含む)は書き換えずに返す。
- CORS のプリフライト(OPTIONS)は 204 で、上流には送らない。

## k3d で動かす(ADR-0203)

```
make up                 # クラスタが無ければ作る(既存の pokecalc クラスタを使う)
make api-k3d-deploy     # calc・gateway のイメージをビルドして k3d に載せる
make api-smoke          # gateway 経由のスモーク(http://localhost:8080。API_URL で上書き)
```

- 全体(namespace・mysql・pokedex-migrate Job を含む)のデプロイは `make up`。API レーンの `api-k3d-deploy` は
  自分の2つの Deployment(calc・gateway)だけを専用の overlay(`deploy/k8s/overlays/local-api`)で適用する
  (共有の `deploy/k8s/overlays/local` を丸ごと apply しない。k3d クラスタは他レーンと共有のため)。
- `/api/pokedex/*` は base の既定 `GATEWAY_POKEDEX_URL=http://pokedex`(ADR-0206)で pokedex-svc に転送する。
  `make up` の直後(DB 未投入)は pokedex-svc が **503 `master_unavailable`** を返す。初回だけ `make import-k8s` で
  マスタを投入すると **200** になる(calc-svc も同じ pokedex-svc からマスタを取るので、投入されるまで
  `/readyz` が 503 のままになる。ADR-0204 §3)。**calc-svc が Ready にならない間は `make api-k3d-deploy` の
  `kubectl rollout status` が `--timeout=120s` で失敗する**。先に `make import-k8s` でマスタを投入してから
  `make api-k3d-deploy` を実行する。
- `make api-smoke` の1行目が `api smoke: master=pokedex species=… move=… nature=…` であれば pokedex-svc への配線を
  実際に検証できている。`master=example`(架空 ID)のときは pokedex-svc が未投入・未接続のフォールバックで、
  配線そのものは確認できていない。
- `/`(Web の静的配信。ADR-0205)は Web レーンの Service `web` がまだデプロイされていなければ **503**、デプロイ済みなら **200**。
- k3d を使わない開発ループは `make dev`(calc-svc と gateway をローカルで起動。`DEV_GATEWAY_PORT` / `DEV_CALC_PORT` で変更可。
  calc-svc はファイル方式 `CALC_MASTER_PATH` のまま。ADR-0206 §2)。
