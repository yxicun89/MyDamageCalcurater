## 2026-10-04: 調整の複数目標の探索(F-11 段階 B。ダメージ計算レーンから Web・iOS へ)
Decision: ADR-0177 を採用した。engine に `SuggestSPForGoals`・`GuaranteedSelfSpeedStage` を足し、calc-svc の `POST /api/calc/adjust/goals` を実装した。Web の `ADJUST_GOALS_ENABLED` を true にした(モードの radio は 6 つ、先頭が「目標から振り方を決める」)。
Reason: 利用者の要望「相手を選んで目標の種類を選ぶ」(usability-round2 F-11)。段階 A(ADR-0331)で契約と Web は先に入っていた。
Impact:
- 契約の型は変えていない(説明文だけ。#631 で入った形のまま)。新しいエラー code も無い。
- 満たせないときの組は目標の順に依存する(前の目標を優先する。ADR-0177 §5)。画面で順を入れ替える操作が要るかは使用感で判断する。
- iOS(ADR-0331 §8): 同じ契約を HTTP で呼ぶ。WASM には出していない(調整は API 専用。ADR-0319 §1)。
- 素早さは持ち物・特性・場・まひの補正を含めない(ADR-0331 §3)。先に使う技の段数は確率 100%・自分が対象・素早さのときだけ掛ける。
- k3d のスモーク(`make e2e`)に「目標(素早さ + 倒す)で 200」を足す案は残っている(ADR-0331)。
