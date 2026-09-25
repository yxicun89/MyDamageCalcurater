# ADR-0304: Web のオンライン MasterSource — 検索ベースの選択 UI と、技の ID 解決の API ギャップ

- 状態: 提案(Web レーン、2026-09-23。§3 の技解決の欠落は **2026-09-24 に API レーンが `GET /api/pokedex/moves/batch`
  〈`getMovesByIds`〉を新設して解決済み**〈§3 末尾の追記・却下・保留節を参照。当初推していた案A は不採用〉。
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

- **案A(既定として推す。→ 2026-09-24 不採用。理由・却下の詳細は本節末尾の追記と「却下・保留」節)**:
  `getSpecies` の応答の `learnset` を、ID配列 (`string[]`) から `Move` 実体の配列に変える
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

**追記(2026-09-24。決定): 案Aではなく `getMove` のバッチ解決を採用した**。この ADR の当初の推奨(§3 冒頭)は
案A(`learnset` を `Move[]` にする)だったが、その後 iOS レーン(M3)が `SpeciesDetail.learnset` を `string[]`
のまま前提にした**実際に動く機能**を先に作り込んでいた(`ios/PokeCalcKit/Sources/PokeCalcCore/CalcViewModel.swift`・
`ReverseViewModel.swift`・`TeamEditViewModel.swift` が `detail.learnset` を ID 集合として扱い、別途検索した
技候補との積集合を取る設計。XCTest 多数でカバー済み)。この状態で `learnset` の型を破壊的に変えると、
Web がまだ使っていない機能のために、iOS の**既に完成し出荷済みの**機能を壊して書き直させることになり、
「契約変更が小さい方」という基準では案Aの方が明らかに大きい変更になった。
`GET /api/pokedex/moves/batch?ids=...`(`getMovesByIds`)を新設して解決した: `learnset`(またはその他の ID 配列)を
1回の呼び出しで `Move[]` に解決できる。既存の `SpeciesDetail.learnset` は無変更なので iOS への影響はゼロ。
ids は1〜64件。64 という値自体は根拠が無い数字ではなく issue #110 の教訓(候補・観測配列には必ず上限を置く。
ADR-0208)を踏まえた保守的な初期値で、ADR-0208 の itemVariants/itemCandidates とは「1回のクエリで増幅させない」
という考え方だけを借りている(持ち物の分類数を根拠にした64ではない)。**技の learnset の実際の分布は当初
実データ検証していなかったが、2026-09-24 に実クラスタで確認した(下記追記)**。契約の `maxItems` は生成ラッパが検証しないため
`services/pokedex/internal/httpapi/search.go` で自前検査(ADR-0208 の前例)。見つからなかった
ID は黙って省き、応答の順序は `ids` と同じにする(DB の `IN` 句は順序を保証しないためハンドラで並べ替える)。
`getMove` と同様に既定のレギュレーションで絞らない。ADR-0105 §3 に追記。
Web レーンへ: `getSpecies` の `learnset`(ID配列)を `GET /api/pokedex/moves/batch?ids=<learnsetのID一覧>`
に渡せば `Move[]` が返る。**追記(2026-09-24。実クラスタで確認)**: `learnset` が64件を超える種族は**まれではない**。
既定のレギュレーション(M-C)で実測したところ、349種族中 **151種族(43%)** が64件を超え、最大は
**106件**(図鑑番号0475、メガ進化フォームも同数)だった(§3 が当初書いていた「20〜30件」という目算は
大幅に外れていた)。したがって **Web 側の分割呼び出しは必須の実装**であり、まれな例外処理ではない。
`learnset` の件数が64件を超えていたら64件ずつに分割して複数回呼ぶ実装にすること(1回で必ず収まる
という前提を置かない)。これで§4 段階3の「技はオンライン未対応」を解消できる。

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
- **案A(`getSpecies.learnset` を ID配列から `Move[]` に変える。2026-09-24 却下)**: §3 が当初「既定として推す」と
  していた案。却下理由: iOS レーン(M3。既に main 統合・出荷済み)の `CalcViewModel`・`ReverseViewModel`・
  `TeamEditViewModel` が `SpeciesDetail.learnset` を `string[]` のまま前提にした機能(learnset を ID 集合として
  保持し、別途検索した技候補との積集合を取る設計。XCTest で広くカバー済み)を既に作り込んでおり、`learnset` の
  型を破壊的に変えると Web がまだ有効化していない機能のために iOS の完成済み機能を壊すことになる。「契約変更が
  小さい方」という基準では、代わりに採用した `GET /api/pokedex/moves/batch`(`getMovesByIds`。ADR-0105 §3・
  §3 末尾の追記)の方が明らかに小さかった。**この却下理由が解消しない限り(= iOS が learnset の型変更を許容する
  形に作り直されない限り)案Aを復活させない**。

