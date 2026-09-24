# 動作確認との対応表

- 対象: [../verify-m1.md](../verify-m1.md) §1〜§4、`make web-k3d-smoke`、`make api-smoke`、他レーンの smoke / e2e。
- 基準: `origin/main` 取り込み後(3379b03 + 更新)。行番号は同時点。
- 詳細は重複させずリンク: コマンドの裏側 [runbook-commands.md](runbook-commands.md) / ターゲット [make-targets.md](make-targets.md) / ルート・ステータス [api-endpoints.md](api-endpoints.md) / 処理フロー [request-flows.md](request-flows.md) / リソースとポート [k8s-local.md](k8s-local.md) / DB [db-mysql.md](db-mysql.md) / 環境変数 [config-env.md](config-env.md)。
- 略記: `gw` = `services/gateway/internal/httpapi`、`calc` = `services/calc/internal/httpapi`、`pdx` = `services/pokedex/internal/httpapi`。

## 1. verify-m1.md の各項目 → 検証対象

### 1.1 §1 準備・§2 自動テスト(host のみ。クラスタ非接触)

| verify-m1 | 検証対象(コンポーネント / 実装) | 落ちたとき最初に見る場所 |
|---|---|---|
| §1 `make doctor` | 開発機のツール(`scripts/doctor.sh`) | 表示された `brew install …` |
| §1 `make web-install` / `web-e2e-install` | `web/package-lock.json` の再現、Playwright chromium | npm のネットワークエラー |
| §2 `make test` | engine(`engine/`)・services(`services/**`)・tools・`scripts/argocd-bootstrap_test.sh`・balance/speed/judge・web(vitest)の全ユニットテスト([make-targets.md](make-targets.md)) | 失敗したパッケージ名 → 該当モジュールの `*_test.go` / `*.test.ts` |
| §2 `make lint` | gofmt/vet、シェル構文、`k8s-render`(`kubectl kustomize` の全 overlay 描画)、`check-publishable`、各レーン lint | `check-publishable` はファイル:行と種類、`k8s-render` は overlay 名 |
| §2 `make build` | Go 全モジュールの build、web(型検査 + vite build + サイズ予算) | 型エラー / サイズ予算超過 |
| §2 `make test-golden` | engine の計算式 vs `testdata/golden`(@smogon/calc 0.12.0 の期待値)。全件一致が絶対ルール3 | 差分の入力ベクタ。許容は `testdata/golden/known_diffs.yaml`(ADR 付き)のみ |
| §2 `make test-wasm` | Go/WASM 境界: `engine/wasmapi` の JSON 契約(ADR-0011)。`scripts/wasm-conformance.mjs` がネイティブ Go(`engine/cmd/wasmexpect`)と `engine.wasm` の出力をバイト比較 | `web/public/engine.wasm` の生成失敗(`scripts/wasm.sh`)/ 不一致ケース名 |
| §2 `make web-test-wasm` | Web が組むリクエスト(`web/src/domain/requests.ts` 等)が本物の wasm を通る(`web/src/engine/wasmEngine.wasm.test.ts`、`reverse.wasm.test.ts`) | vitest.wasm の失敗ケース |
| §2 `make web-e2e` | オフライン(WASM)経路の画面。`web/e2e/{calc,reverse,routing,a11y,offline}.spec.ts`(vite preview `127.0.0.1:4317`) | Playwright の trace / スクリーンショット |
| §2 `make web-e2e-container` | 配信設定: `web/nginx.conf`(SPA フォールバック・MIME・キャッシュ・gzip・`/api` 404)を、k8s と同じ制約の `docker run` で。`web/e2e/container.spec.ts` | `docker build` の失敗 / nginx の応答ヘッダ |
| §2 `make web-e2e-online` | オンライン(API)経路: `web/src/api/apiEngine.ts` → vite preview の `/api` プロキシ → `go run ./calc/cmd/calc`。`online.spec.ts`(API とオフラインの結果一致) | calc-svc のログ(`127.0.0.1:18317`) |
| §2 `make web-e2e-balance` | Web ↔ balance API: `web/src/api/balanceClient.ts` → `go run ./balance/cmd/api`。`balance.spec.ts` | balance のログ(`:18318`) |

