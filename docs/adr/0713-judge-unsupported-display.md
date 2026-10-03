# ADR-0713: 判定画面に calc-svc の「未対応」の印を出す

- 状態: 採用(2026-10-03。判定レーンの判断。issue 271・270 の判定レーン分。ADR-0708 §8 の「画面は別タスク」分)
- 関連: ADR-0708(印の契約)、ADR-0123(印の文言)、ADR-0215(target・reason を enum にしない)

## 決定
1. **表示位置**: 各行(相手候補)の確定数の `<p>` の直下に、方向ごとに別の `<div role="note">` を出す。
   自分の技の確定数の直下は `judge-unsupported-attacker-ko`(attackerKoUnsupported)、相手の技の確定数の直下は
   `judge-unsupported-defender-ko`(defenderKoUnsupported)。印が空なら何も出さない。確定数の文言は消さず注意文を添える。
2. **向きの読み替え**: 印の target の attacker_* / defender_* は、その calc から見た役割。順方向は attacker = 自分・
   defender = 相手候補、逆方向はその逆(`judgeScreenText.koUnsupportedTargetLabel`)。未知の target は向きの無い「項目」。
3. **再利用**: `unsupportedLabels.ts` の `unsupportedMarkName`(ID → 表示名)を export して使い、書式
   `<対象>「名前または ID」(<理由>)` は `unsupportedText` の理由表を使って画面側で組み立てる(unsupported_effect は理由を省く、
   未知の reason は `unknownReason`)。計算・逆算・調整画面の `unsupportedMarkLabel` は変えない。
   技の名前は `master.moves` に、自分側と候補ごとの種族検索で解決した技を足して引く。自分側の技は送信時点のスナップショット(結果のあとに種族を変えても表示名が変わらない。候補側と同じ作法)。
4. **注意文**: `judgeScreenText.koUnsupportedNote`(確定数ごと。`unsupportedText.notice` は使わない)。
   共通印の集約(`splitUnsupportedMarks`)は判定では使わない(行ごとの 2 方向で足りる)。
