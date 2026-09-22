# タイプバランスチェッカー テスト戦略

## TB0 の対象

TB0 は、型・タイプ相性コア・差し替え可能なマスタ境界・HTTP 最小疎通・コンテナ・
Kubernetes/Argo CD 定義を検証する。チーム集計、攻撃範囲、特性反映は TB1 以降とする。

## レイヤー

| レイヤー | 対象 | 合格条件 |
|---|---|---|
| Unit | `internal/balance` | 18タイプ、整数倍率、単/複合タイプ、入力エラー、`EffectSource` |
| Adapter | temporary static type chart | 18×18 を返し、代表的な弱点・耐性・無効が既存表と一致 |
| Contract/HTTP | service-local OpenAPI / health / analyze stub | 生成型を使用。health は200。端末ID/セッションID欠落は400。有効な疎通は501(TB1 で 200 に置換) |
| Build | Go / Docker / Kustomize | `go test`、`go vet`、`go build`、Docker build、overlay build が成功 |
| Smoke | k3d | Pod Ready、health 200、Ingress経由 analyze 501(TB1 で 200・422 に置換) |

## TB1 の対象(防御タイプバランス)

契約の正は [ADR-0014](adr/0014-balance-tb1-defense-analysis.md)。最大6体の各メンバーについて18攻撃タイプごとの
防御倍率・6分類・`source` を返し、攻撃タイプごとにチーム集計(`weak` / `quadWeak` / `resist` / `immune` / `neutral`)を返す。
総合点・ランキング・独自スコアは作らないため、それを検証するテストも置かない。テストは実装より先に書く(spec 先行)。

| レイヤー | 対象 | 合格条件 |
|---|---|---|
| Unit | `internal/balance`(`ClassifyMultiplier` / `AnalyzeDefense` / `ResolveMembers`) | 16/8/4/2/1/0 → `quad_weak`/`weak`/`neutral`/`resist`/`quad_resist`/`immune`、それ以外の値はエラー。各メンバー18件・正準順・`source=type`。members は入力順。単/複合タイプ、4倍弱点、1/4耐性、無効。集計は `quadWeak ⊂ weak`、`resist` は無効を含まない、全18タイプで `weak+resist+immune+neutral = メンバー数`、メンバー結果から数え直した値と一致。同一 `pokemonId` の重複はそのまま数える。1体・6体。0体/7体・不正タイプ・タイプ0/3個・重複タイプ・chart nil はエラー。temporary chart で単/複合171通りすべてが `CalculateDefense` と一致。未登録IDは `ErrUnknownPokemon` を wrap |
| Adapter | `internal/master` の JSON read model ローダ | `io.Reader` 版とパス版。正常系を読める。`schemaVersion≠1`・ID 形式不正・ID 重複・タイプ0/3個・不正タイプ(大文字含む)・タイプ重複・未知フィールド・空/壊れた/後続付き JSON はすべて `ErrInvalidPokemonTypes` で、部分的な model を返さない。存在しないパスはエラー。未登録IDは `balance.ErrUnknownPokemon`。返すスライスを書き換えても read model が変わらない。Git の example は架空ID(9001-000 以降)だけで、単/複合・4倍弱点・無効を含む |
| Config | `cmd/api` の `BALANCE_POKEMON_TYPES_PATH` 読み込み | 未設定なら provider なし(型付き nil ではない nil interface)でエラーなし。example を指せば読める。不正/存在しないファイルはエラー(main は起動失敗) |
| Contract/HTTP | service-local OpenAPI 0.2.0 / analyze | 生成型 `api.AnalyzeResponse` へ未知フィールド禁止で decode できる 200。members 順序・types・18件の正準順 `defense`・倍率は文字列 enum・`category`・`source="type"`・18件の `teamSummary` と不変条件。未登録ID → 422 `unknown_pokemon`。provider 未設定 → 503 `master_unavailable`(health は 200、ヘッダー欠落は 400 のまま)。TB0 の 400/413 と境界(6体・16 KiB ちょうど)は維持し、境界の成功は 200 |
| Build | Go / Docker / Kustomize | `go test`、`go vet`、`go build`、Docker build、overlay build が成功 |
| Smoke | k3d(local overlay) | local overlay が `testdata/pokemon-types.example.json` を ConfigMap でマウントし `BALANCE_POKEMON_TYPES_PATH` を設定する。Pod Ready、health 200、Ingress 経由で架空ID の analyze が 200(`members`/`teamSummary`/`quadWeak`/`source:"type"` を含む)、未登録ID が 422 `unknown_pokemon` |

