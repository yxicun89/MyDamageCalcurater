# Claude Code と Codex の協調運用(レーン制・PR で統合)

- 状態: 2026-09-21 制定、同日改訂(ユーザー決定: レーン制、main への統合は PR 経由)。Codex の確認は、Codex が次のセッションで `DECISIONS.md` に記録する
- 目的: **どちらかがレートリミット・上限・セッション切れで止まっても、もう一方(または同じ種類の別セッション)が同じ場所から続けられる**状態にする

## 原則

1. **担当は AI ではなく「レーン」に持たせる**: 作業は2本のレーンに分かれ、どの AI(Claude Code / Codex)がどのレーンを進めてもよい。
   引き継ぎ資料は作らない。レーンのブランチと `CURRENT_STATE.md` のレーン欄の `Next` がそのまま引き継ぎになる。
2. **1レーン = 同時に1セッション**: 同じレーンを2つのセッションで同時に進めない。着手時に `CURRENT_STATE.md` のレーン欄の `Active` を自分にして push する。
3. **止まる前に安全な場所へ**: 未コミットの作業を残さない。区切りごとに commit・push し、`Next` を具体的に書く。
4. **main は常に緑。main へは PR でしか入れない**: 直接 push・直接 merge をしない(Argo CD の GitOps が main を見ているため。ユーザー決定)。
   PR は、テスト・lint・公開前検査が通った作業だけ。WIP は PR にしない。
5. **相談は「既定値付きの提案」**: 判断が要るときは既定案を添えて `DECISIONS.md` に書き、既定案で先へ進む。人間の確認が本当に必要なものだけ `docs/plan.md` のブロッカーに書く。

## レーン・ディレクトリ・ブランチ

| レーン | 作業ディレクトリ | ブランチ | 範囲 |
|---|---|---|---|
| **ダメージ計算**(damage calc) | `~/MyDamageCalcurater` | `feat/calc-<phase名>`(既存の `feat/claude-p1-engine` はマージまでそのまま使う) | `docs/plan.md` の M1〜M4。`services/balance/` 以外 |
| **タイプバランス**(type balance) | `~/MyDamageCalcurater-tb`(同じリポジトリの git worktree) | `feat/tb-<stage名>`(既存の `feat/codex-tb0-foundation` はマージまでそのまま使う) | `services/balance/` とその Kustomize / Argo CD 定義。設計の正は `docs/type-balance-design.md` |

- ディレクトリはこの2つだけにする。レーンの作業ディレクトリは、どの AI が使ってもよい(同時に2つのセッションで開かない)。
- タイプバランスの worktree が無いときは作る: `git -C ~/MyDamageCalcurater worktree add ~/MyDamageCalcurater-tb <ブランチ>`
- 単発の修正は `fix/<レーン>-...`(例 `fix/calc-...`)。1つのブランチに複数の Phase/ステージを積まない。
- git の作者情報は、このリポジトリのローカル設定(`pokecalc-dev <noreply@example.com>`)を使う。個人の identity をコミットしない。
- リモートの URL・認証情報を文書・コミットに書かない。リポジトリを公開(public)にするのは、ユーザーの明示的な指示と `make check-publishable-full` の後だけ。

## 始めるとき

```
cd <レーンの作業ディレクトリ>
git fetch origin
git status --short --branch        # 未コミット・未 push が無いか
```
1. `origin/main` の `docs/ai-shared/CURRENT_STATE.md` と `DECISIONS.md` を読む(`git show origin/main:docs/ai-shared/CURRENT_STATE.md`)。
   **feature ブランチ内のコピーは古いことがある**。現在状態の正は `origin/main` と、そのレーンのブランチの最新コミット。
2. レーン欄の `Branch` をチェックアウトし、`git pull` してから `Next` の続きをする。別の AI が途中まで進めたブランチでも、そのまま続けてよい
   (前任の完了記録・テスト結果はうのみにせず、自分で検証する)。
3. レーン欄の `Active` を自分(例 `Claude Code` / `Codex`)にする。

## main への統合(PR)

条件(すべて満たすとき PR を作ってマージしてよい):
1. そのレーンのテストが通る。ダメージ計算は `make test` と、計算を変えたら `make test-golden`、WASM 境界を変えたら `make test-wasm`。
   タイプバランスは `docs/type-balance-test-strategy.md` に沿ったテスト(未作成の間は `services/balance` のテスト全件)。
2. `make lint` と `make check-publishable` が通る。独立レビュー(下記)を受けて指摘を反映済み。
3. 別レーンの範囲を、共有ファイルの規約が許す範囲を超えて変更していない。

手順(自分のブランチ上で完結する。main をチェックアウトしない):
```
git fetch origin
git merge origin/main              # 競合はここで解決する
make test && make lint && make check-publishable
git push origin HEAD
gh pr create --base main --head <ブランチ> --title "<要約>" --body "<何を・検証結果・レビュー結果>"
gh pr merge <番号> --merge         # マージコミットで入れる。squash・rebase・force push はしない
```
- PR の本文には、テスト・lint・公開前検査の結果と、独立レビューの判定を書く。
- **競合の解決**: `CURRENT_STATE.md` は自分のレーン欄を残し、他のレーン欄は main 側を採用。`DECISIONS.md` は両方の追記を残す。
  それ以外のファイルで別レーンの変更と競合したら、**推測で解決しない**。PR を作らず、`DECISIONS.md` に内容と既定案を書いて、自分の作業を続ける。
