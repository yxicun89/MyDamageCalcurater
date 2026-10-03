## 2026-09-25: 攻撃側プリセットの正を engine/presets/attacker.json にした(issue #71。データレーン → Web・iOS レーンへ依頼)
Decision: 攻撃側プリセット(無振り / A(C)特化 / A(C)振り)の正を `engine/presets/attacker.json` の1ファイルにした(ADR-0114)。
engine は embed で読み、`AttackerPresetCatalog()`・`DefaultAttackerPreset()`・`ResolveAttackerPreset(key, category)` を持つ。
JSON はキー(`none` / `x_full` / `x`)・並び順・既定(`none`)・X に振る SP・性格の規則(`neutral` / `boost` と `boostMinus`)を持ち、
表示の文言は持たない。OpenAPI・WASM 境界には足していない。
**依頼(Web レーン)**: `web/src/domain/attackerPresets.ts` が JSON と一致することの契約テストを足す(複製せず読む)。現行の値は一致している。
**依頼(iOS レーン)**: `AttackerPreset.swift` を JSON と突き合わせる契約テストを足す。キーの対応(`aFull`↔`x_full`・`aMax`↔`x`)を明示するか
raw value を揃える。**並び順が JSON(無振り → 特化 → 振り)と逆なので揃える**(画面のセグメントの並びが変わる)。
Reason: Web と iOS が同じ規則を別々に持ち、キー・並び順がすでに食い違っていた。JSON なら Go・TS・Swift のテストがそのまま読める。
Impact: ADR-0300 §5 の「engine への移管」提案(2026-09-21)は engine 側を実施済み。#71 は Web・iOS の契約テストが入るまで残す。
