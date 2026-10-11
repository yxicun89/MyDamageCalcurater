# wishlist API のヘキサゴナル化(ポートとアダプタ)の受け入れ条件

- 決定: ユーザー(2026-10-12)。wishlist の Go API(`apps/wishlist/api`)を、プロジェクト共通の形にそろえる。
- 規則の正: [../../../docs/plan/architecture-adoption.md](../../../docs/plan/architecture-adoption.md) §0・§1・§3.1・§5.1。
  §8 の Q7・Q8 は推奨どおりにする(層のフォルダに `doc.go` を置く。oapi-codegen の生成物 `internal/api` は今の場所のまま、外側として扱う)。
- 現状の確認と 4 つの崩れ: [architecture-check.md](architecture-check.md)。
- 機械検査: `api/internal/archtest/arch_test.go`(import 文だけを読む。ビルドも生成も要らない)。

## 決めたこと

- フォルダは `internal/domain`(純粋な核)・`internal/app`(ユースケース。ポートをここで宣言する)・`internal/adapters/<名前>`(外側)の 3 つ。
- 依存は **adapters → app → domain の一方向だけ**。内側は外側を import しない。`cmd/*` は組み立て役で、何を import してもよい。
- **動作は一切変えない**。API・OpenAPI・DB・migration・HTTP の応答(ステータス・メッセージ)・ログの内容を変えない。
- 動かす量は、向きの規則が成り立ち、4 つの崩れが直る最小にする。識別子の名前は変えない(変わるのは修飾するパッケージ名だけ)。
  例外は、パッケージをまたぐようになったために公開が要る関数だけ(下の「公開にするもの」)。
- テストはコードと一緒に動かす。消さない・弱めない。テストで変えてよいのは package 行・import・修飾するパッケージ名・fixture への相対パスだけ。

## 受け入れ条件

機械で確かめるもの(`api/internal/archtest`):

- **AC-A1** `internal/domain/doc.go`・`internal/app/doc.go`・`internal/adapters/doc.go` がある。中身はその層の役割(何を置くか・何を import してよいか)を書いたコメントと `package domain` / `package app` / `package adapters` だけ。
- **AC-A2** `internal/` の直下にあるディレクトリは `adapters`・`api`・`app`・`archtest`・`domain`・`testimg` だけ。旧パッケージ(`item`・`refresh`・`fetcher`・`query` ほか 14 個)は残らない。
- **AC-A3** 下の「置き場所の表」の移動先のパッケージがすべてあり、それぞれ本番の `.go` を 1 つ以上持つ。
- **AC-A4** import の向き(モジュール内の import だけを見る。生成物 `Code generated ... DO NOT EDIT.` のファイルは見ない):

  | import する側 | 本番コードが import してよいもの | `_test.go` が import してよいもの |
  |---|---|---|
  | `internal/domain/**` | domain | domain・`internal/testimg` |
  | `internal/app/**` | domain・app | domain・app・adapters・`internal/testimg` |
  | `internal/adapters/**` | domain・app・adapters・`internal/api`・`migrations` | 同じ + `internal/testimg` |
  | `cmd/**` | すべて | すべて |
  | `migrations` | すべて(cmd を除く) | すべて(cmd を除く) |

  - 誰も `cmd/**`・旧パッケージ・`internal/archtest` を import しない。`internal/testimg` を import してよいのは `_test.go` だけ。
  - app のテストがアダプタを使ってよいのは、今のテストが実物の部品(メモリのリポジトリ・ファイルの画像置き場・fixture を読む fetcher)を組み合わせて手順を確かめているため。
    そのテストは外部テストパッケージ(`package xxx_test`)のままにする(循環 import を避ける)。
- **AC-A5** domain と app の本番コードは、外側のライブラリを import しない:
  `database/sql`・`net/http`・`os`・`github.com/labstack/echo`・`github.com/go-sql-driver/mysql`・`github.com/golang-migrate/*`・`github.com/chromedp/*`・
  `github.com/PuerkitoBio/goquery`・`github.com/google/uuid`・`golang.org/x/net/html`・`github.com/oapi-codegen/runtime`(それぞれの下のパッケージも)。
  `github.com/oapi-codegen/nullable`(`ItemPatch` の「省略と null の区別」に使う値の型)と `log/slog` は使ってよい。
- **AC-A6** from-url の手順は `internal/app/fromurl` にあり、`domain/page` を使う。`internal/adapters/httpapi` の本番コードは `internal/adapters/ogp` を import しない。