TB0 の「有効な疎通は501」「Ingress経由 analyze 501」は TB1 で上の 200 に置き換えた(弱めたのではなく、未実装の暫定応答を実応答の検証へ強めた)。
テストの `pokemonId` は架空ID(9001-000 以降)を使う。実在ポケモンの ID・名前・タイプの組をテストデータとして Git に置かない(ADR-0002)。

## マスタデータの扱い

TB0 の静的タイプ相性表は開発継続用の temporary adapter で、恒久正本ではない。
純粋コアは provider interface だけを参照し、ADR-0012 と共通マスタ確定後に adapter を差し替える。
テストは temporary であることを隠さず、データ出典を `services/balance/README.md` に記録する。

## 実行コマンド

ルートの `make test` / `make build` / `make lint` の vet は、入れ子のモジュールである `services/balance` を対象にしない
(ルート Makefile の `include` で balance のターゲットを呼べるだけ)。このレーンの完了確認では、下の `balance-*` を必ず実行する。

```sh
make -f services/balance/Makefile balance-gen
make -f services/balance/Makefile balance-test
make -f services/balance/Makefile balance-lint
make -f services/balance/Makefile balance-build
make -f services/balance/Makefile balance-kustomize
make -f services/balance/Makefile balance-gitops-template-check
make -f services/balance/Makefile balance-docker-build
make -f services/balance/Makefile balance-k3d-deploy
make -f services/balance/Makefile balance-smoke
```

Docker/k3d/Argo CD を実行できない場合は、未実施理由と再実行コマンドを記録し、成功扱いにしない。

## TB0 の GitOps 検証(ADR-0018)

ローカル k3d に Argo CD v3.5.3 とクラスタ内レジストリを入れ、`Git 変更(gitops overlay の digest)→ PR で main → manual sync → Pod 更新` を確認する。
合格条件: Argo CD の Application が Synced / Healthy、同期 revision が main の該当 commit、稼働 Pod の image が overlay の digest と一致、health 200。
gitops overlay には read model のマウントが無いので analyze / coverage は 503 が正しい(smoke の analyze 以降は local overlay で確認する)。
`check-gitops.sh` は template・ready の両モードで application.yaml の repoURL が placeholder のままであることを必須にし(実 URL を Git に入れない)、ready では `BALANCE_GITOPS_REPO_URL`(適用時の repoURL)を検査する。`make balance-argocd-app`(`scripts/argocd-local-app.sh`)が値を渡して `check-gitops.sh ready` を呼び、置換が1か所で placeholder が残っていないことを確かめてから適用する。

## TB1b の対象(相性表のデータ化。ADR-0015)

| レイヤー | 対象 | 合格条件 |
|---|---|---|
| Adapter | `master.LoadTypeChart` / `EmbeddedTypeChart` | 同梱の複製が `testdata/golden/typechart.json` とバイト一致。代表的な弱点・耐性・無効。省略は等倍。schemaVersion・タイプ集合(過不足・重複)・未知キー・不正コード・未知フィールド・後続 JSON を `ErrInvalidTypeChart` で拒否 |
| Unit | `internal/balance` | 171 通りの単/複合タイプで `AnalyzeDefense` と `CalculateDefense` が一致(データの表で) |

TB0 の `TemporaryTypeChart` のテストは、コードごと削除した(削除前にデータの表と 18×18 全件一致を確認)。

## TB2 の対象(攻撃範囲。ADR-0016)

契約の正は [ADR-0016](adr/0016-balance-tb2-offense-coverage.md)(§1 はユーザー回答)。`POST /api/balance/v1/team-balance/coverage` は
メンバーごとに技 ID を 0〜4 個受け取り、変化技を除いた技のタイプ(`attackTypes`)と、18 の単タイプの防御側それぞれに対する
`bestMultiplier`(`"0" "1/2" "1" "2"`、攻撃技なしは `null`)・`effective`(×1 以上)・`superEffective`(×2)を返し、防御タイプごとに
メンバー単位のチーム集計(`effectiveMembers` / `superEffectiveMembers`)を返す。複合タイプの防御側・STAB・特性・特殊な技は範囲外(TB4 / TB3 以降)。
テストは実装より先に書いた(spec 先行)。倍率の期待値は同梱の相性表(`master.EmbeddedTypeChart()`)で成り立つ組だけを使う。

