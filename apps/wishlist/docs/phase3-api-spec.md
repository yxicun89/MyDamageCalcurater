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
  price が 1 未満または上限超え・name か url が空の hit は除く。応答本文は 2 MiB まで。エラーの文言に appid を含めない(`*url.Error` は URL を含むため包み直す)
- **アクセスの節度**:同じ**ホスト**(検索 URL テンプレートのホスト名。小文字・ポートなし。読めなければ sites.id)へのリクエストは、サイト行が別でも直列にし、前回の取得が**終わってから** 5 秒あける(失敗した取得も数える。初回は待たない)。
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
- **CronJob**:`schedule: "0 3 * * *"`・`timeZone: Asia/Tokyo`・`concurrencyPolicy: Forbid`・`startingDeadlineSeconds: 43200`(Mac が 03:00 にスリープしていても、12 時間以内に起きればその日の分を 1 回実行する。600 秒では毎回飛ばされた。2026-10-09。このとき api の裏の更新を止める時間帯〈02:30〜04:00〉の外で動くことがあり、同じサイトへの 5 秒の間隔はプロセスをまたいでは守られない〈既知の制限〉)・Job の `activeDeadlineSeconds: 7200`、イメージは api(`wishlist/api`)、
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
| AC-F1 | 同じホストは直列・前回の終わりから 5 秒あける(失敗も数える・初回は待たない・違うホストは待たない。同じホストの別サイト行も 5 秒あく。ホストが読めなければ ID)。待ち中の取り消しは inner を呼ばない。並列に呼ばれても重ならない | `TestThrottle_Interval`・`TestThrottle_SameHostDifferentSites`・`TestThrottle_SameHostNoParallel`・`TestThrottle_UnparsableHostFallsBackToID`・`TestThrottle_HostMinIntervals`(駿河屋 30 秒)・`TestThrottle_FromEndAndFailures`・`TestThrottle_CanceledWhileWaiting`・`TestThrottle_NoParallelSameSite` |
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

## サイト別の scrape と確認済みサイトの初期データ

実サイトの確認結果は [sites.md](sites.md)(2026-10-03、手動)。セレクタ・JSON パス・robots.txt の注意はそこに書かれた事実だけを使う。
fixture は `internal/fetcher/testdata/{cardrush,amiami,surugaya,yahoofurima}.html`(構造だけを写した架空データ)。テストは実サイトに接続しない(httptest)。

### 決めたこと

- **取得の対応表**:カードラッシュ(`www.cardrush-dm.jp`)・あみあみ(`slist.amiami.jp`)・Yahoo!フリマ(`paypayfleamarket.yahoo.co.jp`)の 3 つを登録表に入れる。
  scrape は `fetch_type = scrape` かつ検索 URL テンプレートのホスト名(大文字小文字を区別しない・ポートは無視・完全一致)で選ぶ。
  headless・link_only はホストが対応済みでも取得しない。api は従来どおり fetch_type と appid だけで選ぶ
- **API**:`Registry.ForSite(site)` を足し、refresh は `For(fetch_type)` ではなく `ForSite` で対象を決める(`internal/refresh/refresh.go` の 1 行)。
  `For(fetch_type)` は従来どおり(scrape・headless は使えない)。`NewRegistryWith`(テスト用)の `ForSite` は fetch_type の表をそのまま使う
- **検索 URL**:`deeplink.Build(site.SearchURLTemplate, query)` で作った 1 回の GET。Yahoo!フリマだけは、作った URL の**クエリと # を外し**、
  `/search/{q}` のパラメータなしで取得する(robots.txt が sort などのパラメータ付き検索を禁じているため)
- **Yahoo!フリマの並び**:関連度順の上位 20 件を取ったあと、こちらで価格の昇順に並べる(安定ソート)。カードラッシュ・あみあみ・駿河屋はサイトの並びのまま先頭 20 件
- **Yahoo!フリマの JSON**:`script#__NEXT_DATA__` の `props.initialState.searchState.search.result.items[]`。items のパスが無い・JSON が壊れている・script が無いのは**エラー**(0 件と区別する。構造の変化に気づくため)。
  `items: []` は 0 件(エラーにしない)。商品 URL は `/item/{id}` を検索ページ基準で絶対化
