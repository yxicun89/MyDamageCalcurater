# wishlist フェーズ5(使用感の作り直し)の受け入れ条件

1 周目の改善ループ([ux-loop.md](ux-loop.md))で決めたことを実装する。画面の正はデザイン案
(ux-loop.md「作ったもの」のリンク。「青みの白 × その他の画面」と「薄グレー / 青みの白 × ホームと詳細」の段の青みの白)。
API 契約は [../api/openapi.yaml](../api/openapi.yaml)。テストは `make wishlist-test`(MySQL と migration は `make wishlist-test-mysql`)。

## 決めたこと(ユーザー決定 2026-10-11)

- 登録は URL を 1 つ渡すだけ。画像・名前・ジャンルはサーバーが推定し、人はジャンルを 1 タップで確かめて「追加」する。写真からの登録も残す
- トークンと接続の設定を無くす。8080 は 127.0.0.1 だけで待ち、外からは Tailscale を通す(ADR-0810)
- Yahoo!ショッピングは appid をやめ、検索ページ(robots.txt で許可されている `/search/<q>/0/?X=2`)を読む
- 見た目: 背景は青みの白(`#edf2f8`)、カードは白、画像に「ジャンル色の細い枠」(1.5px・不透明度 約 35%)。強調色は青(`#1d5fd1`)
- ジャンルの色(と絵柄)は設定で選べる。使いながら自分で調整する
- 状態は文ではなく、色・形・位置で伝える(価格の帯と各サイトの点、強調の縁、アイコン)。文字が要るところ(名前・編集欄)だけ文字

## 5-1 API

### 5-1a 認証を外す

- `bearerAuth` を外す。全エンドポイントがトークン無しで 2xx を返す。`Authorization` が付いていても無視する
- Secret の `api-token` は読まない(bootstrap.sh は作らない。既存の Secret に残っていても害はない)
- Ingress・CronJob・ネットワークの設定は変えない

### 5-1b ジャンルの色と絵柄

- `Genre` に `color`(`#rrggbb`。小文字に揃えて返す)と `icon`(列挙。`card` `figure` `robot` `goods` `star` `box`)を足す。必須で返す
- 作成・更新で受け取る。形式違反は 422。作成で省略したら `color=#1d5fd1`・`icon=box`
- migration で列を足し、既存の 4 ジャンルに初期値を入れる:デュエル・マスターズ `#e0393e`/`card`、S.H.Figuarts `#7c4dff`/`figure`、ガンプラ `#1f6fe5`/`robot`、ポケモングッズ `#f2b705`/`goods`(名前で引く。無ければ何もしない)
- ジャンル「METAL BUILD・ROBOT魂」を足す(`#5b6b7f`/`robot`。表示するサイトは S.H.Figuarts と同じ。同名があれば足さない)

### 5-1c URL からの下書き(推定)

`POST /api/items/from-url` の `ItemDraft` を次のように変える。取得は今と同じ netguard 経由・1 回だけ。

- **名前**:og:title(無ければ `<title>`)から、サイト名の部分を除く。除く規則はホストごと:
  - `dm.takaratomy.co.jp`:末尾の ` | デュエル・マスターズ` と、直前の `(型番 番号/番号)`
  - `tamashiiweb.com`:末尾の ` | 商品一覧 | 魂ウェブ`
  - `www.pokemoncenter-online.com`:末尾の ` ｜【公式】ポケモンセンターオンライン`
  - `bandai-hobby.net`:末尾の `｜バンダイ ホビーサイト`
  - それ以外:末尾の ` | …`・` ｜ …`・` - …` を 1 つだけ除く(除いて空になるなら除かない)
- **画像**:og:image。ただし
  - `bandai-hobby.net` は og:image がサイト共通の画像なので、本文の `.pg-products__sliderMain` の最初の `img` の `src` を使う(無ければ null)
  - `www.suruga-ya.jp`・`suruga-ya.jp` の `/product/detail/<番号>`(末尾の `/` 可。どちらも同じ商品ページに着く)は**ページを取りに行かず**、画像を `https://www.suruga-ya.jp/database/photo.php?shinaban=<番号>&size=l`、
    名前を空文字で返す(商品ページは Cloudflare の確認画面で読めない。回避しない)
- **ジャンル**:リクエストに `genre_id` があればそれ。無ければ推定し、決まらなければ null
  - ジャンルごとに `match_hosts`(ホスト名の配列)と `match_keywords`(語の配列)を持つ(`Genre` に足す。設定で編集できる)
  - 推定の順:(1) 名前を `query.Normalize` した文字列に、`match_keywords` の語(正規化)を含むジャンル。複数なら一致した語が最も長いもの、同じ長さなら sort_order が小さいもの
    (2) URL のホストが `match_hosts` に一致するジャンル(複数なら sort_order が小さいもの)
  - 初期値:デュエマ hosts `dm.takaratomy.co.jp` keywords `デュエル・マスターズ`・`デュエマ`/ S.H.Figuarts keywords `S.H.Figuarts`・`フィギュアーツ`/
    ガンプラ hosts `bandai-hobby.net` keywords `ガンプラ`・`HG `・`MG `・`RG `・`PG `・`30MF`・`30MM`(空白を含む語は正規化で空白が消えるので `HG` 等で入れる)/
    ポケモングッズ hosts `www.pokemoncenter-online.com`/ METAL BUILD・ROBOT魂 keywords `METAL BUILD`・`ROBOT魂`・`METAL ROBOT魂`
