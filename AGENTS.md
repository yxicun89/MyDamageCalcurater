# AGENTS.md — Claude Code / Codex 共通の開発運用

まず `CLAUDE.md` を読む。プロジェクトの絶対ルール・ドメイン規約・技術規約の正は
`CLAUDE.md`、Git・役割分担・引き継ぎの共通運用はこのファイルとする。
続いて `docs/plan.md`、`docs/requirements.md`、`docs/test-strategy.md`、
`docs/design.md`、関連する `docs/adr/`、コードを書く・直す・レビューするときは `docs/coding-rules.md`(共通のコーディング規約)を読む。
手順と Claude → Codex の対応は `docs/development-workflow.md` を参照。

## 共有状態(docs/ai-shared/)

Claude Code と Codex は記憶を共有しない。共有記憶は `docs/ai-shared/` だけ。
**作業は AI ではなく「レーン」(ダメージ計算 / タイプバランス)に属する**。どちらの AI がどのレーンを進めてもよく、
同じレーンは同時に1セッションだけ。レーン・ディレクトリ・ブランチ・PR での統合・止まるときの作法は
`docs/ai-shared/COORDINATION.md` を正とする(2026-09-21 ユーザー決定)。

1. 作業開始時: `git fetch origin` し、`origin/main` の `docs/ai-shared/CURRENT_STATE.md` と `DECISIONS.md` を読む(`DECISIONS.md` は巨大なので見出しから自レーンと直近だけ。COORDINATION.md「コンテキストを膨らませない」)。
   続きはレーン欄の `Branch` の最新コミットと `Next` から始める
2. 作業終了時: 自分のログ(`CLAUDE_LOG.md` / `CODEX_LOG.md`)に追記し、進めたレーンの欄(`Status`・`Next`・`Active`)を更新して commit・push する

## 開始時と Git 運用

- 最初に `git status --short --branch` を確認し、現在のブランチ、`git diff`、
  `git diff --cached`、未追跡ファイル、最近の `git log` を調べる。
- **main に直接実装しない。** main 上なら既存変更を保持したまま
  「Git ブランチ運用」の命名で作業ブランチを作る(例 `git switch -c feat/claude-<phase名>`)。
  既存の適切な作業ブランチなら継続する。保護領域の書き込み承認が必要なら正式な承認手順を使う。
- 既存の未コミット・未追跡変更を勝手に破棄しない。
  `reset --hard`、`clean`、`checkout --`、`restore` 等で上書き・削除しない。
  引き継いだ差分と今回の変更を区別して記録する。
- 実装前に既存コード・仕様・ADR・テストを照合する。前任 AI の完了記録や変更も検証する。
- 依頼された範囲の最小変更に留め、無関係な整形、依存更新、大規模リファクタリングを混ぜない。
  レビューのみの依頼では変更しない。
- コミットはメインエージェントが担当する。1タスク = 1コミットを基本とし、先に
  `docs/plan.md` を更新する。直前に `git diff` と `git diff --cached` を確認し、
  対象ファイルを明示して stage する。無関係な既存変更を一括で取り込まない。
  作業ブランチへの push は区切りごとに行う。main へは PR 経由でのみ入れる(直接 push・直接 merge をしない。COORDINATION.md)。

## Git ブランチ運用

- ブランチはレーン単位で切る(AI 単位ではない)。ダメージ計算: `feat/calc-<phase名>`、タイプバランス: `feat/tb-<stage名>`、
  単発の修正: `fix/<レーン>-...`。既存の `feat/claude-p1-engine` / `feat/codex-tb0-foundation` はマージまでそのまま使う
- 1つのブランチに複数の Phase/ステージ分の作業を積み上げない。Phase/ステージが終わったら PR でマージしてブランチを削除する
- レートリミットや上限で一方の AI が止まったら、**もう一方の AI が同じレーンのブランチの `Next` から続けてよい**
  (前任の記録はうのみにせず検証する)。引き継ぎ資料は作らない
- 同じレーンを2つのセッションで同時に進めない。別レーンの作業は、PR で main に入ってから `git merge origin/main` で取り込む

## 共有ファイルの編集規約

担当ディレクトリを分けても、次の5つは両方が触る可能性があり、コンフリクトの原因になる。
以下の規約で編集する。**main への統合は、各 AI が自分のブランチを自分で行う**(2026-09-21 改訂。
マージコーディネーターは廃止。手順・条件・止まるときの作法は `docs/ai-shared/COORDINATION.md` を正とする)。

