# ADR-0315: 計算画面の「詳細」に防御側のランク(B / D)を足す(issue #274 の Web 残り)

- 状態: 採用
- 日付: 2026-10-02
- レーン: Web
- 関連: ADR-0312(「詳細」の既存実装。防御側のランクは契約が無く対象外だった)、ADR-0216(`defenderOverride.ranks/status` の契約)、
  ADR-0311(防御側の特性。`defenderOverride.abilityId` の送り方)、ADR-0501「issue #274」(iOS。防御側のランクの文言は未定義のため
  攻撃側のランクと同じ作りに揃える)

## 背景

ADR-0216 で `BulkCalcRequest.defenderOverride { abilityId?, ranks?, status? }` が入り、WASM(`calcBulk` の `defenderOverride.ranks/status`)も
同じ形で受け付ける。Web の「詳細」は攻撃側のランクしか持たない。防御側の状態異常は今のダメージ式に効かないので出さない(ADR-0216 §3)。

## 決定

1. **場所と文言**: 「詳細」の中、「攻撃側のランク」の次(並び: 急所・やけど → 天候 → フィールド → 防御側の壁 → 攻撃側のランク → 防御側のランク)。
   `fieldset`+`legend`「防御側のランク」、ボタンの名前は「防御側のランクを上げる」「防御側のランクを下げる」。文言は `calcConditionsText`。
2. **編集対象**: 選択中の技の分類に関連する防御側ステータスだけ(物理 = def〈B〉、特殊 = spd〈D〉、変化・技なし = def〈B〉)。
   表示は攻撃側と同じ作りで「B +1」「D -2」「B ±0」(`formatRank` を def→B、spd→D に広げる)。-6..+6 で止まる。
3. **保持**: `CalcConditions.defenderRanks: { def, spd }`(既定 0・0)。def / spd は別々に保持し、技の分類を往復しても消さない。
4. **要求**: 既定(0・0)なら `defenderOverride` を載せない(要求は従来とバイト同一)。どちらかが非 0 なら
   `defenderOverride: { ranks: { atk:0, def, spa:0, spd, spe:0 } }`。`conditionRequestParts` が `defenderOverride` を返し、
   `buildBulkRequest` が `BulkRequest.defenderOverride`(型は `{ ranks?: Ranks }`)として通す。
5. **API と WASM の同じ形**: `BulkRequest.defenderOverride` は WASM の境界 JSON そのまま(`wasmEngine` は素通し)。
   `apiEngine` は `defenderAbilities` がちょうど1件のときの `abilityId`(ADR-0311)と `ranks` を**同じ** `defenderOverride` にまとめて送る
   (片方だけでも、両方でも可。どちらも無ければ `defenderOverride` 自体を送らない)。WASM 側の特性は従来どおり `defenderAbilities`。
6. **寿命・古い結果**: 攻守入れ替え・種族・技の変更・「詳細」の開閉で消さない。`CompletedCalc.conditions` の一致比較に含まれるので追加作業なし。
7. **対象外**: 防御側の状態異常(式に効かない)、逆算(`ReverseRequest` に防御側 override は無い)。

## 検討した案

- 物理・特殊の両方を同時に表示: 画面が詰まる。攻撃側のランクと同じく関連する方だけを出す。
- 防御側のランクを `CalcConditions` の外に持つ: 条件の寿命・古い結果の判定を二重に書くことになる。不採用。

## 影響

- 既存テストの更新: `calcConditions.test.ts` の既定値の期待に `defenderRanks`、`CalcScreen.conditions.test.tsx` の
  「出さない」一覧から「防御側のランク」を外した(契約が入ったための仕様変更。弱めてはいない)。
- 新規テスト: `calcConditions.defender.test.ts`・`requests.defenderRanks.test.ts`・`apiEngine.defenderRanks.test.ts`・
  `CalcScreen.defenderRanks.test.tsx`・`defenderRanks.wasm.test.ts`、`wasmEngine.test.ts` に素通し2本、`e2e/calc.spec.ts` に1本。
