#!/usr/bin/env bash
# バックアップ→復元の実 DB での往復テスト(P7-4。ADR-0225。AC-B1・AC-B2・AC-B2b・AC-B3)。
# Docker の使い捨て TiDB(record DB。unistore)と MySQL(pokedex DB)を起動し、scripts/db-backup.sh・
# scripts/db-restore.sh を実際に流して、復元後に削除済み・期限切れのデータが戻らないことを DB で確かめる。
#
# - make test-db-backup から流す。make test には含めない(実 DB が要る。scripts/test-db-docker.sh と同じ流儀)。
# - Docker が無い・起動しないときはスキップせず失敗する(CLAUDE.md 絶対ルール 6)。
# - 消すのはこのスクリプトが作った使い捨てのコンテナ・ネットワーク・一時ディレクトリだけ。
# - 「API から見えない」は、record-svc が同じ表だけを読むので、表の行の有無で確かめる(API 越しの確認は E2E / 実クラスタの人間確認)。
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# イメージは scripts/test-db-docker.sh と同じ digest にする(版を二重管理しないよう、ここでは同スクリプトから読む)。
MYSQL_IMAGE="$(grep -E '^readonly MYSQL_IMAGE=' scripts/test-db-docker.sh | sed -E 's/^readonly MYSQL_IMAGE="(.*)"$/\1/')"
TIDB_IMAGE="$(grep -E '^readonly TIDB_IMAGE=' scripts/test-db-docker.sh | sed -E 's/^readonly TIDB_IMAGE="(.*)"$/\1/')"
readonly MYSQL_IMAGE TIDB_IMAGE
readonly WAIT_SECONDS="${TEST_DB_WAIT_SECONDS:-180}"

command -v docker >/dev/null 2>&1 || { echo "db-backup-restore_docker_test: docker が無い(スキップせず失敗する)" >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo "db-backup-restore_docker_test: Docker が起動していない(スキップせず失敗する)" >&2; exit 1; }

suffix="$$-$(date +%s)"
readonly NETWORK="pokecalc-bkp-${suffix}"
readonly MYSQL_NAME="pokecalc-bkp-mysql-${suffix}"
readonly TIDB_NAME="pokecalc-bkp-tidb-${suffix}"
work="$(mktemp -d)"
readonly work

