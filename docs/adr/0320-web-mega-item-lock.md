# ADR-0320: メガ種族の持ち物をメガストーンに固定する(Web の共通ドメインと計算・逆算画面。issue #515)

- 状態: 提案
- 日付: 2026-10-03
- レーン: Web
- 関連: issue #515、docs/mega-evolution-spec.md §4-3、ADR-0002 決定 10(メガ後の姿は別のポケモンとして登録し、持ち物はメガストーンで固定)、
  ADR-0304(オンラインのマスタ取得口)、ADR-0313(マスタのキャッシュ)、ADR-0311・0312(計算画面の入力)、`docs/coding-rules.md`

## 背景

ADR-0002 決定 10 で、メガシンカ後の姿は別のポケモンとして登録し、その持ち物はメガストーンで固定すると決まった。API の検証
(メガ種族に `requiredItemId` 以外の持ち物を持たせた計算は 400)は別 issue で進行中なので、UI は送れない入力を出さない。
Web は種族を選んだあとの持ち物を独立した選択にしており、`isMega` / `requiredItemId` を使っていなかった。

issue #515 の Web 分を2つの PR に分ける。**PR-A(この ADR の範囲)= 共通ドメイン + 計算画面 + 逆算画面。PR-B = 構築の編集・判定画面**
(古い保存データの読み込み時の補正を含む。PR-B で必要なら追記する)。

## 決定

1. **共通ドメイン `web/src/domain/mega.ts`(純粋関数。PR-B が再利用する)**。名前・ID を直書きせず、マスタの `isMega` / `requiredItemId` だけから導く。
   - `isMegaSpecies(species)`: `isMega === true` のときだけ true(省略・null は false)。
   - `megaStoneItemIds(species[])`: メガ種族の `requiredItemId` に現れる ID の集合(「メガストーンを単独の選択肢に出さない」ための判別)。
   - `selectableItems(items, stoneIds)`: その集合の持ち物を外した選択肢(順序・実体は保つ)。
   - `megaItemLock(species, items)`: `none`(メガでない)/ `locked`(メガ + マスタにストーンあり。`item` を返す)/ `missing`(メガだがストーンを引けない)。
   - `itemIdAfterSpeciesChange({ previous, next, items, currentItemId })`: 種族を変えたときの持ち物 ID。
     メガへ → ストーンの ID(`missing` は空)、メガから非メガ・未選択へ → 空(メガストーンを残さない)、非メガどうし → 現在の持ち物を保つ。
2. **文言は `i18n/ja.ts` の `megaItemText`**(`lockedReason`「メガシンカ: メガストーンを持ちます」・`missingReason`・`compareDisabledReason`・`fixedItemName(name)`)。
   iOS は同じ語にそろえる(spec §4-3)。画面に直書きしない。
3. **画面(計算・逆算の個体を選ぶすべての場所)**:
   - メガ種族を選ぶと持ち物をストーンに設定し、持ち物欄を `disabled` にしてストーンの名前を見せる。理由を見える文言で出し、`aria-describedby` で欄に結び付ける。
   - メガでない種族に変えたら固定を解除して未選択に戻す。メガストーンは単独の選択肢に出さない(「持ち物の候補も比較」の候補・逆算の持ち物候補にも混ぜない)。
   - 「持ち物の候補も比較」は**防御側がメガ**のとき `disabled`(理由つき)にし、`itemVariants` はストーン1件。逆算は**相手がメガ**のとき
     `itemCandidates` をストーン1件にする(`null` も混ぜない。メガ種族の持ち物なしは API が 400 にするため)。相手のカードには持ち物欄が無いので、理由とストーン名を文で出す。
   - 攻守入れ替え・観測した側の切り替えは、持ち物が種族に付いて動くので固定が保たれる。
4. **ストーンをマスタの持ち物から引けないメガ種族(`missing`)**: 固定せず持ち物は空にし、欄は `disabled` で理由(`missingReason`)を出す。逆算の相手は `itemCandidates = [null]`。
   黙って別の持ち物にしない。API の 400 は最後の安全網で、UI は見せない(spec §4-3)。
5. **マスタの型**: `MasterSpecies` に `isMega?: boolean`・`requiredItemId?: string | null` を足す(**省略可**。省略は非メガと同じ。既存の fixture・例データを変えずに済み、
   公開 API が項目を返さない間も壊れない)。engine には渡さない(`toEngineSpecies` が落とす。境界は未知のフィールドを拒否する)。
6. **公開 API の契約**: 公開 API の `SpeciesDetail` には `isMega` / `requiredItemId` が無かった(`MasterSpecies` は内部の共通マスタのスキーマ)。
   `api/openapi.yaml` の `SpeciesDetail` に**省略可の項目として**足した(pokedex-svc が `species.is_mega`・`required_item_id` を返すまで、Go の応答は項目を返さない。
   required にすると Go が false・null を常に出して、メガ種族を非メガと誤って返すため)。**pokedex-svc の応答の実装は別レーン(API・データ)の作業**。
   Web の写像は応答の値をそのまま写し、省略は省略のまま(`isMegaSpecies` が false と読む)。
7. **メガストーンの集合の出どころ**: 種族の全件一覧があるマスタ(`speciesList`)は `master.species` から、検索で解決するマスタ(オンライン・キャッシュ済みオフライン)は
   「いままでに解決した種族」(`useSpeciesResolutions` が持つ覚え書き + `master.species`)から導く。公開 API に持ち物がメガストーンかを示す項目が無いため、
   **検索で解決するマスタでは、まだ解決していないメガ種族のストーンは単独の選択肢に残りうる**(その種族を選べば固定され、以後は出なくなる)。
   完全にするには API 側の項目(例: `Item.isMegaStone`)が要る。別 issue にする。
8. **マスタのキャッシュのスキーマ版を 1 → 2 に上げる**(ADR-0313 §5)。保存済みの種族はメガの項目を持たないので、版 1 の記録を使い続けると、メガ種族が `isMega` なしで再利用され固定されない。
   版違いは破棄して空として扱う(次のオンライン取得で作り直す)。保存形そのもの(キー構造)は変えない。
9. **E2E の pokedex フィクスチャ**に架空のメガ種族とメガストーン(`src/test/megaMaster.ts` の `withMegaFixture`。単体テストと共有)を足し、詳細の応答に `isMega` / `requiredItemId` を常に出す。

## 検討した代替

- **`isMega` を必須にする**: 既存の fixture・テストを全部直す必要があり、公開 API が項目を返さない移行期に型が嘘をつく。省略可にして読む側(`isMegaSpecies`)を1か所にした。
- **メガストーンの判別を持ち物の名前(「ナイト」)・ID で行う**: ハードコード禁止(CLAUDE.md)。マスタの `requiredItemId` から導く。
- **メガでも持ち物欄を操作可能のまま警告だけ出す**: 送れない入力が UI から出てしまう(spec §4-3)。
- **逆算の相手がメガのとき `itemCandidates = [null, ストーン]`**: `null` はメガ種族の持ち物なしで API が 400 にするので入れない。

## 結果・追跡

- 受け入れ条件は `docs/mega-evolution-spec.md` §5 の 3(UI)の Web 分(計算・逆算)。構築・判定と古い保存データ(同 4)は PR-B。
- 追跡: pokedex-svc の応答に `isMega`・`requiredItemId` を載せる(API・データ)。`Item` がメガストーンかを返す項目(§決定 7 の残り)。
