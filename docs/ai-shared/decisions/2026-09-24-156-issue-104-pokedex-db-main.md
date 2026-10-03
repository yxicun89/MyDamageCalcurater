## 2026-09-24: issue #104(pokedexのDB資格情報を用途別の最小権限へ分離)を main へ統合(データレーン)
Decision: PR #176(`feat/claude-p1-engine` → `main`)をマージした。`pokedex_reader`/
`pokedex_importer`/`pokedex_migrator`の3ロール分離(ADR-0110)。critic PASS(1往復)。
Reason: 独立レビュー PASS・`make test`(911件)/`test-db`/`lint`/`k8s-render`すべてgreen。
実クラスタでSHOW GRANTSにより権限確認済み、既存クラスタからの無停止移行も実地確認済み。
Impact: 他レーンへの影響なし。cloud overlay実装時はSecretのDSNキー名を契約として踏襲する
ことを推奨(ADR-0110決定8)。
