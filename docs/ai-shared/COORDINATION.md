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
| **データ**(damage calc: engine・マスタ) | `~/MyDamageCalcurater` | `feat/calc-<phase名>`(既存の `feat/claude-p1-engine` はマージまでそのまま使う) | `engine/`、`tools/golden/`・`testdata/golden/`、Phase 2(`services/pokedex/`・`tools/importer/`・`services/internal/master/`・MySQL の k8s 定義)、および他のレーンに属さない M1〜M4 のタスク |
| **API**(damage calc: サービス) | `~/MyDamageCalcurater-api` | `feat/api-<phase名>` | Phase 3(`services/calc/`・`services/gateway/`・契約テスト・k3d のスモーク)。**`api/openapi.yaml` と生成物(`services/internal/api/`)を変更できるのはこのレーンだけ** |
| **Web**(damage calc: 画面) | `~/MyDamageCalcurater-web` | `feat/web-<phase名>` | Phase 4(`web/`・Playwright)。`make wasm` の成果物を使う |
| **タイプバランス**(type balance) | `~/MyDamageCalcurater-tb`(同じリポジトリの git worktree) | `feat/tb-<stage名>`(既存の `feat/codex-tb0-foundation` はマージまでそのまま使う) | `services/balance/` とその Kustomize / Argo CD 定義。設計の正は `docs/type-balance-design.md` |
| **iOS**(damage calc: iOS アプリ) | `~/MyDamageCalcurater-ios` | `feat/ios-<phase名>` | M3 の Phase 6(`ios/`)。API クライアントは `api/openapi.yaml` から swift-openapi-generator で生成し、手で書かない。署名・実機インストールは人間(CLAUDE.md) |
| **素早さ**(speed) | `~/MyDamageCalcurater-speed` | `feat/speed-<stage名>` | 素早さ比較サービス(`services/speed/` とその Kustomize、Web の素早さ画面 `web/src/speed/`)。設計の正は `docs/speed-design.md`(このレーンが作る)。Web のタブ登録(`web/src/App.tsx` 等のアプリの骨組み)は共有ファイルとして自分の1項目を足すだけにし、骨組みの変更が要るときは Web レーンに DECISIONS.md で提案する |
| **判定**(judge。素早さ×ダメージ連動) | `~/MyDamageCalcurater-judge` | `feat/judge-<stage名>` | 素早さと確定数を1回で判定するサービス(`services/judge/` とその Kustomize)。設計の正は `docs/judge-design.md`(このレーンが作る)。calc-svc の公開 API と pokedex-svc の公開 API を呼ぶ(speed-svc には依存しない)。Web/iOS の画面は着手時に判断する |

- ディレクトリはレーンの数だけ(いまは7つ)にする。レーンの作業ディレクトリは、どの AI が使ってもよい(同時に2つのセッションで開かない)。
- レーンの worktree が無いときは作る: `git -C ~/MyDamageCalcurater worktree add ~/MyDamageCalcurater-<レーン> <ブランチ>`

### レーン間の依存と共有ファイル(4レーン。2026-09-21 ユーザー決定)

- **他のレーンの範囲のファイルは変更しない**。必要な変更は `DECISIONS.md` に既定案付きの提案として書き、そのレーンに任せる。待たずに進めるため、暫定の境界(インターフェース・架空データ・fake)を自分のレーン内に置いてよい。
  - API レーン: マスタの読み込みは `services/internal/master`(データレーン)の写像が main に入るまで、自分の中の差し替え可能なインターフェースと架空データで作る。engine は変更せず、公開 API を呼ぶだけ。
  - Web レーン: 計算は WASM(`engine/wasmapi` の JSON 契約。ADR-0011)で先に作る。API の型が要る部分(P4-5)は、API レーンが `api/openapi.yaml` を更新して main に入れてから追従する。マスタ(種族・技の一覧)は pokedex-svc ができるまで架空データで作る。
  - iOS レーン: API の契約は `api/openapi.yaml`(API レーンが持ち主)に追従するだけで変更しない。P3 のサーバーができるまでは生成クライアントに対するモック(架空データ)で画面を作る。
    Xcode が無い間は Swift Package(生成クライアント・モデル・デザイントークン)と `swift test` の範囲で進め、Xcode プロジェクトとシミュレータのテスト(`make ios-test`)は Xcode の導入後に行う。デザイントークンの値は Web と同じ(docs/design.md)
  - データレーン: `api/openapi.yaml` を変えない。pokedex の API が要るときは DECISIONS.md で API レーンに提案する。
- **両方が触る共有ファイル**:
  - `docs/plan.md`: 自分のレーンのタスクの行(とブロッカー節の自分の項目)だけを更新する。
  - ルートの `Makefile`・`go.work`: 自分のレーンのターゲット・`use` 行の追加だけ。統合時の競合は両方を残して解決する。
  - `docs/ai-shared/CURRENT_STATE.md`: 自分のレーンの欄だけ。
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

### 承認省略の設定と運用上の注意(2026-09-23 ユーザー決定)

