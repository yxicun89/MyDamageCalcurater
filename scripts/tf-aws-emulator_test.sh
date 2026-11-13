#!/usr/bin/env bash
# scripts/tf-aws-emulator.sh と scripts/lib/tf-targets.py の自動テスト(ADR-0815)。
# `make test-scripts`(make test に含む)から流す。本物のエミュレータ・terraform・ネットワークには触らない
# (aws-emulator・curl・terraform は PATH の先頭に置いた偽物に差し替える)。
#
# 固定すること:
#   - STACK は infra/terraform/aws/ の下だけ受け付け、.. を拒否する(GCP やリポジトリ外をエミュレータに流さない)
#   - terraform には、本物の AWS へ向きうる環境変数(鍵・プロファイル・サービス別エンドポイント・TF_VAR_*)が渡らず、
#     エンドポイントは 127.0.0.1、鍵はエミュレータの認証情報ファイルだけになる
#   - 本物の値が入りうる *.tfvars を作業ディレクトリへ持ち込まない
#   - 既知の差があるとき、除外したリソースを -target に入れない
#   - tf-targets.py は、除外したリソースと、それを参照するリソース(入れ子のブロック・count・depends_on・間接の参照)を外し、
#     構成に無い除外アドレスは失敗にする
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT

failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

# --- STACK の検査(エミュレータの起動より前に、決まった文言で止まる) ---
check_rejected() {
  local stack=$1 want=$2 err
  err=$(cd "$ROOT" && STACK="$stack" PATH="$TMP/empty:/usr/bin:/bin" ./scripts/tf-aws-emulator.sh 2>&1 >/dev/null)
  if echo "$err" | grep -qF "$want"; then
    ok "STACK=$stack を「$want」で拒否する"
  else
    ng "STACK=$stack の拒否の文言が違う: [$err]"
  fi
}
mkdir -p "$TMP/empty"
check_rejected infra/terraform/gcp/10-network "STACK は infra/terraform/aws/ の下だけ指定できる"
check_rejected /tmp/outside "STACK は infra/terraform/aws/ の下だけ指定できる"
check_rejected infra/terraform/aws/../gcp/10-network "STACK に .. は使えない"

# --- 環境の隔離(偽の aws-emulator・curl・terraform で最後まで流す) ---
BIN="$TMP/bin"
mkdir -p "$BIN" "$TMP/home"
cat >"$BIN/aws-emulator" <<'EOS'
#!/usr/bin/env bash
port=9297
while [ $# -gt 0 ]; do [ "$1" = -p ] && port=$2; shift; done
d="$HOME/.aws/emulator/$port"
mkdir -p "$d" && printf '[default]\naws_access_key_id = EMU\n' >"$d/emulator_credentials" && : >"$d/ca.pem" && : >"$d/config"
exec sleep 30
EOS
# 起動前のポート確認(-k を付けた呼び出し)には「空いている」、起動後のヘルスチェックには「準備できた」と答える
cat >"$BIN/curl" <<'EOS'
#!/usr/bin/env bash
for a in "$@"; do [ "$a" = -sk ] && exit 7; done
exit 0
EOS
cat >"$BIN/terraform" <<'EOS'
#!/usr/bin/env bash
dir=""
for a in "$@"; do case "$a" in -chdir=*) dir=${a#-chdir=} ;; esac; done
{
  echo "ARGS $*"
  env | grep -E '^(AWS_|TF_VAR_|TF_CLI_ARGS)' | sort
  ls "$dir"
} >>"$TF_FAKE_LOG"
case " $* " in
  *" show "*) cat "$TF_FAKE_PLAN_JSON" ;;