- **カードラッシュ・あみあみ・駿河屋の 0 件**:HTML に出品の要素が無ければ 0 件(エラーにしない。レイアウト変更との区別はつかない)
- **除く出品**:価格が読めない・1 円未満・1 億円以上(上限 `MaxPrice` = 99,999,999 円。既定案。ParseYen・Yahoo!フリマの float・Yahoo!ショッピングの price で同じ。DB の price INT のあふれで更新全体が失敗しないため)・タイトルまたは URL が空(Yahoo!フリマは id も空)の出品は黙って除く
- **価格の表記**:`ParseYen` は NFKC で半角にし、最初に現れる数字(カンマ区切り可)を円とする(`50円`・`1,280円`・`8,080`・`￥500 税込`・`税込 1,234円`)。数字が無い・0 は読めない扱い
- **在庫(InStock)**:
  - カードラッシュ:`.stock` の「在庫数 N枚」が 0 なら false、それ以外(読めない・無いを含む)は true
  - あみあみ:一覧からは判定できない(sites.md)ので**常に true**
  - 駿河屋:一覧に出ている販売価格のある商品は true(新品が品切れで販売価格が無い商品は出品に含めない)
  - Yahoo!フリマ:`itemStatus` が `OPEN` のときだけ true(売り切れの値は未確認なので、OPEN 以外は false)
- **駿河屋の販売価格**:`.item_price .price_teika strong`(中古の販売価格)、無ければ `.item_price .price` が円として読めるもの。どちらも無い(定価だけ)商品は除く
- **URL・画像**:商品 URL・画像 URL は検索ページの URL を基準に絶対 URL にする。あみあみの画像は `data-src`(`src` は blank.gif)
- **外部取得**:既存の netguard クライアント(Client が nil なら既定。ループバックは `ErrForbiddenAddress`)。応答は `MaxResponseBytes` まで
- **駿河屋(ユーザー決定 2026-10-04: robots.txt の Crawl-delay 30 秒を守り、夜間の CronJob だけで取得)**:`NewSurugaya` を登録表に入れ、`ForSite` で `www.suruga-ya.jp` を選ぶ。
  ホストごとの最小間隔は `fetcher.HostMinIntervals`(既定 5 秒、`www.suruga-ya.jp` は 30 秒。`ThrottleWith`)。夜間だけの印は `fetcher.NightlyOnlyHosts`(`Registry.NightlyOnly`)。
  refresh は `ModeNightly`(`RefreshAll` = cmd/refresher の CronJob だけ)のときだけ夜間専用のサイトを取る。`ModeAll`(手動)・`ModeStale`(api の裏の更新)では取らずに飛ばし、前回値を残し、夜間専用だけの商品では裏の更新も起動しない。駿河屋は fetch_type=scrape・is_reference=true(ショップ)で初期データに入れる
- **依存**:`github.com/PuerkitoBio/goquery` を最新の安定版(2026-10-03 時点 v1.13.0)に完全固定して足し、go.sum も更新する(テストは goquery を import しない)
- **初期データ(migration 000004_seed_sites)**:sites.md で確認できた 5 サイトだけを足す(プレバン・魂ウェブ・ポケセンは未確認の点があるので入れず、sites.md に人が登録する候補として残す)。

  | サイト | fetch_type | is_reference | 検索 URL テンプレート |
  |---|---|---|---|
  | Yahoo!フリマ | scrape | false | `https://paypayfleamarket.yahoo.co.jp/search/{q}` |
  | カードラッシュ | scrape | true | `https://www.cardrush-dm.jp/product-list?keyword={q}&order=asc&available=1&num=20` |
  | あみあみ | scrape | true | `https://slist.amiami.jp/top/search/list?s_keywords={q}&s_sortkey=pricea` |
  | Yahoo!ショッピング | api | true | `https://shopping.yahoo.co.jp/search/{q}/0/?X=2`(人が開く URL。価格は公式 API) |
  | 駿河屋 | scrape | true | `https://www.suruga-ya.jp/search?category=&search_word={q}&rankBy=price%3Aascending&inStock=On` |

  ジャンルの表示順(既定案。メルカリ・Amazon は 000002 の行のまま、sort_order 10・20 を変えない。新しい行は各ジャンル内で重ならない sort_order にする):
  デュエマ = カードラッシュ → Yahoo!フリマ → メルカリ → Yahoo!ショッピング → Amazon /
  S.H.Figuarts・ガンプラ = あみあみ → 駿河屋 → Yahoo!フリマ → メルカリ → Yahoo!ショッピング → Amazon /
  ポケモングッズ = 駿河屋 → Yahoo!フリマ → メルカリ → Yahoo!ショッピング → Amazon
