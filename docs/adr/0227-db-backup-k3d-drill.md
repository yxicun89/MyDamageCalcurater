# ADR-0227: k3d の実 TiDB でのバックアップと復元訓練

- 状態: 採用(2026-10-03)
- 関連: ADR-0225(バックアップ/復元の本体。§7 の2を本 ADR で解消)、ADR-0211(record/team の DB ユーザー・権限)、ADR-0226(k3d への TiDB 適用)

## 決定

1. `scripts/db-backup-k3d.sh`(`make db-backup-k3d`)は、TiDB(record・team)と MySQL(pokedex)へ port-forward し、
   接続情報を Secret から**スクリプト内部で**読んで `db-backup.sh full <kind>` を流す。
   値はコマンド行・ログ・画面・`ps` に出さず、`MYSQL_PWD` 環境変数だけで渡す(`pokedex-export-local.sh` と同じ流儀)。バックアップは非破壊。
   - 使う権限は最小のもの: record・team は `<db>_migrator`(SELECT を含む)、pokedex は `pokedex_reader`(SELECT のみ)。
2. `scripts/db-restore-drill-k3d.sh`(`make db-restore-drill-k3d`)は、稼働中の DB を上書きせず、使い捨ての別名 DB
   `record_restore_drill` / `team_restore_drill` に最新世代を復元し(`RESTORE_FROM_DB=<元> CONFIRM_RESTORE=<別名>`)、
   全表の行数・墓石(`devices.purged_at` 有り)・purge_journal が元と一致することを確かめ、別名 DB を削除する。
   - 別名 DB の作成・削除は migrator の権限(自分の DB の DDL のみ。ADR-0211 §4)に無いので `tidb-root-auth` の root を使う。
     root のパスワードもスクリプト内部で読み、権限付与の追加はしない(TidbInitializer・SQL への GRANT 追加は不要)。
   - DROP は「今回このスクリプトが作った別名 DB」だけ。名前を `^(record|team)_restore_drill$` で検査し、既存の同名 DB があれば作らず失敗する(自動では消さない)。
   - 復元手順5の失効ジョブは、別名 DB に向けて実際に `record expire` / `team expire` を流す(保持日数は本番の ConfigMap と同じ値)。
3. ホストに `mysql`・`mysqldump` が無いときは、固定 digest の mysql イメージの docker ラッパー(`-e MYSQL_PWD` で値を渡さず継承)を使い、
   接続先は `host.docker.internal` にする(`scripts/k3d-db-lib.sh`)。
4. 静的検査は `scripts/db-backup-k3d_test.sh`(`make test-scripts`)。実クラスタでの実行は下の §実行記録。

## 実行記録(2026-10-03。k3d-pokecalc。TidbCluster pokecalc-tidb・record/team・mysql-0)

- `make db-backup-k3d`: record(6表)・team(5表)・pokedex(20表)の世代を取得。devices(墓石)を含むことをスクリプトが確認。
- `make db-restore-drill-k3d`: record・team とも `restore-ok`。全表の行数が一致(record: calc_events 38・devices 12・frequent_opponents 2 ほか。team: devices 11 ほか)、
  墓石 0 件・purge_journal 0 件で一致。別名 DB は削除された。`restore-drill-ok record team`。
- **mysqldump(mysql 9.7.2 クライアント)の `column_masking_policy` の `SELECT command denied` はエラー表示だが終了コード 0 で、ダンプは完全**
  (MySQL 9 のクライアントがマスキングポリシーを読もうとして権限不足になる。migrator/reader に権限を足さず、そのまま許容)。
- TiKV に対して `--init-command="SET autocommit=0"`(ADR-0225 追記)が実クラスタでも有効で、SAVEPOINT 関連のエラーは出なかった。
  `--single-transaction` のダンプは TiKV(unistore ではない実 TiKV)でも通った。
- 限界(未検証): 実クラスタのデータには墓石・purge_journal が 0 件だったので、墓石・journal の再適用の効果そのものは Docker の
  実往復テスト(`make test-db-backup`)に依る。復元後の API 越しの確認も未実施(ADR-0225 §7 の7)。

## 影響

- ADR-0225 §7 の2(実クラスタでのバックアップ・復元)は解消。1・3・4・5・6・7 は未対応のまま。
- 値を出さない運用の前提として、Secret の読み取りはスクリプト内部に限る(Claude Code のガードは直接の `kubectl get secret` を止めるので、人間も Claude もスクリプト経由で実行する)。
