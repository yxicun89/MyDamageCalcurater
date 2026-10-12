// Package app は pokedex-svc のユースケース層(ADR-0811・ADR-0180)。
//
// 公開の検索(種族・技・持ち物・性格・技を覚える種族)、内部 API のマスタ一式(MasterExport)、
// readiness(Ready)、balance・speed 向けの read model(ExportReadModel)の 1 操作の手順を持つ:
//
//	入力の検証(DB を呼ぶ前)→ 既定のレギュレーション → 読み出し(必要なら1つの読み取り専用スナップショット)
//	→ 共通マスタでの検証 → 結果の組み立て
//
// DB の読み出しはここで宣言するポート(Reader・Snapshot・Store・SnapshotBeginner)で、実装は adapters/store
// (sqlc の services/pokedex/internal/store を readtx 経由で使う)。呼び出しの順番・引数は HTTP 境界に
// あったときと同じに保つ(テストの偽の DB が呼び出しの列を確かめている)。
// 生成型(services/internal/api)・HTTP・database/sql を知らない。
package app
