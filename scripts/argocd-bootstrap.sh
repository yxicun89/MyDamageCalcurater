#!/usr/bin/env bash
# 共有の Argo CD を導入する(balance・speed の runbook から呼ぶ1本の手順。ADR-0405)。
# 取得元はコミットSHA固定、取得した install.yaml は SHA-256 を検証し、3イメージを digest 参照に書き換えて
# digest 形式を検査してから `kubectl apply -n argocd --server-side -f -` する。
# 自動テスト: scripts/argocd-bootstrap_test.sh(curl・kubectl を PATH 上の偽物に差し替えて流す)。
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# --- 固定値(このファイル1箇所にだけ持つ。版を上げるときはここを明示的に更新する。ADR-0405 §1) ---
# 比較用: v3.5.3 タグが指すコミット(2026-09-23 に refs/tags/v3.5.3 で確認)
readonly ARGOCD_INSTALL_COMMIT=c9c369efcc5b2a0bd720803f8d14a1c3eaddf579
readonly EXPECTED_INSTALL_YAML_SHA256=7efe2d6bbc03f63623640f1e4198f16c84009d510fb810ef71e56df1b7614ba9
# quay.io/argoproj/argocd:v3.5.3
readonly ARGOCD_IMAGE_DIGEST=sha256:dd3f47d5a5e4da563a7a398506e892481b358a7cec50abdf320c71aa55904bfa
# ghcr.io/dexidp/dex:v2.45.1
readonly DEX_IMAGE_DIGEST=sha256:8499afd690c437f52301efd2b05b2455da5bd2dfc20332cd697dc9937f808462
# public.ecr.aws/docker/library/redis:8.2.3-alpine
readonly REDIS_IMAGE_DIGEST=sha256:08ad0b1d280850169a790dba1393ff7a90aef951fc19632cf4d3ce4f78e679ba

# 書き換え対象の3リポジトリ(タグは上流の版に追従してよいので固定しない。digest だけ固定する)。
readonly ARGOCD_REPO="quay.io/argoproj/argocd"
readonly DEX_REPO="ghcr.io/dexidp/dex"
readonly REDIS_REPO="public.ecr.aws/docker/library/redis"

readonly INSTALL_URL="https://raw.githubusercontent.com/argoproj/argo-cd/${ARGOCD_INSTALL_COMMIT}/manifests/install.yaml"

# sha256_of ファイル — macOS(shasum)・Linux(sha256sum)のどちらでも動く SHA-256。
sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{ print $1 }'
  else
    shasum -a 256 "$1" | awk '{ print $1 }'
  fi
}

# sed_escape_repo リポジトリ名 — sed 拡張正規表現のパターンとして使うため "." をエスケープする。
sed_escape_repo() { printf '%s' "$1" | sed 's/\./\\./g'; }

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

fetched="$tmp_dir/install.yaml"
pinned="$tmp_dir/install.pinned.yaml"

echo "argocd-bootstrap: install.yaml を取得します ($INSTALL_URL)" >&2
curl -fsSL -o "$fetched" "$INSTALL_URL"

actual_sha256=$(sha256_of "$fetched")
if [ -z "$EXPECTED_INSTALL_YAML_SHA256" ] || [ "$actual_sha256" != "$EXPECTED_INSTALL_YAML_SHA256" ]; then
  echo "argocd-bootstrap: install.yaml の SHA-256 が一致しません(取得内容が想定と違います)。適用を中止します" >&2
  echo "  期待: $EXPECTED_INSTALL_YAML_SHA256" >&2
  echo "  実際: $actual_sha256" >&2
  exit 1
fi

argocd_pattern=$(sed_escape_repo "$ARGOCD_REPO")
dex_pattern=$(sed_escape_repo "$DEX_REPO")
redis_pattern=$(sed_escape_repo "$REDIS_REPO")

# 各イメージの image: 行を <repo>:<tag> から <repo>@<digest> へ書き換える。他の内容には触れない。
sed -E \
  -e "s#(image:[[:space:]]*\"?)${argocd_pattern}:[^[:space:]\"']+#\1${ARGOCD_REPO}@${ARGOCD_IMAGE_DIGEST}#g" \
  -e "s#(image:[[:space:]]*\"?)${dex_pattern}:[^[:space:]\"']+#\1${DEX_REPO}@${DEX_IMAGE_DIGEST}#g" \
  -e "s#(image:[[:space:]]*\"?)${redis_pattern}:[^[:space:]\"']+#\1${REDIS_REPO}@${REDIS_IMAGE_DIGEST}#g" \
  "$fetched" >"$pinned"

# 書き換え後、すべての image: 行が digest 参照(<repo>@sha256:<64桁の16進>)になっていることを検査する。
# 3イメージ以外のタグ参照が紛れていた場合や、digest 定数が空・不正な形式だった場合もここで捕まる。
bad_images=$(grep -E '^[[:space:]]*(-[[:space:]]+)?image:' "$pinned" |
  sed -E 's/^[[:space:]]*(-[[:space:]]+)?image:[[:space:]]*//; s/^"//; s/"[[:space:]]*$//; s/[[:space:]]+$//' |
  grep -Ev '^[^@]+@sha256:[0-9a-f]{64}$' || true)
if [ -n "$bad_images" ]; then
  echo "argocd-bootstrap: digest 形式(<repo>@sha256:<64桁の16進>)でない image 参照が残っています。適用を中止します:" >&2
  printf '  %s\n' "$bad_images" >&2
  exit 1
fi

# namespace 作成は冪等に(balance が先に導入済みでも speed から再実行して壊れない。ADR-0605)。
kubectl get namespace argocd >/dev/null 2>&1 || kubectl create namespace argocd

kubectl apply -n argocd --server-side -f - <"$pinned"
kubectl -n argocd rollout status deployment/argocd-server --timeout=300s
