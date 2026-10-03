# wishlist フェーズ4 の受け入れ条件

仕様の正は [../CLAUDE.md](../CLAUDE.md)(§4 正規化・§12 フェーズ4)、進め方の決定は
[docs/ai-shared/decisions/2026-10-04-wishlist-phase4-plan.md](../../../docs/ai-shared/decisions/2026-10-04-wishlist-phase4-plan.md)。
API 契約は [../api/openapi.yaml](../api/openapi.yaml)。書き方は [phase3-api-spec.md](phase3-api-spec.md)・[phase3-web-spec.md](phase3-web-spec.md) に合わせる。
テストは `make wishlist-test`(MySQL 実装と migration は `make wishlist-test-mysql`)。

## 4-1 表記揺れの辞書

ジャンルごとに「同じものを指す語の集合」(別名グループ)を持ち、参考外の判定(title_mismatch)で同一視する。
例:S.H.Figuarts ジャンルの `[S.H.Figuarts, SHフィギュアーツ]` があれば、商品名「S.H.Figuarts グリス」と
出品「SHフィギュアーツ 仮面ライダーグリス」は一致する。iOS の設定画面への反映は後のタスク(このタスクでは iOS を変えない)。

### 決めたこと(仕様に書かれていなかった部分。既定案)

- **単位と形**:辞書はジャンルに属する別名グループの配列 `[][]string`。API では `Genre.aliases`(例 `[["HG","ハイグレード"],["MG","マスターグレード"]]`)。
  グループの順・グループ内の語の順は保存した順のまま返す
- **照合の規則(internal/estimate)**:
  - name を Unicode の空白で分けたトークンごとに、`AliasVariants(token, groups)` で「タイトルに含まれていれば一致とみなす語」を作る
  - トークンを `query.Normalize` した値と**完全一致**する語(正規化後)を持つグループに属するとし、そのグループの全語の正規化を返す(グループ内の順・重複を除く・正規化して空の語は除く)。
    部分一致ではグループに属さない(`HGUC` は `HG` のグループに属さない)。属さなければトークン自身の正規化だけ。正規化して空のトークンは空(従来どおり無視)
  - 各トークンについて、返した語のどれかが正規化タイトルに部分文字列として含まれれば一致。全トークンが一致すれば title_mismatch にしない
  - 辞書が nil・空なら `TitleMatches` と同じ(フェーズ3 の AC-E1 は変えない)
  - 辞書は **name のトークンにだけ**効く。ジャンルの検索ワードのテンプレート(`S.H.Figuarts {name}` の固定部分)は照合に使わない(フェーズ3 と同じ。照合は name だけ)
  - 基準価格の計算(Evaluate の 1 段目)も同じ照合を使う
- **API**:`estimate.Item` に `Aliases [][]string` を足し、`Judge`・`Evaluate` はそれを使う。純関数 `AliasVariants`・`TitleMatchesWithAliases(name, title, groups)` を足す。既存の `TitleMatches(name, title)` は残す(辞書なし)
- **検査(item.Service。違反は ErrInvalid → 422)**:
  - 各語は前後の空白(全角空白・タブを含む)を除いて保存する
  - 除いたあと空の語、`MaxAliasLen`(64 文字・rune)を超える語、`query.Normalize` して 2 文字(rune)未満になる語(`・・` など。短い語は部分一致でほとんどのタイトルに当たるため。HG・MG・RG の 2 文字は通す。`MinAliasNormalizedLen`)、区切り文字(`,` `，` `、`)を含む語(PWA が 1 行をカンマ区切りで編集するため)
  - 2 語未満のグループ(空のグループを含む)
  - 正規化後の語がジャンル内で重複する(同じグループ内・別グループとも)
  - グループ数・語数の上限は設けない(本文の大きさの上限 `limitBody` に任せる)
- **Repository**:正規化後の重複だけを守る(ErrInvalid。不可分で、同じ呼び出しの他の項目も変えない)。空の語・グループの大きさは Service が検査する。
  作成で渡さなければ辞書なし、更新(`GenrePatch.Aliases`)は nil なら変えない・空スライスなら全部消す・値なら全件置き換え。読み出しは無ければ長さ 0
