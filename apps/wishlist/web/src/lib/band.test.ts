import { describe, expect, it } from "vitest";
import type { SiteEstimate } from "../api/types";
import { makeSite } from "../test/factories";
import { bandDots, bestSiteIds } from "./band";

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
