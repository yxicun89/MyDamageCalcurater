# ADR-0333: お気に入りから計算を開く(計算の入力の保存と復元)— F-09 / I-web-8

- 状態: 採用(2026-10-04。実装済み)
- 関連: docs/usability-round2.md F-09(原文14)、docs/plan/improvements/web.md I-web-8、
  ADR-0228(`FavoriteInput.calc` / `Favorite.calc`)、ADR-0227(お気に入り API・冪等)、ADR-0327(お気に入りの Web 画面)、
  ADR-0308(訪問済みタブの保持)、ADR-0311(特性の選択)、ADR-0312・ADR-0315(詳細の条件・防御側ランク)、
  ADR-0320(メガの持ち物固定)、ADR-0323(画面レジストリ)、ADR-0326(持ち物の役割)、ADR-0329(攻撃・特攻の入力)、
  ADR-0331(F-12 の部品)、docs/ai-shared/decisions/2026-10-04-api-favorite-calc-team-name.md(API レーンの決定)

## 背景

利用者の要望(原文14): 「登録しただけで何もできないなら無駄。クリックするとそのときのダメージ計算がすぐ出せる、などにする」。
ADR-0327 §3 は「お気に入り → 計算画面」を見送った。API は ADR-0228 で `calc`(`CalcRequest` そのもの)を持てるようになった。
Web の計算は一括計算(防御側 = 種族 + プリセット5行。`calcBulk`)で、`CalcRequest`(1対1)とは形が違う。
「Web の状態を `CalcRequest` にどう落とし、どう戻すか」と「タブをまたいで復元をどう渡すか」を決める。

## 決定

### 1. 保存する内容(状態 → `CalcRequest`。`favorites/favoriteCalc.ts` の `favoriteCalcOf`)

純粋関数 `favoriteCalcOf({ state, natures, moveCategory })` が、計算画面の状態(`FavoriteCalcState`)から `CalcRequest` を作る。
攻撃側・防御側・技のどれかが未選択、攻撃側の入力が不正(SP 範囲外)、性格を決められない(ADR-0329 §4)ときは `null`(`calc` を付けない)。
既定のままの条件はキーごと省く(`conditionRequestParts` と同じ考え方。保存内容を小さく保ち、重複判定を安定させる)。

| 画面の状態 | 保存先(`CalcRequest`) | 保存する | 備考 |
|---|---|---|---|
| 攻撃側の種族 | `attacker.speciesKey` | する | |
| 攻撃側の特性(実際に使う特性) | `attacker.abilityId` | する | 「種族の先頭」でも実際の ID を保存(マスタの並びが変わっても同じ特性に戻す)。特性なしは省く |
| 攻撃側の持ち物 | `attacker.itemId` | する | メガは固定のメガストーン。なしは省く |
| 攻撃・特攻の SP(ADR-0329) | `attacker.sp.atk` / `.spa` | する | H/B/D/S は 0 |
| 攻撃・特攻の性格補正 | `attacker.natureId` | する | `resolveAttackerNature` → `resolveNatureId`(計算と同じ性格) |
| 攻撃側ランク(A/C) | `attacker.ranks` | する | 両方 0 なら省く。B/D/S は 0 |
| やけど | `attacker.status: "burn"` | する | なしは省く |
| 防御側の種族 | `defender.speciesKey` | する | 個体は「無振り」: `sp` 全 0・`natureId` は無補正(`resolveNatureId(NEUTRAL_NATURE)`) |
| 防御側の特性 | `defender.abilityId` | する | 「おまかせ」は省く(省略 = おまかせ) |
| 防御側の持ち物 | `defender.itemId` | する | なしは省く |
| 防御側ランク(B/D。ADR-0315) | `defender.ranks` | する | 両方 0 なら省く |
| 技 | `moveId` | する | |
| 天候・フィールド | `field.weather` / `field.terrain` | する | none は省く |
| 防御側の壁 | `field.defenderScreens` | する | どれか1つでも true のときだけ(3値とも) |
| 急所 | `options.critical` | する | true のときだけ |
| 形式 | `format` | する | Web は常に `single` |
| 「持ち物の候補も比較」 | — | しない | `CalcRequest` に場所が無い。復元後はオフ(従来の既定) |
| 防御側のプリセット(5行) | — | しない | 一括計算が毎回出す。防御側の個体は無振りで保存 |
| 結果(ダメージ・確定数) | — | しない | 復元時に計算し直す(マスタ・engine の更新を反映) |