- **DB(migration `000005_genre_aliases`)**:

  ```sql
  CREATE TABLE IF NOT EXISTS genre_aliases (
    id BIGINT NOT NULL AUTO_INCREMENT,
    genre_id BIGINT NOT NULL,
    group_no INT NOT NULL,            -- ジャンル内のグループの順(0 始まり)。グループ内の語の順は id 昇順
    alias VARCHAR(64) NOT NULL,       -- 入力された語(前後の空白を除いたもの)
    normalized VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL, -- query.Normalize(alias)
    PRIMARY KEY (id),
    UNIQUE KEY uq_genre_aliases_normalized (genre_id, normalized),
    CONSTRAINT fk_genre_aliases_genre FOREIGN KEY (genre_id) REFERENCES genres (id) ON DELETE CASCADE
  ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;
  ```

  - 指示の素案の `group_key` は、グループの並びを保つため `group_no`(整数の順)にした
  - `normalized` を `utf8mb4_bin` にするのは、DB 既定の `utf8mb4_0900_ai_ci` だと濁点(ガ/カ)・ひらがなとカタカナを同一視し、
    正規化後に違う語を重複として弾いてしまうため(契約テスト `AliasDuplicates` と `TestGenreAliases000005_Constraints` で確かめる)。
    `normalized` の幅は NFKC で語が伸びうるため 255
  - NFKC 後の小文字化などは Go の `query.Normalize` で行い、SQL では正規化しない(seed の normalized はリテラルで書き、MySQL テストで `query.Normalize(alias)` と一致を確かめる)
- **初期データ(000005 の seed。既定案)**:「フィギュアーツ」単独は入れない(部分一致のため「フィギュアーツZERO」「Figuarts mini」等の別シリーズまで一致扱いになり、不正確な目安になる)。NOT EXISTS は語の単位で判定し、golang-migrate は 1 回しか流さない前提。ジャンル名で引き(id を書かない)、無いジャンルは飛ばす。同じ `(genre_id, normalized)` があれば足さない(もう一度流しても壊れない。000004 と同じ作り方)。
  ジャンル・サイト・紐づけは足さない・変えない。down は `DROP TABLE IF EXISTS genre_aliases` だけ

  | ジャンル | グループ |
  |---|---|
  | S.H.Figuarts | `S.H.Figuarts, SHフィギュアーツ` |
  | ガンプラ | `HG, ハイグレード` / `MG, マスターグレード` / `RG, リアルグレード` |

- **更新(internal/refresh)**:判定のたびに、商品のジャンルの `Aliases` を `estimate.Item.Aliases` に渡す(ジャンルは既に `ListGenres` で引いている。キャッシュしない)
- **OpenAPI**:`AliasGroups`(`array` of `array` of `string`)を足し、`Genre`・`GenreCreate`・`GenreUpdate` に `aliases` を足す。
  `Genre.aliases` は **required にしない**(iOS のコミット済みの生成コードと、そのテストの JSON が aliases を持たないため。iOS は後のタスク)。
  ただしサーバーは**常に返す**(無ければ `[]`)。長さ・個数の制約は schema に書かず(oapi-codegen の strict server は検査しない)、description と Service の検査で 422 にする。
  形が違う本文(`["HG"]`・`[[1]]`)は従来どおり 400 bad_request
- **PWA の設定画面**:ジャンルのダイアログに別名グループの欄を足す
  - 1 グループ = 1 行のテキスト欄(アクセシブルな名前 `別名グループN`、N は 1 始まり)。既存のグループは `, ` でつないで表示する
  - 「別名グループを追加」で空の行を足し、「別名グループNを削除」で行を消す(番号は詰める)
  - 区切りは半角カンマ `,`・全角カンマ `，`・読点 `、`。各語の前後の空白(全角を含む)を除き、空の語は捨てる。語の中の空白は残す(`マスター グレード`)
  - 空の行は送らない。1 語だけの行があれば alert(文言に「2語以上」)を出して API を呼ばない。正規化後の重複は API の 422 をそのまま表示する
  - 編集は、読み直した辞書が元(`genre.aliases ?? []`)と違うときだけ `aliases` を送る(他の項目と同じ「変えた項目だけ」)。作成は常に `aliases` を送る
  - 応答に `aliases` が無い(古いサーバー)ジャンルは辞書なしとして扱う
  - 行の読み書きは純関数 `src/lib/aliases.ts`(`parseAliasLine`・`formatAliasGroup`・`parseAliasGroups`)

