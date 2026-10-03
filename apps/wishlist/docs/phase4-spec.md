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

## 4-3 公式サイトの販売状況の監視

商品の `source_url`(公式ページ)を**夜間の CronJob(RefreshAll)の中だけ**で取得し、本文の決まった語から販売状況を判定して保存する。
詳細シートに「公式: 予約受付中(10/4 確認)」と根拠の語を出し、7 日以内の変化を添える。監視はオプトイン(`watch_official`)。
サイト固有の構造は推測しない(語の一致だけ)。判定が確かでないときは「判定できません」と出す。プッシュ通知は作らない(決定 4-4)。

### 決めたこと(仕様に書かれていなかった部分。既定案)

- **対象**:`items.watch_official`(既定 false)が true で、`source_url` が http(s) の絶対 URL(ホストあり)の商品だけ(純関数 `item.WatchTarget`)。
  作成(multipart・JSON)と PATCH で切り替える。**更新後に** ON で source_url が無い(null・空文字)なら 422(Service が検査。何も作らない・変えない。
  JSON の作成では画像を取りに行く前に検査する)。source_url を null にするときは `watch_official: false` も同時に送る。multipart の値は `true` / `false` だけ(他は 400)
- **source_url を変えたら状態を消す**:PATCH で source_url を**今と違う値・null** にすると、Repository が保存済みの `official_status` を消す(別のページの状態を見せないため)。
  同じ値・他の項目・watch_official の切り替えでは消さない(OFF にしても行は残し、表示だけしない)
- **取得(internal/official.Checker)**:
  - 夜間の `RefreshAll` だけ。**価格の更新をすべて終えてから**、ListItems の順に 1 商品ずつ(並列にしない)。`RefreshItem`(全モード)・`Estimates`・`Refresh`(api のプロセス)では取らない。
    api のプロセスは `refresh.Deps.Official` を渡さない(nil なら監視しない)
  - netguard のクライアント(nil なら `netguard.NewClient`)、User-Agent は `fetcher.UserAgent`(正直な名乗り)、本文の上限 `MaxPageBytes` = 2 MiB(超えたら failed)
  - **ホスト単位の間隔は価格の取得と共有する**:`fetcher.HostGate`(Throttle の中身を共有できるようにしたもの)を足し、`fetcher.Registry.Gate()` で NewRegistry の全 Fetcher と同じものを返す。
    refresher は `official.NewChecker(official.Config{Gate: reg.Gate()})` に渡す。robots.txt とページの取得の両方が Gate を通る(同じホストは 5 秒、`HostMinIntervals` のホストはその値)
  - **別のホストへのリダイレクトは追わない**(failed)。判定は「`URL.Host`(ポート込み)と scheme が同じ」。http → https の移動も別とみなす(source_url を https で登録し直せばよい)。
    同じホストの移動先が robots.txt で Disallow なら blocked(移動先を取らない)
  - ctx が終わったら failed を返し、RefreshAll は ctx の err で止まる
- **robots.txt**:
  - 取得の前に `scheme://host[:port]/robots.txt` を取る。**Checker の寿命の間、オリジンごとに 1 回**(取れなかった結果も覚える)。refresher は 1 回の実行で Checker を 1 つ作る = 1 回の実行の中でキャッシュ
  - 2xx → 解析。**404・410 → 全部許す**。**それ以外の 4xx(401・403 など)→ blocked**。**5xx・通信失敗・`MaxRobotsBytes`(512 KiB)超え → failed**(どちらもページは取らない)
  - 解析(純関数 `official.ParseRobots(body, RobotsProduct)`・`Robots.Allowed(path)`):`#` 以降はコメント、項目名は大文字小文字を区別しない、連続する User-agent 行が 1 グループ。
    **`*` のグループと自分(`wishlist-price-checker`。`fetcher.UserAgent` の「/」の前、大文字小文字を区別しない完全一致)のグループの規則を両方使う**(厳しいほうに寄せる。指示どおり)。
    空の Disallow・空の Allow は規則にしない。最初の User-agent より前の規則は無視。パス(+ `?クエリ`)に前方一致する規則のうち**最も長いもの**に従い、同じ長さなら Allow を優先(RFC 9309)。
    `*`(任意の文字列)と末尾の `$` に対応する(RFC 9309。無視すると `Disallow: /*?` を守れないため)。パスの大文字小文字は区別、パーセントエンコードの正規化はしない