- **すでに使われている DB に流しても壊れない**:sites は id を明示せず、同じ名前があれば足さない(既存の URL・方式を変えない)。
  genre_sites は名前で引いて、無い組だけ足す(既存の行・sort_order を変えない。UPDATE・REPLACE・DELETE をしない)。ジャンルは足さない。
  down は 000004 で足した行だけを消す:サイトは**名前と URL の両方が一致するもの**とその紐づけだけ(ユーザーが同じ名前を別の URL で登録していた行は消さない)
- **確認済み URL の検査**:`migrations/layout_test.go` の確認済み一覧(`confirmedURLs`)を 7 件に広げ、000002 と 000004 の両方を検査する。一覧の各 URL は sites.md に書かれていること

### 受け入れ条件とテスト

`internal/fetcher/scrape_test.go`(サイト別のテーブルは `scrapeCases`。駿河屋を含む 4 つ)

| ID | 条件 | テスト |
|---|---|---|
| AC-S1 | テンプレートから deeplink.Build で作った 1 回の GET。fixture をタイトル・整数の円・絶対 URL・画像の Listing にする | `TestScrape_RequestAndParse` |
| AC-S2 | 先頭 20 件まで。Yahoo!フリマだけ上位 20 件を価格の昇順に並べ替える | `TestScrape_LimitAndOrder` |
| AC-S3 | 0 件のページはエラーにしない | `TestScrape_Empty` |
| AC-S4 | 2xx 以外 → `ErrUpstreamStatus`、2 MiB 超 → `netguard.ErrTooLarge`、既定クライアントでループバック → `ErrForbiddenAddress`、取り消し → `context.Canceled` | `TestScrape_Errors` |
| AC-S5 | 価格・タイトル・URL が読めない出品は除く | `TestScrape_SkipsBrokenEntries` |
| AC-S6 | 相対 URL(商品・画像)は検索ページ基準で絶対化 | `TestScrape_RelativeURLs` |
| AC-S7 | 在庫の判定(上の決めたこと) | `TestScrape_InStock` |
| AC-S8 | Yahoo!フリマはクエリ・# なしの `/search/{q}` だけを 1 回取得 | `TestYahooFurima_NoQueryParameters` |
| AC-S9 | Yahoo!フリマの埋め込み JSON:同額は元の順、構造の欠落・壊れた JSON はエラー | `TestYahooFurima_PageStructure` |
| AC-S10 | 価格表記を円にする(全角・円記号・税込・カンマ・読めない・0) | `TestParseYen` |
| AC-S11 | 登録表:scrape はホスト名で選ぶ(4 サイトだけ。駿河屋を含む。未対応・前方一致・読めないテンプレートは不可)。headless・link_only は不可、api は appid があるときだけ。`For(fetch_type)` は従来どおり、`NewRegistryWith` は型の表のまま | `TestRegistry_ForSite`・`TestRegistry_ForTypeUnchangedForScrape`・`TestRegistryWith_ForSite` |
| AC-S12 | 本番の表の scrape は Config.Client を使い、同じホストを Throttle で 5 秒あける(違うホストは待たない) | `TestRegistry_ScrapeUsesClientAndThrottle` |
| AC-S13 | refresh は ForSite で対象を決める(対応済みの scrape だけ取得し、他は取得も目安の行もなし) | `internal/refresh/refresh_scrape_test.go` の `TestRefreshItem_UsesForSite` |

