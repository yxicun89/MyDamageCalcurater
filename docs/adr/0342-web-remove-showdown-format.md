# ADR-0342: Showdown 形式の取り込み・書き出しを廃止する — G-03

- 状態: 採用(2026-10-11。ユーザー決定。実装 PR で採用にした)
- 関連: docs/usability-round3.md G-03、ADR-0310(変換部 `showdownFormat.ts`)・ADR-0321(取り込み・書き出し UI)・ADR-0332(構築の作り直し。Showdown は補助の入口)・
  ADR-0213 §4(クライアント担当)・ADR-0506(iOS の日本語名 Showdown 風テキスト)を一部置き換える。docs/ai-shared/decisions/2026-10-11-web-g03-remove-showdown.md

## 背景

ユーザーが「Showdown 形式の取り込みは使いにくい」とし、入口ごとの廃止を決めた(G-03)。構築は 6 枠の編集画面で作れるため、補助の入口は要らない。

## 決定

1. Web の構築画面から、一覧の下の「Showdown 形式で取り込む」と編集画面の下の「Showdown 形式で書き出す」を外す。ほかの Web 画面(計算・逆算・タイプバランス・お気に入り・調整・このアプリについて)に Showdown 形式の入口は無かった。
2. 取り込み・書き出しのためだけにあったコードは消す(他レーンが import していないことを確認済み。`showdownFormat.ts` を使うのは構築画面の部品とそのテストだけだった):
   `team/showdownFormat.ts`・`showdownMaster.ts`・`showdownImportPlan.ts`・`TeamShowdownImport.tsx`・`TeamShowdownExport.tsx`、`i18n/team.ts` の `teamShowdownText`、`TeamScreen.css` の `.team-fold*`・`.team-showdown*`、およびそれらだけのテスト
   (`showdownFormat*.test.ts`・`showdownMaster.test.ts`・`showdownImportPlan.test.ts`・`TeamScreen.showdown.test.tsx`・`i18n/teamShowdownText.test.ts`)。テストの削除はこの機能ごと廃止するユーザー決定による(CLAUDE.md 絶対ルール 6 の「通すために消す」には当たらない)。
   ほかの振る舞いを確かめるテスト(`TeamScreen.rebuild`・`App.teamTab`・`teamRebuildText`・E2E `teamRebuild`)は Showdown の検査だけを外し、入口が無いことの検査(R-9・E2E)を足した。
3. 残すもの: 「このアプリについて」の出典「Pokémon Showdown(MIT License)」はマスタの取得元の表記(ADR-0002)で、この機能とは別。マスタ API の `showdownId`(`master/exportSnapshot.ts`)も取得元との突き合わせ ID で別。どちらも変えない。
4. API・team-svc・engine への影響なし。保存済みの構築データはそのまま。
5. 過去の ADR(0310・0321・0332 ほか)と計画の完了印は履歴として直さない。冒頭の注記と本 ADR で廃止を示す。iOS の入口は iOS レーンが外す(decisions を参照)。

## 結果

`git revert` 相当で戻せるが、戻す予定はない。再び必要になったら新しい ADR で設計し直す。
