# ADR-0225: MySQL / TiDB のバックアップと復元(P7-4)

- 状態: 提案(2026-10-03。spec-writer。テスト先行で、実装は後続。critic 未実施)
- 日付: 2026-10-03
- 関連: ADR-0209 §3 #5・#5b・#7、§9(要件の正)、ADR-0211(TiDB。`devices`・`purge_journal`)、ADR-0220(失効ジョブ)、
  ADR-0100(pokedex の DB 運用)、docs/runbooks/data.md「d. 再生成できないものと MySQL の論理バックアップ」、
  issue #262(MySQL バックアップ)・#297(Argo CD・レジストリ image・DB を戻す手順。needs-decision)、docs/plan.md P7-4

## 背景

ADR-0209 §9 は P7-4 に、復元で削除済み・期限切れのデータが復活しないことを要件として課している。
現状は `docs/runbooks/data.md` に pokedex の手動 `mysqldump` があるだけで、record / team(TiDB)のバックアップ・復元、
purge journal の DB 外保管、復元の順序を担うものが無い。一方、TiDB は k3d に未デプロイで record / team の Deployment も
未配備のため、実クラスタでは試せない部分がある。この ADR は、**いま実装して自動テストで証明できる範囲**と、
**人間判断・実クラスタが要る範囲**を分ける。

## 決定

### 1. 範囲(実装する)

- `scripts/db-backup.sh full|journal <kind>` と `scripts/db-restore.sh <kind> <世代|latest>`。`kind` は `pokedex`(MySQL)・`record`・`team`
  (TiDB。MySQL プロトコル互換なので `mysqldump` / `mysql` で扱う)。
- 実 DB での往復の自動テスト(Docker の使い捨て TiDB〈unistore〉・MySQL)。`scripts/test-db-docker.sh` の流儀。
- Makefile: `db-backup` / `db-restore`(スクリプトの薄い入口。実装時に追加)、`test-db-backup`(本 ADR で追加済み)。
- runbook(`docs/runbooks/data.md` の「d」を置き換える形で、バックアップ・復元の手順と Ready の前の確認)。実装時に書く。

### 2. 接続とバイナリ(スクリプトの入力)

環境変数で受ける: `DB_HOST` `DB_PORT` `DB_USER` `DB_NAME`、パスワードは **`MYSQL_PWD`(コマンドライン引数に出さない)**。
`MYSQLDUMP_BIN`・`MYSQL_BIN`(既定は PATH の `mysqldump`・`mysql`。テストで偽物・docker exec ラッパーに差し替える)。
`BACKUP_DIR`(既定 `data/generated/backups`)、`BACKUP_NOW`(エポック秒。テスト用に「今」を固定)、
`BACKUP_RETENTION_DAYS`(既定 30)、`JOURNAL_RETENTION_DAYS`(既定 90)。

### 3. 保存先と機密の扱い

- 保存先は `BACKUP_DIR`(既定 `data/generated/backups/`。`data/generated/` は `.gitignore` 済み)。**リポジトリ内で Git の無視対象でない場所は拒否する**
  (`git check-ignore`)。リポジトリ外は許す(外付けディスク等)。
- ディレクトリは 0700・ファイルは 0600。ダンプには端末 ID・保存データ(計算イベント・構築名・ニックネーム)が入るので機密として扱う
  (ログ・PR・Issue に中身を貼らない。ADR-0209 §3 のログ規則と同じ)。スクリプトは件数・世代 ID・DB 名・テーブル名だけを標準出力に出す。
- 暗号化は**今回は入れない**。理由: v1 は個人利用で tailnet 内(ADR-0209 §1・ADR-0210)、保存先は手元の端末のローカルディスクで
  ディスク暗号化(FileVault)の下にある。スクリプト内で鍵を持つと鍵管理が増える。**クラウドや共有ストレージへ出すとき(§7)は暗号化を必須**とし、
  そのときの方式を別 ADR で決める。
- レイアウト: `BACKUP_DIR/<kind>/<世代 ID>/dump.sql.gz` と `MANIFEST`。世代 ID は取得時刻の UTC `YYYYMMDDTHHMMSSZ`。
  `MANIFEST` は `taken_at:`(ISO)・`db:`・`tables:`(ダンプに入った表。ダンプから機械的に拾う)の3行以上のテキスト。

### 4. バックアップ(`db-backup.sh full`)

1. `mysqldump --single-transaction`(一貫したスナップショット。表の除外はしない)を gzip して世代ディレクトリへ。完了前は一時名で書き、成功後に改名
   (途中で失敗した世代を残さない)。
