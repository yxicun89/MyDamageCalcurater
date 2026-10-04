# ADR-0336: ビジュアル基盤の残り画面への適用 — F-12 PR-2 / I-web-12

- 状態: 採用(2026-10-04。実装済み)
- 関連: ADR-0334(基盤。§3 の共通部品)、docs/usability-round2.md F-12、docs/plan/improvements/web.md I-web-12、
  docs/design.md「ポップ配色」「共通の部品」、ADR-0319・0331(調整)、ADR-0333(お気に入りの復元)、ADR-0314・0318(このアプリについて)

## 背景

F-12 PR-1 で計算・素早さ・構築に共通部品(`.ui-*`)を当てた。残りの逆算・タイプバランス・お気に入り・このアプリについて・
調整は、ブラウザ既定のボタンやラジオ・素の表のまま。調整画面は ec レーンの依頼(ec は今は触らない)で Web レーンがまとめて進める。
判定画面(web/src/judge/)は hidden なので対象外。

## 決定

### §1 原則(PR-1 と同じ)

- DOM・アクセシブルな名前・role・テキスト・API 呼び出し・挙動は変えない。**クラスの追加と包みだけ**。既存のクラス(`reverse-*` など)は残す。
- 新しい色・トークンは足さない(`tokens.css` の変更なし。よって design.md の popPalette 表・iOS 向けトークン決定ファイルの更新も不要)。
  結論の色分けは既存の `--success`/`--danger`、強弱は既存の `--font-size-*`/`--text-secondary` で付ける。
- CSS の予算(gzip ≤ 30KB。design.md「パフォーマンス予算」・check-bundle-size.mjs)を守る。各画面の CSS は、共通部品で置き換えられる
  旧い見た目(枠・角丸・ボタンの塗り)を削って相殺する。超えそうなら実装者が design.md の値を上げず、先に重複を削る。
- 常時動くアニメーションは足さない。動きは `.ui-*` の操作時の transition だけ(reduced-motion では 0 秒。PR-1 のまま)。
- タイプ色のカードは `typeAccentStyle(種族の最初のタイプ)`。種族が未選択なら変数を置かない(ブランド色のまま)。

### §2 画面ごとの「どの要素にどの .ui-* を付けるか」(テストで固定する)

