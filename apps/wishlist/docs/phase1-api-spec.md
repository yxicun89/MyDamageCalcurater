# wishlist フェーズ1 API の受け入れ条件

仕様の正は [../CLAUDE.md](../CLAUDE.md)(§4 検索ワード・§7 DB・§8 API)、設計は [design.md](design.md)(W-01〜W-09)、
API 契約は [../api/openapi.yaml](../api/openapi.yaml)。ここには、実装が満たす条件と、それを確かめるテストの対応を書く。
テストは `apps/wishlist/api` 配下(`make wishlist-test`。MySQL 実装は `make wishlist-test-mysql`)。

## 決めたこと(仕様に書かれていなかった部分)

- **空の override**:`query_override`・`site_overrides[].query` が空文字・空白だけなら「無い」として扱う(次の優先順位へ進む)
- **空白の詰め**:空白は Unicode の空白(Go の `unicode.IsSpace`。全角空白・タブを含む)。連続は半角空白 1 つに詰め、前後を除く。override にも同じ整形をかける
- **置換は 1 回の走査**:`{name}` の値に `{option}` が含まれても再置換しない(`strings.NewReplacer` 相当)。未知の `{foo}` はそのまま
- **ディープリンク**:`{q}` をすべて置換。エスケープは JS の `encodeURIComponent` と同じ(`A-Za-z0-9-_.!~*'()` 以外を UTF-8 の %XX。空白は `%20`)
- **エラーの使い分け**:形式の不正(JSON が壊れている・型違い・必須項目が無い・パスの id が数でない)は 400 `bad_request`。
  値の規則違反(存在しない genre_id / site_id・重複名・空の名前・長すぎる値・負の min_price・不正な fetch_type・不正な検索 URL・
  http/https 以外の URL・禁止アドレス・対応外/大きすぎる画像)は 422 `unprocessable`。外部取得の失敗(2xx 以外・接続失敗・HTML が大きすぎる)は 502 `bad_gateway`
- **from-url / image_url の http(s) 以外**:422(外部取得はしない)
- **画像の Cache-Control**:`public, max-age=31536000, immutable`(openapi.yaml にヘッダーを追記。下記「契約の変更」)。`X-Content-Type-Options: nosniff` も付ける
- **存在しない商品への refresh**:404(存在すれば 501)
- **画像の削除**:形式は正しいがファイルが無いときは成功扱い(冪等)。形式外の名前は ErrNotFound

## 受け入れ条件とテスト

### internal/query(純粋)— `internal/query/query_test.go`

| ID | 条件 | テスト |
|---|---|---|
| AC-Q1 | 仕様 §4 の表の 3 例が生成できる | `TestBuild_SharedVectors`・`TestBuild_SpecTableIsInVectors` |
| AC-Q2 | 優先順位 site_query > query_override > template | `TestBuild_SharedVectors` |
| AC-Q3 | 空の `{option}` 等で生じた余分な空白を詰める(全角空白・タブも) | `TestBuild_SharedVectors` |
| AC-Q4 | 空文字・空白だけの override は無いものとして扱う。置換は 1 回の走査 | `TestBuild_SharedVectors` |
| AC-Q5 | `Normalize`:NFKC → 小文字 → 空白・記号(Unicode P・S・Z)除去。長音符は残す | `TestNormalize` |

共通テストベクタ:`apps/wishlist/testdata/query-cases.json`(W-07。TS のテストも同じファイルを読む)。

### internal/deeplink(純粋)— `internal/deeplink/deeplink_test.go`

| ID | 条件 | テスト |
|---|---|---|
| AC-D1 | `{q}` を encodeURIComponent 相当で全置換(再置換しない) | `TestBuild_SharedVectors` |
| AC-D2 | 仕様 §5 のメルカリ URL で空白が `%20` | `TestBuild_SpecExamples` |
| AC-D3 | `ValidateTemplate`:http/https の絶対 URL(ホストあり)で `{q}` を含む。違反は `ErrInvalidTemplate` | `TestValidateTemplate` |

### internal/storage — `internal/storage/storage_test.go`