### 受け入れ条件

1. 変化技(`status`)は `attackTypes` と倍率の計算に入らない。`moveIds` は request の順のまま(変化技も含めて)返す。
2. `attackTypes` は重複なし・正準順(normal … fairy)。同じタイプの技を複数持っても1回。
3. `coverage` と `teamCoverage` は 18 件・正準順。`bestMultiplier` は攻撃技のタイプの倍率の最大値。攻撃技が無いメンバー(技 0 個・変化技のみ)は全件 `null`・false。
   ×0 と ×1/2 は `effective` でなく、×1 は `effective` だけ、×2 は両方。
4. チーム集計はメンバー単位(技の数ではない)。`superEffectiveMembers ≤ effectiveMembers ≤ メンバー数`。チームの `bestMultiplier` は全メンバーの最大で、攻撃技を持つメンバーがいなければ `null`。
5. 判定順: ヘッダー 400 → body 400/413 → ポケモンまたは技の read model 未設定 503 → `unknown_pokemon` 422 → `unknown_move` 422(message `unknown moveId: <ID>`)→ 200。その他の内部エラーは 500 固定文言 `internal error`。
6. request の検証(400): メンバー 1〜6、pokemonId の形式、moveIds 5 件以上、同じメンバー内の重複、moveId の形式(`^[a-z0-9]+(-[a-z0-9]+)*$`・最大 40 文字)、未知フィールド・後続 JSON。別メンバー間の同じ moveId・同じ pokemonId は許す。
7. 技の read model(`BALANCE_MOVES_PATH`)は起動時に1回読み、不正なら起動失敗。未設定・空文字は provider なし(型付き nil ではない nil interface)。Git には架空 ID(move-9001 以降)の example だけ。

| レイヤー | 対象 | 合格条件 |
|---|---|---|
| Unit | `internal/balance`(`MoveCategory` / `ResolveMoves` / `UnknownMoveError` / `AnalyzeCoverage`) | 受け入れ条件 1〜4。`ResolveMoves` は順序維持・未登録は ID だけを持つ `*UnknownMoveError`(`Unwrap` は `ErrUnknownMove`、`Error()` は `unknown move: <ID>`)・provider nil は `ErrNilMoves`・その他の失敗は伝播(unknown にしない)。`AnalyzeCoverage` の入力エラー: 0/7 体は `ErrMemberCount`、技 5 個は `ErrMoveCount`、メンバー内の同じ moveId は `ErrDuplicateMove`、攻撃技の不正タイプは `ErrInvalidType`、不正分類は `ErrInvalidMoveCategory`、chart nil は `ErrNilTypeChart`、chart のエラーは伝播、単タイプとしてありえない倍率はエラー。データの表で単タイプ 18 + 2 タイプ 153 通りの攻撃側を、`Matchup` から作った oracle と全件照合 |
| Adapter | `internal/master` の技 read model(`LoadMoves` / `LoadMovesFile`) | 正常系(40 文字の ID を含む)。`schemaVersion≠1`・欠落・文字列、`moves` の欠落/null/空、moveId の欠落・空・大文字・`_`・空白・先頭/末尾/連続ハイフン・41 文字・重複、type の欠落・空・未知・大文字・配列、category の欠落・空・未知・大文字、未知フィールド、空/壊れた/null/後続付き JSON はすべて `ErrInvalidMoves` で部分 model を返さない。存在しないパス・ディレクトリはエラー。未登録 ID は `balance.ErrUnknownMove`(zero の Move)。example は架空 ID のみで physical/special/status・同タイプの攻撃技 2 個以上・×0 と ×2 の組を含み、local overlay の複製とバイト一致 |
| Config | `cmd/api` の `BALANCE_MOVES_PATH` 読み込み(`moveProviderFromEnv`) | 未設定・空文字は nil interface でエラーなし。example を読める。不正/存在しないファイル・ディレクトリはエラー(main は起動失敗) |
| Contract/HTTP | service-local OpenAPI 0.3.0 / coverage | 生成型 `api.CoverageResponse` へ未知フィールド禁止で decode できる 200 と内容。`moveIds` / `attackTypes` は空でも `[]`(null でない)、`bestMultiplier` は文字列か明示的な `null`。受け入れ条件 5・6 の各ケース、境界(6 体×4 技、40 文字の moveId、技 0 個)の成功。技の read model が無くても health 200・analyze 200。analyze の既存テストは変更なし |
| Smoke | k3d(local overlay) | local overlay が `testdata/moves.example.json` の複製を ConfigMap でマウントし `BALANCE_MOVES_PATH` を設定する。Ingress 経由で coverage が 200(`teamCoverage`・`"attackTypes":["fire"]` 等を含む)、未登録の技が 422 `unknown_move` |