ユーザーの Claude Code グローバル設定(ユーザー設定ファイル。全レーン共通)で、次を**確認なし**にしている。
- `git push`(feature ブランチへの push。リモートブランチの `--delete` を含む。2026-09-23 追記: 当初は確認要にしていたが、
  マージ後のブランチ削除のたびに確認が挟まる運用負荷が大きいためユーザーが allow へ変更)
- `gh pr create`
- `gh pr merge`

一方、次は**引き続き禁止**(deny。実行されない)。
- main への直接 push(`git push … main` 系すべて)・force push(`--force`/`-f`/`+refspec`)・`--mirror`・`--all`
- `rm` は変更していない。auto mode の既定判断のまま(危険なものは引き続き確認を求める)

**この設定により、`gh pr merge` の直前にユーザーが目を通す機会が無くなる**。したがって、PR を作る前に必ず次を自分で確認してから進める(これまで以上に重要):
1. そのレーンのテスト・`make lint`・`make check-publishable` が通っていること。
2. 独立レビュー(critic)が PASS していること(FAIL のまま PR を作らない)。
3. PR の本文に検証結果とレビュー結果を書く(あとから確認できるように)。

ローカルで `git merge` して `origin/main` へ直接 `push` することは、main への直接 push を拒否する deny ルールで従来どおり止まる。
main への統合は必ず PR(`gh pr create` → `gh pr merge`)を経由する。

**注意(2026-09-23 に判明)**: `.claude/settings.json`(このリポジトリに Git 管理されている、全ワークツリー共通のプロジェクト設定)は
ユーザーのグローバル設定より優先される。当初グローバル設定だけ `gh pr merge` を allow にしたが、プロジェクト側の `.claude/settings.json` に
`gh pr merge` の ask ルールが残っていたため実際には確認を求められ続けていた。プロジェクト側も allow に揃えたので、以後は両方が allow の状態。
権限設定を変えるときは、グローバル設定だけでなくこのプロジェクトの `.claude/settings.json`(Git 管理下)も確認すること。

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
| `docs/adr/` | **新しい ADR の番号はレーンごとの帯から取る**(2026-09-22。並列で「main の最新の次」を取ると衝突するため): データ `0100〜` / API `0200〜` / Web `0300〜` / タイプバランス `0400〜` / iOS `0500〜` / 素早さ `0600〜` / 判定 `0700〜`。帯の中で自分のレーンの最新の次を使う。`0001〜0019` の既存の番号はそのまま(衝突しているものは、後から統合する側が自分の帯へ振り直す) |

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
- **深夜の PR マージ**: 検証済みの PR は、**マージしないと作業が止まる場合に限り**深夜でも main にマージしてよい。条件: マージ前に
  `make test`(balance を含む)・`make lint`・`make build` を通し、engine を変えたなら `make test-golden` も通す。独立レビューが PASS であること。
  マージした理由を `DECISIONS.md` に記録し、朝の最初の報告に含める。止まらないなら PR を作って朝の確認に回す(2026-09-21 ユーザー決定)。
- 時間帯を変えたいときは、この表だけを直す。

## レビュー

- **各 AI が自分の独立レビュー**(Claude Code は critic、Codex は自分のレビュー役)を使う。他方の AI にレビューを依頼しない(ユーザー指示 2026-09-21)。
- 独立レビューのスキップや未実装ターゲットの正常終了を成功と数えない(CLAUDE.md)。

## ユーザーの確認が要ること(変更なし)

Xcode の署名・実機インストール、Codex / 外部サービスのログイン、`known_diffs.yaml` への追加(ADR 付き。人間の承認)、クラスタ・DB データの削除、
リポジトリの公開(public 化)、LICENSE の決定。

## 起動の目安

Claude Code のメインセッションは **Sonnet で起動**する(`claude --model sonnet ...`)。重い設計の判断のときだけ `/model opus` にして、終わったら `/model sonnet` に戻す。spec-writer・critic は engine・逆算・DB・API 契約に関わるときだけ Opus(`.claude/agents/*.md` の既定)、それ以外(文書・k8s・スクリプト・軽い修正)は `model: "sonnet"` で呼ぶ。利用枠が厳しいときは M1 のレーン(データ・API・Web)を優先し、他のレーンは区切りで止める(2026-09-22 ユーザー決定。Max の5時間の枠を6レーンで使い切らないため)。


```
cd ~/MyDamageCalcurater      && claude   # または codex(データレーン)
cd ~/MyDamageCalcurater-api  && claude   # または codex(API レーン)
cd ~/MyDamageCalcurater-web  && claude   # または codex(Web レーン)
cd ~/MyDamageCalcurater-tb   && claude   # または codex(タイプバランスレーン)
cd ~/MyDamageCalcurater-ios  && claude   # または codex(iOS レーン)
cd ~/MyDamageCalcurater-speed && claude  # または codex(素早さレーン)
cd ~/MyDamageCalcurater-judge && claude  # または codex(判定レーン)
```