- `ItemDraft` に `genre_guessed`(boolean。推定で決めたら true)を足す
- `FromURLRequest` に任意の `title`・`image_url` を足す。`title` があれば**ページを取りに行かず**、`title` を上の規則で整えた名前と、
  `image_url`(http/https の絶対 URL だけ。違えば null)をそのまま使い、ジャンルを推定する(iOS の共有拡張がページから読んだ値を渡す)
- 取得に失敗しても、駿河屋のように取りに行かない場合は 200。それ以外の失敗は今と同じ(422・502)

### 5-1d 一覧に目安を載せる

- `Item` に `summary`(nullable)を足す:`{ low: int|null, status: "ok"|"no_result"|"failed"|"none", fetched_at: date-time|null }`
  - `low`・`fetched_at` は `ItemEstimates.summary_low`・`summary_fetched_at` と同じ値
  - `status`:low があれば `ok`、取得済みのサイトがすべて no_result なら `no_result`、取得済みが 1 つも無ければ `none`、それ以外は `failed`
- `GET /api/items` は 1 回のクエリ(または商品数に依らない回数)で作る(N+1 にしない)

### 5-1e Yahoo!ショッピングを検索ページから取る

- `sites` の Yahoo!ショッピングを `fetch_type=scrape` に替える(migration。テンプレートは今のまま `https://shopping.yahoo.co.jp/search/{q}/0/?X=2`)
- 取得は検索ページの `__NEXT_DATA__`(JSON)から、商品名・価格・URL・画像・在庫を上位 20 件。保存した HTML の fixture でテストする(実サイトを叩かない)
- 間隔は他の scrape と同じ 5 秒。詳細シートを開いたときにも取ってよい(headless ではない)
- appid の環境変数・Secret のキーは読まない(残っていても使わない)。`fetch_type=api` の Fetcher は消す

### 5-1 テスト一覧

受け入れ条件の番号(テストのコメントの `AC-5-1x-n`)と、確かめるテスト。`make wishlist-test`(★は `make wishlist-test-mysql`)。

