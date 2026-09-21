# balance service

パーティのタイプ相性・弱点・耐性・攻撃範囲を分析するドメインモノリス。
damage-calc とは兄弟サービスで、互いの実行時 API には依存しない。

## TB0 の範囲

- `internal/balance`: HTTP・DB・Kubernetes に依存しない型と防御相性コア
- `api/openapi.yaml`: balance 外部 API 契約の正
- `internal/api`: oapi-codegen による生成型
- `internal/master`: 共通マスタへ差し替えるための adapter
- `internal/httpapi`: health と TB1 用 endpoint の最小疎通
- `cmd/api`: プロセス起動と graceful shutdown
- `deploy`: balance 専用 Kustomize と manual-sync の Argo CD Application
- `DEPENDENCIES.md`: 公開前確認用の直接依存・利用理由・license

`TemporaryTypeChart` は開発継続用であり恒久正本ではない。2026-09-21 時点の
`engine/typechart.go` と同じ第6世代以降の18タイプ相性を複製し、ADR-0012 の共通マスタが
確定したら `balance.TypeChartProvider` の adapter だけを差し替える。
複製元は main の commit `cc9a816` 時点の `engine/typechart.go`(最終変更 `0e4616a`)、SHA-256 は
`cc41f76f82b0c79818ea784436c6a760bd4a8699a0d2596855526b7fa634edf5`。

## TB0 HTTP 契約

- `GET /healthz`: Pod probe。200 `{"status":"ok"}`
- `GET /api/balance/healthz`: Ingress 経由の smoke。200
- `POST /api/balance/v1/team-balance/analyze`: `X-Device-Id` と `X-Session-Id` が必須
  - ヘッダー欠落: 400 `missing_request_context`
  - request は1〜6件の `{ "pokemonId": "NNNN-NNN" }`。不正なら400 `invalid_request`
  - 16 KiB を超える request: 413 `request_too_large`
  - 有効な TB0 疎通: 501 `tb1_not_implemented`

分析の成功 response は TB1 でテストと契約を先に確定する。TB0 は未実装を200で偽装しない。
HTTP の path・必須 header・handler interface は service-local OpenAPI から生成し、実装を
`api.ServerInterface` へコンパイル時に適合させる。

## ローカル検証

リポジトリルートから実行する。

```sh
make -f services/balance/Makefile balance-gen
make -f services/balance/Makefile balance-test
make -f services/balance/Makefile balance-lint
make -f services/balance/Makefile balance-build
make -f services/balance/Makefile balance-kustomize
make -f services/balance/Makefile balance-gitops-template-check
```

`balance-k3d-deploy` は、ルートの `deploy/k3d.yaml` と基盤 Kustomize により `pokecalc` クラスタ・
Namespace が作成済みであることを前提とする。共有 Namespace は balance 側では所有しない。

Argo CD用にはlocal imageを参照しない専用overlayを用意している。予約済み`.invalid` domainとzero digestは
意図的なplaceholderであり、`balance-gitops-check`は置換されるまで失敗する。private repository・registryの
credentialやSecretはGitへ入れない。公開用クリーンコピーとimage registryを準備した後の手順は
[`deploy/argocd/README.md`](deploy/argocd/README.md)を参照する。
