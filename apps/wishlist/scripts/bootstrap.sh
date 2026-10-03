#!/usr/bin/env bash
# wishlist の DB とユーザー、Secret wishlist-api を用意する(1 回だけ。再実行しても既存は上書きしない)。
#
#   apps/wishlist/scripts/bootstrap.sh
#
# - 共有 MySQL(pokecalc 名前空間の mysql-0)に DB `wishlist` とユーザー `wishlist` を作る。
#   root パスワードは Secret pokecalc/mysql-auth から読み、端末やコマンドラインには出さない(stdin で渡す)。
# - wishlist 名前空間に Secret wishlist-api(database-dsn・api-token。環境変数 WISHLIST_YAHOO_APPID があれば yahoo-appid も)を作る。
#   値は Git に置かない。Yahoo!ショッピングの appid は任意(無ければ入れず、api 型のサイトは取得しない)。
# - API トークンは最後に 1 回だけ表示する(PWA の設定画面・iOS ショートカットに入れる)。
set -euo pipefail

mysql_ns=${MYSQL_NAMESPACE:-pokecalc}
mysql_pod=${MYSQL_POD:-mysql-0}
ns=wishlist

kubectl get namespace "$ns" >/dev/null 2>&1 || kubectl create namespace "$ns"

if kubectl -n "$ns" get secret wishlist-api >/dev/null 2>&1; then
  echo "Secret '$ns/wishlist-api' は既に存在します。何もしません(作り直すときは人が削除してから)。"
  exit 0
fi

root_pw="$(kubectl -n "$mysql_ns" get secret mysql-auth -o jsonpath='{.data.mysql-root-password}' | base64 -d)"
if [ -z "$root_pw" ]; then
  echo "Secret $mysql_ns/mysql-auth の mysql-root-password が読めません(make up 済みか確認)" >&2
  exit 1
fi

user_pw="$(openssl rand -hex 16)"
api_token="$(openssl rand -hex 32)"

# パスワードを含む SQL は stdin で渡す(ps に出さない)。MYSQL_PWD も exec の env に載らないよう sh -c 内で読む。
# 値は openssl rand -hex の 16 進文字列だけなので、SQL に埋め込んでも引用符を壊さない。
{
  printf '%s\n' "$root_pw"
  cat <<SQL
CREATE DATABASE IF NOT EXISTS wishlist CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
CREATE USER IF NOT EXISTS 'wishlist'@'%' IDENTIFIED BY '${user_pw}';
ALTER USER 'wishlist'@'%' IDENTIFIED BY '${user_pw}';
GRANT ALL PRIVILEGES ON wishlist.* TO 'wishlist'@'%';
SQL
} | kubectl -n "$mysql_ns" exec -i "$mysql_pod" -- sh -c 'IFS= read -r MYSQL_PWD; export MYSQL_PWD; mysql -uroot'

# API サーバー用の DSN。multiStatements は付けない(migrate だけが内部で付ける。docs/design.md W-09)。
dsn="wishlist:${user_pw}@tcp(mysql.${mysql_ns}.svc.cluster.local:3306)/wishlist?parseTime=true&loc=UTC"
# 値は argv に載せない(--from-literal は ps に出る)。プロセス置換のファイル経由で渡す。
secret_args=(
  --from-file=database-dsn=<(printf '%s' "$dsn")
  --from-file=api-token=<(printf '%s' "$api_token")
)
yahoo_appid="$(printf '%s' "${WISHLIST_YAHOO_APPID:-}" | tr -d '[:space:]')"
if [ -n "$yahoo_appid" ]; then
  secret_args+=(--from-file=yahoo-appid=<(printf '%s' "$yahoo_appid"))
fi
kubectl -n "$ns" create secret generic wishlist-api "${secret_args[@]}" >/dev/null

echo "DB 'wishlist' と Secret '$ns/wishlist-api' を作りました。"
echo "API トークン(PWA の設定・ショートカットに入れる。再表示: kubectl -n $ns get secret wishlist-api -o jsonpath='{.data.api-token}' | base64 -d):"
echo "$api_token"
