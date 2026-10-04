# ADR-0172: docs/plan.md を区画ごとのファイル docs/plan/<区画>.md に分ける

- 状態: 採用(2026-10-03。ユーザー決定「全レーンが同じファイルを編集して PR が衝突する問題を構造で解決する」)
- 日付: 2026-10-03
- 関連: ADR-0805(共有状態ファイルのレーン別・1件1ファイル化。本 ADR はその follow-up)、docs/ai-shared/COORDINATION.md

## 背景

ADR-0805 で `CURRENT_STATE.md`・`DECISIONS.md` は分けたが、`docs/plan.md`(約 450 行・約 70KB)は「共有のまま、自分のレーンの行だけ編集する」とした。
plan.md は直近 2 日の `docs/` の変更回数が最多(37 回)で、全レーンの PR が同じファイルの近い位置(各区画の末尾・「改善要望」の末尾・
「ブロッカー」)に行を足すため、行単位の編集規則だけでは git の 3-way merge が隣接行の変更として衝突させる。

## 決定

1. **区画(H2 見出し)ごとにファイルを分ける**。`docs/plan.md` には凡例・区画の索引・マイルストーン表だけを残す。

   | 区画(見出し) | ファイル |
   |---|---|
   | M1: ブラウザで計算できる | `docs/plan/m1.md` |
   | M2: 保存・構築 | `docs/plan/m2.md` |
   | M3: iOS | `docs/plan/m3.md` |
   | TB: タイプバランスチェッカー | `docs/plan/tb.md` |
   | SP: 素早さ比較 | `docs/plan/speed.md` |
   | JD: 判定 | `docs/plan/judge.md` |
   | AJ: 調整 | `docs/plan/adjust.md` |
   | DOC: 文書 | `docs/plan/doc.md` |
   | M4: 運用 | `docs/plan/m4.md` |
   | 後続: 要件との対応 | `docs/plan/followups.md` |
   | ブロッカー | `docs/plan/blockers.md`(既存分)+ `docs/plan/blockers/<レーン>.md`(新規分) |
   | 改善要望 | `docs/plan/improvements.md`(既存分)+ `docs/plan/improvements/<レーン>.md`(新規分) |

2. **移動は機械的に行う**。見出し・チェックボックスの行・タスク ID・本文は一字も変えない。唯一の例外は、1 段深くなったことで壊れる
   相対リンク 2 か所(`](plan-archive.md)`・`](coding-rules.md)` → `](../…)`)。移動の前後で「旧 plan.md の区画部分の行」と
   「区画ファイルを索引の順に連結した行」が(相対リンクの `../` を除いて)完全に一致することを確かめた(PR 本文の検証)。

3. **全レーンが追記する区画はレーン別ファイルに分ける**。「改善要望」(/improve で全レーンが末尾に足す)と「ブロッカー」(深夜の
   【人間の確認待ち】を全レーンが足す)は、区画ファイルを分けても同じファイルの末尾で衝突する。そこで:
   - 既存の項目は内容を変えずに `improvements.md`・`blockers.md` に移す。これらは**状態の更新・解決済みの削除だけ**を行い、新しい項目を足さない。
   - 新しい項目は `docs/plan/<区画>/<レーン>.md`(レーン名は ADR-0805 の `state/<レーン>.md` と同じ: data・api・web・ios・type-balance・
     speed・judge・ops。ダメージ計算レーン〈engine・マスタ〉は `data`。calc という名前は使わない。ファイルが無いレーンは同じ形で新規作成し、索引はディレクトリへのリンクなので更新不要)に足す。/improve のタスク ID は、レーン間で連番が衝突しないよう `I-<レーン>-<連番>` とする。
   - 既存項目をレーン別に振り分けなかったのは、項目にレーンの記載が無いものが多く、振り分けが機械的な移動でなくなる(判断が入る)ため。

