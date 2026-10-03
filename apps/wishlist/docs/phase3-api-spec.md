# wishlist フェーズ3(目安価格)API の受け入れ条件

仕様の正は [../CLAUDE.md](../CLAUDE.md)(§6 目安価格・§7 DB・§8 API・§11 k8s・§12 フェーズ3・§13 開発ルール)、設計は [design.md](design.md)、
API 契約は [../api/openapi.yaml](../api/openapi.yaml)。書き方はフェーズ1の [phase1-api-spec.md](phase1-api-spec.md) に合わせる。
テストは `apps/wishlist/api` 配下(`make wishlist-test`。MySQL 実装は `make wishlist-test-mysql`)。

## 決めたこと(仕様に書かれていなかった部分)

### 判定と算出(internal/estimate)

- **タイトル照合(title_mismatch)**:name を Unicode の空白(全角空白・タブを含む)で分け、各トークンを `query.Normalize` する。
  正規化したタイトルにすべてのトークンが部分文字列として含まれれば一致。正規化して空になるトークンは無視し、トークンが無ければ一致とする。
  カタカナ表記(`S.H.フィギュアーツ`)などの表記揺れは吸収しない(辞書はフェーズ2以降)
- **too_cheap の閾値**:`cheap_ratio` 0.3 を整数で扱う(`price × 100 < 基準価格 × 30` なら too_cheap)。浮動小数を使わない
- **理由の順**:`title_mismatch` → `too_cheap` → `below_min`(仕様 §6 の番号順)。`below_min` は `price < min_price`(ちょうどは参考)
- **基準価格**:`is_reference` かつ fetch_type が `api` / `scrape` のサイトの出品のうち、`title_mismatch` でも `below_min` でもないものの価格の中央値。
  基準を決めてから、基準サイト自身の出品も too_cheap で判定する(2 段階)。取れなければ too_cheap を判定しない
- **中央値**:偶数件は中央 2 つの平均を切り捨てる
- **下位 25 パーセンタイル**:最近傍順位法(nearest-rank)。昇順に並べて `ceil(n/4)` 番目(1 始まり)。3 件未満は最小値、mid は null
- **件数**:`count` は参考外を除いた件数、`suspicious_count` は参考外の件数。`count` が 0 なら `no_result`(全部参考外でも no_result)
- **在庫(仕様 §3 の「状態」。design.md の未決事項)**:`in_stock_count`(参考外を除き、在庫ありの件数)を `SiteEstimate` と `estimates` に足す
- **サマリ**:low を持つサイト(候補。failed で前回値を持つサイトも含む)のうち low が最小のサイトを選び、`summary_low` はその low、
  `summary_mid` はそのサイトの mid(null ならその low)。low が同じなら mid(null は low とみなす)の小さい方、それも同じなら表示順で先。
  `summary_fetched_at` は**候補の** fetched_at のうち最も古いもの(サマリの「最も低い」が言える時点のため)

### 取得(internal/fetcher)

- **登録表**:`api` → Yahoo!ショッピング(appid があるときだけ)、`scrape`・`headless` → 未実装(取得しない)、`link_only` → 取得しない。
  「取得できるサイト」= 登録表に Fetcher があるサイト。appid が無い・空白だけなら api も取得しない(エラーにしない)
- **Yahoo のリクエスト**:`GET …/V3/itemSearch?appid&query&sort=%2Bprice&results=20`。`in_stock` は付けない(在庫の有無を `in_stock_count` で見せるため)
- **Yahoo の応答**:`hits[].name`・`price`・`url`・`image.small`(無ければ `exImage.url`、どちらも無ければ空)・`inStock`。
  price が 1 未満・name か url が空の hit は除く。応答本文は 2 MiB まで。エラーの文言に appid を含めない(`*url.Error` は URL を含むため包み直す)
- **アクセスの節度**:同じサイト(sites.id)へのリクエストは直列にし、前回の取得が**終わってから** 5 秒あける(失敗した取得も数える。初回は待たない)。
  違うサイトは互いに待たない。待ちは差し替えられる時計(`fetcher.Clock`)で行い、テストでは実際に待たない。
  本番の登録表(`NewRegistry`)はすべての Fetcher をこの待ちで包む
- **外部取得**:既存の `netguard` のクライアント(SSRF 対策・タイムアウト 10 秒)を使う

