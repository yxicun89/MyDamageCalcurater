# ADR-0170: 共有ログを1エントリ=1ファイル(レーン別・区画別)に分ける

- 状態: 採用(2026-10-03。ユーザー決定「全レーンが同じファイルの末尾付近を編集して PR が頻繁に衝突する問題を、構造で解決する」)
- 日付: 2026-10-03
- 関連: docs/ai-shared/COORDINATION.md(共有ファイルの編集・競合の解決)、AGENTS.md、CLAUDE.md「最初に読むもの」、
  DECISIONS.md 2026-09-24(古い節の退避はユーザーの決定があってから)

## 背景

7〜8 レーンが並行して PR を出し、次の3ファイルの末尾付近(または同じ節)を毎回編集していた。

| ファイル | 規模 | 編集のされ方 |
|---|---|---|
| `docs/ai-shared/DECISIONS.md` | 約 2,100 行・211 エントリ | 全レーンが末尾に追記 |
| `docs/ai-shared/CURRENT_STATE.md` | 約 500 行 | 各レーンが自分の節を編集(隣の節と行が近い) |
| `docs/plan.md` | 約 420 行 | 各レーンが自分の区画のチェックを更新し、ブロッカー・改善要望の末尾に追記 |

git の3方向マージは「同じ位置への追記」を競合として扱うため、内容が独立していても PR ごとに競合解決が要った。

## 決定

### 1. DECISIONS.md: 新規は `docs/decisions/<YYYY-MM-DD>-<lane>-<slug>.md` に1件ずつ。既存は凍結アーカイブのまま残す
- 新しい判断・提案・依頼は `scripts/new-decision.sh <lane> <slug>` で1ファイルを作って書く。別ファイルなので、別レーンの PR と競合しない。
- ファイルの先頭に、既存の書式の項目(日付・レーン・状況・既定案・状態)を `- 項目: 値` の形で持つ。本文は既存エントリと同じく
  `Decision:` / `Reason:` / `Impact:` か箇条書き。
- 状態の値: `open`(既定案で進行・ユーザー未確認)/ `未回答`(相手レーン・ユーザーの回答待ち)/ `決定` / `撤回`。
  `scripts/list-decisions.sh` が `open` と `未回答` を一覧する(`--all` で全件、`--archive` でアーカイブの見出し)。
- 既存エントリの本文は編集しない。状態の行(`- 状態:`)だけは、回答・決定のたびに発信レーンか宛先レーンが更新してよい(一覧が実態に合うため)。
- **既存の DECISIONS.md は移さない**(削除もしない)。先頭に「凍結・新規は docs/decisions/」の注記だけを足し、本文は一字も変えない。
  移さない理由:
  1. 既存エントリは「下の『…』を参照」のように**位置で他のエントリを指す**ものがあり、分割すると参照が壊れる。
  2. 見出しにレーンが構造化されておらず(括弧書きの自由記述)、ファイル名の `<lane>`・`<slug>` を機械的に決められない。
  3. コード・テスト・ADR の約 100 箇所が `DECISIONS.md 2026-09-25「…」` の形で参照しており、アーカイブがその場所に残れば参照はそのまま有効。
- アーカイブの索引は `scripts/list-decisions.sh --archive`(見出しの行番号・日付・タイトル)で引く。

### 2. CURRENT_STATE.md: レーン別の `docs/ai-shared/state/<lane>.md` に分ける
- `<lane>` はブランチの接頭辞と同じ: `data`・`api`・`web`・`ios`・`tb`・`speed`・`judge`・`ops`。
- 各レーンの節(見出し行・Lane/Active/Branch/Status/Next)は**一字も変えずに**移す。
- `CURRENT_STATE.md` には索引(レーン → ファイル)と、全レーン共通の `## Shared Interfaces` だけを残す。
- 各レーンは自分のファイルだけを編集するので、レーン間で競合しない。

### 3. plan.md: H2 の区画ごとに `docs/plan/<区画>.md` に分ける
`docs/plan.md` は凡例・「区画の索引」・マイルストーン表だけにする。区画と主な編集者:

