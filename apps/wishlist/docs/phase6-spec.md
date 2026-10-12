# wishlist フェーズ6(使用感の改善ループ 2 周目)の受け入れ条件

2 周目の改善ループ([ux-loop.md](ux-loop.md)「2 周目」)で決めたことを実装する。見た目の土台は [phase5-spec.md](phase5-spec.md)(青みの白・強調色 `#1d5fd1`・ジャンル色・M PLUS Rounded 1c)。
API 契約は変えない(`api/openapi.yaml` の変更なし・`make gen` 不要)。テストは Web が `make wishlist-test` 内の vitest、iOS が `make wishlist-ios-test`(WishlistKit)と `make wishlist-ios-xcode-test`(XCUITest)。

## 決めたこと(ユーザー決定 2026-10-12)

- ジャンルのボタンは、**選ばれているものだけ**「絵柄 + 名前」の丸薬(ピル)に広がる。選ばれていないものは今まで通り絵柄だけ
- 適用先(すべて同じ規則): Web ホームの絞り込み(`.genre-nav`。「すべて」を含む)、Web 追加の確認(`Register.tsx` の `.genre-pick`)、Web 編集(`Edit.tsx` の `.genre-pick`)、iOS ホームの絞り込み(`HomeView.genreButtons`)、iOS 追加(`RegisterSheet` の `GenreChooser`)、iOS 共有拡張(`ShareView` の `GenreChooser`)、iOS 編集(`EditItemSheet` の `GenreChooser`)
- 設定のジャンル一覧(名前が文字で出ている)・絵柄を選ぶ `.icon-pick`(絵柄の選択でジャンル名ではない)は対象外

### デザインの決め(実装者向け)

- 名前の文字色: ボタンの絵柄と同じ色。ホームの絞り込みは地が強調色なので白、追加・編集は地がジャンル色なので白(`--on-accent` / `#ffffff`)。文字色にジャンル色そのものは使わない(選ばれていない側には文字が無く、選ばれた側の地がジャンル色のため)
- 文字: 太さ 700、サイズ 14px 前後(iOS は `.subheadline.weight(.bold)` 程度。Dynamic Type に追従)。1 行(`white-space: nowrap` / `lineLimit(1)`)
- 最大幅: 名前の部分に 140px(Web は `max-width: 140px`、iOS は `GenreLabel.maxNameWidth`)。超えたら末尾を `…` で省略(`text-overflow: ellipsis` / `truncationMode(.tail)`)。375px 幅でも、選ばれたボタン + 隣のボタンが横スクロールの行に収まり、行全体が縦に折り返さない
- 横スクロール: ホームの絞り込みは今まで通り横スクロール。選ばれたボタンが画面の外にあるときは見える位置までスクロールしなくてよい(今と同じ)
- 追加・編集の `.genre-pick` は折り返す配置(`flex-wrap`)のまま。選ばれたボタンは幅が広がるだけで、高さ(60 / 48)は変えない。今ある選択中の拡大(`scale(1.08)`)は外した(Web の `.genre-pick`・iOS の `GenreIconButton`。絵柄を選ぶ `.icon-pick` は対象外なので残す)
- 文字色の切り替え(実装で確定。2026-10-12): 追加・編集(地がジャンル色)では、名前と絵柄の線を、地の相対輝度(WCAG)で白 `#ffffff` と濃い色 `#0e1a2b` のうち対比の高いほうにする。黄 `#f2b705`・橙・黄緑・白に近い色は濃い色になる。純粋な関数: Web `web/src/lib/contrast.ts` の `onColor`(CSS 変数 `--on-genre`)、iOS `GenreContrast.prefersDarkText`。ホームの絞り込みは地が強調色なので常に白
- 動き: 選択の切り替えに幅・色の短い遷移(150ms 以下)を付けてよい。`prefers-reduced-motion: reduce`(iOS は `accessibilityReduceMotion`)のときは遷移なし。常時動くものは無し

## 6-1 Web

対象: `web/src/ui/Home.tsx`・`Register.tsx`・`Edit.tsx`・`styles.css`。ボタンの見える文字は **ボタン内の `textContent`** で判定する(絵柄の svg は文字を持たない)。