- **判定(純関数 `official.ExtractText`・`official.Judge`)**:
  - 本文:`<body>` のテキスト(無ければ文書全体)。`script`・`style`・`noscript`・`template` の中身を除く(指示の script・style に加え、表示されない noscript・template も除いた)。
    要素の境目には空白を入れる(「在庫」「あり」が別要素なら一致しない = 推測で一致させない)。NFKC、連続する空白を 1 つ
  - 照合:テキストと語をそれぞれ NFKC・小文字・空白 1 つにしてから部分一致。全語(下の表と Neutral)を**長い順**(rune 数。同じ長さは表の順、Neutral は後)に探し、見つけた箇所を消してから次を探す
  - 見つけた種類(Neutral を除く)が**ちょうど 1 つ**ならその状態、0 なら unknown、2 つ以上なら ambiguous
  - 根拠(evidence):見つけた語を**表の表記のまま**、テキストに最初に現れた順に最大 3 つ(重複なし)。unknown・failed・blocked は空(`[]`。nil にしない)
  - **Neutral**(表の語を含むが意味が逆・別の語。消すだけで数えない・根拠にしない):`販売中止`・`在庫ありません`・`予約受付前`・`予約受付開始前`・`受付を終了`・`受付は終了`(「受付を終了しました」だけでは何の受付か分からないので、数えない)。
    「販売中止」が available、「在庫ありません」が available になる誤判定を防ぐ(推測の判定をしないための既定案)
  - **「予約受付は終了しました」は ended にする**(既定案。「予約受付」を含むので、そのままだと preorder と誤判定するため、より長い語として ended に足した。
    「予約受付を終了」「販売は終了」も同様)。「受付を終了」「受付は終了」のように何の受付か分からないものは Neutral(unknown になる)
  - **既知の限界**:ページの本文全体の語を見るので、ナビ・関連商品・おすすめの欄にある「在庫あり」などの語も拾う(別の種類の語が混じれば ambiguous、同じ種類だけなら誤って確定し得る)。
    根拠の語を一緒に出しているので、人が気づける。サイト固有の構造は推測しないため、除外しない
  - 文字コードは Content-Type・BOM・`<meta charset>` から判定して UTF-8 にする(`charset.NewReader`。Shift_JIS 等)。robots.txt は先頭の BOM(U+FEFF)を除いてから読む
  - 語の表(`official.Terms`。変えるときはこの表とテスト `TestTerms` も変える):

    | 状態 | 語 |
    |---|---|
    | available(販売中) | 販売中・在庫あり・カートに入れる・購入手続きへ |
    | preorder(予約受付中) | 予約受付中・予約する・予約受付 |
    | soldout(在庫切れ) | 在庫切れ・売り切れ・SOLD OUT・在庫なし |
    | ended(販売終了) | 販売終了・受付終了・予約受付終了・販売を終了・予約受付は終了・予約受付を終了・販売は終了 |

- **保存(`official_status`。純関数 `item.MergeOfficial(前回, 今回)` で重ねる)**:
  - `status`・`evidence`・`checked_at` は**最後に判定できた状態**とそれを確かめた時刻。今回が判定済み(available〜ambiguous)なら置き換える
  - 今回が **failed・blocked なら、判定済みの status・evidence・checked_at・changed_at・previous_status を上書きしない**。
    最後の試行の結果は **`last_result`・`last_attempt_at` の 2 列**に持つ(「どう持つか」の既定案)。一度も判定できていなければ status も failed・blocked(evidence は空)
  - `changed_at`・`previous_status` は、**判定済みの status が別の判定済みの状態に変わったときだけ**更新(初回・同じ状態・failed/blocked からの判定は変化にしない。
    failed/blocked → 判定は「前の判定が無い」ので変化ではない)。available → unknown は変化として記録する(判定済み同士)
  - **確認中に source_url が変わった場合**:`OfficialCheck.SourceURL`(確かめた URL)が空でなければ、保存のトランザクションの中で今の source_url と比べ、違えば `ErrSourceChanged` で何も保存しない
    (古いページの状態を新しい URL に付けない)。RefreshAll はこれを数えず次へ進む
  - 時刻は秒未満を切り捨てた UTC。`SaveOfficialCheck` は状態が 8 つ以外・根拠が 3 つ超・1 語 64 文字超・空の語なら ErrInvalid、商品が無ければ ErrNotFound(何も変えない)
