# ADR-0521: iOS のビジュアルの基盤(ポップ・カラフル)— F-12 / I-ios-6

- 状態: 採用(2026-10-10)
- 日付: 2026-10-10
- 関連: docs/usability-round2.md F-12、ADR-0334・ADR-0336(Web の基盤)、docs/ai-shared/decisions/2026-10-04-web-visual-base-pop-tokens.md、
  ADR-0300 §4・ADR-0500 §1(iOS と Web で同じトークン名・値)、ADR-0501 末尾「F-12 iOS ビジュアルの基盤」

## 背景

Web は「ポップ・カラフル」(タイプ色のカード・丸いボタン・アイコン・やさしい背景)に改めた。iOS は無彩色トークン(bgBase・bgGlass)と
Liquid Glass のカードのままで、同じアプリに見えなかった。Web の決定ファイルが指定したトークンを iOS に同じ名前・同じ値で足し、全画面に当てる。

## 決定

1. **トークン**(`PokeCalcDesign.swift`): `ColorToken` に決定ファイルの 20 色(brandPrimary … shadowColor)、`TextStyleToken.title`(22)と
   `FontWeightToken`(title 800・heading 700・strong 700・body 400)、`ShadowToken`(card 0 2 8・raised 0 6 16。SwiftUI の radius は blur の半分)、
   `MotionToken`(押下 0.15 秒・縮み 0.96・視差効果を減らすで 0 秒)。`ColorToken.popPalette` に名前つきで一覧を持つ(検査用)。
2. **部品**(`ios/PokeCalc/PopComponents.swift`。App ターゲット。View に依存するため Kit の外):
   `popScreenBackground`・`popCard`・`popInset`・`popRow`/`PopRowButtonStyle`・`PillButtonStyle(kind:)`・`PopChipStyle`・`PopNoticeView`/`popNotice`・
   `PopIcon`/`PopHeading`/`PopLabel`・`PopSymbol`。対応表は design.md「共通の部品」の iOS 節。
3. **`glassCard` を廃止して `popCard` に置き換える**。Liquid Glass(`glassEffect`)は背景のグラデーションと混ざって文字のコントラストを保証できず、
   Web の surface.card と見た目が揃わないため、不透明の surfaceCard にした。`bgGlass` トークン自体は残す(design.md の表・既存テスト。コントラスト検査対象)が、
   画面では使わない。入力欄・ピル・円の面は tableZebra に置き換えた(カードの白の上でも見分けられる)。
4. **既存の規則は変えない**: アクセシビリティ識別子・ラベル文字・`.contain`/`.ignore` の構造・`isAccessibilitySize` の分岐・タップ領域 36pt 以上(ボタンは最小高さを持つ)。
   装飾アイコンは `accessibilityHidden`、状態は文字・アイコンを併記(案内は種類ごとのアイコン、自分の段は「自分」のバッジ)。
   常時動くアニメーションは無い(押下の縮みは操作時のみ)。
5. **未選択のチップは tableZebra**(Web は surfaceCard + ヘアライン)。iOS ではカードの上(surfaceCard)にチップが載る画面が多く、白同士で面が消えるため。
6. **TextStyleToken.allCases が 4 → 5**(title の追加)。`DesignTokenTests` の件数検査は design.md の文字の段階が 5 になったことに合わせて更新した(弱めていない。理由は ADR-0501 追記の表)。

## 結果・申し送り

- 画面ごとのタイトルの太さ(ナビゲーションバーの principal)は heading のまま(太さだけ 700 になった)。アプリ名だけ title(22/800)。
- 逆算・構築・調整・判定・タイプバランスの結果行の細部(タイプバッジの隣の装飾など)は、トークンと部品を当てた範囲まで。画面の作り直し
  (F-08 構築・F-13 文言)はこの基盤の上で行う。