| ID | 条件 | テスト |
|---|---|---|
| AC-S1 | 内容の判定で jpeg/png/webp/gif を受け入れ、名前は小文字 UUID v4 + `.jpg/.png/.webp/.gif`。Open は同じ内容と拡張子から決めた Content-Type。保存のたびに名前が変わる | `TestSaveOpen_Accepted`・`TestSave_UniqueNames` |
| AC-S2 | それ以外(SVG・HTML・テキスト・空)は `ErrUnsupportedImage`。ctx 取り消しは `context.Canceled`。失敗時はファイルを残さない(一時ファイルも) | `TestSave_Rejected`・`TestSave_Canceled` |
| AC-S3 | 10 MiB ちょうどは可、超えたら `ErrTooLarge` | `TestSaveOpen_Accepted`・`TestSave_Rejected` |
| AC-S4 | 形式外の名前(パストラバーサル・大文字・v4 以外・NUL)は Open/Delete とも `ErrNotFound`。保存先の外に触れない。形式は正しいが無い:Open は `ErrNotFound`、Delete は nil | `TestOpenDelete_InvalidNames`・`TestOpenDelete_Missing` |
| AC-S5 | Delete 後は Open が `ErrNotFound` | `TestDelete` |
| AC-S6 | `NewFileStorage` は無いディレクトリを作る | `TestNewFileStorage_CreatesDir` |

### internal/netguard(SSRF 対策。W-05)— `internal/netguard/netguard_test.go`

| ID | 条件 | テスト |
|---|---|---|
| AC-N1 | 既定ではループバック等への接続を**ダイヤル時に**拒否し `ErrForbiddenAddress`(ホスト名 `localhost` でも)。サーバーに届かない。`IsPublic` の表(CGNAT 100.64/10・クラウドのメタデータ用アドレス・IPv4 射影 IPv6 を含む) | `TestClient_RejectsLoopback`・`TestClient_RejectsLocalhostName`・`TestIsPublic` |
| AC-N2 | `Options.AllowAddr` を差し替えれば取得できる | `TestClient_AllowedByOverride` |
| AC-N3 | リダイレクト先も検査(禁止アドレス → `ErrForbiddenAddress`、http/https 以外 → `ErrInvalidURL`、回数超え → `ErrTooManyRedirects`) | `TestClient_RedirectToForbidden`・`TestClient_RedirectToUnsupportedScheme`・`TestClient_TooManyRedirects` |
| AC-N4 | タイムアウト(既定 10 秒) | `TestClient_Timeout`・`TestClient_DefaultTimeout` |
| AC-N5 | プロキシの環境変数を使わない | `TestClient_IgnoresProxyEnv` |
| AC-N6 | `ValidateURL`:http/https の絶対 URL(ホストあり)だけ | `TestValidateURL` |
| AC-N7 | `ReadLimited`:ちょうど上限は可、超えたら `ErrTooLarge` | `TestReadLimited` |

### internal/ogp — `internal/ogp/ogp_test.go`(fixture は `internal/ogp/testdata/*.html`。架空の内容)

| ID | 条件 | テスト |
|---|---|---|
| AC-O1 | og:title(property、無ければ name 属性。複数なら最初)。前後の空白除去・連続空白を 1 つに・実体参照を解く | `TestParse` |
| AC-O2 | og:title が無い・空白だけなら `<title>`。どちらも無ければ空文字 | `TestParse` |
| AC-O3 | og:image を base URL で絶対 URL に解決(相対・プロトコル相対)。無い・空・http/https 以外は nil | `TestParse` |
| AC-O4 | `<head>` の外の meta も読む | `TestParse` |
| AC-O5 | `Fetcher.Draft` は取得して Parse。リダイレクト後の最終 URL を基準に解決 | `TestFetcher_Draft` |
| AC-O6 | 2xx 以外 → `ErrUpstreamStatus`、HTML 2 MiB 超 → `netguard.ErrTooLarge`、URL 違反 → `netguard.ErrInvalidURL`、既定クライアントでループバック → `netguard.ErrForbiddenAddress` | `TestFetcher_Draft_Errors` |
| AC-O7 | `Fetcher.Image` は最大 max バイト(ちょうどは可)。エラーは Draft と同じ規則 | `TestFetcher_Image` |

