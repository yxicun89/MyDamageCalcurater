#!/usr/bin/env bash
# scripts/pr-merge.sh <PR番号> — 検証ゲートを通った PR だけをマージする(ADR-0801)。
#
# AI エージェントが main へ反映する唯一の経路。素の `gh pr merge` は bash-guard が止める(ADR-0800 §2)ので、
# マージは必ずこのスクリプトを通す。ゲートに通らない PR はマージしない(非0で終了。理由を stderr に出す)。
#
# ゲート(すべて満たすときだけマージする):
#   1. PR が OPEN・ドラフトでない・コンフリクトが無い。
#   2. GitHub の checks が1件でも在るなら、すべて成功(失敗・実行中・保留があれば中止)。
#      checks が1件も無い(CI が走っていない)ときは、手順 4 のローカル検証が CI の代わり。
#   3. 次のファイルを変更する PR は対象外(人間がマージする): AI の権限・ガード自体(.claude/・.codex/・
#      scripts/ai-guard/・このスクリプト)、クラウドへのデプロイ・費用に関わる設定(deploy/k8s/overlays/cloud/・
#      terraform/・.github/workflows/ のデプロイ系)。ゲート自身を AI が緩めて通すことを防ぐ。
#   4. PR の先頭コミットを使い捨ての worktree に取り出し、`make lint`・`make check-publishable`・`make test` を通す。
#      engine/ または testdata/golden/ を変える PR は `make test-golden` も(make test に含まれるが明示して確認)。
#      web/ に package-lock.json があれば先に `npm ci` する。機密・実データ・個人情報の混入は
#      check-publishable が検査する(ユーザー指示 2026-10-03: 機密の公開と費用の発生だけは止める)。
#
# マージ方式は --merge(従来どおり。履歴にブランチの区切りを残す)。ブランチは削除しない。
# 使い方: scripts/pr-merge.sh 123            # ゲート → マージ
#         scripts/pr-merge.sh 123 --check    # ゲートだけ(マージしない)
set -euo pipefail

usage() {
  printf '使い方: %s <PR番号> [--check]\n' "$0" >&2
  exit 2
}

[ $# -ge 1 ] && [ $# -le 2 ] || usage
PR=$1
CHECK_ONLY=0
if [ $# -eq 2 ]; then
  [ "$2" = "--check" ] || usage
  CHECK_ONLY=1
fi
case "$PR" in
  '' | *[!0-9]*) usage ;;
esac

die() {
  printf 'pr-merge: 中止 - %s\n' "$1" >&2
  exit 1
}

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

# 1. PR の状態
json=$(gh pr view "$PR" --json state,isDraft,mergeable,headRefOid,baseRefName,headRefName) || die "PR #$PR を取得できません"
state=$(printf '%s' "$json" | jq -r .state)
draft=$(printf '%s' "$json" | jq -r .isDraft)
mergeable=$(printf '%s' "$json" | jq -r .mergeable)
sha=$(printf '%s' "$json" | jq -r .headRefOid)
base=$(printf '%s' "$json" | jq -r .baseRefName)
[ "$state" = "OPEN" ] || die "PR #$PR は OPEN ではありません($state)"
[ "$draft" = "false" ] || die "PR #$PR はドラフトです"
[ "$mergeable" != "CONFLICTING" ] || die "PR #$PR はコンフリクトしています。base($base)を取り込んでから再実行してください"

# 2. GitHub の checks
checks=$(gh pr checks "$PR" --json name,bucket 2>/dev/null || printf '[]')
total=$(printf '%s' "$checks" | jq 'length')
if [ "$total" -gt 0 ]; then
  bad=$(printf '%s' "$checks" | jq -r '[.[] | select(.bucket != "pass" and .bucket != "skipping")] | map(.name + "=" + .bucket) | join(", ")')
  [ -z "$bad" ] || die "GitHub の checks が成功していません: $bad"
  printf 'pr-merge: GitHub checks %s 件すべて成功\n' "$total"
else
  printf 'pr-merge: GitHub checks なし(CI が走っていない)。ローカル検証を CI の代わりにします\n'
fi

# 3. 変更ファイルの制限
files=$(gh pr diff "$PR" --name-only) || die "PR #$PR の変更ファイルを取得できません"
protected=$(printf '%s\n' "$files" | grep -E '^(\.claude/|\.codex/|scripts/ai-guard/|scripts/pr-merge(_test)?\.sh$|deploy/k8s/overlays/cloud/|terraform/|\.github/workflows/)' || true)
if [ -n "$protected" ]; then
  printf '%s\n' "$protected" | sed 's/^/  - /' >&2
  die "AI の権限・ガード・クラウド/費用に関わるファイルを変更する PR は、人間がマージします(上記)"
fi

# 4. ローカル検証(PR の先頭コミットを使い捨ての worktree で)
git fetch -q origin "pull/$PR/head" || die "PR #$PR の先頭コミットを取得できません"
fetched=$(git rev-parse FETCH_HEAD)
[ "$fetched" = "$sha" ] || die "取得したコミット($fetched)が PR の先頭($sha)と一致しません。再実行してください"
WT=$(mktemp -d)
cleanup() {
  git worktree remove --force "$WT" >/dev/null 2>&1 || rm -rf "$WT"
}
trap cleanup EXIT
git worktree add -q --detach "$WT" "$sha"
# 注意: `( ... ) || die` の中では set -e が効かないので、&& で明示的につなぐ(どれか1つの失敗で全体が失敗する)。
run_local_gate() {
  cd "$WT" || return 1
  if [ -f web/package-lock.json ]; then
    (cd web && npm ci --silent) || return 1
  fi
  make lint && make check-publishable && make test || return 1
  if printf '%s\n' "$files" | grep -qE '^(engine/|testdata/golden/)'; then
    make test-golden || return 1
  fi
}
(run_local_gate) || die "ローカル検証に失敗しました(PR #$PR @ ${sha:0:7})"
printf 'pr-merge: ローカル検証 OK(PR #%s @ %s)\n' "$PR" "${sha:0:7}"

if [ "$CHECK_ONLY" -eq 1 ]; then
  printf 'pr-merge: --check のためマージしません\n'
  exit 0
fi

# 5. マージ(PR の先頭が検証した SHA と一致するときだけ)
gh pr merge "$PR" --merge --match-head-commit "$sha"
printf 'pr-merge: PR #%s をマージしました(base=%s)\n' "$PR" "$base"
