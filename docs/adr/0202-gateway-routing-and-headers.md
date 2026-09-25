# ADR-0202: gateway のルーティングとヘッダ検証

- 状態: 採用(2026-09-22。P3-2 の設計。受け入れ条件とテストは spec-writer が先に書き、実装は implementer)
- 日付: 2026-09-22
- 関連: ADR-0001(技術スタック)、ADR-0012(サービス境界。balance は兄弟で `/api/balance` は独自の Ingress)、
  ADR-0200(calc-svc の契約・ErrorCode の語彙と HTTP ステータス・healthz の扱い)、ADR-0201(Echo v5)、
  docs/requirements.md §3(認証なし・端末ID)・§4(アーキテクチャ)・§8(画像は gateway の `/assets/` から配信)、plan.md P3-2

## 背景

クライアント(Web / iOS)の入口は gateway ただ1つ(requirements.md §4)。calc-svc(P3-1)は出来ており、pokedex-svc(P2-3)は
まだ無い。ADR-0200 は UUID 形式の検証と `upstream_unavailable` を gateway の仕事として持ち越した。
ブラウザ版は別オリジン(Vite の開発サーバ等)から呼ぶので CORS が要る。画像は `<img>` から読むのでヘッダを付けられない。

## 決定

### 1. 構成(`services` モジュール内。Echo v5.3.1。上流への転送は標準ライブラリの `net/http/httputil.ReverseProxy`)

- `services/gateway/internal/httpapi`: `Config` と `NewHandler(cfg Config) (http.Handler, error)`。`Config` が不正
  (CalcURL が無い・UpstreamTimeout が 0 以下・許可オリジンに `*`)なら `ErrInvalidConfig` を包んで返す。
  上流への RoundTripper は非公開フィールド `transport` でテストだけが差し替える(nil なら既定)。
- `services/gateway/cmd/gateway`: `loadConfig(lookup)` と `run(ctx, lookup)` を切り出す(calc-svc と同じ形)。設定の不正は
  `errInvalidConfig` を包む。`http.Server` にタイムアウトを設定する(ADR-0200 R7 と同じ理由)。
- 新しい外部依存は足さない(UUID の検証は正規表現か手書きの走査。`google/uuid` の `Parse` は波括弧・`urn:uuid:`・ハイフン無しも
  受け付けるので正準形の判定には使えない)。

### 2. 環境変数(1か所で読み、起動時に検証する。docs/coding-rules.md §2)

| 変数 | 必須 | 既定 | 検証 |
|---|---|---|---|
| `GATEWAY_ADDR` | いいえ | `:8080` | 空は既定 |
| `GATEWAY_CALC_URL` | はい | なし | 絶対 URL・スキームは http/https・ホストあり。無い・空・不正は起動エラー |
| `GATEWAY_POKEDEX_URL` | いいえ | 未設定 | 同上(空は未設定)。未設定なら `/api/pokedex/*` は 503 `upstream_unavailable` |
| `GATEWAY_ASSETS_URL` | いいえ | 未設定 | 同上(空は未設定)。未設定なら `/assets/*` は 404 `not_found` |
| `GATEWAY_CORS_ALLOWED_ORIGINS` | いいえ | 空 | カンマ区切り(前後の空白は除く)。各要素は `scheme://host[:port]` のオリジン(パス・末尾スラッシュなし)。`*` は起動エラー。空なら CORS ヘッダを付けない |
| `GATEWAY_UPSTREAM_TIMEOUT` | いいえ | `10s` | Go の duration。0 以下・解析できない値は起動エラー |

### 3. ルーティング

| パス | 転送先 | 備考 |
|---|---|---|
| `/api/calc`、`/api/calc/*` | calc-svc | `/api/calcx` のような前方一致は拾わない |
| `/api/pokedex/*` | pokedex-svc | `/api/pokedex` そのものは 404 |
| `/api/record/*` | record-svc | `/api/record` そのものは 404(2026-09-25 追記。P5-3・ADR-0209 §10-1)。上流(`GATEWAY_RECORD_URL`)未設定なら 503 `upstream_unavailable` |
| `/api/team/*` | team-svc | `/api/team` そのもの・`/api/teamx` は 404(2026-09-25 追記。P5-4・ADR-0213 §7)。上流(`GATEWAY_TEAM_URL`)未設定なら 503 `upstream_unavailable` |
| `/assets/*`(GET / HEAD のみ) | assets の上流(MinIO) | それ以外のメソッドは 404 `not_found` |
| `GET /healthz` | gateway 自身 | 200 `{"status":"ok"}`。openapi に載せない(ADR-0200 と同じ)。上流の `/healthz` は外に出さない |
| それ以外(`/api/balance` を含む) | なし | 404 `not_found`(Error 形式)。`/api/balance` は独自の Ingress(ADR-0012) |

