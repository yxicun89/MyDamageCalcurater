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
  - `www.suruga-ya.jp` の `/product/detail/<番号>` は**ページを取りに行かず**、画像を `https://www.suruga-ya.jp/database/photo.php?shinaban=<番号>&size=l`、
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

## 5-3 iOS

- 画面・配色は Web と同じ(SwiftUI。Liquid Glass はシートとツールバーだけ)
- 接続先はビルド設定(`WISHLIST_API_BASE_URL`。git に入れない xcconfig から読む)。設定画面に接続の欄を出さない
- Share Extension:Safari から共有したとき、拡張の JavaScript(`NSExtensionJavaScriptPreprocessingFile`)でページの og:image・og:title(無ければ最初の大きな画像・document.title)を読み、
  `POST /api/items/from-url` に `url`・`title`・`image_url` を渡して下書きを作る(サーバーはページを取りに行かない。駿河屋のようにサーバーから読めないページでも登録できる)
- トークンを保存・送信しない
