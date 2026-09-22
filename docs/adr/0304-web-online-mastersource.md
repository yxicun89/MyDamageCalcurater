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
