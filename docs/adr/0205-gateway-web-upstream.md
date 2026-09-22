# ADR-0205: gateway が Web の静的配信を後ろに置く

- 状態: 採用(2026-09-22。受け入れ条件とテストは spec-writer が先に書き、実装は implementer)
- 日付: 2026-09-22
- 関連: ADR-0202(gateway のルーティング・ヘッダ検証・CORS・上流の失敗)、ADR-0203(k3d デプロイとスモーク)、
  ADR-0204(`/internal/*` を gateway が外に出さない)、ADR-0012(`/api/balance` は独自の Ingress)、
  Web レーンの DECISIONS(ユーザー決定 2026-09-22「Web は gateway の後ろに置く。localhost:8080 だけで画面も API も使える」)

## 背景

ユーザー決定(2026-09-22)で、Web の画面(Vite のビルド成果物を nginx が配る)は gateway の後ろに置き、k3d の
`http://localhost:8080` だけで画面も API も使えるようにする。Web レーンは `deploy/k8s/base/web` に Service `web`
(port 80、namespace pokecalc)を用意する。gateway は今 `/api/calc`・`/api/pokedex/*`・`/assets/*`・`/healthz` 以外を
404 にしている(ADR-0202 §3)。Web の静的配信は API 契約ではないので `api/openapi.yaml` は変えない。

## 決定

### 1. 環境変数 `GATEWAY_WEB_URL`(任意)

| 変数 | 必須 | 既定 | 検証 |
|---|---|---|---|
| `GATEWAY_WEB_URL` | いいえ | 未設定 | 絶対 URL・スキームは http/https・ホストあり・クエリ無し。空は未設定と同じ。不正は起動エラー(`errInvalidConfig`) |

`httpapi.Config` に `WebURL *url.URL` を足す(nil は未設定)。

### 2. ルーティング(`WebURL` が設定されているとき)

予約したパスはセグメント単位で判定し、従来どおりのルートに行く(Web には流さない):

| パス | 扱い |
|---|---|
| `/api`、`/api/*` | 既存のルート(calc・pokedex)。未知の `/api/*` と `/api` そのものは 404 `not_found` |
| `/assets`、`/assets/*` | `/assets/*` の GET / HEAD は assets(未設定なら 404)。それ以外は 404 |
| `/healthz` | GET は gateway 自身。他のメソッドは 404 |
| `/internal`、`/internal/*` | 404 `not_found`(ADR-0204。Web にも流さない) |
| 上のどれにも当たらない GET / HEAD(`/` を含む) | Web の上流へ転送。パスとクエリはそのまま(SPA のフォールバックは nginx の仕事) |
| 上のどれにも当たらない GET / HEAD 以外 | 404 `not_found` |

- セグメント単位なので `/apix`・`/internals`・`/healthzx`・`/assetsx/*` は予約に当たらず Web へ行く(`/api/calcx` が calc に
  当たらないのと同じ考え方)。
- 判定の順序は ADR-0202 §3 のまま: CORS プリフライト(204。Web にも送らない)→ ドットセグメント拒否 → ルーティング →
  `/api/*` のヘッダ検証 → 上流の有無 → 転送。Web へのルートには `X-Device-Id` / `X-Session-Id` を課さない
  (ブラウザの画面の取得にヘッダは付かない)。
- `//internal/pokedex/master`・`//api/calc` のように連続スラッシュで先頭セグメントが空になるパスは、予約語との
  完全一致判定をすり抜けて Web に転送される抜け道になるので、ドットセグメントと同じくルーティングより前に
  (`WebURL` の有無によらず)404 `not_found` にする(critic 指摘)。
- `WebURL` が未設定なら従来どおり(どこにも当たらないパスは 404 `not_found`)。

### 3. 上流の扱い(既存の転送と同じ)

- 接続できない・タイムアウト(`GATEWAY_UPSTREAM_TIMEOUT`)は 503 `upstream_unavailable`(Error 形式。内部情報を出さない)。
- 上流の応答(404・500 を含む)はステータス・Content-Type・Cache-Control・本文をそのまま返す。
- 上流が付けた `Access-Control-*` は取り除き、許可オリジンのときだけ gateway が ACAO を1つ付ける(ADR-0202 §6)。
- Host と `X-Forwarded-*` は既存の `Rewrite`(`SetURL`・`SetXForwarded`)と同じ扱い。

### 4. k3d

- `deploy/k8s/overlays/local/api/gateway-patch.yaml` に `GATEWAY_WEB_URL=http://web` を足す。base には置かない
  (クラウドの Web の置き方は未定。base は従来どおり `GATEWAY_CALC_URL` だけで動く)。
- Web レーンの Service `web` がまだ無い間、k3d の `/` は **503 `upstream_unavailable`** になる(従来は 404)。Web を
  デプロイすると 200 になる。
