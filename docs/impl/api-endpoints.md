# API エンドポイント一覧

- 基準: `origin/main` 3379b03。契約 = 4 本の OpenAPI(`api/openapi.yaml`・`services/{balance,speed,judge}/api/openapi.yaml`)。実ルートは各サービスの登録コード(生成 `RegisterHandlers*` と手書き)から採取して突き合わせた。
- 共通ヘッダ: `X-Device-Id`・`X-Session-Id`(以下「ID ヘッダ」。認証ではない。検証の違いは [architecture.md §6](architecture.md))。
- ステータスは契約(OpenAPI `responses`)+ 実装で確認できたもの。`gen` = 各サービスの `internal/api/openapi.gen.go`。
- エラー本文は全サービス `{"code","message"}`。code の語彙は §9。

## 1. 入口(Traefik Ingress → 誰に届くか)

Traefik は最長一致で選ぶ。ホスト 8080 が入口(`deploy/k3d.yaml`)。

| Ingress | path(Prefix) | 転送先 Service:port | 定義 |
|---|---|---|---|
| `gateway` | `/` | `gateway`:http(80) | `deploy/k8s/base/gateway/ingress.yaml`(cloud overlay では削除) |
| `balance` | `/api/balance` | `balance`:http | `services/balance/deploy/k8s/base/ingress.yaml` |
| `speed` | `/api/speed` | `speed`:http | `services/speed/deploy/k8s/base/ingress.yaml` |
| `judge` | `/api/judge` | `judge`:http | `services/judge/deploy/k8s/base/ingress.yaml` |

- `/api/balance/*`・`/api/speed/*`・`/api/judge/*` は gateway を**通らない**(ID ヘッダの UUID 検証・CORS 処理なし)。
- `http://localhost:5173`(`make web-k3d-open`)は Ingress ではなく `kubectl port-forward svc/web 5173:80`(`web/Makefile:86`)。

## 2. gateway のルート(`services/gateway/internal/httpapi`)

`serve`(`server.go:106`)が唯一のハンドラ(`e.Any("/*", g.serve)` `:78`)。判定順: CORS プリフライト → ドット/連続スラッシュ拒否 → `matchRoute` → ID ヘッダ検証 → 上流の有無 → 転送。

| # | メソッド | パス | ID ヘッダ | 処理 | 上流 | 結果 |
|---|---|---|---|---|---|---|
| G1 | GET | `/healthz` | 不要 | gateway 自身が `{"status":"ok"}`(`server.go:139`。`routing.go:61`) | — | 200 |
| G2 | OPTIONS(`Access-Control-Request-Method` あり) | 任意 | 不要 | 許可オリジンなら CORS ヘッダを付け 204。上流に送らない(`server.go:112`)。許可: `GET, POST, OPTIONS`・`Content-Type, X-Device-Id, X-Session-Id`・max-age 600(`cors.go`) | — | 204 |
| G3 | **任意メソッド** | `/api/calc`・`/api/calc/*` | 必須(UUID) | `calcProxy`(`routing.go:63`。`/api/calcx` は不一致) | `GATEWAY_CALC_URL`(必須) | 上流の応答。接続不可/タイムアウト 503 `upstream_unavailable` |
| G4 | **任意メソッド** | `/api/pokedex/*`(`/api/pokedex` 単独は不一致) | 必須(UUID) | `pokedexProxy`(`routing.go:66`) | `GATEWAY_POKEDEX_URL`(任意) | 未設定 → 503 `upstream_unavailable`(`server.go:158`) |
| G5 | GET・HEAD のみ | `/assets/*` | 不要 | `assetsProxy`(`routing.go:69`) | `GATEWAY_ASSETS_URL`(任意) | 未設定 → 404 `not_found`。他メソッド → 404 |
| G6 | GET・HEAD のみ | 上記以外で先頭セグメントが `api`・`assets`・`healthz`・`internal` **でない**パス | 不要 | `webProxy`(`server.go:132-138`) | `GATEWAY_WEB_URL`(任意。local overlay で `http://web`) | 未設定 → 404 |
| G7 | 任意 | 上記以外すべて(`/internal/*`・`/api/unknown`・`/api/balance/*`・`//…`・`.`/`..` を含むパス) | 不要(ヘッダ前に 404) | gateway 自身が 404 `not_found` `ルートが無い` | — | 404 |

