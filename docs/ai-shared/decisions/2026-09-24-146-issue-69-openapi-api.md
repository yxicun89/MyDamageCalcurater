## 2026-09-24: issue #69(技・持ち物検索の並びがOpenAPI契約と一致しない)を修正(API レーン)
Decision: `api/openapi.yaml` の `searchMoves`/`searchItems` の description が「並びは ID 順」としていたが、
実装(`services/pokedex/db/query/pokedex.sql` の `SearchMoves`/`SearchItems`。`ORDER BY <table>.name_ja, <table>.id`)は
P2-3 導入時から一貫して日本語名の照合順序(同順位は ID)だった。ADR-0105 §3 は既に「技・持ち物は name_ja, id」と
正しく明記していたため、**誤っていたのは契約の説明文だけ**(SQL・ADR は無変更・新規 ADR も不要)。`searchSpecies`
(`dex_no, form`)は SpeciesKey が固定幅ゼロ埋めのため文字列としての ID 順と一致し対象外、`listNatures`
(`ORDER BY id`)も元から契約どおりで対象外。
契約の description を実態に合わせて訂正し、`make gen`・`make ios-gen` を実行(絶対ルール1)。
**iOS の生成物は PR #161(getMove。P3-7)の分も含めて `make ios-gen` が漏れており未追従だった
(`make ios-gen-check` が失敗する状態だった)。今回まとめて解消した**(API レーンの取りこぼしの修復のため
同一コミットに含めた。iOS レーンの範囲への継続的な変更ではない)。
テストは2層: DB 層(`db.TestSearchMovesAndItemsOrderIsNameJaNotID`。`-tags mysql`。実 MySQL
〈kubectl port-forward で `pokecalc` クラスタの `svc/mysql` に接続、使い捨ての `pokedex_test` DB を都度作成・削除〉で
確認)と httpapi 層(`TestSearchMovesAndItemsPreserveGivenOrderAndLimitCutsThatOrder`。ハンドラが行順を並べ替えず、
`limit` がその並びの先頭から切ることを固定。ID 順に並べ替えてから切ると集合自体が変わることを issue の指摘どおり
変異テストで確認: 一時的に `ORDER BY m.id` に変えて DB 層のテストが落ちることを確認 → revert、一時的に
ハンドラへ `sort.Slice`(ID順)を差し込んで httpapi 層のテストが落ちることを確認 → revert)。
Reason: issue #69。契約(`api/openapi.yaml`)が唯一の正であるべきなのに実態とずれていた
(クライアントが契約どおり ID 順を前提にできない・limit 境界で返る集合自体が変わりうる)。
Impact: `docs/plan.md` の改善要望に issue #69 の行を追加。データ・Web・iOS レーンへの追加対応は無し
(SQL・ADR は無変更、iOS 生成物は本コミットで追従済み)。issue #69 はこの PR のマージでクローズしてよい。
