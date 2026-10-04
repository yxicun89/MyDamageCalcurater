## 2026-10-04: 判定の素早さに特性・持ち物の補正をマスタの効果データで反映する(判定レーン。ADR-0714)
Decision: judge は pokedex-svc の内部 API(`/internal/pokedex/master`)から特性・持ち物の id と effect だけを読んだ小表(ID → 素早さ効果。ADR-0139 の `SpeedMods`・`IgnoresParalysisSpeedDrop`)を遅延ロード(TTL 既定10分、`JUDGE_SPEED_EFFECTS_TTL`)で持ち、素早さに反映する。連鎖は追い風 → 特性 → 持ち物(スカーフ優先)を4096基準で連結して1回丸め、まひは最後(まひの半減を受けない特性は半減しない)。取得失敗は200のフェイルソフト(第1段と同じ Ignored)。契約は 0.3.0(`SpeedFactor` に `ability`・`item`。`*SpeedIgnored` は「効き方を確定できない」ときだけ)。
Reason: データレーンが素早さ効果を載せたので、特性・持ち物の ID を judge に書かずに反映できる。種族・技は読まず保持しないので、ADR-0700 の「マスタ一式を持たない」前提は保たれる。
Impact:
- iOS: 生成された `SpeedFactor` は `@frozen` の厳格な enum で、`ability`・`item` を受けると判定応答の復号が失敗する。iOS レーンで `ios/PokeCalcKit/Sources/PokeCalcJudgeAPI/Generated/` を再生成(`PokeCalcCore` の `APIJudgeService` は `rawValue` で文字列にしているので、再生成すれば追従する)。
- データ・デプロイ順: (1) calc-svc・Web(WASM と判定画面の i18n)→ (2) master-release で再取り込み → (3) judge。理由: 新しい judge は SpeedMods の無い ID を効果なしと確定するので、再取り込み前に出すと誤応答(第1段より悪化)になり、Web より先だと文言が空になる。古い judge はマスタを読まないので再取り込みが先でも安全(ADR-0714 §6)。
- 他レーン: API・engine は不変。
