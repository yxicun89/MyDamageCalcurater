# ADR-0304: Web のオンライン MasterSource — 検索ベースの選択 UI と、技の ID 解決の API ギャップ

- 状態: 提案(Web レーン、2026-09-23。§3 の技解決の既定案は DECISIONS.md でデータ/API レーンへ提案中・未確認。
  型の設計は「追記(2026-09-23)」で確定し、P4-16 の受け入れ条件・テストの正になっている)
- 日付: 2026-09-23
- 関連: plan.md Next(1)、ADR-0301 §4(オンライン用 MasterSource は「まだ作れない」として持ち越した宿題)、
  ADR-0300(Web の構成)、api/openapi.yaml(`searchSpecies`・`getSpecies`・`searchMoves`・`searchItems`・`listNatures`)、
  CLAUDE.md 絶対ルール4(サービスは自分のDBにだけ触る)

## 背景

ADR-0206(データレーンの依頼d。PR #87)で gateway・calc-svc が pokedex-svc に配線され、`verify-m1.md` の
「実マスタでの計算」は解決済みになった。残るのは Web 自身がポケモン・技・持ち物・性格の一覧を pokedex-svc の
公開 API から読む「オンライン用 MasterSource」(ADR-0301 §4 が持ち越した宿題)。

このタスクに着手する前提として、公開 API(gateway 経由で `web/` から呼べる範囲)の実際の形を調べた。

### 分かったこと

`api/openapi.yaml` の `searchSpecies`・`searchMoves`・`searchItems` は日本語名の**前方一致検索**で、
`q` 省略時は「全件・limit まで」を返すが、`limit` は `maximum: 200` で固定(オフセット・カーソル等の
ページングパラメータは無い)。`listNatures` だけは無条件に全件を返す。

実クラスタで確認した実データ件数(`kubectl -n pokecalc exec mysql-0 -- mysql ... pokedex` で直接集計):

| マスタ | 件数 | 1回の `limit=200` の検索で全件取れるか |
|---|---|---|
| 種族(species) | 349 | **取れない**(200件で打ち切り) |
| 技(moves) | 515 | **取れない**(200件で打ち切り) |
| 持ち物(items) | 166 | 取れる(`q=""` 1回で全件) |
| 性格(natures) | 25 | 取れる(`listNatures` はそもそも無条件で全件) |

加えて、`SpeciesDetail.learnset`(`getSpecies` の応答)は技の**ID配列**(`string[]`)だけを返す。
`searchMoves` は日本語名の前方一致でしか引けず、ID で個別に引く公開エンドポイントは無い
(`getMasterExport` は ADR-0204 で internal-only と明記されており、gateway の Ingress からブラウザには届かない)。

pokedex-svc の実装を確認すると、`getSpecies` のハンドラ(`services/pokedex/internal/httpapi/search.go`)は
`store.ListSpeciesLearnset` で ID 配列を作った後、`Move` の実体には解決せずそのまま返している。一方
`store.Queries` には全技を返す `ListMoves(ctx) ([]Move, error)` が既にある(`MasterExport` が内部で使っている想定)。
つまり**データレーン側には技を ID→実体で引く材料は既にある**が、それを公開 API の形にする変更はまだ無い。

## 決定

### 1. 種族選択は検索ベースの UI にする(ドロップダウン方式をやめる)

オフラインモード(架空の例データ、高々十数件)は既存どおり一覧ドロップダウンのままでよいが、オンラインモードの
種族選択は `searchSpecies` をそのまま活かした**入力補完(インクリメンタルサーチ)**にする。349件を一括取得して
クライアント側の一覧にする案は、API の `limit` 上限(200)がある限り原理的に実現できない(打ち切られた169件が
決して選べなくなる)。API 自体が「検索」として設計されている(`operationId: searchSpecies`)こととも整合する。

- 入力するたびに `searchSpecies(q, limit)` を叩き、候補を出す(デバウンスする)。
- `q` が空のときは何も出さない、または既定の `limit`(50)ぶんだけ「候補の例」として出す(全件を意味しない旨を明示する)。
- 持ち物(166件)は `limit=200` の1回の取得で全件を賄えるので、こちらは引き続き一覧(ドロップダウン)でよい。
- 性格(25件)は `listNatures` で全件を1回取得し、一覧のままでよい。

### 2. 技選択も検索ベースにする(ただし種族の learnset との突き合わせが必要)

技は515件あり種族と同様に一括取得できないため、技そのものの選択 UI も `searchMoves(q, limit)` による検索ベースにする。
ただし種族選択後の技候補は「その種族が覚えられる技」に絞りたい(ADR-0300 のオフラインの体験と揃える)。
`getSpecies` の `learnset` は ID 配列しか返さないため、**技の実体(名前・タイプ・分類)への解決手段が公開 API に無いと
learnset を人が読める形で出せない**。これは Web 側だけでは解決できない、データ/API レーンへの依頼が要る欠落。

### 3. 技の ID 解決の欠落 — 既定案(データ/API レーンへ提案。DECISIONS.md に転記)

次のいずれかを既定案として提案する(どちらでも Web 側の対応は小さく変わるだけ):

- **案A(既定として推す)**: `getSpecies` の応答の `learnset` を、ID配列 (`string[]`) から `Move` 実体の配列に変える
  (`ListSpeciesLearnset` が返す ID をそのままハンドラ内で `Move` に解決してから返す。`ListMoves` 相当の材料は
  既に store にあるため実装コストは小さいはず)。種族を1回引けば技も揃うので、追加のラウンドトリップが要らない。
  破壊的変更なので `api/openapi.yaml` のスキーマ変更として扱う(API レーンの持ち物)。
- **案B**: `learnset` の形は変えず、`GET /api/pokedex/moves/{id}` のような ID 個別解決エンドポイントを追加する。
  種族1体につき技20〜30件ぶんのラウンドトリップが要るか、`ids` のクエリでバッチ解決できるようにするかの検討が要る。

案Aを既定として推す理由: 呼び出し回数が増えない、`SpeciesSummary`(検索結果一覧)は変えずに `SpeciesDetail`
(個別取得の応答)だけを変えるので影響範囲が閉じている、店側(pokedex-svc)は名前解決の材料を既に持っている。

Web 側でこのための回避策(例: 名前の先頭文字を全パターン検索して515件を力技で全件収集する)は採用しない。
HTTPリクエストを数十回打つ設計になり、pokedex-svc の実装の内部詳細(名前がどの文字体系で始まるか)に
Web 側が暗黙に依存することになるため、素直に壊れやすい。ここは待つ。

**追記(2026-09-23)**: 案B の `GET /api/pokedex/moves/{key}`(`getMove`)は API レーンが実装した(ADR-0105 §3
追記。判定レーンの JD4 の依頼が主目的で、このADRの欠落解消を主目的にしたものではない)。**この欠落自体は
まだ解消していない**: `getMove` は技1件だけを返すため、`learnset` の解決にそのまま使うと種族1体あたり
技20〜30件ぶんのラウンドトリップが要るという、上の案Bの欠点がそのまま残る。案A(`getSpecies` の `learnset`
を `Move` 実体の配列にする)か、`getMove` 側に `ids` のバッチ解決を足すかは、依然として API レーンへの
未決の提案のまま(§3 は据え置き)。

### 4. 段階的な導入(技解決の欠落が解消されるまで)

ブロッカーで止まるのではなく、今の公開 API だけで作れる範囲を先に作る:

1. `MasterSource` のオンライン実装(`createPokedexMasterSource` 等)を用意し、**持ち物・性格**は全件取得する
   (166件・25件とも1回の呼び出しで収まる)。
2. **種族**は検索ベースの選択 UI(§1)にし、選んだ種族の `baseStats`・`types`・`abilities` は `getSpecies` からそのまま使う。
3. **技**は、§3 の欠落が解消されるまでの間、`getSpecies` の `learnset`(ID配列)と `searchMoves` を組み合わせても
   「その種族が覚えられる技の一覧」を正しく出せない(名前検索でしか技を引けず、ID との突き合わせができないため)。
   この間は技選択欄を「(オンラインでは未対応。据え置き)」として明示し、技を必要とする操作(ダメージ計算のダメージ技選択等)は
   オンラインモードでは無効化して案内を出す(壊れた計算結果を返すよりも、機能を絞って正直に出す)。
4. データ/API レーンの対応後、技も検索/一覧に切り替える。

## 却下・保留

- **全件を複数回のページングで取得する**: API に offset/cursor が無いため不可能(`limit` の上限が唯一の蛇口)。
  API レーンへの追加提案にはしない(検索 UI で十分要件を満たせるため、ページング API 自体の追加は過剰と判断)。
- **技名の先頭文字を総当たりして515件を集める回避策**: §3 で却下理由を記載(壊れやすい・実装詳細への暗黙依存)。
- **`getMasterExport`(internal-only)をそのまま公開する**: ADR-0204 の設計(calc-svc からの内部利用に限定)を崩す。
  gateway の Ingress 経由で任意のクライアントに全マスタを晒す変更になり、影響が大きすぎる。

## 影響

- `web/src/master/`: 新しい `MasterSource` 実装(検索ベースの種族選択、持ち物・性格は全件取得、技は当面据え置き)。
- `web/src/App.tsx`: オンラインモード選択時の `masterSource` の差し替え(ADR-0301 §4 の既定見直しの一部)。
- 画面側: 種族選択 UI がオンライン/オフラインで一覧/検索に分かれる(コンポーネントの分岐が増える)。
- データ/API レーンへの依頼(DECISIONS.md に転記): `getSpecies.learnset` を `Move[]` に変える(案A、既定)。
  返答があるまで、Web のオンラインモードは技選択を無効化した状態で先に進める。

## 追記(2026-09-23、Web レーン): 実際の TypeScript インターフェース設計

§1〜§4 は概念の決定にとどまり、型の設計は未定だった。P4-16 の spec(受け入れ条件と失敗するテスト)を書くにあたり、
以下を決めた。**最優先の制約は「オフライン(`exampleMasterSource`)の同期的な使い方を一切壊さない」**
(既存の全画面・既存テストを1行も変えずに今までどおり動くこと)。

### A-1. 公開 API の欠落は §3 の技だけではない(調査で追加で判明)

`api/openapi.gen.ts` の生成型を読み直したところ、§1 の件数の問題・§3 の技の ID 解決に加えて、次の2つが欠けている:

- **持ち物・特性の効果データが無い**。公開 API の `Item`・`Ability` は `{id, nameJa}` だけで、
  engine の `ItemEffect`・`AbilityEffect`(4096基準の固定小数)に当たるフィールドを持たない
  (効果を持つのは internal-only の `getMasterExport` のみ。ADR-0204)。
  このため `domain/requests.ts` の `defensiveItemCandidates` と `domain/reverseItems.ts` の
  `reverseItemCandidates` は**効果データで候補を選ぶ**設計(ADR-0300 §6)なので、オンラインでは
  「候補なし」しか返せない。黙って空の候補を出すのは誤解を招くため、技と同じく**明示的に無効化**する。
- **特性の全件一覧を引く公開 API が無い**(`listNatures` に相当するものが特性には無い)。
  特性の実体は `getSpecies` の応答に種族ごとに付いてくるので、`MasterData.abilities` は
  「これまでに引いた種族の特性」を積み上げる形になる(`defaultAbility` が `species.abilities` の ID を
  引けるようにするため)。

### A-2. `MasterData` は変えない。使えない機能を `MasterCapabilities` で伝える

案(a)(`MasterData` 本体を変えず別インターフェースを足す)を採る。`MasterData` に**省略可能な**
`capabilities?: MasterCapabilities` を1つ足すだけにし、省略は「全部使える」(オフライン相当)とみなす。
省略可にしたのは、既存の `MasterData` の作り手(`exampleMasterSource` と各画面テストの fixture)を
1つも変えずに済ませるため。既定の補完は `master/capabilities.ts` の `masterCapabilities()` 1か所に集める
(コーディング規約 §2「単一の正」)。

```
MasterCapabilities { speciesList: boolean; moves: boolean; effects: boolean }
```

画面は「オンラインかどうか」ではなく「この機能が使えるか」で分岐する(モードの名前を画面に持ち込まない)。
オンラインは `{speciesList: false, moves: false, effects: false}`(`ONLINE_MASTER_CAPABILITIES`)。

### A-3. 種族の都度取得は `MasterSource` と別のインターフェースにする

`MasterSource.load()` の形(全件を1回返す)は変えない。検索は `SearchableMasterSource`(= `MasterSource` +
`search: MasterSpeciesSearch`)で足し、`isSearchableMasterSource()` で絞り込む。

```
MasterSpeciesSummary  { key, dexNo, form, nameJa, types }     // 公開 API の SpeciesSummary 相当
MasterSpeciesResolution { species: MasterSpecies; abilities: readonly Ability[] }
MasterSpeciesSearch {
  searchSpecies(query, signal?): Promise<readonly MasterSpeciesSummary[]>
  resolveSpecies(key, signal?): Promise<MasterSpeciesResolution>
}
SearchableMasterSource extends MasterSource { readonly search: MasterSpeciesSearch }
```

`MasterSpecies` は `MasterSpeciesSummary` を構造的に満たすので、オフラインの種族もそのまま候補として扱える
(検索 UI をオフラインにも流用できる)。`resolveSpecies` が特性も返すのは A-1 の後半の理由による。
取得結果を覚えるのは呼び出し側(画面)の責務とし、`MasterSpeciesSearch` の実装はキャッシュを持たない。

### A-4. 検索 UI のパラメータ(`master/onlineSource.ts` の定数)

- `SPECIES_SEARCH_MIN_LENGTH = 1`(日本語名の前方一致なので1文字で十分絞れる)
- `SPECIES_SEARCH_LIMIT = 50`(§1 の「既定の limit(50)」。349件を一覧にはしない)
- `SPECIES_SEARCH_DEBOUNCE_MS = 250`(入力1文字ごとに投げない)
- 空・空白だけのクエリは **fetch せず空配列**(空 = 全件にしない。§1)
- 候補が `SPECIES_SEARCH_LIMIT` に達したら「全件ではない」旨を出す(`masterOnlineText.speciesSearchTruncated`)
- `ITEMS_FETCH_LIMIT = 200`(公開 API の maximum。実データ166件)。
  **応答が limit ちょうどなら打ち切りの疑いがあるため `load()` は失敗する**(黙って欠けたマスタを配らない)

### A-5. 技・持ち物候補が無効なときの画面の見え方

文言は `web/src/i18n/ja.ts` の `masterOnlineText`(コーディング規約 §2)。

- **技**(`capabilities.moves === false`): 技のセレクトは**残すが `disabled`** にし(欄そのものを消すと
  画面の構造が両モードで変わりすぎる)、選択肢は「なし」相当の空だけ。すぐ下に
  `masterOnlineText.movesUnavailable`(「オンラインでは技を選べません…」)を出す。
  技が決まらないので計算は実行しない(壊れた結果を出さない。§4)。
- **持ち物の候補比較**(`capabilities.effects === false`): 計算画面の「持ち物の候補も比較」トグルと
  逆算画面の持ち物候補を `disabled` にし、`masterOnlineText.itemCandidatesUnavailable` を添える。
  持ち物そのものの選択は残す(API の計算は `itemId` だけを送るので成立する)。
- **種族**(`capabilities.speciesList === false`): ドロップダウンの代わりに検索欄(A-4)。
  **追記(P4-16b 実装。critic 指摘): 検索モードで種族がまだ解決していない間は、持ち物欄も一時的に出さない**
  (「欄を消さず `disabled` にする」という上の原則からの意図的な例外)。理由: HTML の `<select><option>` も
  検索候補の `<li role="option">` も同じ ARIA ロール `option` を持つため、種族が未解決の間に持ち物欄を
  `disabled` のまま描画すると、カード内の `role="option"` 要素数だけでは「検索候補が0件」と「持ち物の
  選択肢がある」を区別できない(テストの検証手段の制約ではなく、支援技術から見ても2つの意味が異なる
  `option` 集合が同じ親カード内に混在すること自体が紛らわしいため、実装上も分けるのが妥当と判断した)。
  種族が解決すれば通常どおり持ち物欄も出る。

### A-6. モードとマスタの結び付け(`App.tsx`)

`AppProps` に `masterSources?: (ids: ClientIds) => MasterSources` を足す(`engines` と同じ「モードごと」の形)。
端末 ID・セッション ID は App が持つ(ADR-0301 §3)ので、オンラインのマスタを組み立てられるよう関数で受ける。
**既定は変えない**(省略時は両モードとも `masterSource`、その既定は架空の例データ)。本番の組み立ては `main.tsx` が渡す。
既定を変えないのは、`engines` を渡してオンラインに切り替える既存テストが、例データのまま今までどおり動くため。

モードを切り替えたらマスタを読み直す。読み終わるまで前のモードのマスタで画面を出さない
(`masterLoad` に「どの取得口の結果か」を持たせ、現在の取得口と一致しないときは読み込み中として扱う。
`useEffect` の中で `setState` しない)。オンラインのマスタが読めなくても自動でオフラインに戻さない(ADR-0301 §4)。

### A-7. 段階の分割

- **P4-16(この spec の範囲)**: 型(A-2・A-3)、`master/onlineSource.ts`、`master/capabilities.ts`、
  `i18n/ja.ts` の `masterOnlineText`、`App.tsx` の配線(A-6)。
- **P4-16b(次)**: 画面側(A-5)。種族の検索コンボボックス、技・持ち物候補の無効化と案内の描画。
- **P4-17**: §3 の技の ID 解決が API レーンで入ったあと、`capabilities.moves` を true にして技を復活させる。

`api/openapi.yaml` はこのタスクでは変えない(必要な変更は §3 の案A = データ/API レーンの持ち物で、
DECISIONS.md で提案済み・未回答)。

### A-8. 変更する既存ファイル(implementer 向け)

| ファイル | 変更 |
|---|---|
| `web/src/master/types.ts` | 型の追加(済。`MasterData.capabilities?` は省略可なので既存の作り手に影響なし) |
| `web/src/master/capabilities.ts` | 新規(spec-writer はスタブ。実装は implementer) |
| `web/src/master/onlineSource.ts` | 新規(同上) |
| `web/src/i18n/ja.ts` | `masterOnlineText` の追加(済) |
| `web/src/App.tsx` | `masterSources` prop の追加(spec-writer は props の宣言のみ)と読み込みの配線 |
| `web/src/main.tsx` | `masterSources` を渡す(オンラインは `createOnlineMasterSource`) |
| 画面(`screens/*.tsx`) | **P4-16 では変えない**(capabilities を省いたマスタ = 今までどおり) |

## 追記2(2026-09-23、Web レーン): 画面側(P4-16b)の決定

A-1〜A-8 は P4-16(型と取得口)の正。ここから先は画面側(A-5 の実装)を書くにあたって決めたこと。
**最優先の制約は A-2 と同じ**: `capabilities` を省いたマスタ(オフラインの例データ・既存テストの fixture)では、
今までの見た目・挙動を1つも変えない。

### A-9. BalanceScreen(タイプバランス)は「使える/使えない」を画面ごと切り替える

A-5 は計算画面・逆算画面しか決めていなかったが、`web/src/screens/BalanceScreen.tsx` も
`master.species` / `master.moves` / `master.abilities` を同期的に使う(パーティ・仮想敵の
ポケモン・特性・技の選択、結果表の ID → 名前の解決)。オンラインでは `species`・`moves`・`abilities` が
空配列なので、メンバー選択欄が常に空になり画面が実質使えない。この画面についての決定が ADR に無かった。

調べて分かったこと:

- この画面の4つの診断のうち、**技に依存しないのは防御相性(analyze)だけ**。攻撃範囲(coverage)は技が要り、
  仮想敵(threats)の「与える倍率」「抜群」と、おすすめタイプ(recommendations)の「攻撃範囲の穴」も技から決まる。
- balance API の `moveIds` は `minItems: 0`(services/balance/api/openapi.yaml)なので、技が空でも
  threats / recommendations の呼び出し自体は**成功する**。ただし返るのは「与える倍率は全部ゼロ」
  「攻撃範囲の穴は18タイプ全部」という、入力不足に由来する**誤解を招く診断**になる。
- つまり技だけを無効化して残りを出すと、4機能のうち1つしか成立しないうえ、残り3つが「穴だらけ」の
  誤った結果を出す。ADR-0304 §4 の方針(壊れた結果を返すより機能を絞って正直に出す)に正面から反する。

**決定**: BalanceScreen は `capabilities.speciesList` と `capabilities.moves` が**両方 true のマスタでだけ**動かす。
どちらかが false のときは:

- 画面の先頭に `masterOnlineText.balanceUnavailable` を出す。
- パーティ・仮想敵の入力(ポケモン・特性・技1〜技4・追加・削除)は A-5 の技と同じ作法で
  **残すが全部 `disabled`** にする(欄ごと消して両モードで画面の構造を変えない)。
- balance API を1本も呼ばない(analyze / coverage / threats / recommendations のすべて)。
- `capabilities.effects` はこの判定に入れない。balance API は ID だけを送り、持ち物・特性の効果データを使わないため
  (`effects` だけが false のマスタでは、BalanceScreen は今までどおり動く)。

却下した案:

- **(a) BalanceScreen にも種族検索を広げる**: 技が無い以上4機能のうち1つしか成立せず、しかも threats /
  recommendations が誤解を招く結果を出す。検索欄を6枠 × 2(パーティ・仮想敵)へ広げる実装コストも小さくない。
  P4-17 で技が戻れば同じ作業を「意味のある形で」行えるので、今やる理由が無い。
- **(c) 技に依存する表示だけ隠す**: 「メンバーは選べるのに診断がほとんど出ない」画面になり、
  なぜ出ないのかが利用者に伝わらない(技の欄が空である理由も伝わらない)。

**P4-17 への申し送り**: `capabilities.moves` を true にするときは、BalanceScreen にも A-10 の種族検索を広げる
(パーティ・仮想敵の各枠)。そのとき**オンラインは BalanceScreen にとって「正しい方の」モード**になる:
balance-svc は自前の read model を持ち、オフラインの架空の例データの key(`9001-000` など)が今そこで
引けるのは P4-12a で例データを read model へ書き出したからで、実データの key を送るオンラインの方が本来の姿になる。

### A-10. 画面に検索を渡す経路と、種族の検索欄の見え方

`ScreenProps`(`web/src/app/screens.tsx`)に `masterSearch?: MasterSpeciesSearch` を足す。App は今選ばれている
取得口が検索付きなら(`isSearchableMasterSource`)その `search` を渡し、そうでなければ渡さない。
`MasterSource` / `MasterData` の型は変えない(P4-16 で確定済み)。**省略可**なので、既存の画面テストの
`<CalcScreen engine master />` のような使い方は1行も変えずに今までどおり動く。

`capabilities.speciesList === false` のとき、ドロップダウンの代わりに出す検索欄:

- **accessible name は今までのスロットのラベルのまま**(「攻撃側のポケモン」「防御側のポケモン」
  「自分のポケモン」「相手のポケモン」)。役割も `combobox` のまま(`<select>` から `role="combobox"` の
  テキスト入力へ)なので、利用者・テストからの引き方が両モードで変わらない。
- `placeholder` に `masterOnlineText.speciesSearchLabel`、補足の `speciesSearchHint` は `aria-describedby` で結ぶ。
- 候補は `role="listbox"` の中の `role="option"`(WAI-ARIA Authoring Practices の Combobox パターン。
  入力欄は `aria-expanded` と `aria-controls` を持つ)。
- パラメータは A-4 の定数をそのまま使う。**`SPECIES_SEARCH_DEBOUNCE_MS` の使い手がこの検索欄**
  (P4-16 の積み残し(4)「未使用なら削除か使用を確認」への答え: 使う)。
- 状態ごとの文言: 入力前 `speciesSearchEmpty` / 0件 `speciesSearchNoResult` / 上限に達した
  `speciesSearchTruncated` / 検索・解決の失敗 `speciesSearchFailed`。
- 入力が変わったら前の検索を `AbortController` で取り消し、古い応答で新しい候補を上書きしない
  (画面の他の非同期処理と同じ作法。CalcScreen.tsx の `cancelled` と同じ考え方)。
- 候補を選んだら `resolveSpecies(key)` を引き、**種族と一緒に返る特性を画面が覚える**(A-1 後半)。
  `MasterData.abilities` は空のままなので、`defaultAbility` に渡すのは画面が覚えた特性になる。
- `speciesList === false` なのに `masterSearch` が渡されていない(組み合わせの誤り)ときは、空のドロップダウンを
  出さずに検索欄を `disabled` にして `speciesSearchFailed` を出す(選択肢ゼロの欄を黙って出さない)。

### A-11. 逆算画面の持ち物候補(A-5 の補足)

A-5 は「逆算画面の持ち物候補を `disabled` にする」と書いたが、実際の `ReverseScreen` に持ち物候補の**操作**は無い
(`domain/reverseItems.ts` の `reverseItemCandidates` が `master.items` の効果データから候補を自動で組む)。
`capabilities.effects === false` では効果データが無いので候補は「持ち物なし」だけになる。これは壊れた結果ではなく
**前提の狭い正しい結果**なので、逆算そのものは実行し、`masterOnlineText.itemCandidatesUnavailable` を添えて
「持ち物の候補を探索していない」ことを明示する。計算画面はトグルという操作があるので、A-5 のとおり `disabled` にする。

### A-12. 検索欄のキーボード操作と見た目(P4-16c。A-10 の具体化)

A-10 は「WAI-ARIA Authoring Practices の Combobox パターンに合わせる」とだけ書いたが、P4-16b の実装は
`onClick` だけで、キーボードでは候補を選べなかった(critic 指摘)。パターンのうちどの形を採るかは
他に合わせるべき既存 UI が無いので、ここで決める(P4-16c の受け入れ条件。テストで固定する)。

**採るのは "List Autocomplete with Automatic Selection"**(候補が出たら先頭が選ばれている形)。
入力の途中でも Enter だけで一番上の候補を確定でき、`aria-selected` が常にちょうど1件 true になる。

- `ArrowDown` / `ArrowUp`: 1件ずつ移動し、**端で止まる**(ループしない)。候補が入れ替わるたびに先頭へ戻す。
- `Enter`: ハイライト中の候補を確定する(`selectCandidate`)。候補が出ていなければ何もしない。
- `Escape`: 候補を閉じる。**入力の文字は消さない**。閉じたときに候補は捨て、`ArrowDown` では戻さない
  (検索はデバウンス付きの非同期なので、復活させると入力文字と候補がずれたまま出る。入力を変えれば再検索される)。
- ハイライトは3つで示す: 入力欄の `aria-activedescendant`、候補の `aria-selected`、見た目のクラス
  (`species-search__option--active`)。
- `aria-controls` は候補を出しているときだけ付ける(閉じているときに付けると IDREF の参照先が DOM に無い)。
- マウスのクリックは今までどおり(キーボードと排他ではない)。フォーカスが外れたときに閉じる処理は**入れない**
  (blur はクリックより先に起きるため、閉じると既存のクリックでの選択を壊す)。

見た目(`web/src/screens/SpeciesSearchField.css`)は docs/design.md のトークンだけで書く
(入力の角丸 `--radius-input`、余白 `--space-*`、色は `--text-*` / `--border-hairline` / `--bg-*`)。
候補一覧は素の箇条書きの既定を消し、最大の高さ + `overflow-y: auto` にする(`SPECIES_SEARCH_LIMIT` = 50件出るため)。
常時動くアニメーションは入れない(CLAUDE.md ドメイン規約)。

**追記(P4-16c 実装後。critic 指摘): IME 変換中は上のキー操作を素通しする**。この欄は日本語の種族名を打つ欄で、
`event.nativeEvent.isComposing === true` のときの `Enter`/矢印キーは IME の変換操作であって候補選択ではない
(`onKeyDown` の先頭でガードし、何もせず return する)。無視すると変換確定の Enter で
ハイライト中の候補を誤って選んでしまう。`preventDefault()` も「実際に処理したとき」だけ呼ぶ(候補が無いときの
キャレット移動等の既定動作を妨げない。`App.tsx` の `handleTabKeyDown` と同じ作法)。
