# ADR-0328: Web の技の選択肢をダメージ技に絞り、メガストーンの表示名をマスタの nameJa に切り替える

- 状態: 採用
- 日付: 2026-10-04
- 関連: ADR-0326(持ち物の役割とメガストーンの表示)、ADR-0324 §2(メガストーンの日本語名は生成しない)、
  ADR-0175 §4(固定中の表示)、ADR-0304(オンライン・オフラインの capabilities)、docs/plan/improvements/web.md I-web-2・I-web-4

## 背景

1. 計算・逆算の技の選択肢に変化技が混ざる。変化技を選ぶと「変化技はダメージを計算しません」(status-move)になるだけで、
   選ぶ意味が無い。調整画面は既に変化技を除いている(`adjust/AdjustScreen.tsx` の `moves.filter(isDamagingMove)`)。
2. ADR-0326 §4 は、マスタのメガストーン名が英語だったため「{基本種名}のメガストーン」を Web で組み立てた。
   データレーンがマスタの `nameJa` を正式名称(リザードナイトＸ・ルカリオナイト等)に直した。日本語名の無い約40件は英語名のまま。

## 決定

### 1. 技の選択肢(I-web-2)

- 絞り込みは `web/src/domain/moves.ts` の純粋関数 `damagingLearnsetMoves(species, moves)`(= `learnsetMoves` から `isDamagingMove` で
  絞る。順は learnset のまま)1か所。判定は `Move.category !== "status"` のみ(技名・ID を直書きしない)。
- 使う画面: 計算(攻撃側の技)・逆算(攻撃側になるほうの技)。調整は既存の絞り込みを同じ関数に寄せる。
  判定画面(`web/src/judge/`)は判定レーンの持ち物なので本 ADR の対象外(別途連絡)。
- 既定の技(`firstDamagingMove`)は従来どおり最初のダメージ技。種族を変えたとき、選択中の技が絞り込み後の一覧に無ければ最初のダメージ技に選び直す。
- 変化技しか覚えない種族(選択肢 0 件): 技欄は空、案内文(`calcScreenText.noDamagingMovesNotice`「このポケモンはダメージを与える技を覚えないため、計算できません」。種族が解決済みで `capabilities.moves` があるとき出す)を出し、
  計算・逆算の要求は送らない(idle 扱い)。`movesAvailable`(`capabilities.moves` または候補 1 件以上)の判定は絞り込み後の件数で行う。
- オンライン(検索で解決した learnset → 技)・オフライン(`MasterData.moves`)は同じ関数を通す。
- `status-move`(変化技を選んだときの案内)は、UI からは到達不能になる。**安全網として状態・文言を残す**
  (起動時に外部から技 ID を渡す経路・将来の入力経路で変化技が入っても、計算要求を送らず案内する。削除は別 ADR)。
  既存テスト「変化技を選ぶと計算せず…」(`CalcScreen.test.tsx`)は、UI から変化技を選べなくなるため選べない前提に直す必要がある
  (弱めない。変化技が選択肢に無いことを `*.damagingMoves.test.tsx` が固定し、案内の表示を起こす分岐は UI から到達できないため、判定 `isStatusMove`(`domain/moves.ts`。画面の分岐とガードが使う)を `domain/moves.statusMove.test.ts` で固定し、画面テストは「変化技が選択肢に無い」ことと「計算・逆算が続く」ことを確かめる。分岐の画面表示そのものの自動テストは無い(初期の技 ID を渡す経路が無いため。経路を足すときに復活させる))。

### 2. メガストーンの表示名(I-web-4)