- パスとクエリはそのまま転送する(上流の基底 URL にパスがあれば前に連結する。`ReverseProxy` の標準の連結)。
  gateway は `/api/calc` の中の操作を知らない(メソッド違い・未知の下位パスの判定は上流に任せる)。
- **ドットセグメント**(`.` / `..` のセグメント)を含むパスは 404 `not_found` にし、どの上流にも送らない
  (`/api/calc/../../healthz` のような経路で上流の運用エンドポイントや他のルートに抜けさせない)。
- 判定の順序: CORS プリフライト(`OPTIONS` + `Access-Control-Request-Method`。ここで 204 を返し、以降には進まない)→
  ドットセグメント拒否 → ルーティング(未知のパスは 404。ヘッダが無くても 404)→ ヘッダ検証(`/api/*` だけ)→
  上流の有無(pokedex 未設定は 503)→ 転送。

### 4. ヘッダ検証(`/api/*` のみ)

- `X-Device-Id` / `X-Session-Id` の欠落・空 → 400 `missing_header`。
- UUID でない値(正準形 8-4-4-4-12 の16進以外。大文字小文字・版は問わない)・同名ヘッダの重複 → 400 **`invalid_header`**(新しい ErrorCode)。
- 欠落と不正が同時にあれば `missing_header` を優先する。
- `/assets/*`・`/healthz`・CORS プリフライト(OPTIONS + `Access-Control-Request-Method`)には課さない(`<img>` はヘッダを送れない。
  プリフライトにはブラウザが独自ヘッダを付けない)。