| 番号      | 受け入れ条件                                                                                                                                      | テスト                                                                                                                                                                                                        |
| --------- | ------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| AC-5-1a-1 | トークン無しで組み立てたサーバーが、`Authorization` 無しで全エンドポイント 2xx。付いていても無視(401・`WWW-Authenticate` を出さない)              | httpapi `TestAuth`(旧 AC-H1 を置き換え)・`TestNoTokenSucceeds`・`TestPriceHistory_Errors`(末尾)                                                                                                               |
| AC-5-1a-2 | `WISHLIST_API_TOKEN`・`WISHLIST_YAHOO_APPID` を読まない。api の Deployment・CronJob に渡さない                                                    | cmd/api `TestLoadServeConfig`・`TestLoadServeConfig_Missing`・`TestLoadServeConfig_NoTokenNoAppID`(旧 AC-C5)/ cmd/refresher `TestLoadConfig`・`TestRefresherCronJob`・`TestAPIDeploymentYahooAppID`(旧 AC-K3) |
| AC-5-1b-1 | `Genre` の color(小文字)・icon・match_hosts・match_keywords が作成・一覧・更新で往復する(手がかりは長さ 0 でも配列)                               | item `TestService_GenreStyleValid`・itemtest `GenreStyleRoundTrip`・`GenreStyleEmptyHints`(メモリ・★MySQL)・httpapi `TestGenreStyle`                                                                          |
| AC-5-1b-2 | 色は大文字も受けて小文字に。作成で省略したら `#1d5fd1`・`box`。形式違反・列挙に無い絵柄は 422(何も変えない)                                       | item `TestService_GenreStyleDefaults`・`TestService_GenreStyleInvalid`・httpapi `TestGenreStyle_Errors`・★`TestGenreStyle000009_Constraints`                                                                  |
| AC-5-1b-3 | migration 000009 が既存の 4 ジャンルに色・絵柄を入れる(名前で引く。無ければ何もしない)                                                            | `TestGenreStyle000009Shape`・★`TestGenreStyle000009_Fresh`・★`_ExistingDB`・★`_NotAppliedTwice`・★`_Down`                                                                                                     |
| AC-5-1b-4 | 「METAL BUILD・ROBOT魂」(`#5b6b7f`/`robot`)を足し、表示するサイトは S.H.Figuarts と同じ。同名があれば足さず紐づけも変えない                       | ★`TestGenreStyle000009_Fresh`・★`_ExistingDB`・★`_Down`                                                                                                                                                       |
| AC-5-1c-1 | 名前は og:title(無ければ `<title>`)からホストごとの規則でサイト名を除く                                                                           | ogp `TestCleanTitle`・`TestParseAndClean_Fixtures`(実サイトの fixture 4 つ)・httpapi `TestDraftFromURL_Guess`                                                                                                 |
| AC-5-1c-2 | bandai-hobby.net の画像は本文 `.pg-products__sliderMain` の最初の img(無ければ null)。他のホストは og:image                                       | ogp `TestParse_BandaiHobbyImage`・`TestParseAndClean_Fixtures`                                                                                                                                                |
| AC-5-1c-3 | 駿河屋(`www.` の有無・末尾の `/` を問わない)の `/product/detail/<番号>` は取りに行かず、名前は空・画像は photo.php で 200                         | ogp `TestStaticDraft`・httpapi `TestDraftFromURL_Surugaya`                                                                                                                                                    |
| AC-5-1c-4 | リクエストの genre_id があればそれ(`genre_guessed: false`)                                                                                        | httpapi `TestDraftFromURL_Guess`                                                                                                                                                                              |
| AC-5-1c-5 | ジャンルの手がかりの初期値(migration 000009)                                                                                                      | `TestGenreStyle000009Shape`・★`TestGenreStyle000009_Fresh`                                                                                                                                                    |
| AC-5-1c-6 | 推定は 語(最長・同じ長さは sort_order・id)→ ホスト(sort_order)→ null。推定で決めたら `genre_guessed: true`                                        | item `TestGuessGenre`・httpapi `TestDraftFromURL_Guess`                                                                                                                                                       |
| AC-5-1c-7 | `title` を渡せば取りに行かず、整えた名前・http/https の絶対 URL の `image_url`(それ以外は null)で推定する。url の検査は今までどおり               | httpapi `TestDraftFromURL_TitleHint`                                                                                                                                                                          |
| AC-5-1d-1 | `summary` の規則(ok・no_result・none・failed、low・fetched_at は estimates のサマリと同じ。ジャンルに無い・上書きで外したサイトは使わない)        | item `TestSummarizeItem`・httpapi `TestListItems_Summary`                                                                                                                                                     |
| AC-5-1d-2 | `GET /api/items` は全商品に summary を付ける(ジャンルで絞っても同じ)                                                                              | itemtest `ListItemsSummary`(メモリ・★MySQL)・httpapi `TestListItems_Summary`                                                                                                                                  |
| AC-5-1d-3 | `GET /api/items` のクエリ回数は商品数に依らない(N+1 にしない)                                                                                     | ★item `TestMySQLListItems_NotNPlusOne`                                                                                                                                                                        |
| AC-5-1e-1 | Yahoo!ショッピングの検索ページの `__NEXT_DATA__` から上位 20 件(名前・整数の価格・URL・image.imageUrl・在庫 true)。壊れたページはエラー、0 件は空 | fetcher `TestYahooShopping_Fixture`・`_SkipsBrokenEntries`・`_PageStructure`・`TestYahoo_PriceRange`・`TestScrape_*`(yahooshopping)                                                                           |
| AC-5-1e-2 | 登録表で shopping.yahoo.co.jp の scrape を選ぶ(5 秒間隔・夜間だけにしない)。fetch_type=api の Fetcher は無い                                      | fetcher `TestRegistry_YahooShopping`・`TestRegistry_ForSite`・`TestNewRegistry`・`TestUserAgent_Sent`                                                                                                         |
| AC-5-1e-3 | migration で Yahoo!ショッピングを scrape に(名前と URL で特定。down で api に戻す)                                                                | `TestGenreStyle000009Shape`・★`TestGenreStyle000009_Fresh`・★`_ExistingDB`・★`_Down`                                                                                                                          |

## 5-2 Web

- 配色・形はデザイン案のとおり(背景 `#edf2f8`・カード白・ジャンル色の細い枠・強調の青)。丸文字(M PLUS Rounded 1c)
- **ホーム**:2 列のグリッド。画像は角丸で枠を付ける。右下に価格の札(`¥A〜`。目安が無ければ出さない。no_result は「出品なし」の札)。上にジャンルの絵柄のボタン(選ぶと強調色で塗る)。右下に「+」
- **追加**:「+」で下からシート。「コピーした URL を追加」(押したときにクリップボードを読む。読めなければ入力欄を開く)・「URL を入力」・「写真」
  - URL のあと:画像・名前・推定したジャンルの絵柄を大きく出し、1 タップで「追加」。ジャンルは絵柄を押して替える
  - 画像が取れなければ「写真を選ぶ」の枠と名前の欄を出す
