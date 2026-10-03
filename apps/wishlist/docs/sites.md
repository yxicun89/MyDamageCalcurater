# 通販サイト調査(2026-10-03 手動確認)

調査方法: curl(ブラウザ風 UA、同一ホスト 5 秒以上あけ、並列なし)。検索語は「グリス」「ボルシャック」。{q} は URL エンコード(空白は %20)。
実ページ HTML は保存していない(一時ファイルは削除済み)。fixture は構造だけを写した架空データ。

## 一覧

| サイト | 検索 URL テンプレート | 方式 | 確認 | セレクタ |
|---|---|---|---|---|
| メルカリ | https://jp.mercari.com/search?keyword={q}&status=on_sale&sort=price&order=asc | headless | 取得 200。静的 HTML は骨格のみ | なし(JS 描画) |
| Amazon | https://www.amazon.co.jp/s?k={q}&s=price-asc-rank | link_only | 調査対象外 | なし |
| Yahoo!フリマ | https://paypayfleamarket.yahoo.co.jp/search/{q} (並び順の指定は下記) | scrape(埋め込み JSON) | 取得 200、JSON 確認 | JSON パス(下記) |
| カードラッシュ(DM) | https://www.cardrush-dm.jp/product-list?keyword={q}&order=asc&available=1&num=20 | scrape | 確認済み | あり |
| ドラゴンスター | 未確認(`https://dorasuta.jp/dm/product-list?keyword={q}` を試したが 403) | 未確認 | Cloudflare のチャレンジで取得不可 | なし |
| あみあみ | https://slist.amiami.jp/top/search/list?s_keywords={q}&s_sortkey=pricea | scrape | 確認済み(下記注意) | あり |
| 駿河屋 | https://www.suruga-ya.jp/search?category=&search_word={q}&rankBy=price%3Aascending&inStock=On | scrape | 検索 URL とマークアップは確認。並び替え・在庫絞り込みの効きは未確認 | あり |
| Yahoo!ショッピング | https://shopping.yahoo.co.jp/search/{q}/0/?X=2 | api(人が開く URL のみ) | 検索ページ取得 200。X=2 はページ内「価格が安い順」リンクの href から(直接は未取得) | 不要(API) |
| プレミアムバンダイ | https://p-bandai.jp/search_bst/?q={q} | link_only | トップのフォーム定義から。検索ページは robots で禁止のため取得せず | なし |
| 魂ウェブ | https://tamashiiweb.com/item/?wo={q} | link_only | フォーム定義から。ページは 200 だが静的 HTML に検索結果は見つからず(下記) | なし |
| ポケモンセンターオンライン | https://www.pokemoncenter-online.com/?word={q}&main_page=search_result | link_only | 302 で待機室へ。結果ページ自体は未確認 | なし |

アクセス回数(robots.txt 含む。リクエスト数):
メルカリ 2 / Yahoo!フリマ 2 / Yahoo!ショッピング 2 / カードラッシュ 4(cardrush.jp の robots 1、cardrush-dm.jp の robots 1、検索 2)/
駿河屋 3 / あみあみ 4(www の robots 1、www 検索 1 は 403、slist 検索 2)/ ドラゴンスター 2(いずれも 403)/
プレバン 2(robots、トップ)/ 魂ウェブ 2(robots、検索)/ ポケセン 2(robots、検索 1 は 302 で終了)。Amazon 0。全サイト 5 回以内。

## robots.txt

- メルカリ: /search は禁止されていない(禁止は /mypage/ /purchase/ /sell/ /transaction/ /v1/ /v2/ 等)。
- Yahoo!フリマ: `/search/*?*sort=` `*order=` `*conditions=` `*minPrice=` `*maxPrice=` `*sold=` `*open=` `*specs=` `*showFilter=` が禁止。**並び替え・絞り込みのパラメータ付き検索ページは取得してはならない**。fetcher は `/search/{q}` のみ(パラメータなし)を使う。人向けのリンクに sort を付けるかは別途判断(付けた場合の挙動は未確認)。
- Yahoo!ショッピング: /search は禁止されていない。`/searchbff/*/itemsInfo` は禁止。`/category/*?...` の絞り込みパラメータが多数禁止(/search には無かった)。価格取得は公式 API を使う。
- カードラッシュ(cardrush.jp / cardrush-dm.jp どちらも同内容): GPTBot・Bytespider・TikTokSpider・meta-externalagent のみ全面禁止。他は制限なし。
- 駿河屋: `User-agent: *` に **Crawl-delay: 30**。`Disallow: /search/`(末尾スラッシュ付き)があり、実際の検索 URL は `/search?...`(スラッシュなし)なのでパターン上は該当しないが、禁止の意図がある可能性が高い。**ユーザー決定 2026-10-04: 30 秒間隔で夜間のみ取得**(Crawl-delay を守る。取るのは夜間の CronJob だけ)。
- あみあみ: robots.txt が 404(制限の記載なし)。www.amiami.jp は curl の UA だと Cloudflare に 403 でブロックされた(検索は slist.amiami.jp なら 200)。
- ドラゴンスター: robots.txt も検索も Cloudflare の "Just a moment..." チャレンジで 403。回避は試みていない。**scrape 不可の可能性が高く、headless でも突破できるかは未確認**。
- プレミアムバンダイ: `Disallow: /search/` と `/search_bst/`。検索結果ページは機械取得禁止。リンクのみ(link_only)なら問題なし。
- 魂ウェブ: robots.txt は 404(制限の記載なし)。
- ポケモンセンターオンライン: 全許可。ただしアクセス集中時は待機室(wr.pokemoncenter-online.com)へ 302 される。

