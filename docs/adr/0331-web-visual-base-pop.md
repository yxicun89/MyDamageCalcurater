# ADR-0331: Web のビジュアルの基盤(ポップ・カラフル)— F-12 / I-web-6

- 状態: 採用(2026-10-04)
- 関連: docs/usability-round2.md F-12(原文19)、docs/plan/improvements/web.md I-web-6、docs/design.md、
  ADR-0300 §4(iOS と同じトークン名・値)、ADR-0323(画面の登録)、ADR-0325(画像は任意)

## 背景

使用感フィードバック(第2回、原文19): 「全体の UI・UX が簡素すぎる。ボタンや文字や表に見た目の装飾が無く、機械的」。
ユーザー決定: 見た目は「ポケモンらしいポップ・カラフル(タイプ色のカード・丸いボタン・アイコン・やさしい言葉)。使っていて楽しいこと」。

現状(2026-10-04 に /calc /speed /team などをライト・ダーク・375px で撮影して確認):
- 色を持つのはタイプバッジとエンブレムだけ。ほかは灰色と白黒(選択中のタブは黒塗り)
- 構築の「作成」「内容を確認」、計算の「攻撃側をお気に入りに追加」などはブラウザ既定のボタン
- 素早さの絞り込み・場の状態はブラウザ既定のチェックボックスが並ぶだけ
- 見出しの太さ・サイズに段階がほぼ無い。案内(読み込み中・失敗・空)が本文と同じ見た目
- アイコンは無い

design.md の旧方針「背景は無彩色。色を持つのはタイプだけ」は、この ADR でブランド色・状態色・やさしいグラデーションを足す形に改める。

## 決定

### §1 トークンの拡張(すべて tokens.css の `--*`。値の正は design.md)

design.md に新しい節「### ポップ配色」(表: `| トークン | ライト | ダーク | 用途 |`)を置く。表に並べる順は下のとおり
(`web/src/styles/popPalette.test.ts` がこの順で読む)。CSS 変数名は `.` を `-` にしたもの(`brand.primary` → `--brand-primary`)。

| トークン | ライト | ダーク | 用途 |
|---|---|---|---|
| brand.primary | #1F5FD6 | #7FA8FF | 主ボタン・選択中のタブとチップの塗り・リンク |
| on.primary | #FFFFFF | #0E1015 | brand.primary の塗りの上の文字 |
| brand.accent | #FFCB05 | #FFD84D | 塗りの装飾だけ(星・ハイライト)。文字色・境界線には使わない |
| on.accent | #14161A | #14161A | brand.accent の塗りの上の文字 |
| success | #17743A | #5FD38A | 成功の文字・アイコン |
| success.soft | #E2F5E8 | #12301F | 成功の案内の塗り |
| warning | #9A5B00 | #FFB547 | 注意の文字・アイコン |
| warning.soft | #FFF1D6 | #33240B | 注意の案内の塗り |
| info | #0B6BA8 | #5EC2F2 | 情報・読み込み中の文字・アイコン |
| info.soft | #DCEFFB | #0C2A3A | 情報・読み込み中の案内の塗り |
| danger.soft | #FDE3E4 | #3A1416 | エラーの案内の塗り(文字は既存の danger) |
| on.danger | #FFFFFF | #0E1015 | danger の塗り(危険ボタン)の上の文字 |
| surface.card | #FFFFFF | #1A1D24 | カード・ボタン(副)の不透明な面 |
| bg.gradient-start | #FFF6E0 | #14131C | 背景のやさしいグラデーション(上) |
| bg.gradient-end | #E8F1FF | #0E1622 | 背景のやさしいグラデーション(下) |
| table.header | #DCE7FB | #1C2638 | 表の見出し行 |
| table.zebra | #EEF2F8 | #151922 | 表・一覧の偶数行 |
| table.hover | #E3ECFB | #1D2535 | 表・一覧の行ホバー(hover: hover の環境だけ) |
| focus.ring | #1F5FD6 | #7FA8FF | フォーカスの輪(:focus-visible) |
| shadow.color | 黒 10% | 黒 40% | 影の色 |

