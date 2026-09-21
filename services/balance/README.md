# balance service

パーティのタイプ相性・弱点・耐性・攻撃範囲を分析するドメインモノリス。
damage-calc とは兄弟サービスで、互いの実行時 API には依存しない。

## TB1 の範囲(防御タイプバランス)

- `internal/balance`: HTTP・DB・Kubernetes に依存しない型・防御相性コア・チーム集計(`AnalyzeDefense`)
- `api/openapi.yaml`: balance 外部 API 契約の正
- `internal/api`: oapi-codegen による生成型
- `internal/master`: 共通マスタへ差し替えるための adapter(タイプ相性表・ポケモンタイプ read model)
- `internal/httpapi`: health と analyze の実装
- `cmd/api`: プロセス起動・`BALANCE_POKEMON_TYPES_PATH` の読み込み・graceful shutdown
- `deploy`: balance 専用 Kustomize と manual-sync の Argo CD Application
- `DEPENDENCIES.md`: 公開前確認用の直接依存・利用理由・license

契約の正は [ADR-0014](../../docs/adr/0014-balance-tb1-defense-analysis.md)。

タイプ相性表はコードに持たない(ADR-0013・ADR-0015)。ダメージ計算レーンの P1-13 でデータ化された
`testdata/golden/typechart.json` を `internal/master/data/typechart.json` にバイト複製して go:embed で同梱し、
`master.EmbeddedTypeChart` が起動時に検証して読む(不正なら起動失敗)。元ファイルとの一致はテスト
(`TestEmbeddedTypeChartMatchesSharedData`)が検査するので、元が更新されたら `make balance-sync-typechart` で複製し直す。
共通マスタ(P2-2)の相性表が別の形で配布されるようになったら、`balance.TypeChartProvider` の adapter だけを差し替える。

ポケモンのタイプは `internal/master.PokemonTypeReadModel`(`BALANCE_POKEMON_TYPES_PATH` が指す JSON を
起動時に1回だけ読む)から引く。これも temporary adapter で、共通マスタのスナップショット schema(P2-2)が
決まったら差し替える。Git に置くのは schema と架空データの example(`testdata/pokemon-types.example.json`、
ID は `9001-000` 以降)だけで、実 Pokémon マスタはコミットしない(ADR-0002)。

## HTTP 契約

- `GET /healthz`: Pod probe。200 `{"status":"ok"}`
- `GET /api/balance/healthz`: Ingress 経由の smoke。200
- `POST /api/balance/v1/team-balance/analyze`: `X-Device-Id` と `X-Session-Id` が必須。判定順は
  ヘッダー(400)→ body(400/413)→ read model 未設定(503)→ pokemonId 解決(422)→ 200。
  - ヘッダー欠落: 400 `missing_request_context`
  - request は1〜6件の `{ "pokemonId": "NNNN-NNN" }`。不正なら400 `invalid_request`
  - 16 KiB を超える request: 413 `request_too_large`
  - `BALANCE_POKEMON_TYPES_PATH` が未設定(または空文字): 503 `master_unavailable`
  - 未登録の `pokemonId`: 422 `unknown_pokemon`(message に該当 ID を含む)
  - 上記以外の内部エラー(相性表や read model の想定外の失敗): 500 `internal_error`(固定文言。内部詳細は返さない)
  - 成功: 200。各メンバー(request順)について18攻撃タイプ(正準順)の防御倍率・6分類・`source`、
    および攻撃タイプごとのチーム集計(`weak`/`quadWeak`/`resist`/`immune`/`neutral`)を返す

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

local overlay(`deploy/k8s/overlays/local`)は架空データの example
(`deploy/k8s/overlays/local/pokemon-types.example.json`、`testdata/pokemon-types.example.json` と同一内容)を
ConfigMap としてマウントし、`BALANCE_POKEMON_TYPES_PATH` を設定する。base と gitops overlay には設定しない。
`balance-smoke` は analyze が 200(架空ID)と 422(未登録ID)を返すことを確認する。

Argo CD用にはlocal imageを参照しない専用overlayを用意している。予約済み`.invalid` domainとzero digestは
意図的なplaceholderであり、`balance-gitops-check`は置換されるまで失敗する。private repository・registryの
credentialやSecretはGitへ入れない。公開用クリーンコピーとimage registryを準備した後の手順は
[`deploy/argocd/README.md`](deploy/argocd/README.md)を参照する。
