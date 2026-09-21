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

## 2026-09-21: TB0 タイプ相性データの取得元・契約を確認待ち
Decision: 未決。TB0 のタイプ相性表は、pokedex-svc の REST API または既存 MySQL 取り込みデータのエクスポートを
入力にする必要があるが、現時点では該当 API・importer・export データがリポジトリに存在しないため実装を停止する。
Reason: `api/openapi.yaml` の pokedex API は種族・技・持ち物・性格のみで、タイプ相性取得契約がない。
`services/pokedex/` は `.gitkeep` のみ、P2-2/P2-3 も未着手であり、取得形式・バージョン・生成手順を推測できない。
既存 `engine/typechart.go` の手書き表を複製することは、指定された取得経路にもマスタ正本の API 化にもならない。
Impact: `feat/codex-tb0-foundation` はブランチ作成のみで、`services/balance/` の実装には未着手。
再開には、(1) pokedex-svc に追加予定のタイプ相性 API 契約、または (2) コミット可能なエクスポートファイルの
配置先・スキーマ・データバージョン・再生成方法の指定が必要。既存 API への変更が必要なら Codex は実装せず担当側の判断を待つ。
