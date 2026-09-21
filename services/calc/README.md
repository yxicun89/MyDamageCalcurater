# calc-svc

ダメージ計算・一括計算・逆算の HTTP サービス(ステートレス)。契約は `api/openapi.yaml` の `calc` タグ、設計は
[ADR-0200](../../docs/adr/0016-calc-svc-api-contract.md)。計算は `engine/` の公開 API を呼ぶだけで、独自の式を持たない。

## 起動

| 環境変数 | 必須 | 意味 |
|---|---|---|
| `CALC_ADDR` | いいえ(既定 `:8080`) | 待ち受けアドレス |
| `CALC_MASTER_PATH` | はい | マスタのスナップショット(下の暫定スキーマ) |
| `CALC_TYPECHART_PATH` | はい | タイプ相性表(`testdata/golden/typechart.json` と同じ schema) |

起動時に両方をメモリへ読み込む。読めない・スキーマ違反なら非ゼロで終了する(既定データへのフォールバックはしない。ADR-0013)。

```sh
cd services
CALC_MASTER_PATH=calc/testdata/master.example.json \
CALC_TYPECHART_PATH=../testdata/golden/typechart.json \
go run ./calc/cmd/calc
```

`GET /healthz` は `200 {"status":"ok"}`(運用エンドポイント。openapi には載せない)。

## マスタのスナップショット(暫定スキーマ schemaVersion 1)

データレーンの共通マスタ(`services/internal/master`。plan.md P2-2a)が入るまでの暫定の形。
実データはコミットしない(ADR-0002)。例 [`testdata/master.example.json`](testdata/master.example.json) は架空データ。

```jsonc
{
  "schemaVersion": 1,
  "species":   [{"key":"9001-000","dexNo":9001,"form":0,"nameJa":"テストモン",
                 "types":["normal"],                       // 1〜2個。PokeType の英語 ID
                 "baseStats":{"hp":80,"atk":100,"def":70,"spa":60,"spd":70,"spe":90},
                 "abilities":["test-plain"]}],
  "moves":     [{"id":"test-beam","nameJa":"テストビーム","type":"normal",
                 "category":"physical",                    // physical / special / status
                 "power":80,"priority":0}],
  "items":     [{"id":"test-orb","nameJa":"テストのたま","effect":ItemEffect|null}],
  "abilities": [{"id":"test-plain","nameJa":"テストとくせい","effect":AbilityEffect|null}],
  "natures":   [{"id":"test-def-up","nameJa":"テストかたい","plus":"def","minus":"atk"}] // 無補正は plus/minus とも null
}
```

- `ItemEffect` / `AbilityEffect` は WASM 境界の DTO と同じ形(ADR-0011 §3。`engine/wasmapi` の `itemEffectDTO` / `abilityEffectDTO`)。
  空文字 `""` の扱いも同じ(ADR-0011 §4 の表)。
- ロード時にエラーにするもの: 壊れた JSON・未知のフィールド・JSON の後ろの余計なデータ・`schemaVersion` が 1 でない・
  空の ID・ID の重複(種類ごと)・未知のタイプ/分類/ステータスキー・種族のタイプが 1〜2 個でない・性格が HP を指す、
  相性表に無いタイプの出現(`master.New`)。
- 性格 ID の写像(`Store.NatureID`): 無補正は「無補正の性格(plus == minus)を ID の昇順で並べた最初」、それ以外は
  (plus, minus) が一致する性格。該当なしは `natureId: null`。
