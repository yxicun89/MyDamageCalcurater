## 2026-10-03: P7-4 の k3d 実バックアップ・復元訓練(ADR-0227)
Decision: `make db-backup-k3d` と `make db-restore-drill-k3d` を追加し、実 TiDB で別名 DB への復元訓練まで通した。Secret の読み取りはスクリプト内部だけで行う。
Reason: ADR-0225 §7 の2(実クラスタ未実施)の解消。稼働中の DB は上書きせず、別名 DB `<db>_restore_drill` を使い、自分が作った名前だけを DROP する。
Impact: 別名 DB の作成・削除に root(tidb-root-auth)を使う(migrator に権限を足さない)。墓石・journal の実データは 0 件で、再適用の効果は Docker 往復テスト依存。