`individual` は `calc` があれば `calc.attacker` と同じにする(一覧の見出し・旧クライアント用。API レーンの決定どおり)。
`calc` が無ければ従来どおり `{speciesKey, level, natureId, sp, itemId?}`。

**往復**: `restoreFavoriteCalc(favoriteCalcOf(S)) = S`(サーバーが既定値を補った `Favorite.calc` からでも同じ)。
例外は「性格が A/C の補正だけでは一意に決まらない入力」(例: A 上昇・C 補正なし → 代表性格 +A/−C)。このときは画面の C の補正が
「下降」で戻ることがあるが、計算に使う性格は同じで、`favoriteCalcOf(restore(calc)) = calc`(もう一度保存すると同じ)を保証する。

**サイズ**: `MAX_FAVORITE_SAVED_BYTES = 4096`(ADR-0228 §3)。最大の状態でも既定値を補って 2KB 以下。万一 4096 バイトを
超える(異常に長い ID)なら `calc` を付けずに従来の本文で保存する(追加を 400 で失敗させない)。

### 2. 復元の操作

- お気に入りの各行に主ボタン「「{見出し}」を計算に使う」(`favoritesScreenText.useLabel(title)`。見える文字 = 名前。SC 2.5.3)。
  `FavoritesScreen` の `onUse?: (favorite) => void` を渡したときだけ出す(従来の呼び出しは変わらない)。
- 押すと計算タブへ移り(URL `/calc` を pushState)、入力を戻し、計算画面の通常の `calcBulk` がそのまま走って結果が出る(追加の操作なし)。
  計算画面に `role=status`「「{見出し}」の計算を開きました」(`favoritesRestoreText.restoredNotice`)。
- `calc` の無い旧お気に入り: `individual` から攻撃側(種族・持ち物・特性・A/C の SP と補正)だけを戻す。防御側・条件はそのまま、
  技は `resolveMoveId`(覚えていれば保つ・覚えていなければ最初のダメージ技)。一覧の行に「攻撃側だけ」の案内
  (`favoritesScreenText.attackerOnlyHint`)、計算画面に `role=status`(`favoritesRestoreText.attackerOnlyNotice(title)`)。
- 計算画面の単発 `POST /api/calc`(`calcDamage`)は呼ばない(API レーンの決定の「そのまま calcDamage」との差。Web は一括計算の画面なので、
  復元は入力を戻して通常の一括計算を走らせる。iOS は単発で計算する。付録)。

### 3. 画面間の受け渡し(App → 計算画面)

- App が「復元の要求」`FavoriteRestoreRequest = { token: number; favorite: Favorite }` を state に持つ。
  `ScreenEnvironment` に `favoriteRestore?: FavoriteRestoreRequest` と `onUseFavorite?: (favorite) => void` を足す。
  `onUseFavorite` は token を +1 した要求を置き、`navigateToTab("calc")` する。
- `favorites.screen.tsx` は `onUse={env.onUseFavorite}`、`calc.screen.tsx` は `restoreRequest={env.favoriteRestore}` を渡すだけ
  (他レーンの画面のファイルは触らない)。
- `CalcScreen` は「最後に適用した token」を持ち、**token が変わったときだけ**適用する(mount 時に要求があれば適用。
  計算タブを初めて開くときも、ADR-0308 で mount 済みのときも同じ)。同じ token の再描画では戻さない(利用者の変更を消さない)。
- 復元は入力の state をまとめて置き換える(攻撃側・防御側の種族・持ち物・特性・技・攻撃側の入力・詳細の条件。持ち物の「外した」通知は消す、
  比較はオフ)。古い結果は `CompletedCalc` の比較でそのまま loading になり、出さない(ADR-0300 §8)。攻守入れ替え・種族変更の寿命ルール
  (ADR-0312・ADR-0329: 条件・攻撃側の入力は種族変更で消さない)はそのまま。復元だけが条件・攻撃側の入力を置き換える。
