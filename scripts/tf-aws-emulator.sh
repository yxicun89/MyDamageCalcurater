#!/usr/bin/env bash
# make tf-aws-emulator: AWS の Terraform を AWS Emulator(手元で動く AWS API の模造品)に対して
# apply → 差分 0 の plan → destroy まで流す(ADR-0815)。本物の AWS には触れない・課金されない。
#
# 本物の AWS へ向かないための二重の安全策:
#   1. AWS_* の環境変数(サービス別のエンドポイント・鍵・プロファイル・ロール)を全部外し、
#      AWS_ENDPOINT_URL を https://127.0.0.1:<port>、CA をエミュレータの証明書にする
#   2. 認証情報はエミュレータがその場で作る鍵だけ(本物の AWS では通らない)
# エミュレータの起動: PATH に aws-emulator(Linux パッケージ)があればそれを、無ければ Docker でイメージを作って使う。
# 使い方: make tf-aws-emulator(STACK=infra/terraform/aws/10-network が既定)。
# 自動テスト: scripts/tf-aws-emulator_test.sh(make test-scripts)。
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

STACK=${STACK:-infra/terraform/aws/10-network}
PORT=${AWS_EMULATOR_PORT:-9297}
AWS_EMULATOR_VERSION=1.0.343
# Docker で動かすときの RPM の SHA-256(版を上げたら作り直す。infra/terraform/README.md)
AWS_EMULATOR_SHA256_X86_64=0edd95d336731304c18249436545e4e1fe0702bba3a1e5119faf880ec8f1c12e
AWS_EMULATOR_SHA256_AARCH64=8d14dbfa9d93e1b46a3cafe9cf6f7ec95096f21c9a486867add444b201bfe1b3
IMAGE=aws-emulator:${AWS_EMULATOR_VERSION}
EMU_DIR="$HOME/.aws/emulator"

case "$STACK" in
  *..*)
    echo "tf-aws-emulator: STACK に .. は使えない: $STACK" >&2
    exit 1
    ;;
  infra/terraform/aws/*) ;;
  *)
    echo "tf-aws-emulator: STACK は infra/terraform/aws/ の下だけ指定できる: $STACK" >&2
    exit 1
    ;;
esac
[ -f "$STACK/versions.tf" ] || { echo "tf-aws-emulator: $STACK に Terraform の構成が無い" >&2; exit 1; }
command -v terraform >/dev/null || { echo "tf-aws-emulator: terraform が無い(infra/terraform/README.md)" >&2; exit 1; }

# 本物の AWS へ向きうる環境変数を全部外す(サービス別の AWS_ENDPOINT_URL_* は AWS_ENDPOINT_URL より優先される)。
# TF_VAR_*・TF_CLI_ARGS* も外し、変数と引数はこのスクリプトが渡すものだけにする
for name in $(env | sed -n 's/^\(AWS_[A-Za-z0-9_]*\)=.*/\1/p; s/^\(TF_VAR_[A-Za-z0-9_]*\)=.*/\1/p; s/^\(TF_CLI_ARGS[A-Za-z0-9_]*\)=.*/\1/p'); do
  unset "$name"
done
export CHECKPOINT_DISABLE=1
export AWS_ENDPOINT_URL="https://127.0.0.1:$PORT"
export AWS_SHARED_CREDENTIALS_FILE="$EMU_DIR/$PORT/emulator_credentials"
export AWS_CONFIG_FILE="$EMU_DIR/$PORT/config"
export AWS_CA_BUNDLE="$EMU_DIR/$PORT/ca.pem"
export AWS_EC2_METADATA_DISABLED=true

# 別のエミュレータ(前回の残り)が同じポートで動いていると、そちらに流してしまうので止まる
if curl -sk --max-time 2 "$AWS_ENDPOINT_URL/__admin/health" >/dev/null 2>&1; then
  echo "tf-aws-emulator: ポート $PORT で既に何かが動いている。止めるか AWS_EMULATOR_PORT を変える" >&2
  exit 1
fi

WORK=$(mktemp -d)
EMU_PID=""
EMU_CONTAINER=""
cleanup() {
  if [ -n "$EMU_PID" ]; then
    kill "$EMU_PID" 2>/dev/null || true
    wait "$EMU_PID" 2>/dev/null || true
  fi
  if [ -n "$EMU_CONTAINER" ]; then
    docker stop "$EMU_CONTAINER" >/dev/null 2>&1 || true
  fi
  rm -rf "$WORK"
}
trap cleanup EXIT

mkdir -p "$EMU_DIR"
if command -v aws-emulator >/dev/null; then
  aws-emulator -p "$PORT" --telemetry off --idle-timeout 900 >"$WORK/emulator.log" 2>&1 &
  EMU_PID=$!
