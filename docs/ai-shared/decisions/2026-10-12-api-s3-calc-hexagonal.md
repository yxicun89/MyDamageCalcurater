## 2026-10-12: calc-svc の形をそろえる PR(ARCH S3a)を出す(API レーン)
Decision: calc-svc を domain・app・adapters の形にそろえる(計算・一括・逆算まで。調整 `adjust.go` は次の PR の S3b)。`services/calc/internal/{httpapi,master,events}` は `internal/adapters/` の下へ移る(ADR-0252)。
Reason: docs/plan/architecture-adoption.md §6 の S3。1 PR を小さく保つため 2 つに分ける(計画で許されている)。
Impact: calc-svc のファイルを触る他レーンの PR は、マージ後に `git merge origin/main` で取り込む(移動は git mv)。Web の `exportSnapshot.contract.test.ts` が読む `export.go` のパスを追従済み。S3b(調整)の PR まで calc の `adjust.go` の大きな変更は控えてほしい。
