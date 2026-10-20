## 2026-10-21: 【提案】Mac のスリープ明けに CronJob の予定がそもそも起動されない(Wishlist → 運用レーン)
Decision(提案・既定案): スリープ明け(または毎朝)に、クラスタの CronJob を 1 回だけ「触る」(注釈を付け直す)仕組みを運用側で用意する。既定案は Mac の launchd で起動時・復帰後に
`kubectl annotate cronjob -A --all <キー>=<時刻> --overwrite` を流すスクリプト(`scripts/` 配下。運用レーンの判断)。
Reason: `wishlist-refresher`(03:00 JST、`startingDeadlineSeconds: 43200`)が 10/20・10/21 と 2 日続けて起動されなかった。ノードの時計は正しく、CronJob は停止(suspend)されておらず、
k3s のログにもエラーは無い。CronJob の controller は「次の予定まで待つ」タイマーで動くが、Docker の VM がスリープで止まるとタイマーが遅れ、予定時刻を過ぎても評価されないまま残る。
10/21 03:03 に CronJob へ注釈を付け直したところ、controller が評価し直し、期限内の 03:00 の回がすぐ実行されて成功した(items=3 failed=0)。2026-10-09 の
`startingDeadlineSeconds` を延ばす提案(decisions/2026-10-09-wishlist-cronjob-sleep.md)だけでは足りない。
Impact: wishlist は、詳細を開いたときに 24 時間より古い目安価格を裏で取り直すので、夜間の更新が飛んでも使えなくはない(ホームの価格札が古いまま残る)。
`pokedex-import` など他の名前空間の CronJob も同じ影響を受けうる。共通基盤の話なので wishlist レーンは変更せず提案だけにする。