テストの技 ID は架空(move-9001 以降)を使う。実在の技の名前・ID をテストデータとして Git に置かない(ADR-0002)。

## TB3 の対象(特性による防御相性の変化。ADR-0017)

契約の正は [ADR-0017](adr/0017-balance-tb3-ability-effects.md)(§1 はユーザー回答、§2〜4 は採用済みの既定案)。analyze のメンバーに任意の
`abilityId` を受け取り、特性の read model(`BALANCE_ABILITIES_PATH`)の正規化された効果(`immune` / `absorb` / `type_multiplier` /
`super_effective_multiplier`)をタイプ相性の後に適用する。倍率は既約分数(float を使わない)。coverage(TB2)は変えない。
テストは実装より先に書いた(spec 先行)。HTTP の倍率の期待値は同梱の相性表で成り立つ組だけを使う。

### 受け入れ条件

1. `abilityId` を指定しないメンバーは TB1 と完全に同じ結果(171 通りの防御タイプで `CalculateDefense` と一致、倍率は TB1 の6値、
   `source=type`、×0 を含むすべてが `effect=none`)。特性の read model が無くても 200 で、read model の有無で body が変わらない。効果が空の特性も同じ結果。
2. 計算順: タイプ相性 → タイプ由来の ×0 はそこで確定(`source=type`)→ 特性の効果。`immune` / `absorb` は ×0(`source=ability`、`effect` は
   `immune` / `absorb`)。`type_multiplier` は該当タイプだけ、`super_effective_multiplier` はタイプ相性が ×1 より大きいときだけ掛ける。
   値が変わらなければ(打ち消し合い・×1 の係数を含む)`source=type`・`effect=none`、変われば `source=ability`・`effect=multiplier`。
3. 倍率の文字列は既約分数(分母 1 は整数だけ。`"0" "1/4" "3/4" "5/4" "3/2" "5/2" "3"` など)。category は範囲で決める:
   0 → `immune`、(0, 1/4] → `quad_resist`、(1/4, 1) → `resist`、1 → `neutral`、(1, 4) → `weak`、[4, ∞) → `quad_weak`。
4. チーム集計の定義(ADR-0014 §3)は変えない。特性の無効・吸収は `immune` に数える。全18タイプで `weak+resist+immune+neutral = メンバー数`、
   メンバーの category から数え直した値と一致。
5. response のメンバーの `abilityId` は指定したときだけ(無ければキー自体を省略)。
6. 判定順: ヘッダー 400 → body 400/413(`abilityId` の形式 `^[a-z0-9]+(-[a-z0-9]+)*$`・1〜40 文字・文字列)→ ポケモンの read model 未設定、
   または `abilityId` を指定したメンバーがいるのに特性の read model 未設定 503 → `unknown_pokemon` 422 → `unknown_ability` 422
   (message `unknown abilityId: <ID>`、adapter の詳細を含めない)→ 200。それ以外(provider の想定外の失敗・不正な効果)は 500 固定文言 `internal error`。
7. 特性の read model は起動時に1回読み、ADR-0017 §2 の検証に1つでも反すれば起動失敗。未設定・空文字は provider なし(型付き nil ではない nil interface)。
   Git には架空 ID(ability-9001 以降)の example だけ。

