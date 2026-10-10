## 2026-10-11: iOS の調整に「目標から振り方を決める」を追加(iOS レーン。ADR-0525。F-11 / I-ios-10)
Decision: Web の ADR-0331 と同じ語・流れで、「調整の内容」の先頭に追加として入れる(従来の5モードは不変)。契約 `adjustGoals` を HTTP で呼ぶ。サーバーが目標の操作を提供していない(404 `not_found`)ときは、案内を出して従来の調整へ戻す(iOS 独自。Web は定数で出し分けている)。
Reason: 目標を数値で入れる手間をなくす要望(usability-round2 F-11)。Web・engine・calc-svc は完了済みで、iOS だけ残っていた。
Impact:
- 契約・engine・services・Web は不変。iOS のみ(`AdjustGoalsService`・`MockAdjustGoalsService`・`RequestLimits.maxAdjustGoals`)。
- Web との差: 機能なしの案内(iOS のみ)。Web の radio 先頭・欄・検査・結果の文は同じ語。
- モック: `POKECALC_MOCK_ADJUST_GOALS` = 未設定/infeasible/unavailable/fail。モックの性格に「素早さ上昇」を1件追加した。
- F-13(文言のやさしい言い換え)で語を変えるときは Web と同時に(目標方式の文言は `AdjustGoalsText.swift` の1か所)。
