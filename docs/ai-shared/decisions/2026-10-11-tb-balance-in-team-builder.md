## 2026-10-11: タイプバランスを構築画面に組み込む(別画面を廃止。ADR-0421。Web・iOS レーンへ)
Decision: 入力は構築編集の 6 枠の下書き(空の枠は無視・1 体から結果が出る)。出力は構築編集の中の「バランス」セクション(防御相性・集計・coverage・おすすめ。仮想敵は任意の節)。別のタイプバランス画面とナビの入口は廃止する。API・契約は変えない。
- 境界: 構築側が渡すのは `BalanceInputMember[]`(Web)/ `BalanceViewModel.setParty`(iOS)だけ。呼び出し・debounce(300ms)・古い応答の無視はバランス側の `useTeamBalance` / `BalanceViewModel` に閉じる。balance が使えなくても構築の編集・保存は止めない。
- 段階: 1 取り出し(タイプバランスレーン。Web の `useTeamBalance`・`BalancePanel`、iOS の `setParty`)→ 2 組み込み(Web: `TeamMemberEditor`、iOS: `TeamEditView`)→ 3 見た目(G-05・ec の配色案)→ 4 旧画面・旧タブ・旧 e2e の整理(`#balance` は構築へ)。
- Web レーンへ: 段階 1 が入るまで `BalanceScreen` の見た目の PR(I-web-13f)は止める(取り出しと同じ行を触る)。段階 1 の後は `BalancePanel` と構築編集の中の「バランス」に対して見た目を作る。
Reason: ユーザー要望(構築で 6 匹を選ぶのでその画面にバランスを出したい。数匹でも判定したい。1 体ずつ追加する手間をなくす)。
Impact: Web(TeamMemberEditor・ナビ・ルート・e2e)、iOS(TeamEditView・RootView の入口)、タイプバランス(取り出し)。
