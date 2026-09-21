# Decisions

## 2026-09-21: ダメージ計算とタイプバランスは同一クラスタ・別サービス
Decision: 1クラスタ上で別Deployment/Service。Ingressで `/api/damage` と `/api/balance` を分離。
Reason: 独立スケール・独立ロールバックの学習目的。
Impact: balance-svc も Docker化。共通クラスタ仕様は本ファイルで共有。

## 2026-09-21: Argo CD を共通デプロイ基盤にする
Decision: GitOpsを採用、Git上のKustomize定義を正本にする。Application はサービスごとに分割。Sync は最初 manual。
Reason: 変更履歴とクラスタ状態を一致させる。2サービス同時 selfHeal のリスクを避ける。
Impact: damage/balance 両方が Argo CD 管理対象。安定後に自動 sync を検討。

## 2026-09-21: balance-svc は pokedex-svc の API を呼ぶ(DB直結・Goモジュール共有はしない)
Decision: マスタデータ取得は pokedex-svc の REST API 経由。型定義の共有モジュール化は当面しない。
Reason: DBスキーマ変更やモジュール変更で相手のビルド・デプロイが壊れることを防ぐ。
Impact: balance-svc は独自DBを持たない(TB1時点)。将来必要なら DECISIONS.md に追記して再検討。

## 2026-09-21: manifest は Kustomize に統一
Decision: plain YAML / Helm ではなく Kustomize(pokecalc と同じ)。
Reason: 個人開発の規模でHelmのテンプレート化は過剰。pokecalcと構成を揃える。
Impact: services/balance/deploy/k8s も base/overlays 構成にする。

## 2026-09-21: fix/codex-workflow-golden は保留(main へマージしない)【撤回済み: 下の「main へマージして各自 main から作業する」を参照】
Decision: Claude Code が内容を確認したが、実装の続行もマージもしない。ブランチは 6d86382 として保全し、そのまま残す。
Reason: engine のダメージ計算コア(damage.go / modifiers.go / model.go の丸め順・補正段階の訂正)と
API 契約(api/openapi.yaml の level を 50 固定)に踏み込んでおり、「ルール違反や設計変更を含まないか」を
明確に判定できない。CLAUDE.md 絶対ルール3(golden 全件一致・known_diffs の人間承認)と AGENTS.md
(engine 変更は実装者と別の reviewer による独立レビュー)の確認が済んでいない。
なお保全時点で make test / make test-golden / make lint は成功しており、内容自体が壊れているわけではない。
Impact: 元の担当(Codex)が次にこのブランチの内容へ着手する際に新規タスクとして再検討する。
その際 AGENTS.md / CLAUDE.md は main 側が先に変更されているため、マージ時に統合が必要
(ブランチ側は共通ワークフロー版の書き換え、main 側はブランチ運用ルールと ai-shared 参照の追記)。
引き継ぎ資料は作らない。ブランチ内の ADR-0007 / ADR-0008 / docs/development-workflow.md は再検討時に参照する。

## 2026-09-21: Git ブランチ運用ルールを導入する
Decision: Claude Code は `feat/claude-<phase名>`、Codex は `feat/codex-<stage名>`、単発修正は `fix/claude-...` /
`fix/codex-...`。Phase/ステージ単位で切り、Claude はその Phase の /verify 通過後、Codex はそのステージの
テスト全件通過後に main へマージしてブランチを削除する。単発修正はそのセッション内でマージまで完了させる。
1ブランチに複数 Phase/ステージ分を積み上げない。
Reason: fix/codex-workflow-golden がレートリミットで宙に浮いたため。以後、詰まったブランチは相手に引き継がず、
保留として記録するか、ルール違反がなければもう片方が完了させる。
Impact: AGENTS.md「Git ブランチ運用」に本文、CLAUDE.md に要約を追記。Codex の TB0 は
feat/codex-tb0-foundation で新規に開始する(旧名 feat/type-balance-tb0 は使わない)。

