# ADR-0345: 計算画面の作り直し(G-05 / I-web-13b)

- 状態: 採用(2026-10-11)
- 関連: ADR-0339(方針)・ADR-0344(共通の部品)・ADR-0329(攻撃・特攻のブロック)・ADR-0343(画像)・ADR-0341(技ピッカー)

## 決めたこと

docs/design.md「画面ごとの方向」の計算の行を実装した。方針に無かった細部を決める。

1. **ポケモン・持ち物の選択は select のまま**。`Picker`(シート)は未実装で、既存テスト・操作の多くが combobox 前提のため、顔(`PokemonCard`)を足すだけにした。`PokemonCard` に `nameAsHeading`(名前を h3 にする)、`SegmentedControl` に `describedBy` を足した。
2. **SP は増減ボタン + 文字の数値欄**。`Stepper`(type=number で範囲に丸める)は使わない。不正な値(33・abc)を「誤り(aria-invalid・alert)として出し計算しない」仕様(ADR-0329 §5)を保つため。`.ui-stepper` のクラスと「<欄>を増やす/減らす」の名前は共通にした。
3. **条件はチップ**(`ToggleChip`。ネイティブの checkbox / radio を包み、入力はチップいっぱいに透明に広げる)。天候・フィールドは 5 択なので区切りボタンにせずチップ。性格補正(3 択)だけ区切りボタン。
4. **結果の行の見た目の順は DOM の順と同じ**(responsive.test の WCAG 1.3.2 の検査)。DOM を「ダメージ幅 → 確定数 → バー → 調整 → 持ち物 → 特性 → 未対応の印」に並べ替えた。
5. **文の整理**: 防御側の残りHPの長い説明は「説明」ボタンの奥。単位・状態・エラーの 1 行(空なら満タン・メガの固定の理由・同じ向きにできない理由など、`aria-describedby` で結ぶもの)は残した。種族検索欄の補足(共通部品 `SpeciesSearchField`)は逆算・タイプバランスと一緒に直すため今回は残した。
6. 共通の `TypeBadge` は CSS 変数名に使えないタイプ ID では中立色に落とす(旧: 既定値つきの `var(--type-<id>, …)`)。未解決の var にならない点は同じ。

## 影響

新しい色の組は無い(brand.primary / on.primary、text.primary / surface.card は既存の検査で足りる)。CSS は gzip 7.8KB(予算 30KB)。
