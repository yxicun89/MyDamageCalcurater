# ADR-0801: タイムアウトの連鎖を「内側 < 外側」にそろえ、過負荷のときは待たせず 503 を返す(issue #299・#330)

- 状態: 採用(2026-10-02)
- 日付: 2026-10-02
- 担当: 運用(deploy・scripts)レーン。ADR 番号帯は ADR-0800 の運用用の 0800 帯
- 関連: issue #299(全体レビュー第3回)、#213(judge の全体締め切り。ADR-0707)、#323(pokedex。ADR-0129)、
  #298(balance recommendations の上限。ADR-0409)、#330(httpmetrics の複製のずれ検出。ADR-0406 §2)、
  ADR-0111(HTTP タイムアウト)、ADR-0202(gateway の上流タイムアウト)

## 背景

- Go の `http.Server.WriteTimeout` は応答の書き込みを失敗させるだけで、ハンドラの context を止めない。
  calc・balance・speed にはハンドラ全体の締め切りが無く、`docker run --cpus 0.2` で逆算の最大入力を同時 120 本送ると、
  `WriteTimeout`(10 秒)を過ぎても最大 19 秒計算を続け、応答は EOF(空応答)になった。
- 同時実行の上限・過負荷時の早期 503 がどのサービスにも無かった(balance の recommendations だけ ADR-0409 で上限あり)。
- k3d 同梱の Traefik(3.6)の既定は `respondingTimeouts.writeTimeout = 0`・`forwardingTimeouts.responseHeaderTimeout = 0`
  (どちらも無期限)。balance・speed・judge の Ingress は gateway を通らず Traefik から直接届く。
- #330: `services/internal/httpmetrics` と balance・speed・judge の複製(ADR-0406 §2)のずれを検出する
  バイト一致テストは、先行コミット(a8db4fa)で各独立モジュールに入っている。この ADR では扱いを変えない。

## 決定

### 1. 共通の守り `httpguard` を置く(ADR-0406 §2 と同じ複製の方針)

`services/internal/httpguard`(calc・pokedex が使う)と、balance・speed・judge の `internal/httpguard` にバイト一致の複製を置く。
共有モジュール化はレーン独立の方針(ADR-0406 §2)に反するので採らない。ずれは各複製のバイト一致テストが `make test` で検出する
(httpmetrics と同じ方式)。

- `Config{MaxInflight, Timeout, Code}` の Echo ミドルウェア。0 の項目は無効。
- 同時実行の上限: 枠が埋まっていれば**待たせず**即座に 503 + `Retry-After: 1` + Error 形式の JSON。計算を始めない。
- 締め切り: リクエストの context に `Timeout` を掛ける。値は `writeTimeout - 1秒`(`DeadlineFor`)。`Validate(writeTimeout)` は
  「`Timeout` は `writeTimeout` 未満」を確かめ、各サービスの `cmd` のテストが固定する。
- 締め切り後は engine(や計算の関数)を**新しく呼ばない**: ハンドラが計算の直前に `httpguard.Expired(ctx)` を見て、
  過ぎていれば 503 を返す。engine に context は持ち込まない(絶対ルール2。計算は 1 回数 ms なので呼び出し前の確認で足りる)。
- 運用エンドポイント(`/healthz`・`/readyz`・`/metrics`)は対象外(probe を 503 にして Pod を外さない)。
  ルートごとのミドルウェアとして掛け、メトリクスのミドルウェアの内側に置くので、503 も `http_requests_total` に数える。

### 2. サービスごとの値

| サービス | 守る対象 | 上限 | 締め切り | 503 の code |
|---|---|---|---|---|
| calc | `/api/calc`・`/bulk`・`/reverse` | 16 | 9 秒(writeTimeout 10 秒 − 1) | `upstream_unavailable` |
| balance | 全 API 操作 | 32(recommendations 単独は ADR-0409 の 4 のまま) | 14 秒(15 − 1) | `overloaded` |
| speed | 全 API 操作 | 32 | 14 秒(15 − 1) | `overloaded` |
| judge | `outspeed-and-ko` | 16 | 既存の `JUDGE_REQUEST_TIMEOUT`(12 秒。ADR-0707)に任せ、guard は上限だけ | `upstream_unavailable` |
| pokedex | DB を使う操作 | 64 | 既存の 5 秒(ADR-0129)に任せ、guard は上限だけ | `upstream_unavailable` |