### internal/item — Repository 契約 `internal/item/itemtest/contract.go`(メモリ実装 `memory_test.go`、MySQL 実装 `mysql_test.go`(`-tags mysql`))

| ID | 条件 | テスト(`RunRepositoryContract` のサブテスト) |
|---|---|---|
| AC-R1 | ジャンルの作成・一覧。site_ids は指定順を保つ。一覧は sort_order 昇順・同順は id 昇順 | `CreateAndListGenres` |
| AC-R2 | 重複名 → `ErrDuplicateName`、存在しないサイト → `ErrSiteNotFound`、site_ids 内の重複 → `ErrInvalid`。失敗時は何も作らない | `CreateGenreErrors` |
| AC-R3 | ジャンルの部分更新。site_ids を渡すと全件置き換え(空なら全部外す)、省略は変えない。存在しない id → `ErrNotFound`、他と重複 → `ErrDuplicateName`。失敗した更新は一部も反映しない | `UpdateGenre`・`UpdateGenreErrors` |
| AC-R4 | サイトの作成・一覧(id 昇順)・部分更新・重複名・存在しない id | `Sites` |
| AC-R5 | 商品の作成・取得。存在しない genre_id → `ErrGenreNotFound`、存在しない id → `ErrNotFound`。省略した任意項目は nil | `CreateGetItem` |
| AC-R6 | 一覧は sort_order 昇順・同順は id 降順。genre_id で絞り込み(存在しないジャンルは空でエラーにしない) | `ListItems` |
| AC-R7 | PATCH の nullable(未指定は変えない・null で消す・値で設定) | `UpdateItemNullable` |
| AC-R8 | site_overrides の全件置き換え(site_id 昇順で返す。空なら全消し。存在しないサイト → `ErrSiteNotFound`・重複 → `ErrInvalid`、そのとき同じ patch の他の変更も入れない)。一覧にも載る | `UpdateItemSiteOverrides` |
| AC-R9 | 更新で存在しない id → `ErrNotFound`、存在しない genre_id → `ErrGenreNotFound` | `UpdateItemErrors` |
| AC-R10 | 削除は ImagePath を返す。以後 `ErrNotFound` | `DeleteItem` |

### internal/item — Service `internal/item/service_test.go`

| ID | 条件 | テスト |
|---|---|---|
| AC-V1 | 商品の作成で画像を保存し、ImagePath は保存した名前 | `TestService_CreateItem` |
| AC-V2 | 作成に失敗したら画像を残さない。画像が不正なら商品を作らない。検査で弾くときは画像を保存しない | `TestService_CreateItem_Rollback` |
| AC-V3 | 入力の検査 → `ErrInvalid`(空・空白だけの名前、文字数上限(rune)、負の min_price、不正な fetch_type、不正な検索 URL(`deeplink.ErrInvalidTemplate` も包む)) | `TestService_Validation` |
| AC-V4 | 既定値:query_template 省略 → `{name} {option}`、fetch_type 省略 → link_only | `TestService_Defaults` |
| AC-V5 | 画像の差し替えは成功したら古い画像を消す。商品が無い・画像が不正なら新しい画像を残さず ImagePath も変えない | `TestService_ReplaceImage` |
| AC-V6 | 商品の削除で画像も消す | `TestService_DeleteItem` |

### internal/httpapi — `internal/httpapi/server_test.go`(メモリ実装・t.TempDir のストレージ・fake の外部取得)

