# ADR-0527: 素早さ(iOS)の「自分の周り」パネル(G-04)

- 状態: 採用(2026-10-11。docs/usability-round3.md の G-04。Web は ADR-0609 で実装済み)
- 日付: 2026-10-11
- レーン: 素早さ(iOS)
- 関連: ADR-0609(Web 版。切り出し規則・文言の正)・ADR-0517(表の LazyVStack と自分の位置)・ADR-0503(素早さ画面)・ADR-0607(トリックルーム)・F-13(やさしい言葉)・G-05(ビジュアル刷新)
- 番号: iOS 帯 0500〜の次の空き(0527)を使う

## 背景
ADR-0517 の表は全体を見る用で、自分の周りを見るにはスクロールか「自分の位置へ」ボタンが要る。知りたいのは「自分のすぐ前後に誰がいるか」。Web は ADR-0609 でパネルを足した。iOS も同じ規則でそろえる。

## 決定
1. **Web と同じ規則**(ADR-0609 §3〜§4)。自分を中央に、先に動く側/後に動く側の直近 3 段(`SpeedNeighborhoodBuilder.defaultSteps`)、各段は実数値と代表名 1 体(多ければ「ほか n 体」)、同速の段は最大 4 体まで並べる。端は片側全体の合計体数(同速の段は含めない)。自分と同じ実数値の段が無ければ「ここに自分が入ります(同速なし)」。
2. **ADR-0517 の表とは独立**。`SpeedTableSection`(LazyVStack・位置の読み上げ・ジャンプボタン)は変えない。パネルは表のスクロール領域の外、自分の結果カードの近くに常時表示(折りたたみなし)。表のスクロール位置に関係なく木に残る。
3. **素早さは iOS で再計算しない**。入力は `SpeedViewModel.tableRows`(ADR-0503 §6 の表示用の並び)と自分の実数値(`positionState` の `speed`)。段の実数値と自分の実数値の直接比較だけで前後を決める。新しい API は足さない。
4. **トリックルーム**: 表の並びが昇順なので、通常は「速い側/遅い側」、トリックルームは「先に動く側/後に動く側」で見出しと合計行を言う(ADR-0607)。切り出しは並びで決まるため分岐は文言だけ。
5. **構造**: 純粋ロジックは `PokeCalcCore/SpeedNeighborhood.swift`(`SpeedNeighborhoodBuilder.build`・`SpeedViewModel.neighborhood`)、文言は `SpeedNeighborhoodLabels.swift`(`SpeedLabels` の拡張。Web の `neighborhood*` と同じ)、View は `ios/PokeCalc/SpeedNeighborhoodSection.swift`。見た目は G-05 の方針が入ったら View だけ差し替えられるようにする。
6. **a11y**: 色だけに頼らず「自分」バッジと矢印(↑ ↓)の文字を持つ。リストは先に動く側/後に動く側で名前付き(`accessibilityLabel`)。固定高にしない(Dynamic Type・AX5 で折り返す)。常時動くアニメーションは無し。識別子は `speedNeighborhood*`。
7. **単位**: 「体」(Web と同じ)。iOS の結果カード・表の既存文言は「行」(ポケモン × 調整)を使うが、パネルは Web の文言をそのまま使う(Web との一致を優先)。一致しない見え方が問題になったら別 ADR で両方そろえる。

## 選ばなかった案
- ADR-0609 の A〜E と同じ理由(表の自動スクロール・折りたたみ・結果カードへの足し込み・サーバー API・段数の可変 UI)。
- 表の `ScrollViewReader` で自分の行を固定表示する案: ADR-0517 の遅延描画と衝突する。

## 結果・今後
- 受け入れ条件と失敗するテストを先に置く(SpeedNeighborhoodTests・SpeedNeighborhoodUITests)。実装者は `build` と View と SpeedScreenView への差し込みを行う。
- G-05 の見た目の方針が入ったら View の見た目だけを差し替える。`SpeedNeighborhoodBuilder.build` の契約は保つ。
