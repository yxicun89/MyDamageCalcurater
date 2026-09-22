# balance service

パーティのタイプ相性・弱点・耐性・攻撃範囲・仮想敵診断を分析するドメインモノリス。
damage-calc とは兄弟サービスで、互いの実行時 API には依存しない。

## TB1(防御タイプバランス)・TB2(攻撃範囲)・TB3(特性による防御相性の変化)・TB4(仮想敵診断)・TB5(おすすめタイプ)の範囲

- `internal/balance`: HTTP・DB・Kubernetes に依存しない型・防御相性コア・チーム集計(`AnalyzeDefense`)、
  技の解決コア(`ResolveMoves`)・攻撃範囲コア・チーム集計(`AnalyzeCoverage`)、特性の解決コア
  (`ResolveAbility`)・有理数の倍率(`Effectiveness`)・特性込みの防御計算(`CalculateDefenseWithAbility`)、
  TB1〜3 の計算を再利用する仮想敵診断コア(`AnalyzeThreats`)、および TB1〜3 の計算を再利用するおすすめ
  タイプコア(`RecommendTypes`)
- `api/openapi.yaml`: balance 外部 API 契約の正
- `internal/api`: oapi-codegen による生成型
- `internal/master`: 共通マスタへ差し替えるための adapter(タイプ相性表・ポケモンタイプ read model 兼
  カタログ(`balance.PokemonCatalog`)・技 read model・特性 read model)
- `internal/httpapi`: health・analyze・coverage・threats・recommendations の実装
- `cmd/api`: プロセス起動・`BALANCE_POKEMON_TYPES_PATH` / `BALANCE_MOVES_PATH` / `BALANCE_ABILITIES_PATH` の
  読み込み・graceful shutdown
- `schema/`: 4つの read model(ポケモンタイプ・技・特性・タイプ相性表)の JSON Schema(draft 2020-12)。
  データレーンの `pokedex export` が出す形の正。`schema/schema_test.go` が example とタイプ相性表を検証する
- `deploy`: balance 専用 Kustomize と manual-sync の Argo CD Application
- `DEPENDENCIES.md`: 公開前確認用の直接依存・利用理由・license

契約の正は TB1 が [ADR-0014](../../docs/adr/0014-balance-tb1-defense-analysis.md)、
TB2 が [ADR-0016](../../docs/adr/0016-balance-tb2-offense-coverage.md)、
TB3 が [ADR-0017](../../docs/adr/0017-balance-tb3-ability-effects.md)、
TB4 が [ADR-0400](../../docs/adr/0400-balance-tb4-threat-check.md)、
TB5 が [ADR-0401](../../docs/adr/0401-balance-tb5-recommend-types.md)。

タイプ相性表はコードに持たない(ADR-0013・ADR-0015)。ダメージ計算レーンの P1-13 でデータ化された
`testdata/golden/typechart.json` を `internal/master/data/typechart.json` にバイト複製して go:embed で同梱し、
`master.EmbeddedTypeChart` が起動時に検証して読む(不正なら起動失敗)。元ファイルとの一致はテスト
(`TestEmbeddedTypeChartMatchesSharedData`)が検査するので、元が更新されたら `make balance-sync-typechart` で複製し直す。
共通マスタ(P2-2)の相性表が別の形で配布されるようになったら、`balance.TypeChartProvider` の adapter だけを差し替える。

ポケモンのタイプは `internal/master.PokemonTypeReadModel`(`BALANCE_POKEMON_TYPES_PATH` が指す JSON を
起動時に1回だけ読む)から引く。これも temporary adapter で、共通マスタのスナップショット schema(P2-2)が
決まったら差し替える。Git に置くのは schema と架空データの example(`testdata/pokemon-types.example.json`、
ID は `9001-000` 以降)だけで、実 Pokémon マスタはコミットしない(ADR-0002)。同じ read model が TB5 の
カタログ(`balance.PokemonCatalog`)も兼ねる:各エントリは省略可能な `nameJa`(1〜64文字)と `abilityIds`
(0〜3件・重複なし、特性 read model と同じ ID 形式)を持てる(ADR-0401 §5、`schemaVersion` は 1 のまま)。