- G3・G4 はメソッドを見ずに転送する。`GET /api/calc` など契約外の組み合わせは上流(calc/pokedex)が 404 `not_found` を返す。
- 上流の `Access-Control-*` は除去し、許可オリジンのときだけ gateway が付け直す(`proxy.go:52`)。
- `GATEWAY_UPSTREAM_TIMEOUT`(既定 10s。`http.Server` の `WriteTimeout` 60s 未満必須)は応答ヘッダ待ちの上限。

## 3. 公開契約 `api/openapi.yaml`(10 操作)

gateway(G3・G4)経由で到達。ID ヘッダは `/internal` 以外の 9 操作で必須(`components/parameters` の `DeviceId`・`SessionId`)。

| # | メソッド | パス | operationId | クエリ/パス | 実装(ハンドラ) | 契約のステータス | 実装で追加確認したステータス | 呼び先 |
|---|---|---|---|---|---|---|---|---|
| 1 | GET | `/api/pokedex/species` | `searchSpecies` | `q`・`format`・`limit`(1〜200・既定 50) | pokedex `search.go:78` `SearchSpecies` | 200・503 | 400 `invalid_input`/`invalid_enum`(`resolveLimit` `:46`・`checkFormat` `:57`) | MySQL(`GetDefaultRegulation`・`SearchSpecies`) |
| 2 | GET | `/api/pokedex/species/{key}` | `getSpecies` | `key` = `^[0-9]{4}-[0-9]{3}$` | pokedex `search.go:158` `GetSpecies` | 200・404・503 | 400 `invalid_input`(形式不正) | MySQL |
| 3 | GET | `/api/pokedex/moves` | `searchMoves` | `q`・`limit` | pokedex `search.go:105` `SearchMoves` | 200・503 | 400 | MySQL |
| 4 | GET | `/api/pokedex/moves/{key}` | `getMove` | `key` | pokedex `search.go:202` `GetMove`(使用可能集合の外も返す) | 200・404・503 | — | MySQL(`GetMove`) |
| 5 | GET | `/api/pokedex/items` | `searchItems` | `q`・`limit` | pokedex `search.go:133` `SearchItems` | 200・503 | 400 | MySQL |
| 6 | GET | `/api/pokedex/natures` | `listNatures` | — | pokedex `search.go:219` `ListNatures`(0 行 → 503) | 200・503 | — | MySQL(`ListNatures`) |
| 7 | POST | `/api/calc` | `calcDamage` | 本文 `CalcRequest` | calc `server.go:177` `CalcDamage` | 200・400・500・503 | — | `MemoryStore`・`engine.CalcDamage` |
| 8 | POST | `/api/calc/bulk` | `calcBulk` | 本文 `BulkCalcRequest` | calc `server.go:223` `CalcBulk` | 200・400・500・503 | — | `engine.CalcBulk` |
| 9 | POST | `/api/calc/reverse` | `calcReverse` | 本文 `ReverseRequest` | calc `server.go:277` `CalcReverse` | 200・400・500・503 | — | `engine.CalcReverse` |
| 10 | GET | `/internal/pokedex/master` | `getMasterExport` | — | pokedex `master.go:19` `GetMasterExport` | 200・503 | — | MySQL(全テーブル)。ID ヘッダ不要。gateway は 404 |

契約と実装のずれ(記録のみ。修正は API レーンの範囲):
- #1〜#3・#5 は契約の `responses` に 400 が無いが、実装は 400(#1 の `limit` 説明文だけが 400 `invalid_input` に言及)。#2 も 400 が契約に無い。
- calc/pokedex は同じ生成ラッパ(`services/internal/api`)を共有するため、担当外の操作は手書きの 404 スタブで塞ぐ(§4・§5)。

