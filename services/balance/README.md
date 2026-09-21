# balance service

パーティのタイプ相性・弱点・耐性・攻撃範囲・仮想敵診断を分析するドメインモノリス。
damage-calc とは兄弟サービスで、互いの実行時 API には依存しない。

## TB1(防御タイプバランス)・TB2(攻撃範囲)・TB3(特性による防御相性の変化)・TB4(仮想敵診断)の範囲

- `internal/balance`: HTTP・DB・Kubernetes に依存しない型・防御相性コア・チーム集計(`AnalyzeDefense`)、
  技の解決コア(`ResolveMoves`)・攻撃範囲コア・チーム集計(`AnalyzeCoverage`)、特性の解決コア
  (`ResolveAbility`)・有理数の倍率(`Effectiveness`)・特性込みの防御計算(`CalculateDefenseWithAbility`)、
  および TB1〜3 の計算を再利用する仮想敵診断コア(`AnalyzeThreats`)
- `api/openapi.yaml`: balance 外部 API 契約の正
- `internal/api`: oapi-codegen による生成型
- `internal/master`: 共通マスタへ差し替えるための adapter(タイプ相性表・ポケモンタイプ read model・技 read model・
  特性 read model)
- `internal/httpapi`: health・analyze・coverage・threats の実装
- `cmd/api`: プロセス起動・`BALANCE_POKEMON_TYPES_PATH` / `BALANCE_MOVES_PATH` / `BALANCE_ABILITIES_PATH` の
  読み込み・graceful shutdown
- `deploy`: balance 専用 Kustomize と manual-sync の Argo CD Application
- `DEPENDENCIES.md`: 公開前確認用の直接依存・利用理由・license

契約の正は TB1 が [ADR-0014](../../docs/adr/0014-balance-tb1-defense-analysis.md)、
TB2 が [ADR-0016](../../docs/adr/0016-balance-tb2-offense-coverage.md)、
TB3 が [ADR-0017](../../docs/adr/0017-balance-tb3-ability-effects.md)、
TB4 が [ADR-0400](../../docs/adr/0400-balance-tb4-threat-check.md)。

タイプ相性表はコードに持たない(ADR-0013・ADR-0015)。ダメージ計算レーンの P1-13 でデータ化された
`testdata/golden/typechart.json` を `internal/master/data/typechart.json` にバイト複製して go:embed で同梱し、
`master.EmbeddedTypeChart` が起動時に検証して読む(不正なら起動失敗)。元ファイルとの一致はテスト
(`TestEmbeddedTypeChartMatchesSharedData`)が検査するので、元が更新されたら `make balance-sync-typechart` で複製し直す。
共通マスタ(P2-2)の相性表が別の形で配布されるようになったら、`balance.TypeChartProvider` の adapter だけを差し替える。

ポケモンのタイプは `internal/master.PokemonTypeReadModel`(`BALANCE_POKEMON_TYPES_PATH` が指す JSON を
起動時に1回だけ読む)から引く。これも temporary adapter で、共通マスタのスナップショット schema(P2-2)が
決まったら差し替える。Git に置くのは schema と架空データの example(`testdata/pokemon-types.example.json`、
ID は `9001-000` 以降)だけで、実 Pokémon マスタはコミットしない(ADR-0002)。

技のタイプ・分類(物理/特殊/変化)は `internal/master.MoveReadModel`(`BALANCE_MOVES_PATH` が指す JSON を
起動時に1回だけ読む)から引く。同じく temporary adapter。Git に置くのは schema と架空データの example
(`testdata/moves.example.json`、ID は `move-9001` 以降)だけ(ADR-0002・ADR-0016 §3)。

特性による防御相性の変化(`immune`/`absorb`/`type_multiplier`/`super_effective_multiplier`)は
`internal/master.AbilityReadModel`(`BALANCE_ABILITIES_PATH` が指す JSON を起動時に1回だけ読む)から引く。
同じく temporary adapter。倍率は既約分数(`balance.Effectiveness`、float は使わない)で持つ。Git に置くのは
schema と架空データの example(`testdata/abilities.example.json`、ID は `ability-9001` 以降)だけ
(ADR-0002・ADR-0017 §2)。

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

HTTP の path・必須 header・handler interface は service-local OpenAPI から生成し、実装を
`api.ServerInterface` へコンパイル時に適合させる。

## ローカル検証

リポジトリルートから実行する。

```sh
make -f services/balance/Makefile balance-gen
make -f services/balance/Makefile balance-test
make -f services/balance/Makefile balance-lint
make -f services/balance/Makefile balance-build
make -f services/balance/Makefile balance-kustomize
make -f services/balance/Makefile balance-gitops-template-check
```

`balance-k3d-deploy` は、ルートの `deploy/k3d.yaml` と基盤 Kustomize により `pokecalc` クラスタ・
Namespace が作成済みであることを前提とする。共有 Namespace は balance 側では所有しない。

local overlay(`deploy/k8s/overlays/local`)は架空データの example
(`deploy/k8s/overlays/local/pokemon-types.example.json`・`moves.example.json`・`abilities.example.json`、それぞれ
`testdata/pokemon-types.example.json`・`testdata/moves.example.json`・`testdata/abilities.example.json` と同一内容)を
ConfigMap としてマウントし、`BALANCE_POKEMON_TYPES_PATH` / `BALANCE_MOVES_PATH` / `BALANCE_ABILITIES_PATH` を
設定する。base と gitops overlay には設定しない。
`balance-smoke` は analyze・coverage・threats が 200(架空ID)と 422(未登録ID)を返すことを確認する。

Argo CD 用には local image を参照しない専用 overlay(`deploy/k8s/overlays/gitops`)がある。image はクラスタ内レジストリの
`localhost:5000/pokecalc/balance@sha256:...`(digest 固定)。Application の repoURL は Git に書かず、`make balance-argocd-app` が
適用時に `git remote get-url origin` から埋め込む。private repository の credential や Secret は Git へ入れない。
手順は [`deploy/argocd/README.md`](deploy/argocd/README.md)、方式は ADR-0018。