### 受け入れ条件とテスト

| ID | 条件 | テスト |
|---|---|---|
| AC-A1 | 作成で渡した別名グループを順のまま返す(作成結果・一覧)。渡さなければ長さ 0。別のジャンルなら同じ語でもよい | `internal/item/itemtest/alias_contract.go` の `CreateGenreWithAliases`(`RunRepositoryContract` のサブテスト。メモリ `TestMemoryRepositoryContract`・MySQL `TestMySQLRepositoryContract`) |
| AC-A2 | 更新で渡すと全件置き換え・省略で変えない・空で全部消す。存在しないジャンルは ErrNotFound。他のジャンルは変えない | 同 `UpdateGenreAliases` |
| AC-A3 | 正規化後の重複(同じグループ・別グループ・大文字小文字・全角・記号・半角カナ)は ErrInvalid で何も変えない(作成・更新)。正規化後に違う語(濁点・かなの違い)は通る | 同 `AliasDuplicates` |
| AC-A4 | `AliasVariants`:トークンと正規化で完全一致する語を持つグループの語(正規化・順・重複除去・空を除く)。属さなければ自身だけ。部分一致では属さない | `internal/estimate/alias_test.go` の `TestAliasVariants` |
| AC-A5 | 辞書つきのタイトル照合(指示の 2 例を含む)。辞書が nil・空なら `TitleMatches` と同じ | `TestTitleMatchesWithAliases`・`TestTitleMatchesWithAliases_NoAliasesSameAsBefore` |
| AC-A6 | `Judge` は `Item.Aliases` で title_mismatch を判定する | `TestJudge_Aliases` |
| AC-A7 | `Evaluate` の基準価格も辞書で照合する。仕様 §6 の例は辞書があっても同じ結果 | `TestEvaluate_Aliases` |
| AC-A8 | Service の検査(空・空白だけ・1 語・空グループ・空白を除いて重複・64 文字超・正規化して空・正規化後の重複は ErrInvalid。64 文字ちょうどは通る。前後の空白は除いて保存) | `internal/item/service_alias_test.go` の `TestService_GenreAliasesInvalid`・`TestService_GenreAliasesValid` |
| AC-A9 | 更新(refresh)は商品のジャンルの辞書で判定し、辞書を変えると次の更新から結果が変わる | `internal/refresh/refresh_alias_test.go` の `TestRefreshItem_UsesGenreAliases` |
| AC-A10 | API:Genre は常に `aliases`(無ければ `[]`)。POST で保存、PATCH で全件置き換え・省略で変えない・`[]` で消す。一覧にも出る | `internal/httpapi/server_alias_test.go` の `TestGenreAliases` |
| AC-A11 | API:空の語・1 語・正規化後の重複・長すぎる語は 422 unprocessable(POST・PATCH。何も変えない)。形が違う本文は 400 | `TestGenreAliases_Errors` |
| AC-A12 | migration 000005:表の形(IF NOT EXISTS・CASCADE・一意制約・utf8mb4_bin)、seed(既定の辞書・normalized が Normalize と一致・もう一度流しても増えない・無いジャンルは飛ばす)、down は表だけを消す | `migrations/aliases_layout_test.go` の `TestGenreAliases000005Shape`、`migrations/aliases_mysql_test.go`(`-tags mysql`)の `TestGenreAliases000005_Fresh`・`_NotAppliedTwice`・`_MissingGenre`・`_Down` |
| AC-A13 | DB:ジャンルを消すと辞書も消える。同じジャンルで normalized は一意、濁点・かなの違いは別の語 | `TestGenreAliases000005_Constraints`(`-tags mysql`) |
| AC-A14 | PWA:1 行のカンマ区切り(`,`・`，`・`、`)の読み書き、空の行の除外と 1 語の行の検出 | `web/src/lib/aliases.test.ts` |
| AC-SET-08 | 編集ダイアログに既存の別名グループが 1 行ずつ `, ` 区切りで出る | `web/src/App.aliases.test.tsx` |
| AC-SET-09 | 行の編集・追加・削除(番号を詰める)で、PATCH は `aliases` だけを全件置き換えで送る。全部消すと `aliases: []` | 同 |
| AC-SET-10 | 1 語だけの行があると alert(「2語以上」)を出し、API を呼ばない | 同 |
| AC-SET-11 | ジャンルの追加でも `aliases` を送る(空の行は除く) | 同 |
| AC-SET-12 | `aliases` の無い古い応答でも開け、別名を変えなければ `aliases` を送らない(既存の AC-SET-04 の `{ site_ids }` だけの PATCH も変わらない) | 同 |