## 4. calc-svc の全ルート(`services/calc/internal/httpapi/server.go:50` / `readiness.go:23`)

| メソッド | パス | ID ヘッダ | ハンドラ | 結果 |
|---|---|---|---|---|
| POST | `/api/calc`・`/api/calc/bulk`・`/api/calc/reverse` | 必須(生成ラッパ。UUID 形式は見ない) | `registerCalcRoutes` `server.go:70` → `wrapper.Calc*`。k3d(URL 方式)は `registerDeferredCalcRoutes` `readiness.go:42` | 200 / 400 / 500 / マスタ未取得の間 503 `master_unavailable` |
| GET | `/healthz` | 不要 | `healthzHandler` `server.go:92` | 200(常に) |
| GET | `/readyz` | 不要 | `readyzHandler` `server.go:63`(deferred は `readiness.go:31`) | 200。deferred でマスタ未取得 → 503 `master_unavailable` |
| GET | `/api/pokedex/species`・`/api/pokedex/species/:key`・`/api/pokedex/moves`・`/api/pokedex/moves/:key`・`/api/pokedex/items`・`/api/pokedex/natures` | 不要(ラッパ非経由) | `registerPokedexNotFoundRoutes` `server.go:82` | 404 `not_found`(担当外) |
| その他(`/internal/*`・メソッド違い含む) | — | — | `httpErrorHandler` `server.go:116` | 404 `not_found` |

- 手書きスタブの理由: ラッパ経由だとヘッダ/クエリ検証が先に走り「担当外は常に not_found」が崩れる(`server.go:76-80` のコメント)。
- 上限: 本文 1MiB(`maxRequestBodyBytes` `server.go:32`)、件数上限は [request-flows.md §2](request-flows.md)。

## 5. pokedex-svc の全ルート(`services/pokedex/internal/httpapi/server.go:32`)

| メソッド | パス | ID ヘッダ | ハンドラ | 結果 |
|---|---|---|---|---|
| GET | `/api/pokedex/species`(`:48`)・`/species/:key`(`:49`)・`/moves`(`:50`)・`/moves/:key`(`:51`)・`/items`(`:52`)・`/natures`(`:53`) | 必須(生成ラッパ) | §3 #1〜#6 | 200 / 400 / 404 / 503 `master_unavailable` |
| GET | `/internal/pokedex/master`(`:54`) | 不要 | `GetMasterExport` | 200 / 503 |
| GET | `/healthz` | 不要 | `healthzHandler` `:68`(DB に触れず常に 200) | 200 |
| POST | `/api/calc`・`/api/calc/bulk`・`/api/calc/reverse`(`:62-64`) | 不要(ラッパ非経由) | `registerCalcNotFoundRoutes` `:60` | 404 `not_found` |
| その他 | — | — | `httpErrorHandler` `errors.go:72` | 404 `not_found` |

- pokedex は起動時に DB へ接続しない(`sql.Open` のみ)。DB 不通・未投入は各操作が 503(`errors.go:53` `unavailable`)。
- サブコマンド(HTTP ではない): `pokedex serve`・`pokedex export -out <dir>`(`cmd/pokedex/main.go:67`)。

## 6. balance-svc(`services/balance`。Traefik `/api/balance`)

`internal/httpapi/server.go:101` `New` が `api.RegisterHandlersWithOptions` で登録(業務 5 操作に `requireRequestContext`)。ID ヘッダは**非空のみ**検証、不足は 400 `missing_request_context`。本文上限 16KiB(`maxAnalyzeBodyBytes`)。