| 画面 | 要素(既存のクラス・role) | 付けるクラス |
|---|---|---|
| 逆算 | 自分/相手のカード(`section.reverse-card`) | `ui-card ui-card--typed` + `typeAccentStyle` |
| 逆算 | 観測の側(`reverse-side__option`)・自分の調整(`reverse-preset__option`)・観測の単位(`reverse-observation__unit-option`) | `ui-chip`(選択中だけ `ui-chip--selected`) |
| 逆算 | 「観測を追加」(`reverse-observations__add`)・観測の削除(`reverse-observation__remove`) | `ui-button ui-button--secondary` |
| 逆算 | `reverse-screen__notice`・結果の案内 | `ui-notice ui-notice--info`(読み込み中は `--loading`) |
| 逆算 | `role=alert`(`reverse-screen__error`) | `ui-notice ui-notice--error` |
| 逆算 | 推定結果の一覧(`ul.reverse-results__list`) | `ui-rows` |
| タイプバランス | メンバー・仮想敵の枠(`fieldset.balance-member`) | `ui-card ui-card--typed` + `typeAccentStyle` |
| タイプバランス | 追加(メンバー・仮想敵)・削除(`balance-member__remove`) | `ui-button ui-button--secondary` |
| タイプバランス | 計算中・案内(`balance-screen__notice`) / `role=alert`(`balance-screen__error`) | `ui-notice --loading`(または `--info`) / `ui-notice --error` |
| タイプバランス | 結果の表(防御相性・集計・攻撃範囲・仮想敵・おすすめ。すべての `table`) | `ui-table`(ゼブラ) |
| タイプバランス | 仮想敵ごとの領域(`balance-screen__threat`)・おすすめの領域(`balance-screen__recommendations`) | `ui-card` |
| お気に入り | 一覧の行(`li.favorites-screen__item`) | `ui-card` |
| お気に入り | 件数(`favorites-screen__count`)・「攻撃側だけ」(`favorites-screen__hint`) | `ui-badge` |
| お気に入り | 「計算に使う」 / 削除 / 削除する / やめる | 既存の `ui-button--primary` / `--secondary` / `--danger` / `--secondary`(変更なし) |
| お気に入り | 削除確認の文(`favorites-screen__confirm > p`) | `ui-notice ui-notice--error` |
| お気に入り | 読み込み中 / 空 / オフライン(role=status) / 失敗(role=alert) | `ui-notice --loading` / `--empty` / `--info` / `--error` |
| お気に入り(計算画面側の復元案内) | `calc-screen__restore` の role=status / `calc-screen__restore-error`(role=alert) | `ui-notice --info` / `ui-notice --error` |
| このアプリについて | 節(`about__section`、データの扱い `about__data` を含む) | `ui-card` |
| このアプリについて | 出典リスト(`ul.about__sources`) | `ui-rows` |
| このアプリについて | 戻るリンク(`a.about__back`。リンクのまま) | `ui-button ui-button--secondary` |
| このアプリについて | `about__button` / `--danger` | `ui-button`(+ `--secondary`)/ `ui-button ui-button--danger` |
| このアプリについて | 削除確認ダイアログ本体(`role=alertdialog`、`about__dialog`) | `ui-card`(スクリムは今のまま) |
| このアプリについて | `about__status`(role=status)/ `--error`(role=alert) | `ui-notice --loading`(進行中)または `--info` / `ui-notice --error` |
| 調整 | 自分/相手の領域(`section.adjust-screen__region`。見出し「自分」「相手」) | `ui-card ui-card--typed` + `typeAccentStyle` |
| 調整 | 目標カード(`fieldset.adjust-screen__goal`) | `ui-card`(group の名前「目標 n」は legend のまま) |
| 調整 | モードの選択(`adjust-screen__choice`。ラジオ) | `ui-chip`(選択中だけ `--selected`) |
| 調整 | 「調整する」(`adjust-screen__submit`) | `ui-button ui-button--primary` |
| 調整 | 「目標を追加」・目標を外す・「この技を覚えるポケモン」の各ボタン | `ui-button ui-button--secondary` |
| 調整 | `adjust-screen__notice` / `role=alert`(`adjust-screen__error`) | `ui-notice --empty`・`--loading`・`--info` / `ui-notice --error` |
| 調整 | 結果の小領域(`adjust-screen__subregion`) | `ui-card` |
| 調整 | 行の一覧(`ul.adjust-screen__lines`) | `ui-rows` |

#### 調整の「種類」は select のまま(依頼からの差分)

依頼は「種類の選択(素早さを上回る/この技を耐える/この技で倒す)を ui-chip に」だが、現状は `<select>`(「目標 n の種類」)で、
ラジオ・チップにするのは構造・挙動・アクセシブル名(combobox → radio)を変える。既存テスト(selectOptions を使う)の期待値も変わる。
したがって既定案は **select のまま、`ui-field` の見た目を当てるだけ**にする(チップ化は選択肢が3つで足りる利点があるが、構造変更なので別タスク)。
チップを付けるのは、現状からラジオであるモードの選択だけ。

### §3 調整の結果の強弱(DOM の順は変えない)

結果の情報量が多いので「結論 → 振り方 → 詳細」の強弱を、DOM 順を変えずに CSS で付ける。

| 役割 | 目印(クラス) | 見た目 |
|---|---|---|
| 結論 | `adjust-screen__verdict`(`data-met="true"`/`"false"`)。目標ごとの結果の各 `li`、目標を満たす/満たさない、素早さを満たす/満たさない、倒せる/耐えられる/倒せない/耐えられない の文 | `--font-size-heading` 以上で太く。満たすは `--success`(+ `--success-soft` の背景)、満たさないは `--danger`(+ `--danger-soft`)。色だけに頼らず、文言(「満たします」「満たしません」)がそのまま残る |
| 振り方 | `adjust-screen__plan`(能力ポイントの行) | `--font-size-body` の太字 |
| 詳細 | `adjust-screen__detail`(ステータス・指数・合計・残り・16n・注記) | `--font-size-caption` か `--text-secondary` で控えめに |

