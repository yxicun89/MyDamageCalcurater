# タイプバランスチェッカー テスト戦略

## TB0 の対象

TB0 は、型・タイプ相性コア・差し替え可能なマスタ境界・HTTP 最小疎通・コンテナ・
Kubernetes/Argo CD 定義を検証する。チーム集計、攻撃範囲、特性反映は TB1 以降とする。

## レイヤー

| レイヤー | 対象 | 合格条件 |
|---|---|---|
| Unit | `internal/balance` | 18タイプ、整数倍率、単/複合タイプ、入力エラー、`EffectSource` |
| Adapter | temporary static type chart | 18×18 を返し、代表的な弱点・耐性・無効が既存表と一致 |
| Contract/HTTP | service-local OpenAPI / health / analyze stub | 生成型を使用。health は200。端末ID/セッションID欠落は400。有効な疎通は501 |
| Build | Go / Docker / Kustomize | `go test`、`go vet`、`go build`、Docker build、overlay build が成功 |
| Smoke | k3d | Pod Ready、health 200、Ingress経由 analyze 501 |

## マスタデータの扱い

TB0 の静的タイプ相性表は開発継続用の temporary adapter で、恒久正本ではない。
純粋コアは provider interface だけを参照し、ADR-0012 と共通マスタ確定後に adapter を差し替える。
テストは temporary であることを隠さず、データ出典を `services/balance/README.md` に記録する。

## 実行コマンド

```sh
make -f services/balance/Makefile balance-gen
make -f services/balance/Makefile balance-test
make -f services/balance/Makefile balance-lint
make -f services/balance/Makefile balance-build
make -f services/balance/Makefile balance-kustomize
make -f services/balance/Makefile balance-gitops-template-check
make -f services/balance/Makefile balance-docker-build
make -f services/balance/Makefile balance-k3d-deploy
make -f services/balance/Makefile balance-smoke
```

Docker/k3d/Argo CD を実行できない場合は、未実施理由と再実行コマンドを記録し、成功扱いにしない。

## TB0 の未完了ブロッカー

2026-09-21 時点で、Docker build、k3d への直接 deploy、Pod Ready、smoke、GitOps template検査は確認済み。
GitOps専用overlay、digest固定、private repository/registryの秘密をGitへ入れない手順も用意した。
一方、実Git remote、取得可能な配布イメージの置き場所、Argo CD Application CRD が未設定のため、
`Git変更 → Argo CD manual sync → Pod更新` は未実施であり、TB0 全体は完了扱いにしない。
実施には利用する Git repository URL、image registry/repository、不変 image tag または digest、
およびversion固定したArgo CDの導入とprivate repository/registry credentialのクラスタ登録が必要である。