| ID | 条件 | テスト |
|---|---|---|
| AC-H1 | `/api/*` はトークン必須。無い・違う・前方一致・余計な文字・Basic・Bearer 無し → 401 `unauthorized`。認証前に外部取得しない。比較は定数時間(`crypto/subtle`。レビューで確認) | `TestAuth` |
| AC-H2 | `/healthz`(`{"status":"ok"}`)と `/images/{name}` は認証なし | `TestNoAuthEndpoints` |
| AC-H3 | エラーはすべて `{"code","message"}` だけ(存在しないパス 404・不正な id/クエリ 400・壊れた JSON 400・型違い 400) | `TestErrorShape` |
| AC-H4 | multipart の登録 → 201。image_url は `images/<UUID v4>.<ext>`、site_overrides は `[]`、null の項目は null か省略 | `TestCreateItem_Multipart` |
| AC-H5 | multipart の不正(画像・name 無し・genre_id が数でない → 400、存在しないジャンル・空の名前・負の min_price・SVG・10 MiB 超 → 422)。失敗時に画像を残さない | `TestCreateItem_MultipartErrors` |
| AC-H6 | JSON(image_url)の登録はサーバーが取得して保存する | `TestCreateItem_JSON` |
| AC-H7 | image_url が http(s) 以外 → 422(取得しない)、禁止アドレス → 422、取得失敗 → 502、大きすぎ・画像でない → 422、image_url なし → 400、存在しないジャンル → 422。画像を残さない | `TestCreateItem_JSONErrors` |
| AC-H8 | from-url は下書きを返し保存しない(genre_id はそのまま返す。省略なら null。画像なしは null) | `TestDraftFromURL` |
| AC-H9 | from-url の失敗(http(s) 以外 → 422・取得しない、url なし → 400、禁止アドレス → 422、取得失敗・HTML 上限超え → 502) | `TestDraftFromURL_Errors` |
| AC-H10 | 一覧の並びと genre_id の絞り込み。空でも `{"items":[]}` | `TestListItems` |
| AC-H11 | 存在しない商品は GET/PATCH/DELETE/PUT image/estimates/refresh/listings すべて 404 | `TestItemNotFound` |
| AC-H12 | PATCH:null で消す・省略は変えない・site_overrides 全件置き換え・422 の各条件 | `TestUpdateItem` |
| AC-H13 | PUT image で差し替え(古い画像は 404)、SVG は 422。DELETE は 204 で画像も消える | `TestReplaceImageAndDelete` |
| AC-H14 | estimates は空の sites と refreshing:false、refresh は 501 `not_implemented`、listings は `{"listings":[]}`(W-06) | `TestEstimatesPhase1` |
| AC-H15 | `/images/{name}` は Content-Type(png/jpeg/gif/webp)・`Cache-Control: public, max-age=31536000, immutable`・`X-Content-Type-Options: nosniff`。形式外・無い → 404 | `TestImages` |
| AC-H16 | ジャンルの一覧(site_ids を含む)・作成(既定値、site_ids は `[]`)・site_ids の置き換え・404・422 | `TestGenres` |
| AC-H17 | サイトの作成(既定値)・検索 URL の検査 → 422・重複名 → 422・不正な fetch_type → 422・PATCH・404・一覧(id 昇順) | `TestSites` |

### cmd/api — `cmd/api/main_test.go`

| ID | 条件 | テスト |
|---|---|---|
| AC-C1 | serve:WISHLIST_DATABASE_DSN・WISHLIST_API_TOKEN・WISHLIST_IMAGE_DIR が必須(空白だけも無いとみなす。エラーに変数名)。PORT は既定 8080、不正値はエラー。DSN は parseTime=true・multiStatements なし | `TestLoadServeConfig`・`TestLoadServeConfig_Missing` |
| AC-C2 | migrate:DSN だけ必須。multiStatements=true を足す(W-09) | `TestLoadMigrateConfig` |
| AC-C3 | DSN の加工(他のパラメータを保つ) | `TestDSN` |
| AC-C4 | サブコマンド:無し・不明 → 2、設定不足 → 1。理由を stderr に出し、トークンは出さない | `TestRun_Usage` |

## 契約の変更(openapi.yaml)

- `GET /images/{name}` の 200 に `Cache-Control` レスポンスヘッダーを追加。strict-server の応答型に `Headers.CacheControl` が生成され、
  ハンドラーから長期・immutable のキャッシュ指定を返せるようにするため(それ以外の変更なし)