### 1.2 §3 k3d 起動

| verify-m1 | 検証対象 | 落ちたとき最初に見る場所 |
|---|---|---|
| `make up` | k3d 作成・namespace・Secret・MySQL・`pokedex-migrate` Job・pokedex-svc([db-mysql.md](db-mysql.md) §3) | `kubectl -n pokecalc get pods,jobs` → `logs job/pokedex-migrate`。Docker が起動しているか |
| `make api-k3d-deploy` | calc・gateway の image 反映と rollout(`deploy/k8s/overlays/local-api`) | `kubectl -n pokecalc rollout status deployment/gateway` → `describe pod` |
| `make web-k3d-deploy` | web の image 反映と rollout(`deploy/k8s/overlays/local-web`) | 同上(`deployment/web`) |
| `kubectl -n pokecalc get pods` | Pod の Running(`calc gateway web balance pokedex mysql-0`) | `describe pod` の Events(ImagePull / Probe) |
| `make import-fetch` | `tools/importer/fetch.mjs`(外部取得。ADR-0101) | ネットワーク / 版固定の不一致 |
| `make import-dry-run` | `services/pokedex/cmd/import -dry-run`(DB 非接触。`blockers: none`) | 最終行の blockers。exit 3 は上流との食い違い |
| `make import-k8s` + `wait --for=condition=complete` | `pokedex-import` CronJob テンプレートから作る手動 Job(`tools/importer/cronjob.sh`。ADR-0104・0109) | `kubectl logs job/<名前>`。exit code は [db-mysql.md](db-mysql.md) §5 |
| `make web-k3d-smoke` / `make api-smoke` | §2・§3(本書) | §4(本書)のエラー早見表 |

### 1.3 §4 画面の確認(ブラウザ)

画面の入口: `web/src/App.tsx`(タブ・URL・キー操作)、`web/src/app/{routes,screens}.ts*`、各画面 `web/src/screens/*.tsx`。エンジンは `createWasmEngine`(`web/src/engine/wasmEngine.ts:142`。オフライン)/ `createApiEngine`(`web/src/api/apiEngine.ts:229`。オンライン)を `web/src/app/calcMode.ts` で切り替え。

