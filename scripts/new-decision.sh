#!/usr/bin/env bash
# scripts/new-decision.sh <lane> <slug> [タイトル] — 判断・提案・依頼のエントリを1件1ファイルで作る(ADR-0170)。
#
# docs/decisions/<YYYY-MM-DD>-<lane>-<slug>.md を作り、そのパスを標準出力に出す。既にあれば作らずに失敗する。
# 日付は日本時間の今日(運用の時刻の基準。COORDINATION.md)。DECISIONS_DATE=YYYY-MM-DD で上書きできる(テスト用)。
# <lane> はブランチの接頭辞と同じ: data api web ios tb speed judge ops。全レーンにかかるものは shared。
# <slug> は英小文字・数字・ハイフン(例 move-target-in-master)。
#
# 使い方: scripts/new-decision.sh judge move-target-in-master "技の対象をマスタに持たせてほしい"
set -euo pipefail

readonly LANES="data api web ios tb speed judge ops shared"

usage() {
  echo "使い方: $0 <lane> <slug> [タイトル]" >&2
  echo "  lane: ${LANES}" >&2
  echo "  slug: 英小文字・数字・ハイフン(先頭は英小文字か数字)" >&2
  exit 2
}

[ "$#" -ge 2 ] && [ "$#" -le 3 ] || usage
lane=$1
slug=$2
title=${3:-"<タイトル>"}

case " ${LANES} " in
  *" ${lane} "*) ;;
  *) echo "不明なレーン: ${lane}" >&2; usage ;;
esac
if ! printf '%s' "$slug" | grep -Eq '^[a-z0-9][a-z0-9-]*$'; then
  echo "slug の形式が不正: ${slug}" >&2
  usage
fi

date_value=${DECISIONS_DATE:-$(TZ=Asia/Tokyo date +%Y-%m-%d)}
if ! printf '%s' "$date_value" | grep -Eq '^[0-9]{4}-[0-9]{2}-[0-9]{2}$'; then
  echo "日付の形式が不正(YYYY-MM-DD): ${date_value}" >&2
  exit 2
fi

root=$(cd "$(dirname "$0")/.." && pwd)
dir="${root}/docs/decisions"
rel="docs/decisions/${date_value}-${lane}-${slug}.md"
path="${root}/${rel}"

if [ -e "$path" ]; then
  echo "既にある: ${rel}(既存のエントリは編集しない。別の slug にする)" >&2
  exit 1
fi

mkdir -p "$dir"
cat > "$path" <<EOF
# ${title}

- 日付: ${date_value}
- レーン: ${lane}(宛先のレーンがあれば「${lane} → <レーン>」)
- 状態: open
- 状況: <何が起きたか・何を決める必要があるか>
- 既定案: <既定案。無ければ「なし」>

Decision: <決めたこと>
Reason: <理由>
Impact: <影響するレーン・ファイル>
EOF

echo "$rel"
