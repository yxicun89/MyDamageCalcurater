## 2026-10-03: 判定に status(状態異常)を足し、まひを素早さに反映した(判定レーン。issue #235 追加分・ADR-0712)

- judge 契約の `Individual`・`DefenderCandidate` に省略可の `status`、`SpeedFactor` に `paralysis` を追加。まひだけ素早さ ×0.5
  (連結・丸めのあとに floor。@smogon/calc 0.12.0 の getFinalSpeed と同じ)。全ての status は calc-svc へ転送する
- まひと abilityId が同時のとき、まひは常に ×0.5 で abilityId は Ignored に残る(既知の差。クイックフィート等。第2段のデータ待ち)
- iOS: judge の生成クライアントは無く影響なし(ios/ に judge の参照なし)
