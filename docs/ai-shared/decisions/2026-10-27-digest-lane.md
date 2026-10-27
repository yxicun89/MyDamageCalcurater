## 2026-10-27: Digest レーンを追加(別アプリ「技術トレンド・ダイジェスト」を apps/digest/ に置く)
Decision: 別アプリ「技術トレンド・ダイジェスト」を `apps/digest/` に置き、レーン `digest`(ブランチ `lane/digest`)を追加した。
状態は `state/digest.md`、`CURRENT_STATE.md` と `COORDINATION.md` のレーン表に 1 行ずつ足した。起動指示は `apps/digest/KICKOFF.md`。
ルート `Makefile` への `include apps/digest/Makefile` は実装の M0 で足す(wishlist と同じ方式。Go は `GOWORK=off`)。
Reason: ユーザー決定(2026-10-27。置き場所は独立リポジトリではなくこのリポジトリの apps/ 配下)。次のセッションが起動指示から M0〜M7 を進めるため。
Impact: 他レーンへの影響は、include 後に `make test` / `lint` / `build` の時間が少し延びることだけ。digest は `apps/digest/` の外を変えない。
設計判断はダメ計の ADR 番号の帯を使わず、`apps/digest/` の中(SPEC の判断一覧)と decisions/ に持つ。
