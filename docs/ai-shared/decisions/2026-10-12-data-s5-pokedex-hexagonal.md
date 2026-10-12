## 2026-10-12: pokedex-svc の形をそろえる PR(ARCH S5)を出す(データレーン)
Decision: pokedex-svc を domain・app・adapters の形にそろえる。`services/pokedex/internal/{httpapi,readtx,readmodel}` は `internal/adapters/` の下へ、`internal/dataversion` は `internal/domain` へ移る。sqlc の `internal/store` と `internal/storetest` は場所を変えない(ADR-0180)。
Reason: docs/plan/architecture-adoption.md §6 の S5(G7 の解消)。
Impact: pokedex-svc のファイルを触る他レーンの PR は、マージ後に `git merge origin/main` で取り込む(移動は git mv)。importer(`services/pokedex/importer`)は対象外で、その mysql テストの import だけを追従した。