- 結果の DOM 順(見出し → 能力ポイント → 合計 → ステータス → … → 目標ごとの結果)は変えない。既存テストの期待値も変えない。
  もし将来「結論を先頭に」並べ替えるなら、構造変更なので別 ADR で期待値更新の対象を明示する。

### §4 テスト

- 画面ごとの visual テスト(クラスが付く・アクセシブル名/テキストが不変・`--card-type`): `ReverseScreen.visual.test.tsx`、
  `BalanceScreen.visual.test.tsx`、`FavoritesScreen.visual.test.tsx`、`AboutScreen.visual.test.tsx`、`AdjustScreen.visual.test.tsx`
  (調整は結果の強弱のクラスと、`AdjustScreen.css` が結論=大きい・詳細=控えめ・成功色/危険色になっていることも見る)
- axe(ライト/ダーク): `e2e/a11y-visual.spec.ts` に /reverse /balance /favorites /adjust /about と、種族を選んだ逆算(タイプ色)を追加
- 375px の横溢れ: `e2e/layoutOverflow.spec.ts`(計算・素早さ・構築・逆算・バランス・お気に入り・調整・/about)
- CSS の予算: 既存の `bundleBudget.test.ts` と build の `check-bundle-size.mjs`(gzip ≤ 30KB)をそのまま使う

## 結果

- 残り画面が計算・素早さ・構築と同じ見た目(カード・チップ・ボタン・案内・ゼブラ表)にそろう。
- 構造・挙動・API は変わらない(既存テストの期待値更新なし)。
- 調整の「種類」を select のままにする差分を、依頼者(ec レーン)に伝える。

## 実装結果(2026-10-04)

- §2・§3 の表どおりにクラスを足した。DOM の順・アクセシブル名・role・テキスト・既存クラスは変えていない(既存テストの期待値更新なし)。
- 画面別の旧い枠・角丸・塗り・ボタンの CSS を削り、`.ui-*` に任せた。`web/src/styles/inputTokens.test.ts` が逆算の `.reverse-observations__add` に
  角丸・文字のトークンを求めるため、そこだけ `ui-button` と同じトークンを画面側にも残した(見た目は変わらない)。
- 逆算の観測入力に `box-sizing: border-box` を足した(幅 100% の入力が単位のチップに重なっていたため。チップ化で目立った)。
- 側・単位・調整のモードのラジオは `ui-chip` のラベルに入れ、ネイティブの input は視覚的に隠した(ラベルのクリックで選べる)。
- 調整の結果は `adjust-screen__verdict`〈data-met〉・`__plan`・`__detail` の3段(CSS のみ)。`ui-rows` のゼブラに負けないよう結論の地色は詳細度を上げた。
- タイプバランスの結果の表(防御相性は 18 列)は 375px でページを横に溢れさせた(critic が検出。scrollWidth 917 / clientWidth 375)。
  表を `div[role=region][tabindex=0][aria-label]`(`balance-screen__table-scroll`、`overflow-x:auto`)で包み、溢れを包みの中で受けるようにした。
  名前は i18n の `balanceScreenText.tableScrollLabel`(「〜の表(横にスクロールできます)」)。表自体のアクセシブル名・role は変えていない。
  `e2e/layoutOverflow.spec.ts` に、API を `page.route` の fake で返して結果の表を出した状態の 375px の検査を足した(既定の設定で動く)。
- 検証: vitest 全件・typecheck・lint・build(CSS gzip 約 6.6KB / 予算 30KB)・`make web-e2e`(axe ライト/ダーク含む)・
  `web-e2e-online`・`web-e2e-container`・check-publishable・check-plan がすべて成功。
- 目視: 逆算・入力部の各画面を preview でライト/ダーク/375px で確認した。お気に入り一覧・調整の結果(結論=緑、振り方=太字、詳細=控えめ)・
  タイプバランスの表は critic がライト/ダークで目視して良好。調整の結果の『満たしません』の赤は未目視で、クラス(`data-met="false"`)と CSS をテストで固定している。
