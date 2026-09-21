# ADR-0017: balance TB3 特性による防御相性の変化

- 状態: 採用(2026-09-21。§1 の3点はユーザー回答。§2 以降はタイプバランスレーンの既定案で、ユーザー未確認)
- 日付: 2026-09-21
- 関連: docs/type-balance-design.md §6 TB3、ADR-0014(TB1。analyze の契約・read model の流儀)、ADR-0015(相性表)、ADR-0016(TB2)、
  docs/ai-shared/claude-review.md 指摘3(`EffectSource`)、ADR-0005(効果はデータ駆動)

## 決定

### 1. ユーザー回答(2026-09-21)
- **特性は request で任意に指定する**: analyze のメンバーに `abilityId` を任意で付ける。省略時は特性なし(TB1 と同じ結果)。
- **効果の範囲は ×3/4 なども含める**: 特定タイプの無効・吸収、特定タイプの倍率変更(×1/2・×5/4・×2 など)、効果抜群の軽減(×3/4)を扱う。
- **集計**: 特性による無効・吸収はチーム集計の「無効」に含め、各倍率の `source`(`type` / `ability`)と効果の種類で区別する。

### 2. 効果は正規化されたデータで表す(巨大な abilityID の switch を作らない。設計書 §6 TB3)
特性の read model(temporary。ADR-0014 と同じ流儀)は環境変数 `BALANCE_ABILITIES_PATH` の JSON を起動時に読む:

```json
{"schemaVersion": 1, "abilities": [
  {"abilityId": "ability-9001", "effects": [{"kind": "immune", "attackType": "ground"}]},
  {"abilityId": "ability-9002", "effects": [{"kind": "absorb", "attackType": "water"}]},
  {"abilityId": "ability-9003", "effects": [{"kind": "type_multiplier", "attackType": "fire", "numerator": 1, "denominator": 2}]},
  {"abilityId": "ability-9004", "effects": [{"kind": "super_effective_multiplier", "numerator": 3, "denominator": 4}]},
  {"abilityId": "ability-9005", "effects": []}
]}
```

| kind | 意味 | 項目 |
|---|---|---|
| `immune` | その攻撃タイプを無効にする(×0) | `attackType` |
| `absorb` | その攻撃タイプを吸収する(ダメージは ×0。回復・能力上昇などの副次効果は扱わない) | `attackType` |
| `type_multiplier` | その攻撃タイプの倍率に num/den を掛ける | `attackType`、`numerator`、`denominator` |
| `super_effective_multiplier` | タイプ相性が等倍より大きいとき num/den を掛ける | `numerator`、`denominator` |

- 検証: schemaVersion 1、`abilities` は1件以上、`abilityId` は `^[a-z0-9]+(-[a-z0-9]+)*$`・40 文字以内・重複なし、`effects` は配列(空可。タイプ相性に関係しない特性)、
  kind ごとの必須項目と余分な項目の禁止、`attackType` は 18 タイプ、`numerator` / `denominator` は 1〜16 の整数、同じ特性の中で同じ攻撃タイプへの
  `immune` / `absorb` の重複は不正。未知フィールド・後続 JSON は不正 → 起動失敗。空文字の env は未設定扱い。
- Git には**架空データの example** だけ(`ability-9001` 以降。実在の特性の名前・ID を使わない)。
- 技の種類に依存する効果(接触・音・弾・風など)や、天候・場で変わる効果、ぶきようなど相手の特性を無視する効果は扱わない(タイプだけでは決まらないため)。

### 3. 倍率の表現を有理数に広げる
- 特性の倍率(×3/4・×5/4 など)を掛けると、TB0 の整数表現(4 = 等倍)では表せない。防御側の最終的な倍率は **既約分数 `numerator/denominator`(整数)** で持つ。
  float は使わない。タイプ相性の1組(`Multiplier`)は従来どおり。
- 計算順: タイプ相性(単・複合の積)→ タイプ由来の無効(×0)ならそこで確定(source=`type`)→ 特性の効果を順に掛ける。
  - `immune` / `absorb` が当てはまれば ×0(source=`ability`、effect=`immune` / `absorb`)。
  - `type_multiplier` は該当タイプなら掛ける。`super_effective_multiplier` はタイプ相性が等倍より大きいときだけ掛ける。
  - 特性で値が変わったら source=`ability`、変わらなければ `type`。