## 詳細

### メルカリ(headless)
- 検索 URL は 200。静的 HTML は Next.js の骨格で、`data-testid="item-cell-skeleton"` が 15 個あるだけ。商品名・価格は HTML に無い(検索語以外の商品語が 0 件)。`__NEXT_DATA__` も無い。
- 商品は JS が別途 API を呼んで描画する。**headless(chromedp)が必要**。セレクタは JS 実行後でないと確認できず**未確認**(`data-testid` ベースになる見込みだが推測)。
- 内部 API への直接アクセスは robots の /v1/ /v2/ 禁止に触れうるため試していない。

### Yahoo!フリマ(scrape。埋め込み JSON)
- 静的 HTML(SSR)に `<script id="__NEXT_DATA__" type="application/json">` があり、商品が入っている。headless 不要。
- JSON パス: `props.initialState.searchState.search.result.items[]`
  - タイトル `title` / 価格 `price`(数値、円) / 商品 ID `id`(URL は `https://paypayfleamarket.yahoo.co.jp/item/{id}`。HTML 上の href は `/item/{id}`) / 画像 `thumbnailImageUrl` / 状態 `itemStatus`(確認できた値は `OPEN` のみ。売り切れの値は未確認) / `condition`(`new` 等)。
- 並び順は `rewriteQueryType: "VECTOR"`(関連度)。robots で sort 指定が禁止のため、最安順では取れない。上位 20 件を取ったあとクライアント側で並べる。
- goquery では `script#__NEXT_DATA__` のテキストを JSON パースする。fixture: yahoofurima.fixture.html
- 価格表記は円の数値(税込かは未確認)。

### カードラッシュ DM(scrape)
- 検索フォームのパラメータ: `keyword`(語)、`order`(`asc`=価格の安い順 / `desc` / `featured` / `rank`、空=関連度)、`available=1`(在庫あり)、`num`(10/20/30/50/100、既定 100)、`page`。
- `order=asc&available=1&num=20` で 20 件返ること、価格が昇順に見えることを確認した。
- 1 件分: `li.list_item_cell > div.item_data`(`data-product-id`)
  - URL: `a.item_data_link` の `href`(絶対 URL)
  - タイトル: `.item_name .goods_name`(検索語が `<b>` で強調されるので text() で取る)。img の `alt` にも同じ名前
  - 価格: `.selling_price .figure` → 「50円」「1,280円」。`.tax_label` が「(税込)」
  - 画像: `.global_photo img` の `src`(`data-x2` は 2 倍画像)
  - 在庫: `.stock` → 「在庫数 63枚」。`available=1` なら在庫ありのみ
- カートボタン `.cartinput` は無視。fixture: cardrush.fixture.html
- 注意: 「ボルシャック」で 50 円のカードが多数あり、1 カードに複数のレアリティ商品が並ぶ。目安は商品名の正規化が重要。

### ドラゴンスター
- 未確認。Cloudflare のチャレンジで 403。検索 URL・構造とも確認できていない。

### あみあみ(scrape)
- 人が開く検索は `https://slist.amiami.jp/top/search/list?s_keywords={q}`(トップのフォーム action)。www.amiami.jp 側の同パスは curl だと 403 になった。
- 並び替え `s_sortkey`: `pricea`(価格が安い)/ `priced` / `regtimed`(新着)。`s_sortkey=pricea` を付けた検索で、価格が昇順に並ぶことを確認した(160, 190, 190, ... 528)。
- 絞り込みフォームのキー(未検証。URL に付けた挙動は未確認): `s_st_list_preorder_available` / `s_st_list_backorder_available` / `s_st_list_newitem_available` / `s_st_condition_flg`(中古等)。
- 1 件分: `div.product_table_list > div.product_box`
  - URL: `a`(box 直下)の `href`(絶対 URL。`gcode` 付き)
  - タイトル: `.product_name_inner`
  - 価格: `.product_price` → 「8,080」(円記号なし。税込かは未確認)
  - 画像: `.product_img img` の `data-src`(`src` は blank.gif。lazyload)
  - 在庫: 確認できた手掛かりは `.product_icon` 内の `icon_preowned`(中古)のみ。在庫あり/なしの表示は検索一覧からは確認できず**未確認**(詳細ページ側にある可能性)。
- fixture: amiami.fixture.html