命名の制約: `-ink` で終わる名前は使わない(タイプバッジの文字色専用。tokens.test.ts が `-ink` の集合を固定している)。
`--type-` で始まる名前も使わない(タイプ色の集合を固定している)。

コントラスト(WCAG 2.2。spec-writer が計算した値。テストは design.md の値から毎回計算する):
on.primary/brand.primary 5.73 / 8.11、brand.primary/bg.base 5.25 / 8.11、success/success.soft 5.13 / 7.61、
warning/warning.soft 4.86 / 8.55、info/info.soft 4.83 / 7.44、danger/danger.soft 4.55 / 5.61、on.danger/danger 5.53 / 6.56、
text.secondary/table.header 4.91 / 6.44(以上 ライト / ダーク)。brand.accent はライトの bg.base に対して 1.40 しかないため
**塗りの装飾にだけ**使う(必ず on.accent の文字か、意味を持たない装飾と組にする)。組み合わせの一覧は popPalette.test.ts の
`CONTRAST_PAIRS`(通常文字 4.5、フォーカスの輪・UI の境界 3)。

色以外のトークン(既存の節に追記):
- 「文字」: `サイズ:` の行に「タイトル 22」を足す → `--font-size-title: 22px`(h1 のアプリ名)。
  新しい行「- 太さ: タイトル 800 / 見出し 700 / 強調 700 / 本文 400」→ `--font-weight-title|heading|strong|body`
- 「形・余白」: 新しい行「- 影: カード 0 2px 8px / 浮き上がり 0 6px 16px(色は shadow.color)」
  → `--shadow-card: 0 2px 8px var(--shadow-color)`、`--shadow-raised: 0 6px 16px var(--shadow-color)`。
  角丸・余白の段階は**増やさない**(`--space-6` を作らない: tokens.test.ts が余白の列を固定している。16px の角丸を足すと
  inputTokens.test.ts の「値 → 変数名」の引き当てが 16px で `--space-4` と衝突するため)。表・案内の角丸は `--radius-input`
- 「動き」: 新しい行「- 押下: ボタン・タブ・チップを押したとき 0.15秒で少し縮む(`duration.press`)」→ `--duration-press: 0.15s`。
  reduced-motion の `:root` で `--duration-press: 0s`
- body: `background-color: var(--bg-base)` は残し、`background-image: linear-gradient(180deg, var(--bg-gradient-start), var(--bg-gradient-end))`
  と `background-attachment: fixed` を足す

### §2 共通の見た目の部品

`web/src/styles/components.css`(main.tsx で tokens.css の**次**に import)に `.ui-*` クラスを置く。
画面の CSS は配置(グリッド・余白)だけを持ち、見た目は共通クラスを**足す**(既存クラスは消さない)。

| クラス | 見た目 |
|---|---|
| `.ui-card` | `--radius-card`・`--surface-card`・`--shadow-card` |
| `.ui-card--typed` | ふち(上端 4px 程度)と `.ui-card__band` を `var(--card-type, var(--brand-primary))` で塗る |
| `.ui-card__band` | カード上端の帯(装飾。見出しの h2 を包んでよい。文字を載せるなら `--type-<id>-ink`) |
| `.ui-section` / `.ui-heading` | セクションの余白・見出し(`--font-weight-heading`。アイコン + 文字) |
| `.ui-button` | ピル・`--font-weight-strong`・`cursor: pointer`・`transition: transform var(--duration-press) …`・`:active { transform: scale(0.96) }`・`:disabled { cursor: not-allowed }` |
| `.ui-button--primary / --secondary / --danger` | 塗り/文字: brand.primary/on.primary、surface.card/brand.primary、danger/on.danger |
| `.ui-chip` / `.ui-chip--selected` | チェックボックス・ラジオを包む label をピルにする。選択中は brand.primary/on.primary |
| `.ui-badge` | ピル・太字(確定数・タイプ) |
| `.ui-table` | `th` に table.header、`tbody tr:nth-child(even)` に table.zebra、`tbody tr:hover` に table.hover、角丸 |
| `.ui-rows` | ul/ol を表のように: `> li:nth-child(even)` ゼブラ、`> li:hover` ホバー、角丸 |
| `.ui-tabs` / `.ui-tab` | `inline-flex` + `gap`(アイコンと文字)。`[aria-selected="true"]` は brand.primary/on.primary |
| `.ui-field` | 中の input・select・textarea に入力の角丸と `:focus-visible` の輪 |
| `.ui-notice` + `--empty / --error / --loading / --info` | 角丸の囲み。empty: surface.card/text.secondary、error: danger.soft/danger、loading・info: info.soft/info |
| `.ui-icon` | `flex-shrink: 0`。色は currentColor |

