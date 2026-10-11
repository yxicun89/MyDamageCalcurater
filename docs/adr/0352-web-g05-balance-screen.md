# ADR-0352: タイプバランス画面の作り直し(G-05 / I-web-13f)

- 状態: 採用(2026-10-11)
- 関連: ADR-0339(方針)・ADR-0344(共通の部品)・ADR-0303(画面)・ADR-0304 A-14(使える口の判定)・ADR-0351(同じ作法)

## 決めたこと

docs/design.md「画面ごとの方向」のタイプバランスの行を実装した。動作・API 呼び出し・読み込みの流れ・状態(analyze / coverage / threats / recommendations、capabilities の判定)は変えない。

1. **入力済みの枠は `PokemonCard`**(画像かエンブレム・名前・タイプバッジ)を fieldset の中に出す。空の枠には出さない。legend(読み上げのグループ名「メンバーn」「仮想敵n」)は文字だけにした(画像は card 側へ)。ポケモン・特性・技の選択は select のまま。
2. **「メンバーを追加」「仮想敵を追加」は「+」の空きタイル**(`Tile empty`)。名前・無効化(6 体)は不変。
3. **升目は記号 + 文字**(`balanceCells.tsx` の `ResultCell`)。記号は既存アイコン(上=弱点・抜群、下=耐性・いまひとつ、×=無効、横線=等倍、チェック=安全、三角=注意)で装飾(aria-hidden)。**セルの textContent は従来の文字のまま**(「×2 弱点」。e2e が完全一致で見るため、記号は文字にせず SVG にした)。背景は `*-soft`、記号は状態色、文字は text.primary。この組は popPalette.test.ts で検査済みで、新しい色の組は無い(`BalanceScreen.g05.test.tsx` が CSS の参照とライト・ダークの比を確かめる)。仮想敵の「受ける/与える倍率」は分類が応答に無いため記号を付けない(Web で弱点・耐性を判定し直さない。ADR-0303 §7)。
4. **列・行の見出しはタイプバッジ**。メンバー・仮想敵の行見出しは画像かエンブレム + 名前(読み上げ名は名前だけ)。集計の列見出しは記号 + 語。
5. **人数は大きい数字**: 集計・攻撃範囲の人数セルは見出しの大きさの太字、仮想敵の人数は結果の大きさ(`.balance-count__number`)。i18n に `safeMembersPrefix`・`superEffectiveMembersPrefix`・`memberCountSuffix` を足した(つなげた文字は従来の `*MembersLabel` と同じ)。
6. **穴・候補・特性の表のタイプはバッジ**(`TypeList`。区切りの「・」は文字として残し、textContent は従来と同じ。空は「なし」)。仮想敵の結果の見出しは小さい `PokemonCard`。
7. 横スクロールの包み(`TableScroll`)は不変。仮想敵の表は語を 1 字ずつ折り返さないよう nowrap にして包みの中でスクロールさせる。選択欄は 44px。

## 適用しなかったもの

- 区切りボタン(`SegmentedControl`): この画面には 2〜4 択のモード・向きの切り替えが無い。
- 技の `MovePicker` 化: BalanceScreen*.test.tsx・visibleLabels・e2e/balance.spec.ts が select 前提(`selectOptions`)で多数あり、機械的な付け替えでは済まないため見送り(必要なら後続)。
- 文の削除: この画面に出ている文は状態(計算中・エラー・使えない案内)と見えるラベルだけで、禁止の説明文は無かった。種族検索欄の補足は 13c で「説明」の奥に済み。

## 既存テストの付け替え(理由つき・assertion は弱めていない)

- `BalanceScreen.visual.test.tsx` 1 件: 追加ボタンの見た目の class 検査を `ui-button ui-button--secondary` から `ui-tile ui-tile--empty` へ(部品が変わったため)。
- `BalanceScreen.test.tsx` 3 件: 「安全に受けられる 5人」「抜群を取れる 4人」「防御の穴: …」(2 件)を、数字・バッジが別要素になったため `getByText`(要素自身の文字)から段落の textContent の一致(`getByParagraph`)へ。語・並び・区切りは同じ。

## iOS・タイプバランスレーンへ

decisions/2026-10-11-web-g05-balance.md。語の変更なし(分割の語は同じ文字になる)。