- **詳細**:画像(大・枠つき)、ジャンル色の点と名前と取得日、価格の帯(安い〜高い)に各サイトの点、サイトのタイル 3 列(押すと検索結果を新しいタブで開く。最安は強調の縁、出品なしは薄く、リンクだけのサイトは価格なし)、「あやしい出品 N」(押すと一覧:小さな画像・価格・理由のタグ・サイト)。出品が無ければ価格の帯の代わりに「出品ないかも」
- **長押し**:背景をぼかし、編集・元のページを開く・削除
- **設定**:ジャンル(絵柄・色・件数)・サイト。トークンと API の URL の欄は消す
- **ジャンルの編集**:名前・色(12 色の見本)・絵柄・見るサイトと順・URL の手がかり(ホスト・語)・別名グループ
- **オフライン**:上に「オフライン」の札。価格の札を消す。リンクは使える

### 5-2 デザインの決まり

実装者はデザイン案のファイルを見なくてよいように、トークンと画面ごとの規則をここに書く(デザイン案は「青みの白 × ホームと詳細」「青みの白 × その他の画面」)。
値は `web/src/styles.css` の CSS カスタムプロパティにする。常時動くアニメーションは入れない(押したときの反応だけ)。ダークテーマは作らない(5-2 の対象外)。

**トークン**

| 名前            | 値                         | 用途                                                                                                          |
| --------------- | -------------------------- | ------------------------------------------------------------------------------------------------------------- |
| `--bg`          | `#edf2f8`                  | 画面の背景                                                                                                    |
| `--surface`     | `#ffffff`                  | カード・シート・入力欄                                                                                        |
| `--line`        | `#d9e2ee`                  | 1px の枠・区切り(区切りの薄い線は `#e3e9f1`)                                                                  |
| `--fg`          | `#0e1a2b`                  | 本文                                                                                                          |
| `--muted`       | `#56677d`                  | 補助の文字・アイコン                                                                                          |
| `--accent`      | `#1d5fd1`                  | 強調色(主ボタン・選択中・最安の縁・「+」)。`--on-accent` は `#ffffff`                                         |
| `--accent-soft` | `#1d5fd1` の 20%           | 価格の帯の「安い〜中央」                                                                                      |
| `--danger`      | `#d92d20`                  | 削除                                                                                                          |
| `--warn`        | `#c2410c`                  | 「あやしい出品」                                                                                              |
| `--disabled`    | `#c9d5e4`                  | 押せない主ボタンの地                                                                                          |
| `--genre-color` | 要素ごとにジャンルの color | 枠・点・アイコン。タイル等の `style` に `--genre-color: #rrggbb` を置き、同じ値を `data-genre-color` にも置く |

- フォント: `'M PLUS Rounded 1c'`(500 と 800)、フォールバックは sans-serif。見出し・価格・ボタンは 800、本文は 500。Google Fonts の CSS は `index.html` から読む(オフラインではフォールバックでよい)
- 角丸: 画像タイル 22px / カード 18〜20px / ボタン 14〜18px / 札は完全な丸(999px)。セーフエリア(`env(safe-area-inset-*)`)を上下に足す
- ジャンルの色の枠: `box-shadow: 0 0 0 1.5px <genre-color の不透明度 約 35%(rgba か `#rrggbb59`)>, 0 6px 16px rgba(0,0,0,0.07)`。画像は `object-fit: cover`

**ジャンルの絵柄(24x24 の線画 svg。`stroke=currentColor`・`stroke-width=2`・`fill=none`。`data-icon` に列挙の値を持つ)**

| icon                  | パス                                                                               |
| --------------------- | ---------------------------------------------------------------------------------- |
| `all`(「すべて」専用) | `M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z`                               |
| `card`                | `M5 2h14v20H5zM12 7l1.5 3 3.3.5-2.4 2.3.6 3.2L12 14.5 9 16l.6-3.2-2.4-2.3 3.3-.5z` |
| `figure`              | `M12 2a3 3 0 1 1 0 6 3 3 0 0 1 0-6zM12 8v7M7 11l5-1 5 1M9 22l3-7 3 7`              |
| `robot`               | `M6 4h12v9H6zM9 8h.01M15 8h.01M12 1v3M8 13v4M16 13v4M5 21h5M14 21h5`               |
| `goods`               | `M12 3a9 9 0 1 1 0 18 9 9 0 0 1 0-18zM3 12h6M15 12h6`                              |
| `star`                | `M12 2l3 7 7 .6-5.3 4.7 1.7 7.2L12 17.8 5.6 21.5l1.7-7.2L2 9.6 9 9z`               |
| `box`                 | `M3 7l9-4 9 4v10l-9 4-9-4zM3 7l9 4 9-4M12 11v10`                                   |

絵柄ボタンの `aria-label` は、ジャンルを選ぶ場面ではジャンル名、絵柄を選ぶ場面(ジャンル編集)では `card`=カード・`figure`=フィギュア・`robot`=ロボ・`goods`=グッズ・`star`=星・`box`=箱。

**色の見本(12 色。ジャンル編集。`aria-label` は日本語名)**

赤 `#e0393e`・朱 `#f0642a`・橙 `#f59e0b`・黄 `#f2b705`・黄緑 `#7cb518`・緑 `#16a34a`・青緑 `#0d9488`・水色 `#0ea5e9`・青 `#1f6fe5`・藍 `#4f46e5`・紫 `#7c4dff`・桃 `#e04f9a`。
6 列 2 行の丸(`aspect-ratio: 1`)。選択中は白 3px + その色 5px のリング(`box-shadow`)にチェックを重ね、`aria-pressed="true"`。新規ジャンルの初期色は `#1d5fd1`(見本に無い色なので、どの見本も押されず、プレビューだけがその色)。

