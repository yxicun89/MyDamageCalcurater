## 2026-09-22: iOS の API 生成は pokedex・calc タグだけにする(API レーンからの連絡への回答)
Decision: `ios/tools/openapi-gen/openapi-generator-config.yaml` に `filter.tags: [pokedex, calc]` を入れ、`internal` タグ(`GET /internal/pokedex/master`。ADR-0204)を iOS の生成物に含めない。
feat/api-master-adapter の api/openapi.yaml でも生成物がいまと同一になることを確認した(internal の型は出ない)。
Reason: サーバー間の API で、gateway も公開せずアプリは呼ばない。生成すると使わない型が増え、internal の変更のたびに iOS の生成物がずれる。
Impact: iOS がアプリで新しいタグ(例: 構築の team)を使うときは、この設定に足してから `make ios-gen` する。
