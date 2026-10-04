import type { DayLow } from "../api/types";

// 価格の推移のグラフ(SVG)の座標計算と文言。純粋関数(フェーズ4-2。docs/phase4-spec.md AC-HIS-*)。
// 日付は API の `YYYY-MM-DD`(JST の日付)をそのまま扱い、実行環境のタイムゾーンに依存させない。

/** 1 点。day は `YYYY-MM-DD` */
export interface ChartPoint {
  day: string;
  value: number;
}

/** 1 本の折れ線。key は `overall` か `site-<id>` */
export interface ChartSeries {
  key: string;
  points: ChartPoint[];
}

/** 描画領域(px)。内側の幅 = width − left − right、高さ = height − top − bottom */
export interface ChartBox {
  width: number;
  height: number;
  left: number;
  right: number;
  top: number;
  bottom: number;
}

export interface ChartModel {
  /** 線のある系列ごと(入力の順。点が 0 の系列は除く)。d は `M x y L x y …`、1 日より空いたところは `M` で切る */
  paths: { key: string; d: string }[];
  /** 実際の点ごと(系列の順・day 昇順) */
  dots: { key: string; day: string; value: number; x: number; y: number }[];
  /** 最初の日と最後の日(同じ日なら 1 つ)。text は `M/D` */
  xLabels: { x: number; text: string }[];
  /** 最大と最小(同じなら 1 つ)。text は `¥3,000` */
  yLabels: { y: number; text: string }[];
}

/** Sheet が使う既定の描画領域 */
export const CHART_BOX: ChartBox = { width: 320, height: 160, left: 56, right: 12, top: 12, bottom: 24 };

/** `YYYY-MM-DD` を、タイムゾーンに依存しない通し日数にする */
function dayNumber(day: string): number {
  const [y, m, d] = day.split("-").map(Number);
  return Date.UTC(y ?? 0, (m ?? 1) - 1, d ?? 1) / 86_400_000;
}

const round1 = (n: number): number => Math.round(n * 10) / 10;
const yen = (n: number): string => `¥${n.toLocaleString("ja-JP")}`;

/** `2026-10-03` → `10/3`(ゼロ詰めしない) */
export function dayLabel(day: string): string {
  const [, m, d] = day.split("-").map(Number);
  return `${String(m)}/${String(d)}`;
}

/**
 * 全系列で共通の軸(x は最初の日〜最後の日、y は最小〜最大)に点を置く。点が 1 つも無ければ null。
 * 期間が 1 日なら x は内側の中央、値が 1 種類なら y は内側の中央。座標は 0.1 px に丸め、文字にするときは末尾の `.0` を付けない。
 */
export function buildChart(series: ChartSeries[], box: ChartBox): ChartModel | null {
  const sorted = series
    .map((s) => ({ key: s.key, points: [...s.points].sort((a, b) => dayNumber(a.day) - dayNumber(b.day)) }))
    .filter((s) => s.points.length > 0);
  const all = sorted.flatMap((s) => s.points);
  if (all.length === 0) return null;

  const days = all.map((p) => dayNumber(p.day));
  const values = all.map((p) => p.value);
  const d0 = Math.min(...days);
  const d1 = Math.max(...days);
  const vMin = Math.min(...values);
  const vMax = Math.max(...values);
  const innerW = box.width - box.left - box.right;
  const innerH = box.height - box.top - box.bottom;
  const x = (dn: number): number =>
    round1(d1 === d0 ? box.left + innerW / 2 : box.left + ((dn - d0) / (d1 - d0)) * innerW);
  const y = (v: number): number =>
    round1(vMax === vMin ? box.top + innerH / 2 : box.top + ((vMax - v) / (vMax - vMin)) * innerH);

  const paths: ChartModel["paths"] = [];
  const dots: ChartModel["dots"] = [];
  for (const s of sorted) {
    let d = "";
    let prev: number | null = null;
    for (const p of s.points) {
      const dn = dayNumber(p.day);
      const px = x(dn);
      const py = y(p.value);
      d += `${d ? " " : ""}${prev !== null && dn - prev === 1 ? "L" : "M"}${String(px)} ${String(py)}`;
      prev = dn;
      dots.push({ key: s.key, day: p.day, value: p.value, x: px, y: py });
    }
    paths.push({ key: s.key, d });
  }

  const dayAt = (dn: number): string => all.find((p) => dayNumber(p.day) === dn)?.day ?? "";
  return {
    paths,
    dots,
    xLabels:
      d0 === d1
        ? [{ x: x(d0), text: dayLabel(dayAt(d0)) }]
        : [
            { x: x(d0), text: dayLabel(dayAt(d0)) },
            { x: x(d1), text: dayLabel(dayAt(d1)) },
          ],
    yLabels:
      vMax === vMin
        ? [{ y: y(vMax), text: yen(vMax) }]
        : [
            { y: y(vMax), text: yen(vMax) },
            { y: y(vMin), text: yen(vMin) },
          ],
  };
}

/** グラフの aria-label。`価格の推移 7/6〜10/3 最安 ¥2,000 最高 ¥3,200`(1 日だけなら期間は `10/3`、空なら `価格の推移はまだありません`) */
export function historyLabel(overall: DayLow[]): string {
  if (overall.length === 0) return "価格の推移はまだありません";
  const days = overall.map((o) => o.day).sort();
  const first = dayLabel(days[0] ?? "");
  const last = dayLabel(days[days.length - 1] ?? "");
  const lows = overall.map((o) => o.low);
  const range = first === last ? first : `${first}〜${last}`;
  return `価格の推移 ${range} 最安 ${yen(Math.min(...lows))} 最高 ${yen(Math.max(...lows))}`;
}
