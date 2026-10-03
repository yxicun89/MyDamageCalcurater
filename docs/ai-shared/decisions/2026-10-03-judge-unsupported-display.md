## 2026-10-03: 判定画面に未対応の印を出した(判定レーン。ADR-0713)
Decision: 判定の各行の確定数の直下に、方向ごとの `role="note"` で calc-svc の未対応の印を出す。target は向きで読み替える(順方向 attacker_* = 自分)。
Reason: issue 271・270 の判定レーン分(ADR-0708 §8)。
Impact: judge API 契約・iOS・他画面は無変更。`unsupportedLabels.ts` に `unsupportedMarkName` の export を追加しただけ。
