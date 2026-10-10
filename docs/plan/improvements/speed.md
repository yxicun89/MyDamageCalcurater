# 改善要望(speed レーン)

/improve で追加する。タスク ID は `I-speed-<連番>`(ADR-0172)。

## 2026-10-04 使用感フィードバック第2回(docs/usability-round2.md の F-06。原文 9・10)
ユーザーの言葉の要約: 素早さ表は左に素早さ・右にポケモンの形はよい。右の自分のポケモンの素早さは、左の一覧の同じ高さに置いてほしい(選んだポケモンが表のどこか視覚的に分かる)。左の一覧は全部出すと長いので、スクロールに合わせて周辺だけ表示してほしい。

- [x] **I-speed-1(Web・F-06)**: 左の表を仮想スクロール(自前の windowing。画面外の段は DOM に出さず、aria-setsize / aria-posinset で総数を伝える)にし、右の位置マーカーを左の該当行(同速の段・境界)と同じ高さに揃えてスクロールに追従させる。「自分の位置へ移動」ボタンで中央に寄せる。トリックルームの昇順の表でも成立。完了: `web/src/speed/speedWindow.ts`(純粋な計算)・`SpeedScreen.tsx`(TierViewport)・テスト(speedWindow.test.ts・SpeedScreen.virtual.test.tsx)。設計は ADR-0608。見た目は F-12 の基盤の後で整える。iOS 分(F-06 の iOS)は別 agent・別 PR。

## I-speed-2(iOS・F-06)素早さ表の遅延描画と自分の位置の表示
- 要望: 左(表)の一覧が長いのでスクロールに合わせて周辺だけ表示する。選んだポケモンが表のどこにいるか同じ高さで分かるようにする(usability-round2 F-06)。
- [x] 表を `LazyVStack` で遅延描画。自分の位置を表の中(強調段・境界線)で示し、位置の読み上げ(全N段・自分はM段目)と「自分の位置へ」ボタンを追加(ADR-0517)
- [x] 純粋なロジック `SpeedTableNavigation`(PokeCalcCore)の XCTest と、UI テスト(遅延描画・ジャンプ)を追加

## 2026-10-11 使用感フィードバック第3回(docs/usability-round3.md の G-04)
- [x] **I-speed-3(Web・G-04)**: 素早さ表の「自分の周り」パネル(スクロール不要・折りたたみなし・前後の直近3段・端の合計・トリックルームは先に動く側/後に動く側)。ADR-0609。実装済み(speedNeighborhood.ts・SpeedNeighborhoodPanel.tsx。SpeedScreen の自分のカード `.speed-self` の結果の下に配置)。iOS は後続 PR。