既存のテストは変えていない(`RunRepositoryContract` の末尾に `runAliasContract` の呼び出しを 1 行足しただけ)。
フェーズ3 の `TestTitleMatches` の「カタカナ表記は辞書が無いので一致しない」は、辞書なしの `TitleMatches` の条件としてそのまま残る。

### 実装の範囲(implementer 向けの目安)

- `migrations/000005_genre_aliases.{up,down}.sql`
- `internal/store/query.sql` に genre_aliases の読み書きを足して `make wishlist-gen`(sqlc)
- `internal/item`:メモリ・MySQL の Repository(CreateGenre・UpdateGenre・ListGenres)、Service の検査
- `internal/estimate`:`AliasVariants`・`TitleMatchesWithAliases` の実装、`Judge`・`Evaluate` で `Item.Aliases` を使う
- `internal/refresh`:`estimate.Item{..., Aliases: genre.Aliases}`
- `internal/httpapi`:`toAPIGenre` で常に `aliases` を出す、Create・Update で `Aliases` を渡す
- `web/src/lib/aliases.ts`・`web/src/ui/Settings.tsx`(ジャンルのダイアログ)

## 4-2 価格の推移

取得のたびにサイト別の目安(low・mid)を 1 商品×1 サイト×1 日(JST)1 行で残し、詳細シートの折りたたみ「価格の推移」に日ごとの推移を折れ線で出す。
PWA は SVG(新しい npm 依存なし)、iOS は Swift Charts。出品が無い日(no_result)・失敗した日(failed)は値を作らず、点を打たない。

### 決めたこと(仕様に書かれていなかった部分。既定案)

- **保存の場所**:`SaveSiteResult` が、`Status == ok` かつ `Low != nil` のときだけ、**同じトランザクションで** `price_history` を upsert する
  (目安と推移が食い違わない。refresh 側に呼び出しを足さない)。no_result・`MarkFailed`・検査で弾いた保存(ErrInvalid・ErrNotFound)は行を作らず、同じ日の既存の行も変えない
- **日付**:`item.HistoryDay(fetched_at)` = JST の日付を 00:00 UTC の `time.Time` で表す(MySQL の DATE を parseTime で読んだ値と同じ形)。同じ日は最後の値で上書き、`recorded_at` は保存した fetched_at(秒未満切り捨て・UTC)
- **DB(migration `000007_price_history`)**:

  ```sql
  CREATE TABLE IF NOT EXISTS price_history (
    item_id BIGINT NOT NULL,
    site_id BIGINT NOT NULL,
    day DATE NOT NULL,          -- JST の日付
    low INT NOT NULL,           -- 点を作るのは low がある日だけ
    mid INT NULL,               -- 件数 3 未満の日は null
    count INT NOT NULL,
    recorded_at DATETIME NOT NULL,
    PRIMARY KEY (item_id, site_id, day),
    KEY idx_price_history_day (day),   -- 古い行の削除用(既定案。形の検査はしない)
    CONSTRAINT fk_price_history_item FOREIGN KEY (item_id) REFERENCES items (id) ON DELETE CASCADE,
    CONSTRAINT fk_price_history_site FOREIGN KEY (site_id) REFERENCES sites (id) ON DELETE CASCADE
  ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;
  ```

  down は `DROP TABLE IF EXISTS price_history` だけ。seed は持たない
