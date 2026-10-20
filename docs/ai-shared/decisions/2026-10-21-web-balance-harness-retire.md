## 2026-10-21: バランス統合テストの足場(balanceTestHarness)を撤去し、本物の TeamBalanceSection を描画する(ADR-0421 段階 4 の残り)
Decision: `web/src/balance/balanceTestHarness.tsx` を削除。`BalancePanel.integration*.test.tsx` 4 ファイル(116 件・削除 0 件)は `web/src/test/balanceTestBed.tsx` 経由で `team/TeamBalanceSection` を描画する。足場はメンバー入力欄(MemberFields)から下書きを作って本物の節に渡すだけ。素の挙動を見る 2 ケースだけ `directPanel`(useTeamBalance + BalancePanel を直接描画)を使う。
Reason: 旧画面の複製(仮想敵・API 配線)をテストに残すと、実アプリとの差が検証されない。
Impact: Web テストのみ。API・アプリ本体は不変。iOS 影響なし。

期待値・方式を変えたケース(7 件):
- online「…画面ごと使えないことを案内する」×4 の「パーティ・仮想敵の入力は残すが、全部 disabled にする」: 仮想敵は本物の節が使えないとき入力欄ごと出さないため、「出ない」の確認に替えた(メンバーは disabled のまま)。
- recommendations「…limit 無しで送る」: 本物の節は limit 5 を足すため直接描画に(期待値は不変)。
- recommendations「候補の表: …」: 本物の節は該当ポケモンを選ぶタイルに替えるため直接描画に(期待値は不変)。
- g05「待機の状態に説明の段落(<p>)が無い」: 本物の節はメンバー 0 体でパネルを描画しないため、直接描画(`directPanel`)でパネルの待機状態を作り、期待値は不変(`p` が 0 件)。
- 注: online 4 件のテスト名は旧のまま(期待値は「仮想敵は出ない」)。仮想敵の削除は旧足場は 1 体で不可、本物の節は常に可(差を検証するケースは元から無い)。
