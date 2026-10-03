import { describe, expect, it } from "vitest";
import {
  buildChart,
  dayLabel,
  historyLabel,
  type ChartBox,
  type ChartModel,
  type ChartSeries,
} from "./history";

// フェーズ4-2 価格の推移の純関数(docs/phase4-spec.md AC-HIS-07〜09)。

// 内側は幅 270・高さ 130(x は 40〜310、y は 10〜140)。
const BOX: ChartBox = { width: 320, height: 160, left: 40, right: 10, top: 10, bottom: 20 };

const s = (key: string, ...pts: [string, number][]): ChartSeries => ({
  key,
  points: pts.map(([day, value]) => ({ day, value })),
});

// AC-HIS-07: 日付の表記
describe("dayLabel", () => {
  it.each([
    ["2026-10-03", "10/3"],
    ["2026-12-31", "12/31"],
    ["2027-01-01", "1/1"],
    ["2026-07-06", "7/6"],
  ])("AC-HIS-07 %s → %s", (day, want) => {
    expect(dayLabel(day)).toBe(want);
  });
});

// AC-HIS-08: 座標計算(全系列で共通の軸。1 日より空いたところで線を切る)
describe("buildChart", () => {
  const cases: {
    name: string;
    series: ChartSeries[];
    paths: ChartModel["paths"];
    xLabels: ChartModel["xLabels"];
    yLabels: ChartModel["yLabels"];
    dots: number;
  }[] = [
    {
      name: "3 日続く 1 本",
      series: [s("overall", ["2026-10-01", 3000], ["2026-10-02", 4000], ["2026-10-03", 3500])],
      paths: [{ key: "overall", d: "M40 140 L175 10 L310 75" }],
      xLabels: [
        { x: 40, text: "10/1" },
        { x: 310, text: "10/3" },
      ],
      yLabels: [
        { y: 10, text: "¥4,000" },
        { y: 140, text: "¥3,000" },
      ],
      dots: 3,
    },
    {
      name: "出品の無い日(10/3)は点を打たず、線を切る",
      series: [s("overall", ["2026-10-01", 3000], ["2026-10-02", 3000], ["2026-10-04", 3600])],
      paths: [{ key: "overall", d: "M40 140 L130 140 M310 10" }],
      xLabels: [
        { x: 40, text: "10/1" },
        { x: 310, text: "10/4" },
      ],
      yLabels: [
        { y: 10, text: "¥3,600" },
        { y: 140, text: "¥3,000" },
      ],
      dots: 3,
    },
    {
      name: "1 点だけ(期間 1 日・値 1 種類)は中央に置き、線は引かない",
      series: [s("overall", ["2026-10-03", 3000])],
      paths: [{ key: "overall", d: "M175 75" }],
      xLabels: [{ x: 175, text: "10/3" }],
      yLabels: [{ y: 75, text: "¥3,000" }],
      dots: 1,
    },
    {
      name: "2 本は同じ軸に載る(点が 0 の系列は除く)",
      series: [
        s("overall", ["2026-10-01", 3000], ["2026-10-03", 3000]),
        s("site-1", ["2026-10-02", 4300]),
        s("site-2"),
      ],
      paths: [
        { key: "overall", d: "M40 140 M310 140" },
        { key: "site-1", d: "M175 10" },
      ],
      xLabels: [
        { x: 40, text: "10/1" },
        { x: 310, text: "10/3" },
      ],
      yLabels: [
        { y: 10, text: "¥4,300" },
        { y: 140, text: "¥3,000" },
      ],
      dots: 3,
    },
    {
      name: "座標は 0.1 px に丸める(期間 7 日)",
      series: [s("overall", ["2026-10-01", 1000], ["2026-10-02", 1000], ["2026-10-08", 2000])],
      paths: [{ key: "overall", d: "M40 140 L78.6 140 M310 10" }],
      xLabels: [
        { x: 40, text: "10/1" },
        { x: 310, text: "10/8" },
      ],
      yLabels: [
        { y: 10, text: "¥2,000" },
        { y: 140, text: "¥1,000" },
      ],
      dots: 3,
    },
    {
      name: "年またぎは続いた日として線をつなぐ。値が 1 種類なら y は中央",
      series: [s("overall", ["2026-12-31", 5000], ["2027-01-01", 5000])],
      paths: [{ key: "overall", d: "M40 75 L310 75" }],
      xLabels: [
        { x: 40, text: "12/31" },
        { x: 310, text: "1/1" },
      ],
      yLabels: [{ y: 75, text: "¥5,000" }],
      dots: 2,
    },
  ];
  it.each(cases)("AC-HIS-08 $name", ({ series, paths, xLabels, yLabels, dots }) => {
    const m = buildChart(series, BOX);
    expect(m).not.toBeNull();
    expect(m?.paths).toEqual(paths);
    expect(m?.xLabels).toEqual(xLabels);
    expect(m?.yLabels).toEqual(yLabels);
    expect(m?.dots).toHaveLength(dots);
  });

  it("AC-HIS-08 点は実際の値だけ(系列の順・日付順。作った値を混ぜない)", () => {
    const m = buildChart(
      [s("overall", ["2026-10-04", 3600], ["2026-10-01", 3000]), s("site-1", ["2026-10-02", 3300])],
      BOX,
    );
    expect(m?.dots).toEqual([
      { key: "overall", day: "2026-10-01", value: 3000, x: 40, y: 140 },
      { key: "overall", day: "2026-10-04", value: 3600, x: 310, y: 10 },
      { key: "site-1", day: "2026-10-02", value: 3300, x: 130, y: 75 },
    ]);
  });

  it("AC-HIS-08 点が 1 つも無ければ null", () => {
    expect(buildChart([], BOX)).toBeNull();
    expect(buildChart([s("overall"), s("site-1")], BOX)).toBeNull();
  });
});

// AC-HIS-09: グラフの要約(aria-label)
describe("historyLabel", () => {
  it.each([
    {
      name: "期間・最安・最高",
      overall: [
        { day: "2026-07-06", low: 2000 },
        { day: "2026-10-01", low: 2800 },
        { day: "2026-10-03", low: 3200 },
      ],
      want: "価格の推移 7/6〜10/3 最安 ¥2,000 最高 ¥3,200",
    },
    {
      name: "最安・最高は日付の順と関係ない",
      overall: [
        { day: "2026-10-01", low: 3500 },
        { day: "2026-10-02", low: 3000 },
      ],
      want: "価格の推移 10/1〜10/2 最安 ¥3,000 最高 ¥3,500",
    },
    {
      name: "1 日だけ",
      overall: [{ day: "2026-10-03", low: 3000 }],
      want: "価格の推移 10/3 最安 ¥3,000 最高 ¥3,000",
    },
    { name: "空", overall: [], want: "価格の推移はまだありません" },
  ])("AC-HIS-09 $name", ({ overall, want }) => {
    expect(historyLabel(overall)).toBe(want);
  });
});