1. `docs/ai-shared/CURRENT_STATE.md`
   - 自分が進めているレーンの欄(`## Damage Calculator` / `## Type Balance Checker`)だけを編集する。
     他のレーン欄は読むだけ。コンフリクトが起きても、該当欄を残すだけで解決できる。
2. `docs/ai-shared/DECISIONS.md`
   - 追記のみ。既存エントリは編集しない。ファイル末尾に新エントリを足す。
3. `go.work`(Go ワークスペース)
   - タイプバランスレーンは `services/balance/go.mod` を作成し、`go.work` の `use` に `./services/balance` を追記してよい(その PR に含める)。
4. ルートの `Makefile`
   - タイプバランスレーンは balance 用のターゲットをルートの `Makefile` に直接書かない。`services/balance/Makefile` を作る。
     ルートの `Makefile` からは `include services/balance/Makefile` の1行だけで取り込み、その1行は追記してよい(その PR に含める)。
   - include されたレシピはルートから実行される。ターゲット名は `balance-` 接頭辞にして既存ターゲットと衝突させず、
     パスは `services/balance/` 起点で書く(または `cd services/balance &&` を付ける)。
5. `AGENTS.md` 自体 / `CLAUDE.md` 自体
   - 運用ルールの変更はユーザーの決定があったときだけ行い、`DECISIONS.md` に記録する。全体の書き直しはしない。
   - 「タイプバランスレーンの範囲」節は、そのレーンを進める AI が設計に合わせて更新してよい。

## 変更・レビューで守ること

`CLAUDE.md` の全規約を適用する。特に次を最優先で確認する。

- API は `api/openapi.yaml` を先に変更して `make gen`。生成型を手書きしない。
- engine に DB・HTTP・ファイル等の I/O や外部依存を導入しない。
- 4096 基準の固定小数、五捨五超入、補正と丸めの順序を守る。float で近似しない。
- SP は各 0〜32・合計 66 以下。ゴールデン照合時の EV は `max(0, 8×SP−4)`。
  SP 0/32/66、HP 1、ランク ±6、無効相性、逆算の候補・探索漏れを確認する。
- サービスは自分の DB だけにアクセスする。record/team/TiDB/NATS 障害で計算を止めない。
- テストを削除・弱体化して通さない。期待値を変更するなら外部実装・仕様による根拠を示し、
  コミット時にも理由を書く。known_diffs 追加は ADR と人間の承認を要する。
- 設計判断は ADR に残す。レビュー指摘は「重大 / 重要 / 軽微」に分け、
  ファイル・行・再現条件・影響を示す。検証できなかった点を合格としない。

## 手順書の書き方(全レーン共通。ユーザー決定 2026-09-22)

人が実行する手順書(`docs/verify-*.md`・README の起動手順・各サービスの README など)は次のとおりに書く。

1. **上から下へ1回読めば終わる**。前の節や別の節への「〜を参照」「§2 と同じ」で行き来させない。必要なら同じコマンドをもう一度書く。
2. **動作を伴うコマンドと、必要最低限の確認点だけ**を書く。行動を伴わない説明(背景・設計の理由・内部の仕組み)は書かず、ADR や README の設計の節に置く。
3. **コマンドの塊は必ずリポジトリのルートへの移動から始める**(`cd "$(git rev-parse --show-toplevel)"`)。make の実行場所で迷わせない。
4. **確認点は「何を見れば成功か」を1行で**書く(例: `http://localhost:8080` で計算結果が5行出る)。
5. ローカルで動かす手順は、ホストで直接より **k3d(コンテナ)で動かす方を主**にする(k8s の構成をそのまま確かめるため)。ホストで直接動かす手順は開発用として後ろに置く。

## エージェントの使い分けとコスト

- メインエージェントが対象タスク、受け入れ条件、変更範囲、依存順を決め、結果を統合する。
- 単純な検索・調査・軽微な修正はメインだけで実施する。
  独立した調査やレビューに効果がある場合はサブエージェントを並列利用する。
  通常は同時に最大 2 子、子からの再委譲はしない。役割があるだけで全員を毎回起動しない。
- Codex の役割は `.codex/agents/` の `scanner`、`spec_writer`、`implementer`、
  `reviewer`、`verifier`。設計担当は必要時に `spec_writer` が兼ねる。
  Claude の既存 `.claude/agents/` と skills は引き続き使用できる。
- 調査・仕様設計・レビューは読み取り専用。ソースの編集はメインまたは割り当てられた
  implementer だけが指定範囲に行う。同じファイルを複数担当が同時編集しない。