| レイヤー | 対象 | 合格条件 |
|---|---|---|
| Unit | `internal/balance`(`Effectiveness` / `ClassifyEffectiveness` / `CalculateDefenseWithAbility` / `AnalyzeDefense` / `ResolveAbility` / `UnknownAbilityError`) | `NewEffectiveness` は約分し 0 は `{0,1}`、分母 0 以下・負の分子は `ErrInvalidEffectiveness`。`Mul`(可換・約分)・`Cmp`・`IsZero`・`String`。TB0 の6値の変換と TB1 ラベルの一致。category の境界(0、1/64、3/16、1/4、5/16、3/4、15/16、1、17/16、5/4、3/2、3、63/16、4、5、8、16)。受け入れ条件 1〜4 の計算(単独・複合の効果、効果の順序によらない結果、タイプ無効の優先)。不正な効果(未知の kind・不正な攻撃タイプ・分母 0)は `ErrInvalidAbilityEffect`。`ResolveAbility` は request の ID を返す・未登録は ID だけを持つ `*UnknownAbilityError`(`Error()` は `unknown ability: <ID>`、`Unwrap` は `ErrUnknownAbility` だけ)・provider nil は `ErrNilAbilities`・その他の失敗は伝播 |
| Adapter | `internal/master` の特性 read model(`LoadAbilities` / `LoadAbilitiesFile`) | 正常系(4 種の kind、空の effects、約分して保持、同じタイプへの `type_multiplier` の重複、別特性の同タイプ、40 文字の ID)。`schemaVersion`、`abilities` の欠落/null/空、abilityId の形式・41 文字・重複・数値、effects の欠落/null/オブジェクト、kind の欠落・空・未知・大文字、kind ごとの必須項目の欠落と余分な項目、attackType の不正、numerator/denominator の 0・17・負・小数・文字列・null、同じ特性内の同じ攻撃タイプへの `immune` / `absorb` の重複、未知フィールド、空/壊れた/null/後続付き JSON はすべて `ErrInvalidAbilities` で部分 model を返さない。存在しないパス・ディレクトリはエラー。未登録は `balance.ErrUnknownAbility`(zero の Ability)。返した effects を書き換えても model は変わらない。example は架空 ID のみで 4 種の kind と空の effects を含み、local overlay の複製とバイト一致 |
| Config | `cmd/api` の `BALANCE_ABILITIES_PATH` 読み込み(`abilityProviderFromEnv`) | 未設定・空文字は nil interface でエラーなし。example を読める。不正・空・存在しないファイル・ディレクトリはエラー(main は起動失敗) |
| Contract/HTTP | service-local OpenAPI 0.4.0 / analyze | 生成型 `api.AnalyzeResponse` へ未知フィールド禁止で decode できる 200。全 entry で倍率が pattern に合い既約、category が範囲と一致、`source` と `effect` の組が整合、集計が数え直しと一致。受け入れ条件 1・5・6 の各ケース。境界(40 文字の abilityId、6 体すべて同じ abilityId)の成功。coverage に `abilityId` を送ると 400(TB2 の契約は不変)。TB1 の analyze テストは期待値を変えない(`DefenseMultiplier` が enum でなくなったため、生成定数と `Valid()` を TB1 の6値の集合での判定に置き換えただけ) |
| Smoke | k3d(local overlay) | local overlay が `testdata/abilities.example.json` の複製を ConfigMap でマウントし `BALANCE_ABILITIES_PATH` を設定する。Ingress 経由で abilityId 付きの analyze が 200(`"effect":"absorb"`・`"multiplier":"3"`・`"source":"ability"` 等を含む)、未登録の特性が 422 `unknown_ability` |

テストの特性 ID は架空(ability-9001 以降)を使う。実在の特性の名前・ID をテストデータとして Git に置かない(ADR-0002)。
タイプ由来の ×0 は `effect=none`・`source=type`(ADR-0017 §5.1・§5.5)で、テストはこれを厳密に確かめる。`"abilityId": null` は省略と同じ(§5.2。read model の有無によらず 200、body は省略時と一致)。
倍率の積が int64 に収まらないデータ(効果を多数重ねた特性)は、黙って折り返さず `ErrEffectivenessOverflow` → HTTP 500(ADR-0017 §5.6)。`Mul` はオーバーフローの境界、`Cmp` は 2^62 のような大きな値で検査する。

## TB4 の対象(仮想敵診断。ADR-0400)