## 影響

- `web/src/master/`: 新しい `MasterSource` 実装(検索ベースの種族選択、持ち物・性格は全件取得、技は当面据え置き)。
- `web/src/App.tsx`: オンラインモード選択時の `masterSource` の差し替え(ADR-0301 §4 の既定見直しの一部)。
- 画面側: 種族選択 UI がオンライン/オフラインで一覧/検索に分かれる(コンポーネントの分岐が増える)。
- データ/API レーンへの依頼(DECISIONS.md に転記): 当初は `getSpecies.learnset` を `Move[]` に変える案A を既定で
  提案した(**2026-09-24 追記: 不採用。`GET /api/pokedex/moves/batch` を新設して解決した。理由は §3 末尾の追記と
  「却下・保留」節を参照**)。返答があるまで、Web のオンラインモードは技選択を無効化した状態で先に進める。

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
  **(2026-09-24 追記: この行は A-13 で置き換えた。`capabilities.moves` は false のままにし、技は
  `resolveSpecies` が種族と一緒に解決する。理由は A-13.1)**

`api/openapi.yaml` はこのタスクでは変えない(必要な変更は §3 の技のID解決 = API レーンの持ち物。
当時は案A を提案していたが、2026-09-24 に `GET /api/pokedex/moves/batch` の新設で解決済み。§3 末尾の追記参照)。

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

## 追記4(2026-09-24、Web レーン): P4-17 — 技の復活(§3 の欠落の解消を受けて)

API レーンが `GET /api/pokedex/moves/batch`(`getMovesByIds`)を新設し(§3 末尾の追記・ADR-0105 §3)、
§4 段階3の「技はオンライン未対応」の前提が消えた。**A-7 の「P4-17: `capabilities.moves` を true にして技を
復活させる」は、この A-13 で置き換える**(true にはしない。理由は A-13.1)。

### A-13. 技は「種族と一緒に解決する」。`capabilities.moves` の意味は変えない

#### 1. `capabilities.moves` は false のまま(意味を変えない)

この項目の意味は `speciesList` と対になる「`MasterData.moves` が**全件**そろっているか」であり、
`searchMoves` の limit 上限200 < 実データ515件という §1 の事実は、技の ID 解決が入っても変わらない。
`getMovesByIds` が解決するのは「**既に確定した ID の集合**」だけで、全件の一覧ではない。

true にする案を却下した理由: `capabilities.moves` は BalanceScreen の `balanceAvailable`(A-9)も決めている。
true にすると BalanceScreen は「有効」に見えるのに `master.moves` が空のままで、各枠の技セレクトが
1件も選べない**壊れた状態**になる(A-9 が避けたかったものそのもの)。この画面の技検索 UI は P4-17b に送る
(下の 5)。

#### 2. 技セレクトの有効・無効は「いま技の候補があるか」で決める

A-5 は技セレクトの `disabled` を `capabilities.moves === false` に結び付けていたが、P4-17 からは
**攻撃側の種族が解決済みなら技を選べる**ので、この条件では足りない。判定を1つに決める:

- **技セレクトが使えるのは `capabilities.moves === true`、または攻撃側の技の候補が1件以上あるとき**
  (`learnsetMoves(攻撃側の種族, 解決済みの技を足した一覧)` が空でないとき)。
- `masterOnlineText.movesUnavailable` は、**技セレクトが `disabled` のときとちょうど同じ条件**で出す
  (A-5 の「欄は残して disabled + 案内」の作法は変えない)。文言はオンラインかどうかに触れない形に改めた
  (画面はモードの名前を持たない。A-2)。
- 結果として「種族が未選択」「検索で選んだ直後、技がまだ届いていない間(解決中)」「解決したが技を
  1つも覚えない」の3つで disabled になる。オフライン相当のマスタ(`capabilities` 省略)は今までどおり
  常に有効で、この変更の影響を受けない。