- オンライン(`capabilities.speciesList` が false)は、保存された key を `masterSearch.resolveSpecies` で引き、`register` してから適用する
  (`movesFor` / `abilitiesFor` の覚え書きに入る前でも、解決結果の moves・abilities を lookup に直接渡す。`handleAttackerResolved` と同じ理由)。
  解決待ちの間に新しい要求が来たら古い解決は捨てる(AbortController)。

### 4. 失敗の扱い

- 純粋関数 `restoreFavoriteCalc(calc, lookup)` → `{ state, issues }`。引けたものは戻し、引けないものは空(既定)にして `issues` に入れる:
  `species`(攻撃側・防御側)/ `move`(マスタに無い・攻撃側が覚えない)/ `item` / `ability` / `nature` / `megaItem` / `ignored`。
  攻撃側の種族が引けなくても持ち物・SP・性格・条件は戻す(種族に依存しない。あとで攻撃側を選び直すと引き継がれる)。
  技が引けなければ未選択のまま(別の技で黙って計算しない)。
- Web で表せない項目(`format: double`・H/B/D/S の SP・テラス・やけど以外の状態異常・A/C 以外のランク・攻撃側の壁・防御側の育成〈SP・補正のある性格・
  テラス・状態異常・A/C/S ランク〉)は反映せず `{kind: "ignored", field}` で知らせる(iOS で作ったお気に入りを想定)。
- 画面は `role=alert` に `favoritesRestoreText.unresolvedHeading` と、項目ごとの日本語名 + ID を出す(`favoritesRestoreText.issueText(issue)`)。
  `megaItem` は `favoritesRestoreText.megaItemNotice`(メガストーンを優先した旨)、`ignored` は案内として出す。
- お気に入りは消さない。計算は他の失敗(record・解決の失敗)に影響されない(絶対ルール5)。持ち物だけが引けないなら計算はそのまま走る。
- メガ(ADR-0320): 保存された `itemId` が `requiredItemId` と違う(なしを含む)ときは固定を採り `megaItem` を報告。

### 5. 追加(AddFavoriteButton)

- 本文は `favoriteInputOf({ ..., calc })`。攻撃側・防御側・技が揃い `favoriteCalcOf` が値を返すときだけ `calc` を付ける
  (攻撃側だけのときは従来の本文 = 既存テストの期待どおり)。
- `label` は `favoriteLabelOf`: 揃っていれば「{攻撃側}→{防御側}({技})」、揃っていなければ攻撃側の名前。30 コードポイントで切る(従来)。
- 重複(同じ内容)は API の 200 `created:false`(ADR-0227)の扱いのまま。`calc` の中身が違えば別のお気に入り(ADR-0228)。
- ボタンの文言「攻撃側をお気に入りに追加」は今回は変えない(e2e・iOS と揃っている。言い換えは F-13 で用語集とまとめて)。

### 6. 文言・a11y

`web/src/i18n/favorites.ts` にだけ足す: `favoritesScreenText.useLabel(title)`「「{title}」を計算に使う」・`attackerOnlyHint`、
`favoritesRestoreText`(`restoredNotice(title)`・`attackerOnlyNotice(title)`・`unresolvedHeading`・`issueText(issue)`・`megaItemNotice`・
`ignoredNotice` など)。iOS の Text と同じ語にする(付録)。ボタンは ADR-0331 の `ui-button`(主ボタン)。

補足(実装後の追記):
- 持ち物は画面の持ち物欄と同じ判定(`itemsForRole`: 役割に合うものだけ・メガストーンは単独で選べない。ADR-0326)で戻せるか調べる。
  合わなければ持ち物なしにして `{kind: "item"}` を報告する(計算で黙って外れる経路を作らない)。
- 復元の案内(role=status)は、利用者が入力を変えたら消す(古い「戻せませんでした」を出したままにしない)。
- 「計算に使う」で計算タブへ移ったときは、非表示になったお気に入りタブのボタンにフォーカスを残さず、復元の案内(`tabIndex=-1`)へ移す(§6 の a11y)。