**画面ごとの規則**

- **ホーム**: 上に「欲しいもの」(26px/800。`<h1>`)と件数(13px・muted。`data-testid="item-count"`、いま表示している商品数)、右に設定ボタン(44px・角丸 14・白・`aria-label="設定"`)。オフラインのときは右に「オフライン」の札(`data-testid="offline-badge"`・高さ 32・`--fg` の地・白い文字・雲の斜線アイコン)。
  その下にジャンルの絵柄ボタンを横に並べる(52px 角・角丸 16。`aria-label`=ジャンル名、先頭に「すべて」。非選択は白地+`--genre-color` の線画+1px の `--line`、選択は `--accent` 地+白の線画。`aria-pressed`)。
  グリッドは 2 列・横の余白 16px・行間 16px。タイルは高さ 220px の `<button>`(`aria-label`=商品名、`data-genre-color`、`--genre-color`)で、画像(`alt`=商品名)が全面。
  価格の札(`data-testid="price-tag"`)はタイルの右下 8px。`¥880〜`(14px/800・padding 4px 10px・半透明の暗い地 `rgba(14,26,43,0.6)`+白い文字+背景ぼかし、`data-kind="price"`)、「出品なし」(12px・地は同じで文字は `#e6ebf2`、`data-kind="none"`)。目安が `none`/`failed`/無しなら札を出さない。
  「いちばん安い商品を強調する」ような絞り込みは 5-2 では行わない(デザイン案の `deal` の基準が決まっていないため。今は全部同じ札)。
  右下の「+」(64px 角・角丸 22・`--accent` 地・`aria-label="追加"`・影 `0 10px 30px rgba(0,0,0,0.3)`)。
- **追加(下から出るシート)**: `Modal` の名前は「追加」。角丸 28px の上辺・上に 40x5 のつまみ・背景 `rgba(14,26,43,0.28)`。
  入口: 主ボタン「コピーした URL を追加」(高さ 64・`--accent` 地・白・クリップボードの絵柄)、その下に 2 列で「URL を入力」「写真」(高さ 56・白・1px の線)。「写真」は `<label>` の中の `<input type="file" accept="image/*" aria-label="写真">`(入力は見えなくてよい)。最下段の補足「Safari の共有からも追加できます」は 13px・muted(任意)。
  すべての段に「キャンセル」(テキストボタン)を置く。クリップボードは「コピーした URL を追加」を押した瞬間にだけ `navigator.clipboard.readText()` で読む(開いただけでは読まない)。`http(s)` の絶対 URL でなければ(読めない・拒否・未対応も)、`URL`(`type=url`)の入力欄と「取得」を出す。
  確認: 画像(190px 角・角丸 22・ジャンル色の枠。`object-fit: cover`)、名前(18px/800・中央。`<input aria-label="名前">`を文字のように見せる)、ジャンルの絵柄(60px 角・角丸 18。`role="group" aria-label="ジャンル"` の中の `aria-pressed` ボタン。選択は `--genre-color` 地+白、非選択は白地+ジャンル色の線画。選択は 1.08 倍)、主ボタン「追加」(高さ 58・全幅・`--accent`、押せないときは `--disabled`)。
  画像が無い・写真から: 200px 角の破線枠(2px dashed `#b9c8db`・地は `--bg`・`--accent` の文字)の「写真を選ぶ」(`<label>`+`<input type=file aria-label="写真を選ぶ">`)、写真を選んだあとはその写真のプレビューに替わる。「このページの画像は取れませんでした」(13px・muted)。名前の欄(高さ 48・角丸 14)。
  「追加」を押せる条件: 名前が空でない・ジャンルが選ばれている・画像(下書きの `image_url` か選んだ写真)がある。ジャンルの初期値は 下書きの `genre_id` → 無ければホームで絞っているジャンル → 無ければ未選択。`genre_guessed` は表示の出し分けには使わない(確かめるのは人)。
  登録: 写真があれば multipart(`createItemWithImage`)、無ければ下書きの `image_url` を JSON(`createItemFromImageUrl`)。どちらも `source_url`(下書きがあれば)を渡す。失敗は `role="alert"` を出し、シートは閉じない。