- **AC-6-1-1** ホームの絞り込み: 最初は「すべて」のボタンにだけ「すべて」という文字が見える。各ジャンルのボタンの `textContent` は空。どのボタンも `aria-label` は(「すべて」またはジャンル名)のまま
- **AC-6-1-2** ジャンルを選び直すと、見える名前が新しく選ばれたボタンに移り、前のボタンからは消える(「すべて」に戻すと「すべて」が出る)
- **AC-6-1-3** 名前が出てもボタンの数は増えない(絵柄と名前は 1 つのボタンの中)。`aria-pressed="true"` は選ばれた 1 つだけ
- **AC-6-1-4** 追加の確認: 推定されたジャンルのボタンにだけ名前が見える。別のジャンルを押すと移る。ジャンルが決まらない(`genre_id` null)ときは、全部絵柄だけ
- **AC-6-1-5** 編集: いまの商品のジャンルのボタンにだけ名前が見える。別のジャンルを押すと移る(保存するまで PATCH しない既存の挙動は変えない)
- **AC-6-1-6** 見た目(目視。verify.md §6): 名前は 1 行で、「デュエル・マスターズ」「METAL BUILD・ROBOT魂」が 375px 幅でも行を壊さず、収まらなければ `…` になる。`prefers-reduced-motion: reduce` で遷移が無い(DevTools の Rendering で切り替え)

### 6-1 テスト一覧

| 番号 | テスト |
|---|---|
| AC-6-1-1〜3 | `web/src/App.genrelabel.test.tsx` 「ホームのジャンル: 選択中だけ名前を出す」 |
| AC-6-1-4,5 | `web/src/App.genrelabel.test.tsx` 「追加・編集のジャンル: 選択中だけ名前を出す」 |
| AC-6-1-6 | 目視(verify.md §6 の手順 1・3・11)。CSS の省略・遷移は jsdom では見られない |
| (維持) | 既存の `App.home.test.tsx` AC-HOME-03・`App.register.test.tsx` AC-5-2e-3・`App.edit.test.tsx` AC-EDIT-01 は `aria-label` と `aria-pressed` で引いているので変更不要 |

**実装者への注意(6-1)**

- 名前は `<span class="genre-name">{name}</span>` のようにボタンの子として、選ばれたときだけ描画する(CSS で隠さない。`textContent` で判定するため)。`aria-label` は消さない
- 3 画面で同じ描画を使うなら共通部品に切り出してよい(`ui/` 内)。ホームの「すべて」は名前 `すべて`・絵柄 `all`
- 既存テストの `getByRole("button", { name })` は `aria-label` で引くため、名前の文字が加わっても壊れないこと

## 6-2 iOS

判断は WishlistCore の純粋な型に置き、View は結果を描くだけにする。

```swift
public enum GenreLabel {
    public static let allName = "すべて"          // ホームの「すべて」
    public static let maxNameWidth = 140.0        // 名前の最大幅(pt)。View はこの値を使う
    /// 選ばれているときだけ前後の空白を除いた名前。選ばれていない・空白だけなら nil。文字列は切らない(省略は View の lineLimit + truncationMode)
    public static func visibleName(_ name: String, selected: Bool) -> String?
}
```

- **AC-6-2-1** `GenreLabel.visibleName`: 選ばれていれば名前(前後の空白を除く)、選ばれていなければ nil、空白だけなら選ばれていても nil。長い名前を切らない
- **AC-6-2-2** 「すべて」も同じ規則(`GenreLabel.allName`)。`maxNameWidth` は 140
- **AC-6-2-3** ホームの絞り込み(XCUITest): 名前の文字に識別子 `genreName-all` / `genreName-<id>` を付け、選ばれているボタンの分だけ存在する。選び直すと移る。ボタン(`genreChip-<id>`)の `accessibilityLabel` はジャンル名のまま(名前が見えていても、見えていなくても)
- **AC-6-2-4** 追加(アプリ・共有拡張)・編集の `GenreChooser`(`GenreIconButton`): 選ばれているボタンだけ `GenreLabel.visibleName` の文字を絵柄の隣に出す。識別子は `genreChoiceName-<id>`、ボタン(`genreChoice-<id>`)の `accessibilityLabel` はジャンル名のまま
- **AC-6-2-5** 見た目(目視。verify.md §8): 1 行・`maxNameWidth` で末尾省略、選ばれたボタンの高さは変わらない。「視差効果を減らす」を入れると遷移せずに切り替わる(`@Environment(\.accessibilityReduceMotion)` で `withAnimation` / `.animation` を外す)。文字サイズを大きくしても行が崩れない(Dynamic Type)