## 2026-09-21: fix/codex-workflow-golden を main へマージし、以後は各自 main から作業する
Decision: 保留を撤回し、ユーザー判断で fix/codex-workflow-golden を main へ --no-ff マージ(ブランチは削除)。
Claude Code と Codex の作業を1つのブランチで混ぜない。相手の作業を使いたいときは、相手のブランチを一度 main に
マージしてから、各自が main から自分のブランチを切って作業する(AGENTS.md「Git ブランチ運用」に追記)。
Reason: 保留のまま Claude が続きの作業を積むと Codex の未レビュー作業と混ざる。一度 main に確定させて起点をそろえる方が
安全というユーザー判断。マージ後の main で make test / make test-golden(キャッシュ無し) / make lint / make build は成功。
Impact: AGENTS.md / CLAUDE.md の衝突は、Codex の共通ワークフロー(開始時と Git 運用・エージェントの使い分け・検証と終了時)と
main 側のブランチ運用・ai-shared 参照・Type Balance 担当範囲を両方残して統合。Codex 側の「feature/<内容> 等」は
新命名(feat/claude-... / feat/codex-...)に、「引き継ぎ文書に分ける」は「docs/ai-shared に書き、別途の引き継ぎ資料は作らない」に直した。
未確認: engine の丸め順訂正(ADR-0008)と openapi.yaml の level 固定は、実装者と別の reviewer による独立レビューを
まだ受けていない。P1-6 は [~] のまま、Claude Code 側で critic レビューを通してから [x] にする。

## 2026-09-21: Codex ブランチの main 取り込みはマージコーディネーター(Claude Code)が行う
Decision: Codex の feat/codex-* を main に取り込むのは Claude Code。Codex が完了を報告し、ユーザーが取り込みを指示したときだけ実施する。
共有ファイル5点(CURRENT_STATE.md=自セクションのみ / DECISIONS.md=追記のみ / go.work=Codex は追記しない /
ルート Makefile=Codex は services/balance/Makefile を作り include の1行は取り込み時に追記 / AGENTS.md・CLAUDE.md=自セクションのみ)の
編集規約を AGENTS.md に、取り込み手順を CLAUDE.md に定めた。
Reason: 担当ディレクトリを分けても共有ファイルでコンフリクトが起きるため。原因を規約で先に潰し、
それでも起きたコンフリクトは異常のサインとして自動解決せず報告する。
Impact: Codex は go.work とルート Makefile を編集しない。取り込み時のテスト確認は docs/type-balance-test-strategy.md を基準にするが、
この文書は未作成(Codex/ユーザーによる作成待ち)。作成されるまで Claude は取り込みを実行せず報告する。

## 2026-09-21: 共有状態は main 専用 worktree の docs/ai-shared を正本にする
Decision: 新しい coordination branch は作らない。`main` 専用 worktree
(`~/pokecalc-main`)の `docs/ai-shared/` だけを現在状態の正本とし、feature branch 内の
同名ファイルは履歴上のスナップショットとして扱う。Claude/Codex は個別 worktree で実装する。
Reason: 既存の共有 MD とブランチ運用を維持しつつ、feature branch ごとの CURRENT_STATE 分岐と、
1 worktree のブランチ切り替えによる相互干渉を解消するため。
Impact: 共有 MD のみ main へ直接コミットしてよい。実装は従来どおり feature branch のみ。
共有 MD の同期だけを目的とする merge/cherry-pick は行わない。詳細は README_AI_SHARED.md。

## 2026-09-21: damage-calc と balance は兄弟のドメインモノリスとする
Decision: 「1機能ドメイン=1サービス、サービス内部はモノリス」とし、damage-calc と balance の
相互 API 依存を作らない。pokemon/type/move/ability/damage-formula 単位の新サービスは作らない。
Reason: Kubernetes を理由に責務を細分化せず、個人開発で管理可能な複雑さを保つため。
Impact: 既存の未実装 pokedex-svc を balance の必須ランタイム依存にはしない。既存文書の
「balance-svc は pokedex-svc API を呼ぶ」は本決定で置き換える。詳細は ADR-0012。

## 2026-09-21: TB0 のタイプ相性表は差し替え可能な temporary adapter とする
Decision: 共通マスタの恒久正本が未確定の間、現行18タイプ相性を balance 内の temporary/static adapter として
利用してよい。ただし純粋コアは provider interface に依存し、正式な共通スナップショット確定後に差し替える。
Reason: Claude 側 P2-1 の ADR-0002 は共通マスタのコミット済みスナップショットを提案しているが人間確認待ち。
一方、タイプ相性コア・HTTP・Kubernetes 基盤はその確定を待たずに検証できる。
Impact: 前エントリ「TB0 タイプ相性データの取得元・契約を確認待ち」の停止条件は解除する。
temporary データを balance 独自の恒久正本として扱わず、API や新サービスを先行追加しない。

