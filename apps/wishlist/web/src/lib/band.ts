import type { Site, SiteEstimate } from "../api/types";

export interface BandDot {
  site: Site;
  low: number;
  /** 整数 0〜100。点にするサイトの最安を 0、最高を 100 とした位置 */
  position: number;
}

/**
 * 価格の帯に置く点。low を持つサイト(取得失敗の前回値も含む)だけを、渡された順で返す。
 * 位置は round((low - min) / (max - min) * 100)。全部同額なら 0。
 */
export function bandDots(sites: readonly Site[], estimates: readonly SiteEstimate[]): BandDot[] {
  const priced = sites.flatMap((site) => {
    const low = estimates.find((e) => e.site_id === site.id)?.low;
    return low == null ? [] : [{ site, low }];
  });
  const lows = priced.map((p) => p.low);
  const min = Math.min(...lows);
  const max = Math.max(...lows);
  return priced.map((p) => ({
    ...p,
    position: max === min ? 0 : Math.round(((p.low - min) / (max - min)) * 100),
  }));
}

/** 最安(low の最小。同額は全部)のサイトの id */
export function bestSiteIds(estimates: readonly SiteEstimate[], siteIds: readonly number[]): Set<number> {
  const rows = estimates.filter((e) => siteIds.includes(e.site_id) && e.low != null);
  const min = Math.min(...rows.map((e) => e.low as number));
  return new Set(rows.filter((e) => e.low === min).map((e) => e.site_id));
}
