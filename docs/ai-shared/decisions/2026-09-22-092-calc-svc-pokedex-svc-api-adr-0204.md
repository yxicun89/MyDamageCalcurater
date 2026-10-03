## 2026-09-22: calc-svc のマスタを pokedex-svc の内部 API から受け取る(ユーザー決定。ADR-0204)
Decision: calc-svc のマスタの入手元を pokedex-svc の内部 API `GET /internal/pokedex/master`(契約は api/openapi.yaml の tag `internal`、operationId `getMasterExport`、200 は MasterExport、503 は master_unavailable)にする。
calc-svc は起動時に取得し(失敗は指数バックオフで再試行、取得後は再取得しない、更新は再起動で反映)、services/internal/master の写像でメモリに載せる。取得できるまで計算は 503 master_unavailable、readiness(/readyz)も 503。
gateway は /internal/* を公開しない。k3d local と `make dev` は同じ形の JSON ファイル(架空データ)で動かす。
Reason: ユーザーが「pokedex-svc の内部 API」を選んだ(絶対ルール4を守り、マスタの正本を pokedex の DB 1つにするため)。
Impact(データレーンへの提案。既定案): (1) pokedex-svc(P2-3)で `GET /internal/pokedex/master` を実装する(クラスタ内の Service だけで Ingress には出さない。DB に未投入なら 503 master_unavailable。使用可能集合で絞らない。effect は item_effects / ability_effects の JSON をそのまま返す)。
(2) natures テーブル(id, name_ja, plus, minus)を追加し、/api/pokedex/natures と内部 API の両方で使う。(3) MasterExport の species には showdownId を含める(共通マスタの Species が形式を検証するため。nameEn は含めない)。
API レーンの後続: pokedex-svc のデプロイ後に calc の local overlay を URL 方式(`CALC_MASTER_URL=http://pokedex`)に切り替える。
Web / iOS へ: openapi に tag `internal` の操作と Master* の型が増える(web/src/api/openapi.gen.ts はこの PR で再生成済み)。iOS は生成し直すか、生成設定で `internal` タグを除外する。