- **DB(migration `000008_official_status`)**:

  ```sql
  ALTER TABLE items ADD COLUMN watch_official BOOLEAN NOT NULL DEFAULT FALSE;
  CREATE TABLE IF NOT EXISTS official_status (
    item_id BIGINT NOT NULL,
    status ENUM('available','preorder','soldout','ended','unknown','ambiguous','blocked','failed') NOT NULL,
    evidence JSON NOT NULL,                 -- ["予約受付中","予約する"]
    checked_at DATETIME NOT NULL,           -- status を確かめた時刻
    changed_at DATETIME NULL,
    previous_status ENUM('available','preorder','soldout','ended','unknown','ambiguous','blocked','failed') NULL,
    last_result ENUM('available','preorder','soldout','ended','unknown','ambiguous','blocked','failed') NOT NULL,
    last_attempt_at DATETIME NOT NULL,
    PRIMARY KEY (item_id),
    CONSTRAINT fk_official_status_item FOREIGN KEY (item_id) REFERENCES items (id) ON DELETE CASCADE
  ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;
  ```

  down は `DROP TABLE IF EXISTS official_status` と `ALTER TABLE items DROP COLUMN watch_official` だけ。seed は持たない
- **Repository**:`item.Item` に `WatchOfficial`・`Official *OfficialStatus` を足し、ListItems・GetItem・CreateItem・UpdateItem が埋める(一覧は数が少ないので別のエンドポイントにしない)。
  `item.OfficialRepository.SaveOfficialCheck` をメモリ・MySQL の両方に足す
- **refresh**:`Deps.Official`(`OfficialChecker`。nil なら監視しない)・`Deps.Officials`(`item.OfficialRepository`)。
  `AllReport.OfficialChecked`(確かめた数)・`OfficialFailed`(failed と保存の失敗。blocked は数えない)を足し、`Failed`(価格)には含めない。保存の失敗はログに出して次へ。
  refresher は 1 行の結果に `official=N official_failed=M` を足す。監視の失敗は終了コードに影響しない
- **API(最小限)**:`Item.watch_official`(boolean)・`Item.official_status`(`OfficialStatus` か null)を足し、`ItemFields`・`ItemCreateMultipart`・`ItemUpdate` に `watch_official` を足した。
  `OfficialState`(8 つの enum)・`OfficialStatus`(status・evidence・checked_at・changed_at・previous_status・last_result・last_attempt_at)。
  `watch_official`・`official_status` は **required にしない**(古いキャッシュ・応答を読めるように。4-1 の aliases と同じ)が、サーバーは常に返す。新しいエンドポイントは無い
- **PWA**:
  - 詳細シート:`watch_official` が true のときだけ、`<section aria-label="公式の販売状況">` に 1 行目 `公式: 予約受付中(10/4 確認)`(日付は checked_at の JST)、
    `根拠: 予約受付中・予約する`、changed_at が **7×24 時間以内**(未来も含む)なら `10/3 に 販売中 → 販売終了`、判定済みのまま最後の試行が failed・blocked なら
    `最新の確認(10/5): 取得できませんでした`。status が null なら `公式: まだ確認していません`。status が failed・blocked(一度も判定できていない)は `(10/4)`(「確認」を付けない)
  - 文言:unknown `判定できません` / ambiguous `判定できません(複数の表示)` / blocked `取得しません(robots.txt)` / failed `取得できませんでした`。alert にしない
  - 純関数 `src/lib/official.ts`(`OFFICIAL_LABELS`・`officialSummary`・`officialEvidence`・`officialChange`・`officialLastAttempt`)
  - 編集:チェック `公式ページを監視する`(初期値 `item.watch_official ?? false`)。**元の商品の source_url が空なら無効**にし、`公式ページの URL が無いので監視できません` を出す
    (編集画面は source_url を変えないため)。変えたときだけ `watch_official` を送る
  - 登録画面には置かない(既定 OFF。編集で ON にする)。**グリッドには文字を出さない**(仕様 §3)
