# 改善要望(speed レーン)

/improve で追加する。タスク ID は `I-speed-<連番>`(ADR-0172)。

## 2026-10-04 使用感フィードバック第2回(docs/usability-round2.md の F-06。原文 9・10)
ユーザーの言葉の要約: 素早さ表は左に素早さ・右にポケモンの形はよい。右の自分のポケモンの素早さは、左の一覧の同じ高さに置いてほしい(選んだポケモンが表のどこか視覚的に分かる)。左の一覧は全部出すと長いので、スクロールに合わせて周辺だけ表示してほしい。

- [x] **I-speed-1(Web・F-06)**: 左の表を仮想スクロール(自前の windowing。画面外の段は DOM に出さず、aria-setsize / aria-posinset で総数を伝える)にし、右の位置マーカーを左の該当行(同速の段・境界)と同じ高さに揃えてスクロールに追従させる。「自分の位置へ移動」ボタンで中央に寄せる。トリックルームの昇順の表でも成立。完了: `web/src/speed/speedWindow.ts`(純粋な計算)・`SpeedScreen.tsx`(TierViewport)・テスト(speedWindow.test.ts・SpeedScreen.virtual.test.tsx)。設計は ADR-0608。見た目は F-12 の基盤の後で整える。iOS 分(F-06 の iOS)は別 agent・別 PR。
