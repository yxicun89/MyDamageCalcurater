# docs 目次

- 作業の起点は [plan.md](plan.md)。要件の正は [requirements.md](requirements.md)、テストの正は [test-strategy.md](test-strategy.md)、画面の正は [design.md](design.md)(CLAUDE.md「最初に読むもの」)。
- `docs/impl/` の `path:行` は作成時点(基準コミットは各文書の冒頭)の行番号。以降の main の変更でずれうるので、関数名で探し直す。
- 「実装がどこにあり、動作確認で何が起きているか」を追う文書は [impl/](#実装解説impl) にまとめた。

## 実装解説(impl/)

| 文書 | 内容 |
|---|---|
| [impl/architecture.md](impl/architecture.md) | 全ディレクトリの責務、レイヤー、サービス間依存、ワークロード、認証ヘッダー、`/internal` API、マスタの扱い |
| [impl/request-flows.md](impl/request-flows.md) | calc / bulk / reverse / pokedex / Web 配信 / WASM / judge / balance・speed / importer の処理フロー(ファイル:関数付き) |
| [impl/api-endpoints.md](impl/api-endpoints.md) | 全ルート(公開契約・各サービス実ルート)、必須ヘッダー、ハンドラ位置、エラーコード |
| [impl/config-env.md](impl/config-env.md) | 環境変数・ConfigMap・Secret の定義側と参照側 |
| [impl/runbook-commands.md](impl/runbook-commands.md) | 手順書の全コマンドが裏で起動するもの(場所・接続先・副作用) |
| [impl/make-targets.md](impl/make-targets.md) | 全 make ターゲット(依存・実行内容・副作用) |
| [impl/k8s-local.md](impl/k8s-local.md) | k3d と Kustomize の全リソース、ポート対応、構成図、実クラスタとの差異 |
| [impl/db-mysql.md](impl/db-mysql.md) | MySQL の起動・migration・接続・import・テーブル一覧 |
| [impl/gitops-argocd.md](impl/gitops-argocd.md) | Argo CD / GitOps(監視対象・同期・未実装の範囲) |
| [impl/verify-mapping.md](impl/verify-mapping.md) | 動作確認の各項目・smoke の各チェックと実装の対応、エラー早見表、他レーンの検証一覧 |

## 要件・設計・テスト・運用

| 文書 | 内容 |
|---|---|
| [overview.md](overview.md) | プロジェクト概要 |
| [architecture.md](architecture.md) | アーキテクチャ全体図 |
| [requirements.md](requirements.md) | 要件の正 |
| [plan.md](plan.md) | 進行状況とタスク(作業の起点) |
| [test-strategy.md](test-strategy.md) | テスト戦略(層・ゴールデン・スモーク) |
| [design.md](design.md) | 画面・ビジュアルのデザインガイド |
| [coding-rules.md](coding-rules.md) | コーディング規約(公開できる状態・ハードコード禁止・読みやすさ) |
| [development-workflow.md](development-workflow.md) | Claude Code / Codex の開発ワークフロー対応 |
| [verify-m1.md](verify-m1.md) | M1 の動作確認手順(上から順に実行) |
| [type-balance-design.md](type-balance-design.md) | タイプバランスチェッカーの設計 |
| [type-balance-test-strategy.md](type-balance-test-strategy.md) | タイプバランスのテスト戦略 |
| [speed-design.md](speed-design.md) | 素早さ比較サービスの設計 |
| [judge-design.md](judge-design.md) | 判定サービス(素早さ×確定数)の設計 |
| [audit-r1.md](audit-r1.md) | R-1 コーディング規約違反の監査結果 |

## 手順書(runbooks/)

| 文書 | 内容 |
|---|---|
| [runbooks/api.md](runbooks/api.md) | API レーン(calc・gateway。ローカル k3d、`make dev`) |
| [runbooks/data.md](runbooks/data.md) | データレーン(マスタ投入・CronJob・排他の確認) |
| [runbooks/balance.md](runbooks/balance.md) | balance(ローカル k3d) |
| [runbooks/speed.md](runbooks/speed.md) | speed(ローカル k3d) |
| [runbooks/ios.md](runbooks/ios.md) | iOS(シミュレータ・モック) |
| [runbooks/ios-device-install.md](runbooks/ios-device-install.md) | iOS 実機インストール(Tailscale serve。人間の作業) |

## Claude Code / Codex 共有(ai-shared/)

| 文書 | 内容 |
|---|---|
| [ai-shared/README_AI_SHARED.md](ai-shared/README_AI_SHARED.md) | この共有ディレクトリの使い方 |
| [ai-shared/COORDINATION.md](ai-shared/COORDINATION.md) | レーン制・PR での統合・止まるときの作法 |
| [ai-shared/CURRENT_STATE.md](ai-shared/CURRENT_STATE.md) | レーンごとの現在状態(Active / Branch / Next) |
| [ai-shared/DECISIONS.md](ai-shared/DECISIONS.md) | 判断の記録(追記のみ) |
| [ai-shared/CLAUDE_LOG.md](ai-shared/CLAUDE_LOG.md) | Claude Code の作業ログ |
| [ai-shared/CODEX_LOG.md](ai-shared/CODEX_LOG.md) | Codex の作業ログ |
| [ai-shared/READ_ONLY_AUDIT_WORKFLOW.md](ai-shared/READ_ONLY_AUDIT_WORKFLOW.md) | 読み取り専用の継続監査ワークフロー |
| [ai-shared/claude-review.md](ai-shared/claude-review.md) | タイプバランス設計への Claude のレビュー |

## ADR(adr/)

番号帯: データ 0100〜 / API 0200〜 / Web 0300〜 / タイプバランス 0400〜 / iOS 0500〜 / 素早さ 0600〜 / 判定 0700〜(COORDINATION.md)。

| ADR | タイトル |
|---|---|
| [0001-tech-stack](adr/0001-tech-stack.md) | ADR-0001: 技術スタック |
| [0002-master-data-source](adr/0002-master-data-source.md) | ADR-0002: マスタデータ(使用可能ポケモン・技・持ち物・特性)の取得元 |
| [0003-kickoff-workflow-and-tooling](adr/0003-kickoff-workflow-and-tooling.md) | ADR-0003: キックオフ時のワークフロー適応と前提ツール |
| [0004-damage-rounding-and-order](adr/0004-damage-rounding-and-order.md) | ADR-0004: ダメージ計算の丸めと補正適用順 |
| [0005-modifier-scope-and-data-driven-effects](adr/0005-modifier-scope-and-data-driven-effects.md) | ADR-0005: 補正の範囲とデータ駆動の効果定義 |
| [0006-ko-probability-model](adr/0006-ko-probability-model.md) | ADR-0006: 確定数/乱数n発の確率モデル |
| [0007-codex-workflow](adr/0007-codex-workflow.md) | ADR-0007: Claude Code と Codex で共有する開発ワークフロー |
| [0008-golden-rounding-corrections](adr/0008-golden-rounding-corrections.md) | ADR-0008: 外部照合に基づく補正段階の訂正 |
| [0009-bulk-calc-presets](adr/0009-bulk-calc-presets.md) | ADR-0009: 一括計算の防御側プリセット定義と行の構成 |
| [0010-reverse-estimation](adr/0010-reverse-estimation.md) | ADR-0010: 逆算(調整推定)の探索空間・型の定義・一致度 |
| [0011-wasm-boundary](adr/0011-wasm-boundary.md) | ADR-0011: WASM 境界(JSON 文字列 API)とビルド・Go/WASM 一致テスト |
| [0012-domain-service-boundaries](adr/0012-domain-service-boundaries.md) | ADR-0012: damage-calc / balance のサービス境界と共通マスタ |
| [0013-type-chart-as-data](adr/0013-type-chart-as-data.md) | ADR-0013: タイプ相性表をデータ(マスタ)として engine に渡す |
| [0014-balance-tb1-defense-analysis](adr/0014-balance-tb1-defense-analysis.md) | ADR-0014: balance TB1 の防御タイプバランスの契約とポケモンタイプの取得 |
| [0015-balance-type-chart-from-data](adr/0015-balance-type-chart-from-data.md) | ADR-0015: balance の相性表を P1-13 のデータから読む |
| [0016-balance-tb2-offense-coverage](adr/0016-balance-tb2-offense-coverage.md) | ADR-0016: balance TB2 攻撃範囲の契約と技の取得 |
| [0017-balance-tb3-ability-effects](adr/0017-balance-tb3-ability-effects.md) | ADR-0017: balance TB3 特性による防御相性の変化 |
| [0018-balance-local-gitops-verification](adr/0018-balance-local-gitops-verification.md) | ADR-0018: balance の GitOps(Argo CD)をローカル k3d で検証する |
| [0100-pokedex-schema-and-migrate](adr/0100-pokedex-schema-and-migrate.md) | ADR-0100: pokedex のスキーマ(MySQL)・migrate・DB 行から engine 型への写像 |
| [0101-importer-fetch-convert-load](adr/0101-importer-fetch-convert-load.md) | ADR-0101: importer の取得・変換・投入(P2-2b) |
| [0102-data-lane-dependency-pins-2026-09](adr/0102-data-lane-dependency-pins-2026-09.md) | ADR-0102: データレーンの依存の最新安定版固定(2026-09)。MySQL は LTS 系列(9.7)を既定にする |
| [0103-importer-reconcile-report-and-pins](adr/0103-importer-reconcile-report-and-pins.md) | ADR-0103: importer の照合と差分報告・取得元の版の固定(P2-2c) |
| [0104-importer-cronjob-and-make-import](adr/0104-importer-cronjob-and-make-import.md) | ADR-0104: importer の CronJob(週1回)と make import の運用(P2-2d) |
| [0105-pokedex-svc-internal-api-export-natures](adr/0105-pokedex-svc-internal-api-export-natures.md) | ADR-0105: pokedex-svc(検索 API・内部 API・natures・pokedex export・k8s) |
| [0106-ability-type-immunity-and-absorption](adr/0106-ability-type-immunity-and-absorption.md) | ADR-0106: 特性によるタイプの無効・吸収を効果定義に足す(P2-3b) |
| [0107-move-secondary-rank-changes](adr/0107-move-secondary-rank-changes.md) | ADR-0107: 技の追加効果(命中時のランク変化)をデータとして持つ |
| [0108-engine-request-limits](adr/0108-engine-request-limits.md) | ADR-0108: engine / engine/wasmapi の候補・観測件数の上限(issue #110 追従) |
| [0109-importer-mutual-exclusion](adr/0109-importer-mutual-exclusion.md) | ADR-0109: importer の手動実行と CronJob の相互排他(issue #106) |
| [0200-calc-svc-api-contract](adr/0200-calc-svc-api-contract.md) | ADR-0200: calc-svc の API 契約とマスタ境界 |
| [0201-api-deps-latest-echo-v5](adr/0201-api-deps-latest-echo-v5.md) | ADR-0201: API レーンの依存を最新の安定版へ(Echo v5・oapi-codegen の echo5-server) |
| [0202-gateway-routing-and-headers](adr/0202-gateway-routing-and-headers.md) | ADR-0202: gateway のルーティングとヘッダ検証 |
| [0203-api-k3d-deploy-and-smoke](adr/0203-api-k3d-deploy-and-smoke.md) | ADR-0203: calc・gateway の k3d デプロイとスモーク |
| [0204-calc-master-from-pokedex-internal-api](adr/0204-calc-master-from-pokedex-internal-api.md) | ADR-0204: calc-svc のマスタを pokedex-svc の内部 API から受け取る |
| [0205-gateway-web-upstream](adr/0205-gateway-web-upstream.md) | ADR-0205: gateway が Web の静的配信を後ろに置く |
| [0206-wire-to-pokedex-svc](adr/0206-wire-to-pokedex-svc.md) | ADR-0206: calc・gateway を pokedex-svc につなぐ |
| [0208-calc-request-limits](adr/0208-calc-request-limits.md) | ADR-0208: calc の候補・観測件数の上限 |
| [0209-record-team-data-retention](adr/0209-record-team-data-retention.md) | ADR-0209: M2 保存データの保持・削除・端末 ID 境界 |
| [0210-private-service-boundary](adr/0210-private-service-boundary.md) | ADR-0210: 私設サービスの境界(issue #148) |
| [0300-web-architecture](adr/0300-web-architecture.md) | ADR-0300: Web(P4)の構成 — WASM 先行・計算の差し替え口・架空マスタ・攻撃側プリセット |
| [0301-web-api-wasm-switch](adr/0301-web-api-wasm-switch.md) | ADR-0301: Web の API / WASM 切り替え(P4-5)— 実体 → ID の写像・生成型・モード・端末 ID |
| [0302-web-container](adr/0302-web-container.md) | ADR-0302: Web をコンテナで配信する(nginx・k3d・gateway の後ろ) |
| [0303-web-balance-screen](adr/0303-web-balance-screen.md) | ADR-0303: タイプバランスの画面(P4-12)— balance API をそのまま使う・例データの ID を揃える |
| [0304-web-online-mastersource](adr/0304-web-online-mastersource.md) | ADR-0304: Web のオンライン MasterSource — 検索ベースの選択 UI と、技の ID 解決の API ギャップ |
| [0305-web-build-assets-dir](adr/0305-web-build-assets-dir.md) | ADR-0305: Web のビルド成果物を `/static/` に出す(gateway の予約パス `/assets/` との衝突を解消) |
| [0400-balance-tb4-threat-check](adr/0400-balance-tb4-threat-check.md) | ADR-0400: balance TB4 仮想敵診断 |
| [0401-balance-tb5-recommend-types](adr/0401-balance-tb5-recommend-types.md) | ADR-0401: balance TB5 おすすめタイプと該当ポケモン |
| [0402-balance-read-model-json-schema](adr/0402-balance-read-model-json-schema.md) | ADR-0402: balance の read model の形を JSON Schema で公開する |
| [0403-balance-readmodel-wiring](adr/0403-balance-readmodel-wiring.md) | ADR-0403: pokedex export の read model を k3d の balance に読ませる |
| [0404-balance-tb6-move-range-checker](adr/0404-balance-tb6-move-range-checker.md) | ADR-0404: balance TB6 技範囲チェッカー |
| [0405-argocd-bootstrap-hash-and-digest-pin](adr/0405-argocd-bootstrap-hash-and-digest-pin.md) | ADR-0405: Argo CD 導入物(install manifest・同梱イメージ)をハッシュと digest で固定する |
| [0500-ios-app-architecture](adr/0500-ios-app-architecture.md) | ADR-0500: iOS アプリの構成(パッケージ分割・API 生成・モック・テスト) |
| [0501-ios-screen-acceptance](adr/0501-ios-screen-acceptance.md) | ADR-0501: iOS の画面ごとの受け入れ条件と判断 |
| [0600-speed-sp0-foundation](adr/0600-speed-sp0-foundation.md) | ADR-0600: 素早さ比較 SP0 基盤 |
| [0601-speed-sp1-table](adr/0601-speed-sp1-table.md) | ADR-0601: 素早さ比較 SP1 素早さの表 |
| [0602-speed-sp2-position](adr/0602-speed-sp2-position.md) | ADR-0602: 素早さ比較 SP2 自分のポケモンの位置 |
| [0603-speed-sp4-readmodel-wiring](adr/0603-speed-sp4-readmodel-wiring.md) | ADR-0603: pokedex export の read model を k3d の speed に読ませる |
| [0604-speed-sp3-web-screen](adr/0604-speed-sp3-web-screen.md) | ADR-0604: 素早さ比較 SP3 Web 画面 |
| [0605-speed-sp5-gitops](adr/0605-speed-sp5-gitops.md) | ADR-0605: 素早さ比較 SP5 GitOps |
| [0700-judge-jd0-foundation](adr/0700-judge-jd0-foundation.md) | ADR-0700: 判定(素早さ×ダメージ連動)JD0 基盤 |
| [0701-judge-jd1-outspeed-and-ko](adr/0701-judge-jd1-outspeed-and-ko.md) | ADR-0701: 判定(素早さ×ダメージ連動)JD1 抜けるか+倒せるか |
| [0702-judge-jd2-field-effects](adr/0702-judge-jd2-field-effects.md) | ADR-0702: 判定(素早さ×ダメージ連動)JD2 場の効果(トリックルーム・追い風) |
| [0703-judge-jd3-multiple-candidates](adr/0703-judge-jd3-multiple-candidates.md) | ADR-0703: 判定(素早さ×ダメージ連動)JD3 複数の相手候補を一度に判定 |
