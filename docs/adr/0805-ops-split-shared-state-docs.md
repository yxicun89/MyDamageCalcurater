# ADR-0805: 共有状態ファイルをレーン別・1件1ファイルに分ける(PR の競合の根本対策)

- 状態: 採用(2026-10-03。ユーザー要望「コンフリクトを根本的に解消したい」)
- 日付: 2026-10-03
- 関連: docs/ai-shared/COORDINATION.md(レーン制)、ADR-0803(CI が緑なら AI がマージ)

## 背景
全レーンの PR が `docs/ai-shared/CURRENT_STATE.md`(各レーンの Status/Next)・`DECISIONS.md`(末尾への追記)・`docs/plan.md`(チェック)
という**同じ数ファイル**を書き換えていた。直近 2 日の `docs/` の変更の上位は plan.md(37)・CURRENT_STATE.md(31)・DECISIONS.md(19)で、
main が進むたびに開いている PR のほぼ全てが競合し、解消の push が互いに競合する(複数セッションが同じブランチを解消しに行って
non-fast-forward になる)状態だった。ガードのスクリプトにマーカーが残って開発環境が止まる事故も起きた。

## 決定
1. `CURRENT_STATE.md` をレーン別ファイル `docs/ai-shared/state/<レーン>.md` に分け、`CURRENT_STATE.md` は索引にする。
   各レーンは**自分のレーンのファイルだけ**を編集する。
2. `DECISIONS.md` を 1 件 1 ファイル `docs/ai-shared/decisions/…` に分け(過去分は内容を変えずに移した)、`DECISIONS.md` は過去分の索引にする。
   新しい決定は**新規ファイルを足すだけ**(既存ファイルを編集しない)。
3. `docs/plan.md` は共有のまま、自分のレーンの行だけを編集する(他レーンの行の整形・並べ替えをしない)。
4. コンフリクトマーカーが追跡ファイルに残っていないかを `make lint` で検査する(`scripts/check-conflict-markers.sh`)。

## 影響と移行
- 分割前のブランチで `CURRENT_STATE.md`・`DECISIONS.md` を編集している PR は、この PR が main に入ったあと **1 回だけ**競合する。
  解消は「自分の変更分を、レーンのファイル / 新規の決定ファイルへ移す」(元のファイルは main の版にする)。
- 内容は一切変えていない(移しただけ。行数は索引・見出しの分だけ増減)。過去の文書にある「`DECISIONS.md` 2026-09-22」等の参照は、索引から該当ファイルへ辿れる。

## 却下した案
- `.gitattributes` の `merge=union`: GitHub 上の mergeability 判定で効くか保証が無く、重複行が静かに増える。
- 競合を GitHub Actions で自動解消する: 内容の取り違えを機械に任せることになる。まず競合しない構造にする。
- マージキュー: 有効だが、競合そのものは減らさない(構造の対策のあとに検討)。