- API の `multiplier` 文字列は、既約分数の文字列(`"0"`、`"1"`、`"2"`、`"1/2"`、`"3/4"`、`"5/4"`、`"3"` など。分母 1 は整数だけ)に広げる。
  TB1 の6値はその部分集合なので、特性を指定しない request の response は TB1 と同じ。
- category(ADR-0014 §3 の6分類)は値の範囲で決める: 0 → `immune`、0 < x ≤ 1/4 → `quad_resist`、1/4 < x < 1 → `resist`、1 → `neutral`、
  1 < x < 4 → `weak`、x ≥ 4 → `quad_weak`。チーム集計の定義(ADR-0014 §3)は変えない(特性による無効も `immune` に数える)。
- 各倍率に `effect`(`none` / `immune` / `absorb` / `multiplier`)を足す。`source` と合わせて、タイプ由来か特性由来か・無効か吸収かを区別できる。

### 4. API と判定順
- analyze の request のメンバーに `abilityId`(任意。形式は §2)を足す。coverage(TB2)は変えない(攻撃側の特性は扱わない)。
- response のメンバーに `abilityId`(指定したときだけ。無ければ省略)。
- 判定順: ヘッダー(400)→ body(400/413)→ ポケモンの read model 未設定、または `abilityId` を指定したメンバーがいるのに特性の read model 未設定(503)
  → unknown_pokemon(422)→ unknown_ability(422、message は `unknown abilityId: <ID>`)→ 200。それ以外は 500 固定文言。
  特性を指定しない request は、特性の read model が無くても従来どおり動く。

### 5. 細部(spec-writer が挙げた未決の確定。レーン内の判断)
1. `effect` は特性の効果の種類を表す。タイプ由来の無効(×0)は `effect=none`・`source=type`(特性を持たないメンバーが `none` 以外になることはない)。
2. request の `"abilityId": null` は省略と同じ(任意の項目なので)。`moveIds`(必須)の null が 400 なのとは扱いが違う。
3. read model は、`type_multiplier` の係数 1(例 2/2)や、同じ種類の効果の重複(`super_effective_multiplier` が2つ等)を拒否しない。効果は書かれた順に掛ける(掛け算なので順序で結果は変わらない)。
   係数 1 は「変化なし」として `source=type`・`effect=none`。
4. `super_effective_multiplier` の条件「等倍より大きい」は、特性を掛ける前のタイプ相性で判定する。
5. タイプ由来の無効と同じタイプを特性でも無効にする場合は、タイプ由来(`source=type`・`effect=none`)として報告する。
6. 倍率の積は int64 の既約分数で計算し、先に約分してから掛ける。収まらない(効果を多数重ねたデータでだけ起きる)ときは黙って折り返さず
   `ErrEffectivenessOverflow`(HTTP では 500 固定文言)。比較は 128 ビットで行い溢れない。係数は 1〜16 の整数の比なので分子 0 の係数は不正。
7. read model で kind に無い項目は、キーがあれば値によらず不正とするのが原則だが、値が `null` の場合は欠落と同じとみなして受け付ける(Go の decode の都合。値は使わない)。
8. 「特性を指定しない request は TB1 と同じ」は、倍率・category・source・集計が同じという意味。response の各倍率には `effect`(常に `none`)が増える(契約の版は 0.4.0)。

## 未決(既定案で進行・ユーザー未確認)
- §2 の効果の種類で足りない特性(例: 効果抜群以外を無効にする特性、半減実を持たせる等の持ち物)は扱わない。必要になったら kind を足す。
- category の境界(×3 は `weak`、×5 は `quad_weak` など)。

## 却下した案
- 倍率の基準値を 4 から 16 などに上げて整数のまま持つ: ×5/4 と ×1/4 の積など分母が増える組み合わせに弱く、将来の効果の追加で再び足りなくなる。
- 特性ごとの switch: 設計書 §6 TB3 が禁じている。