- **詳細(シート `Modal` の名前「詳細」)**: 上に画像(300px 角・角丸 24・白地・ジャンル色の枠・padding 10・`object-fit: contain`)と右上の閉じる(44px の丸・`aria-label="閉じる"`)。
  その下に 1 行: ジャンル色の点(10px・`data-testid="genre-dot"`・`data-genre-color`)・名前(15px/800・1 行で省略)・時計のアイコン+取得日(`data-testid="fetched-at"`・12px・muted・`summary_fetched_at` の JST `M/D`。無ければ出さない)。
  検索ワード(`検索ワードを編集` ボタン)は名前の行の下に小さく置く(挙動は今のまま: その場編集・保存を押すまで一時的)。
  価格の帯(`data-testid="price-band"`・カード・角丸 20): 左に大きく `¥low`(30px/800・`data-testid="band-low"`)と小さく `〜 ¥mid`(16px/800・muted・`data-testid="band-high"`。mid が無ければ「〜」だけ)。
  その下に帯(高さ 34px): 4px の線(`--line`)と、左端から全体の幅いっぱいまでの範囲を `--accent-soft` の 8px の太線で示す。各サイトの点(`data-testid="price-dot"`・28px の丸・サイトの色・頭文字・2px の白縁・`aria-label`=サイト名・`data-site-id`・`data-position`)を `left` で置く。
  `data-position` は整数 0〜100: 点にするサイト(このジャンルのサイトで `low` が非 null)の最安を 0、最高を 100 として `round((low - min) / (max - min) * 100)`。全部同額(点が 1 つ)は 0。CSS では `left: calc(6% + position * 0.88%)` のように両端に余白を取る。
  価格の帯の代わり: どのサイトにも `low` が無く、`sites` が 1 つ以上あり全部 `no_result` のとき、`data-testid="no-listing"` のカード(虫めがねのアイコン+「出品ないかも」17px/800)。`sites` が空なら何も出さず、`role="status"` に「まだ価格情報はありません」。
  `role="status"`(常に置く。短い文だけ): 「読み込み中…」「オフライン」「価格情報を取得できませんでした」「更新中…」「オフライン(前回の値)」「更新できませんでした」「まだ価格情報はありません」。成功して更新中でもないときは空。
  サイトのタイル(`<ul aria-label="サイト" data-columns="3">`・3 列・gap 10px): 各 `<li><a>` は `aria-label`=サイト名・`href`=検索 URL・`target="_blank"`・`rel="noopener"`・高さ 76 以上・角丸 16・白・1px の `--line`。中に マーク(30px の角丸 10・サイトの色・サイト名の先頭 1 文字・`data-testid="site-mark"`)と価格(13px/800・`data-testid="site-price"`)。
  価格: `status != no_result` で `low` があれば `¥low`(`failed` で前回値でもそのまま出し、`title="最終取得: N日前"`(今日なら「今日」)を付ける)、`no_result` は「なし」+タイルを薄く(`data-dim="true"`・マークの不透明度 0.45)、`failed` で `low` が無ければ価格なしで `data-failed="true"`、目安が無いサイト(link_only など)は価格なし。
  最安(`low` の最小。同額は全部)のタイルは 2px の `--accent` の縁・価格を `--accent`・`aria-current="true"`・`data-best="true"`。サイトの色(マークの地)はサイト名から決めてよい(`sites` に色の項目はまだ無い。固定の表で、無い名前は `--muted`)。
  「あやしい出品 N」(N は `suspicious_count` の合計。0 なら出さない。高さ 44・角丸 14・白・1px の線・`--warn` の文字+三角のアイコン・`aria-expanded`)。押すと同じシートの中身が一覧に替わる(上に「戻る」ボタンと「あやしい出品 N」の見出し)。一覧は `<ul aria-label="あやしい出品">`、1 件は白いカード(角丸 18)で、左に 64px の画像(`alt`=タイトル・灰色寄り・http(s) の画像だけ)、中央に価格(18px/800)と理由のタグ、右に出品元サイトのマーク(32px・`data-testid="listing-site"`・`aria-label`=サイト名)。URL が http(s) なら行全体が `<a target="_blank" rel="noopener">`。
  理由のタグ(`data-testid="reason-tag"`・`data-reason`・高さ 24・角丸 999・12px/800・小さなアイコン+文字): `title_mismatch`「名前が違う」(地 `#fff1e6`・文字 `#c2410c`)、`too_cheap`「安すぎ」(`#fde8ec`・`#be123c`)、`below_min`「下限より下」(`#eef2ff`・`#4338ca`)。
  「更新」ボタン・価格の推移(`<details>`)・公式の販売状況は今のまま(位置はタイルの下)。件数・在庫の文は 5-2 では出さない(状態は形で伝える決定)。
- **長押しメニュー**: 長押し 500ms は今のまま。出ている間はホーム(`data-testid="home"`)に `data-blurred="true"` を付け、背景を 6px ぼかして `rgba(14,26,43,0.28)` で暗くする。中央よりやや上に、押した商品の画像(210x230・角丸 22・ジャンル色の枠・1.04 倍。`data-testid="menu-preview"` の中)、その下に幅 240 のメニュー(`role="menu"`・白 96%・角丸 18・各行 52px): 「編集」・「元のページを開く」(`role="menuitem"` の `<a>`・`source_url` が http(s) のときだけ・`target="_blank"`・`rel="noopener"`)・「削除」(`--danger`)。背景のタップか Escape で閉じる。削除は今までの確認(`alertdialog`)を通す。
- **編集(シート `Modal` の名前「編集」)**: 上辺の余白 70px まで広げる。画像(88px 角・角丸 18・ジャンル色の枠)と「画像を差し替え」(`<input type=file>`のラベルは今のまま)。ジャンルは絵柄ボタン(48px 角・`role="group" aria-label="ジャンル"`・選択は `--genre-color` 地+白)。
  名前・オプション・検索ワード上書き・最低価格(円)・公式ページを監視する の項目名は今のまま。主ボタン「保存」は下に寄せる。