契約の正は [ADR-0400](adr/0400-balance-tb4-threat-check.md)(§1 はユーザー回答)。`POST /api/balance/v1/team-balance/threats` は
自分のメンバー(1〜6)と仮想敵(1〜6)を同じ形 `{pokemonId, moveIds(0〜4), abilityId(任意)}` で受け取り、仮想敵ごとに
`attackTypes`・メンバーごとの `incoming`(受ける最大倍率)/ `outgoing`(与える最大倍率)/ `safe` / `superEffective`・
`safeMembers` / `superEffectiveMembers` を返す。計算は TB1〜3 の `CalculateDefenseWithAbility` の再利用で、新しい read model は無い。
STAB・持ち物・威力・天候・場は扱わないので、それを検証するテストも置かない。テストは実装より先に書いた(spec 先行)。
倍率の期待値は同梱の相性表(`master.EmbeddedTypeChart()`)で成り立つ組だけを使う。

### 受け入れ条件

1. `incoming` は仮想敵の攻撃技(変化技を除く)のタイプごとに、メンバーのタイプと**メンバーの特性**で求めた倍率の最大。
   `outgoing` はメンバーの攻撃技のタイプごとに、仮想敵のタイプと**仮想敵の特性**で求めた倍率の最大。攻撃側の特性は自分の攻撃に掛からない。
2. 倍率は既約分数の文字列(`"0" "1/4" "1/2" "3/4" "1" "5/4" "3/2" "2" "3" "4"` など。分母 1 は整数だけ)。攻撃技が無い側の倍率は `null`(JSON で明示的な `null`)。
3. `safe` は `incoming < 1`(×0・×1/4・×1/2・特性の ×3/4・無効・吸収)、`superEffective` は `outgoing ≥ 2`(×2・×3・×4)。
   ×1 は両方 false、×3/2(×2 に ×3/4)と ×5/4 は false。`null` のときは false。
4. `safeMembers` / `superEffectiveMembers` は仮想敵ごとの `safe` / `superEffective` の人数。
5. `attackTypes` は仮想敵の攻撃技のタイプ(重複なし・正準順。変化技のみ・技なしは `[]`)。`threats` は request の順、`matchups` はメンバーの request の順(同じ pokemonId の重複もそのまま)。
   仮想敵の `abilityId` は指定したときだけ(無ければキーを省略)。
6. 判定順: ヘッダー 400 → body 400/413(メンバー・仮想敵とも 1〜6、moveIds 0〜4・重複・形式、abilityId の形式、pokemonId の形式、未知フィールド・後続 JSON)→
   ポケモンの read model 未設定、または moveId が1つでもあるのに技の read model 未設定、または abilityId が1つでもあるのに特性の read model 未設定(503)→
   `unknown_pokemon` → `unknown_move` → `unknown_ability`(422。それぞれ members → threats・request の順で最初のもの。message は `unknown pokemonId: <ID>` 等で adapter の詳細を含めない)→ 200。
   それ以外(provider の想定外の失敗・chart nil・不正な特性効果・倍率の積のオーバーフロー)は 500 固定文言 `internal error`。

| レイヤー | 対象 | 合格条件 |
|---|---|---|
| Unit | `internal/balance`(`AnalyzeThreats` / `Combatant` / `ThreatMatchup` / `ThreatResult`) | 受け入れ条件 1〜5。境界(×0・×1/4・×1/2・×1・×2・×4、特性の ×3/4 で等倍未満・×5/4・×1/2 でちょうど ×1・×3/4 で ×3/2 と ×3、特性の無効・吸収)。すべての結果で `safe` / `superEffective` が倍率から、人数が matchups から数え直した値と一致し、倍率が既約。171 通りの防御タイプ × 18 攻撃タイプで `incoming` / `outgoing` が `CalculateDefenseWithAbility` と一致。メンバー・仮想敵それぞれ 0/7 件は `ErrMemberCount` / `ErrThreatCount`、1 件と 6 件は成功。chart nil は `ErrNilTypeChart`、chart のエラーは伝播、単タイプとしてありえない倍率はエラー。倍率の積のオーバーフローは incoming・outgoing のどちらでも `ErrEffectivenessOverflow` |
| Contract/HTTP | service-local OpenAPI 0.5.0 / threats | 生成型 `api.ThreatsResponse` へ未知フィールド禁止で decode できる 200 と内容(4 体の仮想敵 × 3 メンバーの全 matchups)。`attackTypes` は空でも `[]`、倍率は文字列か明示的な `null`、仮想敵の `abilityId` は指定時だけ。受け入れ条件 6 の各ケースと境界(6 × 6・4 技・40 文字の moveId / abilityId、技 0 個、メンバーと仮想敵で同じ moveId)。moveIds が全員空・abilityId なしなら技・特性の read model が無くても 200。analyze・coverage・health の既存テストは変更なし(threats の追加で影響しないことも確認) |
| Smoke | k3d(local overlay) | 既存の3つの example read model のマウントのまま、Ingress 経由で threats が 200(`"incoming":"2"`・`"outgoing":null`・`"superEffectiveMembers":1` 等を含む)、未登録の技が 422 `unknown_move` |