`migrations/`(layout_test.go は MySQL 不要。seed_mysql_test.go は `-tags mysql`・`make wishlist-test-mysql`)

| ID | 条件 | テスト |
|---|---|---|
| AC-D0 | 初期データの URL は確認済みのものだけ(000002・000004)。確認済み一覧は sites.md と一致し、未確認のサイトを含まない。000004 の SQL の形(id 明示なし・名前で重複回避・up に UPDATE/DELETE なし・down の対象は 5 サイト) | `TestSeedSitesAreConfirmedOnly`・`TestConfirmedURLsMatchSitesDoc`・`TestSeedSites000004Shape` |
| AC-D1 | 空の DB:5 サイトが仕様どおり入り、各ジャンルの表示順が既定案。000002 の行は不変 | `TestSeed000004_Fresh` |
| AC-D2 | 2 回適用しない(Up は ErrNoChange)。SQL をもう一度流しても行が増えない | `TestSeed000004_NotAppliedTwice` |
| AC-D3 | 使われている DB:同名のサイトは足さず変えない。既存の genre_sites の行は不変。足りない分だけ増える | `TestSeed000004_ExistingDB` |
| AC-D4 | down:空の DB は 000002 の状態に戻る。ユーザーが登録した同名(別 URL)のサイトと紐づけは消さない | `TestSeed000004_DownFresh`・`TestSeed000004_DownKeepsUserRows` |

### 夜間専用(駿河屋)のテスト

`TestRefresh_NightlyOnlySite`(ModeAll・ModeStale・Refresh では取らない、RefreshAll で取る、前回値は残る)・`TestRegistry_SurugayaThrottleAndNightlyOnly`・`TestHostIntervalTable`

### 取得の安全側の決めごと(critic の推奨)

- **User-Agent**:scrape・Yahoo の取得は定数 `fetcher.UserAgent`(`wishlist-price-checker/0.1 (personal use; +https://github.com/)`。偽装せず目的が分かる短い文字列。リポジトリの URL・個人情報は入れない)を送る。UA は正直に名乗る。403 が返るサイトは取得せずリンクだけにする判断を人に仰ぐ(UA を偽って回避しない)。`TestUserAgent_Sent`
- **価格の上限**:`MaxPrice` = 99,999,999 円(1 億円以上は除外)。`TestParseYen_UpperBound`・`TestYahooFurima_PriceRange`・`TestYahoo_PriceRange`
- **応答本文の上限 2 MiB**:Yahoo!フリマの実ページが足りるかは未確認。初回の手動確認で ErrTooLarge が出たら上限を見直す(値は変えていない)

### 残りの TODO

| サイト | 状態 |
|---|---|
| ドラゴンスター | Cloudflare のチャレンジで**取得不可(回避しない)**。登録しない(ユーザー決定 2026-10-04。sites.md) |
| 駿河屋 | ユーザー決定 2026-10-04: 30 秒間隔で夜間のみ取得(robots.txt の Crawl-delay 30)。初期データは scrape・基準サイト |
| メルカリ | headless。次の節(AC-H*)。夜間のみ・refresher 専用イメージ(Chromium)だけ |
| プレバン・魂ウェブ・ポケセン | リンク用として 000006 で入れる(次の節 AC-D5。プレバンの検索語の文字コードは未確認と sites.md に明記) |
| Amazon | link_only(取得しない) |

## メルカリの headless 取得・リンク用サイト・refresher イメージ

確認結果の正は [sites-headless.md](sites-headless.md)(2026-10-04 の手動確認。書かれた事実だけを使い、推測で足さない)。fixture は `internal/fetcher/testdata/mercari.html`(架空データ)。
実サイト・実際のブラウザはテストから起動しない(偽のレンダラ・文字列検査)。

### 決めたこと