### 6-2 テスト一覧

`make wishlist-ios-test`(WishlistKit の swift test)と `make wishlist-ios-xcode-test`。

| 番号 | テスト |
|---|---|
| AC-6-2-1,2 | `ios/WishlistKit/Tests/WishlistCoreTests/GenreLabelTests.swift`(現在は `GenreLabel` が無いのでコンパイルできない = 失敗) |
| AC-6-2-3 | `ios/WishlistUITests/WishlistUITests.swift` `testOnlySelectedGenreChipShowsItsName` |
| AC-6-2-4 | 目視(verify.md §8)。`GenreChooser` は追加・編集・共有拡張の 3 か所で共有する部品なので、ホームと同じ `GenreLabel` を通っていれば足りる。XCUITest を足すなら `genreChoiceName-<id>` で引く |
| AC-6-2-5 | 目視(verify.md §8) |

**実装者への注意(6-2)**

- `GenreLabel` は WishlistCore に新しいファイルで置く(`GenreLabel.swift`)。View に文字列の判断を書かない。ホームの `genreButton`(HomeView.swift)と共有部品 `GenreIconButton`(Shared/GlassStyle.swift)の両方で使う
- `Shared/GlassStyle.swift` は共有拡張とアプリの両方のターゲットに入っている。WishlistCore を import できることを確認する
- 識別子(`genreName-…`)は名前の `Text` に `accessibilityIdentifier` で付ける。ボタンに `.accessibilityLabel(name)` を付けると中の `Text` が隠れて XCUITest から見えなくなることがある。そのときは `.accessibilityElement(children: .contain)` にするか、識別子を持つ `Text` を `.accessibilityHidden(false)` にする(ボタン自身の `genreChip-…` の識別子とラベルは変えない)
- Xcode プロジェクトに新しい `.swift` を足す場合は `project.pbxproj` も更新する(WishlistCore は SwiftPM なので不要)

## 6-3 選んだボタンが列の端で見切れない

6-1 で選んだボタンが「絵柄 + 名前」に伸びるようになり、横に流れる列(ホームの絞り込み。iOS は追加・編集の `GenreChooser` も)の右端に近いジャンル(ポケモングッズ、METAL BUILD・ROBOT魂 など)を押すと、伸びた名前が列の右端で切れて見えない不具合が出た(375px 幅の Playwright 確認で発見)。列が自動で動かないのが原因。追加・編集の Web は `.genre-pick` が折り返すので対象外。

- **AC-6-3-1** Web: 初回表示(マウント時)では `scrollIntoView` を呼ばない(読み込みでページが跳ねない)
- **AC-6-3-2** Web: 選択が変わったとき、新しく選ばれた `GenreButton` の要素が `element.scrollIntoView({ block: "nearest", inline: "nearest", behavior })` を呼ぶ。`behavior` は `prefers-reduced-motion: reduce` のとき `"auto"`、それ以外は `"smooth"`。`block: "nearest"` なので、列が見えているときにページは縦に動かない
- **AC-6-3-3** Web: 上は `GenreButton` の共通部品で行うので、ホームの絞り込みでテストされていれば追加・編集にも効く(折り返す列では何も動かない)
- **AC-6-3-4** iOS: ホームのチップ列(`HomeView.genreButtons`)と `GenreChooser` の横並びを `ScrollViewReader` で包み、選択が変わったら `proxy.scrollTo(<選ばれた id>, anchor: nil)` を呼ぶ。`accessibilityReduceMotion` が false のときだけ `withAnimation` の中で呼ぶ。各ボタンに `.id(...)` を付ける(「すべて」と各ジャンルで衝突しない値)
- **AC-6-3-5** 見た目(目視。verify.md §6・§8): 375px 幅 / iPhone SE 幅で最後のジャンルを選ぶと、丸薬が全部(収まらなければ `…` つき)列の中に見える。「視差効果を減らす」では動きが滑らかにならず切り替わる
- **AC-6-3-6** iOS: 丸薬の名前は Dynamic Type を `xxxLarge` までで止める(丸薬の高さが 52 で固定のため。止めないと最大の文字サイズで「デュ…」しか見えなかった。読み上げの名前は全文のまま)
- **AC-6-3-7** iOS: 名前が出ているときの余白を Web の丸薬(左 14・絵柄・8・名前・右 16)にそろえる(絵柄の 52pt の枠のせいで、絵柄の左右に余計な空きが出ていた)