2. record / team は、ダンプに `devices` の `CREATE TABLE` が無ければ**失敗して世代を残さない**(ADR-0209 §9-1・AC-B1)。pokedex は `devices` を持たないので要求しない。
3. record / team は続けて purge journal を同期する(§5)。
4. 世代の失効: `BACKUP_RETENTION_DAYS`(30日)より古い世代を、**その kind の世代だけ**消す。判定は世代 ID の時刻(ファイルの mtime ではない)。journal には触れない。

### 5. purge journal の別保管(ADR-0209 §5b・§9-2 への回答)

- 保管先: `BACKUP_DIR/journal/<kind>.tsv`(record・team)。**世代ディレクトリとは別の場所**で、世代の失効に連動しない。
- 形式: 1行 `device_id<TAB>requested_at`(UTC、`YYYY-MM-DDTHH:MM:SS.ffffffZ`)。**追記のみ**: DB の `purge_journal` を読み、まだ無い行だけを末尾に足す
  (既存行の書き換え・並べ替え・重複はしない。DB 側の行が失効・全損で消えても、保管済みの行は消えない)。
- 保持: `JOURNAL_RETENTION_DAYS`(90日)より古い行だけを、`db-backup.sh` の実行時に取り除く(ADR-0209 #5b の90日。世代30日より長い)。
- 同期のタイミング: `db-backup.sh journal <kind>`(軽い。定期実行向け)と、`full` の中。**同期の間隔の間に受けた削除要求は、DB が全損するとこの保管先にも無い**
  (ADR-0209 §9-2 の「削除要求の受付時に同時に DB 外へ追記」は、サービスが DB 外へ書く実装が要るため未達。§7 の1)。
  既定案として、`journal` を**日次〜1時間ごと**に流す(運用側の cron / launchd。手順は runbook)。この穴を runbook に明記する。

### 6. 復元(`db-restore.sh`。ADR-0209 §9-3 の順序)

サービスを Ready にする前に完了させる。**record-svc / team-svc は止めた状態で流す**(手順書で「Deployment を 0 にする → 復元 → 戻す」)。

1. 確認: `CONFIRM_RESTORE=<DB 名>` が `DB_NAME` と一致しなければ、DB に触れずに失敗する(DB を上書きするので人間の確認が要る。`migrate-down` の `CONFIRM_DESTROY` と同じ流儀)。
2. 世代の検証: `MANIFEST`・`dump.sql.gz` があり、record / team は `devices` を含むこと。無ければ DB に触れずに拒否(AC-B1)。`latest` は検証を通る最新の世代。
3. ダンプの読み込み(`mysqldump` は `DROP TABLE IF EXISTS` を含むので、その DB の表を置き換える)。失敗したら以降を流さず失敗。
4. 保管済みの journal を DB の `purge_journal` へ取り込む(重複なし。世代に無い削除要求を DB に戻す)。
5. **再適用**(a. 墓石 `devices.purged_at`・b. journal。ADR-0209 §9-3a・b。どちらも冪等):
   - journal の端末は `devices` に行が無ければ作り、`purged_at` を `max(既存, requested_at)` にする(イベントの再出現の抑止も戻る)。
   - 墓石(`purged_at` がある端末)ごとに、その時刻以前の業務の行を消す。record: `calc_events`(`occurred_at <= purged_at`)・`favorites`(`created_at <= purged_at`)・`frequent_opponents`(端末の全行。再集計される)。
     team: `team_members`(`team_id` が消える `teams` のもの)・`teams`(`created_at <= purged_at`)。
     時刻以後の行を残すのは、削除の後に行った計算・保存は利用者の期待どおり残す(ADR-0209 §5・§7)ため。
6. 失効ジョブの強制1回実行(ADR-0209 §9-3c): `RESTORE_EXPIRE_CMD`(record は `record expire`、team は `team expire`)を実行する。失敗したら失敗。
   pokedex は 4〜6 を行わない(墓石・journal・失効が無い)。
7. 最後の1行に `restore-ok <kind> <世代 ID>` を出す。**途中のどこで失敗しても出さない**。これを Ready にしてよい印にする。
- JetStream・NATS には一切触れない(AC-B3。再生すると削除済みデータが戻る)。スクリプトに NATS・kubectl の呼び出しを書かない(静的検査あり)。
- 「`purged_at` が無い端末」の削除済みデータが世代に残っている場合は、journal が唯一の手掛かり。journal に無い削除要求(§5 の間隔の穴)は復元できない。

### 7. 対象外と、その理由・既定案(人間判断または実クラスタ未配備)

| # | 未対応 | 理由 | 既定案 |
|---|---|---|---|
| 1 | サービスが削除要求の受付時に purge journal を DB 外へ**同時に**追記する(ADR-0209 §9-2 の厳密な形) | record / team の store 実装と、DB 外の書き込み先(オブジェクトストレージ・別 DB・NATS KV 等)が要る。保存先の選択はクラウドの判断 | §5 の同期(日次〜1時間ごと)で運用し、穴を runbook に書く。クラウドへ出す前に、同時追記を別 ADR で決める |
| 2 | 共有クラスタ(k3d)上の実バックアップ・復元 | TiDB(TidbCluster)が未適用(P5-1 の残り)で、record / team の Deployment も未配備 | 配備後に runbook どおり1回通し、結果を plan.md に記録(人間の確認付き) |
| 3 | クラウドの保存先・暗号化 | 公開しない・tailnet 内の方針(ADR-0210)で、クラウドに出す判断が未確定 | ローカルの `data/generated/backups/` のみ。出すなら暗号化(例: age)を別 ADR で必須に |
| 4 | PVC のスナップショット(`pokedex-import-cache` 等) | StorageClass(local-path)はスナップショット非対応 | 論理バックアップ(本 ADR)で足りるものに限る。import キャッシュは再取得 |
| 5 | Argo CD の設定・リポジトリ Secret・レジストリ image の復元(#297) | needs-decision(レジストリを永続化するか、再作成時に push し直して digest を更新する PR を作るか) | #297 を別途決める。本 ADR は DB の部分だけ(#297 の「DB を戻す」側)を満たす |
| 6 | 定期実行(cron・CronJob)でのバックアップ | k8s では TiDB の置き場所・保存先 PVC が未決 | まず手元の手動 + launchd/cron の例を runbook に書く。CronJob 化は #2・#3 が決まってから |
| 7 | 復元後の **API 越し**の確認 | record / team の実クラスタが無い | 自動テストは表の行の有無(record-svc が読むのと同じ表)。API 越しは配備後の人間確認 |

### 8. テスト(受け入れ条件)

| AC | 内容 | テスト |
|---|---|---|
| AC-B1 | バックアップに `devices` を含む。含まない世代・ダンプは拒否 | `scripts/db-backup_test.sh`(取得側)・`scripts/db-restore_test.sh`(復元側)・`scripts/db-backup-restore_docker_test.sh`(実 DB) |
| AC-B2 | 世代取得前の削除(墓石)と期限切れが、復元後に出ない | docker テスト(X・Y)。順序は `db-restore_test.sh` |
| AC-B2b | 世代取得後の削除が、journal の再適用で復元後に出ない | docker テスト(W)。journal の別保管・追記のみは `db-backup_test.sh` |
| AC-B3 | JetStream・NATS に触れない | `db-restore_test.sh` の静的検査 |
| AC-B4 | 保存先は Git の無視対象・0700/0600・パスワードを引数に出さない | `db-backup_test.sh` |
| AC-B5 | 世代30日・journal 90日。他 kind と journal を世代の失効で巻き込まない | `db-backup_test.sh` |
| AC-B6 | 確認なし(`CONFIRM_RESTORE`)では DB に触れない。失敗時は `restore-ok` を出さない | `db-restore_test.sh` |

(ADR-0210 の AC-B1〜B9 は別の機能の条件。ここでの AC-B1〜B3 は ADR-0209 §9 のもの。AC-B4 以降は本 ADR の追加。)
`make test` は偽物テスト(`db-backup_test.sh`・`db-restore_test.sh`)だけで速い。実 DB の往復は `make test-db-backup`(Docker が無ければ失敗)。

## 却下した案

- **サービスの Go コードに復元を持たせる**: 復元はサービスが止まっている間の運用で、DB に直接 SQL を流す。サービスの `internal` を越えて使えず、
  再適用の SQL を二重に持つことになる。→ スクリプト + SQL。再適用の SQL が失効・削除 API の意味とずれないことは docker テストで固定する。
- **purge journal を世代のダンプに含めるだけ**: 世代取得後の削除要求を拾えないので、ADR-0209 §9 の要件(AC-B2b)を満たさない。
- **ファイルの mtime で世代を失効**: コピー・復元で変わる。世代 ID(取得時刻)で判定する。

## 影響

- 追加: `scripts/db-backup.sh`・`scripts/db-restore.sh`(実装は後続)、runbook、Makefile の `db-backup`・`db-restore`。
- docs/plan.md の P7-4 を更新。ADR-0209 の「追記(2026-09-24 の既知のギャップ)」は、§5 の同期で**部分的に**埋まる(間隔の穴は残る。§7 の1)。
- `.gitignore` は変更不要(`data/generated/` が既に対象)。

## 人間の確認が必要なこと

- 実クラスタ・共有 k3d での実バックアップ・復元の初回実行(DB を上書きする。`CONFIRM_RESTORE` が必須)。
- クラウドの保存先と暗号化(§7 の3)、purge journal の同時追記先(§7 の1)、#297 の方針。