4. **マイルストーン区画(M1〜M4)は区画ファイルのまま共有する**。M1〜M4 は複数レーン(データ・API・Web・iOS)が担当するが、
   行の追加は区画内の自分のレーンの行の近くに行い、区画の末尾にまとめて足さない。それでも衝突が続く区画は、同じ要領(区画/レーン)で
   後から分ける(分けた時点で本 ADR に追記する)。

5. **他レーンの区画・行を編集するときの約束**: 状態の印(`[ ]`/`[~]`/`[x]`/`[!]`)だけの更新(自分の作業で他レーンのタスクが
   完了・不要になったとき)と、そのレーンから依頼されたときに限る。本文は変えず、PR 本文に書く。それ以外は decisions に提案として書く
   (COORDINATION.md「共有状態ファイルの分割」)。

6. **plan.md を参照する既存の記述**:
   - 運用の手順(CLAUDE.md・AGENTS.md・COORDINATION.md・development-workflow.md・skills・KICKOFF・README 類)は新しい場所に直す。
     CLAUDE.md・AGENTS.md はパスの事実の訂正だけに留める。
   - 行番号での参照(`docs/plan.md:331` 等)は区画ファイルの名前に直す。
   - タスク ID・見出しでの参照(コードのコメントの「plan.md AJ4」、ADR・decisions 等の過去の記録)は直さない。
     `docs/plan.md` の索引から区画ファイルを引けるので壊れない。過去の記録は書き換えない(ADR-0805 と同じ)。

7. **整合検査を `make lint` に足す**(`scripts/check-plan.sh`。テストは `scripts/check-plan_test.sh` を `make test-scripts` に追加):
   - 索引(`docs/plan.md`)からリンクされたファイル・ディレクトリが存在する。
   - `docs/plan/` 配下の Markdown がすべて索引から辿れる(索引に無い区画ファイルを作らない)。
   - `docs/plan.md` にチェックボックスの行が無い(タスクは区画ファイルに置く)。
   - 同じチェックボックスの行が 2 回現れない(マージの解消で行が二重になる事故)。
   - 先頭のタスク ID(`P4-17` 等)が重複しない。分割前の main で重複していた ID(`P4-17`・`P5-5c`・`P5-5d`・`P6-21`・`P6-24`)だけは許可リストに置く(内容を変えずに移す方針のため)。`P6-24` は同じ行が `[ ]` と `[x]` で 2 回ある(main でのマージの残り)ので、iOS レーンが古い行を消したら許可リストから外す。許可した ID の行は「同じ本文の行」の検査からも外す。

## 影響と移行

- 分割前のブランチで `docs/plan.md` の区画を編集している PR は、この PR が main に入ったあと **1 回だけ**競合する。解消の手順:
  1. `git merge origin/main`(競合する)。
  2. `docs/plan.md` は main の版にする(`git checkout origin/main -- docs/plan.md`)。
  3. 自分のブランチで plan.md に加えた変更(`git diff $(git merge-base HEAD origin/main) HEAD -- docs/plan.md` で確認。
     merge の途中でも HEAD は自分のブランチの先端)を、索引で該当する
     `docs/plan/<区画>.md` に同じ行のまま移す。改善要望・ブロッカーの新しい項目は `docs/plan/<区画>/<レーン>.md` に移す。
  4. `scripts/check-plan.sh` を実行して通ることを確かめ、merge commit を作る。
- `/phase` で引数を省略したときの「最初の未完了タスク」は、`docs/plan.md` の索引の順に区画ファイルを横断して探す。

## 却下した案

- **レーン別に全部分ける(docs/plan/<レーン>.md)**: M1〜M4 はマイルストーン単位で人間が進捗を見る表であり、レーンで割ると
  マイルストーンの見通しが失われる。行の振り分けに判断も入る。
- **`.gitattributes` の `merge=union`**: ADR-0805 と同じ理由(GitHub の mergeability で効く保証がなく、重複行が静かに増える)。
- **plan.md を共有のまま運用規則だけ強める**: ADR-0805 で採ったが、隣接行の追加は規則を守っても衝突する。