### 6-3 の確認(2026-10-12。手元の k3d の実データ・ジャンル 5 つ)

- Web(Playwright・375px): 直す前は「ポケモングッズ」「METAL BUILD・ROBOT魂」を押すと丸薬の右端が 401px で切れていた。直した後は 359px で列の中。「METAL BUILD・ROBOT魂」は `…` で省略。`prefers-reduced-motion: reduce` で遷移 0s。ページの横スクロールは出ない。編集画面で明るい色(ポケモングッズ `#f2b705`)の上は濃い文字
- iOS(iPhone 17e・幅 390pt。一時的な XCUITest で押して撮影し、コミットしない): 5 つとも丸薬が画面内(右端 ≤ 390)。文字サイズ標準と最大(AX5)の両方で 1 行・`…` 省略・高さ 52 のまま

### 6-3 テスト一覧

| 番号 | テスト |
|---|---|
| AC-6-3-1 | `web/src/App.genrescroll.test.tsx` 「AC-6-3-1 初回表示では scrollIntoView を呼ばない」 |
| AC-6-3-2 | 同 「AC-6-3-2 ジャンルを押すと…smooth で scrollIntoView される」 |
| AC-6-3-2 (reduce) | 同 「AC-6-3-2 prefers-reduced-motion: reduce のときは behavior が auto」 |
| AC-6-3-3 | 目視(追加・編集の折り返す列) |
| AC-6-3-4〜7 | 目視(上の「6-3 の確認」)。XCUITest のフィクスチャ(`WishlistFixtures`)のジャンルは 2 つ(`genreChip-1`・`-2`)で、電話幅の列があふれないため、枠外の判定を書いても何も確かめられない。ジャンルが 8 つ前後ある実データで verify.md §8 の手順を行う |

**実装者への注意(6-3)**

- Web: `GenreButton` の `selected` が false から true に変わった副作用(`useEffect`、`useRef` で前回の値を覚える)で呼ぶ。マウント時に最初から `selected` のボタンでは呼ばない。`window.matchMedia` が無い環境(jsdom の既定)でも落ちないようにする(`window.matchMedia?.(...)`)。`scrollIntoView` が無い環境も同様に守る(`el.scrollIntoView?.(...)`)
- 追加・編集で 6-1 の `Register`・`Edit` が開くときにも最初から選ばれたボタンがあるが、マウント時は呼ばない(AC-6-3-1)
- iOS: `HomeView` と `Shared/GlassStyle.swift` の `GenreChooser` はそれぞれ ScrollView の中身を `ScrollViewReader { proxy in … }` で包む。`onChange(of: selection)` で `proxy.scrollTo` を呼ぶ。`GenreIconButton`/`GenrePill` の識別子・ラベルは変えない
- 既存の 6-1 のテストは変えない

## リスク・未決

- 選ばれたボタンの地がジャンル色のとき、白い名前の読みやすさがジャンル色しだい(黄色 `#f2b705` では白との対比が足りない)。ユーザーが色を自由に選べるため、実装者は名前の文字色を地の明るさで白 / 濃い色に切り替えるかを判断してよい(切り替えるなら ADR は不要、この文書に追記する)。絵柄の線も同じ色にそろえること