| ファイル | 区画(見出しはそのまま) | 編集するレーン |
|---|---|---|
| `docs/plan/m1.md` | M1: ブラウザで計算できる(Phase 0〜4) | データ・API・Web(Phase ごとにレーンが分かれる) |
| `docs/plan/m2.md` | M2: 保存・構築 | API・Web・データ |
| `docs/plan/m3.md` | M3: iOS | iOS |
| `docs/plan/tb.md` | TB: タイプバランスチェッカー | タイプバランス |
| `docs/plan/speed.md` | SP: 素早さ比較 | 素早さ |
| `docs/plan/judge.md` | JD: 判定 | 判定 |
| `docs/plan/adjust.md` | AJ: 調整 | ダメージ計算(データ・API・Web・iOS) |
| `docs/plan/doc.md` | DOC: 文書 | 全レーン(1行ずつ) |
| `docs/plan/m4.md` | M4: 運用 | 運用・各レーン |
| `docs/plan/followups.md` | 後続: 要件との対応 | 担当レーン付き |
| `docs/plan/blockers.md` | ブロッカー | 全レーン |
| `docs/plan/improvements.md` | 改善要望(/improve で追加) | 全レーン |

- 1レーンに閉じた区画(M3・TB・SP・JD)は分けるだけで競合が無くなる。
- **複数レーンが同じ区画を編集するもの(M1・M2・AJ・DOC・M4・後続・ブロッカー・改善要望)も分ける**。理由: 競合の多くは
  「別の区画への編集が、同じファイルの近い行(区画の境目・末尾)に重なる」ことで起きていた。区画ごとのファイルにすれば、別区画の編集は
  別ファイルになり衝突しない。同じ区画の中の既存行のチェック更新([ ]→[x])は行が離れていれば競合しない。
- 残る衝突点は `blockers.md`・`improvements.md` の**末尾への同時追記**だけ。これは区画をさらにエントリ単位へ割らない(チェックボックスの行を
  変えない・plan の読み方を変えない)。代わりに、新しい軽微な指摘・要望は従来の方針どおり GitHub issue を正とし、plan には1行だけ足す。
- チェックボックスの行は一字も変えない。区画の中の相対リンク(`plan-archive.md`・`coding-rules.md`)だけ、1段深くなった分 `../` を付けた。
- `docs/plan.md` の索引に各区画の見出しをそのまま載せるので、既存の参照「docs/plan.md「SP: 素早さ比較」」は索引から辿れる。

### 4. 参照の更新範囲
- 運用の手順(AGENTS.md・CLAUDE.md・COORDINATION.md・README_AI_SHARED.md・development-workflow.md・`.claude/skills/*`・
  `.claude/agents/*`・`scripts/`・テスト)は新しい構造に直す。CLAUDE.md・AGENTS.md は**パスの事実の訂正だけ**(人間のレビュー対象)。
- 過去の記録(ADR 本文・コード・テストのコメントの `DECISIONS.md 2026-09-25「…」`・`docs/plan.md P4-9` などの出典表記)は変えない。
  DECISIONS.md はアーカイブとして同じ場所に残り、plan.md は索引から区画へ辿れるため、参照は壊れない。
- `scripts/gitops_test.sh` の「plan に issue #237 の行がある」検査は、検査対象を `docs/plan.md` と `docs/plan/*.md` に広げる(検査は弱めない)。

### 5. 並行している PR への配慮
- `.gitattributes` に `docs/ai-shared/DECISIONS.md merge=union` を足す。未マージの PR が凍結後のアーカイブに追記していても、
  ローカルの `git merge origin/main` で両方の追記が残る(追記専用ログにだけ使う)。
  `CURRENT_STATE.md`・`plan.md` には使わない(既存行の書き換えが重複行として両方残る危険があるため)。
  注意: GitHub の PR 画面のマージ判定は `.gitattributes` のマージドライバを使わないため、競合の解消はローカルの `git merge origin/main` で行う
  (COORDINATION.md の手順どおり)。
- 未マージの PR の移行方法は COORDINATION.md「共有ログの構造(ADR-0170)」に書く。
- 「凍結アーカイブへの新規追記を lint で警告」は**見送る**。既存の `make lint` には文書の検査が無く(Go・shell・Node・k8s・公開前検査のみ)、
  1件のために文書 lint の枠組みを足すのは過剰。代わりにアーカイブの先頭の注記・COORDINATION.md・`new-decision.sh` で誘導する。

## 結果
- 別レーンの PR が共有ログで競合するのは、同じ区画の同じ行・`blockers.md`/`improvements.md` の末尾への同時追記に限られる。
- 起動時に読む量が減る(自レーンの state ファイル・区画ファイル・`list-decisions.sh` の未決一覧だけ)。
- 判断の記録が2か所(アーカイブと docs/decisions/)になる。2026-10-03 以前は DECISIONS.md、以後は docs/decisions/。