## 受け入れ条件(テストで固定)

1. `favoriteCalcOf` は表 §1 の形の `CalcRequest` を作り、既定の条件のキーを作らない。未選択・不正・性格未解決は `null`。(favoriteCalc.test.ts AC-1)
2. 状態 → calc → 状態が一致する(クライアント形・サーバーの既定値補完形の両方)。一意でない性格は calc の再保存が一致。(AC-2)
3. 戻せない種族・技・持ち物・特性・性格は空に戻して `issues` に報告し、引けた部分は戻す。表せない項目は `ignored`。(AC-3)
4. メガ種族の持ち物は固定を優先し、食い違いを `megaItem` で報告。(AC-4、CalcScreen R-5)
5. お気に入りの各行に「「{見出し}」を計算に使う」。押すと `onUse(favorite)`。旧お気に入りには「攻撃側だけ」の案内。(FavoritesScreen.restore.test.tsx)
6. 計算画面は token の変化で入力を戻し、追加の操作なしで `calcBulk` が走って結果が出る。古い結果を出さない。戻せない項目は `role=alert`。
   オンラインは `resolveSpecies` で引いて戻す。(CalcScreen.favoriteRestore.test.tsx R-1〜R-7)
7. App: お気に入りタブで押すと計算タブ(`/calc`)に切り替わり、入力が戻って一括計算の要求が送られる(初回 mount・mount 済み・2回目・旧お気に入り)。
   (App.favoriteRestore.test.tsx)
8. 追加の本文に `calc` が付き、`label` は「攻撃側→防御側(技)」、`individual = calc.attacker`。戻した直後の追加は元の `calc` と同じ。
   保存内容は 4096 バイト以内。(favoriteCalc.test.ts AC-6、CalcScreen R-8)
9. e2e: 追加 → 攻撃側を変える → お気に入りタブで「計算に使う」→ 計算タブで入力が戻り結果が出る。(e2e/favorites.spec.ts)

## 既存テストへの影響

- 期待値の変更は不要の見込み: 既存の追加のテスト(CalcScreen.favorites.test.tsx・App.favorites.test.tsx・e2e の1本目)は攻撃側だけを選んで追加するので
  `calc` が付かず、本文 `{label, individual}`・label = 種族名のまま。`FavoritesScreen.test.tsx` は `onUse` を渡さないのでボタンが増えない。
- e2e/favorites.spec.ts の fake は `calc` を受け取って返すよう広げた(既存の検査は弱めていない)。
- 実装で既存テストの期待値を変える必要が出たら、弱めずに理由をコミットメッセージに書く(絶対ルール6)。

## 実装の結果(2026-10-04)

- 実装: `favorites/favoriteCalc.ts`(`favoriteCalcOf`・`restoreFavoriteCalc`・`restoreFavoriteAttacker`・`favoriteLabelOf`・
  `MAX_FAVORITE_SAVED_BYTES`)、`favoriteInput.ts`(calc 対応・4096 バイト超は calc を付けない)、`FavoritesScreen`(`onUse`・行の主ボタン・
  旧お気に入りの案内)、`CalcScreen`(`restoreRequest`。token が変わったときだけ非同期に入力を戻す。オンラインは `resolveSpecies` を AbortController つきで引く)、
  App(`favoriteRestore`・`onUseFavorite`。マスタ読み込み失敗中の描画分岐にも `onUseFavorite` を渡す)。
- 受け入れ条件 1〜9 はテストで固定(vitest 3672 件・`make web-e2e` 89 件が通る)。
- 追加の判断(実装で決めたこと):
  - 技を戻せなかったとき、技の選択欄は DOM 上は先頭の技を表示してしまう(`<select>` の仕様)。別の技で計算したように見えないよう、
    `value === ""` で候補があるときだけ未選択の選択肢(`favoritesRestoreText.moveUnselectedOption`。disabled)を出す。
  - `megaItem` は復元の案内(role=status)の中の1文、`ignored` は同じ案内の中に日本語名の一覧で出す。それ以外(species・move・item・ability・nature)は role=alert。
  - 一覧の行は見出し → 操作(「計算に使う」・削除)の縦並びにし、削除の3ボタンにも F-12 の `ui-button` を当てた(長い見出しでも375px で崩れない)。