テストの ID は架空(9001-000 / move-9001 / ability-9001 以降)を使う。実在ポケモン・技・特性の名前・ID を Git に置かない(ADR-0002)。

## TB5 の対象(おすすめタイプと該当ポケモン。ADR-0401)

契約の正は [ADR-0401](adr/0401-balance-tb5-recommend-types.md)(§1 はユーザー回答、§2〜6 は採用済みの既定案)。
`POST /api/balance/v1/team-balance/recommendations` はメンバー(1〜6、`{pokemonId, moveIds(0〜4), abilityId?}`)と任意の `limit`(1〜20、既定 10)を受け取り、
防御の穴・攻撃範囲の穴・おすすめタイプの候補(171 通りから)と各候補のタイプを持つポケモン全員・特性で穴をふさげるポケモン(別枠)を返す。
ポケモンの read model に省略可能な `nameJa`・`abilityIds` を足す(`schemaVersion` は 1 のまま)。read model に入っているポケモンを使用可能とみなし、
レギュレーションの絞り込みはしない(read model を出力する側の責務)ので、それを検証するテストも置かない。テストは実装より先に書いた(spec 先行)。
純粋コアと HTTP の順位の期待値は、手で数えられる架空の相性表(既定 ×1)で作り、同梱の相性表では独立に組んだ oracle(総当たり)と上位 20 件を照合する。

### 受け入れ条件

1. 防御の穴は、メンバーの特性を反映した TB1 の集計で `resist + immune = 0` の攻撃タイプ(正準順)。×5/4 の特性や等倍への ×3/4 は穴を消さない。
2. 攻撃範囲の穴は、TB2 のチーム集計で `bestMultiplier` が ×1 未満の防御タイプ(正準順)。技が1つも無いチームでは穴を出さない(`offenseHoles` は `[]`)。
3. 候補ごとに `defenseCovered`(防御の穴のうち候補のタイプだけで ×1 未満)、`offenseCovered`(攻撃範囲の穴のうち候補のどちらかのタイプで ×1 以上)、
   `weaknesses`(×2 以上の攻撃タイプの数)を返す。穴を1つもふさがない候補は出さない。候補の特性は考えない。
4. 並びは `defenseCovered + offenseCovered` の多い順 → `weaknesses` の少ない順 → 正準順(単タイプがすべての複合より先、複合は1つ目・2つ目の正準順)。
   上位 `limit` 件(既定 10、1〜20)。候補が `limit` より少なければ全件。
5. 候補の `pokemon` は、複合タイプの候補ならタイプの集合が一致する(順不同)ポケモン全員、単タイプの候補ならそのタイプを含むポケモン全員(もう片方のタイプで候補の `defenseCovered` を等倍未満で受けられなくなるものは除く。ADR-0401 §8)。`exactMatch` が真のもの → `pokemonId` の昇順。`types` は read model の順のまま、
   `nameJa` は read model にあるときだけ(無ければキーを省略)。該当が無ければ `[]`。
6. `abilityOptions` は防御の穴ごと(正準順)に、`abilityIds` の特性のどれかで ×1 未満にできる(タイプだけでは ×1 以上の)ポケモンを、
   ポケモンと特性の組で `pokemonId` の昇順に(`multiplier` は特性込みの既約分数)。特性の read model が無ければ `[]` で 200(503 にしない)で、ほかの結果は変わらない。
7. 判定順は TB4 と同じ流儀: ヘッダー 400 → body 400/413(メンバー 1〜6、pokemonId・moveIds・abilityId の形式、`limit` の範囲と型、未知フィールド・後続 JSON)→
   ポケモンの read model(型の provider または一覧の catalog)未設定、または moveId があるのに技、abilityId があるのに特性の read model 未設定(503)→
   `unknown_pokemon` → `unknown_move` → `unknown_ability`(422、request 順で最初のもの)→ 200。それ以外は 500 固定文言 `internal error`。
