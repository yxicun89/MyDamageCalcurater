#!/usr/bin/env bash
# scripts/tidb-operator-bootstrap.sh の自動テスト(ADR-0226)。make test-scripts から流す。
# kubectl・curl・shasum・tar・helm は偽物に差し替える(ネットワーク・クラスタに触れない)。
# 確かめること: アーカイブの sha256 が固定値と違えば helm を呼ばず非ゼロで終わる(fail closed)。一致すれば helm upgrade --install を呼ぶ。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT
failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin"
log="$work/calls.log"
want_sha=$(sed -nE 's/^archive_sha256="([0-9a-f]+)"$/\1/p' "$ROOT/scripts/tidb-operator-bootstrap.sh")
[ -n "$want_sha" ] && ok "固定の sha256 がスクリプトにある" || ng "固定の sha256 が見つからない"

for c in kubectl helm; do
  printf '#!/usr/bin/env bash\necho "%s $*" >> "${FAKE_LOG:?}"\n' "$c" > "$work/bin/$c"
done
cat > "$work/bin/curl" <<'FAKE'
#!/usr/bin/env bash
# -o <file> を探してダミーを書く(CRD の取得ではなく chart の取得のときだけ)。
while [ "$#" -gt 0 ]; do [ "$1" = "-o" ] && { echo dummy > "$2"; }; shift; done
FAKE
cat > "$work/bin/shasum" <<'FAKE'
#!/usr/bin/env bash
echo "${FAKE_SHA:?}  -"
FAKE
cat > "$work/bin/tar" <<'FAKE'
#!/usr/bin/env bash
dest=""
while [ "$#" -gt 0 ]; do [ "$1" = "-C" ] && dest="$2"; shift; done
mkdir -p "$dest/tidb-operator-x/charts/tidb-operator"
FAKE
chmod +x "$work/bin/"*

run() {
  : > "$log"
  rc=0
  env PATH="$work/bin:$PATH" FAKE_LOG="$log" FAKE_SHA="$1" "$ROOT/scripts/tidb-operator-bootstrap.sh" >"$work/out" 2>&1 || rc=$?
}

run "0000000000000000000000000000000000000000000000000000000000000000"
if [ "$rc" -ne 0 ]; then ok "sha256 が違えば非ゼロで終わる"; else ng "sha256 が違うのに成功した"; fi
if grep -q "^helm" "$log"; then ng "sha256 が違うのに helm を呼んだ"; else ok "sha256 が違えば helm を呼ばない(fail closed)"; fi
grep -q "sha256 が固定値と違う" "$work/out" && ok "理由を表示する" || ng "理由が表示されない"

run "$want_sha"
if [ "$rc" -eq 0 ]; then ok "sha256 が一致すれば成功する"; else ng "一致したのに失敗: $(cat "$work/out")"; fi
grep -q "^helm upgrade --install tidb-operator " "$log" && ok "helm upgrade --install が呼ばれる" || ng "helm upgrade --install が呼ばれない"
grep -q "^kubectl apply --server-side" "$log" && ok "CRD を server-side apply する" || ng "CRD の apply が無い"

if [ "$failures" -ne 0 ]; then echo "tidb-operator-bootstrap_test: ${failures} 件失敗" >&2; exit 1; fi
echo "tidb-operator-bootstrap_test: すべて成功"
