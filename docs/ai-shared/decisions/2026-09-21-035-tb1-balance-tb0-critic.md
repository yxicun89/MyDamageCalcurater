## 2026-09-21: TB1 のタイプ取得・balance のテスト対象・相性表の出どころ(ユーザー決定。TB0 の critic セッション経由で受領)
Decision: (1) TB1 のポケモンのタイプは、まず仮の adapter と架空データでテストし(ADR-0014 の read model)、後で `data/generated/` のスナップショットを読む adapter に置き換える。
request は `pokemonId` のみ(ADR-0014 §1。type-balance-design.md §16 の未決「pokemonId のみかタイプまで送るか」への回答)。
(2) ルートの `make test` に balance を含める(「テスト漏れで成果物にエラーが出るのが嫌」)。ルート Makefile は include の1行のまま、
`services/balance/Makefile` で `test: balance-test` / `lint: balance-lint` / `build: balance-build` を前提条件として追加する。
(3) balance の相性表は、ダメージ計算レーンの P1-13(ADR-0013)でデータ化されたもの(現状 main の `testdata/golden/typechart.json`、同じ schema)を使う。
Reason: ユーザーが TB0 レビュー中に回答した(Claude Code のタイプバランスレーンのセッションが受領し記録)。
Impact: (2) により、balance が失敗するとルートの `make test` / `lint` / `build` も失敗する(ダメージ計算レーンの統合条件にも balance が入る)。
(3) は TB1 の続きとして balance 側で読む adapter を作り、TemporaryTypeChart を置き換える。ダメージ計算レーンの変更は不要
(typechart.json の schema を変える場合は balance の adapter も追従が要るので DECISIONS.md に書くこと)。