### 更新(internal/refresh)

- **対象サイト**:ジャンルの `site_ids`(表示順)から、商品の override で `enabled=false` のものと取得できないサイトを除く。対象は 1 つずつ順番に取る
- **検索ワード**:`query.Build(テンプレート, name, option, query_override, サイト別 query)`(フェーズ1と同じ優先順位)
- **件数と列幅**:取得結果は先頭 20 件まで。タイトルは 512 文字(rune)に切り詰め、URL が 1024 文字を超える出品は捨て、image_url が長すぎる・空なら null
- **2 段階**:全対象を取り終えてから基準価格を決める。今回取得に成功しなかった基準サイト(ModeStale で取らなかった・失敗した)は、
  保存済みの出品を**基準の計算にだけ**使う(保存し直さない)
- **保存**:成功したサイトは 1 トランザクションで listings を消して入れ、estimates を upsert(fetched_at は取得した時刻)。参考外も理由つきで保存する
- **失敗**:status だけ failed にし、前回の low・mid・件数・fetched_at と listings を残す。前回値が無ければ `failed`・null・0・fetched_at = 試した時刻の行を作る
- **更新の範囲(Mode)**:
  - `ModeStale`(GET の裏の更新):目安が無い・failed・**24 時間より古い**(ちょうど 24 時間は古くない)対象のうち、
    **最後に試してから 1 時間以上**たったものだけ取る。最後に試した時刻はプロセス内で持つ(再起動で忘れてよい)
  - `ModeAll`(手動の更新・CronJob):すべての対象を取る
- **同時実行**:同じ商品の更新は同時に 1 つだけ。実行中に呼ばれたら取得を重ねず、その完了を待つ(singleflight)
- **裏の更新**:リクエストの ctx ではなく `Deps.BaseContext` を使う。`Wait()` で終わりを待てる(テスト・終了処理)

### API

- **GET estimates**:`sites` はジャンルの表示順で、**取得できる対象サイトのうち目安を保存済みのもの**だけ(link_only・enabled=false・未保存は出さない)。
  サマリはその `sites` から作る。ModeStale で取るべき対象があれば裏で更新を起動して `refreshing: true`(実行中も true)。
  対象が無ければ空の結果と `refreshing: false`
- **POST refresh**:202。本文は現時点のキャッシュ(GET と同じ形)で、`refreshing` は取得できる対象があれば true(実行中なら起動しない)、無ければ false
- **GET listings**:参考外を含む。並びは price 昇順・同額は id 昇順。`suspicious_reasons` は理由が無ければ `[]`、`image_url` は無ければ null。
  存在しない site_id は空(エラーにしない。genre_id の絞り込みと同じ)。0 以下・数でない site_id は 400
- **refresh の 501**:フェーズ1の `501 not_implemented` は廃止(openapi から 501 の応答と `NotImplemented` 応答を外した。Error の code 列挙の `not_implemented` は残す)

### cmd/refresher・k8s

- **refresher**:引数なしの別バイナリ `/wishlist-refresher`(api イメージに同梱)。全商品を順に ModeAll で更新し、1 商品の失敗で止めない。
  失敗した商品 = RefreshItem がエラー、または対象サイトがあって 1 つも取得できなかった(対象サイトが無い商品は失敗に数えない)。
  最後に `refresher: items=N failed=M` を 1 行出す。終了コード:商品があって全件失敗なら 1、設定の誤り 1、引数あり 2、それ以外 0
- **設定**:`WISHLIST_DATABASE_DSN`(必須。parseTime を付け multiStatements を外す)、`WISHLIST_YAHOO_APPID`(任意。前後の空白を除く)。
  api の `serve` も `WISHLIST_YAHOO_APPID` を任意で読む
- **起動時の DB 接続**:Pod の起動直後は NetworkPolicy の許可が反映されず接続が拒否されるため、api(serve・migrate)と refresher は起動時の Ping を再試行する(`internal/dbwait`。1 秒から 2 倍ずつ最大 5 秒、合計 30 秒。ctx の取り消しで止まる。時計と待ちは差し替え可能で `TestRetry_*` で確かめる)
- **5 秒の間隔の限界と対策**:間隔(`Throttle`)は同じプロセスの中でしか守れない。03:00 JST の CronJob と api の裏の更新が重なると、別のプロセスから同じサイトに 5 秒以内に届きうる。
  軽い対策として、api の裏の更新(GET estimates・POST refresh が起動するもの)は **02:30 以上 04:00 未満 JST には起動しない**(`refresh.InQuietWindow`。時計は `Deps.Now`。起動しないとき `refreshing` は false で、キャッシュだけを返す。refresher の同期の更新は影響を受けない。固定オフセット +09:00 で判定する。境界は `TestBackgroundQuietWindow`)。それでも重なりは完全には防げない(制限)
