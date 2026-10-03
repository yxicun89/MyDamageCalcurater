# ADR-0251: 技の逆引き API `GET /api/pokedex/moves/{key}/learners`(AJ5)

- 状態: 採用(AJ5 の仕様。spec 段階)
- 日付: 2026-10-02
- レーン: ダメージ計算(API 帯 `0200〜`。0250 の次)
- 関連: plan.md AJ5、ADR-0150(調整機能)、ADR-0250(調整 API)、ADR-0105 §3(公開の検索 API・使用可能集合・limit)、
  ADR-0127(読み取り専用 Tx は内部 API と export だけ)、ADR-0202(gateway の前方一致)、ADR-0124(migration の書き換え規則)、
  CLAUDE.md 絶対ルール 1・4

## 背景

調整タブの機能 1「技を選ぶと、その技を覚えるポケモンの一覧が出る」には learnset の逆引きが要る。
pokedex の公開 API には種族 → 技(`getSpecies.learnset`)しか無い。

## 決定

### 1. 契約(api/openapi.yaml `listMoveLearners`)

- `GET /api/pokedex/moves/{key}/learners`。タグ `pokedex`。必須ヘッダは他の公開操作と同じ(X-Device-Id / X-Session-Id)。
- 応答 200 は **`SpeciesSummary[]`**(既存のスキーマを再利用。新しいスキーマを作らない)。一致なしは `[]`(null にしない)。
- 並びは `searchSpecies` と同じ **`dex_no, form` の昇順**。`species` に UNIQUE(dex_no, form) があるので決定的。
- ページング: **`limit`(既定 50・1〜200。ADR-0105 の検索と同じ)と `offset`(既定 0・0〜10000)**。
  総数は返さず、返った件数が `limit` 未満なら最後のページ。応答を配列のままにして pokedex の他の一覧操作と形をそろえる
  (クライアントの生成物に新しい包みの型を増やさない)。`offset` の上限 10000 は int32 に収め、
  種族の全件(千数百)より十分大きい安全側の値で、ドメインの定数ではない。範囲外・整数でない値は 400 `invalid_input`。
  マスタは週1回の全置換でしか変わらないので、offset 方式のページずれは実害が無い(keyset にしない)。
- 404 / 503:
  - 既定のレギュレーションが無い(マスタ未投入)・DB の失敗 → 503 `master_unavailable`(一覧系の流儀。
    `getMove` の「未投入でも 404」とは違う。この操作は既定のレギュレーションで絞るため、`getSpecies` と同じく先に引く)。
  - 既定のレギュレーションはあるが技 `key` が `moves` に無い → 404 `not_found`。`key` の形式は検査しない(`getMove` と同じ)。
  - 技がマスタにあるが使用可能集合の外 → **200 `[]`**(404 にしない。`getMove` はその技を返すので「存在しない」ではない)。
- 判定の順: 入力の検証(ヘッダ・limit・offset。DB を呼ばない)→ `GetDefaultRegulation`(無ければ 503)→ `GetMove`
  (`sql.ErrNoRows` なら 404、他の失敗は 503)→ `ListMoveLearners`(失敗は 503)。

### 2. 「使用可能集合で絞る」の規則

種族が `regulation_species`、技が `regulation_moves`(どちらも既定のレギュレーション)にあり、`learnsets` に (種族, 技) があるもの。
`getSpecies` の `learnset`(習得技 ∩ 使用可能な技)と同じ規則で、使用可能な種族 S・技 M について
「S が M の逆引きに出る」⇔「`getSpecies(S).learnset` に M がある」が成り立つ(テストで固定)。
`format` は持たない(使用可能集合は形式で分かれていない。ADR-0105 §3)。

### 3. DB・クエリ

- sqlc クエリ `ListMoveLearners`(`services/pokedex/db/query/pokedex.sql`)。引数 `RegulationID, MoveID, Limit, Offset`、
  行は `SearchSpeciesRow` と同じ列(key, dex_no, form, name_ja, type1, type2)。