- **2 つに分ける**:描画(`fetcher.Renderer` → `fetcher.Page`。本番は `internal/chromium` の chromedp)と、パース(`fetcher.ParseMercari`。純粋・goquery)。`fetcher` は chromedp を import しない
- **Page の操作**:`Navigate` / `WaitVisible` / `CountVisible` / `ScrollToBottom` / `HTML` / `Close`。Renderer は `OpenPage(ctx)`。呼び出し側が必ず Close する
- **待つ要素**:`fetcher.MercariReadySelector` = `li[data-testid="item-cell"] a[data-testid="thumbnail-link"]`。`item-cell` だけはスケルトンにも付くので使わない
- **待つ手順**:Navigate → WaitVisible(期限 `MercariWaitTimeout` 10 秒)→ 数える → (20 未満なら)スクロール → `MercariScrollWait` 500ms 待つ(`Clock.Sleep`)→ 数え直す、を繰り返す。
  やめる条件は、数えた値が 20 以上 / 増えないスクロールが `MercariNoGrowthRounds`(3)回続いた(増えたら 0 に戻す)/ 開始からの経過(Clock)が `MercariScrollBudget` 10 秒に達した。やめたら HTML を取り、ページを閉じる。
  500ms ずつ待つので、予算で止まるのは 20 回のスクロールのとき
- **失敗**:WaitVisible の失敗(期限切れ含む)は `ErrNotRendered`(0 件と区別する。前回値を残す)。それ以外の失敗は原因を包んだ error。どの経路でも Close する。ctx の取り消しは `errors.Is(context.Canceled)`(開く前に取り消し済みでも)。描画はされたが 0 件のページはエラーにしない
- **パース**:`li[data-testid="item-cell"]` のうち `a[data-testid="thumbnail-link"]` を持つものだけ(持たないのはスケルトンとして捨てる)。URL = `href` を `MercariOrigin`(`https://jp.mercari.com`)基準で絶対化(`/item/m…`・`/shops/product/…`)。
  タイトル = 同リンク内 `img` の `alt` の**末尾**の「のサムネイル」を除く(空になる・alt が無いものは捨てる)。価格 = `[data-testid="item-tile-price"]` の text を `ParseYen`(`¥`・カンマを除く。1〜`MaxPrice` の範囲外・読めないものは捨てる)。
  画像 = `img` の `src`(絶対化。無ければ空)。在庫 = `InStock: true`(`status=on_sale` で絞っているため。売り切れ表示は未確認)。捨てたものは件数に数えず、HTML の順のまま先頭 20 件まで。class は使わない(ハッシュ化)。0 件でも nil でなく空のスライス
- **登録表**:`Config.Renderer` を足す。nil なら headless は登録しない(Chromium が無い環境)。あれば `jp.mercari.com`(fetch_type=headless のサイトだけ。ホストは小文字・ポート無視・完全一致)を `ThrottleWith` で包んで登録する。`NightlyOnlyHosts` に `jp.mercari.com` を足す(Renderer の有無によらず印は付く)。`HostMinIntervals` には足さない(既定の 5 秒)。ドラゴンスターは登録しない
- **refresher の設定**:`WISHLIST_CHROMIUM_PATH`(前後の空白を除く)。空なら Renderer を渡さず headless は取らない(エラーにしない)。あれば `chromium.New(path)` を渡す。`newRegistry(cfg)` が組み立てる。API 側(`cmd/api`)は Renderer を渡さない(Chromium は refresher 専用)
- **chromium パッケージ**:`New(execPath)` は作るだけで起動しない。`OpenPage` で ExecAllocator(headless-shell を起動)する。起動できなければ `chromium.ErrLaunch` を包む。UA は既定のまま(偽装しない。sites-headless.md)。chromedp は最新の安定版で完全固定(go.mod・go.sum)
- **既定案の補足(実装者が実イメージで確かめる)**:headless-shell の実行ファイルのパス(CronJob の `WISHLIST_CHROMIUM_PATH` に書く)、`--no-sandbox` の要否(非 root・seccomp RuntimeDefault・capabilities drop ALL で動くか)、user-data-dir を書ける場所(読み取り専用ルートなので `/tmp` の emptyDir。64Mi で足りるか)は sites-headless.md に書かれていない。docker 上で確かめて Dockerfile・CronJob に反映し、決めたことをこの節に追記する
- **refresher イメージ**:`api/Dockerfile.refresher`(multi-stage)。実行段は `chromedp/headless-shell` の最新の安定版(sites-headless.md の確認は 151.0.7922.109。実装時の最新を確認して決める)を `タグ@sha256:` で固定。ビルド段の golang は api の Dockerfile と同じ固定。`/wishlist-refresher` だけを入れ、USER 65532。
  api のイメージには Chromium を入れない。CronJob は `wishlist/refresher:0.1.0` を使い、`/dev/shm` に emptyDir(`medium: Memory`・sizeLimit あり)、メモリは requests 256Mi・limits 1Gi(既定案。共有メモリは limits に含まれる)。`docker build` が通ることは実装者が手元で確かめる
