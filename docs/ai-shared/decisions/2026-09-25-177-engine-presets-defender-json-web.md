## 2026-09-25: 防御側プリセットの正を engine/presets/defender.json にした(データレーン。Web レーンの申し送りへの対応)
Decision: ADR-0009 §1 のカタログ8件の正を `engine/presets/defender.json` にし、`DefenderPresetCatalog()` は embed した JSON から作る(ADR-0009 2026-09-25 追記)。値・順序・キーは不変。攻撃側 JSON と違い `label` も持つ(API の `presetLabel` の出力を変えないため)。
**Web レーンの持ち物に触れた理由**: `web/src/domain/defenderPresets.contract.test.ts` は `engine/bulk.go` の Go リテラルを読んでいたため、engine だけ変えるとこのテストと CI が落ちる。main を赤くしないよう、同じ PR で読み先を JSON に変えた(比べる項目と期待値は同じ、`defenderPresets.ts` は不変)。
**連絡(Web レーン)**: 契約テストの読み先を `engine/presets/defender.json` に変えた。`defenderPresets.ts` を JSON の読み込みに置き換えるか(`defenderPresetForCategory` は Web 限定で残る)は Web レーンが決める。
**連絡(iOS レーン)**: 逆算の自分側に防御側プリセットを出すときは、同じ JSON を契約テストで読める。
Reason: Web が Go ソースを正規表現でパースする契約テストは Go の書き方に依存して壊れやすく、iOS からは読めない。ADR-0114 と同じ理由。
Impact: engine・calc-svc・WASM の出力は不変(`make test-golden` 全件一致)。OpenAPI・WASM 境界の変更なし。
