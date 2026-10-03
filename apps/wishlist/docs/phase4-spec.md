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