- **iOS**:PWA と同じ文言・規則。`OfficialFormat`(純関数)・`OfficialLines`、`ItemDetailViewModel.official`(watchOfficial が false なら nil。変化は `now` で判定)、
  `EditItemViewModel.watchOfficial`・`canWatchOfficial`・`watchOfficialHint`(patch は変えたときだけ。sourceURL が無ければ送らない)。
  `Item.watchOfficial`・`officialStatus`、`ItemPatch.watchOfficial`、`OfficialState`・`OfficialStatus` は Domain に置いた(古いキャッシュの JSON は既定値で読む Decodable を spec-writer が実装済み)。
  モック起動 `WISHLIST_USE_FAKE=official`(`WishlistFixtures.makeServiceWithOfficial`。キャッシュも `officialItems`・`officialGenres`)。
  accessibilityIdentifier:詳細シート `officialStatus`(ラベル = summary)・`officialEvidence`・`officialChange`・`officialLastAttempt`、編集 `watchOfficialToggle`(Toggle)・`watchOfficialHint`
- 常時動くアニメーションは入れない(進捗表示も文字だけ)

### 受け入れ条件とテスト

テストは Go が `make wishlist-test`(MySQL は `make wishlist-test-mysql`)、Web が `web/` で `npm test`、iOS が `make wishlist-ios-test`(XC は XCUITest)。