- **版番号の欠け(想定内)**:000006 は別の PR #601 で入る。それが main に入るまでは `migrations.TestLayout` が「版 000006 の up が無い」で落ちる。
  golang-migrate は欠けを飛ばして進むので、MySQL のテスト(`mustMigrate(t, m, 7)`)と実環境の migrate は動く。#601 のあとに取り込めば TestLayout も通る
- **保持期間と削除**:`item.PriceHistoryRetentionDays = 180`(今日を含む 180 日)。`RefreshAll`(CronJob)の**最初に** `PrunePriceHistory(今日 − 179 日)` で、それより前の日の行を全商品から消す。
  時計は `refresh.Deps.Now`。削除の失敗はログに出して更新を続ける(更新は止めない)。API の読み出し・手動の更新(RefreshItem)では消さない(読み出しは期間で絞るので、消し遅れても表示は変わらない)
- **読み出し(internal/refresh)**:`Service.PriceHistory(ctx, itemID, days)`。期間は今日(JST、`Deps.Now`)を含む直近 `days` 日(since = 今日 − (days−1) 日)。
  商品が無ければ `item.ErrNotFound`、days が 1〜180 の外なら `item.ErrInvalid`。
  `Sites` は点のあるサイトだけ、商品のジャンルの `site_ids` の順(link_only・enabled=false も順の決定には使う)、ジャンルに無いサイトはその後に site_id 昇順。点は day 昇順。
  `Overall` は点のある日ごとの全サイトの low の最小(day 昇順)。組み立ては純関数 `refresh.BuildHistory(points, siteOrder)`。Sites・Overall は空でも nil にしない
- **API**:`GET /api/items/{id}/price-history?days=`(既定 90・最大 180)。本文は `PriceHistory{item_id, days, sites:[{site_id, points:[{day, low, mid}]}], overall:[{day, low}]}`、
  day は `YYYY-MM-DD`(OpenAPI の `format: date`)、mid は null を許す。days が範囲外・数でない、id が 0 以下・数でない → 400 bad_request(handler が検査する。Service の ErrInvalid → 422 にはしない)、商品なし → 404。
  `httpapi.Estimator` に `PriceHistory` を足した(実装は `*refresh.Service` だけ)
- **PWA(詳細シート)**:
  - サイト行・参考外のあとに `<details>`、`<summary>価格の推移</summary>`。**開いたときに** `getPriceHistory(itemId)`(days を付けない)を 1 回。シートを開いている間は閉じて開き直しても取り直さない
  - 中身:読み込み中は `読み込み中…`(文字だけ)/ 全体の最安の点が 2 未満なら `推移はまだありません` / 通信失敗は `オフライン`、それ以外の失敗は `価格の推移を取得できませんでした`(どちらも `role="alert"` にしない。シートの他の部分は壊さない)
  - グラフ:`<svg role="img" aria-label="価格の推移 7/6〜10/3 最安 ¥2,000 最高 ¥3,200">`(`historyLabel(overall)`)。線は `<path data-series="overall|site-<id>" stroke="var(--chart-…)">`、
    実際の点ごとに `<circle data-series=…>`、軸の文字は最初と最後の日(`M/D`)と最大・最小(`¥` 表記)を `<text>` で。`<animate>` 等は使わない
  - 凡例:`role="group" aria-label="凡例"` の中に、推移の `sites` の順でサイトごとの `<button aria-pressed>`(名前はサイト一覧から、無ければ `サイト<ID>`)。**既定はどれも押されていない = 全体の最安だけ**。押すとそのサイトの線(low)を足し、もう一度で消す
  - 色:`styles.css` の最初の `:root` と `@media (prefers-color-scheme: dark)` の両方に `--chart-overall`・`--chart-site-1`〜`--chart-site-4`・`--chart-axis` を定義する。サイトの線は推移の `sites` の順で `--chart-site-((i % 4) + 1)`
  - 座標:純関数 `src/lib/history.ts` の `buildChart(series, box)`。全系列で共通の軸(x は最初の日〜最後の日を日数で等分、y は最小〜最大)。
    期間 1 日なら x は内側の中央、値が 1 種類なら y は内側の中央。**1 日より空いたところは `M` で線を切る**(出品の無い日をつないで値を作らない)。座標は 0.1 px に丸め、末尾の `.0` は付けない。点が 0 なら null。
    Sheet は `CHART_BOX`(320×160)を使う。`dayLabel`(`2026-10-03` → `10/3`)・`historyLabel` も同じファイル