cleanup() {
  docker rm -f "$MYSQL_NAME" "$TIDB_NAME" >/dev/null 2>&1 || true
  docker network rm "$NETWORK" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

pw="$(openssl rand -hex 16)"
export MYSQL_ROOT_PASSWORD="$pw"
docker network create "$NETWORK" >/dev/null
docker run -d --name "$MYSQL_NAME" --network "$NETWORK" -p 127.0.0.1::3306 -e MYSQL_ROOT_PASSWORD -e MYSQL_DATABASE=pokedex_test "$MYSQL_IMAGE" >/dev/null
docker run -d --name "$TIDB_NAME" --network "$NETWORK" -p 127.0.0.1::4000 "$TIDB_IMAGE" >/dev/null

# MySQL コンテナ内のクライアントを使うラッパー(ホストに mysql / mysqldump が無くても流せる)。
# 第1引数 = 接続先 mysql|tidb。残りは mysql / mysqldump にそのまま渡す。パスワードは環境変数で渡す。
cat > "$work/mysql" <<EOF
#!/usr/bin/env bash
exec docker exec -i -e MYSQL_PWD "$MYSQL_NAME" mysql "\$@"
EOF
cat > "$work/mysqldump" <<EOF
#!/usr/bin/env bash
exec docker exec -i -e MYSQL_PWD "$MYSQL_NAME" mysqldump "\$@"
EOF
chmod +x "$work/mysql" "$work/mysqldump"

# 接続の環境変数を切り替える(db-backup.sh / db-restore.sh の入力)。
use_tidb()   { export DB_HOST="$TIDB_NAME" DB_PORT=4000 DB_USER=root DB_NAME=record_test; export MYSQL_PWD=""; }
use_mysql()  { export DB_HOST="$MYSQL_NAME" DB_PORT=3306 DB_USER=root DB_NAME=pokedex_test; export MYSQL_PWD="$MYSQL_ROOT_PASSWORD"; }
sql() { "$work/mysql" -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -N -B "$DB_NAME" -e "$1"; }

waited=0
use_mysql
until sql "SELECT 1" >/dev/null 2>&1; do
  [ "$waited" -ge "$WAIT_SECONDS" ] && { echo "MySQL が起動しない" >&2; exit 1; }
  sleep 2; waited=$((waited + 2))
done
use_tidb
export DB_NAME=mysql
until sql "SELECT 1" >/dev/null 2>&1; do
  [ "$waited" -ge "$WAIT_SECONDS" ] && { echo "TiDB が起動しない" >&2; exit 1; }
  sleep 2; waited=$((waited + 2))
done
sql "CREATE DATABASE IF NOT EXISTS record_test; SET GLOBAL tidb_skip_isolation_level_check=1;" >/dev/null

tidb_port="$(docker port "$TIDB_NAME" 4000/tcp | head -1 | sed 's/.*://')"
(cd services && RECORD_DATABASE_DSN="root:@tcp(127.0.0.1:${tidb_port})/record_test?parseTime=true" go run ./record/cmd/migrate up)

export BACKUP_DIR="$work/backups" MYSQL_BIN="$work/mysql" MYSQLDUMP_BIN="$work/mysqldump"
# 失効ジョブ(record expire)を実 DB に向けて流すラッパー。保持日数は ADR-0209 §3 の既定。
cat > "$work/expire" <<EOF
#!/usr/bin/env bash
cd "$PWD/services"
export RECORD_APP_DSN="root:@tcp(127.0.0.1:${tidb_port})/record_test?parseTime=true"
export RECORD_CALC_EVENTS_RETENTION_DAYS=90 RECORD_FAVORITES_RETENTION_DAYS=540 RECORD_DEVICE_ROW_EXPIRY_DAYS=30
export RECORD_PURGE_JOURNAL_RETENTION_DAYS=90 RECORD_EXPIRE_BATCH_LIMIT=1000
exec go run ./record/cmd/record expire
EOF
chmod +x "$work/expire"
export RESTORE_EXPIRE_CMD="$work/expire"

# ---- 初期データ(record DB)
# X: 世代取得前に削除要求済み(墓石あり。部分削除でイベントが残っている想定)  → 復元後に消える(AC-B2)
# Y: 90日より古い期限切れイベントと、新しいイベントを持つ                    → 古い方だけ消える(AC-B2)
# Z: 普通の端末                                                              → 残る
# W: 世代取得後に削除される端末(この時点では普通に存在)                      → 復元後に消える(AC-B2b)
use_tidb
ev() { echo "INSERT INTO calc_events (event_id, device_id, session_id, operation, occurred_at, defender_species_key, payload, created_at) VALUES ('$1','$2','s','calc',$3,'','{}',NOW(6));"; }
sql "
INSERT INTO devices (device_id, last_seen_at, purged_at) VALUES
  ('dev-x', NOW(6), DATE_SUB(NOW(6), INTERVAL 1 DAY)), ('dev-y', NOW(6), NULL), ('dev-z', NOW(6), NULL), ('dev-w', NOW(6), NULL);
$(ev ex1 dev-x 'DATE_SUB(NOW(6), INTERVAL 2 DAY)')
$(ev ey-old dev-y 'DATE_SUB(NOW(6), INTERVAL 100 DAY)')
$(ev ey-new dev-y 'DATE_SUB(NOW(6), INTERVAL 1 DAY)')
$(ev ez1 dev-z 'DATE_SUB(NOW(6), INTERVAL 1 DAY)')
$(ev ew1 dev-w 'DATE_SUB(NOW(6), INTERVAL 1 DAY)')
INSERT INTO favorites (device_id, species_key, snapshot, created_at, updated_at) VALUES ('dev-w', 'k', '{}', NOW(6), NOW(6));
" >/dev/null
count() { sql "SELECT COUNT(*) FROM $1 WHERE device_id='$2'$3"; }

# ---- 世代を取る
scripts/db-backup.sh full record
gen="$(ls "$BACKUP_DIR/record" | tail -1)"
if [ -f "$BACKUP_DIR/record/$gen/dump.sql.gz" ] && gzip -dc "$BACKUP_DIR/record/$gen/dump.sql.gz" | grep -q 'CREATE TABLE `devices`'; then
  ok "実 DB のバックアップに devices が入る(AC-B1)"
else
  ng "実 DB のダンプに devices が無い"
fi

# ---- 世代取得後に W の削除要求(本番の PurgeDevice と同じ状態: 墓石・journal・行の削除)。journal を別保管へ同期する。
sql "
UPDATE devices SET purged_at = NOW(6) WHERE device_id='dev-w';
INSERT INTO purge_journal (device_id, requested_at) VALUES ('dev-w', NOW(6));
DELETE FROM calc_events WHERE device_id='dev-w';
DELETE FROM favorites WHERE device_id='dev-w';
" >/dev/null
scripts/db-backup.sh journal record
if grep -q '^dev-w' "$BACKUP_DIR/journal/record.tsv"; then ok "世代取得後の削除要求が、世代とは別の journal に残る"; else ng "journal に dev-w が無い"; fi

# ---- 障害: DB を空から作り直す(クラスタ全損の想定。journal は別の場所なので残る)
sql "DROP DATABASE record_test; CREATE DATABASE record_test;" >/dev/null 2>&1 || { export DB_NAME=mysql; sql "DROP DATABASE record_test; CREATE DATABASE record_test;" >/dev/null; export DB_NAME=record_test; }

# ---- 復元(確認の値は DB 名)
CONFIRM_RESTORE=record_test scripts/db-restore.sh record "$gen" | tee "$work/restore.out" >/dev/null || ng "db-restore.sh が失敗した"
if tail -1 "$work/restore.out" | grep -q '^restore-ok'; then ok "復元が最後まで成功し restore-ok を出す"; else ng "restore-ok が無い: $(cat "$work/restore.out")"; fi

if [ "$(count calc_events dev-x)" = 0 ]; then ok "世代取得前に削除済みの端末 X のイベントが復元後に出ない(墓石の再適用。AC-B2)"; else ng "dev-x のイベントが復活した"; fi
if [ "$(count calc_events dev-y " AND event_id='ey-old'")" = 0 ] && [ "$(count calc_events dev-y " AND event_id='ey-new'")" = 1 ]; then
  ok "期限切れイベントは復元後に消え、期限内は残る(失効ジョブの強制実行。AC-B2)"
else
  ng "Y の失効が正しくない"
fi
if [ "$(count calc_events dev-w)" = 0 ] && [ "$(count favorites dev-w)" = 0 ]; then
  ok "世代取得後に削除された端末 W のデータが復元後に出ない(journal の再適用。AC-B2b)"
else
  ng "dev-w のデータが復活した"
fi
if [ "$(count calc_events dev-z)" = 1 ]; then ok "無関係の端末 Z のデータは残る"; else ng "dev-z のデータが消えた"; fi
if [ "$(sql "SELECT COUNT(*) FROM purge_journal WHERE device_id='dev-w'")" -ge 1 ] && [ "$(sql "SELECT COUNT(*) FROM devices WHERE device_id='dev-w' AND purged_at IS NOT NULL")" = 1 ]; then
  ok "復元後の DB に dev-w の journal と墓石が戻っている(イベントの再出現を抑止できる)"
else
  ng "dev-w の journal / 墓石が復元後の DB に無い"
fi

# ---- AC-B3: 復元スクリプトは JetStream に触れない(静的には db-restore_test.sh が見る)。ここでは実行中に nats コンテナを作っていないことだけ確かめる。
if docker ps -a --format '{{.Names}}' | grep -q nats; then ng "復元の途中で nats に関わるコンテナがある"; else ok "復元は NATS / JetStream を使わない(AC-B3)"; fi

# ---- pokedex(MySQL): マスタの往復
use_mysql
sql "CREATE TABLE species (k VARCHAR(16) PRIMARY KEY); INSERT INTO species VALUES ('a'),('b'),('c');" >/dev/null
scripts/db-backup.sh full pokedex
pgen="$(ls "$BACKUP_DIR/pokedex" | tail -1)"
sql "DROP TABLE species;" >/dev/null
CONFIRM_RESTORE=pokedex_test scripts/db-restore.sh pokedex "$pgen" >/dev/null || ng "pokedex の復元が失敗した"
if [ "$(sql "SELECT COUNT(*) FROM species")" = 3 ]; then ok "pokedex(MySQL)のダンプを復元すると同じ件数が戻る"; else ng "pokedex の件数が戻らない"; fi

if [ "$failures" -ne 0 ]; then
  echo "db-backup-restore_docker_test: ${failures} 件失敗" >&2
  exit 1
fi
echo "db-backup-restore_docker_test: すべて成功"