- **CronJob**:`schedule: "0 3 * * *"`・`timeZone: Asia/Tokyo`・`concurrencyPolicy: Forbid`・`startingDeadlineSeconds: 600`・Job の `activeDeadlineSeconds: 7200`、イメージは api(`wishlist/api`)、
  securityContext は api の Deployment と同じ。DSN と appid は Secret `wishlist-api`(appid はキー `yahoo-appid`、`optional: true`)。
  api の Deployment にも同じ appid を optional で渡す

## 受け入れ条件とテスト

### internal/estimate(純粋)— `internal/estimate/estimate_test.go`

| ID | 条件 | テスト |
|---|---|---|
| AC-E1 | タイトル照合:name を空白で分けた各トークンの Normalize がすべて正規化タイトルに含まれる(全角・大文字・記号の揺れは吸収、カタカナ表記は吸収しない) | `TestTitleMatches` |
| AC-E2 | 参考外の判定と理由の順(title_mismatch → too_cheap → below_min)。too_cheap は `price×100 < base×30`、境界は参考。base が無ければ too_cheap なし、min_price が無ければ below_min なし | `TestJudge` |
| AC-E3 | 中央値(偶数件は切り捨て。仕様 §6 の 5000・4800 → 4900)。入力を書き換えない | `TestMedian` |
| AC-E4 | サイトの目安:low = nearest-rank の 25 パーセンタイル、mid = 中央値。3 件未満は最小値と null、0 件は no_result | `TestEstimate` |
| AC-E5 | in_stock_count は参考外を除いた在庫ありの件数 | `TestEstimate_InStockCount` |
| AC-E6 | 仕様 §6 の例:基準 4900、メルカリの 300 は title_mismatch と too_cheap で参考外・サマリに含めない | `TestEvaluate_SpecExample` |
| AC-E7 | 基準価格:Reference のサイトの、title_mismatch でも below_min でもない出品の中央値。基準サイト自身も too_cheap で判定。非 Reference は基準に入れない。取れなければ too_cheap なし | `TestEvaluate_BasePrice` |
| AC-E8 | サマリ:low の最小とそのサイトの mid(null なら low)。同値の扱い。fetched_at は候補の最も古いもの | `TestSummarize` |

### internal/fetcher — `internal/fetcher/fetcher_test.go`・`yahoo_test.go`(fixture は `internal/fetcher/testdata/yahoo-itemsearch.json`。架空の内容)

| ID | 条件 | テスト |
|---|---|---|
| AC-F1 | 同じサイトは直列・前回の終わりから 5 秒あける(失敗も数える・初回は待たない・違うサイトは待たない)。待ち中の取り消しは inner を呼ばない。並列に呼ばれても重ならない | `TestThrottle_Interval`・`TestThrottle_FromEndAndFailures`・`TestThrottle_CanceledWhileWaiting`・`TestThrottle_NoParallelSameSite` |
| AC-F2 | 登録表:api は appid があるときだけ、scrape・headless・link_only は使えない。本番の表の Yahoo は Endpoint・Client の設定を使い、待ちで包まれている | `TestNewRegistry`・`TestNewRegistryWith`・`TestNewRegistry_YahooIsThrottled` |
| AC-F3 | Yahoo のリクエスト(appid・query・`sort=%2Bprice`・results=20 だけ)と応答の変換(画像の代替・不正な hit の除外・20 件まで)。エンドポイントは公式の URL | `TestYahooEndpoint`・`TestYahoo_Request`・`TestYahoo_Parse`・`TestYahoo_Limit` |
| AC-F4 | 失敗:2xx 以外 → `ErrUpstreamStatus`、2 MiB 超 → `netguard.ErrTooLarge`、壊れた JSON → エラー、既定クライアントでループバック → `netguard.ErrForbiddenAddress`、取り消し → `context.Canceled`。文言に appid を出さない。appid なし → 取得せず `ErrUnavailable` | `TestYahoo_Errors`・`TestYahoo_NoAppID` |