- 純粋関数 `megaStoneDisplayName(stoneNameJa, baseSpeciesNameJa)`(`web/src/domain/itemRoles.ts`)に集約する。
  - `stoneNameJa` を trim して、ひらがな・カタカナ・漢字(`\p{Script=Hiragana}|\p{Script=Katakana}|\p{Script=Han}`)を1文字以上含めば、そのまま返す。
    全角英数字だけ(「ＸＹ」)・英語名・空・null は日本語として使えない。ただし「リザードナイトＸ」はカタカナを含むので日本語。
  - 使えないとき: `baseSpeciesNameJa` が有れば「{基本種名}のメガストーン」、無ければ「メガストーン」(ADR-0326 §4 のフォールバック)。
- 表示名を出す全箇所(`megaStoneLabel` 呼び出し元: 計算・逆算・判定・構築編集の持ち物欄/固定表示、`itemsWithStoneLabels`
  による結果の行・調整・逆算・未対応の印)を同じ関数経由にする。`megaStoneLabel(species)` はストーンの `nameJa` を受けられる形へ改める。
  判定画面の呼び出しは機械的な引数追加のみで、判定レーンの挙動は変えない(要連絡)。
- 文言(「のメガストーン」「メガストーン」)は `i18n/items.ts` の `itemRoleText` に残す(フォールバック用)。

### 3. 文書の更新(実装後に implementer が行う)

- ADR-0326 §4: 「ストーンの nameJa は使わない」を「日本語として使えるときは nameJa、使えないときだけ『{基本種名}のメガストーン』」に書き換え、
  本 ADR への参照を足す(新方針は本 ADR が上書き)。
- ADR-0324 §2: 「Web は固定中の表示を文言にするので英語名は出ない」の記述を、マスタ側が正式名称を持つようになった事実と
  フォールバックの条件に合わせて更新する。
- ADR-0175 §4(および §72・§82 付近の表示の記述): 「ストーンの nameJa ではなく文言資源」を新方針に書き換える。
- `web/src/test/megaMaster.ts` の `MEGA_*_STONE_LABEL` は、架空ストーン名が日本語なので `nameJa` と同じ値に更新する(既存の
  `*.mega.test.tsx` の期待が変わるのは本 ADR による仕様変更。理由をコミットメッセージに書く)。
- iOS: 同じ表示規則にそろえる(データレーンが連絡)。

## 受け入れ条件と担当テスト

- `damagingLearnsetMoves`: `domain/moves.damaging.test.ts`
- `megaStoneDisplayName`・`itemsWithStoneLabels`: `domain/megaStoneName.test.ts`
- 計算・逆算の技の選択肢(オフライン・オンライン・0 件): `screens/CalcScreen.damagingMoves.test.tsx`・`screens/ReverseScreen.damagingMoves.test.tsx`
- 計算・逆算のメガストーン固定表示: `screens/CalcScreen.stoneName.test.tsx`・`screens/ReverseScreen.stoneName.test.tsx`
- 既存の `e2e/mega.spec.ts` が壊れないこと(e2e の追加は不要)

## 実装メモ

- `megaStoneLabel(species, stoneNameJa?)` は `megaStoneDisplayName(stoneNameJa, species.baseSpeciesNameJa)` に委譲する。呼び出し元(計算・逆算・判定・構築の
  持ち物欄・構築の補正通知)は固定したストーンの `nameJa`(`itemLock.item.nameJa`・`correction.item.nameJa`)を渡す。判定画面の変更は引数の追加だけ。
- 調整画面は `chooseFromList` の `learnsetMoves` を `damagingLearnsetMoves` に寄せた(検索経路の `moves.filter(isDamagingMove)` はそのまま)。
- 仕様変更で期待値を直した既存テスト: `test/megaMaster.ts` の `MEGA_*_STONE_LABEL`(ストーンの `nameJa` と同値)、`*.itemRoles.test`・
  `JudgeScreen.*.test` の固定表示、`CalcScreen.test`・`ReverseScreen.test` の変化技の選択(UI から選べないので「選択肢に無い」ことの確認へ)。
  英語名・基本種名なしのフォールバックは `CalcScreen.itemRoles.test` と `megaStoneName.test` で確かめる。
