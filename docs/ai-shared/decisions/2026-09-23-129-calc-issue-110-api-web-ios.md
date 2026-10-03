## 2026-09-23: calc の候補・観測件数に上限を置く(issue #110。API レーンから データ/Web/iOS レーンへ)
Decision: api/openapi.yaml に maxItems / uniqueItems / maximum を入れた(presets 8+unique、itemVariants 64+unique、
itemCandidates 64+unique、observations 16、maxCandidates 0..128)。calc-svc は生成ラッパの schema 検証に依存できない
(oapi-codegen の echo5/strict サーバーはヘッダしか検証しないことを生成物と実測で確認)ため、ID 解決・engine 呼び出しより
前に自前で検証し 400 invalid_input で拒否する。presets の重複だけは既存どおり duplicate_preset(件数 9 以上は invalid_input が先)。
新しい ErrorCode は足さない。設計は ADR-0208。critic PASS(実HTTPで境界値・issueの再現手順の解消を確認: 2,000×2,000が9.43秒→0.9ms)。
Reason: 1MiB 未満の本文で presets × itemVariants、itemCandidates × 33SP × observations × 16rolls の全組合せを計算させられる
(issue #110)。maxCandidates は出力を切るだけで計算量が減らず、本文サイズ制限も gateway の 10 秒タイムアウトもサーバー内部の
増幅を止めない。
Impact(依頼):
- データレーン(engine): engine.CalcBulk / CalcReverse にも同じ防御上限(presets 8 / ItemVariants 64 / ItemCandidates 64 /
  Observations 16 / MaxCandidates 0 または 1..128)を置いてください。HTTP を通らない直接呼び出し・WASM でも巨大入力を計算しない
  ようにするためです。上限超過は engine の sentinel(命名はデータレーンの判断)。engine の純粋性は保てます(定数と sentinel だけ)。
- データレーン(engine/wasmapi): 同じ上限を wasmapi の語彙で invalid_input 相当に写し、HTTP/WASM parity テストを足してください。
  HTTP 側の code は invalid_input です(presets 重複だけ duplicate_preset)。
- Web レーン: 観測追加 UI を 16 件で無効化(理由表示・アクセシビリティ通知)。持ち物候補が 64 件を超える場合は黙って切り捨てず、
  明示的なエラーか仕様で決めた決定的な絞り込みにしてください。生成型(openapi.gen.ts)の差分はコメントのみで型は変わりません。
- iOS レーン: 同様の対応(観測 16 件上限・候補 64 件超の扱い)。
- 残存リスク(このADRの範囲外): 同時実行数・レート制限は扱っていない。上限ちょうどの reverse は約22.6msのCPUを要するため、
  calc-svc の CPU limit(200m)は理論上 約9 req/s 程度で飽和しうる。レート制限は gateway かクラスタ側の別課題。
- issue #110 は API レーンの分だけでは閉じない。engine/WASM・Web・iOS が追従してから閉じる。
