# ADR-0712: 判定に status(状態異常)を足し、まひを素早さに反映する

- 状態: 採用(2026-10-03。判定レーンの判断。issue 235 の追加分。ユーザー決定: 反映する範囲は全て)
- 日付: 2026-10-03
- 関連: ADR-0701 §2(status を置かない決定を更新)、ADR-0710(Applied / Ignored。「状態異常は入力に無い」を更新)、
  ADR-0702 §2(連結と丸め)、ADR-0706 §2(契約の書き下し)

## 決定
1. **契約**: `Individual` と `DefenderCandidate` に省略可の `status`(none・burn・paralysis・poison・badly_poison・sleep・freeze)を足す。
   ルートの `StatusCondition` と同じ値をそれぞれに書き下す(ADR-0706 §2 の方針。allOf で継承しない)。`nullable: true`
   (`null` は省略と同じ。abilityId・itemId と同じ流儀)。`SpeedFactor` に `paralysis` を足す。
2. **省略と none は同じ**で、応答は従来と完全に同じ。calc-svc へは none・省略のときは `status` を送らず、それ以外は
   値をそのまま転送する(順方向は attacker=自分・defender=候補、逆方向は入れ替え)。
3. **素早さへの反映はまひだけ**。`@smogon/calc` 0.12.0 の `getFinalSpeed` と同じく、追い風・スカーフの連結(4096 基準)と
   五捨五超入を済ませた**あと**に `floor(speed × 50 / 100)`(切り捨て)を掛ける。まひは連結に含めない
   (120 → 60、91 → 45、93 → 46、追い風 + スカーフ 360 → 180)。他の状態異常は素早さ・Applied・Ignored を変えない。
4. **`*SpeedApplied` の順序**は rank → tailwind → choiceScarf → paralysis(計算の連鎖順)。まひの側だけに入る。
5. **特性との同時指定の既知の差**: judge は特性の素早さ補正のデータを持たないため、まひと abilityId が同時でも
   まひは常に ×0.5 を掛ける。実ゲームではクイックフィート(まひ中に素早さ ×1.5 で、まひの低下を受けない)などで
   値が変わる。Applied に `paralysis`、Ignored に `abilityId` を残して画面が「特性は反映していない」と示せる。
   第2段(特性のデータ駆動)で解消する。
6. **不正な status は 400 invalid_request** で上流を呼ばない(attacker・候補とも)。大文字小文字は厳密で、未知の値・空文字・
   文字列以外を拒否する。enum の判定は生成した `api.IndividualStatus.Valid()` を使い、値の一覧を二重に持たない。
   `individualWireKeys` に `status` を足す(キーの厳密検査)。
7. **Web**: 各個体(自分・候補)に常に見える「状態異常」select(「なし」が先頭で既定)。「なし」のままなら `status` を送らない。
   結果の「素早さに反映」の行に「まひ」が出る。

## 更新した過去の決定
- ADR-0701 §2: 「`Individual` に `status` を置かない」は、本 ADR で置く。ダメージに効く状態異常を calc-svc へ転送し、
  素早さに効くまひは同時に反映するため、「ダメージには効くのに素早さには効かない」半端さは、まひについては無くなった。
  他の状態異常は素早さに効かないのが正しい。
- ADR-0710: 「状態異常(麻痺など)は入力に無いので Ignored に現れない」を、「まひは反映済みなので Ignored に現れない」に更新。

## 却下した案
- まひを 4096 基準の連結に含める: 原典(getFinalSpeed)と丸めが変わる(奇数で 1 ずれる)。
- まひと abilityId が同時のとき、まひを掛けない・400 にする: 特性のデータが無くても大半のケースで正しい値を返せる。
  既知の差として明記して Ignored の印で補う。