- テストの誤りを最小限で直した(弱めていない): `FavoritesScreen.restore.test.tsx` の `rowOf` は見出しの部分一致で行を探していたため、
  「テストみず」が「テストほのお→テストみず(…)」の行に当たっていた。見出しの文字列が一致する行を探すよう直した。
  `e2e/favorites.spec.ts` の新規テストは、この設定に calc-svc が無く結果の行が出ないため、`/api/calc/bulk` の応答だけ差し替え、
  戻した入力で一括計算が要求されたこと(攻撃側・防御側の key)も確かめるようにした。

## 結果・トレードオフ

- 往復の「計算に使う性格は同じ」は、技の分類が同じときだけ成り立つ。例: 物理技で「A 上昇・C 補正なし」を保存すると代表性格 +A/−C になり、
  戻すと C は「下降」で表示される。その状態で特殊技へ変えると、C の下降が計算に効いて結果が変わりうる(保存時と別の技にした場合)。
- 4096 バイトの判定は、クライアントが送る本文(サーバーが既定値を補う前)の JSON で行う。サーバーの補完分(約 300 バイト)だけ境界がずれうる。
  最大の状態でも補完後 2KB 以下なので、通常の入力では上限に届かない。
- 利点: お気に入りが「押せば同じ計算がすぐ出る」ものになる。契約は ADR-0228 のまま(API 変更なし)。純粋関数で往復を固定し、Web/iOS の差を明文化。
- 欠点: 「候補も比較」・防御側のプリセット以外の育成は Web では保存・復元しない(一括計算の画面のため)。iOS で作った詳細な防御側は Web では無振りとして
  開き、`ignored` で知らせる。性格が一意でない入力は C/A の補正表示が変わりうる(計算は同じ)。

## 付録: iOS 向けの決定ファイルの下書き(docs/ai-shared/decisions/2026-10-04-web-favorite-calc-restore.md として実装 PR で追加)

```
## 2026-10-04: お気に入りの calc の Web の保存形と復元規則(Web レーンから iOS〈ios-6f〉へ)
Decision: Web はピン留め時の計算画面の入力を `FavoriteInput.calc`(CalcRequest)に保存し、一覧の「「{見出し}」を計算に使う」で
計算タブに戻して一括計算を走らせる(ADR-0333)。
- Web が保存する calc: format=single / attacker = 種族・特性(実際に使う ID)・持ち物・SP は atk/spa のみ・natureId・ranks(atk/spa のみ、0 なら省略)・status(burn のみ)/
  defender = 種族の無振り個体(sp 全 0・無補正の natureId)+ 特性(おまかせは省略)・持ち物・ranks(def/spd のみ)/ moveId /
  field = weather・terrain・defenderScreens(既定は省略)/ options.critical(true のときだけ)。attackerScreens・teraType は作らない。
- individual は calc.attacker と同じ。label は「{攻撃側}→{防御側}({技})」(30 コードポイントで切る)。
- 復元: Web は一括計算(防御側はプリセット5行)なので、calc.defender の SP・性格・テラス等は使わず、Web で表せない項目は「反映していません」と案内する。
  iOS は calc 全体を戻して calcDamage(単発)を呼ぶ想定のままでよい(Web が保存した calc は無振りの防御側として計算される)。
- 引けない種族・技・持ち物・特性は、引けた部分だけ戻して項目を明示する(お気に入りは消さない)。メガの持ち物は固定を優先して知らせる。
- 文言: 行のボタン「「{見出し}」を計算に使う」、計算画面の案内「「{見出し}」の計算を開きました」、旧お気に入り(calc なし)の案内「攻撃側だけ」。iOS も同じ語にする。
Reason: F-09(原文14)。Web と iOS で同じお気に入りを開けるようにし、差(一括計算 / 単発計算)を明記する。
Impact: iOS は calc を作るとき Web と同じ省略規則に揃えると、同じ計算の重複判定(ADR-0227)が端末をまたいで効く(必須ではない)。
```