esac
exit 0
EOS
chmod +x "$BIN"/*

STACK_DIR="infra/terraform/aws/zz-test-$$"
mkdir -p "$ROOT/$STACK_DIR"
trap 'rm -rf "$TMP" "$ROOT/$STACK_DIR"' EXIT
: >"$ROOT/$STACK_DIR/versions.tf"
echo 'owner = "real-value"' >"$ROOT/$STACK_DIR/terraform.tfvars"
echo 'aws_security_group.vm' >"$ROOT/$STACK_DIR/emulator-known-gaps.txt"
cat >"$TMP/plan.json" <<'EOS'
{"configuration": {"root_module": {"resources": [
  {"address": "aws_vpc.this", "expressions": {}},
  {"address": "aws_security_group.vm", "expressions": {"vpc_id": {"references": ["aws_vpc.this.id", "aws_vpc.this"]}}}
]}},
 "resource_changes": [{"address": "aws_vpc.this"}, {"address": "aws_security_group.vm"}]}
EOS

log="$TMP/tf.log"
(cd "$ROOT" && env HOME="$TMP/home" PATH="$BIN:$PATH" STACK="$STACK_DIR" AWS_EMULATOR_PORT=19297 \
  TF_FAKE_LOG="$log" TF_FAKE_PLAN_JSON="$TMP/plan.json" \
  AWS_ACCESS_KEY_ID=REAL AWS_SECRET_ACCESS_KEY=REAL AWS_SESSION_TOKEN=REAL AWS_PROFILE=prod \
  AWS_ENDPOINT_URL_EC2=https://ec2.example.invalid AWS_ROLE_ARN=arn-real TF_VAR_owner=real TF_CLI_ARGS_apply=-real \
  ./scripts/tf-aws-emulator.sh >"$TMP/out.txt" 2>&1)
status=$?

if [ "$status" -eq 0 ] && grep -q "tf-aws-emulator: OK" "$TMP/out.txt"; then
  ok "偽のエミュレータ・terraform で最後まで流れる"
else
  ng "偽物での実行が失敗した(終了コード $status): $(cat "$TMP/out.txt")"
fi
leaked=$(grep -E '^(AWS_ACCESS_KEY_ID|AWS_SECRET_ACCESS_KEY|AWS_SESSION_TOKEN|AWS_PROFILE|AWS_ENDPOINT_URL_EC2|AWS_ROLE_ARN|TF_VAR_owner|TF_CLI_ARGS_apply)=' "$log" | sort -u)
if [ -z "$leaked" ]; then
  ok "本物の AWS へ向きうる環境変数が terraform に渡らない"
else
  ng "terraform に渡った: $leaked"
fi
if grep -q '^AWS_ENDPOINT_URL=https://127.0.0.1:19297$' "$log" &&
  ! grep '^AWS_ENDPOINT_URL=' "$log" | grep -qv '127.0.0.1:19297' &&
  grep -q "^AWS_SHARED_CREDENTIALS_FILE=$TMP/home/.aws/emulator/19297/emulator_credentials$" "$log"; then
  ok "エンドポイントは 127.0.0.1、鍵はエミュレータの認証情報ファイル"
else
  ng "エンドポイント・認証情報ファイルが違う: $(grep -E '^AWS_(ENDPOINT_URL|SHARED_CREDENTIALS_FILE)=' "$log" | sort -u)"
fi
if grep -q 'terraform.tfvars' "$log"; then
  ng "terraform.tfvars を作業ディレクトリへ持ち込んだ"
else
  ok "*.tfvars を作業ディレクトリへ持ち込まない"
fi
apply_line=$(grep '^ARGS .* apply ' "$log" | head -1)
if echo "$apply_line" | grep -q -- '-target=aws_vpc.this' && ! echo "$apply_line" | grep -q 'aws_security_group'; then
  ok "既知の差のリソースを除いて -target で apply する"
else
  ng "apply の引数が違う: $apply_line"
fi

# --- tf-targets.py ---
plan_json='{
  "configuration": {"root_module": {"resources": [
    {"address": "aws_vpc.this", "expressions": {"cidr_block": {"references": ["var.vpc_cidr"]}}},
    {"address": "aws_security_group.vm", "expressions": {"vpc_id": {"references": ["aws_vpc.this.id", "aws_vpc.this"]}}},
    {"address": "aws_vpc_security_group_egress_rule.vm_all", "expressions": {"security_group_id": {"references": ["aws_security_group.vm.id", "aws_security_group.vm"]}}},
    {"address": "aws_instance.uses_rule", "depends_on": ["aws_vpc_security_group_egress_rule.vm_all"]},
    {"address": "aws_instance.nested", "expressions": {"network_interface": [{"security_groups": {"references": ["aws_security_group.vm.id", "aws_security_group.vm"]}}]}},
    {"address": "aws_eip.counted", "count_expression": {"references": ["aws_security_group.vm"]}},
    {"address": "aws_eip.nat", "expressions": {}}
  ]}},
  "resource_changes": [
    {"address": "aws_vpc.this"},
    {"address": "aws_security_group.vm"},
    {"address": "aws_vpc_security_group_egress_rule.vm_all"},
    {"address": "aws_instance.uses_rule"},
    {"address": "aws_instance.nested"},
    {"address": "aws_eip.counted[0]"},
    {"address": "aws_eip.nat[0]"}
  ]
}'
got=$(echo "$plan_json" | python3 "$ROOT/scripts/lib/tf-targets.py" aws_security_group.vm | tr '\n' ' ')
want="aws_vpc.this aws_eip.nat[0] "
if [ "$got" = "$want" ]; then
  ok "tf-targets.py は除外したリソースと、それを参照するリソース(入れ子・count・depends_on・間接)を外す"
else
  ng "tf-targets.py の出力が違う: got=[$got] want=[$want]"
fi

got=$(echo "$plan_json" | python3 "$ROOT/scripts/lib/tf-targets.py" | wc -l | tr -d ' ')
if [ "$got" = "7" ]; then
  ok "tf-targets.py は除外が無ければ全リソースを出す"
else
  ng "tf-targets.py が除外なしで $got 件を出した(期待 7)"
fi

if echo "$plan_json" | python3 "$ROOT/scripts/lib/tf-targets.py" aws_security_group.typo >/dev/null 2>&1; then
  ng "tf-targets.py が構成に無い除外アドレスを受け付けた"
else
  ok "tf-targets.py は構成に無い除外アドレスで失敗する"
fi

if [ "$failures" -gt 0 ]; then
  echo "tf-aws-emulator_test: $failures 件失敗" >&2
  exit 1
fi
echo "tf-aws-emulator_test: OK"
