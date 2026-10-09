## 2026-10-09: 【提案】夜間の CronJob が Mac のスリープで毎回飛ばされる(Wishlist → データレーン)
Decision(提案・既定案): `pokedex-import`(pokecalc 名前空間)の CronJob にも、`startingDeadlineSeconds` を十分長く(既定案 43200 = 12 時間)設定する。
Reason: k3d を動かしている Mac は夜間スリープしており、予定時刻(03:00)にクラスタが止まっている。`startingDeadlineSeconds` が短い(または未設定で
長く止まっていた)と、起きたときには期限切れで、その回が飛ばされる。2026-10-09 時点で `pokedex-import` の最後の実行は 10/3、`wishlist-refresher` は 10/5(600 秒)だった。
Impact: wishlist は `apps/wishlist/deploy/k8s/base/refresher-cronjob.yaml` を 43200 に変えた(このレーンの PR)。`pokedex-import` はデータレーンの判断で変える。
起きた時刻に実行されるため、夜間を前提にした他の処理(api の裏の更新を止める時間帯など)と重なることがある。