人と CI で確かめるもの:

- **AC-A7** 動作が変わらない。`make wishlist-test`・`make wishlist-lint`・`make wishlist-build` が成功し、`make wishlist-test-mysql` も成功する。
  `api/openapi.yaml`・`api/internal/api/openapi.gen.go`・`api/migrations/`・`web/`・`ios/`・`deploy/` は差分なし。
- **AC-A8** テストが減らない。`func Test` の数は移動前と同じ(archtest を除いて 271 個。うち `-tags mysql` が 27 個)。
  テストの差分は package 行・import・修飾するパッケージ名・fixture の相対パスだけ。期待値・条件・`t.Skip` を変えない。
- **AC-A9** `make wishlist-gen` が新しい場所に生成し、生成し直しても差分が出ない(sqlc の生成物は package 名が `sqlcgen` になる以外は今と同じ)。
- **AC-A10** 文書の現状の記述を直す: `docs/design.md` のパッケージ一覧(§構成の表)と `docs/sites.md` の fixture のパス、`architecture-check.md` の結論。
  `phase*-spec.md` は当時の記録なので、パスは直さない。

## 置き場所の表

パスは `apps/wishlist/api/` から。「そのまま」は、ファイルの中身を変えずに場所(と import)だけ変えること。

### domain(純粋な核)

| 今 | 移動先 | 中身・注意 |
|---|---|---|
| `internal/query/*` | `internal/domain/query` | そのまま |
| `internal/deeplink/*` | `internal/domain/deeplink` | そのまま |
| `internal/estimate/*` | `internal/domain/estimate` | そのまま |
| `internal/item/item.go` | `internal/domain/item/item.go` | `Repository` インターフェースを除く(app へ)。`ErrNotFound` などのエラー、`Genre`・`Site`・`Item`・`New*`・`*Patch`・`FetchType`・`GenreIcon` |
| `internal/item/price.go` | `internal/domain/item/price.go` | `PriceRepository` を除く(app へ)。`Listing`・`Estimate`・`EstimateStatus`・`MaxListingTitleLen`・`MaxListingURLLen` |
| `internal/item/official.go` | `internal/domain/item/official.go` | `OfficialRepository` を除く(app へ)。`OfficialState`・`OfficialCheck`・`OfficialStatus`・`MergeOfficial`・`WatchTarget`・`ErrSourceChanged` |
| `internal/item/history.go` | `internal/domain/item/history.go` | `PriceHistoryRepository` を除く(app へ)。`PricePoint`・`HistoryDay`・`PriceHistoryRetentionDays` |
| `internal/item/summary.go`・`guess.go`・`check.go` | `internal/domain/item/` | そのまま(公開にするものは下) |
| `internal/item/service.go` の `invalid` | `internal/domain/item/` | `item.go`・`official.go` が使うので domain へ(公開する) |
| `fetcher.Listing`(取得した 1 件) | `internal/domain/item/` の `FetchedListing` | 中身は同じ。`fetcher` は `type Listing = item.FetchedListing` を持つ(下) |
| `official.Result` | `internal/domain/item/` の `OfficialResult` | 中身は同じ(`State`・`Evidence`)。`official` は `type Result = item.OfficialResult` を持つ |
| `fetcher.MaxListings`・`MinInterval`・`HostMinIntervals`・`NightlyOnlyHosts`・`mercariHost`・`siteHost` | `internal/domain/crawl`(新規) | アクセスの節度の規則(仕様 §6)。`MaxListings`・`MinInterval`・`HostMinIntervals`・`NightlyOnlyHosts`・`MercariHost`・`SiteHost(item.Site) string`・`NightlyOnly(item.Site) bool`。値は今と同じ |
| `ogp/clean.go`(`CleanTitle`・`StaticDraft`)と `ogp.Draft`・`collapse` | `internal/domain/page`(新規) | 商品ページの URL と題名の規則。`ogp` は `type Draft = page.Draft` を持ち、`collapse` を `page` から使う(重複させない) |
| `netguard.ValidateURL`・`netguard.ErrInvalidURL` | `internal/domain/page` | 純粋な URL の検査。エラー文(`netguard: invalid or unsupported url`)は変えない。`netguard` は `var ErrInvalidURL = page.ErrInvalidURL` と、`page.ValidateURL` を呼ぶだけの `ValidateURL` を残す(netguard・ogp・fetcher のテストを変えないため) |

