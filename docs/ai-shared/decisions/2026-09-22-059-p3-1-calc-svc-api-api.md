## 2026-09-22: P3-1 calc-svc の API 契約(API レーン。既定案で進行・ユーザー未確認)
Decision: ADR-0200 のとおり。`CalcResult.category` を足す(落とさない)、`BulkCalcRow.defender{sp,nature,natureId,stats}`、逆算は P1-12 の形
(`known` / `unknownSpeciesKey` / `itemCandidates` / 観測は percent・percentTenths・damage のちょうど1つ)、`Error.code` を enum `ErrorCode`
(WASM 境界の語彙 + HTTP だけの missing_header・unknown_*・not_found・master_unavailable・upstream_unavailable)。
マスタは calc-svc 内の暫定 `Store`(services/calc/internal/master)と架空データで作り、データレーンの共通マスタ(P2-2a)が main に入ったら差し替える。
Reason: plan.md P3-1 の小項目と ADR-0010 §9・§R8 / ADR-0011 §10 の持ち越しを解消するため。いずれも取り消しやすい契約の既定値。
Impact: Web レーン(P4-5)は生成型の変更に追従する。データレーンへ: P2-2a の master が入ったら、calc-svc の `Store` インターフェース
(Species / Move / Item / Ability / Nature / NatureID / TypeChart)を満たす adapter を API レーンが作る。
