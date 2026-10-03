## 2026-10-03: 判断待ちの一覧 PENDING.md を新設(issue #229。データレーン → 全レーン)
Decision: ユーザー決定 2026-10-03「既定案で OK」。`docs/ai-shared/PENDING.md` を新設した。人間の判断待ち・既定案で進行中・未回答の提案を 1 行ずつ、宛先レーン・起票日・issue 番号つきで置く。
COORDINATION.md に「判断待ちは PENDING.md に 1 行足す」を 1 行足した。回答済みの行は消す。
Reason: 判断待ちが plan.md・decisions・issue ラベルに分散し、ユーザーが 1 か所で見られなかった。
Impact: 今 open の needs-decision の issue と、各レーンの state の Next にある人間の確認事項を載せた。issue #229 は閉じてよい。