技のタイプ・分類(物理/特殊/変化)は `internal/master.MoveReadModel`(`BALANCE_MOVES_PATH` が指す JSON を
起動時に1回だけ読む)から引く。同じく temporary adapter。Git に置くのは schema と架空データの example
(`testdata/moves.example.json`、ID は `move-9001` 以降)だけ(ADR-0002・ADR-0016 §3)。

特性による防御相性の変化(`immune`/`absorb`/`type_multiplier`/`super_effective_multiplier`)は
`internal/master.AbilityReadModel`(`BALANCE_ABILITIES_PATH` が指す JSON を起動時に1回だけ読む)から引く。
同じく temporary adapter。倍率は既約分数(`balance.Effectiveness`、float は使わない)で持つ。Git に置くのは
schema と架空データの example(`testdata/abilities.example.json`、ID は `ability-9001` 以降)だけ
(ADR-0002・ADR-0017 §2)。

4つの read model すべての JSON Schema(draft 2020-12)を `schema/` に置く: `pokemon-types.schema.json`
(ADR-0014 §2・ADR-0401 §5)、`moves.schema.json`(ADR-0016 §3)、`abilities.schema.json`(ADR-0017 §2)、
`type-chart.schema.json`(ADR-0015)。データレーンの `pokedex export` はこの schema に合うことを確かめて出力する(形の意味の正は各 ADR で、schema はそれを機械で
確かめる形、各 loader(`internal/master`)は実装)。schema で表せない制約(特性の係数は既約でない比も受け付け、loader が約分して保持する・ID の重複禁止など)は
schema の `description` に書く。方針は ADR-0402。

## HTTP 契約

- `GET /healthz`: Pod probe。200 `{"status":"ok"}`
- `GET /api/balance/healthz`: Ingress 経由の smoke。200
- `POST /api/balance/v1/team-balance/analyze`: `X-Device-Id` と `X-Session-Id` が必須。判定順は
  ヘッダー(400)→ body(400/413)→ read model 未設定(503)→ pokemonId 解決(422)→ abilityId 解決(422)→ 200。
  - ヘッダー欠落: 400 `missing_request_context`
  - request は1〜6件の `{ "pokemonId": "NNNN-NNN", "abilityId"?: "..." }`。`abilityId` は任意で
    `^[a-z0-9]+(-[a-z0-9]+)*$` かつ1〜40文字。不正なら400 `invalid_request`
  - 16 KiB を超える request: 413 `request_too_large`
  - `BALANCE_POKEMON_TYPES_PATH` が未設定(または空文字)、あるいは `abilityId` を指定したメンバーがいるのに
    `BALANCE_ABILITIES_PATH` が未設定: 503 `master_unavailable`
  - 未登録の `pokemonId`: 422 `unknown_pokemon`(message に該当 ID を含む。全メンバーの pokemonId を先に解決する)
  - 未登録の `abilityId`: 422 `unknown_ability`(message に該当 ID を含む。pokemonId 解決後、メンバー順に解決する)
  - 上記以外の内部エラー(相性表や read model の想定外の失敗): 500 `internal_error`(固定文言。内部詳細は返さない)
  - 成功: 200。各メンバー(request順、`abilityId` は指定したときだけ)について18攻撃タイプ(正準順)の
    防御倍率(既約分数)・6分類・`source`(`type`/`ability`)・`effect`(`none`/`immune`/`absorb`/`multiplier`)、
    および攻撃タイプごとのチーム集計(`weak`/`quadWeak`/`resist`/`immune`/`neutral`。特性による無効・吸収も
    `immune` に数える)を返す。`abilityId` を指定しない request は TB1 と完全に同じ結果になる