テスト: `item/guess_test.go`・`history_test.go`・`official_test.go`・`summary_test.go` は `internal/domain/item` へ。
`query`・`deeplink`・`estimate` のテストはそれぞれの移動先へ。`ogp/clean_test.go` は `Parse` を使うので `internal/adapters/ogp` に残し、`CleanTitle` を `page.CleanTitle` と書く。

### app(ユースケース。ポートを宣言する)

| 今 | 移動先 | 中身・注意 |
|---|---|---|
| `internal/item/service.go` | `internal/app/items/service.go`(package `items`) | `Service`・`NewService`・`ValidateNewItem`・`Max*Len`・`MinAliasNormalizedLen`・`check*`。`storage.Storage` の代わりにポート `ImageStore` を受け取る |
| `item.Repository`・`PriceRepository`・`OfficialRepository`・`PriceHistoryRepository` | `internal/app/items/ports.go` | 永続化のポート。中身(メソッド)は今と同じ |
| (新規)`items.ImageStore` | `internal/app/items/ports.go` | `Save(ctx, io.Reader) (string, error)`・`Delete(name string) error`。`*storage.FileStorage` が満たす |
| `internal/item/itemtest/*` | `internal/app/items/itemtest` | リポジトリの契約テスト(ポートの約束)。そのまま |
| `item/service_test.go`・`service_alias_test.go`・`service_official_test.go`・`genre_style_test.go` | `internal/app/items`(package `items_test`) | `newEnv` を使うテスト。メモリのリポジトリは `adapters/memory`、画像は `adapters/storage` を使う |
| `internal/refresh/*` | `internal/app/refresh` | `fetcher`・`official` を import しない。下のポートを使う |
| (新規)`refresh.Fetcher` | `internal/app/refresh` | `Fetch(ctx, site item.Site, query string) ([]item.FetchedListing, error)`。`fetcher` は `type Fetcher = refresh.Fetcher` を持つ |
| (新規)`refresh.FetcherRegistry` | `internal/app/refresh` | `ForSite(site item.Site) (Fetcher, bool)`。`Deps.Fetchers` の型をこれにする。`*fetcher.Registry` が満たす |
| `refresh.OfficialChecker` | `internal/app/refresh`(今のまま) | 戻り値を `item.OfficialResult` にする(`official.Result` は別名なので同じ型) |
| `s.d.Fetchers.NightlyOnly(st)` | `crawl.NightlyOnly(st)` | 夜間だけのサイトは domain の規則で決める(`Registry.NightlyOnly` も同じ関数を呼ぶので結果は同じ) |
| `fetcher.MaxListings`(`sanitize`) | `crawl.MaxListings` | |
| `httpapi/handlers.go` の `DraftItemFromURL` の手順 | `internal/app/fromurl`(新規) | 下の「from-url」 |

### adapters(外側)

| 今 | 移動先 | 中身・注意 |
|---|---|---|
| `internal/httpapi/*` | `internal/adapters/httpapi` | from-url は app を呼んで変換するだけにする。`Deps` は変えない(`NewServer` が `Deps.Remote` と `Deps.Items` から `fromurl.Service` を作る。テストの組み立てを変えないため) |
| `internal/item/mysql*.go`(テストを含む) | `internal/adapters/store`(package `store`) | `MySQLRepository`・`NewMySQLRepository`。`Genre.withStyle` はメソッドにできなくなるので同じファイル内の関数にする |
| `internal/store/*`(sqlc の生成物・`query.sql`・`sqlc.yaml`) | `internal/adapters/store/sqlcgen`(package `sqlcgen`) | `sqlc.yaml` の `schema` を `../../../../migrations`、`package` を `sqlcgen` にする。`mysql*.go` の `store.` は `sqlcgen.` になる |
| `internal/item/memory*.go`(テストを含む) | `internal/adapters/memory`(package `memory`) | `MemoryRepository`・`NewMemoryRepository`。テストは package `memory_test` |
| `internal/fetcher/*`(`testdata/` を含む) | `internal/adapters/fetcher` | `Site`・`Listing`・`Fetcher` は別名(`item.Site`・`item.FetchedListing`・`refresh.Fetcher`)。`UserAgent`・`MaxResponseBytes`・`HostGate`・`Registry`・`Throttle*` は残す。規則の値は `crawl` から読む |
| (規則の値の参照) | `crawl.X` | `fetcher` は `MaxListings`・`MinInterval`・`HostMinIntervals`・`NightlyOnlyHosts` を再公開しない(値の置き場所を 1 つにする)。`fetcher`・`official`・`refresh` のテストの `fetcher.MaxListings` などは `crawl.MaxListings` と書く(パッケージ名だけの変更) |
| `internal/chromium/*` | `internal/adapters/chromium` | そのまま |
| `internal/official/*`(`testdata/` を含む) | `internal/adapters/official` | `type Result = item.OfficialResult`。判定(`Judge`・`Terms`)は HTML を読むので今回は動かさない |
| `internal/ogp/*`(`testdata/` を含む) | `internal/adapters/ogp` | `Parse`・`Fetcher`。`clean.go` は `domain/page` へ |
| `internal/storage/*` | `internal/adapters/storage` | そのまま。`Storage`(Open を含む)は httpapi が使うので残す |
| `internal/netguard/*` | `internal/adapters/netguard` | `ValidateURL`・`ErrInvalidURL` は `page` に委ねる(上) |
| `internal/dbwait/*` | `internal/adapters/dbwait` | そのまま |

