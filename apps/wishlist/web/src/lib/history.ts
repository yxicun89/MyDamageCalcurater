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

/** `2026-10-03` → `10/3`(ゼロ詰めしない) */
export function dayLabel(day: string): string {
  return notImplemented(day);
}

/**
 * 全系列で共通の軸(x は最初の日〜最後の日、y は最小〜最大)に点を置く。点が 1 つも無ければ null。
 * 期間が 1 日なら x は内側の中央、値が 1 種類なら y は内側の中央。座標は 0.1 px に丸め、文字にするときは末尾の `.0` を付けない。
 */
export function buildChart(series: ChartSeries[], box: ChartBox): ChartModel | null {
  return notImplemented(series, box);
}

/** グラフの aria-label。`価格の推移 7/6〜10/3 最安 ¥2,000 最高 ¥3,200`(1 日だけなら期間は `10/3`、空なら `価格の推移はまだありません`) */
export function historyLabel(overall: DayLow[]): string {
  return notImplemented(overall);
}

/** TODO(implementer): スタブ。実装したらこの関数ごと消す */
function notImplemented(...args: unknown[]): never {
  throw new Error(`history.ts is not implemented (${String(args.length)} args)`);
}