- **設定**: 上に戻る(44px・`aria-label="戻る"`)と「設定」(22px/800)。「接続」の区画は無い。「ジャンル」の区画: 白いカードに行(高さ 58): 絵柄の四角(36px・角丸 11・ジャンル色の地+白の線画・`data-testid="genre-icon"`・`data-genre-color`)・名前(16px)・件数(13px・muted・`data-testid="genre-count"`・そのジャンルの商品数)・「<名前>を編集」ボタン。最後に「ジャンルを追加」。「サイト」の区画は今のまま(行ごとに「<名前>を編集」)。マーク付きのチップ表示への切り替えは 5-2 では要らない。
- **ジャンルの編集(シート「ジャンル」)**: 上に絵柄(60px・ジャンル色の地)とジャンル名(`aria-label="ジャンル名"`)。続けて 検索ワードのテンプレート・色(`role="group" aria-label="色"`・12 色)・絵柄(`role="group" aria-label="絵柄"`・6 つ)・見るサイトと順(今のまま: チェックと「<名前>を上へ」)・URL の手がかり(`<textarea aria-label="手がかりのホスト">`・`<textarea aria-label="手がかりの語">`。カンマ `,`・`、`・改行のどれでも区切れる。前後の空白と空の項目は除く)・別名グループ(今のまま)。
  「ホームでの見え方」のプレビュー(`data-testid="genre-preview"`・64x78・角丸 14・ジャンル色の枠・`data-genre-color`)は選んだ色にすぐ追従する。保存は変えた項目だけ PATCH(`color`・`icon`・`match_hosts`・`match_keywords` を含む)。追加は `color`・`icon`・`match_hosts`・`match_keywords` を常に送る。
- **オフライン**: 「オフライン」の札を出すのは、(a) 一覧(items・genres・sites)のどれかを取れず保存済みを使ったとき(`fetchWithCache` の `stale`)、(b) ブラウザの `offline` イベント後(`online` イベントで戻す)、(c) 起動時に `navigator.onLine === false`。オフライン中は価格の札を消す(`price-tag` を出さない)。サイトのタイルは使える。
- **API クライアント**: `createApiClient` は `Authorization` を付けない(`token` オプションを消す)。ベース URL は `defaultBaseUrl()` だけ(`wishlist.settings` と `lib/settings.ts` を消す。`openHome` の `seedSettings` と `test/fakeApi.ts` の `seedSettings` も一緒に消す)。

#### 実装者への注意(5-2)

- 変更のないテストを緑に保つため、`openHome` は移行用に `seedSettings()` を呼んでいる。`lib/settings.ts` を消すとき、`lib/settings.test.ts`・`seedSettings`(`test/fakeApi.ts`・`test/render.tsx`・`App.offline.test.tsx`・`App.home.test.tsx`)も消す(決定: ユーザー 2026-10-11)。`App.noauth.test.tsx` が「何も保存しなくても動く」ことを守る
- 画面のテスト用 ID(`data-testid`)・`aria-label`・`data-*` は上の規則とテストの一覧が契約。CSS の見た目はテストしない
- 件数・在庫の文を消したので、`SiteDetail`・`summaryText` の「だいたい…買えそう」・`formatRange` の画面での使い方(帯は `formatYen` を使う)が不要になる。`reasonLabel` の文言は新しい短いラベルに変える(`lib/price.ts`)
- 「参考外」の語は画面から消す(`<details className="suspicious">` は「あやしい出品」のボタン+一覧に置き換える)
- `from-url` には `genre_id` を渡さない(サーバーが推定する)。`FromURLRequest.title`・`image_url` は iOS の共有拡張用で Web は使わない

### 5-2 テスト一覧

`make wishlist-web-test`(単体は `cd apps/wishlist/web && npx vitest run src/<ファイル>`)。「置換」は、旧テストの趣旨を新しい仕様で置き換えたもの(各テストの上にコメントで ユーザー決定 2026-10-11 / phase5-spec 5-2 を書いてある)。