elif command -v docker >/dev/null; then
  case "$(uname -m)" in
    arm64 | aarch64) ARCH=aarch64 SHA256=$AWS_EMULATOR_SHA256_AARCH64 ;;
    *) ARCH=x86_64 SHA256=$AWS_EMULATOR_SHA256_X86_64 ;;
  esac
  if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
    docker build -q -t "$IMAGE" --build-arg AWS_EMULATOR_VERSION="$AWS_EMULATOR_VERSION" \
      --build-arg AWS_EMULATOR_ARCH="$ARCH" --build-arg AWS_EMULATOR_SHA256="$SHA256" \
      infra/terraform/aws-emulator >/dev/null
  fi
  # -p は 127.0.0.1 に限る(公式の注意: 短い形はネットワーク全体へ開く)。-e HOME で中と外のパスをそろえる
  EMU_CONTAINER=$(docker run -d --rm --memory=2g --memory-swap=2g -p "127.0.0.1:$PORT:$PORT" \
    --user "$(id -u):$(id -g)" -e HOME="$HOME" -v "$EMU_DIR:$EMU_DIR" \
    "$IMAGE" -p "$PORT" --telemetry off --idle-timeout 900)
else
  echo "tf-aws-emulator: aws-emulator も docker も無い(infra/terraform/README.md)" >&2
  exit 1
fi

ready=""
for _ in $(seq 60); do
  if [ -f "$AWS_CA_BUNDLE" ] && curl -sf --cacert "$AWS_CA_BUNDLE" "$AWS_ENDPOINT_URL/__admin/health" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
if [ -z "$ready" ]; then
  echo "tf-aws-emulator: エミュレータが 60 秒で起動しなかった" >&2
  if [ -f "$WORK/emulator.log" ]; then cat "$WORK/emulator.log" >&2; fi
  if [ -n "$EMU_CONTAINER" ]; then docker logs "$EMU_CONTAINER" >&2 || true; fi
  exit 1
fi
if ! grep -q aws_access_key_id "$AWS_SHARED_CREDENTIALS_FILE" 2>/dev/null; then
  echo "tf-aws-emulator: エミュレータの認証情報($AWS_SHARED_CREDENTIALS_FILE)が無い" >&2
  exit 1
fi

# state と .terraform を作業用の一時ディレクトリに閉じる(リポジトリに state を残さない)。
# 本物の値が入りうる *.tfvars(自動で読まれる terraform.tfvars・*.auto.tfvars を含む)は持ち込まない
cp -R "$STACK/." "$WORK/stack"
rm -rf "$WORK/stack/.terraform" "$WORK/stack"/*.tfstate* "$WORK/stack"/*.tfvars "$WORK/stack"/*.tfvars.json
TF=(terraform -chdir="$WORK/stack")
VARS=(-input=false -var owner=emulator)

"${TF[@]}" init -input=false -backend=false >/dev/null

# エミュレータの既知の差(STACK/emulator-known-gaps.txt)に書いたリソースと、それを参照するリソースを除いて流す。
# 除くものがあるときだけ、残りを -target で指定する(除いたものは最後に「未検証」として出す)
GAPS=()
if [ -f "$STACK/emulator-known-gaps.txt" ]; then
  while IFS= read -r line; do
    line=${line%%#*}
    line=${line//[[:space:]]/}
    if [ -n "$line" ]; then GAPS+=("$line"); fi
  done <"$STACK/emulator-known-gaps.txt"
fi
TARGETS=()
if [ "${#GAPS[@]}" -gt 0 ]; then
  "${TF[@]}" plan -out="$WORK/all.tfplan" "${VARS[@]}" >/dev/null
  "${TF[@]}" show -json "$WORK/all.tfplan" >"$WORK/all.json"
  python3 scripts/lib/tf-targets.py "${GAPS[@]}" <"$WORK/all.json" >"$WORK/targets.txt"
  while IFS= read -r addr; do
    TARGETS+=(-target="$addr")
  done <"$WORK/targets.txt"
fi

# 空の配列を "${A[@]}" で展開すると bash 3.2(Mac)の set -u で止まるため、${A[@]+...} の形にする
echo "== $STACK: apply(AWS Emulator ${AWS_ENDPOINT_URL})"
"${TF[@]}" apply -auto-approve "${VARS[@]}" ${TARGETS[@]+"${TARGETS[@]}"}
echo "== plan(差分 0 であること)"
"${TF[@]}" plan -detailed-exitcode "${VARS[@]}" ${TARGETS[@]+"${TARGETS[@]}"}
echo "== destroy"
"${TF[@]}" destroy -auto-approve "${VARS[@]}"
echo "tf-aws-emulator: OK($STACK)"
if [ "${#GAPS[@]}" -gt 0 ]; then
  echo "エミュレータで未検証(emulator-known-gaps.txt と、それを参照するリソース): ${GAPS[*]}"
fi
