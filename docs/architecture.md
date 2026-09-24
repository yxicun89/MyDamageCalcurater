# アーキテクチャ(全体図)

ポケモンチャンピオンズ向けのダメージ計算・タイプバランス・素早さ比較・判定。計算の中心は純粋な Go の `engine` で、
サーバー(calc-svc)とブラウザ(WASM)の両方から同じコードを使う。細部は各 README、決定の理由は `docs/adr/`。

```mermaid
flowchart LR
  subgraph Client
    Web["Web (React)<br/>web/"]
    WASM["engine.wasm<br/>(オフライン計算)"]
    iOS["iOS (SwiftUI)<br/>ios/"]
  end
  subgraph k3d["k3d (Kubernetes)"]
    GW["gateway<br/>services/gateway"]
    Calc["calc-svc<br/>services/calc"]
    Pokedex["pokedex-svc<br/>services/pokedex"]
    Balance["balance-svc<br/>services/balance"]
    Speed["speed-svc<br/>services/speed"]
    Judge["judge-svc<br/>services/judge"]
    MySQL[("MySQL<br/>pokedex DB")]
    Import["importer CronJob<br/>(週1回)"]
  end
  Engine["engine (純粋 Go)<br/>engine/"]
  Web --> GW
  iOS --> GW
  Web -.-> WASM
  GW --> Calc & Pokedex
  Web -- Ingress /api/balance --> Balance
  Web -- Ingress /api/speed --> Speed
  Web -- Ingress /api/judge --> Judge
  Calc -- 内部API: マスタ --> Pokedex
  Pokedex --> MySQL
  Import --> MySQL
  Calc -. 呼ぶ .-> Engine
  WASM -. 同じコード .-> Engine
  Balance -. read model(JSON) .-> Pokedex
  Speed -. read model(JSON) .-> Pokedex
  Judge -- 公開API --> Pokedex
  Judge -- 公開API --> Calc
```

## コンポーネント

| コンポーネント | 役割 | レーン | README |
|---|---|---|---|
| engine | ダメージ・確定数・一括計算・逆算・実数値(I/O なし) | データ | [engine/](../engine/README.md) |
| pokedex-svc / importer | マスタの DB・取込(calc・Showdown・PokeAPI)・マスタ API | データ | [services/pokedex/](../services/pokedex/README.md) |
| calc-svc | 計算 API(engine を呼ぶだけ) | API | [services/calc/](../services/calc/README.md) |
| gateway | ダメージ計算の入口(ルーティング・端末ID・/assets)。balance・speed・judge は各自の Ingress で公開 | API | [services/gateway/](../services/gateway/README.md) |
| balance-svc | 構築のタイプバランス | タイプバランス | [services/balance/](../services/balance/README.md) |
| speed-svc | 素早さ比較 | 素早さ | [services/speed/](../services/speed/README.md) |
| judge-svc | 素早さ×ダメージ連動の判定(抜けるか・倒せるか)。pokedex-svc・calc-svc の公開 API を呼ぶだけ | 判定 | [services/judge/](../services/judge/README.md) |
| Web | 画面(API とオフライン WASM を切り替え)。計算・逆算・タイプバランス・素早さの画面を持つ | Web | [web/](../web/README.md) |
| iOS | iPhoneアプリ。ダメージ計算・逆算・構築のみ(タイプバランス・素早さ・判定は未着手) | iOS | [ios/](../ios/README.md) |

## データの流れ(マスタ)

```mermaid
flowchart LR
  Src["取得元<br/>calc 0.12.0 / Showdown / PokeAPI<br/>(版は Git で固定)"] --> Fetch["tools/importer<br/>(取得)"]
  Fetch --> Gen[("data/generated/<br/>Git 管理外")]
  Gen --> Conv["importer<br/>(照合・変換)"] --> DB[("MySQL")]
  DB --> API["pokedex-svc"]
```

- 実マスタ・スナップショットは Git に入れない(ADR-0002)。Git にあるのはコード・schema・架空データ・版の記録だけ。
- 動作確認の手順は `docs/runbooks/`、進捗は `docs/plan.md`、AI の運用は `docs/ai-shared/COORDINATION.md`。