- engine・逆算・DB 設計・API 契約の変更は、実装者と別の reviewer に独立レビューを依頼する。
  実行できない場合は未実施と記録し、レビュー完了とは扱わない。
- モデル ID をプロジェクトで固定しない。利用可能な通常モデルを継承する。
  単純調査は低推論、通常の仕様・実装・レビュー・検証は標準(medium)を基本とする。
  複雑な原因究明・設計判断などで高推論が必要な場合だけ、作業前に理由を短く説明する。
  実際に設定変更できない環境で「切り替えた」とは報告しない。

## タイプバランスレーンの範囲(タイプバランスチェッカー)

旧「Codex の実装担当範囲」。2026-09-21 のユーザー決定で、**このレーンは Claude Code・Codex のどちらが進めてもよい**
(レーン制。COORDINATION.md)。以下の範囲・規約は、進める AI によらず同じ。

### 範囲(厳守)

- **範囲**: `services/balance/`(タイプバランスチェッカー)とその Deployment/Service/Kustomize
- **範囲外・変更禁止**(ダメージ計算レーンの範囲): `engine/`, `services/pokedex/`, `services/calc/`, `services/record/`, `services/team/`, `web/`, `ios/`, `api/openapi.yaml` の damage 関連エンドポイント
  - pokedex-svc は実装やスキーマを変更しない。ADR-0012 により balance の必須ランタイム依存にもしない
  - 変更が必要だと思ったら実装せず `docs/ai-shared/DECISIONS.md` に提案を書いて止まる
- このレーンのセッションで、ダメージ計算レーンの未完了タスクを実装しない(ダメージ計算レーンは別セッションが進める)

### 最初に読むもの(この順で)

1. `docs/ai-shared/CURRENT_STATE.md`
2. `docs/ai-shared/DECISIONS.md`
3. `docs/type-balance-design.md`(設計書。実装の唯一の起点)
4. `docs/ai-shared/claude-review.md`(Claude によるレビューと修正指摘)
5. 必要なときだけ `docs/ai-shared/CLAUDE_LOG.md`

### 規約

- `services/balance/internal/balance/` は純粋 Go(HTTP・DB・Kubernetes に依存しない)
- 倍率は float ではなく整数表現(claude-review.md 参照)
- TB1 の時点から `EffectSource`(タイプ由来/特性由来)を型に持たせる
- 共通マスタの恒久正本は1つとし、balance 独自の正本や DB 直結を作らない。TB0 は ADR-0012 の
  temporary adapter を provider 境界の後ろで使い、正式マスタ確定後に adapter だけを差し替える
- balance の API 契約は `services/balance/api/openapi.yaml` を正とし、生成型を手書きしない。
  ルート `api/openapi.yaml` は既存 damage/gateway 契約の正として Codex は変更しない
- 認証なし。pokecalc と同じ端末ID/セッションIDの流儀に合わせる
- manifest は Kustomize(`services/balance/deploy/k8s/base` + `overlays/local`)
- Argo CD Application は balance 専用に分ける。Sync は最初 manual

### 完了条件

- ユニットテストが通ること(`go test ./services/balance/...`)
- k3d 上で `/api/balance/v1/team-balance/analyze` が疎通すること
- セッション終了時に自分のログ(`CLAUDE_LOG.md` / `CODEX_LOG.md`)と `CURRENT_STATE.md` の `## Type Balance Checker` 欄を更新すること

## 検証と終了時

- 変更に応じて test / lint / build を実行する。利用可能なターゲットは Makefile を確認する。
  最低限 `make test`、計算変更は `make test-golden`、実数値・網羅性の変更は
  `make test-all-species`。API 変更は生成差分も確認する。
- Go の変更ファイルを `gofmt` し、静的検査は `go vet`、ビルドは `go build` を各 Go モジュールで行う。
  `make lint` / `make build` が定義されていればそれを使う。
  Web 等は package.json の scripts が実装されている範囲で型検査・lint・test・build を実行する。
- コマンドの終了コードだけで合格としない。対象テスト 0 件、`[no tests to run]`、
  未実装の echo ターゲット、レビューの skip は「未実施 / 未実装」と記録する。
- 同じ失敗で 3 回修正を繰り返しても進まない場合、原因・試行・依存を
  `docs/plan.md` のブロッカーに残し、依存しない許可済みタスクを進める。
- 終了時は `docs/plan.md` に実施内容・検証結果・残作業・既知の問題・次の開始点を残す。
  詳細は `docs/ai-shared/` のログ・状態に書き、別途の引き継ぎ資料は作らない。最終報告に変更点、
  ブランチ、実行した検証と未実施理由を示す。
