# ADR-0326: Web の持ち物欄を役割で絞り、メガストーンを「{基本種名}のメガストーン」と表示する

- 状態: 採用
- 日付: 2026-10-03
- レーン: Web
- 関連: ADR-0175(持ち物の役割とメガストーン判定。特に §4 のクライアント規約)、ADR-0320(Web のメガ持ち物固定)、
  ADR-0323(画面レジストリ・文言のレーン別ファイル)、ADR-0313(マスタのキャッシュ)、ADR-0319(調整画面)、
  ADR-0321(境界の持ち物規則)、docs/mega-evolution-spec.md §4-3

## 背景(2026-10-03 のユーザー報告)

1. メガストーンが英語表記(`Absolite Z`・`Meganiumite` など)で持ち物の選択肢に出る。
2. 攻撃側にオボンのみ、ルカリオにボーマンダナイトのような、その側で意味の無い持ち物を選べる。
3. 持ち物はすべて日本語にしたい。
4. メガ種族を選ぶと持ち物はメガストーンに固定(#537・#540 で実装済み。ADR-0320)。

PR #579(ADR-0175)で pokedex-svc が `Item.roles`(`attacker`/`defender`。メガストーンは空)・`Item.isMegaStone`、
`SpeciesDetail.baseSpeciesKey`・`baseSpeciesNameJa` を返すようになった。上流に日本語名の無い新しいメガストーン 40 件は
英語のままなので、名前を推測せず、ストーンは選択肢に出さず、固定中は日本語の文言で表示する。

## 決定

### 1. 絞り込みの関数は `web/src/domain/itemRoles.ts` の1か所

計算・逆算・判定・調整・構築の編集の**すべての持ち物欄**が `itemsForRole(items, filter, stoneIds?)` で選択肢を作る。

- `filter`(`ItemRoleFilter`)は `attacker` / `defender` / `either`(どちらかの役割を持つ = `roles` が空でない)/ `any`(役割で絞らない)。
- 役割の判定は `roles` だけを読む。**`roles` が無い持ち物(古いサーバー・古いキャッシュ・例データ)は役割で絞らない**
  (ADR-0175 §4。効果から再導出しない)。`roles: []` は「役割が無い」で絞る。判定は持ち物ごと。
- **メガストーンはどの欄にも出さない**(`any` でも)。判別は `isMegaStone`(サーバーの判定が正。`false` と明示されていればそれに従う)、
  `isMegaStone` が無い持ち物だけ `megaStoneItemIds`(種族から導く集合。ADR-0320 §7)で補う。
  `isMegaStone` があれば、まだ解決していないメガ種族のストーンも外れる(ADR-0320 §7 の追跡の解消)。
- 並びはマスタの順のまま、実体も保つ。「持ち物なし」は返さない(欄が先頭に足す。従来どおり「なし」「未選択」「(なし)」)。
- `domain/mega.ts` の `selectableItems` は `itemsForRole` に置き換え、削除した(`megaStoneItemIds`・
  `megaItemLock`・`itemIdAfterSpeciesChange` は残す)。

### 2. 画面と欄ごとの役割

| 画面・欄 | 役割 | 理由 |
|---|---|---|
| 計算: 攻撃側の持ち物 | `attacker` | 攻撃側の持ち物だけが攻撃側として計算に効く |
| 計算: 防御側の持ち物・「持ち物の候補も比較」の候補 | `defender` | 同上。比較の候補(`defensiveItemCandidates`)の母集合も `defender` で絞ってから効果で選ぶ |
| 逆算: 自分の持ち物 | 与えたダメージ(`side` = `defender`)→ `attacker`、受けたダメージ(`side` = `attacker`)→ `defender` | `side` は逆算する相手の側。自分はその反対(`reverseMyItemRole`) |
| 逆算: 相手の持ち物候補(`itemCandidates`) | `side` と同じ役割 | `reverseItemCandidates` の母集合を絞ってから効果で選ぶ |
| 判定: 自分・相手の候補 | `either` | 自分と相手が互いに攻撃する(双方向の確定数) |
| 調整: 自分の持ち物 | `either` | 火力指数と耐久指数を両方出し、モードを切り替えても入力を消さない(ADR-0319 §2) |
| 構築の編集 | `any`(ストーンだけ外す) | 構築は実際の対戦で持たせる持ち物の記録。計算に効かない持ち物(回復のきのみ等)も記録できなければならない |

### 3. 役割が変わって選択済みの持ち物が合わなくなったとき

計算の「攻守入れ替え」(持ち物は種族に付いて動く)と、逆算の「観測したダメージ」の切り替えで起きる。
`itemAfterRoleChange({ items, role, currentItemId })` で決める。

- 合う持ち物(両方の役割を持つもの、`roles` が無いもの)は保つ。
- 合わない持ち物は**未選択(持ち物なし)に戻し**、その欄に `role="status"` で `itemRoleText.droppedNotice(名前, 役割)`
  (「{持ち物}は{攻撃側/防御側}では計算に影響しないため、持ち物を外しました」)を出す。黙って外さない。
  通知の要素の id を持ち物欄の `aria-describedby` に足す(固定の理由と同じ作法)。
- 通知は、その欄の持ち物を選び直したとき・種族を変えたとき・もう一度入れ替えたときに消える。入れ替えて戻しても持ち物は戻さない。
- メガストーン(固定)はここで外さない。固定は種族から毎回導く(ADR-0320)。

### 4. メガ種族の固定中の表示

- 欄の表示は `megaStoneLabel(species)`: `baseSpeciesNameJa` があれば `itemRoleText.megaStoneOf(基本種名)`(「ルカリオのメガストーン」)、
  null・省略・空白なら `itemRoleText.megaStoneUnnamed`(「メガストーン」)。
  **ADR-0328 で上書き**: 表示はマスタのストーンの `nameJa` を使い(`megaStoneDisplayName`)、ひらがな・カタカナ・漢字を含まないとき(英語名・全角英数字だけ・空)だけ、
  上のフォールバック(基本種名・「メガストーン」)にする。`megaStoneLabel(species, stoneNameJa)`。
- 同じ名前を、逆算の相手のカードの文(`megaItemText.fixedItemName(...)`)と構築の補正の通知(`megaItemText.correctedNotice(...)`)にも使う。
- 持ち物名を ID から引く場所もすべて同じ名前にする(ADR-0328 により、日本語として使えるストーンの `nameJa` はそのまま、使えないときだけフォールバック。英語名は画面・aria・title・テキストのどこにも出さない): 計算の結果の行(`.calc-results__item`)・
  逆算の候補の行(`.reverse-results__item`)・未対応の印(計算・逆算・判定の確定数の注意・調整)。表示用の一覧 `itemsWithStoneLabels(items, species, stoneIds?)` が
  ストーンの `nameJa` を `megaStoneDisplayName`(使えないときはその ID を `requiredItemId` に持つ種族の基本種名から。引けなければ「メガストーン」)に通し、
  ID 引きはその一覧を通す(要求には使わない)。
- 構築で非メガのメンバーが古いデータでメガストーンを持つとき(ADR-0320 PR-B 4a。値は直さない)、現在値の選択肢の表示は「メガストーン」。
- 固定の理由(`megaItemText.lockedReason`・`missingReason`)と `aria-describedby` は変えない(iOS と同じ語)。
- 固定に使うストーンは、絞り込む前の全件(`master.items`)から `megaItemLock` で引く(従来どおり)。ストーンの役割が欄の役割に合わなくても固定する。
- メガ以外の種族に変えたときの解除(未選択に戻す)は従来どおり(`itemIdAfterSpeciesChange`)。

### 5. 型と境界

- `MasterItem extends Item { roles?: readonly ItemRole[]; isMegaStone?: boolean }` を `master/types.ts` に置き、`MasterData.items`・
  キャッシュの `items` をこの型にする。`MasterSpecies` に省略可の `baseSpeciesKey`・`baseSpeciesNameJa` を足す。
- **engine(WASM 境界)と calc-svc は未知のフィールドを拒否する**(`DisallowUnknownFields`)。要求を組み立てる関数
  (`domain/requests.ts` の `buildIndividual`・`buildBulkRequest`・`buildReverseRequest`)が `toEngineItem` で `id`・`nameJa`・`effect` だけにする。
  項目を持たない持ち物は同じ実体を返す(既存の要求の挙動を変えない)。`toEngineSpecies` は従来どおり項目を拾うので基本種の項目は渡らない。
- 判定・調整は持ち物 ID だけを送るので影響しない。スナップショットの書き出し(`exportSnapshot.ts`)は項目を明示的に拾うので影響しない。

### 6. オンライン・オフライン

- オンライン(`onlineSource.ts`): `Item.roles`・`isMegaStone`、`SpeciesDetail.baseSpeciesKey`・`baseSpeciesNameJa` を応答のまま写す。
  応答に無ければキーを作らない(「分からない」= 絞らない、と読むため)。
- オフライン(IndexedDB。ADR-0313): 取得したマスタをそのまま保存するので、項目も残る。
  **`MASTER_CACHE_SCHEMA_VERSION` を 2 → 3 に上げ**、版 2(`roles` の無い記録)は破棄して空として扱う
  (残すと、オフラインで全持ち物が「役割が分からない」になり、意味の無い持ち物が選択肢に戻る)。
- 例データ(架空。`master/example`)は `roles` を持たないので絞らない(従来の一覧。ストーンは種族から外す)。

### 7. 文言の置き場所

`web/src/i18n/items.ts` の `itemRoleText`(`megaStoneOf`・`megaStoneUnnamed`・`droppedNotice`)。持ち物の文言は5つの画面(複数レーン)が共有するので、
レーン別ファイルではなく持ち物の文言ファイルとし、画面から直接 import する(ADR-0323。`ja.ts` の再エクスポートに足さない。
`ja.ts` を import しない)。iOS は同じ語にそろえる。

## 対象外・追跡

- **素早さに効く持ち物**(`StatMods[spe]`)は ADR-0175 で役割を持たないので、判定・調整の `either` の欄に出ない。
  M-C には該当する持ち物が無い。必要になったらデータレーンに `speed` の役割を足してもらい、判定・調整の役割に加える(ADR-0175 §1 末尾)。
- 調整画面にはメガの固定が無い(ADR-0320 の範囲外。calc-svc の調整 API はメガの持ち物を検査しない)。ストーンは選択肢から外れるので、
  メガ種族の調整は持ち物なしで計算される。固定を入れるなら別 issue。
- 構築の Showdown 書き出しの持ち物名は本 ADR の対象外。

## 検討した代替

- **構築も `either` で絞る**: 回復のきのみ・きあいのタスキ類のような、計算に効かないが実戦で持たせる持ち物を記録できなくなり、
  既存の構築・Showdown 取り込みと食い違う。却下。
- **入れ替えで合わない持ち物を残し、警告だけ出す**: 選択肢に無い値を select が持つと表示が「なし」とずれ、要求には効かない持ち物が載る。却下。
- **入れ替えで役割の合う持ち物を相手と交換・自動で選ぶ**: 利用者が選んでいない持ち物を黙って使うことになる。却下。
- **役割が無いときクライアントが効果から導く**: ADR-0175 §4 で禁止(規則が分かれる)。
- **ストーンの名前を基本種名から推測する・英語名を出す**: ユーザー報告の不具合そのもの。却下。

## 受け入れ条件と担当テスト

1. 絞り込み関数が4つの役割・`roles` 無し・ストーン(`isMegaStone`/`stoneIds`)・順序で決まった結果を返す — `web/src/domain/itemRoles.test.ts`
2. 計算: 攻撃側は attacker・防御側は defender の持ち物だけ。役割なし・ストーンは出ない。固定中は「{基本種名}のメガストーン」/「メガストーン」。
   入れ替えで合わない持ち物は未選択+`role="status"`+`aria-describedby`。比較の候補は defender だけ — `web/src/screens/CalcScreen.itemRoles.test.tsx`
3. 逆算: 自分の欄は観測した側で attacker/defender、切り替えで外す+通知、相手の候補は相手の側の役割、固定中の表示(欄・相手のカード) —
   `web/src/screens/ReverseScreen.itemRoles.test.tsx`
4. 判定・調整は either、構築は any(ストーンだけ外す。補正の通知・非メガの古いストーンの表示) —
   `web/src/judge/JudgeScreen.itemRoles.test.tsx`・`web/src/adjust/AdjustScreen.itemRoles.test.tsx`・`web/src/team/TeamScreen.itemRoles.test.tsx`
5. 要求に `roles`・`isMegaStone` が載らない — `web/src/domain/requests.itemRoles.test.ts`(と各画面テストの要求の確認)
6. オンラインの写像・キャッシュ版 3・版 2 の破棄・オフラインで項目が残る — `web/src/master/onlineSource.itemRoles.test.ts`・
   `web/src/master/cache/cachedSources.itemRoles.test.ts`
7. 文言 — `web/src/i18n/itemText.test.ts`
8. 既存のメガのテスト(`*.mega.test.tsx`)と E2E(`e2e/mega.spec.ts`・`e2e/team.spec.ts`)の固定中の表示の期待値を、ストーンの `nameJa` から
   「{基本種名}のメガストーン」(基本種名の無い fixture は「メガストーン」)に改めた(仕様の変更。弱めていない)。