- **migration は足さない**。`learnsets` の PK は (species_key, move_id) だが、000002 の FK `fk_learnsets_move` のために
  MySQL(InnoDB)が move_id を先頭にした索引を自動で作っている。逆引きはこの索引で引ける
  (`TestLearnsetsHasMoveIDIndex` で固定。将来 FK を外す migration を書くときは索引を明示的に足すこと)。
  000002 を書き換えない(ADR-0124: 適用済みの up は書き換えない)。
- 読み出しは autocommit の3文(検索・`getSpecies` と同じ。ADR-0127 の1 Tx は内部 API と export が対象)。
  import の全置換と重なった数秒だけ新旧が混ざり得るのは既存の公開 API と同じ限界。

### 4. 載せないもの・他レーンへの影響

- read model(`pokedex export`)には載せない。balance・speed の loader のファイル形は変わらない。
- gateway は変えない(`/api/pokedex/` の前方一致で pokedex-svc に届く。テストで固定)。
- calc-svc・record-svc・team-svc は `api.ServerInterface` を満たすための 404 スタブだけ足す(ルートは登録しない。
  未登録パスは既存のエラーハンドラで 404 `not_found`)。
- ルーティング: echo のルーターで `/api/pokedex/moves/:key/learners` を登録すると、`/moves/:key`(getMove)と
  `/moves/batch`(getMovesByIds)は従来どおり。`/moves/batch/learners` は key=`batch` の逆引き(技が無ければ 404)。
- Web(`web/src/api/openapi.gen.ts`)は `make gen` で型が増えるだけ。iOS の生成物(`make ios-gen`)は追加だけで
  既存の型を変えない。AJ4 で再生成し忘れた adjust 4 操作と合わせて、AJ5 で `make ios-gen` を実行した(追加だけ。`make ios-gen-check` で一致を確認)。iOS の画面は AJ7。

## 受け入れ条件と担当テスト

| AC | 内容 | テスト |
|---|---|---|
| AC-L1 | 200 の応答・リクエストが契約どおり | `httpapi.TestListMoveLearnersMatchesContract` |
| AC-L2 | 集合の外の種族を出さない・dex_no, form 昇順・要約の写し方・store に既定のレギュレーションと技 ID・既定の limit 50/offset 0 を渡す | `TestListMoveLearnersFiltersAndOrders` |
| AC-L3 | getSpecies の learnset と同じ規則(双方向) | `TestListMoveLearnersAgreesWithGetSpeciesLearnset`、db `TestListMoveLearnersQuery` |
| AC-L4 | 集合の外の技・覚える種族が無い技は 200 `[]` | `TestListMoveLearnersEmpty` |
| AC-L5 | limit・offset のページング(連結で全件・末尾超えは `[]`・上限ちょうどは受け付ける・値を store に渡す) | `TestListMoveLearnersPaging` |
| AC-L6 | 未知の技は 404 not_found(逆引きのクエリを呼ばない)。`batch` も技 ID として扱う。getMove は影響を受けない | `TestListMoveLearnersUnknownMove` |
| AC-L7 | ヘッダ欠落 400 missing_header、limit・offset の範囲外・非整数 400 invalid_input(DB を呼ばない) | `TestListMoveLearnersInputValidation` |
| AC-L8 | 既定のレギュレーション無し(key が未知でも)・DB の失敗は 503 master_unavailable(内部情報なし) | `TestListMoveLearnersUnavailable`、`TestDBRoutesReturnUnavailableWithinRequestDeadline`(dbRoutes に追加) |
| AC-L9 | 実 MySQL でのクエリ: 絞り込み・並び・LIMIT/OFFSET | db `TestListMoveLearnersQuery`(`make test-db`) |
| AC-L10 | learnsets に move_id を先頭にした索引がある(migration 不要の根拠) | db `TestLearnsetsHasMoveIDIndex`(`make test-db`) |
| AC-L11 | gateway は前方一致でパス・クエリをそのまま pokedex に転送・上流の 404/503 を素通し・ヘッダ検証が効く | gateway `TestLearnersRouteReachesPokedexUpstream` / `TestLearnersUpstreamErrorsPassThrough` / `TestLearnersRouteRequiresHeaders` |
| AC-L12 | calc-svc では担当外として 404 not_found | calc `TestPokedexRoutesAreNotFound`(パスを追加) |
