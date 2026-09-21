# ADR-0401: balance TB5 おすすめタイプと該当ポケモン

- 状態: 採用(2026-09-22。§1 はユーザー回答。§2 以降は深夜のため**既定案で進行・ユーザー未確認**。朝に確認する)
- 日付: 2026-09-22
- 関連: DECISIONS.md 2026-09-22「TB5」、ADR-0014(TB1 防御・read model)、ADR-0016(TB2 攻撃範囲)、ADR-0017(TB3 特性)、ADR-0400(TB4)、
  ADR-0100 §8(pokedex が balance 向けの read model を出力する)、ADR-0002(実データを Git に置かない)

## 決定

### 1. ユーザー回答(2026-09-22)
- 既存のタイプバランスチェッカーは「一貫を切るためのおすすめタイプ」は出すが、該当するポケモンを別のサイトで探す必要がある。
  おすすめのタイプ候補と、**そのタイプを持つ使用可能なポケモン全員**(日本語名付き)を一緒に出す。
- おすすめの基準は**防御の穴と攻撃範囲の穴の両方**。特性で穴をふさげるポケモンは**別枠**。

### 2. 穴の定義(既定案)
- **防御の穴**(一貫している攻撃タイプ): TB1 の集計で、耐性・無効を持つメンバーが 0 人の攻撃タイプ(`resist + immune = 0`)。
  メンバーの特性(TB3)も反映する。
- **攻撃範囲の穴**: TB2 の `teamCoverage` で、有効打(×1 以上。ユーザー回答 ADR-0016 §1)を取れない防御タイプ(`bestMultiplier` が null または 1 未満)。
  技が1つも無いチームでは攻撃範囲の穴を出さない(判断材料が無い)。

### 3. おすすめタイプの候補(既定案)
- 候補は防御タイプの組み合わせ 171 通り(単タイプ 18 + 複合 153)。
- 各候補について:
  - `defenseCovered`: 防御の穴のうち、その候補が等倍未満(×1/2・×1/4・×0)で受けられる攻撃タイプの数と一覧
  - `offenseCovered`: 攻撃範囲の穴のうち、その候補のタイプ(一致技として持てるタイプ)で ×1 以上を取れる防御タイプの数と一覧
  - `weaknesses`: その候補の弱点(×2 以上)の攻撃タイプの数(同点の並べ替えにだけ使う)
- 並び: `defenseCovered + offenseCovered` の多い順 → `weaknesses` の少ない順 → 正準順(単タイプが先、複合は 1つ目・2つ目の正準順)。
  穴を1つもふさがない候補は出さない。上位 **10** 件を返す(request の `limit` で 1〜20 に変えられる)。
- 候補の特性は考えない(特性は §4 の別枠で扱う)。

### 4. 該当ポケモン
- 候補ごとに、read model のポケモンのうち**タイプの集合が候補と一致する**ものを全員(`pokemonId` の昇順)。`nameJa` があれば付ける。
- **特性の別枠**: 防御の穴ごとに、持ちうる特性(read model の `abilityIds`)のどれかで、その攻撃タイプを等倍未満にできるポケモン
  (特性の read model の効果で判定。タイプだけでは受けられないもの)を、ポケモンと特性の組で出す。
- 「使用可能なポケモン」はレギュレーションに依存する(ADR-0002)。balance は **read model に入っているポケモンを使用可能とみなす**。
  レギュレーションでの絞り込みは read model を出力する側(pokedex export。データレーン)の責務とする(DECISIONS.md に依頼)。

### 5. read model の拡張
- ポケモンの read model(ADR-0014 §2)の各要素に、省略可能な `nameJa`(文字列 1〜64 文字)と `abilityIds`(特性 ID の配列。0〜3 件、重複なし、
  ADR-0017 §2 の ID 形式)を足す。`schemaVersion` は 1 のまま(省略可能な項目の追加で、既存のファイルはそのまま読める)。
- Git の example は架空データ(架空の名前)だけ。

### 6. API
- 新しい endpoint `POST /api/balance/v1/team-balance/recommendations`。request は `{members: [{pokemonId, moveIds, abilityId?}] (1〜6), limit?: 1〜20}`
  (ヘッダー・body 上限・ID の形式・moveIds の必須は TB2・TB4 と同じ)。
- response: `defenseHoles`(攻撃タイプの正準順)、`offenseHoles`(防御タイプの正準順)、`candidates`(§3 の順。各 `types`・`defenseCovered`・
  `offenseCovered`・`weaknesses`・`pokemon: [{pokemonId, nameJa?, types}]`)、`abilityOptions`(防御の穴ごとに `attackType` と
  `pokemon: [{pokemonId, nameJa?, abilityId, multiplier}]`)。
- 判定順とエラーは TB4 と同じ流儀(400 → 413 → 503 → unknown_pokemon → unknown_move → unknown_ability → 200、それ以外は 500)。
  特性の別枠は、特性の read model があるときだけ出す(無ければ空配列。503 にしない)。

## 却下した案
- 候補ごとにチーム全体の再計算で最適化する(例: 候補を入れたときの穴の総数の最小化): 1体入れ替えの前提が要り、仕様が膨らむ。まず §3 の単純な数え上げにする。
- balance がレギュレーションを判定する: レギュレーションはマスタのデータで、balance に持たせると正本が分かれる(ADR-0012)。