- calc の 16 は、0.2 CPU・逆算の最大入力(1 回約 85ms)で 16 並列でも最後の 1 件が約 1.4 秒で終わる値。
- 契約への影響を最小にするため、**新しい ErrorCode は足さない**: balance は既存の `overloaded`(ADR-0409)、speed は
  `overloaded` を enum に追記(speed の `api/openapi.yaml` と生成物のみ)、calc・pokedex・judge は契約に宣言済みの
  `upstream_unavailable`(ルートの `api/openapi.yaml` は型を変えず、calc 3 操作・pokedex の 503 と code 表の説明文に
  「自サービスの過負荷・締め切り超過でも返す」を追記するだけ。この説明文の生成物への反映は §結果のとおり)。
  `overloaded` を全サービスの契約に足す案は、calc・gateway・Web・iOS の再生成を伴うので見送り、必要になれば API レーンに依頼する。
- 上限を環境変数にしない(`writeTimeout` と同じく定数。負荷試験で値を動かすときはコードを変える)。

### 3. Traefik に有限のタイムアウトを置く(k3d)

`deploy/k8s/overlays/local/traefik/helmchartconfig.yaml`(k3s の HelmChartConfig)で、
`entryPoints.web.transport.respondingTimeouts.writeTimeout=70s` と
`serversTransport.forwardingTimeouts.responseHeaderTimeout=20s` を設定する。

- 20 秒は、直接届くサービスの writeTimeout(15 秒)と gateway の上流の既定(10 秒)より長い。70 秒は gateway の
  writeTimeout(60 秒。assets の転送。ADR-0202 §5)より長い。内側が先に 503 を返せる。
- kube-system の資源なので overlay の `namespace: pokecalc` の変換に含めず、`scripts/up.sh` が `kubectl apply -f` で適用する。
  クラウドには Traefik の設定を置かない(クラウドの Ingress は別の判断。ADR-0210)。
- 値は `services/gateway/deploytest/traefik_timeouts_test.go` が各 `cmd/*/main.go` の定数と突き合わせる(数値を二重管理しない)。

### 4. 連鎖の大小関係(付録 A の表の確定)

内側 → 外側: pokedex の DB 締め切り 5 秒・judge 全体 12 秒 <
calc ハンドラ締め切り 9 秒 < calc writeTimeout 10 秒 ≦ gateway の上流 10 秒、balance/speed のハンドラ締め切り 14 秒 <
writeTimeout 15 秒 < Traefik の responseHeaderTimeout 20 秒 < gateway の writeTimeout 60 秒 < Traefik の writeTimeout 70 秒。
各段の関係はサービスの `cmd` のテストと deploytest が固定する。

## 結果・限界

- 過負荷のとき、上限を超えた分は待たずに 503 JSON になり、受け付けた分は締め切りまでに終わる。締め切りを過ぎた要求は
  engine を新しく呼ばない。ただし**進行中の 1 回の engine 呼び出し(数 ms〜数十 ms)は止められない**(engine は純粋で context を見ない)。
- Docker の負荷試験(同時 120 で EOF 0 件)は k3d・Docker が要るためメインでの実地確認とする(検証手順は issue #299 のとおり)。
- 契約の変更は speed の `overloaded` の追記と、ルート `api/openapi.yaml` の `upstream_unavailable` の説明文の追記のみ(型は変えない)。
  Web の生成物(`web/src/speed/speed.gen.ts`・`judge.gen.ts`・`balance.gen.ts`・`openapi.gen.ts`)は再生成で変わる(型の enum の追加と説明文)。
  iOS の生成物(`ios/PokeCalcKit/Sources/PokeCalcAPI/Generated`)も `ios/scripts/openapi-gen.sh` で再生成し、説明文の差分(コメントのみ)をコミットした。