| ID | 条件 | テスト |
|---|---|---|
| AC-O1 | `MergeOfficial`:初回・同じ状態・判定済み同士の変化・failed/blocked は上書きしない・failed だけのあとの判定は変化にしない・秒未満切り捨ての UTC・入力と配列を共有しない | `internal/item/official_test.go` の `TestMergeOfficial`・`TestMergeOfficial_DoesNotAlias`(`TestOfficialState_ValidJudged` は型の定義なので最初から通る) |
| AC-O2 | `WatchTarget`:ON かつ http(s) の絶対 URL(ホストあり)だけ | `TestWatchTarget` |
| AC-O3 | Service:更新後に ON で source_url が無い(null・空文字)なら ErrInvalid で何も変えない(作成は画像も残さない) | `internal/item/service_official_test.go` の `TestService_CreateWatchOfficial`・`TestService_UpdateWatchOfficial` |
| AC-O4 | watch_official を作成・更新で保存し、Get・List・Update の結果に出る。省略は変えない | `internal/item/itemtest/official_contract.go` の `WatchOfficialPersisted`(`RunOfficialRepositoryContract`。メモリ `TestMemoryOfficialRepositoryContract`・MySQL `TestMySQLOfficialRepositoryContract`) |
| AC-O5 | 保存した状態が Get・List の `Official` に出る(MergeOfficial と同じ・戻り値も同じ・他の商品は nil・返した配列を変えても保存値は変わらない) | 同 `SaveAndRead` |
| AC-O6 | 商品なし → ErrNotFound、状態・根拠の規則違反 → ErrInvalid で何も変えない(3 語・64 文字ちょうどは通る) | 同 `SaveErrors` |
| AC-O7 | source_url を別の値・null にすると状態を消す。同じ値・他の項目・watch_official の切り替えでは残す | 同 `SourceURLChangeClearsStatus` |
| AC-O8 | 商品を消すと状態も消える | 同 `DeleteItemCascades` |
| AC-O9 | `Judge`:各語・同じ種類の複数・最大 3・長い語を先に消す・Neutral・ambiguous・unknown・NFKC/小文字/空白の揺れ | `internal/official/judge_test.go` の `TestJudge` |
| AC-O10 | 語の表と Neutral が spec と同じ | `TestTerms`(定数なので最初から通る) |
| AC-O11 | `ExtractText`:body だけ・script/style/noscript/template を除く・要素の境目は空白・空白を詰める・NFKC・文字参照 | `TestExtractText` |
| AC-O12 | 架空の公式ページの fixture(`internal/official/testdata/*.html`)の判定 | `TestJudge_Fixtures` |
| AC-O13 | `ParseRobots`・`Allowed`:Allow・Disallow・コメント・大文字小文字・空の Disallow・最長一致・ワイルドカード・グループ(* と自分の両方) | `internal/official/robots_test.go` の `TestRobots` |
| AC-O14 | `AllowAll` と `RobotsProduct`(fetcher.UserAgent の名乗りと同じ) | `TestRobots_AllowAllAndProduct` |
| AC-O15 | Checker:robots.txt → ページの順で取り、判定する。どちらも正直な User-Agent | `internal/official/checker_test.go` の `TestChecker_Judges` |
| AC-O16 | robots.txt は Checker ごと・オリジンごとに 1 回(取れなかった結果も覚える)。Disallow のページは取らない | `TestChecker_RobotsCachedPerChecker` |
| AC-O17 | robots.txt の応答:404・410 は許可、他の 4xx は blocked、5xx・大きすぎる・通信失敗は failed(ページを取らない・根拠は空) | `TestChecker_RobotsStatus` |
| AC-O18 | ページの失敗(2xx 以外・2 MiB 超え・別ホストへのリダイレクト・http(s) でない・ctx の終了)は failed。同じホストのリダイレクトは追い、移動先が Disallow なら blocked | `TestChecker_PageFailures` |
| AC-O19 | robots.txt とページの取得は Gate を通る(共有した Gate で直前の取得とも間隔をあける) | `TestChecker_UsesGate` |
| AC-O20 | `HostGate`:同じホストは間隔をあける・違うホストは待たない・HostMinIntervals・失敗も数える・取り消し | `internal/fetcher/gate_test.go` の `TestHostGate_Intervals`・`TestHostGate_FailureAndCancel`(`TestHostOf` は最初から通る) |
| AC-O21 | `NewRegistry` の Fetcher と `Registry.Gate()` は同じ HostGate(NewRegistryWith は nil) | `TestRegistry_GateIsShared` |
| AC-O22 | RefreshAll は対象の商品だけを ListItems の順に確かめ、Now の時刻で保存する | `internal/refresh/refresh_official_test.go` の `TestRefreshAll_ChecksWatchedItems` |
| AC-O23 | failed は数えて判定済みを上書きしない、blocked は失敗に数えない、保存の失敗は次へ進む。どれも `Failed` には数えない | `TestRefreshAll_OfficialFailures` |
| AC-O24 | RefreshItem(全モード)・Estimates・Refresh では取らない。価格の更新をすべて終えてから確かめる | `TestRefreshAll_OfficialOnlyNightlyAndAfterPrices` |
| AC-O25 | Official が nil なら監視しない。ctx が終わったら止める | `TestRefreshAll_OfficialNilAndCancel` |
| AC-O26 | API:作成(multipart・JSON)と PATCH で watch_official を切り替える。source_url なしの ON は 422(JSON は画像を取らない)、multipart の不正な値・JSON の型違いは 400。常に `watch_official` を返す | `internal/httpapi/server_official_test.go` の `TestItemWatchOfficial` |
| AC-O27 | API:Get・一覧・PATCH の応答に `official_status`(無ければ null。時刻は UTC の RFC 3339、変化が無ければ changed_at・previous_status は null)。source_url を変えると null | `TestItemOfficialStatus` |
| AC-O28 | API の estimates の取得・更新では公式ページを取らない | `TestOfficialNotFetchedByAPI` |
| AC-O29 | migration 000008 の形(items の列・IF NOT EXISTS・主キー・CASCADE・ENUM・JSON・NULL の可否・行を足さない)、down は表と列だけを消す | `migrations/official_layout_test.go` の `TestOfficialStatus000008Shape`、`migrations/official_mysql_test.go`(`-tags mysql`)の `TestOfficialStatus000008_Down` |
| AC-O30 | DB:watch_official の既定は false、1 商品 1 行、8 つ以外の status は入らない、previous_status は null 可、存在しない商品は入らない、商品を消すと消える | `TestOfficialStatus000008_Constraints`(`-tags mysql`) |
| AC-O31 | refresher の 1 行の結果に `official=N official_failed=M`。監視の失敗は終了コードに影響しない | `cmd/refresher/official_test.go` の `TestPrintReport_Official` |
| AC-OFF-01〜04 | PWA の文言(状態・1 行目・根拠・7 日以内の変化・最新の試行の注記) | `web/src/lib/official.test.ts`(テーブル駆動。`OFFICIAL_LABELS` は定数なので最初から通る) |
| AC-OFF-05 | 詳細シートに「公式: 予約受付中(10/4 確認)」と根拠。7 日以内の変化を添え、過ぎたら出さない | `web/src/App.official.test.tsx` 「AC-OFF-05 …」2 件 |
| AC-OFF-06 | unknown・ambiguous・blocked・failed の文言、最後の試行の失敗の注記 | 同 「AC-OFF-06 …」5 件 |
| AC-OFF-07 | 監視中で未確認は「まだ確認していません」、監視していなければ出さない、古い応答でも開ける | 同 「AC-OFF-07 …」3 件(古い応答の件は最初から通る) |
| AC-OFF-08 | グリッドには文字を出さない | 同 「AC-OFF-08 …」(回帰の確認。最初から通る) |
| AC-OFF-09 | 編集のチェック(初期値・外す/付けると watch_official だけ PATCH・source_url が無ければ無効と理由・古い応答は OFF) | 同 「AC-OFF-09 …」4 件 |
| AC-IOS-OFF-01〜04 | `OfficialFormat`(PWA と同じ文言・規則) | `OfficialFormatTests` 全件 |
| AC-IOS-OFF-05 | `ItemDetailViewModel.official`(要約・根拠・now で判定する変化・最新の試行・未確認・監視していなければ nil) | `ItemDetailOfficialTests` 全件 |
| AC-IOS-OFF-06 | `EditItemViewModel` の監視のトグル(初期値・可否と理由・変えたときだけ送る・sourceURL が無ければ送らない) | `EditItemOfficialTests` 全件 |
| AC-IOS-OFF-07 | 古いキャッシュの JSON(新しい項目なし)を既定値で読み、書き戻しても失われない | `OfficialCacheCompatibilityTests`(spec-writer が実装済みなので最初から通る) |
| AC-IOS-OFF-08 | `WISHLIST_USE_FAKE=official` のフィクスチャ | `WishlistFixturesOfficialTests`(最初から通る) |
| AC-IOS-OFF-API-01 | `APIWishlistService`:watch_official・official_status の写像(null・無い場合)、PATCH の watch_official | `APIWishlistOfficialTests` の `testItemMapsWatchOfficialAndOfficialStatus`・`testItemWithNullOrMissingOfficialFields`・`testUpdateItemSendsWatchOfficialOnlyWhenSet` |
| AC-IOS-OFF-UI-01 | **XC** 商品 12 の詳細シートに `officialStatus`(`公式: 予約受付中(10/4 確認)`)と `officialEvidence`、商品 11 には出ない。編集に `watchOfficialToggle`(値 1) | `WishlistUITests.testOfficialStatusInDetailSheetAndEditToggle` |

