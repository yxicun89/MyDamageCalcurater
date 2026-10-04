import type { SuspiciousReason } from "../api/types";

// 目安価格の表示用の純粋関数(フェーズ3。docs/phase3-web-spec.md)。

const JST_OFFSET_MS = 9 * 60 * 60 * 1000;
const DAY_MS = 24 * 60 * 60 * 1000;

/** `3000` → `¥3,000`(ja-JP の桁区切り) */
export function formatYen(n: number): string {
  return `¥${n.toLocaleString("ja-JP")}`;
}

/** low 〜 mid。mid が null / undefined なら `¥low〜`。 */
export function formatRange(low: number, mid: number | null | undefined): string {
  return `${formatYen(low)}〜${mid == null ? "" : formatYen(mid)}`;
}

/** 時刻を JST の壁時計にずらした Date(UTC のゲッターで読む) */
const toJst = (d: Date): Date => new Date(d.getTime() + JST_OFFSET_MS);

/** ISO 日時 → JST の `M/D`(ゼロ詰めしない)。例 `2026-10-02T15:30:00Z` → `10/3` */
export function formatJstDate(iso: string): string {
  const d = toJst(new Date(iso));
  return `${String(d.getUTCMonth() + 1)}/${String(d.getUTCDate())}`;
}

const jstDayIndex = (d: Date): number => Math.floor((d.getTime() + JST_OFFSET_MS) / DAY_MS);

/** JST の暦日の差で「今日」「N日前」。未来(時計のずれ)は「今日」。 */
export function formatAge(iso: string, now: Date): string {
  const days = jstDayIndex(now) - jstDayIndex(new Date(iso));
  return days <= 0 ? "今日" : `${String(days)}日前`;
}

const REASON_LABELS: Record<SuspiciousReason, string> = {
  title_mismatch: "商品名が一致しない",
  too_cheap: "安すぎる",
  below_min: "下限価格未満",
};

export function reasonLabel(reason: SuspiciousReason): string {
  return REASON_LABELS[reason];
}

/** http(s) の絶対 URL だけ true(javascript: などは false) */
export function isHttpUrl(url: string): boolean {
  try {
    const { protocol } = new URL(url);
    return protocol === "http:" || protocol === "https:";
  } catch {
    return false;
  }
}