### internal/item — PriceRepository 契約 `internal/item/itemtest/price_contract.go`(メモリ `memory_test.go`、MySQL `mysql_test.go`(`-tags mysql`))

| ID | 条件 | テスト(`RunPriceRepositoryContract` のサブテスト) |
|---|---|---|
| AC-P1 | 保存した目安・出品を読める。出品は price 昇順・同額は id 昇順。出品の ID・ItemID・SiteID・FetchedAt は目安の値を使う。時刻は秒未満切り捨て・タイムゾーンに依らず同じ時点 | `SaveAndList` |
| AC-P2 | 同じ商品×サイトの保存は出品の置き換えと目安の upsert。他サイトは変えない。site_id の絞り込み。目安は site_id 昇順 | `ReplacePerSite` |
| AC-P3 | 0 件の保存で出品が空、目安は no_result | `SaveEmpty` |
| AC-P4 | MarkFailed は status だけ failed(前回値・出品を残す)。行が無ければ failed・null・0・fetched_at = at | `MarkFailed` |
| AC-P5 | 商品なし → `ErrNotFound`、サイトなし → `ErrSiteNotFound`、列幅超え・負の価格 → `ErrInvalid`。どれも何も変えない。読み出しは存在しない商品でも空 | `Errors` |
| AC-P6 | 商品の削除で目安・出品も消える | `DeleteItemCascades` |

### internal/refresh — `internal/refresh/refresh_test.go`(メモリ実装・偽の Fetcher・差し替えた時計)

| ID | 条件 | テスト |
|---|---|---|
| AC-U1 | 対象サイト(表示順・enabled=false と取得できないサイトを除く)を順に取り、検索ワードは query.Build の優先順位 | `TestRefreshItem_TargetsAndQuery` |
| AC-U2 | 仕様 §6 の例を保存(参考外も理由つき・fetched_at は今・0 件は no_result) | `TestRefreshItem_SavesSpecExample` |
| AC-U3 | 基準に使うのは is_reference かつ api / scrape のサイトだけ | `TestRefreshItem_ReferenceSites` |
| AC-U4 | 先頭 20 件まで。タイトル 512 文字に切り詰め、長すぎる URL の出品は捨て、長すぎる・空の image_url は null | `TestRefreshItem_Sanitize` |
| AC-U5 | 失敗は前回値を残し status だけ failed。他のサイトは保存。エラーにせず Report に数える | `TestRefreshItem_Failure` |
| AC-U6 | ModeStale:無い・failed・24 時間超のうち、最後に試して 1 時間以上のものだけ。ModeAll は全部 | `TestRefreshItem_StaleMode` |
| AC-U7 | 取得に成功しなかった基準サイトは保存済みの出品を基準に使う(保存し直さない) | `TestRefreshItem_BaseFromStoredListings` |
| AC-U8 | 同じ商品の更新は同時に 1 つ(取得を重ねない) | `TestRefreshItem_Singleflight` |
| AC-U9 | 存在しない商品は `item.ErrNotFound`(取得しない) | `TestNotFound` |
| AC-U10 | Estimates:キャッシュを即返し、取るべき対象があれば裏で更新して refreshing。Sites は表示順・対象のうち保存済みだけ・サマリはそこから。失敗が続くサイトは 1 時間取り直さない。対象が無ければ空と false | `TestEstimates`・`TestEstimates_FailureBackoff`・`TestEstimates_NoTargets` |
| AC-U11 | Refresh:新しくても全対象を裏で取り直す。実行中なら起動しない(refreshing は true)。対象が無ければ false | `TestRefresh`・`TestEstimates_NoTargets` |
| AC-U12 | Listings:保存済みの出品(参考外を含む)と site_id の絞り込み | `TestListings` |
| AC-U13 | RefreshAll:全商品を順に更新し 1 商品の失敗で止めない。失敗の数え方(全対象失敗・エラー。対象なしは数えない) | `TestRefreshAll` |

### internal/httpapi — `internal/httpapi/server_test.go`