### そのまま残すもの

| 場所 | 理由 |
|---|---|
| `internal/api`(oapi-codegen の生成物・`cfg.yaml`) | §8 Q8 の推奨。外側として扱う。`cfg.yaml` の出力先と Makefile の該当行は変えない |
| `internal/testimg` | テスト用の画像。層に属さない。`_test.go` からだけ使う(AC-A4) |
| `cmd/api`・`cmd/refresher` | 組み立て役。import パスだけ直す。`deploy_test.go`・`deploy_headless_test.go` の相対パスは cmd が動かないので変えない |
| `migrations` | 置き場所は変えない。`aliases_mysql_test.go` の import(`internal/query` → `internal/domain/query`)だけ直す |

## from-url(`internal/app/fromurl`)

今 `httpapi/handlers.go` の `DraftItemFromURL` にある手順を、そのままの順で移す。

- ポート: `PageReader`(`Draft(ctx, rawURL string) (page.Draft, error)`。`*ogp.Fetcher` が満たす)と `GenreLister`(`ListGenres(ctx) ([]item.Genre, error)`。`*items.Service` が満たす)。
- 入力: URL・ジャンル ID(任意)・共有拡張が読んだ題名と画像 URL(任意)。出力: 名前・`source_url`・画像 URL(任意)・ジャンル ID(任意)・推定したか。
- 手順(今と同じ): `page.ValidateURL` → 題名があれば `page.CleanTitle`(画像 URL は検査に通ったときだけ採用)/ `page.StaticDraft` に当たればそれ / それ以外は `PageReader.Draft` して `page.CleanTitle` → ジャンル ID が無ければ `ListGenres` と `item.GuessGenre`。
- エラー: 入力の URL の違反は `page.ErrInvalidURL` を包んで返す。`PageReader` の失敗は、それと区別できる型(例 `*fromurl.FetchError`。`Unwrap` で元のエラー)で包む。
  httpapi は今と同じ応答にする: 入力の URL の違反 → 422 `url must be an absolute http(s) URL`、取得の失敗 → 今の `remoteError(err, "url", 502)`、それ以外 → 今と同じ `handleError`。
- `url` が空(400 `url is required`)と `genre_id` の形式(400)は HTTP の入力の形なので httpapi に残す。
- 既存の from-url のテスト(`server_test.go`・`server_phase5_test.go`)が変更なしで通ること。app/fromurl 自体の単体テストを足すのは任意(足すなら同じ表のケースで)。

## 公開にするもの(domain/item)

パッケージをまたぐようになるため公開が要る。名前は先頭を大文字にするだけ。中身は変えない。

| 今 | 公開名 | 使う側 |
|---|---|---|
| `validateSave` | `ValidateSave` | memory・store |
| `validateOfficialCheck` | `ValidateOfficialCheck` | memory・store |
| `storedTime` | `StoredTime` | memory・store |
| `checkUniqueSiteIDs` | `CheckUniqueSiteIDs` | memory・store |
| `checkUniqueOverrides` | `CheckUniqueOverrides` | memory・store・app/items |
| `checkAliasDuplicates` | `CheckAliasDuplicates` | memory・store・app/items |
| `cloneOverrides` | `CloneOverrides` | memory |
| `cloneAliases` | `CloneAliases` | memory |
| `applyNullable` | `ApplyNullable` | memory |
| `invalid`(service.go から) | `Invalid` | domain/item・app/items |

## 直す相対パス