- **実イメージでの確認結果(2026-10-04。実装者。実サイトには繋いでいない)**:`docker build -f api/Dockerfile.refresher` と api の `docker build` は通った。`chromedp/headless-shell:151.0.7922.109`(latest・stable と同じ。arm64 で確認)を、CronJob と同じ制限(非 root 65532・読み取り専用ルート・cap drop ALL・no-new-privileges・docker 既定の seccomp)で起動し about:blank と data: URL を開いた。
  - 実行ファイル:`/headless-shell/headless-shell`(`WISHLIST_CHROMIUM_PATH`)。イメージの ENTRYPOINT は `/headless-shell/run.sh`(root 起動)なので使わず、refresher を ENTRYPOINT にした
  - `--no-sandbox`:**要る**。付けないと "No usable sandbox!" で起動に失敗した。`chromium.Options{NoSandbox: true}`(refresher の `chromiumNoSandbox`)。コンテナが境界で、開くのは夜間のこの Pod だけ。NetworkPolicy は Ingress しか制限しておらず、**Egress は未制限**(事実)
  - user-data-dir:`os.MkdirTemp`(`/tmp/wishlist-chromium-*`)。ページごとに作り、Close で消す。`/tmp` は emptyDir。8Mi の tmpfs でも about:blank は動いた。実ページ(キャッシュ)の使用量は未測定なので、64Mi から 128Mi に広げた(足りなければ上げる)
  - `/dev/shm`:emptyDir(Memory・256Mi)。about:blank は 64Mi の tmpfs でも動いた。実ページでの必要量は未測定(Chromium の既定は 64Mi で足りなくなりやすいため 256Mi)
  - GOMEMLIMIT:100MiB → 128MiB(Go のヒープだけの目安。Chromium は別プロセスで、コンテナ全体は limits 1Gi)
  - 将来の提案(共通基盤の変更なので未実施):refresher の Egress を DB・外部 HTTPS・DNS に絞る NetworkPolicy
  - CronJob のイメージは他と同じくタグ(`wishlist/refresher:0.1.0`)で固定。cloud の overlay で digest に固定するかは後で決める
  - 設定なしで `docker run --rm wishlist/refresher:local` は WISHLIST_DATABASE_DSN の設定エラーで終了コード 1
  - 注意:`chromedp.Run` の最初の ctx がブラウザの寿命になるため、起動の期限は子 ctx ではなくタイマーで取り消している(子 ctx にするとブラウザが即終了した)
