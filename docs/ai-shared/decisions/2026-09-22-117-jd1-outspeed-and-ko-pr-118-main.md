## 2026-09-22: 判定 JD1(outspeed-and-ko)を PR #118 で main に統合
Decision: ADR-0701(`POST /api/judge/v1/outspeed-and-ko`。素早さの求め方・こだわりスカーフ・性格解決・上流呼び出し順序・エラー対応表)を PR #118 で main に統合した。critic は2回目で PASS(1回目 NG 重要3件: 上流エラーのログ未記録・pokedex 400 の扱いが ADR 未記載・defender 側スカーフ/種族差の未検証。いずれも修正し、期待値は実行結果で検算済み)。
Reason: `make test`・`make lint`・`make build`(ルート)が緑、critic PASS、他レーンの範囲外変更なし(COORDINATION.md の共有ファイル規約の範囲内)を確認してマージした。
Impact: 判定レーンのブランチを `feat/judge-jd2` に切り替えた(JD1 の `feat/judge-jd1` は削除)。JD2(複数の相手候補・場の効果・画面)は plan.md の方針どおり、着手前にユーザーへ確認する。