- **iOS**:
  - `WishlistService.priceHistory(itemID:days:)`(API 実装と Fake。Fake は `setPriceHistory(_:)`・`Call.priceHistory(itemID:days:)`、未設定なら推移なし)。ドメイン型 `PriceHistory`・`SitePriceHistory`・`PricePoint`・`DayLow`(day は API の文字列のまま)
  - グラフ用の純関数 `PriceHistoryFormat`(`date(fromDay:)` = JST 0:00 の Date・読めない日付は nil、`dayLabel`、`points`(day 順・読めない日付は捨てる・1 日より空いたら `segment` を進める)、`accessibilityLabel`、`chart(_:sites:)`)。
    Swift Charts の `LineMark(series:)` には「系列の id + segment」を渡して線を切る。`PointMark` で実際の点を打つ
  - `ItemDetailViewModel`:`priceHistory`(`PriceHistoryState`:notLoaded / loading / empty / loaded(chart) / failed(文言))・`loadPriceHistory()`(取得中・取得済みなら何もしない。失敗のあとは取り直す。目安価格の状態は変えない)・
    `toggleHistorySite(_:)`・`historyLegend`・`visibleHistorySeries`(全体の最安 + 選んだサイトを凡例の順)・`selectedHistorySiteIDs`
  - View(ItemDetailSheet):参考外のあとに自前の折りたたみ(参考外と同じ作り)。開いたときに `loadPriceHistory()`。accessibilityIdentifier は
    `historyDisclosure`(ラベル「価格の推移」)・`priceHistoryChart`(`.accessibilityElement(children: .ignore)`、ラベル = `chart.accessibilityLabel`)・
    `historySite-<siteID>`(凡例のボタン。ラベル = サイト名、値 = `表示中` / `非表示`)・`historyEmpty`(ラベル「推移はまだありません」)・`historyMessage`(読み込み中・失敗の文)
  - モック起動:`WISHLIST_USE_FAKE=estimates` に `WishlistFixtures.priceHistories`(商品 12 = メルカリ 3 日分、商品 11 = 推移なし)を入れた
- 常時動くアニメーションは入れない(Swift Charts の既定の描画のまま。`.animation(…repeatForever…)`・`ProgressView` を使わない)

### 受け入れ条件とテスト

テストは Go が `make wishlist-test`(MySQL は `make wishlist-test-mysql`)、Web が `web/` で `npm test`、iOS が `make wishlist-ios-test`(XC は XCUITest)。