| ID | 条件 | テスト |
|---|---|---|
| AC-H14(置き換え) | 取得できる対象が無い商品:estimates は空の sites・refreshing:false・サマリは null、refresh は 202・refreshing:false、listings は `{"listings":[]}` | `TestEstimates_NoTargets` |
| AC-H18 | GET estimates:初回は refreshing:true、取れた後は SiteEstimate(site_id・low・mid・count・suspicious_count・in_stock_count・status・fetched_at)とサマリ。24 時間超で再び refreshing | `TestEstimates_Flow` |
| AC-H19 | POST refresh:202・本文はキャッシュと refreshing:true。新しくても取り直す。不正な id は 400 | `TestRefreshEstimates` |
| AC-H20 | GET listings:参考外を含む・price 昇順・suspicious_reasons は `[]` か理由・image_url は null 可。site_id の絞り込み、存在しない site_id は空、0・数でないのは 400 | `TestListings` |
| AC-H11 | 存在しない商品は estimates・refresh・listings も 404(フェーズ1のまま) | `TestItemNotFound` |

フェーズ1の `TestEstimatesPhase1`(AC-H14:空の結果・refresh は 501)は削除ではなく、上の 4 つのテストに**置き換えた**(仕様どおりの振る舞いになったため)。

### cmd — `cmd/api/main_test.go`・`cmd/refresher/main_test.go`・`cmd/refresher/deploy_test.go`

| ID | 条件 | テスト |
|---|---|---|
| AC-C5 | serve は WISHLIST_YAHOO_APPID を任意で読む(前後の空白を除く) | `TestLoadServeConfig_YahooAppID` |
| AC-C6 | refresher の設定:DSN 必須(parseTime・multiStatements なし・不正な DSN の中身を出さない)、appid 任意 | `TestLoadConfig` |
| AC-C7 | 終了コード(商品があって全件失敗なら 1)と 1 行の結果出力 | `TestExitCode`・`TestPrintReport` |
| AC-C8 | DSN なし → 1(変数名を出す)、引数あり → 2 | `TestRun_Usage` |
| AC-K1 | Dockerfile が `./cmd/refresher` を `/wishlist-refresher` としてビルドする | `TestDockerfileBuildsRefresher` |
| AC-K2 | CronJob wishlist-refresher(03:00 JST・Forbid・api イメージ・同じ securityContext・Secret の DSN と optional の appid)が base に載る | `TestRefresherCronJob` |
| AC-K3 | api の Deployment に optional の appid | `TestAPIDeploymentYahooAppID` |

`make wishlist-kustomize`(`kubectl kustomize overlays/local`)が通ること。

## 未実装(サイト別の取得処理)

実サイトの URL・HTML 構造を確認してから、保存した HTML の fixture でテストを書いて実装する(仕様 §5・§13。推測で書かない)。
それまでは登録表に Fetcher が無く、取得しない(目安の行を作らない。リンクは出る)。

| サイト | fetch_type | 状態 |
|---|---|---|
| Yahoo!ショッピング | api | **実装する**(このフェーズ。公式 API) |
| カードラッシュ | scrape | TODO:検索 URL と結果 HTML を確認して fixture を保存 |
| ドラゴンスター | scrape | TODO:同上 |
| あみあみ | scrape | TODO:同上 |
| 駿河屋 | scrape | TODO:同上 |
| メルカリ | headless | TODO:chromedp の導入(イメージ・リソース。仕様 §11)と fixture |
| Yahoo!フリマ | headless | TODO:同上 |
| Amazon・プレバン・魂ウェブ・ポケモンセンターオンライン | link_only | 取得しない(仕様どおり) |

Yahoo!ショッピングのサイト(検索 URL)は seed に入れない(フェーズ1と同じく、人が確認して設定画面から `fetch_type: api`・`is_reference: true` で登録する)。

## 契約の変更(openapi.yaml)と DB

- `SiteEstimate.in_stock_count`(必須の integer)を追加:仕様 §3 の「状態(在庫あり等)」を表すため(design.md の未決事項の解消)。あわせて count・suspicious_count・summary_* に説明を足した
- `POST /api/items/{id}/estimates/refresh` の `501` と `components.responses.NotImplemented` を削除:実装したため(成功を装う 501 の明示はもう要らない。W-06 の役目を終えた)
- GET estimates・refresh・listings の説明をフェーズ3の振る舞いに更新(形は変えていない)
- migration `000003_estimates_in_stock_count`:`estimates.in_stock_count INT NOT NULL DEFAULT 0` を追加(down は列を消す)