既存のテストは変えていない。`internal/httpapi/server_test.go` の `env` に `repo`(メモリの Repository)を 1 つ足しただけ。
`web/src/lib/api.ts` の multipart の値の型に boolean を足した(生成した型に `watch_official` が増え、型検査を通すため。`String(true)` = `"true"`)。

### 実装の範囲(implementer 向けの目安)

- Go:`internal/item/official.go` の `MergeOfficial`・`WatchTarget`、`memory_official.go`・`mysql_official.go`、メモリ・MySQL の CreateItem・UpdateItem・GetItem・ListItems(watch_official・Official・
  source_url の変更で状態を消す・`cloneItem` で Official を複製)、Service の検査(作成・更新)
- `migrations/000008_official_status.{up,down}.sql`、`internal/store/query.sql` に official_status の読み書きと items.watch_official を足して `make wishlist-gen`(sqlc)
- `internal/fetcher/gate.go`(`HostGate`。Throttle もこれを使う。`Registry.Gate()`)、`internal/official`(`ExtractText`・`Judge`・`ParseRobots`・`Allowed`・`Checker`)
- `internal/refresh`(RefreshAll の最後に監視)、`internal/httpapi`(toAPIItem・Create〈JSON・multipart の `watch_official`〉・Update)、`cmd/refresher`(Checker を作って Deps に渡す・printReport)
- Web:`src/lib/official.ts`(`notImplemented` ごと消す)・`src/ui/Sheet.tsx`・`src/ui/Edit.tsx`
- iOS:`OfficialFormat.swift`・`ItemDetailViewModel.official`・`EditItemViewModel`(canWatchOfficial・hint・patch)・`APIWishlistService`(Item・ItemUpdate の写像)、
  `Wishlist/AppModel.swift`(`WISHLIST_USE_FAKE=official`)・`ItemDetailSheet.swift`・編集の View
- 実行確認:Go は `make wishlist-test` と `make wishlist-test-mysql`、Web は `npm run typecheck && npm run lint && npm test`、iOS は `make wishlist-ios-test` と XCUITest
  (spec-writer は `swift build --build-tests` と `xcodebuild build-for-testing` が通ることだけ確認した)