| ID | 条件 | テスト |
|---|---|---|
| AC-H1 | `HistoryDay`:JST の日付を 00:00 UTC で返す(UTC 15:00 は翌日・年またぎ・他のタイムゾーン) | `internal/item/history_test.go` の `TestHistoryDay` |
| AC-H2 | ok の保存は (商品, サイト, JST の日) を upsert。同じ日は最後の値(mid が null になる値も)、翌日(JST)は行が増える。recorded_at は秒未満切り捨て | `internal/item/itemtest/history_contract.go` の `HistorySavedOnOK`(`RunPriceRepositoryContract` のサブテスト。メモリ `TestMemoryPriceRepositoryContract`・MySQL `TestMySQLPriceRepositoryContract`) |
| AC-H3 | no_result・MarkFailed・low の無い ok・弾いた保存は行を作らず、同じ日の既存の行も変えない | 同 `HistoryOnlyForOK` |
| AC-H4 | `ListPriceHistory` は Day >= since を site_id 昇順・day 昇順。他の商品は返さない。存在しない商品は空 | 同 `HistoryListOrderAndSince` |
| AC-H5 | 商品を消すと推移も消える | 同 `HistoryDeleteItemCascades` |
| AC-H6 | `PrunePriceHistory(before)` は全商品の Day < before を消して件数を返す(Day == before は残す) | 同 `HistoryPrune` |
| AC-H7 | 更新は ok のサイトだけ推移に残し、同じ日は上書き・翌日は追加。RefreshAll は最初に 180 日より前(今日 − 180 日以前)を全商品から消し、179 日前は残す | `internal/refresh/refresh_history_test.go` の `TestRefreshItem_RecordsPriceHistory`・`TestRefreshAll_PrunesOldPriceHistory` |
| AC-H8 | `PriceHistory`:直近 days 日(89 日前は含み 90 日前は含まない・days=1 は今日だけ)、サイトはジャンル順 + ジャンル外は後ろ、Overall は日ごとの最小。推移なしは空のスライス | `TestPriceHistory_View`・`TestPriceHistory_Empty` |
| AC-H9 | `BuildHistory`(純関数。並び・日ごとの最小・同額) | `TestBuildHistory` |
| AC-H10 | 商品なし → ErrNotFound、days が範囲外 → ErrInvalid | `TestPriceHistory_Errors`(refresh) |
| AC-H11 | API:200 の本文(保存した目安と同じ low・mid、day は YYYY-MM-DD、days の既定 90・指定値)。推移なしは `[]`、mid の無い日は `null` | `internal/httpapi/server_history_test.go` の `TestPriceHistory`・`TestPriceHistory_EmptyAndNullMid` |
| AC-H12 | API:days が 0・負・181・数でない・小数 → 400、id 不正 → 400、商品なし → 404、トークンなし → 401 | `TestPriceHistory_Errors`(httpapi) |
| AC-H14 | migration 000007 の形(IF NOT EXISTS・主キー・DATE・low NOT NULL・mid NULL・2 つの CASCADE・既存の表に触れない)、down は表だけを消す | `migrations/history_layout_test.go` の `TestPriceHistory000007Shape`、`migrations/history_mysql_test.go`(`-tags mysql`)の `TestPriceHistory000007_Down` |
| AC-H15 | DB:1 日 1 行・low は null 不可・mid は null 可・存在しない商品/サイトは入らない。サイトを消すとそのサイトの推移、商品を消すとその商品の推移が消える | `TestPriceHistory000007_Constraints`(`-tags mysql`) |
| AC-HIS-API-01 | PWA の `getPriceHistory`:GET・Bearer・days を省くとクエリなし・`days=180`・失敗は ApiError(通信失敗は network) | `web/src/lib/api.test.ts` 「createApiClient(価格の推移)」 |
| AC-HIS-01 | 折りたたみで、開くまで取らない。開くと 1 回(days なし・Bearer)。閉じて開き直しても取り直さない | `web/src/App.history.test.tsx` 「AC-HIS-01 …」2 件 |
| AC-HIS-02 | 全体の最安の点が 2 未満(0・1)なら「推移はまだありません」でグラフなし | 同 「AC-HIS-02 …」2 件 |
| AC-HIS-03 | SVG(role=img)の aria-label(期間・最安・最高)、既定は全体の線だけ・点は実際の日だけ・軸の文字・アニメーション要素なし。色は CSS 変数 | 同 「AC-HIS-03 …」2 件 |
| AC-HIS-04 | 凡例(group「凡例」)はサイトごとの aria-pressed ボタン(推移の順・名前の無いサイトは「サイトN」)。押すと線と点を足し、もう一度で消す | 同 「AC-HIS-04 …」 |
| AC-HIS-05 | 失敗:通信失敗「オフライン」/ それ以外「価格の推移を取得できませんでした」。alert なし・サマリとサイト行は残る | 同 「AC-HIS-05 …」2 件 |
| AC-HIS-06 | 読み込み中は「読み込み中…」の文字だけ(progressbar なし) | 同 「AC-HIS-06 …」 |
| AC-HIS-07〜09 | `dayLabel`・`buildChart`(連続・欠けた日で線を切る・1 点・2 系列の共通軸・0.1 px の丸め・年またぎ・実際の点だけ・点なしは null)・`historyLabel` | `web/src/lib/history.test.ts`(テーブル駆動) |
| AC-HIS-10 | グラフの色をライトとダークの両方で定義 | `web/src/App.history.test.tsx` 「AC-HIS-10 …」 |
| AC-IOS-HIS-01 | 開くまで取らない。`loadPriceHistory` は days なしで 1 回、要約つきのグラフ。取得済みなら取り直さない | `ItemDetailPriceHistoryTests.testDoesNotFetchUntilLoadIsCalled`・`testLoadFetchesOnceWithoutDaysAndBuildsTheChart` |
| AC-IOS-HIS-02 | 点が 2 未満は `.empty`(線・凡例なし) | `testFewerThanTwoPointsIsEmpty` |
| AC-IOS-HIS-03 | 失敗の文言(通信失敗 / それ以外)。サマリ・注記は変えない。失敗のあとは取り直す | `testFailureTextsAndRetry` |
| AC-IOS-HIS-04 | 凡例(推移の順・名前・既定は全体だけ)、切り替え・凡例の順で描く・推移に無いサイトは無視 | `testLegendTogglesSiteSeries` |
| AC-IOS-HIS-05 | 取得中は `.loading`、重ねて呼んでも取得は 1 回 | `testLoadingStateWhileWaiting` |
| AC-IOS-HIS-06〜08 | `PriceHistoryFormat`:JST 0:00 の Date・読めない日付は nil / `dayLabel` / `points`(並べ替え・捨てる・segment)/ `accessibilityLabel` / `chart`(名前・サイトの線・2 未満は nil) | `PriceHistoryFormatTests` 全件 |
| AC-IOS-HIS-API-01 | `APIWishlistService.priceHistory`:GET・パス・Bearer・全項目の写像(null の mid)・`days=180`・404 と通信失敗 | `APIWishlistPriceHistoryTests` 全件 |
| AC-IOS-HIS-09 | モック用フィクスチャ(商品 12 = メルカリ 3 日分で 10/3 は estimates と同じ、商品 11 = なし、参照整合) | `WishlistFixturesPriceHistoryTests`(spec-writer が実装済みなので最初から通る) |
| AC-IOS-HIS-UI-01 | **XC** 商品 12:`historyDisclosure` を開くと `priceHistoryChart`(ラベル `価格の推移 10/1〜10/3 最安 ¥3,000 最高 ¥3,200`)と `historySite-1`(メルカリ・`非表示` → 押すと `表示中`)。商品 11 は `historyEmpty` | `WishlistUITests.testPriceHistoryChartAndEmptyState` |