| ファイル(移動後) | 今 | 移動後 |
|---|---|---|
| `internal/domain/query/query_test.go` | `filepath.Join("..", "..", "..", "testdata", "query-cases.json")` | `filepath.Join("..", "..", "..", "..", "testdata", "query-cases.json")` |
| `internal/domain/deeplink/deeplink_test.go` | 同上 | 同上 |
| `internal/app/refresh/refresh_scrape_test.go` | `"../fetcher/testdata/cardrush.html"`・`"../fetcher/testdata/surugaya.html"` | `"../../adapters/fetcher/testdata/…"` |
| `internal/app/refresh/refresh_headless_test.go` | `"../fetcher/testdata/mercari.html"` | `"../../adapters/fetcher/testdata/mercari.html"` |
| `internal/adapters/store/sqlcgen/sqlc.yaml` | `schema: "../../migrations"` | `schema: "../../../../migrations"` |
| `Makefile`(`apps/wishlist/Makefile`) の `wishlist-gen` | `internal/store/sqlc.yaml`・echo の文 | `internal/adapters/store/sqlcgen/sqlc.yaml` |

- `fetcher`・`official`・`ogp` の `testdata/` はパッケージと一緒に動くので、`"testdata/…"` の参照は変えない。
- `cmd/refresher/deploy_test.go`・`deploy_headless_test.go`(`../../Dockerfile`・`../../../deploy/k8s/…`・`../../../Makefile`)は cmd が動かないので変えない。
- `Dockerfile`・`Dockerfile.refresher` は `COPY . .` と `./cmd/...` のビルドだけなので変えない。
- `scripts/test-mysql.sh` は `./...` なので変えない。
- `apps/wishlist` の外(ルートの Makefile・CI・`.gitignore`)は wishlist の内部パスを参照していない(2026-10-12 に確認)。変更は要らない。

## 実装の順番(案)

1 つずつ動かし、そのたびに `cd apps/wishlist/api && GOWORK=off go build ./... && GOWORK=off go vet ./...` を通す。

1. 層のフォルダと `doc.go` を作る
2. `query`・`deeplink`・`estimate` を domain へ(`git mv`)
3. `domain/item`・`domain/crawl`・`domain/page` を作り、型と規則を移す(上の表)
4. `app/items`(service・ports・itemtest)、`adapters/memory`・`adapters/store`(+ `sqlcgen`)へ分ける。`make wishlist-gen` で生成し直して差分を見る
5. `fetcher`・`chromium`・`official`・`ogp`・`storage`・`netguard`・`dbwait` を adapters へ。別名(`Site`・`Listing`・`Fetcher`・`Result`・`Draft`)を置く
6. `app/refresh` をポートに切り替える
7. `app/fromurl` を作り、`httpapi` の from-url をそれを呼ぶ形にする
8. `cmd` の import を直す。`make wishlist-test`・`make wishlist-lint`・`make wishlist-build`・`make wishlist-test-mysql` を流す
9. 文書(AC-A10)

ファイルの移動は `git mv` を使い、履歴を追えるようにする。

## 実装で足したもの(表からの小さな差)

- `item.CloneP`: `MergeOfficial`(domain)と memory が同じ `cloneP` を使うため、domain に置いて公開した。
- `page.Collapse`: `ogp` と `page.CleanTitle` が同じ関数を使うため、`collapse` を公開した(重複させないため)。
- `fromurl.IsInvalidURL`: 入力の URL の違反と、`PageReader` の失敗(`*fromurl.FetchError`)を区別する小さな関数。
- `domain/item/helpers_test.go`: `official_test.go` が使う `strp`。元は `service_test.go`(app へ移った)にあったため、同じ 1 行を domain のテスト側にも置いた。
- `Invalid` は domain/item に置いたが、使うのは app/items だけ(domain の `item.go`・`official.go` は `ErrInvalid` を直接包んでいる)。

## やらないこと

- `official` の判定(語の表・`Judge`)と `fetcher` の HTML の読み取りを domain に分けること(HTML を読む処理と分けにくい。必要になったら別に行う)
- `CreateItem` の `image_url` の取得(`httpapi.fetchImage`)を app に移すこと(4 つの崩れに入っていない。移すなら別の PR)
- ルートの `tools/archcheck`(ダメ計の S0)への組み込み。できたら、このモジュールの archtest と置き換えるかを決める
- 識別子の改名(`items.NewService`・`memory.NewMemoryRepository`・`store.NewMySQLRepository` のような重複した名前もそのまま)
