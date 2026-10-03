# ADR-0714: 判定の素早さに特性・持ち物の補正をマスタのデータ駆動で反映する(issue 235 第2段)

- 状態: 採用(2026-10-04。判定レーンの判断。ユーザー決定: issue 235 第2段を判定側で実装する)
- 日付: 2026-10-04
- 関連: ADR-0139(素早さ効果データ SpeedMods・IgnoresParalysisSpeedDrop)、ADR-0700 §2・§3(上流の呼び方・エラーの正規化)、
  ADR-0701 §2・§3(スカーフ)、ADR-0702 §2(連結と丸め)、ADR-0710(Applied / Ignored)、ADR-0712(まひ)

## 背景

第1段(ADR-0710)は、素早さに効く特性・持ち物のデータが無いので、指定された特性・持ち物を「反映していない入力」として応答に残すだけだった。
データレーンが ADR-0139 で、マスタの特性・持ち物の効果に素早さの項目(`SpeedMods`・`IgnoresParalysisSpeedDrop`)を載せた。
judge はこれを読んで、ID を分岐に書かずに素早さへ反映する。

## 決定

### 1. データの取り方(補助データ・フェイルソフト)

- pokedex-svc の内部 API `GET /internal/pokedex/master` から、特性・持ち物の `id` と `effect` だけを読んで
  「ID → 素早さ効果」の小表にする(`internal/speedeffects`)。最大 4 MiB の一式を取得して全部読み、小表だけを保持する(一時メモリを使う。種族・技・相性表は保持しない)。
  素早さ効果の無い ID も「効果なしと確定」として表に載せる。内部 API は端末 ID・セッション ID を要らないので送らない。
  本文の上限は `client.MaxMasterExportBytes`(4 MiB。calc-svc の `MaxExportBytes` と同じ。Pod の memory limit は 64Mi)。
- 遅延ロード。TTL は既定 10 分(`JUDGE_SPEED_EFFECTS_TTL`。30 秒以下は起動エラー)。マスタの更新は数日に 1 回なので 10 分遅れで足りる。
  同時の取得は 1 回に束ねる。取得に失敗したら 30 秒は取りに行かない(上流が落ちている間、要求ごとに叩かない)。
  取得済みの表があれば、更新に失敗しても古い表を使い続ける。取得の ctx は呼び出し元から切り離し(1 人の中断で他の要求を巻き込まない)、
  待つ側は自分の ctx が終わったら「データなし」で戻る。
- リクエストに特性も、スカーフ以外の持ち物も無ければ取りに行かない(種族・技を解決した後、calc-svc を呼ぶ前に 1 回だけ取る)。
- **フェイルソフト**: 取得できない・本文が契約に合わない場合でも判定は 200 で返す。特性・持ち物は素早さに掛けず、
  `*SpeedIgnored` は第1段と同じ(「指定されたが効き方を確定できない」)。上流の事情は応答に出さず、ログだけ(`slog.Warn`。本文・URL は出さない)。
- デコードは `SpeedMods` と `IgnoresParalysisSpeedDrop` だけを、キーの大文字小文字を区別して読む(encoding/json は寄せてしまうため)。
  素早さの項目が不正な ID(SpeedMods が空・null・17 件以上、要素が `Condition`・`Modifier` のちょうど 2 キーでない、条件の重複、
  `Modifier` が整数でない・`1..engine.MaxEffectModifier` の外・4096)は、その ID だけ表に載せない(確定できない)。
  未知の `Condition` は形として受け付け、評価で「確定できない」にする。文書全体の形の不正(items / abilities が無い・null、
  id が無い・空・重複、後ろにゴミ)は表全体を「データなし」にする。

### 2. 条件の評価

`always` は常に成立、`weather_*` は共通の `field.weather` との一致、`terrain_electric` は `field.terrain == electric`、
`has_status` はその個体の `status` が none 以外。`item_lost`(judge は「持ち物を失った」入力を持たない)と未知の条件は判定できない。
配列は前から評価し、成立した最初の要素だけを掛ける。判定できない要素に当たったらそこで止めて何も掛けず(前の要素が実は成立していたかも
しれないので後ろを掛けると誤る)、その ID を Ignored に残す。`IgnoresParalysisSpeedDrop` は無条件のフラグ。

### 3. 連鎖(@smogon/calc 0.12.0 の getFinalSpeed と同じ)

追い風 → 特性 → 持ち物の順に 4096 基準で連結して 1 回だけ五捨五超入し、そのあとにまひの `floor(x × 50 / 100)`。
こだわりスカーフは持ち物の効果データより優先し、スカーフのときは持ち物の効果データを掛けない(持ち物は 1 つ。原典も Choice Scarf を先に見る)。
まひの半減を受けない特性(`IgnoresParalysisSpeedDrop`)なら、まひの半減を掛けず、`paralysis` も Applied に入れない。
`@smogon/calc` の `chainMods` の範囲制限 [410, 131172] と、素早さの上限は合わせない(既知の差。Champions 世代の上限は 999〈`gen.num <= 2 ? 999 : 10000`〉。judge は上限を持たない)。
現行の効果データ〈×0.5〜×2〉が `chainMods` の範囲に当たらないことは正しいが、素早さの上限 999 については当たりうる(実数値が大きい個体への ×2 など)ので、既知の差として残す。

### 4. 契約(`services/judge/api/openapi.yaml` 0.3.0)