規則: `:hover` の見た目は `@media (hover: hover)` の中にだけ置く。`.ui-button / .ui-tab / .ui-chip / .ui-field` は
`:focus-visible` で `outline: 2px solid var(--focus-ring)`(offset 2px)。components.css に `animation` は置かない。
transition の時間は `var(--duration-press)` だけ。

React 部品(必要最小限):
- `web/src/ui/Icon.tsx`: `export const ICON_NAMES`、`export type IconName`、`export function Icon({ name, label?, size = 20, className? })`。
  インライン SVG(`viewBox="0 0 24 24"`、`fill`/`stroke` は `currentColor` か `none` だけ)。`label` なし → `aria-hidden="true" focusable="false"`、
  あり → `role="img" aria-label`。名前: calc / reverse / speed / balance / judge / team / favorites / adjust / about / screen(既定)/
  swap / plus / trash / edit / search / alert / info / check。外部依存・画像ファイル・`<use href>` を使わない
- `web/src/ui/typeAccent.ts`: `CARD_TYPE_VARIABLE = "--card-type"`、`typeAccentStyle(typeId?: string): CSSProperties`。
  `/^[a-z0-9]+(-[a-z0-9]+)*$/` に合う ID だけ `{ "--card-type": "var(--type-<id>, var(--brand-primary))" }`、それ以外は `{}`
- `web/src/app/screenIcons.ts`: `screenIconName(screenId: string): IconName`(8画面の対応表 + 既定 "screen")。
  画面の登録ファイル(`*.screen.tsx`)と defineScreen の型は**変えない**(調整画面〈web/src/adjust/〉をこの PR で触らないため。
  後で defineScreen に任意の `icon` を足すなら別 ADR)

### §3 適用範囲と段階

この PR(F-12 前半):
- 基盤: tokens.css・components.css・Icon・typeAccent・screenIcons・design.md の更新
- シェル(App.tsx / App.css): tablist に `ui-tabs`、tab に `ui-tab` とアイコン(`<Icon name={screenIconName(id)} />`。装飾)、
  計算モードの label に `ui-chip`(選択中 `ui-chip--selected`)、h1 に `--font-size-title`/`--font-weight-title`、
  フッターのリンクに about のアイコン(装飾)。既存の `app-tabs__*`・`app-mode__*`・`app-footer__link` のクラスは残す
- 計算(CalcScreen): 2枚のカードに `ui-card ui-card--typed` と `style={{ ...holo.style, ...typeAccentStyle(primaryType) }}`
  (holo の `--holo-x/y` と共存)、タイプのバッジに `ui-badge`、攻守入れ替えに `ui-button ui-button--secondary` + swap アイコン(装飾)、
  「詳細」に `ui-button ui-button--secondary`、計算結果の ul に `ui-rows`、確定数に `ui-badge`
- 素早さ(SpeedScreen): 2つの section に `ui-card`、絞り込み・場の状態・入力の方法の label に `ui-chip`(選択中 `ui-chip--selected`)、
  読み込み中に `ui-notice ui-notice--loading`、`role="alert"` に `ui-notice ui-notice--error`、段の ul に `ui-rows`
- 構築(TeamScreen): 新規作成の囲みに `ui-card`、構築名の欄を `ui-field` で包む、「作成」に主ボタン、「名前を変更」「削除をやめる」
  などに副ボタン、「削除」「削除を確定」に危険ボタン、一覧に `ui-rows`、読み込み中/空/失敗に `ui-notice--loading/--empty/--error`
  (Showdown 取り込み・メンバー編集は同じクラスを当ててよいが、テストで固定するのは上のものだけ)

