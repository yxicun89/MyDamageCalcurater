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

## TB0 の未完了ブロッカー

2026-09-21 時点で、Docker build、k3d への直接 deploy、Pod Ready、smoke、GitOps template検査は確認済み。
GitOps専用overlay、digest固定、private repository/registryの秘密をGitへ入れない手順も用意した。
一方、実Git remote、取得可能な配布イメージの置き場所、Argo CD Application CRD が未設定のため、
`Git変更 → Argo CD manual sync → Pod更新` は未実施であり、TB0 全体は完了扱いにしない。
実施には利用する Git repository URL、image registry/repository、不変 image tag または digest、
およびversion固定したArgo CDの導入とprivate repository/registry credentialのクラスタ登録が必要である。

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