- `POST /api/balance/v1/team-balance/coverage`(ADR-0016): `X-Device-Id` と `X-Session-Id` が必須。判定順は
  ヘッダー(400)→ body(400/413)→ ポケモンまたは技の read model 未設定(503)→ pokemonId 解決(422)→
  moveId 解決(422)→ 200。
  - request は1〜6件の `{ "pokemonId": "NNNN-NNN", "moveIds": [moveId, ...] }`。`moveIds` は0〜4件、
    各 moveId は `^[a-z0-9]+(-[a-z0-9]+)*$` かつ40文字以下、メンバー内で重複禁止。`moveIds` の欠落・
    `null` は400(空配列 `[]` は「攻撃技なし」として有効)
  - `BALANCE_POKEMON_TYPES_PATH` か `BALANCE_MOVES_PATH` のどちらかが未設定: 503 `master_unavailable`
  - 未登録の `pokemonId`: 422 `unknown_pokemon`(pokemonId を先に解決)、未登録の `moveId`: 422 `unknown_move`
  - 上記以外の内部エラー: 500 `internal_error`(固定文言)
  - 成功: 200。各メンバー(request順)について変化技を除いた技のタイプ(`attackTypes`、重複なし・正準順)と、
    18防御タイプ(正準順)ごとの最大倍率・`effective`(×1以上)・`superEffective`(×2)、および防御タイプごとの
    チーム集計(`effectiveMembers`/`superEffectiveMembers`、メンバー単位)を返す。攻撃技を持つメンバーが
    いない防御タイプの `bestMultiplier` は `null`
- `POST /api/balance/v1/team-balance/threats`(ADR-0400): `X-Device-Id` と `X-Session-Id` が必須。判定順は
  ヘッダー(400)→ body(400/413)→ ポケモン、または moveId を1つでも指定したのに技、または abilityId を
  1つでも指定したのに特性の read model 未設定(503)→ pokemonId 解決(422)→ moveId 解決(422)→
  abilityId 解決(422、いずれも members → threats・request 順で最初のもの)→ 200。
  - request は自分の `members` と仮想敵の `threats`(各1〜6件)を同じ形
    `{ "pokemonId": "NNNN-NNN", "moveIds": [moveId, ...], "abilityId"?: "..." }` で受け取る。`moveIds` は
    coverage と同じ検証(0〜4件・重複禁止・欠落や `null` は400)、`abilityId` は analyze と同じ検証
    (任意。`null` は省略と同じ)
  - `moveIds` が全員空・`abilityId` を誰も指定しなければ、技・特性の read model が未設定でも 200
  - 未登録の `pokemonId`/`moveId`/`abilityId`: 422(message に該当 ID を含む)
  - 上記以外の内部エラー(相性表や read model の想定外の失敗・不正な特性効果・倍率の積のオーバーフローを含む):
    500 `internal_error`(固定文言)
  - 成功: 200。`threats`(request順)ごとに `abilityId`(指定時のみ)・`attackTypes`(変化技を除く技のタイプ、
    重複なし・正準順)・`matchups`(`members` の request 順。各メンバーの `incoming`(受ける最大倍率、
    仮想敵の攻撃技が無ければ `null`)・`outgoing`(与える最大倍率、自分の攻撃技が無ければ `null`)・
    `safe`(`incoming < 1`)・`superEffective`(`outgoing >= 2`))・`safeMembers`/`superEffectiveMembers`
    (`matchups` の人数)を返す。倍率は既約分数の文字列。TB1〜3 の `CalculateDefenseWithAbility` を
    そのまま再利用し、新しい read model は無い