次の PR(F-12 後半。共通クラスを当てるだけで済む): 逆算・タイプバランス・判定・お気に入り・このアプリについて。
調整(web/src/adjust/)は ec レーンが改修中なので、F-12 が main に入ったあと ec レーンが同じクラスを当てる。

### §4 動き

常時動くアニメーションは置かない。演出は押下(`:active` の scale)とタブ・チップの選択の色の transition だけ。
時間は `--duration-press`、`prefers-reduced-motion: reduce` で 0(tokens.css の既存の `*` の無効化 + 変数の 0)。

### §5 画像は必須にしない

PokemonImage と type-emblem のフォールバック(ADR-0325)は変えない。カードの色は画像の有無と独立(--card-type)。

### §6 パフォーマンス予算

design.md「パフォーマンス予算」の行を「Web の初期ロード: JS ≤ 300KB・CSS ≤ 30KB(gzip、WASM 除く)」にし、
scripts/check-bundle-size.mjs に `CSS_BUDGET_GZIP_BYTES = 30 * 1024` と .css の合計の検査を足す
(2026-10-04 時点: JS 130,820 B・CSS 約 5 KB)。アイコンは SVG のパスを JS に持つので数 KB 以内に収める。

### §7 既存テストを弱めない・期待値更新の対象

既存のクラス・アクセシブルな名前・テキスト・DOM の役割を変えず、クラスの**追加**とアイコンの**挿入**で済ませる設計にした。
spec-writer の確認では、設計どおりなら期待値の更新が要る既存テストは**無い**見込み。壊しやすい箇所(実装者が確認する):
- tokens.test.ts: `-ink` の集合・`--type-` の集合・余白の列(`--space-6` 禁止)・ベースの表の6行 → 新トークンは別の節・別の名前にする
- inputTokens.test.ts: 計算・逆算・タイプバランスの select・ボタンの角丸を画面 CSS の規則で見ている → 画面 CSS の該当規則は残す
- motion.test.ts: animation は `.is-*` だけ → components.css に animation を置かない。`.calc-screen__swap:active { rotate(180deg) }` と
  `.ui-button:active { scale }` は同じ詳細度で衝突する → 入れ替えボタンは rotate を保つ(`.calc-screen__swap.ui-button:active` で両方を合成するなど)
- responsive.test.ts / e2e/mobile.spec.ts: タブにアイコンが入って幅が増える → タブ列だけ横スクロール(既存)のまま、ページは溢れさせない
- a11y-about.spec.ts(axe): フッターのリンクのコントラスト → 色は text.secondary のまま
- CalcScreen.test.tsx・SpeedScreen.test.tsx: エンブレムの style に `var(--type-…)` を含むこと → エンブレムの style は変えない
もし更新が必要になったら、理由をコミットメッセージに書き、この節に追記する(テストを消さない・弱めない)。

### §8 確認の手順(人間とレビュー用。画像比較のテストは入れない)

```
cd "$(git rev-parse --show-toplevel)/web"
npm run build
node e2e/support/pokedexFixtureServer.mjs 18329 &
API_PROXY_TARGET="" POKEDEX_PROXY_TARGET=http://127.0.0.1:18329 npx vite preview --host 127.0.0.1 --port 4339 --strictPort &
```
Playwright(chromium)で `/calc /speed /team /reverse /balance /favorites /about` を、1280×900(ライト・ダーク)と 375×812(ライト)で
`page.screenshot({ fullPage: true })` する(`colorScheme` は `browser.newContext({ colorScheme })`)。計算は「テストほのお」「テストみず」を
選んだ状態も撮る。確認点: タブが色付きの丸いピルで選択中が主色、カードのふち・帯がタイプ色、ボタンが丸く主/副/危険の色、
一覧にゼブラ、案内が色付きの囲み、375px でページが横に溢れない。

## 結果(実装 2026-10-04)