| # | 操作 | 検証している実装 | Network タブで見るもの / 落ちたとき |
|---|---|---|---|
| 1 | 攻撃側・防御側を選ぶ → 5 行 | `CalcScreen`(`web/src/screens/CalcScreen.tsx:250`)→ engine の一括計算 → gateway `POST /api/calc/bulk`(`gw/routing.go:59` `matchRoute` → calc)。e2e: `calc.spec.ts:23` | `bulk` が 200 か。400 は [§4](#4-エラー早見表) |
| 2 | 調整を「A特化」 | `web/src/domain/attackerPresets.ts` が組むリクエスト → 同上。e2e: `calc.spec.ts:41` | リクエスト本文の SP |
| 3 | 「持ち物の候補も比較」 | `CalcScreen.tsx:230,316`(`defenderItemVariants`)。マスタの持ち物は `web/src/master/` | 持ち物の一覧が空なら pokedex(`GET /api/pokedex/*`)。e2e: `calc.spec.ts:81` |
| 4 | 「攻守入れ替え」 | `CalcScreen.tsx` の `swapping`(演出は CSS `--duration-swap`)。e2e: `calc.spec.ts:57` | 画面のみ(API は再計算だけ) |
| 5 | 「逆算」タブ → `/reverse` | `App.tsx:207` `history.pushState`、`web/src/app/routes.ts`。e2e: `routing.spec.ts:31` | URL |
| 6 | 観測 1 を入れる → 「H32 を仮定」と候補 | `ReverseScreen`(`web/src/screens/ReverseScreen.tsx:157`)→ `POST /api/calc/reverse`(engine の総当たり探索。ADR-0010)。e2e: `reverse.spec.ts:40` | `reverse` が 200 か |
| 7 | 「観測を追加」 → 候補が増えない | `web/src/domain/observations.ts`(上限・検証)。e2e: `reverse.spec.ts:65` | — |
| 8 | 「タイプバランス」タブ → 2 体選ぶ | `BalanceScreen`(`web/src/screens/BalanceScreen.tsx:187`)→ `balanceClient.ts` → Traefik `/api/balance/v1/team-balance/analyze`(gateway を通らない。[api-endpoints.md](api-endpoints.md) §6)。e2e: `balance.spec.ts:36` | `analyze` が 200 か。404 は balance の Ingress、422 は入力 |
| 9 | 技を選ぶ → 攻撃範囲 | 同 `…/coverage`。e2e: `balance.spec.ts:76` | `coverage` |
| 10 | 「仮想敵を追加」 | 同 `…/threats`(最大 6 体。ADR-0303 §7)。e2e は無し(vitest `BalanceScreen.test.tsx`) | `threats` |
| 11 | ブラウザの戻る | `App.tsx:254-266` `popstate`。e2e: `routing.spec.ts:31` | — |
| 12 | `/reverse` を直接開く | 8080: gateway が `/` を web へ転送([request-flows.md](request-flows.md) §5)→ nginx の SPA フォールバック(`web/nginx.conf` `location /`)。e2e: `routing.spec.ts:21` | 白画面 → [§4](#4-エラー早見表) の 404/503。`/reverse` が 404 なら gateway か nginx |
| 13 | ← → キー | `App.tsx:216-229`(ArrowLeft/Right/Home/End)。e2e: `a11y.spec.ts:7,52` | — |

## 2. `make web-k3d-smoke`(`web/scripts/k3d-smoke.sh`)

- 接続先: `WEB_URL`(既定 `localhost:5173` = `make web-k3d-open` の port-forward → `svc/web:80` → Pod `:8080`(nginx)。**gateway を通らない**)。ステータスと Content-Type だけを curl で見る。1 つでも NG なら exit 1。

| # | 行 | チェック | 期待 | 検証する実装 | ADR / 補足 |
|---|---|---|---|---|---|
| 0 | `:22` | `/healthz` に接続できるまで 1 秒間隔で最大 `WEB_SMOKE_RETRIES`(30)回 | 200 | `web/nginx.conf:40` `location = /healthz`。Pod の probe も同じ | 全回失敗 = `NG … 30 回つながらなかった`(今回の症状。port-forward 無し) |
| 1 | `:55` | ヘルスチェック | 200 | 同上 | — |
| 2 | `:56` | `/` | 200 `text/html` | nginx `location /`(`try_files` で `index.html`) | ADR-0300 §1 |
| 3 | `:57` | `/reverse` | 200 `text/html` | SPA フォールバック(URL で画面を切り替えるため) | ADR-0300 §1 |
| 4 | `:58` | `/engine.wasm` | 200 `application/wasm` | `nginx.conf:53`。イメージに `engine.wasm` が含まれる(`web/Dockerfile`) | ADR-0011 §6・§11(`instantiateStreaming` の条件) |
| 5 | `:59` | `/wasm_exec.js` | 200 | `nginx.conf:59` | — |
| 6 | `:60` | `/assets/no-such-file.js` | **404** | `nginx.conf:46` `location ^~ /assets/`。JS の代わりに HTML を返して壊れるのを防ぐ | — |
| 7 | `:61` | `/api/calc` | **404** | `nginx.conf:32,35`。`/api` は gateway の担当で web は転送しない | DECISIONS.md 2026-09-22。**`WEB_URL` を 8080 にすると gateway の 400 になり NG(想定外の使い方)** |

出力の読み方: `OK  <説明>: <パス> -> <status>`、NG は `NG  <説明>: <パス> のステータスが … (期待 …)`。最終行 `web smoke: すべて成功(<URL>)`。

## 3. `make api-smoke`(`services/gateway/scripts/smoke.sh`)

- 接続先: `API_URL`(既定 `localhost:8080` = k3d loadbalancer → Traefik → Ingress `gateway` → gateway)。端末 ID・セッション ID は架空 UUID(`:23-24`)。再試行は `000`(接続不可)と `502` だけ(`:76-90`。ADR-0203 §5・ADR-0205 で更新)。`404`・`503` は意味のある最終状態なので再試行しない。
- スクリプト自体のテスト: `services/gateway/deploytest/smoke_test.go`(正常・壊れた構成・一過性の 502)。

| # | 行 | 叩くもの | 期待 | 検証する実装(コンポーネント) | ADR |
|---|---|---|---|---|---|
| 0 | `:119` | `GET /api/pokedex/natures`(入手元の判別) | 200 → `master=pokedex` / 503 `upstream_unavailable` → 例データ / 503 `master_unavailable` → 未投入(`import-k8s` の案内) / それ以外は失敗 | gateway → pokedex-svc(`pdx`)。`master_unavailable` は `pdx/errors.go:51`(DB 失敗・マスタ不完全)、`upstream_unavailable` は gateway が上流に届かない(`gw/proxy.go:61`)/ `PokedexURL` が nil(`gw/server.go:32`) | ADR-0206 §3・§4 |
| 0a | `:163,179` | `GET /api/pokedex/species?limit=1`、`/moves?limit=200` | 200 | pokedex-svc の検索(`pdx/search.go`)。計算に使う ID を**実データから**引く(マスタを直書きしない) | ADR-0206 |
| 1 | `:232` | `POST /api/calc`(候補の技を先頭から最大 10 件) | 200、`maxDamage >= 1`、`rolls` 16 個、`category":"physical"` | gateway `checkAPIHeaders` → calc `CalcDamage`(`calc/server.go:177`)→ engine。[request-flows.md](request-flows.md) §1 | ADR-0200・0206 |
| 2 | `:255` | `POST /api/calc/bulk` | 200、`rows` が 1 行以上 | calc の bulk。[request-flows.md](request-flows.md) §2 | ADR-0009 |
| 3 | `:262` | `POST /api/calc/reverse`(1 の `maxDamage` を観測に) | 200、`candidates` が 1 件以上 | calc の reverse(engine 総当たり) | ADR-0010 |
| 4a | `:267` | 同 `/api/calc` を**ヘッダ無し** | **400 `missing_header`** | gateway `checkAPIHeaders`(`gw/headers.go:19`)。**上流に届く前に gateway が弾く = 期待どおり** | ADR-0202 §4 |
| 4b | `:269` | 同 `/api/calc`、`X-Session-Id: not-a-uuid` | **400 `invalid_header`** | 同上(正準形 UUID のみ許可) | ADR-0202 §4 |
| 5 | `:274` | `GET /internal/pokedex/master`(ヘッダ無し) | **404 `not_found`** | gateway が `/internal/*` を公開しない(`gw/routing.go:59` `matchRoute` に無いパス = 404)。内部 API は calc → pokedex のサービス間専用 | ADR-0204・0210 |
| 6 | `:281` | `GET /`(ヘッダ無し) | 200(web 到達)/ 503 `upstream_unavailable`(web 未デプロイ)。それ以外は失敗 | gateway が `/api` 以外を web に転送(`GATEWAY_WEB_URL`) | ADR-0205 |
| 7 | `:308` | `GET /api/balance/healthz`(balance の Ingress があるときだけ。`API_SMOKE_BALANCE`) | 200 | Traefik が `/api/balance` を balance に振る(gateway の `/` が奪わない) | ADR-0012・0203 |

最終行 `api smoke: calc=200 bulk=200 reverse=200 missing_header=400 invalid_header=400 pokedex=200 internal=404 balance=200 web=200` の読み方:

| 項目 | 値 | 意味 | 異常のとき |
|---|---|---|---|
| `calc/bulk/reverse` | 200 | 計算 3 操作が通った | §4 |
| `missing_header` / `invalid_header` | **400** | 異常系を意図して叩き、gateway が正しく弾いた | 200 など → gateway の検証が抜けた |
| `internal` | **404** | 内部 API が外に出ていない | 200 → **内部 API の露出(重大)** |
| `pokedex` | 200(投入済み)/ 503 | 503 は `master=example`(例データ)で継続 | `master_unavailable` → `make import-k8s` |
| `balance` | 200 / `skipped` | balance の Ingress が無いと `skipped` | 404 → balance の Ingress |
| `web` | 200 / 503 | 503 は web 未デプロイ | 404 → gateway の web 転送設定 |

## 4. エラー早見表

判別の基準(実測。2026-09-24 に `curl localhost:8080` で確認): gateway が返すエラーは**必ず JSON**(`{"code":"…","message":"…"}`、`application/json`)。本文がそれ以外なら gateway より手前(Traefik)か、上流(web の HTML / nginx)の応答。

| 症状 | どの層 | 主な原因 | 見るコマンド |
|---|---|---|---|
| `000` / `connection refused`(8080) | ホスト → k3d の loadbalancer | クラスタ停止、8080 が別プロセスに占有(`lsof -nP -iTCP:8080 -sTCP:LISTEN`) | `k3d cluster list`、`docker ps`(`k3d-pokecalc-serverlb`)、[runbooks/api.md](../runbooks/api.md) §6(`make dev` は `DEV_GATEWAY_PORT` で避ける) |
| `000`(5173) | ホスト | **`make web-k3d-open`(port-forward)未実行** | 別ターミナルで `make web-k3d-open`。`lsof -nP -iTCP:5173 -sTCP:LISTEN` |
| `502`(rollout 直後) | Traefik | 終了中の Pod に振り分け。smoke は自動再試行 | 待つ。続くなら `kubectl -n pokecalc rollout status deployment/<名前>`、`get endpoints` |
| `503` + JSON `upstream_unavailable` | gateway → 上流 | gateway が上流(calc/pokedex/web)に接続できない・`GATEWAY_UPSTREAM_TIMEOUT` 超過。`GATEWAY_POKEDEX_URL` / `GATEWAY_WEB_URL` 未設定でも同じ(`gw/server.go:32`) | `kubectl -n pokecalc get pods,endpoints`、`logs deployment/gateway`、`kubectl -n pokecalc exec deploy/gateway -- env`([config-env.md](config-env.md)) |
| `503` + JSON `master_unavailable` | calc / pokedex | pokedex の DB が未投入・不完全、または calc がまだマスタを取得できていない(`calc/readiness.go:21`) | `make import-k8s` → `wait`。`kubectl -n pokecalc logs deployment/calc`(起動時の取得。[request-flows.md](request-flows.md) §4)、`get pods`(pokedex Running か) |
| `503`(JSON でない / Traefik の本文) | Traefik | Service に Ready な Endpoint が無い | `kubectl -n pokecalc get endpoints`、`describe ingress <名前>` |
| `404` + JSON `not_found` | gateway | ルートが無い。**`/internal/*` は意図どおり**。ほか: メソッド違い(`GET /api/calc` は 404)、`.` / `..`・連続スラッシュ(`gw/routing.go:81,95`) | `matchRoute`(`gw/routing.go:59`)と [api-endpoints.md](api-endpoints.md) §2 |
| `404`(JSON でない) | Traefik / nginx | `/api/balance|speed|judge` の Ingress 不在、または web の `/api` / `/assets/*` の 404(想定) | `kubectl -n pokecalc get ingress`([k8s-local.md](k8s-local.md) §6)、`web/nginx.conf:32-46` |
| `400` + `missing_header` / `invalid_header` | gateway(`gw/headers.go:19`) | `X-Device-Id` / `X-Session-Id` が無い・非 UUID。**smoke の異常系は正常** | ブラウザ: `web/src/api/apiEngine.ts:240-241` が付与(ID は `clientIds.ts`)。curl は 2 ヘッダを足す |
| `400` + `invalid_json` / `unknown_field` / `invalid_enum` | calc(`calc/errors.go:95`、`convert.go:16`) | 本文の不正 | `logs deployment/calc`、[api-endpoints.md](api-endpoints.md) §4・§9 |
| `400` + `unknown_species` 等(`unknown_*`) | calc(`convert.go:117,165`) | マスタに無い ID(例データの ID を実マスタに送った等) | pokedex の `GET /api/pokedex/species` で ID を確認 |
| `400` + `missing_request_context` / `invalid_request` | balance / speed・judge | ヘッダ欠落・不正のコードがサービスごとに異なる([architecture.md](architecture.md) §6) | 各サービスの表([api-endpoints.md](api-endpoints.md) §6・§7) |
| `422` + `unknown_pokemon` / `unknown_move` / `unknown_ability` | balance / speed | read model に無い ID | `data/generated/readmodel`(balance/speed の read model) |
| 画面が白い(200 だが空) | web | `engine.wasm` 未同梱・MIME 違い、API モードで例データ ID を送る(ADR-0301 §4) | ブラウザの Network / Console。`make web-k3d-smoke` の項目 4・5 |

## 5. 既知の失敗パターン(根拠つき)

| パターン | 症状 | 根拠 | 対処 |
|---|---|---|---|
| port-forward 未実行 | `NG  http://localhost:5173/healthz に 30 回つながらなかった` | `web/Makefile:61,85-86`、`web/scripts/k3d-smoke.sh:22-28` | `make web-k3d-open` を別ターミナルで先に実行(`verify-m1.md` §3 に追記済み) |
| 8080 を Web 直の smoke に使う | `/api/calc のステータスが 400(期待 404)` の 1 件 NG | `k3d-smoke.sh:61`、gateway が `/api/*` を検証 | 5173 で実行する |
| rollout 直後 | 一過性の 000 / 502 | `smoke.sh:76-90`、ADR-0203 §5 | 自動再試行(既定 30 回・1 秒間隔) |
| pokedex 未投入 | `pokedex=503` / `master_unavailable`、`master=example` | `smoke.sh:118-150`、ADR-0206 | `make import-fetch && make import-k8s`([db-mysql.md](db-mysql.md) §5) |
| pokedex 未接続 | 503 `upstream_unavailable` | `gw/server.go:32`、ADR-0206 | `GATEWAY_POKEDEX_URL` と pokedex Service を確認 |
| 8080 の競合 | `make dev` と k3d が同じポート | `docs/runbooks/api.md:57-66` | `DEV_GATEWAY_PORT=18080 DEV_CALC_PORT=18081 make dev` |
| api-smoke の結果が直前の apply に依存 | overlay を取り違える | ADR-0206 の却下案、`docs/impl/k8s-local.md` §8 | `make api-k3d-deploy`(local-api だけを apply)を使い、共有の `overlays/local` を丸ごと apply しない |
| 共有クラスタを他レーンが操作中 | Pod・Job が再作成される | [k8s-local.md](k8s-local.md) §9 | 読み取り(`kubectl get`)で状態を確認してから操作する |

## 6. 他レーンの smoke / e2e(存在する検証の全件)

| 検証 | 実行 | 対象コンポーネント | 内容 |
|---|---|---|---|
| balance smoke | `make balance-smoke`(`services/balance/scripts/smoke.sh`) | balance-svc(Traefik `/api/balance`。`BALANCE_URL` 既定 8080) | healthz 200 → analyze 200 / unknown pokemon 422 → coverage 200 / unknown_move 422 → ability 200 / unknown_ability 422 → threats 200 / unknown_move 422 → recommendations 200 → move-range 200 / unknown_move 422 / 状態技のみ 400。最終行に全ステータス |
| balance read model smoke | `make balance-smoke-readmodel`(`smoke-readmodel.sh`) | balance が pokedex export の read model で動く(ADR-0403) | 先頭のポケモンで analyze と recommendations が 200 |
| speed smoke | `make speed-smoke`(`services/speed/scripts/smoke.sh`。`SPEED_URL`) | speed-svc(`/api/speed`) | healthz 200 → pokemon 200(8 体)→ ヘッダ無し 400 `invalid_request` → table 200(7 tier・同速 219)→ 不明 presets 400 → position 200 → 不要フィールド 400 → 不明 pokemonId 422 |
| speed read model smoke | `make speed-smoke-readmodel`(`smoke-readmodel.sh`) | speed の read model(ADR-0603 §4) | 先頭のポケモンで pokemon / table が 200 |
| judge smoke | `make judge-smoke`(`services/judge/scripts/smoke.sh`。`JUDGE_URL`) | judge-svc(`/api/judge`) | `healthz` が 200 かつ `"status":"ok"`(ADR-0700 §5。判定 API 自体は smoke 対象外) |
| gateway smoke のテスト | `go test ./services/gateway/deploytest/` | `smoke.sh` 自体(POSIX sh・正常/壊れた構成/一過性 502) | ADR-0203 AC-S6 |
| wasm 一致テスト | `make test-wasm`(`scripts/wasm-conformance.mjs`) | `engine.wasm` ↔ ネイティブ Go | JSON バイト一致 |
| Web e2e(オフライン)`playwright.config.ts` | `make web-e2e` | Web(WASM) | `a11y`(2)・`calc`(4)・`offline`(1)・`reverse`(3)・`routing`(5)。`web/e2e/*.spec.ts` の `test(` 定義数 |
| Web e2e(オンライン)`playwright.online.config.ts` | `make web-e2e-online` | Web ↔ calc-svc | `online.spec.ts`(2) |
| Web e2e(balance)`playwright.balance.config.ts` | `make web-e2e-balance` | Web ↔ balance-svc | `balance.spec.ts`(2) |
| Web e2e(コンテナ)`playwright.container.config.ts` | `make web-e2e-container` | nginx イメージ | `container.spec.ts`(8 定義。SPA・`/assets` 404・wasm MIME/gzip・キャッシュ・`/healthz`・`/api` 404) |
| iOS ユニット | `make ios-test-unit` / `ios-swift-test`(`ios/PokeCalcKit/Tests`) | PokeCalcKit(モック・API クライアント・ViewModel) | `func test` 332 件 |
| iOS UI | `make ios-test-ui`(`ios/PokeCalcUITests`。モック強制) | SwiftUI アプリ | `func test` 16 件(4 ファイル) |
| iOS その他 | `make ios-lint` / `ios-gen-check` / `ios-check-infoplist` | 生成クライアントと `openapi.yaml` の一致、Info.plist への `POKECALC_API_BASE_URL` 反映(ADR-0500 §5) | `make ios-test` が全て束ねる |
| `make e2e` | `scripts/e2e.sh` | (未実装) | `e2e: (P4-6 で スモーク + Playwright を実装)` と表示して exit 0。**成功と数えない**(CLAUDE.md) |
| Argo CD / GitOps 検査 | [gitops-argocd.md](gitops-argocd.md) | balance / speed の `*-gitops-check`、`scripts/check-gitops.sh` 等 | 詳細は C に置く |

## カバレッジ

- **読んだ**: `docs/verify-m1.md` 全体、`web/scripts/k3d-smoke.sh` 全体、`services/gateway/scripts/smoke.sh` 全体、balance/speed/judge の `smoke*.sh`(先頭とステータス判定の行)、`web/e2e/*.spec.ts` の `test(` 定義行、`web/nginx.conf` の location、`web/src/App.tsx` のタブ・キー・popstate 部分、各画面の関数定義行、ADR-0202〜0206 の該当節、`docs/runbooks/api.md` の該当行。実応答は `curl localhost:8080`(読み取りのみ)で確認。
- **件数の突き合わせ**: web-k3d-smoke のチェック 8(healthz 待機 + `check` 7)= `k3d-smoke.sh:55-61` の 7 行 + 待機。api-smoke の表 10 行(0・0a・1〜3・4a・4b・5〜7)= `smoke.sh` の検査ブロックと対応(0a は natures 以外の 2 リクエストをまとめた行)。Web e2e の `test(` 定義 = a11y 2 + balance 2 + calc 4 + container 8 + offline 1 + online 2 + reverse 3 + routing 5 = 27。iOS の `func test` = Kit 332 + UI 16。
- **読んでいない / 確認していない**:
  - `make test` が束ねる各 `*_test.go` の中身(コンポーネントとの対応はターゲット単位まで)。
  - balance / speed の smoke の各リクエスト本文と `jq` 部分(ステータス期待値のみ)。
  - Traefik 自体のエラー本文(リポジトリに記述が無い。JSON かどうかで判別する運用に留めた)。
  - `ReverseScreen` / `BalanceScreen` の内部の状態遷移(関数定義行まで)、`web/src/screens/*.test.tsx` の個別ケース。
  - iOS の各テストの中身(件数のみ)。
- **未実装**: `make e2e`(`scripts/e2e.sh` はメッセージのみ)。画面確認 #10 は Playwright の e2e が無い(vitest のみ)。judge の smoke は healthz のみ(判定 API は対象外)。
- **リンクの検証**: 本書から張った `docs/impl/*.md`・`../verify-m1.md`・`../runbooks/api.md` の実在は作成時に確認。`gitops-argocd.md` は別 fork が作成中(未確認)。
