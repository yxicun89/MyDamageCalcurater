# calc-svc

ダメージ計算・一括計算・逆算の HTTP サービス(ステートレス)。契約は `api/openapi.yaml` の `calc` タグ、設計は
[ADR-0200](../../docs/adr/0016-calc-svc-api-contract.md)・[ADR-0204](../../docs/adr/0204-calc-master-from-pokedex-internal-api.md)。
計算は `engine/` の公開 API を呼ぶだけで、独自の式を持たない。

## 起動

マスタ一式(`MasterExport`。下記)は `CALC_MASTER_URL` か `CALC_MASTER_PATH` の**ちょうど1つ**で渡す。
k3d(base・local・local-api のどの overlay でも)は **URL 方式**(`CALC_MASTER_URL=http://pokedex`。ADR-0206)。
ファイル方式(`CALC_MASTER_PATH`)は `make dev` とテスト専用。

| 環境変数 | 必須 | 意味 |
|---|---|---|
| `CALC_ADDR` | いいえ(既定 `:8080`) | 待ち受けアドレス |
| `CALC_MASTER_URL` | どちらか一方 | pokedex-svc のベース URL。`GET {URL}/internal/pokedex/master` からマスタ一式を取得する |
| `CALC_MASTER_PATH` | どちらか一方 | マスタ一式(`MasterExport` の形)の JSON ファイル(`make dev`・テスト用) |

`CALC_TYPECHART_PATH` は廃止した(タイプ相性表は `MasterExport` に含まれる)。設定されていると起動しない。

- **ファイル方式**(`CALC_MASTER_PATH`): 起動時に読み込む。読めない・契約違反なら非ゼロで終了する
  (既定データへのフォールバックはしない。ADR-0013)。`/readyz` は最初から `200`。
- **URL 方式**(`CALC_MASTER_URL`): HTTP サーバはすぐ起動し、バックグラウンドで取得を指数バックオフ再試行する。
  取得できるまで calc の3操作と `GET /readyz` は `503 master_unavailable`、`GET /healthz` は `200`。
  取得後は再取得しない(マスタの更新は `kubectl rollout restart` で反映する)。

```sh
cd services
# ファイル方式
CALC_MASTER_PATH=calc/testdata/master.example.json go run ./calc/cmd/calc

# URL 方式(pokedex-svc の内部 API から取得する)
CALC_MASTER_URL=http://pokedex go run ./calc/cmd/calc
```

`GET /healthz` は `200 {"status":"ok"}`(liveness)。`GET /readyz` はマスタを読み込み済みなら `200 {"status":"ok"}`、
まだなら `503 master_unavailable`(readiness)。どちらも運用エンドポイントで openapi には載せない。

## マスタ一式(MasterExport)

正本は pokedex-svc の DB(ADR-0100)。契約は `api/openapi.yaml` の `GET /internal/pokedex/master`
(タグ `internal`。gateway は公開しない)が返す `MasterExport`。形は pokedex の DB の行
(`services/internal/master` の `TypeRow` / `TypeChartRow` / `SpeciesRow` + `SpeciesAbilityRow` / `MoveRow` /
`ItemRow` / `AbilityRow`)に対応する。相性表・種族・技・持ち物・特性の値の検証(ID の形式・範囲・組の整合・
効果定義)は共通マスタ(`services/internal/master`)の写像で行う。性格(`natures`)だけは、まだ pokedex に
テーブルが無いため calc-svc 側で検証する(ID が空・重複でない、`plus`/`minus` が `StatKey` で HP を指さない)。

実データはコミットしない(ADR-0002)。例 [`testdata/master.example.json`](testdata/master.example.json) は
架空データ + 相性表(ファイル方式・`make dev`・`calctest`・契約テスト・スモークの fallback で使う。ADR-0206)。

k3d では pokedex-svc(ADR-0105)の内部 API からマスタを取る。`make up` の直後は DB が未投入なので、初回だけ
`make import-k8s` を実行するまで `/readyz` が `503 master_unavailable` のままになる(ADR-0204 §3)。

- `FromExport`(`internal/master/export.go`)がロード時にエラーにするもの: `schemaVersion` が 1 でない・
  `dataVersion` が空、種類ごとの ID の重複、種族/技/持ち物/特性/性格の値の不正(共通マスタ・engine の検証)、
  種族の特性・メガの元種族・メガストーンが対応する一覧に無い、性格の不正。すべて `ErrInvalidMaster` に包み、
  該当すれば共通マスタの `ErrInvalidRow` / `ErrInvalidEffect`、`engine.ErrInvalidTypeChart` も `errors.Is` で判別できる。
- `DecodeExport` は厳格デコード(未知のフィールド・後続データ・必須のトップレベルフィールドの欠落/null を拒否)。
  効果定義(`item_effects` / `ability_effects` の JSON)の数値は字面のまま保つ(`5324.0` を `5324` に丸めない)。
- 性格 ID の写像(`Store.NatureID`): 無補正は「無補正の性格(plus == minus)を ID の昇順で並べた最初」、それ以外は
  (plus, minus) が一致する性格。該当なしは `natureId: null`。