- マージしたら `DECISIONS.md` に「何を統合したか(PR 番号)」を1行追記し、レーン欄を更新する(次の PR に含める)。
- Phase/ステージが完了してマージしたブランチは削除する。途中の区切りで PR を出したブランチは、そのまま続けて使ってよい。

## 共有ファイルの編集

| ファイル | 規約 |
|---|---|
| `docs/ai-shared/CURRENT_STATE.md` | 自分が進めているレーンの欄だけ編集する |
| `docs/ai-shared/DECISIONS.md` | 追記のみ。既存エントリは編集しない |
| `go.work` | 自分のレーンのモジュールの `use` 行を追記してよい(タイプバランスは `./services/balance`) |
| ルートの `Makefile` | 自分のレーンのサービスの `include <path>/Makefile` の1行を追記してよい(タイプバランスは `include services/balance/Makefile`。ターゲット名は `balance-` 接頭辞) |
| `AGENTS.md` / `CLAUDE.md` / 本ファイル | 運用ルールの変更は、ユーザーの決定があったときだけ。変更したら `DECISIONS.md` に記録する |
| `docs/adr/` | 番号は `git fetch origin` した後の main の最新の次を取る。統合時に番号が衝突したら、後から統合する側が自分の ADR とその参照を振り直す |

## 止まるとき(レートリミット・上限・セッション終了の前後)

止まる前(予兆があるとき、または区切りごとに):
1. すべて commit する。未完了・未レビューなら `WIP(<タスク>): <何が未検証か>` の形にする(PR にはしない)。
2. `git push origin <ブランチ>`。
3. レーン欄の `Status` と `Next` に、**次にやることを具体的に**書き、`Active` を `なし` にする。この更新もブランチに commit・push する。
   (main へはまだ入らないので、次に始める人はレーン欄の `Branch` の最新コミットを見る。)

## 人間への質問(時間帯のルール。2026-09-21 ユーザー決定)

人間の判断が本当に必要になったら(仕様の未確定・`known_diffs.yaml` の承認・取り消しにくい操作など。CLAUDE.md「人間の確認が必要なこと」)、
**時刻を `TZ=Asia/Tokyo date +%H` で確かめてから**、次のとおりにする。

| 時間帯(日本時間) | やること |
|---|---|
| **日中 8:00〜23:00** | 質問する(Claude Code は `AskUserQuestion`)。質問はタスクの区切りでまとめて出し、各問に**既定案(推奨)**を付ける。返事を待つ間も、その判断に依存しない作業を進める |
| **深夜 23:00〜8:00** | **質問しない**。判断待ちの内容と既定案を `docs/plan.md` のブロッカー節の「【人間の確認待ち】」に書き、判断に依存しない作業を続ける。取り消しやすい判断(文書・設計の既定値など)は既定案で進めてよい。取り消しにくいもの(データ削除・公開・`known_diffs.yaml` への追加・force push など)は深夜に実行しない |

- 朝(8:00 以降)に最初に区切りが来たら、夜の間にたまった判断待ちを1回の質問にまとめて出す。
- 質問・既定案で進めた判断は `DECISIONS.md` に記録する(既定案で進めたものは「既定案で進行・ユーザー未確認」と明記)。
- 時間帯を変えたいときは、この表だけを直す。

## レビュー

- **各 AI が自分の独立レビュー**(Claude Code は critic、Codex は自分のレビュー役)を使う。他方の AI にレビューを依頼しない(ユーザー指示 2026-09-21)。
- 独立レビューのスキップや未実装ターゲットの正常終了を成功と数えない(CLAUDE.md)。

## 判断が必要なときの質問と深夜の自律作業(2026-09-21 ユーザー決定)

- **日中(8:00〜23:00 JST)**: 人間の判断が必要になったら、作業の区切りごとにまとめてユーザーに質問する。
  回答を待つ間は、その判断に依存しない作業を進める。
- **深夜(23:00〜翌 8:00 JST)**: ユーザーに質問しない。判断が必要になったら、元に戻しやすく影響の小さい案を選んで進め、
  `DECISIONS.md` に「暫定(深夜の自律判断。朝に確認)」と書く。朝(8:00 以降)の最初の報告で、その判断の確認を求める。
  - 検証済み(テスト・lint・独立レビューが通った)の PR は、**マージしないと作業が止まる場合に限り**深夜でも main にマージしてよい。
    止まらないなら、PR を作って朝の確認に回す。
  - 下の「ユーザーの確認が要ること」は深夜でも自動で進めない。ブロッカーとして記録し、別の作業へ進む。
- 時刻は作業環境の時計(`date`)で判断する。

## ユーザーの確認が要ること(変更なし)

Xcode の署名・実機インストール、Codex / 外部サービスのログイン、`known_diffs.yaml` への追加(ADR 付き。人間の承認)、クラスタ・DB データの削除、
リポジトリの公開(public 化)、LICENSE の決定。

## 起動の目安

```
cd ~/MyDamageCalcurater     && claude   # または codex(ダメージ計算レーン)
cd ~/MyDamageCalcurater-tb  && claude   # または codex(タイプバランスレーン)
```
