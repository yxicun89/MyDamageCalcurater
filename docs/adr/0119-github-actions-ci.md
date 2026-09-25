# ADR-0119: GitHub Actions で PR・main を機械的に検証する(issue #215)

- 状態: 採用
- 日付: 2026-09-25
- レーン: 運用(専用の ADR 帯が無いため、COORDINATION.md の帯表に合わせてデータ帯 `0100〜` の空きを使う)
- 関連: issue #215、docs/ai-shared/COORDINATION.md「main への統合」、docs/impl/make-targets.md

## 背景

`.github/` が存在せず、PR にも main への push にも何も自動実行されない。`gh pr merge` の直前にユーザーが
目を通す機会も無い(COORDINATION.md 2026-09-23)ため、テスト・lint・kustomize 描画・公開前検査は各セッションの
自己申告に頼っていた。実際に壊れた main がそのままマージされた事例がある(issue #215 所見1)。

## 決定

`.github/workflows/ci.yml` を追加し、PR と main への push で1ジョブ(`ci`、ubuntu-latest)を実行する。

1. **バージョンは二重管理しない**。`actions/setup-go` は `go-version-file: go.work`(全 Go モジュール共通の
   `go 1.27.1` を1箇所で宣言)、`actions/setup-node` は `node-version-file: web/.node-version`
   (`web/package.json` の `engines.node` と同じ値)を読む。
2. **`make test` → `make lint` → `make build` → `make test-golden` → `make test-wasm` の順に1ジョブで実行する**。
   `test`/`lint`/`build` は Web・`services/balance`・`services/speed`・`services/judge` のターゲットを
   ルート `Makefile` が前提条件として合成済み(`docs/impl/make-targets.md`「test/lint/build の合成結果」)。
   したがって「go ジョブ」「web ジョブ」に分けると Web 分がまるごと二重実行になり、private リポジトリの
   無料枠(月 2,000 分)を不必要に消費する。1ジョブにまとめることで二重実行を避け、Makefile の実際の挙動と
   ワークフローがずれる(drift)リスクも避ける(ワークフロー側で lint の中身を再実装しない)。
3. `kubectl` を `azure/setup-kubectl`(`v1.37.1` = `https://dl.k8s.io/release/stable.txt` の当時の安定版)で
   入れる。`make lint` の `k8s-render` に加え、`make api-kustomize` / `web-kustomize` / `balance-kustomize` /
   `speed-kustomize` / `judge-kustomize`(各レーン専用 overlay。base local/cloud/tidb だけの k8s-render では
   描画されない)も明示的に描画確認する。`kubectl` の追加は `services/gateway/deploytest` の
   `t.Skip("kubectl が無いので...")` / `t.Skip("curl が無いので...")` を 0 件にする効果も兼ねる
   (`curl` は ubuntu-latest に標準で入っている)。
4. `make check-publishable` を明示的な最後のステップとしても実行する(`make lint` に含まれるが、Actions の
   ログで単独の合否として見えるようにするため)。
5. すべて Secret 不要(`test-scripts` が検査するブートストラップ系スクリプトは偽の `curl`/`kubectl`/`helm` を
   使い、ネットワークをプロキシで塞いで検査する。`test-golden`/`test-wasm` は `testdata/golden/` コミット済み・
   `wasm` ビルドともにネットワーク非依存)。
6. 使う action はすべて公開タグのコミット SHA へ固定する(`actions/checkout` v7.0.1、`actions/setup-go` v7.0.0、
   `actions/setup-node` v7.0.0、`azure/setup-kubectl` v5.1.0。2026-09-25 時点の各リポジトリの最新安定版)。
   `permissions: contents: read` のみ。`concurrency` で同一 PR/ブランチの古い実行をキャンセルし、
   重複実行での無駄な分消費を避ける。

## 対象外(このADRでは扱わない)

- iOS(`make ios-test`): macOS ランナー必須で、GitHub-hosted の課金消費倍率(10倍)が大きい。署名・実機確認は
  人間の作業(CLAUDE.md「人間の確認が必要なこと」)。
- Playwright e2e・k3d に実際に apply するもの(`make e2e`・`*-k3d-deploy` 系・`make up`): k3d クラスタの
  起動が要り、GitHub-hosted ランナーだけでは完結しない。別途 issue #72 の範囲。
- `make gen` 後の差分ゼロ確認・`make test-db`(MySQL/TiDB 要): 今回のタスク範囲外(issue #223 等)。

## 副作用・トレードオフ

- 1ジョブ構成のため、go だけ/web だけの失敗でも実行時間はフルパス分かかる(ローカル実測: test 全体で
  数十秒、lint 数秒、build 数秒、golden 3秒、wasm 数秒。合計で無料枠に対して無視できる範囲)。
  ジョブを分けて並列化する高速化は、Web を二重実行しない形に Makefile 側を分割してから検討する。
- `make check-publishable` と各レーンの kustomize 描画はどちらも `make lint` と一部重複するが、実行コストは
  数秒未満で、ログの可読性(issue の受け入れ条件に明示された項目がそれぞれ単独ステップで緑/赤になる)を優先した。