- `SpeedFactor` に `ability`・`item` を足す。`*SpeedApplied` の並びは rank → tailwind → ability → choiceScarf / item → paralysis。
- `*SpeedIgnored` は「指定されていて、効き方を確定できない」ときだけ(表が無い・ID が表に無い・その ID の効果が不正・判定できない条件がある)。
  素早さ効果が無いと確定した ID と、条件が不成立と確定した ID は出さない。`fieldWeather` は天候があり、かつ `abilityId` が Ignored に入るときだけ。
- `SpeedFactor` の値の追加は、厳格な enum で読む iOS の生成物に再生成を要する(iOS レーンへ連絡。judge 側では契約の版 0.3.0 で表す)。

### 5. 実装の境界

judge コア(`internal/judge`)は ID を見ない。`Individual` に `Ability`・`Item`・`Env` を持たせ、`ResolveSpeedMod`・`SpeedEnvironment.Holds`
で評価する。`SpeedEffects` を渡さない(nil の)ときの結果は第1段と完全に同じ。engine は変更しない(`engine.SpeedMod`・
`SpeedCondition`・`MaxEffectModifier` を参照するだけ)。

### 6. デプロイ順

(1) calc-svc・Web(WASM と判定画面の i18n)→ (2) master-release で再取り込み → (3) judge。新しい judge は「マスタにあって `SpeedMods` の無い ID」を
「効果なしと確定」にするので、再取り込み前(どの特性・持ち物にも `SpeedMods` が無い)に出すと、「雨で ×2」の特性でも ×2 が掛からないのに
Ignored も空になり、全て反映済みに見える誤応答になる(第1段より悪化)。Web より先に出すと `speedFactorLabel` の ability・item が未定義で文言が空になる。
古い judge はマスタを読まないので、先に再取り込みしても安全。

### 7. データへの前提

- 「効果なし確定」はマスタの素早さの節の網羅性に依存する。将来、素早さに効く ID が追加されたのにデータが無いと、警告なしで確定になる。
- 最上位・項目のキーは上流の正準形(`SpeedMods`・`IgnoresParalysisSpeedDrop`・`Condition`・`Modifier`)を前提にする。大文字小文字違いは読まれず、効果なし確定になる
  (要素内のキーの違いは不正としてその ID が確定できない)。

## 更新した過去の決定

- **ADR-0710 の Ignored の意味**: 「指定されたが素早さに掛けていない(効果が無い特性でも出る)」を、「指定されていて素早さへの効き方を
  確定できない」に更新する。効果が無いと確定したもの・条件が不成立と確定したものは出ない(仕様変更)。データが引けないときの応答は第1段と同じ。
  `IgnoredSpeedInputs` は第1段の規則として残し、`IgnoredSpeedInputsFor` が新しい規則を持つ。
- **ADR-0712 §5 の既知の差の解消**: まひと特性が同時のときにクイックフィート相当の特性でも半減していた差を、`IgnoresParalysisSpeedDrop` で解消する。
- **ADR-0700 の却下案との違い**: ADR-0700 は「pokedex-svc の内部 API でマスタ一式を起動時に取る」を、メモリと鮮度の管理を背負うこと・
  read model を持たない前提を理由に却下した。本 ADR が持つのは特性・持ち物の素早さ効果だけの小表で(一式は一時的に読むが、種族・技・相性表は保持しない)、
  リクエストに特性・持ち物があるときだけ遅延で取り、TTL で更新する。補助データで、欠けたことは `*SpeedIgnored` で応答の中に表せるので、
  ADR-0700 §3 の「4 区分に畳む」(取得失敗を 503 にする)対象にせず、フェイルソフトにできる。種族・技の取得は従来どおり 1 リクエストごと。

## 却下した案

- **特性・持ち物の ID を judge に書く**: CLAUDE.md のハードコード禁止。マスタの効果データで評価する。
- **取得失敗を 503 にする**: 素早さの特性・持ち物は補助で、欠けても残りの判定(スカーフ・ダメージ・優先度)は正しい。Ignored で表せる。
- **起動時にマスタ一式を取る**: 上の ADR-0700 の却下理由のまま。小表を遅延ロードにする。
- **不成立・効果なしも Ignored に出す(第1段の規則のまま)**: データで確定できたものまで「反映していない」と言うのは誤解を招く。

## テストの期待値

素早さの期待値は @smogon/calc 0.12.0(完全固定)の Champions 世代と照合した。手順(コミットしない使い捨てスクリプトで行う):

1. `tools/golden` で `npm ci`。
2. `Generations.get(0)` で世代を取る。
3. `new Pokemon(gen, 種族, { ability, item, status, boosts })` で個体を作る(`abilityOn` などの条件は与えない)。
4. `rawStats.spe` を実数値(例 91・93。奇数にして丸めの順を区別する)で上書きする。
5. `getFinalSpeed(gen, pokemon, field, side)` を呼ぶ(追い風は `side.isTailwind`、天候・場は `field` で与える)。

テスト(`speed_effects_test.go`・`outspeed_speed_effects_test.go`)の効果データは架空の倍率・条件の組で、実在の特性・持ち物の ID は書かない
(実在の名前は使い捨てスクリプトにだけ書く)。連結してから 1 回丸める(93 × ×1.5 × ×0.5 → 70。補正ごとに丸めると 69)、まひを最後に掛ける、
まひの半減を受けない特性、スカーフ優先を、原典の値と一致する数値で固定した。