| # | メソッド | パス | operationId | ID ヘッダ | ハンドラ | 契約のステータス |
|---|---|---|---|---|---|---|
| 1 | GET | `/healthz` | `health` | 不要 | `server.go:123` → `:145` `health` | 200 |
| 2 | GET | `/api/balance/healthz` | `publicHealth` | 不要 | `server.go:127` | 200 |
| 3 | POST | `/api/balance/v1/team-balance/analyze` | `analyzeTeamBalance` | 必須 | `server.go:131` → `:149` `analyze` | 200・400・413・422・503・500 |
| 4 | POST | `/api/balance/v1/team-balance/coverage` | `analyzeTeamCoverage` | 必須 | `server.go:136` → `:355` `coverage` | 同上 |
| 5 | POST | `/api/balance/v1/team-balance/threats` | `analyzeTeamThreats` | 必須 | `server.go:141` → `threats.go:22` | 同上 |
| 6 | POST | `/api/balance/v1/team-balance/recommendations` | `recommendTeamTypes` | 必須 | `recommendations.go:13` → `:28` | 同上 |
| 7 | POST | `/api/balance/v1/move-range/analyze` | `analyzeMoveRange` | 必須 | `moverange.go:13` → `:25` | 同上 |

- 503 の条件: 該当 read model が未設定(`BALANCE_POKEMON_TYPES_PATH`・`BALANCE_MOVES_PATH`・`BALANCE_ABILITIES_PATH`)。

## 7. speed-svc(`services/speed`。Traefik `/api/speed`)/ judge-svc(`services/judge`。Traefik `/api/judge`)

ID ヘッダの検証: judge は非空のみ、不足は 400 `invalid_request`(`server.go:79`)。speed は gateway と同じ正準形 UUID
検証(ADR-0606)、欠落は `missing_header`・不正/重複は `invalid_header`(`requestctx.go`)。

| サービス | # | メソッド | パス | operationId | ID ヘッダ | ハンドラ | 契約のステータス |
|---|---|---|---|---|---|---|---|
| speed | 1 | GET | `/healthz` | `health` | 不要 | `server.go:57` | 200 |
| speed | 2 | GET | `/api/speed/healthz` | `publicHealth` | 不要 | `server.go:61` | 200 |
| speed | 3 | GET | `/api/speed/v1/pokemon` | `listPokemon` | 必須 | `server.go:65` → `:168` | 200・400・503・500 |
| speed | 4 | GET | `/api/speed/v1/table`(クエリ `presets`) | `getSpeedTable` | 必須 | `server.go:72` → `:84` | 200・400・503・500 |
| speed | 5 | POST | `/api/speed/v1/position` | `getSpeedPosition` | 必須 | `server.go:78` → `position.go:25` | 200・400・413・422・503・500 |
| judge | 1 | GET | `/healthz` | `health` | 不要 | `server.go:52` | 200 |
| judge | 2 | GET | `/api/judge/healthz` | `publicHealth` | 不要 | `server.go:57` | 200 |
| judge | 3 | POST | `/api/judge/v1/outspeed-and-ko` | `outspeedAndKo` | 必須 | `server.go:64` → `outspeed.go:120` | 200・400・413・422・503・500 |

- speed の 503: `SPEED_POKEMON_PATH` 未設定(`master_unavailable`)。judge の 503: `JUDGE_POKEDEX_BASE_URL`・`JUDGE_CALC_BASE_URL` 未設定または上流不通(`upstream_unavailable`)。

## 8. web(nginx。`web/nginx.conf`、Service `web`:80 → :8080)

| メソッド | パス(location) | 応答 |
|---|---|---|
| GET | `= /healthz` | 200 `ok`(k8s probe。gateway 越しには届かない) |
| any | `= /api`・`^~ /api/` | 404(gateway の持ち物。index.html で代用しない) |
| GET | `^~ /assets/` | `try_files $uri =404`、`Cache-Control: public, max-age=31536000, immutable` |
| GET | `= /engine.wasm`・`= /wasm_exec.js` | `no-cache`。`.wasm` は `application/wasm` |
| GET | `/`(それ以外) | `try_files $uri /index.html`(SPA。`no-cache`) |

- SPA 画面パス(クライアント側): `/calc`・`/reverse`・`/balance`・`/speed`(`web/src/app/routes.ts`)。

## 9. エラーコード → ステータス(サービス別)