- **Makefile**:`WISHLIST_REFRESHER_IMAGE ?= wishlist/refresher:local`。`wishlist-docker-build` が `-f Dockerfile.refresher` で build、`wishlist-k3d-deploy` の `k3d image import` に含める。overlays/local の images に `wishlist/refresher`(newTag: local)
- **000006_seed_link_sites**(000005 は別の作業が使う。000004 と同じ作り方:名前で冪等・id を書かない・既存の行を変えない・down は名前と URL の両方が一致する行だけ):

  | サイト | fetch_type | is_reference | URL | 紐づけ(sort_order) |
  |---|---|---|---|---|
  | 魂ウェブ | link_only | false | `https://tamashiiweb.com/item/?wo={q}` | S.H.Figuarts 30 |
  | ポケモンセンターオンライン | link_only | false | `https://www.pokemoncenter-online.com/search/?q={q}` | ポケモングッズ 25 |
  | プレバン | link_only | false | `https://p-bandai.jp/search_bst/?q={q}` | S.H.Figuarts 25・ガンプラ 25 |

  リンク用は各ジャンルの末尾(Amazon 20 の後)。プレバンの robots は機械取得の話で、人がブラウザで開くリンクとして出すのは問題ない。検索語の文字コードが未確認であることを sites.md に書く(書いた)。
  `layout_test.go` の確認済み一覧(`confirmedURLs`)を 10 件に広げ、000006 も検査する(検査は弱めない)。ポケセンの旧 URL を確認済みに入れない検査は残し、プレバン・魂ウェブを「未確認だから入れない」一覧から外した(今回、リンク用として入れる決定のため)
- **000005 との順序**:`TestLayout`(版番号が 1 から欠けない)は、000005 が無い間は 000006 を足すと失敗する。このブランチ単独では緑にならないので、000005 を持つブランチの後に取り込む(または rebase)。TestLayout は弱めない

### 受け入れ条件とテスト

`internal/fetcher/mercari_test.go`・`mercari_registry_test.go`

| ID | 条件 | テスト |
|---|---|---|
| AC-H1 | fixture(個人出品・Shops・スケルトン)を Listing にする(タイトル・円・絶対 URL・画像・在庫あり) | `TestParseMercari_Fixture` |
| AC-H2 | セルごとの読み方(スケルトン・末尾の「のサムネイル」・相対/絶対 URL・価格 0・上限・読めない・alt なし・item-cell でない li) | `TestParseMercari_Cells` |
| AC-H3 | 先頭 20 件まで(捨てたセルは数えない。HTML の順) | `TestParseMercari_Limit` |
| AC-H4 | 手順:開く → 移動(deeplink の URL)→ 待つ(期限 10 秒以内)→ 数える → HTML → 閉じる。20 ならスクロールしない。セレクタは MercariReadySelector | `TestMercari_Procedure` |
| AC-H5 | 待つ条件:20 に届く / 増えず 3 回 / 増えたら数え直す / 予算 10 秒(20 回のスクロール)でやめる。1 回ずつ 500ms 待つ | `TestMercari_ScrollLoop` |
| AC-H6 | 失敗の扱い(原因を包む・待っても出ない→ErrNotRendered・どの経路でも Close・取り消し→context.Canceled・0 件はエラーにしない) | `TestMercari_Errors` |
| AC-H7 | 登録表:Renderer ありで headless かつ jp.mercari.com だけ取得できる。Renderer なし・別の型・別のホスト・読めないテンプレートは不可。`For(headless)` は不可のまま | `TestRegistry_MercariNeedsRenderer` |
| AC-H8 | NightlyOnly(Renderer の有無によらず)。ホスト間隔は既定 5 秒で、本番の表は Throttle で包む | `TestRegistry_MercariNightlyOnlyAndThrottle` |
| AC-H9 | refresh:メルカリは夜間(RefreshAll)だけでブラウザを開く。ModeAll・ModeStale・手動では開かない。Renderer が無ければ夜間でも取らず失敗にもしない | `internal/refresh/refresh_headless_test.go` の `TestRefresh_MercariNightlyOnly`・`TestRefresh_MercariWithoutRenderer` |
| AC-H10 | refresher の設定:`WISHLIST_CHROMIUM_PATH` を読む(空白を除く・無ければ空)。あるときだけメルカリを取得でき夜間専用。作るだけでは起動しない | `cmd/refresher/headless_test.go` の `TestLoadConfig_ChromiumPath`・`TestNewRegistry_Headless` |
| AC-H11 | chromium:New は起動しない。起動できなければ ErrLaunch を包む。取り消し済みなら context.Canceled | `internal/chromium/chromium_test.go` の `TestNew_DoesNotLaunch`・`TestOpenPage_MissingBinary`・`TestOpenPage_Canceled` |
| AC-H12 | ドラゴンスターは取得しない(登録表に無い=AC-H7)。sites.md に「取得不可(回避しない)」、プレバンの文字コード未確認を書く | `TestRegistry_MercariNeedsRenderer`・`migrations/seed_link_sites_test.go` の `TestSitesDocNotes` |