- 技は**常に攻撃側**の learnset から選ぶ(ADR-0010)。攻守入れ替え(計算画面)と「与えた/受けた」の
  切り替え(逆算画面)では、技の出どころも新しい攻撃側に付いて変わる。

#### 3. 技は `resolveSpecies` が種族・特性と一緒に返す(`MasterSpeciesResolution.moves`)

種族の検索・解決の経路(A-10)をもう1本増やさない。1回の `resolveSpecies` で種族・特性・技がそろう。
画面は `speciesResolution.ts`(A-10 の覚え書き)に `movesFor(master.moves, key)` を足し、
`abilitiesFor` と同じ形で「全件の一覧 + 解決で覚えた分」を返す(`domain/moves.ts` の `learnsetMoves` は無変更)。

**技の解決に失敗したら `resolveSpecies` 全体を失敗させる**(種族・特性だけ返さない)。技の無い種族を
選べてしまうと、その後の計算・逆算が「技が選べないのに種族だけ入っている」壊れた状態になる。
§4 の「壊れた結果を返すよりは機能を絞って正直に出す」に合わせ、種族の選択自体を失敗として
`speciesSearchFailed` を出す(検索欄の既存の失敗表示。A-10 の経路をそのまま使う)。

#### 4. 64件ずつに分割して呼ぶ(1回で収まる前提を置かない)

