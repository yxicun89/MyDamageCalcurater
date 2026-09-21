# ADR-0019: balance TB4 仮想敵診断

- 状態: 採用(2026-09-22。§1 はユーザー回答。§2 以降はタイプバランスレーンの判断)
- 日付: 2026-09-22
- 関連: docs/type-balance-design.md §6 TB4、ADR-0014(TB1 防御)、ADR-0016(TB2 攻撃範囲)、ADR-0017(TB3 特性・既約分数)

## 決定

### 1. ユーザー回答(2026-09-22)
仮想敵を最大 6 体(pokemonId・技 ID 最大 4・特性は任意)で入力し、各仮想敵について、自分の各メンバーが**受ける**最大倍率(受けやすさ)と
**与えられる**最大倍率(打ちやすさ)を表にし、安全に受けられるメンバー数を集計する。TB1〜3 の計算を再利用する。

### 2. API
- 新しい endpoint `POST /api/balance/v1/team-balance/threats`。ヘッダー・body 上限・ID の形式は既存と同じ。
- request:
  - `members`: 自分のチーム 1〜6 体。各 `{pokemonId, moveIds(0〜4、同メンバー内で重複不可), abilityId(任意)}`
  - `threats`: 仮想敵 1〜6 体。形は `members` と同じ
- response:
  - `threats[i]`: `pokemonId`、`abilityId`(指定時のみ)、`attackTypes`(変化技を除いた技のタイプ。重複なし・正準順)、
    `matchups`(自分のメンバーごと。request の順): `pokemonId`、`incoming`(仮想敵の攻撃技から自分が受ける最大倍率)、`outgoing`(自分の攻撃技で仮想敵に与える最大倍率)、
    `safe`(incoming < 1)、`superEffective`(outgoing ≥ 2)
  - 同じ仮想敵について `safeMembers`(`safe` の人数)、`superEffectiveMembers`(`superEffective` の人数)
- 倍率は既約分数の文字列(ADR-0017 §3)。攻撃技が1つも無ければ `null`(`safe` / `superEffective` は false)。

### 3. 計算
- incoming: 仮想敵の攻撃タイプ(変化技を除く)ごとに、自分のメンバーのタイプと特性で `CalculateDefenseWithAbility` を計算し、その最大。
- outgoing: 自分のメンバーの攻撃タイプごとに、仮想敵のタイプと特性で `CalculateDefenseWithAbility` を計算し、その最大。
- 「安全に受けられる」= incoming が等倍未満(×1/2・×1/4・×0、特性による無効・吸収を含む)。「打ちやすい」= outgoing が ×2 以上。
- STAB・持ち物・技の威力・急所・天候・場は考えない(タイプ相性と特性だけの診断)。技の種類で変わる効果は ADR-0017 §2 と同じく扱わない。

### 4. 判定順とエラー
ヘッダー(400)→ body(400/413。メンバー・仮想敵とも 1〜6、moveIds 0〜4 と重複、ID の形式)→
ポケモンの read model 未設定、または moveIds が1つでもあるのに技の read model 未設定、または abilityId が1つでもあるのに特性の read model 未設定(503)
→ unknown_pokemon → unknown_move → unknown_ability(いずれも 422、members → threats の順・メンバー順で最初のもの)→ 200。
それ以外は 500 固定文言(倍率の積のオーバーフローを含む)。

### 5. read model
既存の read model(ポケモンのタイプ・技・特性)をそのまま使う。新しいデータは要らない。

## 却下した案
- ダメージ量(威力・能力値)で「受けられる」を判定する: ダメージ計算(damage-calc)の責務で、balance はタイプバランスに限る(ADR-0012)。
- 仮想敵をタイプだけで入力する: ユーザー回答(ポケモン + 技 ID)。