`cmd/refresher/deploy_headless_test.go`(既存 `deploy_test.go` の `TestRefresherCronJob` は image を `wishlist/refresher:` に変更した。ほかの検査は同じ)

| ID | 条件 | テスト |
|---|---|---|
| AC-K4 | `Dockerfile.refresher`:headless-shell はタグ+ダイジェスト固定、golang もダイジェスト固定、refresher だけ、非 root | `TestDockerfileRefresherImage` |
| AC-K5 | api の Dockerfile に Chromium を載せない(コメント行は除く) | `TestAPIDockerfileHasNoChromium` |
| AC-K6 | CronJob:refresher イメージ・WISHLIST_CHROMIUM_PATH・/dev/shm の emptyDir(Memory・sizeLimit)・メモリ 256Mi/1Gi | `TestRefresherCronJobHeadless`・`TestRefresherCronJob` |
| AC-K7 | overlays/local と Makefile が refresher イメージを扱う(WISHLIST_REFRESHER_IMAGE・`-f Dockerfile.refresher`・k3d image import) | `TestRefresherImageWiring` |

`migrations/`(`seed_link_sites_test.go` は MySQL 不要。`seed_link_sites_mysql_test.go` は `-tags mysql`・`make wishlist-test-mysql`)

| ID | 条件 | テスト |
|---|---|---|
| AC-D5 | 000006 の SQL の形(id 明示なし・名前で重複回避・up に UPDATE/DELETE なし・down は名前と URL で特定・3 サイトとも link_only・is_reference=false)、確認済み URL の検査に 000006 を含める | `TestSeedLinkSites000006Shape`・`TestSeedSitesAreConfirmedOnly`・`TestConfirmedURLsMatchSitesDoc` |
| AC-D5 | 実 DB:空の DB で 3 サイトと紐づけが既定どおり、2 回適用しない、使われている DB(同名の魂ウェブ)を変えない、down はユーザーの同名行を消さない | `TestSeed000006_Fresh`・`TestSeed000006_NotAppliedTwice`・`TestSeed000006_ExistingDB`・`TestSeed000006_Down` |

### 実装者への注意(この節の範囲)

- スタブ:`internal/fetcher/mercari.go`(型・定数・`NewMercari`・`ParseMercari`)、`internal/chromium/chromium.go`、`cmd/refresher/main.go` の `config.ChromiumPath`・`newRegistry`、`Config.Renderer`。置き換える。
- `NewMercari(r, clock)` の clock が nil なら `SystemClock`。待ちは必ず `clock.Sleep(ctx, …)`、経過は `clock.Now()`(テストは偽の時計)。
- `Fetch` は最初に ctx を確かめる。ブラウザ・URL を error の文言に含めなくてよい。
- 検索 URL は `deeplink.Build(site.SearchURLTemplate, query)`。

## 契約の変更(openapi.yaml)と DB

- `SiteEstimate.in_stock_count`(必須の integer)を追加:仕様 §3 の「状態(在庫あり等)」を表すため(design.md の未決事項の解消)。あわせて count・suspicious_count・summary_* に説明を足した
- `POST /api/items/{id}/estimates/refresh` の `501` と `components.responses.NotImplemented` を削除:実装したため(成功を装う 501 の明示はもう要らない。W-06 の役目を終えた)
- GET estimates・refresh・listings の説明をフェーズ3の振る舞いに更新(形は変えていない)
- migration `000003_estimates_in_stock_count`:`estimates.in_stock_count INT NOT NULL DEFAULT 0` を追加(down は列を消す)