- `services/gateway/scripts/smoke.sh` は `/` が 404 でないこと(200、または 503 `upstream_unavailable`)を確かめ、出力の最後に
  `web=200` / `web=503` を足す。`/internal/pokedex/master` が 404 のままであることの確認は残す。
- スモークの再試行(critic 指摘): ロールアウト直後は Traefik がまだ終了中の Pod に振り分けて `000`(接続不可)・
  `502`(Bad Gateway)を返すことがあるので、`services/gateway/scripts/smoke.sh` はスクリプト内のすべてのリクエストで
  `000`/`502` だけを数回(既定 `API_SMOKE_RETRIES`。1秒間隔)再試行する。`503`(pokedex 未設定・Web 未デプロイ)や
  `/` の `404`・`500` は意味のある最終状態でありうるので再試行しない(空振りせず、すぐに最終状態として扱う)。

## 受け入れ条件と担当テスト

| AC | 内容 | テスト |
|---|---|---|
| AC-W1 | `WebURL` 設定時、どこにも当たらない GET / HEAD(`/`・SPA のパス・静的ファイル・予約語で始まるだけの別名)が Web にメソッド・パス・クエリ付きで1回だけ届き、他の上流には届かない。上流の応答がそのまま返る。プリフライトは gateway の 204 | `httpapi.TestWebRoutesReachWebUpstream` / `TestWebPreflightStaysAtGateway` |
| AC-W2 | `WebURL` 設定時も `/api`・`/api/*`・`/assets`・`/assets/*`・`/healthz` は従来どおり(assets 未設定・HEAD /healthz も Web に落ちない)。`/internal`・`/internal/*`、GET / HEAD 以外、ドットセグメント(エンコード含む)は 404 で Web に届かない | `TestWebDoesNotShadowReservedRoutes` / `TestWebRejectsInternalMethodsAndDotSegments` |
| AC-W3 | Web へのルートはヘッダ検証をしない(無い・UUID でない・重複でも通る) | `TestWebRoutesSkipHeaderValidation` |
| AC-W4 | `WebURL` 未設定なら `/` 等は 404 のまま(従来の `TestUnroutedPathsAreNotFound` も維持) | `TestWebUnsetKeepsNotFound` |
| AC-W5 | Web の接続拒否・タイムアウトは 503 `upstream_unavailable`、上流の 404・500 はそのまま | `TestWebUpstreamFailures` |
| AC-W6 | Web の応答の CORS 付け替え(503 にも ACAO)、Host / X-Forwarded-* の書き換え | `TestWebCORS` / `TestWebForwardedHeadersAreRewritten` |
| AC-W7 | `WebURL` 設定時に gateway が作る 404 / 503 は Error スキーマに合う | `TestWebGatewayErrorsMatchErrorSchema` |
| AC-W8 | 環境変数名、`GATEWAY_WEB_URL` の正常・空・不正(スキーム無し・http(s) 以外・ホスト無し・クエリ付き・解析不可)。base に無く local overlay で `http://web` | `cmd/gateway.TestEnvNames` / `TestLoadConfig` / `TestLoadConfigRejects` / `TestManifestGatewayWebURLOnlyInLocal` |
| AC-W9 | スモーク: Web 未デプロイ(接続拒否)で成功して `web=503`、Web あり で成功して `web=200`。`/` が 404・Web が 500 なら失敗 | `deploytest.TestSmokeScriptAcceptsWebNotDeployed` / `TestSmokeScriptAcceptsWebDeployed` / `TestSmokeScriptFailsOnBrokenStack` |
| AC-W10 | gateway の README の環境変数の表に `GATEWAY_WEB_URL` の行がある | `deploytest.TestWebUpstreamIsDocumented` |

## 却下した案

- **Web 用に別の Ingress(path `/`)を置く**: gateway の Ingress も `/` Prefix なので衝突する。gateway の許可リスト
  (`/internal` を出さない等)を1か所に保てない。
- **gateway が静的ファイルを自分で配る**: gateway のイメージに Web の成果物を焼くことになり、レーンの独立が崩れる。
- **全メソッドを Web に流す**: 静的配信に書き込みは要らない。未知の POST が nginx に届く理由が無い。
- **`/api` 以外をすべて Web に流す(`/internal` を含む)**: ADR-0204 の「内部 API を外に出さない」を Web 側の設定に委ねることになる。

## 影響

- k3d の `http://localhost:8080/` は、Web のデプロイ前は 503、後は 200(従来は 404)。
- Web レーンは `deploy/k8s/base/web` の Service `web`(80 番)を用意する。SPA のフォールバック(未知のパス → index.html)は nginx の設定で行う。
- クラウドの overlay で Web をどう置くかは、クラウドの構成を決めるときに別途決める。
