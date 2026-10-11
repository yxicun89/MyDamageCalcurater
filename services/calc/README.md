# calc-svc

ダメージ計算・一括計算・逆算の HTTP サービス(ステートレス)。契約は `api/openapi.yaml` の `calc` タグ。
計算は `engine/` の公開 API を呼ぶだけで、独自の式を持たない。マスタは起動時に pokedex-svc の内部 API(または
テスト用ファイル)から読み、メモリに載せる。

```mermaid
flowchart LR
  GW["gateway"] -->|"/api/calc, /api/calc/bulk, /api/calc/reverse"| HTTP
  subgraph calc["calc-svc"]
    HTTP["internal/adapters/httpapi<br/>デコード・変換・応答"] --> App["internal/app<br/>手順(検証・ID解決・発行)"] --> Engine["engine<br/>CalcDamage/CalcBulk/CalcReverse"]
    App --> Domain["internal/domain<br/>入力の検証"]
    Source["internal/adapters/master<br/>Source(File/HTTP)"] --> Store["MemoryStore<br/>(app.Master を実装)"] --> App
  end
  Pokedex["pokedex-svc<br/>GET /internal/pokedex/master"] -.->|"CALC_MASTER_URL"| Source
  File["testdata/master.example.json<br/>(架空データ)"] -.->|"CALC_MASTER_PATH"| Source
```

## ディレクトリ

| パス | 役割 |
|---|---|
| `internal/domain` | 純粋な核。列挙・個体・メガシンカ後の持ち物・観測・対戦の状態の検証と、失敗の語彙(`Code`・`FromEngine`)。計算の核は `engine` |
| `internal/app` | ユースケース。計算・一括・逆算の手順(検証 → ID解決 → engine → イベント発行)。ポート `Master`・`EventPublisher`・`Expired` を宣言 |
| `internal/adapters/httpapi` | `api.ServerInterface` の実装。ヘッダ・厳格デコード・件数上限・生成型 ⇄ app の変換・エラー整形。調整(`adjust.go`)の手順はまだここ |
| `internal/adapters/master` | `MemoryStore`(`app.Master` の実装。`Store` はその別名)と、`MasterExport` からの読み込み(`export.go`)、入手元 `Source`(ファイル・HTTP) |
| `internal/adapters/events` | NATS JetStream への計算イベントの発行(ADR-0212) |
| `cmd/calc` | 起動・環境変数の読み込み・URL方式のバックオフ再取得・graceful shutdown |
| `calctest` | 他サービスのテストから calc-svc の実物を起動するための口(gateway の契約テスト等が使う) |
| `testdata/master.example.json` | 架空データの `MasterExport`(ファイル方式・`make dev`・テストの fallback で使う) |

## よく使うコマンド(リポジトリのルートで)

```sh
cd "$(git rev-parse --show-toplevel)"
make test lint build          # engine・services 全体の一部として実行される
make api-docker-build         # イメージのビルド
make api-kustomize            # k8s マニフェストが描画できるか
```

ローカルでの起動・k3d への疎通確認は [`docs/runbooks/api.md`](../../docs/runbooks/api.md)。

## 関連 ADR

- [ADR-0200](../../docs/adr/0200-calc-svc-api-contract.md) calc-svc の API 契約とマスタ境界
- [ADR-0204](../../docs/adr/0204-calc-master-from-pokedex-internal-api.md) マスタを pokedex-svc の内部 API から受け取る(起動時1回・準備状態・`/readyz`)
- [ADR-0206](../../docs/adr/0206-wire-to-pokedex-svc.md) k3d を pokedex-svc につなぐ
- [ADR-0811](../../docs/adr/0811-arch-hexagonal-adoption.md)・[ADR-0252](../../docs/adr/0252-calc-hexagonal-boundary.md) ヘキサゴナルの形(domain・app・adapters)
