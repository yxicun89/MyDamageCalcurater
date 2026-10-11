# ADR-0531: ナビゲーション案 B の採用(関連画面をまとめた下部タブ 4 つ。iOS・Web 共通)

- 状態: 採用(2026-10-11。ユーザー決定「案 B」)
- 関連: `docs/ai-shared/decisions/2026-10-11-navigation-options.md`(案 A/B/C の比較と根拠)、ADR-0507(画面レジストリ)、
  ADR-0308(Web: 訪れたタブは mount したまま hidden)、ADR-0421(タイプバランスを構築編集へ統合)、ADR-0501「ナビゲーション案 B」章(iOS のテストの変更)
- 担当: **iOS は本 ADR と同時に実装(iOS レーン)。Web の実装は Web レーンが担当**(本 ADR は区分・語・保持の方針を決めるだけで、Web のコードは変えていない)

## 背景

iOS は起動するとホームに全機能のピルが並び、機能を移るたびにホームへ戻る作り(ハブ型)で、戻ると画面の入力が消えていた。
ユーザー要望「機能を移るたびにホームへ戻るのが面倒。上部のバーや下部のタブで移りやすくしてほしい」に、決定記録で案 A/B/C を比べ、案 B に決まった。

## 決定

### 1. 区分(iOS・Web 共通)

下部タブ 4 つ。語は iOS・Web で同じにする。

| タブ | 中身 | 識別子(iOS) |
|---|---|---|
| ダメージ | 上部の区切りで 計算 / 逆算 / 調整 を切り替える | `tab-damage` |
| 構築 | 構築一覧 → 編集(タイプバランスは ADR-0421 で編集へ統合予定) | `tab-team` |
| 素早さ | 素早さの表 | `tab-speed` |
| 記録 | お気に入り・計算履歴 | `tab-records` |

- 「このアプリについて」は各タブの右上のアイコン(設定エラー時も出す)。判定は非表示のまま(F-07)。出すときはダメージタブの区切りに並ぶ。
- 画面間の移動は下部タブ 4 つだけ。右下の丸ボタン(F)は画面間の移動用ではない(ユーザー訂正 2026-10-11)ので、「最近使った順」は持たない。
- タイプバランスは構築編集への統合(ADR-0421。tb レーン)までの間、構築一覧の行(`openBalanceScreen`)から開く(機能を失わない)。統合の PR で行ごと外す。

### 2. 入力の保持

タブを移っても、区切りを切り替えても、画面の入力と画面の中で積んだ状態(構築の編集など)は残す(Web の ADR-0308 と同じ「訪れた画面は保持」)。

- iOS: 選択(タブ・区切り)と訪れた画面の集合を純粋な値の型 `AppNavigationModel`(PokeCalcCore)に持ち、`RootView` が 1 つだけ `@State` で持つ。
  画面は一度表示したら破棄しない。タブの中は訪れた画面を `ZStack` に重ね、選択中の 1 つ以外は隠す。
  **画面ごとに `NavigationStack` を分け**、path を `RootView` の辞書(画面 ID → `NavigationPath`)に持つ(積んだ状態がタブ・区切りをまたいで残る。
  キーボードのツールバーなど画面ごとのツールバーが混ざらない)。ViewModel の持ち主は各画面の `@State` のまま(画面が破棄されないので残る)。
- 隠し方: 各画面のスタックを `UIHostingController` に載せ(`RetainedScreenHost`)、選んでいないときは UIKit の `isHidden` にする
  (見えない・押せない・VoiceOver と UI テストにも出ない)。最初は SwiftUI の `opacity(0)` + `accessibilityHidden(true)` にしたが、
  `NavigationStack` の中身が支援技術の木に残り、隠れた区切りのボタンが 2 つ見つかった(UI テストで確認)ため変えた。
  XCUITest は `isHidden` の画面の要素も木に残す(ナビゲーションバーの項目は消える)ので、区切りは選択中の画面にだけ描き(状態を持たないので
  描き直してよい)、識別子を 1 つに保つ。隠れた画面が操作を遮らないことは「戻った画面のボタンを押せる」ことで UI テストする。
- 画面が破棄されなくなったので、ほかのタブで変わるデータは「2 回目以降の表示」で読み直す(`onReappear`): 計算・逆算・判定の構築の選択肢(`loadTeams()`)、
  記録の お気に入り・計算履歴・よく計算する相手。構築一覧は従来どおり表示のたびに読む。

### 3. iOS の画面の登録(ADR-0507 を保つ)

`FeatureEntry` を「タブ内の画面(`.tab(AppTab, title:, accessibilityIdentifier:)`)/ タブの根の画面の中の行(`.link(in:)`)/ 右上のアイコン(`.toolbarIcon`)」にした
(`.rootButton` は廃止)。タブ・区切りの並びは `FeatureSpec.tab` と `order` から組むので、画面を足しても `RootView` は編集しない。
区切りのボタンの識別子は旧ホームのピルの ID(`openCalcScreen` など)を引き継ぐ。起動時に開く環境変数(`POKECALC_OPEN_*_AT_LAUNCH`)は
「そのタブ・区切りを選んで起動」に意味を変えて残す(`IOS_SCREEN` も同じ)。モック(`POKECALC_MOCK_*`)・`POKECALC_SHOW_JUDGE` は不変。

### 4. iOS の部品

- タブ: SwiftUI の `TabView(selection:)` + `Tab`。ラベルは短い日本語、アイコンは SF Symbols(`PopSymbol.forTab`)。切り替えはシステムの標準の遷移だけ(常時動く演出なし)。
- 区切り: 自前の `TabSegmentsView`(常に見えるボタンの並び。メニュー・シートにしない)。高さ 36pt 以上、`isSelected` の特性、入れ物の識別子 `tabSegments-<タブ>`。
  横 1 列に収まるときは横、AX5 などで収まらないときだけ縦に積む(`ViewThatFits`。文字を省略・縮小しない。`lineLimit` 不使用)。
  `Picker(.segmented)` にしなかったのは、文字を縮めて 1 行に詰めるため AX5 で読めなくなること、識別子を区切りごとに付けにくいこと。
- `Tab` に付けた `accessibilityIdentifier` は XCUITest からタブバーのボタンとして引ける(決定記録の懸念。`AppNavigationUITests` で検証)。
  初回の実行で 2 件だけ識別子で見つからないことがあった(その後の実行では再現せず)ため、UI テストの移り方の補助(`selectTab`)は
  識別子で見つからないときだけラベルで引き直し、要素の一覧を添付する。識別子そのものの検査は `AppNavigationUITests` に残す。

### 5. Web(Web レーン)

決定記録の「Web の見通し」のとおり: タブの定義を 4 区分 + 区分内の区切りにし、URL(`/calc` `/reverse` `/adjust` …)は残す。
狭い幅(目安 600px 未満)は 4 つを下部固定、広い幅は上部タブ。`docs/design.md` の「下の固定ナビにはしない」の行は Web レーンが改める。

## 影響

- API・engine・サービスの変更なし。
- iOS の UI テストは、画面への移り方を `PokeCalcUITests/UITestNavigation.swift` の `openScreen(_:)` 1 か所に集約した(変更表は ADR-0501「ナビゲーション案 B」章)。
- 起動直後はホームではなくダメージタブの計算。ホームの状態バッジ(`backendModeBadge`)は設定エラー時の文言だけに残る。
