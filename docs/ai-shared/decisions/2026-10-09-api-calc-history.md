## 2026-10-09: 計算履歴の一覧 API(GET /api/record/calc-history)の契約とテスト(spec)を追加(API レーンから Web・iOS レーン・実装者へ)
Decision: ADR-0230 を追加し、`api/openapi.yaml` に `listCalcHistory`(`GET /api/record/calc-history`)と schema
`CalcHistoryPage` / `CalcHistoryEntry` / `CalcHistoryResult` / `CalcHistoryCursor` を足した。1件の計算(`POST /api/calc`)だけを
新しい順に返し、行は `occurredAt`・`calc`(`CalcRequest` そのもの。お気に入りの `calc` と同じ正規化)・`result`(表示%の幅)の3つだけ。
ページングは keyset のカーソル(`limit` 1〜50・既定20、`nextCursor` が null なら終わり)。一括計算・逆算は載らない。個別削除は無い。
Reason: requirements.md §2「履歴」と ADR-0209 §3 の calc_events の目的「履歴表示」が未実装で、Web(P5-5c)は対象外、
iOS は「契約待ち」と注記していたため。calc_events.payload は1件の計算の入力全体をすでに持つので、イベントの拡張は要らない。
Impact:
- Web・iOS: 経路・型・ページング・エラーの扱いは ADR-0230「Web・iOS レーンへの依頼」。行の `calc` はお気に入りから計算を出す処理を
  そのまま使える。503 は一覧の場所にだけ出し計算画面を塞がない。iOS の「契約待ち」の注記は main 統合後に外せる。
  型の網は `web/src/record/calcHistoryContract.test.ts`・`ios/PokeCalcKit/Tests/PokeCalcCoreTests/CalcHistoryContractTests.swift`(生成後は通る)。
- 実装者(API レーン): `services/record` の store(`ListCalcHistory`・型3つ)・httpapi(`ListCalcHistory`・`WithCalcEventsRetention`)・
  migration 000008(索引)・cmd/record の配線、calc / pokedex / team の `api.ServerInterface` スタブ、`events/consumer_test.go` の fake。
  形は ADR-0230 §8 と `services/record/internal/httpapi/calc_history_fixture_test.go` の冒頭。gateway は変更なし。

実装結果(2026-10-09): 上のとおり実装済み(`services/record` の store・httpapi・migration 000008・cmd/record の配線、calc/pokedex/team の 404 スタブ)。
契約・ADR からの変更なし。ADR-0230 の状態は critic 後に「採用」にする。k3d への適用は未実施(migrate ジョブで 000008 が当たる)。
