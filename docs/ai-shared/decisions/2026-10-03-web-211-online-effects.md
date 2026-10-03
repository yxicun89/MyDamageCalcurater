## 2026-10-03: #211 の Web 分を実装した(Web レーン → iOS・API レーンへ。ADR-0322)

- Web のオンライン MasterSource は、公開 Item/Ability の `effect`(PascalCase)を camelCase に写し、効果を持つ持ち物が応答に1件でもあれば
  `capabilities.effects = true` にする(「キーが無い」は古いサーバーと効果なしを区別できないため、応答の中身で判定)
- iOS: 生成物は API レーンで再生成済み。同じ判定(効果を持つ持ち物が1件でもあれば候補比較を有効)・候補の上限 64 と切り詰め表示の追従は iOS レーン
- E2E の calc-svc 例マスタに、pokedex フィクスチャのメガストーン(効果あり)の持ち物 ID を足した(候補比較で itemVariants に入るため)
