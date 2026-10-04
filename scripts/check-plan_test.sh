#!/usr/bin/env bash
# scripts/check-plan.sh の自動テスト(ADR-0172)。make test-scripts から流す。
# 一時ディレクトリに架空の docs/plan.md と docs/plan/ を作って検査する(リポジトリの実ファイルは読まない。最後の 1 件だけ実物を検査する)。
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT
readonly SCRIPT="$ROOT/scripts/check-plan.sh"

failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# fixture 名 … 正常な索引と区画を作る。
fixture() {
  local d="$work/$1"
  mkdir -p "$d/docs/plan/improvements"
  cat >"$d/docs/plan.md" <<'EOF'
# 開発計画と進行状況

| 区画 | ファイル |
|---|---|
| A | [docs/plan/a.md](plan/a.md) |
| 改善要望 | [docs/plan/improvements/](plan/improvements/) |
EOF
  cat >"$d/docs/plan/a.md" <<'EOF'
## A
- [x] X1-1 一つめ
- [ ] **X1-2 二つめ**(強調)
  - [~] X1-2a 子
- [x] X1〜X3 の範囲の確認
- [x] X1-3: コロン区切り
EOF
  printf '# 改善要望(web)\n- [ ] I-web-1 要望\n' >"$d/docs/plan/improvements/web.md"
  echo "$d"
}

# run 期待終了コード 説明 出力に含む語 ルート
run() {
  local want="$1" desc="$2" needle="$3" dir="$4"
  local rc=0 out
  out=$("$SCRIPT" "$dir" 2>&1) || rc=$?
  if [ "$rc" -ne "$want" ]; then
    ng "$desc: 終了コード $rc(期待 $want)。出力: $out"
  elif [ -n "$needle" ] && ! grep -qF -- "$needle" <<<"$out"; then
    ng "$desc: 出力に「$needle」が無い。出力: $out"
  else
    ok "$desc"
  fi
}

d=$(fixture ok)
run 0 "正常な索引と区画は通る(「X1〜X3」の範囲は ID とみなさない)" "check-plan: OK" "$d"

d=$(fixture missing-link)
rm "$d/docs/plan/a.md"
run 1 "索引のリンク先が無いと失敗する" "リンク先がありません: docs/plan/a.md" "$d"

d=$(fixture orphan)
printf '## B\n- [ ] Y1-1 孤立\n' >"$d/docs/plan/b.md"
run 1 "索引に無い区画のファイルがあると失敗する" "docs/plan/b.md が docs/plan.md の索引から辿れません" "$d"

d=$(fixture checkbox-in-index)
printf -- '- [ ] Z1-1 索引に書いたタスク\n' >>"$d/docs/plan.md"
run 1 "索引にチェックボックスの行があると失敗する" "docs/plan.md にチェックボックスの行があります" "$d"

d=$(fixture dup-line)
printf -- '- [~] X1-1 一つめ\n' >>"$d/docs/plan/improvements/web.md"
run 1 "状態だけ違う同じタスクの行が二重にあると失敗する" "同じタスクの行が複数あります" "$d"

d=$(fixture dup-id)
printf -- '- [ ] X1-2a 別の内容\n' >>"$d/docs/plan/improvements/web.md"
run 1 "同じタスク ID が別の行にあると失敗する" "タスク ID X1-2a が重複しています" "$d"

d=$(fixture allowed-dup-id)
printf -- '- [x] P4-17 一つめ\n- [x] P4-17(別レーン)二つめ\n' >>"$d/docs/plan/a.md"
run 0 "分割前からの重複 ID(許可リスト)は通る" "check-plan: OK" "$d"

d=$(fixture no-index-links)
printf '# 開発計画と進行状況\n' >"$d/docs/plan.md"
run 1 "索引にリンクが 1 件も無いと失敗する" "区画のファイルへのリンク" "$d"

run 0 "リポジトリの docs/plan.md と docs/plan/ が整合している" "check-plan: OK" "$ROOT"

if [ "$failures" -gt 0 ]; then
  echo "check-plan_test: $failures 件失敗" >&2
  exit 1
fi
echo "check-plan_test: 全件成功"