## 2026-09-21: balance の API 契約はサービスローカル OpenAPI を正とする
Decision: balance の契約は `services/balance/api/openapi.yaml` を正とし、oapi-codegen で型を生成する。
ルート `api/openapi.yaml` は damage/gateway の契約として Codex は変更しない。
Reason: damage-calc と balance を兄弟のドメインモノリスとして分離しつつ、仕様先行と生成型の規律を維持するため。
Impact: CLAUDE.md の「API はルート OpenAPI が唯一の正」は damage/gateway の範囲に限定して読み替える。
balance の契約変更はサービス内 spec → 生成 → テストの順で行う。詳細は ADR-0012。

## 2026-09-21: 共通マスタ候補 ADR-0002 は Claude feature branch 上の提案として参照する
Decision: ADR-0012 と CURRENT_STATE が参照する ADR-0002 は `feat/claude-p1-engine` 上にあり、main へは未統合であることを明記する。
Reason: main の共有状態を正本にした時点で、ブランチ指定のない ADR-0002 参照が main 上では辿れなかったため。
Impact: 共通マスタ方式は確定扱いにしない。Claude ブランチが通常手順で main に統合された後は main の ADR-0002 を参照する。

## 2026-09-21: 公開用クリーンコピーの作成担当をClaude Codeへ固定しない
Decision: 現在のrepositoryとworktreeは変更せず、公開用に調整したクリーンコピーを別directory・別repositoryとして作る。
作成担当はClaude Code/Codexのどちらかへ固定せず、ユーザーから依頼された側が行う。元repositoryへ公開用remoteを
追加せず、作者情報・個人accountを含むpath・秘密・第三者データ等の公開前検査が完了したコピーだけをprivate remoteへ
接続する。credentialやtoken、個人accountを含むremote URLは文書へ記録しない。
Reason: 公開準備を特定AIのfeature branchだけに置くと、次に作業するAIがクリーンコピーの場所・基点・検証状態を
把握できず、古いrepositoryで作業を再開したり、未消毒の履歴をpushしたりする危険があるため。
Impact: クリーンコピーを作成・更新した担当は、自分のfeature branchだけで完了を記録してはならない。元repositoryの
main正本`docs/ai-shared/CURRENT_STATE.md`の担当欄と自分のlogへ、秘密を含まない相対path、source commit、clean copyの
branch/commit、sanitization方式、公開前検査結果、remote設定/push状態を記録する。切替時はclean copy側の
`docs/ai-shared/`も更新し、以後の開発正本を明記する。元repository側は新正本へのpointerとして残し、以後そこで実装しない。

## 2026-09-21: 開発の正本を private のクリーンコピーへ移し、Claude と Codex の協調運用を「互いを待たない」形に改める(ユーザー指示)
Decision: (1) 履歴を消毒(作者を `pokecalc-dev <noreply@example.com>` に統一、個人用の手順書を全履歴から除去、module path とホームの絶対パスを置換)したクリーンコピーを
`~/MyDamageCalcurater` に作り、private の GitHub リポジトリ(origin)へ push した(`main` / `feat/claude-p1-engine` / `feat/codex-tb0-foundation`)。以後の開発の正本は origin の main。
旧ディレクトリはアーカイブ。(2) 互いのレートリミットで作業が止まらないよう、docs/ai-shared/COORDINATION.md を新設した: 各 AI が自分のブランチの統合まで単独で完了できる
(マージコーディネーター Claude Code の廃止、Codex の go.work / Makefile 追記を許可)、止まる前に WIP を commit・push して `Next` を書く、相手を待たず既定値付きの提案で進める、
レビューは各 AI の自己完結(相手に依頼しない)、ADR 番号は統合時に後から統合する側が振り直す。
Reason: ユーザーが「お互いのレートリミットでお互いの作業が止まるのが問題。先に調整して、それぞれレートリミットに関係なく作業できる状態にしたい」と依頼した。
Impact: AGENTS.md の共有ファイル編集規約と CLAUDE.md の「Codexブランチの取り込み手順」を更新(後者は廃止)。Claude 側の type-chart ADR は、Codex の ADR-0012(domain-service-boundaries)との衝突を避けて 0013 に振り直した。
Codex は次のセッションで COORDINATION.md を確認し、異議・修正があれば DECISIONS.md に追記する。それまでは本文の内容で進めてよい(既定案で進む原則)。