- §1〜§3 のとおり実装。ライト・ダーク・375px で /calc(種族選択後)・/speed・/team を撮影して確認(横の溢れ 0、フォーカス・コントラストは axe で 0 件)。
- §7 の確認結果: 既存テストの期待値更新は不要だった。App.css のタブ・計算モードの見た目(黒塗り)は共通部品へ移し、配置の指定だけ残した。
  攻守入れ替えは `.calc-screen__swap.ui-button:active` で rotate と scale を合成。
- 追加の適用: 計算のプリセット・性格補正のピルを `ui-chip`、「詳細」・お気に入り追加を副ボタン、構築の Showdown 取り込み・書き出しのボタンとカード。
- `.ui-rows > li` の余白は `:where()` で詳細度 0 にし、画面側の余白が勝つようにした。
- 新規テスト 3 ファイルの `getByRole(..., { exact: true })` は型エラー(ByRole に exact は無い。name は元から完全一致)なので、`exact` だけ削除した。
- 予算: JS 約 132KB・CSS 約 6KB(gzip)。

## 代替案

- 外部のアイコンライブラリ・UI キット: 依存と予算が増え、ハードコード禁止・色トークンの規約と合わない。不採用
- 画面ごとの CSS に見た目を直接書く: 9画面で同じ見た目を繰り返し、iOS とトークンの対応も追えない。不採用
- defineScreen に `icon` を必須で足す: 調整画面(ec レーン)を触ることになる。今回は app/screenIcons.ts の対応表 + 既定アイコン

## 付録: iOS 向けトークン一覧の下書き(実装後に docs/ai-shared/decisions/2026-10-0X-web-visual-base-pop-tokens.md として置く)

> Web(F-12、ADR-0331)で次のトークンを足した。iOS は `PokeCalcDesign.swift` に同じ名前(camelCase)・同じ値で足す(ADR-0300 §4)。
>
> | Web(CSS 変数) | iOS(案) | ライト | ダーク |
> |---|---|---|---|
> | --brand-primary | brandPrimary | #1F5FD6 | #7FA8FF |
> | --on-primary | onPrimary | #FFFFFF | #0E1015 |
> | --brand-accent | brandAccent | #FFCB05 | #FFD84D |
> | --on-accent | onAccent | #14161A | #14161A |
> | --success | success | #17743A | #5FD38A |
> | --success-soft | successSoft | #E2F5E8 | #12301F |
> | --warning | warning | #9A5B00 | #FFB547 |
> | --warning-soft | warningSoft | #FFF1D6 | #33240B |
> | --info | info | #0B6BA8 | #5EC2F2 |
> | --info-soft | infoSoft | #DCEFFB | #0C2A3A |
> | --danger-soft | dangerSoft | #FDE3E4 | #3A1416 |
> | --on-danger | onDanger | #FFFFFF | #0E1015 |
> | --surface-card | surfaceCard | #FFFFFF | #1A1D24 |
> | --bg-gradient-start | bgGradientStart | #FFF6E0 | #14131C |
> | --bg-gradient-end | bgGradientEnd | #E8F1FF | #0E1622 |
> | --table-header | tableHeader | #DCE7FB | #1C2638 |
> | --table-zebra | tableZebra | #EEF2F8 | #151922 |
> | --table-hover | tableHover | #E3ECFB | #1D2535(iOS はホバーが無いので押下中の行に使ってよい) |
> | --focus-ring | focusRing | #1F5FD6 | #7FA8FF |
> | --shadow-color | shadowColor | 黒 10% | 黒 40% |
>
> 色以外: `--font-size-title` 22 / `--font-weight-title` 800・`heading` 700・`strong` 700・`body` 400 /
> `--shadow-card` 0 2 8・`--shadow-raised` 0 6 16(色は shadowColor)/ `--duration-press` 0.15 秒(視差効果を減らす で 0)。
> 部品の対応: カード(タイプ色のふち・帯)、ピルのボタン(主/副/危険)、チップ、ゼブラの一覧。アイコンは SF Symbols で同じ意味のものを選ぶ。