8. ポケモンの read model の `nameJa`(1〜64 文字)・`abilityIds`(0〜4 件・重複なし・ADR-0017 §2 の ID 形式)は省略可能。不正なら `ErrInvalidPokemonTypes` で起動失敗。
   既存の v1 ファイル(項目なし)はそのまま読める。

| レイヤー | 対象 | 合格条件 |
|---|---|---|
| Unit | `internal/balance`(`RecommendTypes` / `PokemonCatalog` / `CatalogPokemon` / `TypeCandidate` / `AbilityOption`) | 受け入れ条件 1〜6(`recommend_test.go`)。防御の穴(特性の無効・吸収・×1/2・×5/4・効果なし、複数メンバー)、攻撃範囲の穴(1体の複数技、複数メンバー、技なし)、手で数えた並び(同点の weaknesses・正準順・単タイプ優先)、穴をふさがない候補の除外(18 件ちょうど)、limit(1・2・10・19・20 は上位の接頭辞、0・21・-1 は `ErrRecommendationLimit`)、該当ポケモン(順不同一致・昇順・nameJa・read model の順)、特性の別枠(タイプだけで受けられるものの除外、×1 ちょうどは入らない、1体の2特性は2組)。provider なしは `AbilityOptions` 空。入力エラー: 0/7 体は `ErrMemberCount`、chart nil は `ErrNilTypeChart`、chart の失敗は伝播、不正タイプは `ErrInvalidType`、不正な特性効果は `ErrInvalidAbilityEffect`、不正な技分類は `ErrInvalidMoveCategory`、catalog の特性の解決失敗は伝播。同梱の相性表で3チームを oracle と上位 20 件照合 |
| Adapter | `internal/master` のポケモン read model(`LoadPokemonTypes` / `AllPokemon`) | 受け入れ条件 8(`pokemon_catalog_test.go`)。64 文字の nameJa・40 文字の abilityId・4 件・空配列を読める。`AllPokemon` は全件を `pokemonId` の昇順で、返した値を書き換えても read model は変わらない。nameJa の空・65 文字・数値・配列、abilityIds の 5 件・重複・空・大文字・`_`・連続/末尾ハイフン・空白・41 文字・数値・文字列、別の未知フィールドはすべて `ErrInvalidPokemonTypes` で部分 model を返さない。example は名前あり/なし・特性あり/なしを含み、`abilityIds` は特性の example に存在する ID だけ(local overlay の複製とのバイト一致は既存テスト) |
| Contract/HTTP | service-local OpenAPI 0.6.0 / recommendations | 生成型 `api.RecommendationsResponse` へ未知フィールド禁止で decode できる 200 と内容(穴・候補 10 件・該当ポケモン・特性の別枠)。空の配列は `[]`、`nameJa` は無ければキーを省略。メンバーの特性が防御の穴に効く。limit(省略=10・1・10・19・20、0・21・-1・1.5・文字列は 400)。特性・技の read model が不要なときは無くても 200。受け入れ条件 7 の各ケース、境界(6 体×4 技・40 文字の ID)、`abilityId: null`。health・analyze・coverage・threats は影響を受けない。example read model と同梱の相性表で smoke と同じ結果 |
| Smoke | k3d(local overlay) | 既存の3つの example read model のマウントのまま、Ingress 経由で recommendations が 200(`"offenseHoles":[]`・`"types":["steel","fairy"]`・`"nameJa":"テストメタル"`・`"abilityId":"ability-9002"` 等を含む) |

テストの ID・名前は架空(9001-000 / move-9001 / ability-9001 以降、`テスト…`)を使う。実在ポケモン・技・特性の名前・ID を Git に置かない(ADR-0002)。

## read model の JSON Schema(ADR-0402)

| レイヤー | 対象 | 合格条件 |
|---|---|---|
| Schema | `services/balance/schema/*.schema.json` | 各 example(`testdata/*.example.json`)と同梱の相性表(`internal/master/data/typechart.json`)が schema に合う。loader が拒否する代表的な入力(未知フィールド・schemaVersion 2・タイプ 3 個・不正な category / kind / code など)を schema も拒否する |
| HTTP | `httpapi.New` | typed nil の provider(nil のポインタ・関数など)は未設定と同じ(503。相性表は 500)。nil の slice / map は空の値として扱う |

