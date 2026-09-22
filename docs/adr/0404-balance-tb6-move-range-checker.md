# ADR-0404: balance TB6 技範囲チェッカー

- 状態: 採用(2026-09-22。§1 はユーザー回答。§2 以降はタイプバランスレーンの判断)
- 日付: 2026-09-22
- 関連: ユーザー要望(https://pokedesiaf.com/skillrange-tool/ 相当の機能に、具体的なポケモンまで出す)、
  ADR-0016(TB2 攻撃範囲。有効打の定義)、ADR-0401(TB5。カタログ照合・特性を考慮した防御計算)、ADR-0402(read model の JSON Schema)

## 背景
既存の TB2(coverage)は「自分のチームの攻撃範囲の穴」を見るための機能で、チームメンバー(pokemonId 必須)を前提にする。
ユーザーが欲しいのは、**特定の技構成(技 ID の集合)そのものの攻撃範囲**を、実在のポケモン図鑑と突き合わせて「この技構成を半減以下で受けられる
ポケモンは誰か」を具体名で出す機能。ポケモン自身を指定する必要はない(技だけで攻撃範囲は決まる)。

## 決定

### 1. ユーザー回答(2026-09-22)
- 入力は**技 ID のみ、最大4つ**(ポケモン自身は指定しない)。
- 「受けに回れる」の基準は**半減以下(×1/2 以下)**。
- 出力に含めるもの: **受けに回れる実在ポケモン一覧**・**18タイプ別の一貫判定**・**特性で受けに回れるポケモンも含める**。

### 2. API
新しい endpoint `POST /api/balance/v1/move-range/analyze`(team-balance 系とは別の path。チームの概念が無いため)。

- request: `{moveIds: [moveId, ...]}`(1〜4件、重複不可。ADR-0016 §6 と同じ moveId の形式・検証)。
- 変化技(status)は攻撃タイプの計算に入れない(ADR-0016 §1 と同じ)。全件が変化技、または moveIds が実質空になる入力は 400
  (`invalid_request`。「技範囲が無い」を返すのではなく、意味のある入力を要求する)。
- response:
  - `attackTypes`: 変化技を除いた技のタイプ(重複なし・正準順)。
  - `typeChart`: 18 の単防御タイプ(正準順)ごとに `{defenseType, bestMultiplier, effective(×1以上), superEffective(×2)}`。
    定義は TB2(ADR-0016 §3)の1メンバー分の `defense` 配列と同じ計算(`bestMultiplier` は必ず値を持つ。技が1つも攻撃技でなければ 400 なので null にならない)。
  - `walledBy`: 半減以下で受けられる実在ポケモン(read model のカタログ全件)。各ポケモンについて、そのポケモンの実際のタイプ(単/複合)に対する
    技構成の最大倍率(`CalculateDefense`。特性は考えない)が ×1/2 以下なら掲載。項目は `{pokemonId, nameJa?, types, bestMultiplier}`。
    `pokemonId` の昇順。
  - `walledByAbility`: **タイプだけでは半減以下にならないが、特性(read model の `abilityIds` のどれか)を使うと半減以下になる**ポケモンを別枠で出す
    (ADR-0401 の `abilityOptions` と同じ発想)。項目は `{pokemonId, nameJa?, abilityId, bestMultiplier}`。`pokemonId` 昇順 → `abilityId` 昇順。
    無効(×0)・吸収も基準(半減以下)を満たすのでここに含む。特性の read model が無ければ空配列 `[]`(503 にしない。ADR-0401 §7 と同じ)。
- 判定順: ヘッダー(400)→ body(400。moveIds の件数・形式・重複)→ 技の read model 未設定(503。moveIds は必須なので常にこの判定に到達する)
  → moveId の解決(422 `unknown_move`)→ ポケモンの read model(カタログ)未設定(503。`walledBy` の計算に必須なので常に到達する)→ 200。
  それ以外の内部エラー(相性表の失敗・不正な技の分類など)は 500 固定文言(ADR-0400 §4 と同じ流儀)。
- ポケモンの read model(カタログ)が無くても、`typeChart` だけは計算できるが、**このステージでは `walledBy` を主目的とするため 503 にする**
  (「一覧だけ見たい」は将来 limit=0 相当のオプションとして検討。今回は作らない)。

### 3. コア計算
- `MoveRangeAnalysis(moveIDs []MoveID, resolvedMoves []Move, catalog []CatalogPokemon, abilities AbilityProvider) (MoveRangeResult, error)` のような形。
- `attackTypes` は既存の `attackTypesOf` を再利用。
- `typeChart` の 18 行は、`AnalyzeCoverage` が1メンバー分の `defense` を作る計算をそのまま流用する(内部的には「攻撃技だけを持つ仮想のメンバー1体」として
  `AnalyzeCoverage` を呼ぶか、共通のヘルパーに切り出す。二重実装しない)。
- `walledBy` / `walledByAbility` は TB5(`recommend.go`)の `matchingPokemon` / `AbilityOption` の考え方を流用するが、TB5 は「候補のタイプ集合」に対する
  一致判定、TB6 は「実在ポケモンの実際のタイプ」に対する `CalculateDefense` の直接計算という違いがあるため、新しいヘルパーを書く
  (カタログの全ポケモンを1件ずつ評価。348 件程度なら計算量は問題にならない)。

### 4. 細部(spec-writer が挙げた未決の確定)
1. エラー名は新設してよい(`ErrMoveRangeMoveCount`・`ErrMoveRangeNoAttackMove`)。重複は既存の `ErrDuplicateMove` を再利用する。
2. 「全件変化技 → 400」の判定は、技を解決した後(unknown_move の 422・read model の 503 より後ろ)になってよい。ADR §2 の判定順はこの位置を妨げない。
3. body が 16 KiB を超えるときは、他の4エンドポイントと同じく 413 `request_too_large` にする(既存の `decodeJSONBody`/`MaxBytesReader` をそのまま使う)。
4. 503 が必要な read model は **ポケモンのカタログ(`PokemonCatalog`)だけ**。`PokemonTypes`(pokemonId→タイプの read model)は TB6 では使わないので、
   nil でも 200 でよい(TB5 が両方必須なのとは非対称だが、TB6 は pokemonId を受け取らないので妥当)。
5. カタログの `abilityIds` に特性 read model が知らない ID があれば、ADR-0401 §7.2 と同じくその特性だけ飛ばす(全体を 500 にしない)。
6. `typeChart` の共通化の実装方法(`AnalyzeCoverage` を仮想メンバー1体として呼ぶか、ヘルパーへ切り出すか)は implementer の判断に委ねる。
   二重実装しないことだけが必須(`TestAnalyzeMoveRangeTypeChartAgreesWithCoverage` で担保)。
7. `bestMultiplier` は必ず値を持つので、既存の `CoverageMultiplier`(nullable)とは別の非 nullable schema `MoveRangeMultiplier` にする。
8. `walledBy`/`walledByAbility` の倍率は複合タイプの ×1/4・×4 も出るため、既存の `DefenseMultiplier`(既約分数の文字列)を再利用する。

## 却下した案
- 既存の coverage(TB2)を拡張してポケモン指定を任意にする: request の形(pokemonId 必須)を壊す。呼び出し側(Web 等)の契約変更が要らない別 endpoint の方が安全。
- 「一貫」を4タイプの複合まで見る: 設計書 TB2 の「防御側は18の単タイプ」の方針に合わせ、単タイプだけにする(複合は TB4 仮想敵診断で個別に見られる)。