## 4-1 iOS 表記揺れの辞書の編集

4-1 で PWA に入れた辞書の編集を iOS の設定画面(ジャンルの編集)に入れる。規則は PWA と同じ(AC-A14・AC-SET-08〜12)。

### 決めたこと(既定案)

- **Domain**:`Genre.aliases: [[String]]`(既定 `[]`。応答・キャッシュに無ければ `[]`)、`GenreCreate.aliases`・`GenreUpdate.aliases`(nil なら送らない、`[]` なら全部消す)。
  Domain と Fake(作成・更新で aliases を保存する)は spec-writer が実装済み
- **行の読み書き**:純関数 `AliasLines`(`parseLine`・`format`・`parseGroups`・`invalidRowMessage`)。区切りは `,`・`，`・`、`、各語の前後の空白(全角を含む)を除き、空の語は捨てる。語の中の空白は残す。表示は `, ` でつなぐ
- **ViewModel**:`SettingsListViewModel.aliasLines(of:)`(編集画面の初期の行)、`addGenre(name:queryTemplate:siteIDs:aliasLines:)`・`updateGenre(_:name:queryTemplate:siteIDs:aliasLines:)` を足した
  (既存の 3 引数版は残す)。1 語だけの行があれば API を呼ばず `別名グループNは2語以上をカンマで区切って入力してください`(PWA と同じ文言)。
  編集は読み直したグループが元と違うときだけ `aliases` を送り(区切り・空白・空の行の違いだけなら送らない)、追加は常に送る(無ければ `[]`)。正規化後の重複は API の 422 をそのまま出す
- **View(GenreEditSheet)**:「別名グループ」の Section。1 グループ = 1 行の TextField(`aliasLine-<N>`、N は 1 始まり)、「別名グループを追加」(`addAliasLine`)、
  行の削除(`removeAliasLine-<N>`。番号は詰める)。保存は既存の「保存」から新しい `addGenre`・`updateGenre` を呼ぶ
- モック起動 `WISHLIST_USE_FAKE=official` のジャンル 1 に `S.H.Figuarts, SHフィギュアーツ` の 1 グループ(`WishlistFixtures.officialGenres`)

### 受け入れ条件とテスト

| ID | 条件 | テスト |
|---|---|---|
| AC-IOS-ALI-01 | 1 行のカンマ区切りの読み書き(3 種の区切り・空白・空の語・語の中の空白)、空の行の除外と 1 語の行の検出・文言 | `AliasLinesTests` 全件 |
| AC-IOS-ALI-02 | 編集画面の初期の行(`, ` 区切り・辞書なしは空) | `SettingsAliasesTests.testAliasLinesOfGenre` |
| AC-IOS-ALI-03 | 編集は変えたときだけ `aliases` を全件置き換えで送る(表記の違いだけなら送らない・全部消すと `[]`・名前と同時なら 1 回の PATCH)。`genres` も更新 | `testUpdateSendsAliasesOnlyWhenChanged`・`testUpdateCombinesNameAndAliases` |
| AC-IOS-ALI-04 | 1 語だけの行は API を呼ばず行番号つきの文言(追加・編集) | `testSingleWordLineIsRejectedWithoutCallingTheAPI` |
| AC-IOS-ALI-05 | 追加は常に `aliases` を送る(空の行を除く・無ければ `[]`)。API の 422 は errorMessage に出す | `testAddAlwaysSendsAliases`・`testServerRejectionIsShown` |
| AC-IOS-ALI-API-01 | `APIWishlistService`:Genre の aliases の写像(無ければ `[]`)、作成・更新で送る(`[]` は送る・nil は送らない) | `APIWishlistOfficialTests` の `testGenreAliasesAreMappedAndMissingMeansEmpty`・`testCreateAndUpdateGenreSendAliases` |
| AC-IOS-ALI-UI-01 | **XC** 設定 → S.H.Figuarts で `aliasLine-1` に既存のグループ、`addAliasLine` で行を足して保存し、開き直すと残っている | `WishlistUITests.testGenreAliasesCanBeEditedInSettings` |

### 実装の範囲(implementer 向けの目安)

- `AliasLines.swift`・`SettingsListViewModel`(新しい 3 つ)・`APIWishlistService`(`genre` の写像、`createGenre`・`updateGenre` の aliases)・`Wishlist/SettingsView.swift`(GenreEditSheet)
