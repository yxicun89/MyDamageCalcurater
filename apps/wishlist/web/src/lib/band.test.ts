import { describe, expect, it } from "vitest";
import type { SiteEstimate } from "../api/types";
import { makeSite } from "../test/factories";
import { bandDots, bandLeft, bestSiteIds, hasBandRange } from "./band";

const est = (site_id: number, low: number | null): SiteEstimate => ({
  site_id,
  low,
  mid: null,
  count: 1,
  suspicious_count: 0,
  in_stock_count: 1,
  status: "ok",
  fetched_at: "2026-10-03T00:00:00Z",
});
const sites = [1, 2, 3].map((id) => makeSite({ id }));

describe("bandDots", () => {
  it("最安を 0、最高を 100 とし、間は比例して四捨五入する", () => {
    const dots = bandDots(sites, [est(1, 6800), est(2, 7200), est(3, 7900)]);
    expect(dots.map((d) => d.position)).toEqual([0, 36, 100]);
  });
  it("点が 1 つ(または全部同額)なら 0。low の無いサイトは点にしない", () => {
    expect(bandDots(sites, [est(1, 3000), est(2, null)]).map((d) => d.position)).toEqual([0]);
    expect(bandDots(sites, [est(1, 3000), est(2, 3000)]).map((d) => d.position)).toEqual([0, 0]);
  });
});

describe("bestSiteIds", () => {
  it("low の最小のサイト(同額は全部)。対象外のサイトは数えない", () => {
    expect(bestSiteIds([est(1, 800), est(2, 800), est(3, 900), est(99, 1)], [1, 2, 3])).toEqual(
      new Set([1, 2]),
    );
  });
});

// phase5-spec 5-3「Web に反映する点」: 全部同額のとき、点は中央に置き、太線(安い〜中央)は描かない。data-position の契約(全部同額は 0)は変えない。
describe("bandLeft / hasBandRange", () => {
  it("通常は両端に 6% の余白を取って position に比例させ、太線を描く", () => {
    const dots = bandDots(sites, [est(1, 1000), est(2, 2000), est(3, 3000)]);
    expect(dots.map((d) => bandLeft(dots, d))).toEqual(["6%", "50%", "94%"]);
    expect(hasBandRange(dots)).toBe(true);
  });
  it("全部同額(点が 1 つを含む)は中央 50% に置き、太線は描かない(position は 0 のまま)", () => {
    const same = bandDots(sites, [est(1, 3000), est(2, 3000)]);
    expect(same.map((d) => d.position)).toEqual([0, 0]);
    expect(same.map((d) => bandLeft(same, d))).toEqual(["50%", "50%"]);
    expect(hasBandRange(same)).toBe(false);
    const single = bandDots(sites, [est(2, 4800)]);
    expect(bandLeft(single, single[0] as (typeof single)[number])).toBe("50%");
    expect(hasBandRange(single)).toBe(false);
  });
  it("点が無ければ太線も無い", () => {
    expect(hasBandRange([])).toBe(false);
  });
});
