# headless Chromium での通販サイト確認(2026-10-04 手動)

環境: docker `chromedp/headless-shell:151.0.7922.109`(Chromium 151、UA は既定のまま。ステルス・UA 偽装なし)を `-p 9222` で起動し、Go の chromedp(RemoteAllocator)で接続。
`--dump-dom` はイメージ既定の ENTRYPOINT だと終了せず使えなかった。chromedp 接続が確実。ブラウザは `--shm-size=1g` 付き。
実 DOM は保存していない(一時ファイルは削除、コンテナ停止済み)。fixture は構造だけの架空データ。同一サイトへは 10 秒以上あけ、並列なし。
注意: 検索語は「グリス」「ボルシャック」「ピカチュウ」。ブラウザの Accept-Language が既定(英語)のため、魂ウェブは WOVN で英語化された。

## メルカリ(headless。確認できた)
- 検索 URL: https://jp.mercari.com/search?keyword={q}&status=on_sale&sort=price&order=asc(リダイレクトなし。sort 指定が効き、先頭 4 件は同額 ¥300 で昇順に見えた。全体の昇順は未確認)
- 待つ要素: `[data-testid="item-cell"]` は**スケルトンにも付く**(113 個中、描画済みは 6 個前後)ので判定に使えない。
  **`li[data-testid="item-cell"] a[data-testid="thumbnail-link"]`(または `[data-testid="item-tile-price"]`)が 1 つ以上出るまで** WaitVisible する。ナビゲート後 約 3 秒で出た。
- 描画完了の判定(全件): 未確認。3 秒時点では先頭数件のみ描画で、残りはスケルトン(lazy 描画)。スクロールで残りが埋まるかは未確認(アクセス回数の上限のため再確認していない)。
  実装案: 上位 20 件に必要なので、描画済みセル数が 20 に届くか、数回スクロールしても増えなくなるまで(上限 ~10 秒)待つ。
- 1 件分(goquery): `li[data-testid="item-cell"]`(`a[data-testid="thumbnail-link"]` を持つものだけ有効。持たないセルはスケルトンとして捨てる)
  - URL: `a[data-testid="thumbnail-link"]` の `href`(相対。`/item/m<数字>` が個人出品、`/shops/product/<ID>` がメルカリ Shops。`https://jp.mercari.com` を前置)
  - タイトル: 専用要素なし。同リンク内 `img` の `alt` から末尾の「のサムネイル」を除く
  - 価格: `[data-testid="item-tile-price"]` の text(`¥300` 形式。`¥` とカンマを除いて数値化)
  - 画像: `a[data-testid="thumbnail-link"] img` の `src`(`loading=lazy`)
  - 在庫: `status=on_sale` で絞っているので在庫あり扱い。売り切れ表示の有無は未確認
- 注意: class はハッシュ化されていて変わりやすいので使わない。`data-testid` ベース(変更リスクは残る)。Shops の商品も混ざる。`__NEXT_DATA__` なし(前回)。
- アクセス回数: 描画後 DOM の取得ページ読み込み 1 回成功。ほかに --dump-dom の試行 2 回(出力 0 バイト。実際に読み込めたかは不明)。合計 3 回扱い。
- fixture: mercari.fixture.html

## ドラゴンスター(取得不可)
- 検索 URL(参考): https://dorasuta.jp/dm/product-list?keyword={q}
- headless でも Cloudflare Turnstile のチャレンジ("Just a moment...")。8 秒待っても通過せず。回避は試みない。**取得不可**。
- 検索 URL としても有効かは未確認。link_only にするなら URL の正しさが未確認なので登録は見送りが妥当。
- アクセス回数: 1

## プレミアムバンダイ(未取得・リンクのみ)
- https://p-bandai.jp/search_bst/?q={q}。robots.txt(前回確認)で `/search_bst/` が禁止のため取得していない(アクセス 0)。
- 人がブラウザで開くリンクとしては使える想定だが、結果の表示・検索語の文字コード(UTF-8 でよいか)は未確認。

## 魂ウェブ(リンクのみ。確認できた)
- https://tamashiiweb.com/item/?wo={q}: 描画後に検索結果が出る。Accept-Language が英語だと `?wovn=en&wo=...` に転送され英語表示(人が日本語ブラウザで開けば日本語の想定だが未確認)。
- 「グリス」で 17 件(`.resultNum em`)。ただし曖昧一致(「クリスタル…」等も混在)。「仮面ライダーグリス」系も含まれる。
- 1 件: `ul.productList__items > li.productList__item`、リンク `a`(絶対 URL)、名前 `.productList__name`、価格 `.productList__price em`(「¥7,700」)、画像 `.productList__img img`。link_only なので使わないが参考。
- 待つ要素: 10 秒以内に DOM に出た(待機要素の厳密な計測は未実施。固定 8 秒待ちで描画済み)。
- アクセス回数: 1
- fixture: tamashii.fixture.html

## ポケモンセンターオンライン(リンクのみ。URL を修正)
- 待機室(行列)は今回出なかった(待機室の混雑時挙動は別。出れば 302 になる点は前回確認)。
- **`?word={q}&main_page=search_result` は検索結果にならず、トップページが表示された**(「検索結果」文字列なし、検索フォームは `action="/search/"`、入力名 `q`)。この URL テンプレートは不適。
- **推奨 URL: `https://www.pokemoncenter-online.com/search/?q={q}`**(フォーム定義と実際の表示から)。「ピカチュウ」で `?prefn1=refCd&prefv1=P_PIKACHU` に転送され「1 ~ 40 / 3,848 件」と一覧が表示された。ポケモン名は絞り込み条件に変換される。名前以外の語や並び替え(「価格の安い順」の存在は確認、URL パラメータは未確認)は未確認。
- 結果欄の class(`item`/`product` 等)は確認が粗くセレクタは未確認(link_only なので不要)。
- アクセス回数: 2(旧 URL 1 + 新 URL 1)
- fixture: pokecenter.fixture.html(構造は未確認のため注記のみ)