既存のテストは変えていない(`RunPriceRepositoryContract` の末尾に `runPriceHistoryContract` の呼び出しを 1 行、`web/src/lib/api.test.ts` の末尾に describe を 1 つ足しただけ)。
`fakeApi`(web)に `priceHistory`・`failPriceHistory`、`FakeWishlistService`(iOS)に `setPriceHistory` を足した(既存の振る舞いは不変)。

### 実装の範囲(implementer 向けの目安)

- スタブ(置き換える):`internal/item/history.go` の `HistoryDay`、`memory_history.go`・`mysql_history.go`、`SaveSiteResult`(メモリ・MySQL)で推移の upsert、
  `internal/refresh/history.go` の `PriceHistory`・`BuildHistory` と RefreshAll の削除、`internal/httpapi/handlers.go` の `GetItemPriceHistory`(days の検査 → 400)
- `migrations/000007_price_history.{up,down}.sql`、`internal/store/query.sql` に price_history の upsert・読み出し・削除を足して `make wishlist-gen`(sqlc)
- Web:`src/lib/history.ts`(`notImplemented` ごと消す)・`src/lib/api.ts` の `getPriceHistory`・`src/ui/Sheet.tsx`・`src/styles.css`
- iOS:`PriceHistoryChart.swift` の `PriceHistoryFormat`、`ItemDetailViewModel` の 4-2 のメンバ、`APIWishlistService.priceHistory`(生成クライアントの `getItemPriceHistory`)、`Wishlist/ItemDetailSheet.swift`(`import Charts`)
- 実行確認:Go は `make wishlist-test`(TestLayout は 000006 の欠けで落ちる。それ以外は通ること)と `make wishlist-test-mysql`、Web は `npm run typecheck && npm run lint && npm test`、
  iOS は `make wishlist-ios-test` と、XCUITest は専用に複製したシミュレータで(spec-writer は `xcodebuild build-for-testing` が通ることだけ確認した)
