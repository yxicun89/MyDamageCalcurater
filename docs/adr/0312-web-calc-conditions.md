# ADR-0312: 計算画面の「詳細」(急所・やけど・天候・フィールド・防御側の壁・攻撃側のランク。issue #274 の Web 分)

- 状態: 採用
- 日付: 2026-10-01
- レーン: Web
- 関連: issue #274、docs/ai-shared/DECISIONS.md 2026-09-25「計算条件の入力 UI」、ADR-0501「issue #274」(iOS の受け入れ条件)、
  ADR-0311(攻撃側の特性セレクトは #272 で実装済み。ここでは作らない)、ADR-0300(Web の engine 差し替え)、docs/design.md

## 背景

engine(WASM)と API は急所(`critical`)・場(`field`: 天候・フィールド・壁)・攻撃側の `status`/`ranks` を受け付けるが、
Web の計算画面はどれも入力できない(`buildIndividual` はランクを入力せず、`buildBulkRequest` は `field`/`critical` を渡さない)。
iOS が先に「詳細」で実装したので、文言・並び・既定値を揃える。

## 決定(テストが具体形を固定する)

1. **「詳細」**: 計算画面に disclosure(`<button aria-expanded aria-controls>`)。既定は閉じる。閉じている間は中を DOM に出さない。
   閉じても条件は消さない。常時表示にする入力は無い。数値の直接入力は「詳細」を開いたときだけ(design.md)。
2. **中身と並び**: 急所・やけど(攻撃側の状態異常 burn のみ)→ 天候(なし・はれ・あめ・すなあらし・ゆき)→ フィールド(なし・エレキ・グラス・
   サイコ・ミスト。openapi の enum 順ではない)→ 防御側の壁(リフレクター・ひかりのかべ・オーロラベールの独立トグル)→ 攻撃側のランク。
   天候・フィールドは `fieldset`+`legend`+ラジオ、壁はチェックボックス、ランクは増減ボタン+表示「A +1」「C -2」「A ±0」。
   文言は `i18n/ja.ts` の `calcConditionsText`。
3. **ランク**: 編集するのは選択中の技の分類の関連ステータス(物理・変化 = atk〈A〉、特殊 = spa〈C〉)だけ。atk/spa は別々に保持し、
   要求には両方入れる(def/spd/spe は 0。engine は関連ステータスしか使わない)。-6..+6 で止まる(境界でボタンを disabled)。
4. **既定は従来とバイト単位で同じ要求**: 急所 off・やけど off・天候/フィールド なし・壁 なし・ランク 0 のときは
   `critical`・`field`・`attacker.status`・`attacker.ranks` を要求に載せない(`false`/`none`/全 0 も送らない)。
   触った分だけ載せる: `critical:true`、`status:"burn"`、`field` は非既定の部分だけ(壁は 1 つでも on なら 3 項目すべて)、
   `ranks` はどちらかが非 0 なら 5 項目。攻撃側の壁(`attackerScreens`)は送らない。
5. **純粋関数**: `domain/calcConditions.ts`(部品は `screens/CalcConditionsPanel.tsx`、開閉の状態だけ持つ)(`CalcConditions`・`DEFAULT_CALC_CONDITIONS`・`conditionRequestParts`・`clampRank`・
   `rankStatFor`・`formatRank`・`WEATHER_IDS`・`TERRAIN_IDS`)。`buildIndividual` は任意の `status`/`ranks`、`buildBulkRequest` は
   任意の `critical`/`field` を受け、無ければキーを作らない。画面は条件の state だけを持ち、要求の形はここで 1 か所に決める。
6. **条件の寿命**: 攻守入れ替え・種族・技の変更で消さない(特性だけは ADR-0311 の既存挙動)。
7. **古い結果**: 完了済みの計算(CompletedCalc)に条件も持たせ、入力との一致を比べる(ADR-0311 と同じ流儀)。変えた直後は「計算中」。
8. **オンライン/オフライン**: `apiEngine` は既に `BulkCalcRequest.field`/`options.critical`、attacker の `status`/`ranks` を写す。
   WASM と同じ要求形になることをテストで固定する(API 契約の変更なし)。

## 対象外

- やけど以外の状態異常、攻撃側の壁、防御側の状態異常(防御側のランクは ADR-0315 で追加。特性は ADR-0311)。
- 1対1(`CalcRequest`)・逆算(`ReverseRequest`)への場の追加。

## 検討した案

- 条件をすべて常時表示: 画面が詰まる(issue の既定案どおり不採用)。
- `critical:false`・`field:{}` を常に送る: 既定の要求が従来と変わる。不採用。

## 影響

- 要求の形は条件を触ったときだけ変わる。ゴールデン・engine は不変。
