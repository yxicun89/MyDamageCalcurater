## 2026-10-21: pokedex-import の startingDeadlineSeconds は 43200 にせず現状の 518400 を維持する(API レーン / ADR-0805)
Decision: 2026-10-09 の提案(43200 に設定)は採らない。`deploy/k8s/base/pokedex/cronjob-import.yaml` は ADR-0104 §5 のとおり最初から `startingDeadlineSeconds: 518400`(6日)・`concurrencyPolicy: Forbid` で、スリープで飛んだ回は 6 日以内に起きれば追いつく。43200 に下げると、半日を超えるスリープでかえって取りこぼす。
Reason: 提案は「未設定または短い」前提だったが実際は設定済み。週1回(土曜 12:00 JST)のため、最後の実行 10/3 は土曜で予定どおり(飛ばされていない)。overlays に startingDeadlineSeconds のパッチは無く(cloud は suspend のみ)、変更不要。43200 は毎日走る wishlist の refresher 向けの値で、週1回の import とは条件が違う。
Impact: マニフェスト無変更。テストに下限(12 時間以上 7 日未満)を足した(services/pokedex/importer/cronjob_layout_test.go)。docs/runbooks/data.md に追記。起きた直後の実行と手動 import の重なりは ADR-0109 の flock で排他。