- `POST /api/balance/v1/team-balance/recommendations`(ADR-0401): `X-Device-Id` と `X-Session-Id` が必須。
  判定順は threats と同じ流儀: ヘッダー(400)→ body(400/413: `members` 件数・各メンバー・`limit` の範囲)→
  ポケモンの read model またはカタログ未設定、`moveId` を1つでも指定したのに技、または `abilityId` を
  1つでも指定したのに特性の read model 未設定(503)→ pokemonId 解決(422)→ moveId 解決(422)→
  abilityId 解決(422、request 順で最初のもの)→ 200。
  - request は `members`(1〜6件、threats の1エントリと同じ形 `{ "pokemonId", "moveIds", "abilityId"? }`)と
    任意の `limit`(1〜20。省略または `null` は10)
  - 防御の穴: TB1〜3 の集計で、耐性・無効を持つメンバーが0人の攻撃タイプ。攻撃範囲の穴: TB2の
    `teamCoverage` で有効打(×1以上)を取れない防御タイプ(メンバー全員が攻撃技を持たない場合は穴を出さない)
  - 候補は防御タイプの組み合わせ171通り(単タイプ18・複合153)のうち、穴を1つ以上ふさぐものだけを
    「ふさぐ穴の数(多い順)→弱点の数(少ない順)→正準順」で並べ、先頭から `limit` 件を返す
  - 上記以外の内部エラー(相性表や read model の想定外の失敗・不正な特性効果・不正な技分類を含む):
    500 `internal_error`(固定文言)。ただしカタログの `abilityIds` に特性 read model が知らない ID が
    あっても、その特性だけ飛ばして 200 を返す(export の不整合で全体を落とさない)
  - 成功: 200。`defenseHoles`/`offenseHoles`(正準順)、`candidates`(各候補の `types`・`defenseCovered`・
    `offenseCovered`・`weaknesses`・`pokemon`(複合タイプの候補はタイプ集合が一致する全員。単タイプの候補はそのタイプを含む全員で、もう片方のタイプで候補の防御の穴を受けられなくなるものを除く。`exactMatch` が真のもの → pokemonId 昇順。ADR-0401 §8、
    `nameJa` は read model にあるときだけ))、`abilityOptions`(防御の穴ごとに1件。特性で穴をふさげる
    ポケモンと特性の組、pokemonId 昇順→abilityId 昇順。特性 read model が無ければ全体が空配列 `[]`)を返す

HTTP の path・必須 header・handler interface は service-local OpenAPI から生成し、実装を
`api.ServerInterface` へコンパイル時に適合させる。

## ローカル検証(k3d)

前提: k3d の `pokecalc` クラスタが起動している(`make up`)。

1. テストと静的検査を通す。
   ```sh
   cd "$(git rev-parse --show-toplevel)"
   make test lint build check-publishable
   ```
   確認: 最後の行が `check-publishable: 0 件` で、エラーで止まらない。

2. k3d にデプロイして疎通を確かめる。
   ```sh
   cd "$(git rev-parse --show-toplevel)"
   make balance-k3d-deploy
   make balance-smoke
   ```
   確認: 最後の行が `balance smoke: health=200 analyze=200 unknown=422 coverage=200 unknown_move=422 ability=200 unknown_ability=422 threats=200 threats_unknown_move=422 recommendations=200`。
   1回目がロールアウト直後で失敗したら `make balance-smoke` をもう一度実行する。

## ローカル検証(ホストで直接。開発用)

```sh
cd "$(git rev-parse --show-toplevel)"
make balance-gen balance-test balance-lint balance-build balance-kustomize balance-gitops-template-check
```
確認: `balance GitOps template: valid` が出て、エラーで止まらない。

## ローカルの構成(説明)

- `balance-k3d-deploy` は、ルートの `deploy/k3d.yaml` と基盤 Kustomize で `pokecalc` クラスタ・Namespace が作成済みであることを前提とする。
  共有 Namespace は balance 側では所有しない。
- local overlay(`deploy/k8s/overlays/local`)は架空データの example(`testdata/*.example.json` と同一内容の複製)を ConfigMap でマウントし、
  `BALANCE_POKEMON_TYPES_PATH` / `BALANCE_MOVES_PATH` / `BALANCE_ABILITIES_PATH` を設定する。base と gitops overlay には設定しない。
- Argo CD 用の gitops overlay(`deploy/k8s/overlays/gitops`)の image はクラスタ内レジストリの `localhost:5000/pokecalc/balance@sha256:...`(digest 固定)。
  Application の repoURL は Git に書かず、適用時に `git remote get-url origin` から埋め込む。手順は [`deploy/argocd/README.md`](deploy/argocd/README.md)、方式は ADR-0018。