| 番号         | 受け入れ条件                                                                                                                        | テスト                                                                                                                                                                                                                                                 |
| ------------ | ----------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| AC-5-2a-1    | 初回起動でも設定画面を挟まずホームが出る。全リクエストに Authorization が付かない                                                   | `App.noauth.test.tsx` AC-5-2a-1 / `App.home.test.tsx` AC-HOME-02(置換)/ `lib/api.test.ts` AC-API-01・AC-API-08・AC-HIS-API-01(置換)/ `App.estimates.test.tsx` AC-EST-01・AC-EST-05 / `App.history.test.tsx` AC-HIS-01                                  |
| AC-5-2a-2    | 保存済みの `wishlist.settings`(旧トークン・API の URL)を読まない。設定画面にトークン・API の URL の欄が無い                         | `App.noauth.test.tsx` AC-5-2a-2 / `App.settings.test.tsx` AC-SET-01(置換)                                                                                                                                                                              |
| AC-5-2c-1〜4 | ホーム: 枠の色=ジャンル色、価格の札(¥low〜 / 出品なし / 出さない)、絵柄のジャンルボタン、見出しと件数                               | `App.home.test.tsx` AC-5-2c-1〜4・AC-HOME-03/04(置換)                                                                                                                                                                                                  |
| AC-5-2d-1〜5 | 追加の入口: コピーした URL(押したときだけ読む。読めない・URL でない・未対応は入力欄)・URL を入力・写真                              | `App.register.test.tsx` AC-5-2d-1〜5                                                                                                                                                                                                                   |
| AC-5-2e-1〜6 | 確認: 推定ジャンルが選ばれ 1 タップで追加、ジャンルを替える、未決定は選ぶまで押せない、名前、画像なしは写真を選ぶ、失敗は alert     | `App.register.test.tsx` AC-5-2e-1〜6                                                                                                                                                                                                                   |
| AC-5-2f-1〜3 | 写真から追加(multipart)・絞り込み中のジャンルが初期値・キャンセル                                                                   | `App.register.test.tsx` AC-5-2f-1〜3(旧 AC-REG-01/03/06 の置換)                                                                                                                                                                                        |
| AC-5-2g-1〜4 | オフライン: 札、価格の札を消す、イベント、リンクは使える                                                                            | `App.offline.test.tsx` AC-5-2g-1〜4・AC-OFF-03(置換)                                                                                                                                                                                                   |
| AC-5-2h-1〜3 | 詳細の頭(ジャンル色の点・取得日)、価格の帯(low・mid)、各サイトの点(位置)                                                            | `App.detail.test.tsx` AC-5-2h-1〜3 / `App.estimates.test.tsx` AC-EST-02(置換)                                                                                                                                                                          |
| AC-5-2i-1〜5 | サイトのタイル: 3 列・リンク、マークと価格、最安の強調、出品なしは薄く、失敗                                                        | `App.detail.test.tsx` AC-5-2i-1〜5 / `App.estimates.test.tsx` AC-EST-03(置換)                                                                                                                                                                          |
| AC-5-2j-1〜2 | 出品ないかも(帯の代わり)・目安がまだ無い                                                                                            | `App.detail.test.tsx` AC-5-2j-1〜2 / `App.estimates.test.tsx` AC-EST-02                                                                                                                                                                                |
| AC-5-2k-1    | あやしい出品 N のボタン・一覧(画像・価格・理由のタグ・サイト・リンク・戻る)                                                         | `App.detail.test.tsx` AC-5-2k-1 / `App.estimates.test.tsx` AC-EST-06(置換)/ `lib/price.test.ts` reasonLabel(置換)                                                                                                                                      |
| AC-5-2l-1〜5 | 設定: ジャンルの行(絵柄・色・件数)、12 色、6 つの絵柄、URL の手がかり、追加の既定値                                                 | `App.settings.test.tsx` AC-5-2l-1〜5・AC-SET-03/04(既存のまま。サイトの選択と順)・`App.aliases.test.tsx`(既存のまま)                                                                                                                                   |
| AC-5-2m-1〜3 | 長押しメニュー: 編集・元のページを開く(source_url がある時だけ)・削除、背景のぼかし、画像のプレビュー。編集のジャンルは絵柄のボタン | `App.edit.test.tsx` AC-5-2m-1〜3・AC-EDIT-01(置換)                                                                                                                                                                                                     |
| (維持)       | ディープリンク・検索ワードの一時編集・オフラインのキャッシュ・価格の推移・公式の販売状況・別名辞書                                  | `App.sheet.test.tsx`・`App.history.test.tsx`・`App.official.test.tsx`・`App.aliases.test.tsx`・`lib/*.test.ts`・`pwa/*.test.ts`(`App.sheet.test.tsx` の AC-SHEET-02 だけ、サイト名を `textContent` から `aria-label` で見るように置換。ほかは変更なし) |

## 5-3 iOS

- 画面・配色は Web と同じ(SwiftUI。Liquid Glass はシートとツールバーだけ)
- 接続先はビルド設定(`WISHLIST_API_BASE_URL`。git に入れない xcconfig から読む)。設定画面に接続の欄を出さない
- Share Extension:Safari から共有したとき、拡張の JavaScript(`NSExtensionJavaScriptPreprocessingFile`)でページの og:image・og:title(無ければ最初の大きな画像・document.title)を読み、
  `POST /api/items/from-url` に `url`・`title`・`image_url` を渡して下書きを作る(サーバーはページを取りに行かない。駿河屋のようにサーバーから読めないページでも登録できる)
- トークンを保存・送信しない