| サービス | code の出所 | 400 | 404 | 413 | 422 | 500 | 503 |
|---|---|---|---|---|---|---|---|
| gateway・calc・pokedex(ルート契約 `ErrorCode`) | `api/openapi.yaml` `ErrorCode`(24 値) | `invalid_json`・`unknown_field`・`invalid_enum`・`invalid_input`・`unknown_preset`・`duplicate_preset`・`invalid_preset`・`invalid_reverse_side`・`no_observation`・`invalid_observation`・`unknown_type`・`missing_header`・`invalid_header`・`unknown_species`/`move`/`item`/`ability`/`nature` | `not_found` | — | — | `internal`・`type_chart_missing`・`invalid_type_chart`(calc `errors.go:42` `statusForCode`) | `master_unavailable`・`upstream_unavailable` |
| balance | `services/balance/api/openapi.yaml` | `missing_request_context`・`invalid_request` | — | `request_too_large` | `unknown_pokemon`・`unknown_move`・`unknown_ability` | `internal_error` | `master_unavailable` |
| speed | 同上 | `missing_header`・`invalid_header`・`invalid_request` | — | `request_too_large` | `unknown_pokemon` | `internal_error` | `master_unavailable` |
| judge | 同上 | `invalid_request` | — | `request_too_large` | `unknown_species`・`unknown_nature` | `internal_error` | `upstream_unavailable` |

- 400/404 の切り分けの目安: gateway が返す 400(`missing_header`/`invalid_header`)は ID ヘッダ、404 `not_found` は gateway のルート外(または `/internal/*`)か、上流の担当外。詳細な切り分けは D 章で扱う。

## 10. 件数の突き合わせ

| 区分 | 契約(OpenAPI paths×methods) | 生成の登録(`RegisterHandlersWithOptions`) | 手書きの登録 |
|---|---|---|---|
| ルート契約 `api/openapi.yaml` | 10 | 10(`services/internal/api/openapi.gen.go:1886-1895`) | calc: `/healthz`・`/readyz`・pokedex 404 スタブ 6 / pokedex: `/healthz`・calc 404 スタブ 3 |
| balance | 7 | 7(`gen:1100-1106`) | なし |
| speed | 5 | 5(`gen:623-627`) | なし |
| judge | 3 | 3(`gen:502-504`) | なし |
| gateway | — | — | G1〜G7(単一の `e.Any("/*")`) |
| web(nginx) | — | — | location 7 種(§8。`/healthz`・`/api`・`/api/`・`/assets/`・`/engine.wasm`・`/wasm_exec.js`・`/`) |

契約 25 = 生成登録 25(差なし)。契約外で存在するルート: calc の `/healthz`・`/readyz`・404 スタブ 6、pokedex の `/healthz`・404 スタブ 3、balance/speed/judge の `/healthz`(契約に載っている)。

## カバレッジ

- 読んだ: 4 本の OpenAPI の `paths`・`operationId`・`responses`・`parameters` 名(機械抽出)・`ErrorCode` 節、生成 4 ファイルの `RegisterHandlersWithOptions`、gateway `httpapi` 全ファイル・`cmd/gateway/main.go`、calc `server.go`・`readiness.go`・`errors.go`・`limits.go`、pokedex `server.go`・`search.go`・`errors.go`・`master.go`(冒頭)、balance `server.go`・(各ハンドラ冒頭のコメント)・speed `server.go`・judge `server.go`・`outspeed.go`(`:113-200`)、`web/nginx.conf`、Ingress 4 件(YAML)。
- 読めていない: 各 OpenAPI の本文スキーマ(フィールド定義)・`description` の全文、speed/balance のハンドラ本体の分岐(ステータスは契約表記を採用)、pokedex `master.go` 後半、iOS 生成クライアントが実際に叩くパス。
- 実行確認はしていない(ステータスはコード・契約の読み取り)。実測は D 章で扱う(`api-smoke` は 400/404 を意図的に確認: `missing_header`・`invalid_header`・`/internal/pokedex/master`)。
- 未実装: record・team の API は契約にも実装にも無い。