`ids` は契約上1〜64件(`MOVES_BATCH_MAX_IDS`。65件以上・省略は 400 `invalid_input`)。
**1種族の learnset が64件に収まる保証は無い**(API レーンも実データ未確認。§3 末尾の追記の明示の依頼)ので、
`learnset` を64件ずつに分けて複数回呼び、応答をチャンクの順につなぐ。分割した呼び出しは**並列でよい**
(1種族の解決の中の話で、issue #110 が懸念した「1リクエストでの増幅」には当たらない)。
`AbortSignal` は**全チャンクに同じものを渡す**(1回の `resolveSpecies` として取り消せるように)。
`learnset` が空なら1回も呼ばない(`ids` の省略は 400)。
定数 `MOVES_BATCH_MAX_IDS` は契約の `maxItems` と一致することをテストで固定する
(pokedex-svc 側の同期テストと同じ考え方)。

#### 5. BalanceScreen は据え置き(P4-17b へ)

A-9 の決定(`speciesList` と `moves` が両方 true のマスタでだけ動かす)は**そのまま**。1 のとおり
`capabilities.moves` は false のままなので、オンラインでは今までどおり画面ごと無効化され、
balance API も呼ばない。A-9 の「P4-17 への申し送り」(各枠に種族検索を広げる)は **P4-17b** として
`docs/plan.md` に積み残す。この画面は枠ごとに技を**4つまで**選ぶので、種族検索 + 種族ごとの技解決を
6枠 × 2(パーティ・仮想敵)へ広げる設計が別途要る(計算画面の「攻撃側1体ぶん」とは規模が違う)。
「有効なのに技が選べない」状態にしないことは回帰テストで固定した(`BalanceScreen.online.test.tsx`)。

#### 6. 変更するファイル(implementer 向け)

| ファイル | 変更 |
|---|---|
| `web/src/master/types.ts` | `MasterSpeciesResolution.moves`(済。spec-writer)・`capabilities.moves` の doc |
| `web/src/master/onlineSource.ts` | `MOVES_BATCH_MAX_IDS`(済)、`resolveSpecies` の技解決(分割呼び出し) |
| `web/src/screens/speciesResolution.ts` | `movesFor` を足す(`abilitiesFor` と同じ形) |
| `web/src/screens/CalcScreen.tsx` | `movesFor` を使う(`attackerMoves`・`resolveMoveId` の呼び出し3か所・攻守入れ替え)、技セレクトの `disabled` と案内の条件(2) |
| `web/src/screens/ReverseScreen.tsx` | 同上(`moveOptions`・`selectSide`・`selectMySpecies`/`selectTheirsSpecies`・`handleMineResolved`/`handleTheirsResolved`) |
| `web/src/i18n/ja.ts` | `movesUnavailable` の文言(済。spec-writer) |
| `web/src/screens/BalanceScreen.tsx` | **変えない**(5。回帰テストのみ) |
| `web/src/domain/moves.ts` | **変えない**(呼び出し側が渡す一覧を変えるだけ) |

`api/openapi.yaml`・`services/`・`engine/`・`ios/` はこのタスクで変えない(API レーンが対応済み)。

## 追記5(2026-09-24、Web レーン): P4-17b — BalanceScreen の種族検索・技選択

A-13.5 で P4-17b に積み残した「BalanceScreen にも種族検索を広げる」の設計。A-9 の決定(`speciesList` と
`moves` が両方 true でなければ画面ごと無効)を**この A-14 で置き換える**。A-13.1 の決定(`capabilities.moves`
は false のまま)は変えない。

### A-14. BalanceScreen は「入力の口があるか」で使える/使えないを決める

#### 1. ゲート条件を「一覧がそろっているか」から「入力の口があるか」に変える

A-9 の `capabilities.speciesList && capabilities.moves` は、A-13.1 で `capabilities.moves` が永続的に false と
決まった結果、**オンラインではこの画面が永久に使えない**ことを意味していた。A-9 が避けたかったのは
「技を1つも選べないまま threats / recommendations を呼び、全部ゼロ・穴だらけの診断を出す」ことだが、
P4-17 で技は種族の解決と一緒に届くようになった(A-13.3)ので、オンラインで種族を選んだメンバーは
**オフラインで「まだ技を選んでいないメンバー」と同じ状態**に帰着する。これは今のオフラインでも普通に起きる
状態(種族だけ選んで threats を呼ぶ)であり、A-9 の懸念はもう当てはまらない。残るのは
「技をどうやっても選べない」マスタだけで、そこは今までどおり止める。

**決定**: 画面の可否は次の式で決める(`capabilities` は `masterCapabilities(master)`、
`masterSearch` は props)。

```ts
// 種族を選ぶ口: 全件の一覧(ドロップダウン)か、検索欄(A-10)。
const speciesInputAvailable = capabilities.speciesList || masterSearch !== undefined;
// 技を選ぶ口: 全件の一覧か、検索で種族と一緒に届く技(A-13.3)。後者は「種族を検索で選ぶ」経路でしか
// 届かないので、speciesList が true(ドロップダウン)のときは検索口があっても技は届かない。
const moveInputAvailable = capabilities.moves || (!capabilities.speciesList && masterSearch !== undefined);
const balanceAvailable = speciesInputAvailable && moveInputAvailable;
```

この式が決める境界(テストで固定する):

| `speciesList` | `moves` | `masterSearch` | 画面 | 理由 |
|---|---|---|---|---|
| true | true | 任意 | **使える** | オフライン相当。今までどおり(`capabilities` 省略を含む) |
| false | false | あり | **使える** | **P4-17b で変わるところ**。種族は検索、技は解決と一緒に届く |
| false | false | なし | 使えない | 種族を選ぶ口が無い(組み合わせの誤り。A-10 と同じ扱い) |
| false | true | なし | 使えない | 同上 |
| false | true | あり | 使える | 種族は検索、技は全件の一覧 |
| true | false | なし | 使えない | 技をどうやっても選べない(A-9 の懸念がそのまま残る形) |
| true | false | あり | 使えない | 種族はドロップダウンで選ぶので `resolveSpecies` が走らず、技が永久に届かない |

`balanceAvailable` が false のときの見せ方は A-9 のまま変えない(`masterOnlineText.balanceUnavailable` を出し、
入力は残すが全部 `disabled`、balance API を1本も呼ばない)。`capabilities.effects` を判定に入れないのも A-9 のまま。

却下した案:

- **(a) `balanceAvailable` を丸ごと廃止し、常に画面を動かす**: `speciesList: true / moves: false / masterSearch なし`
  のマスタで「技の欄が永久に空なのに診断は出る」状態が残り、A-9 の懸念がそのまま再発する。
- **(b) `masterSearch !== undefined` だけを見る**: 上の表の最終行(ドロップダウン + 検索口)を取りこぼす。
  この組み合わせでは検索欄が描かれないので `resolveSpecies` が1度も走らず、技が届かない。
- **(c) 「技を選んだメンバーが1人もいないと threats / recommendations を呼ばない」に変える**: 呼び出しの条件を
  ADR-0400 §1・ADR-0303 §7 から動かすことになり、**オフラインの既存の挙動が変わる**(A-2 の最優先の制約に反する)。

`masterOnlineText.balanceUnavailable` の文言は、条件が「オンラインかどうか」から「入力の口があるか」に変わった
ので、モードの名前に触れない形に直す(A-2)。

#### 2. 12枠(メンバー6 + 仮想敵6)を独立に解決する

`useSpeciesResolutions()`(A-10・A-13.3)の覚え書きは `Map<speciesKey, MasterSpeciesResolution>` なので、
**1画面に1つ持てば12枠で共有できる**(枠ごとに持つ必要は無い。同じ種族を2枠で選んでも1件で足りる)。
`speciesResolution.ts` は変更しない。

- `MemberFields` に `speciesListAvailable` / `masterSearch` / `onSpeciesResolved` を足し、CalcScreen の
  `SpeciesCard`(A-10)と同じく `speciesListAvailable ? <select> : <SpeciesSearchField label={speciesLabel} …>` で
  出し分ける。accessible name は今までのラベル(`balanceScreenText.speciesLabel` =「ポケモン」)のままにする。
- `MemberFields` が種族・特性・技を引くのは `master.*` からではなく、画面から渡す
  `speciesFor(master.species, member.speciesKey)` / `abilitiesFor(master.abilities, member.speciesKey)` /
  `movesFor(master.moves, member.speciesKey)` に変える(4 も参照)。
- `useMemberListActions` に `resolveSpecies(index, resolution)` を足す。既存の `selectSpecies` と同じく
  種族 key・既定の特性(`resolution.species.abilities[0] ?? ""`)を入れ、技の枠は空に戻す。
  **`movesFor` / `abilitiesFor` をこの中で呼ばない**: `register()` の `setState` は非同期で、直後はまだ古い
  覚え書きのままだから(CalcScreen の `handleAttackerResolved` と同じ理由)。必要な実体は `resolution` が持っている。
- 画面側のハンドラは `registerSpeciesResolution(resolution)` と `memberActions.resolveSpecies(index, resolution)`
  (仮想敵は `threatActions`)を呼ぶ。メンバーと仮想敵で同じ `MemberListActions` の形を保つ(ADR-0303 §7)。
- 結果表の ID → 名前(`findSpeciesName` / `findAbilityName`)も `speciesFor` / `abilitiesFor` を通す。
  オンラインで `master.species` が空だと、選んだメンバーの行見出しが key(`9001-000`)のまま出てしまうため。
  おすすめタイプの候補のように**利用者が選んでいない**種族は解決されていないので、今までどおり
  応答の `nameJa`(あれば)か ID を出す。

#### 3. `moveById` は「選んだメンバーが引ける技」から組み、不明な ID は攻撃技と見なさない

現在の `hasDamagingMove`(coverage を呼ぶかどうかの判定)は `master.moves` の全件から作った Map で技の
分類を引き、`moveById.get(moveId)?.category !== "status"` と書いている。この式は**技が見つからないとき
`undefined !== "status"` が true になり、「変化技ではない = 攻撃技」と誤判定する**。オンラインでは
`master.moves` が空なので、変化技しか選んでいないメンバーでも coverage を呼んでしまう
(=「攻撃技が無いのに攻撃範囲の診断を出す」。まさに A-9 が避けたかった誤解を招く診断)。

**決定**: 判定を2か所直す。

```ts
// (1) 参照する一覧を、選んだメンバーが実際に引ける技にする(master.moves + 解決で覚えた分)。
const moveById = new Map(
  members.flatMap((member) => movesFor(master.moves, member.speciesKey)).map((move) => [move.id, move]),
);
// (2) 実体が分からない ID は攻撃技と見なさない(fail-closed)。
const hasDamagingMove = members.some((member) =>
  member.moveIds.some((moveId) => {
    const move = moveById.get(moveId);
    return move !== undefined && move.category !== "status";
  }),
);
```

`speciesResolution.ts` に「解決済みの技を全部返す」accessor を足す案は採らない: 必要なのは
**今このパーティが選びうる技**だけで、12枠すべての解決結果をかき集めると仮想敵の種族の技まで混ざる
(coverage は自分のパーティの話。ADR-0303 §7)。`movesFor` を枠ごとに呼んで合成すれば過不足が無い。

fail-closed(実体不明を攻撃技と見なさない)にする理由: 技セレクトの選択肢は解決済みの learnset からしか
作られないので、実体不明の ID は本来現れない。現れたなら入力側が壊れているので、**呼ばない**方が
「壊れた結果を返すより機能を絞って正直に出す」(§4)に合う。

#### 4. 技セレクトの選択肢と有効・無効(A-13.2 をこの画面に当てはめる)

- 選択肢は `learnsetMoves(species, movesFor(master.moves, member.speciesKey))`(今は `master.moves` を
  直接渡しているので、検索で解決した技が1件も出ない)。
- 枠ごとの有効・無効は A-13.2 と同じ式: `capabilities.moves || その枠の技の候補が1件以上`。
  オフライン(`capabilities` 省略)は `capabilities.moves === true` なので**今までどおり常に有効**で、
  「種族未選択でも技の欄は押せる(選択肢は『なし』だけ)」という既存の見え方は1つも変わらない。
- **`masterOnlineText.movesUnavailable` はこの画面では出さない**(A-13.2 の「disabled と同じ条件で案内を出す」
  から意図的に外れる)。枠が12個あるので、同じ案内が最大12回並んで画面が読めなくなる。この画面では
  技の欄が `fieldset`(「メンバーn」)の中でポケモンの欄と並んでおり、ポケモンが空 → 技が空、の対応が
  その場で読み取れる。計算画面は技の欄が1つで、かつ攻撃側のカードから離れているので案内が要る、という違い。

#### 5. 変更するファイル(implementer 向け)

| ファイル | 変更 |
|---|---|
| `web/src/screens/BalanceScreen.tsx` | ゲート条件(1)、`masterSearch` を使う、`useSpeciesResolutions` の導入、`MemberFields` の出し分け(2)、`moveById`(3)、技セレクト(4)、結果表の名前解決(2) |
| `web/src/i18n/ja.ts` | `balanceUnavailable` の文言(済。spec-writer) |
| `web/src/screens/speciesResolution.ts` | **変えない**(2: Map なので12枠で共有できる) |
| `web/src/screens/SpeciesSearchField.tsx` | **変えない**(`masterSearch === undefined` の自己無効化だけで足りる) |
| `web/src/domain/moves.ts` | **変えない**(呼び出し側が渡す一覧を変えるだけ) |
| `web/src/app/screens.tsx` / `web/src/App.tsx` | **変えない**(`masterSearch` は既に全画面へ渡している) |

`api/openapi.yaml`・`services/`・`engine/`・`ios/` はこのタスクで変えない。

## 追記6(2026-09-25、Web レーン): issue 308 ― マスタの読み込みに失敗したときの立て直し

A-6 は「読み終わるまで前のモードのマスタで画面を出さない」「オンラインのマスタが読めなくても自動でオフラインに
戻さない」を決めたが、失敗した**あとの立て直し**(再試行・タブからの離脱・オフラインへの切り替え)は決めていなかった。
実装は `currentMasterLoad.ok === false` のとき `role=alert` の1行(`appText.masterLoadError`)だけを出し、タブ一覧
ごと消していた。この状態からは、ページを再読み込みする以外に抜け出す手段が無かった(issue 308)。

### A-15. 失敗中もタブ一覧は残し、マスタを使わない画面はその場で使える

- 失敗時(`currentMasterLoad !== null && !currentMasterLoad.ok`)も `role="tablist"` は描画したままにする。
  タブの選択・キーボード操作(`navigateToTab`・`handleTabKeyDown`)は元々マスタの状態を見ていないので、
  そのまま動く。
- どの画面がマスタを使うかを `app/routes.ts` の `SCREEN_ROUTES` に `usesMaster: boolean` として1列足し(既存の
  `segment`・`label` と同じ「1か所の正」)、`screenUsesMaster(id)` / `isMasterlessScreen(id)`(型ガード)を添える。
  現時点で `usesMaster: false` は素早さ(`speed`)だけ(ADR-0604 §5 が元々「engine・master のどちらも使わない」と
  決めていた画面)。
- 選択中のタブが `usesMaster: false` なら、失敗中でも `ActiveScreen` 相当の画面を描画する。`usesMaster: true` の
  4画面(計算・逆算・タイプバランス・判定)が選ばれているときだけ、失敗の案内(下記 A-17)を出す。

### A-16. `ScreenProps.master` は変えない。マスタ不要の画面だけ別の型で受ける

`ScreenProps.master` を省略可(`master?: MasterData`)にする案を最初に試したが、`SCREEN_COMPONENTS:
Record<ScreenId, ComponentType<ScreenProps>>` に `CalcScreen` 等(それぞれ独立に `master: MasterData` を**必須**で
宣言している `CalcScreenProps` 等)を代入する箇所で型エラーになった(関数コンポーネントの引数は反変チェックされ、
`ScreenProps.master` を省略可にすると「`undefined` も来うる」型を要求する側に、`master` を必須のまま受け取る側を
割り当てられなくなるため)。`calc`/`reverse`/`balance`/`judge` の4画面の型・実装を壊さない、という依頼のとおり
これらは変えず、代わりに次の設計にした:

- `ScreenProps` は変えない(`master: MasterData` のまま)。
- `MasterlessScreenProps = Omit<ScreenProps, "master">`(`master` キーそのものを持たない型)を新設し、
  `MASTERLESS_SCREEN_COMPONENTS: Record<MasterlessScreenId, ComponentType<MasterlessScreenProps>>` に
  `usesMaster: false` の画面(今は `speed: SpeedScreen`)だけを登録する(`app/screens.tsx`)。
  `MasterlessScreenId` は `SCREEN_ROUTES` の `usesMaster: false` の行から `Extract` で導出するので、
  今後 `usesMaster: false` の画面を足して `MASTERLESS_SCREEN_COMPONENTS` への登録を忘れると型エラーになる
  (`SCREEN_COMPONENTS` が担ってきた「足し忘れは型エラー」という既存の性質をこちらにも及ぼした)。
- `App.tsx` は `isMasterlessScreen(tab)` で `tab: ScreenId` を `MasterlessScreenId` に絞り込んでから
  `MASTERLESS_SCREEN_COMPONENTS[tab]` を引く。`as` によるキャストは使わない。

### A-17. 失敗の案内: 原因・再試行・(オンラインのときだけ)オフラインに切り替える

`role="alert"` の中に、`appText.masterLoadError`(見出し)・`appText.masterLoadErrorDetailLabel`(「原因」)+
`error.message`(握りつぶさずそのまま出す)・「再試行」ボタン・オンラインのときだけ出す「オフラインに切り替える」
ボタンを置く(`App.tsx` の `MasterLoadFailureNotice`)。

- **再試行**は `activeMasterSource`(今の取得口。オンラインならオンラインのまま)をもう一度読み直す。
  `activeMasterSource` 自体はモードが変わらない限り参照が変わらないため、依存配列に載せるだけでは
  `useEffect` を再実行できない。値そのものに意味の無い `retryToken`(数値、押すたびに +1)を追加の依存に足し、
  ボタンから `setRetryToken` するだけの最小限の実装にした。
- **オフラインに切り替える**は、ヘッダーの計算モードのラジオと同じ `selectMode("offline")` をそのまま呼ぶ
  (自動フォールバックではなく、利用者の操作による切り替え。A-6 の方針は変えない)。オフラインのマスタ自体が
  読めない(`mode === "offline"` で失敗)ときはこのボタンを出さない(条件は `mode === "online"`)。

却下した案:

- **自動リトライ(指数バックオフ等)**: 依頼の範囲外であり、ADR-0301 §4「オンラインのマスタが読めなくても
  自動でオフラインに戻さない」と同じ理由(失敗の原因を利用者に見せず裏で状態を変えると、原因不明の待ちが増える)
  で今回も見送った。「再試行」は利用者の操作でだけ起きる。
- **`ScreenProps.master` を省略可にする**: A-16 で述べたとおり、型エラーで断念した。

## 影響(追記6)

- `web/src/app/routes.ts`: `ScreenRoute.usesMaster`、`SCREEN_ROUTES` 各行、`screenUsesMaster`・
  `isMasterlessScreen`・`MasterlessScreenId` を追加。
- `web/src/app/screens.tsx`: `MasterlessScreenProps`・`MASTERLESS_SCREEN_COMPONENTS` を追加(`ScreenProps` 自体は無変更)。
- `web/src/App.tsx`: `retryToken` state・`retryMasterLoad`・`MasterLoadFailureNotice`、失敗時のタブ描画の分岐。
- `web/src/App.css`: `.app-master-error` 系のクラスを追加(色は `tokens.css` の変数のみ参照)。
- `web/src/i18n/ja.ts`: `masterLoadErrorDetailLabel`・`masterLoadRetryLabel`・`masterLoadSwitchToOfflineLabel`
  (spec-writer が追加済み)。
- `web/src/app/screens.tsx` / `web/src/App.tsx` 以外の画面ファイル(`CalcScreen.tsx` 等)は無変更(A-16)。
