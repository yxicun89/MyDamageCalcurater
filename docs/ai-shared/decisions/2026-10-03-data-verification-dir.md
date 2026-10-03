## 2026-10-03: `~/MyDamageCalcurater` は動作確認専用、データ(damage calc)レーンは `~/MyDamageCalcurater-calc` で作業する(ユーザー決定)
Decision: `~/MyDamageCalcurater` は常に `main`(全レーンのマージ後)を置き、ユーザーが動作確認する場所にする。どのレーンの AI もここで作業・
コミット・ブランチの切り替えをしない(読み取りと、ユーザーに頼まれた `git pull`・`make deploy-latest` 等の確認だけ)。データ(damage calc:
engine・マスタ・pokedex・運用)レーンの作業ディレクトリは新しい worktree `~/MyDamageCalcurater-calc`(main から作成)。
Reason: ユーザーの意向「全レーンのマージ先と、ダメージ計算の作業を分けたい。ここで自分が動作確認する」。2026-09-24 に決めたが未実施だった移行を行う。
Impact:
- データレーンの起動は `cd ~/MyDamageCalcurater-calc && claude`(COORDINATION.md の起動の目安・レーン表を更新)
- 他のレーンの worktree は変わらない
- `~/MyDamageCalcurater` に残っている stash(`stash@{0}`。verify-m1.md の誤入力の退避)はユーザーの判断に任せる(AI は触らない)
- 動作確認の手順は `docs/verify-m1.md`(M2 は `docs/verify-m2.md`)を `~/MyDamageCalcurater` で上から流す