- 検証を通ったリクエストのヘッダは書き換えずに上流へ転送する(大文字の UUID も大文字のまま)。
- **追記(2026-09-25。issue #326「転送ヘッダ」)**:
  - 検証済みの `X-Device-Id` / `X-Session-Id` は `/api/*` の上流(calc・pokedex)へ**必ず**届ける。クライアントが
    `Connection: X-Device-Id, X-Session-Id` と列挙すると、`ReverseProxy` は `Rewrite` の前に hop-by-hop として消す
    (上流は `missing_header` を返し、record/team が gateway の検証を前提にすると空の ID を受ける)。`Rewrite` で受信側の
    値(検証済みでちょうど1つ)を `Set` し直す。値は書き換えない(大文字もそのまま)。`/assets/*`・Web への転送は
    ヘッダを検証しないので付け直さない。拒否(400 `invalid_header`)ではなく付け直しを選んだのは、`Connection` の
    列挙は HTTP として正しく、検証済みのリクエストを別理由で落とす必要がないため。
  - クライアントが送った `X-Real-Ip` と `Forwarded` はどの上流にも転送しない(偽装したクライアント IP を上流が信じない
    ように)。`X-Forwarded-For` は従来どおり `SetXForwarded` が gateway の直前の相手の IP で付け直す(§5)。
  - **対象外(人間の判断待ち)**: k3d / 公開構成では gateway の直前は Traefik なので、`X-Forwarded-For` に入るのは
    Traefik の Pod IP で、実際のクライアント IP は上流に残らない。信頼するプロキシ(Traefik)を設定で持ち、そこからの
    `X-Forwarded-For` だけ引き継ぐかは、公開構成と合わせて決める(issue #246・#326。既定案: 公開時に
    `GATEWAY_TRUSTED_PROXIES` の CIDR 一覧を足し、その範囲からの接続に限って `X-Forwarded-For` の右端を採る)。

### 5. 上流の失敗

- 接続できない・タイムアウト(`GATEWAY_UPSTREAM_TIMEOUT`。上流の応答ヘッダが届くまでの上限。本文の転送は打ち切らない)→
  503 `upstream_unavailable`(Error 形式。Go の内部情報[dial・アドレス・context のエラー文]を message に出さない)。
- 上流が返したレスポンス(4xx / 5xx を含む)はステータス・ヘッダ・ボディをそのまま返す(gateway は書き換えない。ただし CORS ヘッダは §6 のとおり付け替える)。
- 転送は `httputil.ReverseProxy` の `Rewrite`(`Director` は非推奨)を使い、`ProxyRequest.SetURL` で Host を上流のホストに書き換え、
  `SetXForwarded` でクライアントが送った `X-Forwarded-For` / `X-Forwarded-Host` / `X-Forwarded-Proto` を信用せず実際の値に付け替える。
- **追記(2026-09-24。issue #113「クライアントのcancel伝播」)**: クライアントが要求を中断した(ブラウザの
  `AbortSignal`・iOS の `Task` cancel)ときは `upstream_unavailable` として扱わない。Go の `http.Server` は
  クライアントの接続が切れると `r.Context()` を `context.Canceled` で終える。`ReverseProxy.ErrorHandler` は
  `err` が `errors.Is(err, context.Canceled)` のときだけ特別扱いし、上流障害の WARN ログを出さず(Debug に
  留める。運用上のノイズと誤検知を避けるため)、応答も書かない(クライアントは既に居ないので届かない)。
  自前のタイムアウト(`GATEWAY_UPSTREAM_TIMEOUT` の `ResponseHeaderTimeout`/dial)が切れたときのエラーは
  `context.Canceled` にならない(`net/http: timeout awaiting response headers` 等の別のエラーになる。実測で
  確認済み)ため、この特別扱いは本来の `upstream_unavailable`(タイムアウト・接続不可)とは混同しない。
  `services/gateway/internal/httpapi/upstream_test.go` の `TestClientCancelIsNotUpstreamUnavailable`
  (フェイクの RoundTripper 版・実 `http.Transport` 版の両方)で固定。
  **限界(2026-09-24 の調査で判明。対応はしない)**: gateway → calc-svc への `context` のキャンセル伝播
  そのものは効く(実測で `r.Context().Done()` が即座に発火することを確認)。しかし
  `services/calc/` のハンドラは受け取ったリクエストの `context.Context` を一度も見ておらず、
  `engine`(`engine.CalcReverse`/`CalcBulk` 等)も `context` を受け取らない(絶対ルール2「engine は
  純粋に保つ」により、キャンセル検査のためだけでも `context` を持ち込む変更はしない判断)。
  そのため issue #113 の達成目標が挙げる「calc-svc CPU消費も止める」は本追記の範囲では**達成しない**:
  クライアントが中断しても、gateway は静かに応答を打ち切るだけで、calc-svc 側の計算(特に逆算の
  総当たり探索)は最後まで完走する。1リクエストあたりの最悪計算量は ADR-0208(件数・範囲の上限)で
  有界なので、実害は「無駄な計算がその上限の範囲で起こりうる」程度に留まる。engine への `context`
  導入が必要になったときは別 ADR で扱う。
- **追記(2026-09-25。issue #209「`Expect: 100-continue` で上流の 4xx/5xx が 200 になる」)**: gateway→上流の
  リクエストからは `Expect` ヘッダを取り除く(`Rewrite` で `pr.Out.Header.Del("Expect")`)。上流が `100 Continue` を
  返すと `ReverseProxy` はそれを `WriteHeader(100)` で転送するが、Echo の `Response` は最初の `WriteHeader` で commit
  され、続く上流のステータスを捨てて(`echo: response already written to client`)既定の 200 を出していた。
  `Expect` が無ければ `http.Transport` は本文を即送り、上流は 1xx を返さない。クライアント側の `Expect` には gateway の
  `http.Server` が本文を読むときに自動で `100 Continue` を返すので、クライアントから見た挙動は変わらない。
  却下: `echo.UnwrapResponse` で素の `ResponseWriter` を渡す案(1xx は正しく転送されるが、`httpmetrics` が Echo の
  `Response.Status` を読むため status のメトリクスが壊れ、追加の対処が要る)。

### 6. CORS

- 許可オリジンに**完全一致**する `Origin` のときだけ `Access-Control-Allow-Origin: <そのオリジン>` と `Vary: Origin` を付ける。
  上流の応答・gateway 自身のエラー・`/assets` のどれにも付ける(ブラウザがエラー本文を読めるように)。
- 上流を経由する応答は、上流が独自に付けた `Access-Control-*`(誤った `*` や別オリジンの反射を含む)を`ReverseProxy.ModifyResponse`
  で全部取り除いてから、許可オリジンのときだけ gateway 自身の ACAO を1つだけ付け直す(上流の判断をそのまま外へ出さない)。
- プリフライト(OPTIONS + `Access-Control-Request-Method`)は 204(本文なし)で、上流に送らない。許可オリジンなら
  `Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS`(**2026-09-25 追記。P5-3**: `DELETE` は端末単位の全削除 API
  〈ADR-0209 §5・§10-2〉のために足した。これが無いと別オリジンのブラウザからプリフライトが通らず Web から呼べない。
  **2026-09-25 追記。P5-4**: `PUT` は構築の置換〈`updateTeam`。ADR-0213 §2・§7〉のために足した。
  `PATCH` は足さない — ADR-0213 §2 で部分更新を持たないと決めたため)、`Access-Control-Allow-Headers: Content-Type, X-Device-Id, X-Session-Id`、
  `Access-Control-Max-Age: 600`。許可外のオリジン・CORS 未設定なら CORS ヘッダ無しの 204(ブラウザが拒否する)。
- 認証なしなので `Access-Control-Allow-Credentials` は付けない。`*` は使わない(設定でも拒否)。

### 7. エラー

- panic は回復して 500 `internal`(panic の値・スタックを出さない)。
- echo の既定エラー(ルート無し・メソッド違い)は Error 形式の 404 `not_found`(ADR-0200 と同じ)。

### 8. 契約(`api/openapi.yaml`。絶対ルール1。`make gen` で再生成)

1. `ErrorCode` に `invalid_header`(400)を追加。対応表: `missing_header` は「欠落・空」、`invalid_header` は「UUID でない・重複」。
2. `DeviceId` / `SessionId` パラメータの schema に `format: uuid` を付け、description に gateway が検証する旨を書く。
   **`x-go-type: string` で生成型は string のまま**にする(oapi-codegen は `format: uuid` を `openapi_types.UUID` にし、生成ラッパが
   bind 時に UUID を解析してしまう。calc-svc は UUID 形式を検証しない[ADR-0200 AC-6]ので、それを保つ)。
3. calc の3操作の '503' と pokedex の5操作(新たに '503' を追加)に、`upstream_unavailable` が返りうることを書く。

### 9. calc-svc の語彙の変更(期待値の変更)

calc-svc は同名ヘッダの重複を `invalid_input` にしていた(ADR-0200 §1.6・critic 指摘 R1)。`invalid_input` は本来「入力検証
(SP・ランク等)」の語彙で、ヘッダの形式の失敗とは意味が違う。gateway が同じ失敗を `invalid_header` にするので、
**calc-svc も `invalid_header` に揃える**(gateway を経由しない直叩き[クラスタ内・テスト]でも同じ失敗は同じ code)。
既存テスト `TestDuplicateHeaderIsInvalidInput` は `TestDuplicateHeaderIsInvalidHeader` に改め、期待値を `invalid_header` に変える
(ケースは消さない・弱めない)。calc-svc は引き続き UUID 形式を検証しない。

## 受け入れ条件と担当テスト

テストはすべて `services/gateway/...`(`services/gateway/internal/httpapi` と `services/gateway/cmd/gateway`)。

| AC | 内容 | テスト |
|---|---|---|
| AC-G1 | calc 3操作・pokedex・assets(GET/HEAD)がそれぞれの上流に、メソッド・パス・クエリ・ボディ・ヘッダ付きで1回だけ届き、他の上流には届かない。上流のステータス・ボディ・Content-Type・Cache-Control がそのまま返る | `TestRoutesReachTheirUpstream` |
| AC-G2 | 上流の 4xx / 5xx(Error 本文を含む)と契約外のステータスも書き換えずに返す | `TestUpstreamResponsesPassThroughUnchanged` |
| AC-G3 | `/api/balance`・未知のパス・`/api/calcx`・`/api/pokedex`・`/assets` への POST/PUT/DELETE・ドットセグメントは 404 not_found で上流に届かない(未知の /api パスはヘッダ無しでも 404)。`GET /healthz` は gateway 自身の 200 で上流に届かない | `TestUnroutedPathsAreNotFound` / `TestHealthzIsGatewayOwn` |
| AC-G4 | 欠落・空 → missing_header、UUID 不正(桁違い・非16進・ハイフン位置・ハイフン無し・波括弧・urn)・重複 → invalid_header、欠落優先。上流に届かない。大文字・混在・版違いは通り、値はそのまま転送。/assets・/healthz・プリフライトは検証しない | `TestHeaderValidationRejects` / `TestHeaderValidationAccepts` / `TestHeaderValidationExemptions` |
| AC-G5 | 接続拒否・タイムアウト(calc / pokedex / assets)→ 503 upstream_unavailable(内部情報を出さない。タイムアウトで打ち切られる)、pokedex 未設定 → 503、assets 未設定 → 404、ヘッダ検証は上流の有無より先 | `TestUpstreamFailures` |
| AC-G6 | CORS: 許可オリジンの単純リクエスト(上流の応答・gateway のエラー・/assets)に ACAO と Vary: Origin、credentials なし。許可外・末尾スラッシュ・ポート違い・null・Origin 無し・CORS 未設定には付けない。プリフライトは 204 で Allow-Methods / Allow-Headers / Max-Age、許可外・未設定は CORS ヘッダ無しの 204、上流に届かない | `TestCORSSimpleRequests` / `TestCORSPreflight` / `TestCORSDisabledWhenNoOrigins` |
| AC-G7 | panic は 500 internal(panic の値を出さない)。Config の不正は ErrInvalidConfig | `TestPanicIsRecoveredAsInternal` / `TestNewHandlerRejectsInvalidConfig` |
| AC-G8 | 契約: invalid_header が ErrorCode にある。gateway のエラー(missing_header / invalid_header / not_found / upstream_unavailable)が契約どおり。上流が calc-svc の実物(架空マスタ)のとき calc・bulk・reverse の成功と calc-svc の 400 が gateway 経由で契約どおり | `TestContractHasInvalidHeader` / `TestGatewayErrorsMatchContract` / `TestRealCalcThroughGatewayMatchesContract` |
| AC-G9 | 起動: 環境変数名、必須・既定・任意の読み込み、不正な URL・`*`・オリジンでない値・不正なタイムアウトは errInvalidConfig、run は /healthz に答え ctx の終了で nil で止まる、設定不正なら待ち受けずにエラー | `cmd/gateway.TestEnvNames` / `TestLoadConfig` / `TestLoadConfigRejects` / `TestRunServesAndStopsOnContextCancel` / `TestRunFailsOnInvalidConfig` |
| AC-G10(2026-09-24追記。issue #113) | クライアントが要求を中断した(`context.Canceled`)ときは `upstream_unavailable` を書かない(応答なし)。自前のタイムアウト(別のエラー文言)とは区別される | `TestClientCancelIsNotUpstreamUnavailable` |
| AC-G11(2026-09-25追記。issue #209) | `Expect: 100-continue` 付きのリクエストでも、上流(calc・pokedex・Web)のステータスと本文がそのまま返る。上流に `Expect` は届かず、Echo の二重 `WriteHeader` のログが出ない | `TestExpectContinueKeepsUpstreamStatus` |
| AC-G12(2026-09-25追記。issue #326) | `Connection` に `X-Device-Id` / `X-Session-Id` を列挙しても、calc・pokedex に検証済みの値がちょうど1つずつ届く。クライアントの `X-Real-Ip` / `Forwarded` はどの上流(calc・pokedex・assets・Web)にも届かず、`X-Forwarded-For` は gateway が付け直す | `TestVerifiedIDsSurviveConnectionHeader` / `TestClientIPHeadersAreNotForwarded` |
| AC-C1 | calc-svc: 同名ヘッダの重複は 400 invalid_header(§9) | `services/calc/internal/httpapi.TestDuplicateHeaderIsInvalidHeader` |

calc-svc の実物は `services/calc/calctest`(`NewExampleHandler`。例のマスタと共有の相性表で `httpapi.NewHandler` を作る)で起動する。
calc-svc の `httpapi` / `master` は `services/calc/internal` にあり、Go の internal 規則で gateway から import できないため、
テスト専用の薄い入口を calc 側に置く(本番コードからは使わない)。

## 却下した案

- **UUID の検証を calc-svc(と各サービス)でも行う**: 検証が重複し、語彙の揺れの元になる。入口の gateway に一本化する(ADR-0200 AC-6)。
- **`format: uuid` で生成型を UUID にする**: 生成ラッパが下流でも UUID を解析してしまい、calc-svc の「形式を見ない」契約と食い違う。
- **CORS で `*` を許す**: 認証なしでも、許可するオリジンを明示する方が意図が読める。誤設定は起動時に落とす。
- **重複ヘッダを `invalid_input` のまま残す**: 同じ失敗が gateway と calc-svc で別の code になる。
- **ドットセグメントを正規化して転送する**: 正規化後のパスで改めてルーティングする必要があり、抜け道の検査が増える。拒否が単純。
- **サードパーティの CORS / プロキシのミドルウェア**: 標準ライブラリと Echo で足りる(依存を増やさない)。

## 影響

- Web(P4)・iOS は `invalid_header` を扱う(生成型の ErrorCode に追加される)。
- P3-3 の契約テストは `TestGatewayErrorsMatchContract` / `TestRealCalcThroughGatewayMatchesContract` を土台にする。
- k8s の manifest(deploy/k8s)は上の環境変数で gateway を設定する(本 ADR では manifest は変えない)。
- ADR-0200 §1.6 の「重複は invalid_input」は本 ADR §9 で `invalid_header` に変わる。
