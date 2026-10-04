# Web のビジュアル基盤(ポップ・カラフル)のトークン — iOS への連絡(ADR-0334 / F-12)

Web で次のトークンを足した。iOS は `PokeCalcDesign.swift` に同じ名前(camelCase)・同じ値で足す(ADR-0300 §4)。

| Web(CSS 変数) | iOS(案) | ライト | ダーク |
|---|---|---|---|
| --brand-primary | brandPrimary | #1F5FD6 | #7FA8FF |
| --on-primary | onPrimary | #FFFFFF | #0E1015 |
| --brand-accent | brandAccent | #FFCB05 | #FFD84D |
| --on-accent | onAccent | #14161A | #14161A |
| --success | success | #17743A | #5FD38A |
| --success-soft | successSoft | #E2F5E8 | #12301F |
| --warning | warning | #9A5B00 | #FFB547 |
| --warning-soft | warningSoft | #FFF1D6 | #33240B |
| --info | info | #0B6BA8 | #5EC2F2 |
| --info-soft | infoSoft | #DCEFFB | #0C2A3A |
| --danger-soft | dangerSoft | #FDE3E4 | #3A1416 |
| --on-danger | onDanger | #FFFFFF | #0E1015 |
| --surface-card | surfaceCard | #FFFFFF | #1A1D24 |
| --bg-gradient-start | bgGradientStart | #FFF6E0 | #14131C |
| --bg-gradient-end | bgGradientEnd | #E8F1FF | #0E1622 |
| --table-header | tableHeader | #DCE7FB | #1C2638 |
| --table-zebra | tableZebra | #EEF2F8 | #151922 |
| --table-hover | tableHover | #E3ECFB | #1D2535(iOS はホバーが無いので押下中の行に使ってよい) |
| --focus-ring | focusRing | #1F5FD6 | #7FA8FF |
| --shadow-color | shadowColor | 黒 10% | 黒 40% |

色以外: `--font-size-title` 22 / `--font-weight-title` 800・`heading` 700・`strong` 700・`body` 400 /
`--shadow-card` 0 2 8・`--shadow-raised` 0 6 16(色は shadowColor)/ `--duration-press` 0.15 秒(視差効果を減らす で 0)。

部品の方針: カード(`--card-type` のタイプ色で上端のふち・帯を染める。未選択はブランド色)、ピルのボタン(主・副・危険。押下で 0.96 に縮む)、
チップ(選択中は主色の塗り)、ゼブラの一覧(偶数行 tableZebra・押下中 tableHover)、案内(空・エラー・読み込み・情報の淡い塗り)。
アイコンの方針: 線画 24x24・文字色に従う。装飾は支援技術から隠す。iOS は SF Symbols で同じ意味のもの
(計算・逆算・素早さ・タイプバランス・判定・構築・お気に入り・調整・このアプリについて・入れ替え・追加・削除・編集・検索・注意・情報・完了)を選ぶ。
値の正は docs/design.md「ポップ配色」「共通の部品」。