### 駿河屋(scrape。ユーザー決定 2026-10-04: 30 秒間隔で夜間のみ取得)
- フォーム項目: `category`(空可)、`search_word`、`adult_s`。
- 並び替えは select `rankBy`: `price:ascending`(値段が安い順)/ `price:descending` / `modificationTime:descending` / `release_date(int):descending` / `relavancy(int)`(関連順)。在庫ありは `inStock=On`(ページ内リンクの href より)。
- ただし `rankBy=price:ascending&inStock=On` を付けた 2 回目の取得でも品切れ表示が並んだため、**パラメータが効いているかは未確認**(検索語「グリス」では在庫のある商品自体が少なかった可能性もある)。
- 1 ページは `div.item_box` が 6 個、その中に `div.item` が複数(1 box = 複数商品)。1 件 = `div.item`。
  - URL: `.photo_box a` または `.title a` の `href`(相対。`/product/detail/{id}?tenpo_cd=` や `/product/other/{id}`)
  - タイトル: `.title h3.product-name`
  - 画像: `.photo_box img` の `src`(`https://www.suruga-ya.jp/database/photo.php?shinaban={id}&size=m`)
  - 価格: `.item_price .price_teika` → 「中古：￥500 税込」「定価：￥660」。新品の販売価格欄 `.item_price .price` は「品切れ」の表示を確認(在庫のある新品の表記は未確認)。中古価格は `.price_teika strong` の「￥500」(税込)。
  - 在庫: `.price` が「品切れ」なら新品は在庫なし。
- fixture: surugaya.fixture.html
- Crawl-delay 30 を守り、取得は夜間の CronJob だけ(ユーザー決定 2026-10-04: 30 秒間隔で夜間のみ取得)。403 が返るようなら取得せずリンクだけにする判断を人に仰ぐ。

### Yahoo!ショッピング
- 人が開く URL: `https://shopping.yahoo.co.jp/search?p={q}` は `/search/{q}/0/` にリダイレクトされ、クエリ(sort)は落ちた。ページ内の並び替えリンクは `https://shopping.yahoo.co.jp/search/{q}/0/?X=2`(価格が安い順)、`X=12`(価格+送料が安い順)。
- 参考: 同ページにも `__NEXT_DATA__` があり、`props.initialState.bff.searchResults.items` に商品(`name`, `price`, `url`, `image`)があった。ただし価格は公式 API で取る方針なので、ここは使わない。

### プレミアムバンダイ(link_only)
- トップの検索フォームは `action="/search_bst/"`、入力名 `q`(隠し項目 `C5` は空)。URL: `https://p-bandai.jp/search_bst/?q={q}`。robots は /search_bst/ を禁止しているので、リンクとして人が開く用途のみ。結果ページの表示は未確認。
- トップページの文字コードは Shift_JIS 系に見えた(日本語が文字化け)。検索語のエンコードが UTF-8 でよいかは**未確認**。

### 魂ウェブ(link_only)
- ヘッダーのフォーム: `action="/item/"`、入力名 `wo`(商品検索)。サイト内検索は `/search?q=`(別機能)。
- `/item/?wo=グリス` は 200 だが、静的 HTML に「グリス」を含む商品が無かった(JS 描画の可能性)。**検索が機能するかは未確認**。

### ポケモンセンターオンライン(link_only)
- `https://www.pokemoncenter-online.com/?word={q}&main_page=search_result` は待機室(wr.pokemoncenter-online.com)へ 302。転送先の target URL に検索語が引き継がれていた。結果ページ自体は未確認。
- 待機室が出ている間は常時これになる可能性がある。

## 判断待ち・人が登録するための候補

- **駿河屋**:ユーザー決定 2026-10-04: 30 秒間隔で夜間のみ取得(robots.txt の `Crawl-delay: 30`)。登録表に入れ(`NewSurugaya`)、初期データ(migration 000004)は scrape・基準サイト
- 初期データ(000004)に入れたのは、上の一覧で確認済みの Yahoo!フリマ・カードラッシュ・あみあみ・Yahoo!ショッピング・駿河屋。URL は一覧のテンプレートと同じ文字列
- 次の 3 サイトは未確認の点があるので初期データに入れていない。人が設定画面から登録する場合の候補(いずれも `link_only`、`is_reference` は false):
  - プレミアムバンダイ `https://p-bandai.jp/search_bst/?q={q}`(検索ページは robots で禁止のためリンクのみ。検索語の文字コードが UTF-8 でよいか未確認)
  - 魂ウェブ `https://tamashiiweb.com/item/?wo={q}`(検索が機能するか未確認)
  - ポケモンセンターオンライン `https://www.pokemoncenter-online.com/?word={q}&main_page=search_result`(結果ページ未確認。待機室へ 302 されうる)
- ドラゴンスターは URL も未確認(403)なので候補にも載せない

## 取得方式の見立て(まとめ)
- scrape(静的 HTML): カードラッシュ、あみあみ(slist ホスト)、駿河屋(30 秒間隔・夜間のみ)
- scrape(埋め込み JSON): Yahoo!フリマ(sort 指定は禁止のため付けない)
- headless: メルカリ、ドラゴンスター(要チャレンジ突破。可否未確認)
- api: Yahoo!ショッピング
- link_only: Amazon、プレバン、魂ウェブ、ポケセン
