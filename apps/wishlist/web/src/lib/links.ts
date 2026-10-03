import type { Genre, Item, Site } from "../api/types";
import { buildDeeplink, isHttpUrl } from "./deeplink";
import { buildQuery } from "./query";

export interface SiteLink {
  site: Site;
  /** そのサイトで使う検索ワード(site_overrides.query > query_override > テンプレート) */
  query: string;
  url: string;
}

const DEFAULT_TEMPLATE = "{name} {option}";

const queryFor = (item: Item, genre: Genre | undefined, siteQuery: string | null): string =>
  buildQuery({
    template: genre?.query_template ?? DEFAULT_TEMPLATE,
    name: item.name,
    option: item.option_text ?? null,
    queryOverride: item.query_override ?? null,
    siteQuery,
  });

/** 詳細シートのサイト行。ジャンルの site_ids 順。enabled=false と未知のサイト ID は除く。 */
export const resolveSiteLinks = (item: Item, genre: Genre | undefined, sites: Site[]): SiteLink[] => {
  if (!genre) return [];
  const links: SiteLink[] = [];
  for (const id of genre.site_ids) {
    const site = sites.find((s) => s.id === id);
    if (!site) continue;
    const override = item.site_overrides.find((o) => o.site_id === id);
    if (override && !override.enabled) continue;
    const query = queryFor(item, genre, override?.query ?? null);
    const url = buildDeeplink(site.search_url_template, query);
    // テンプレートは登録時に検査するが、API から来た値なので描画前にも http(s) 以外を落とす
    if (!isHttpUrl(url)) continue;
    links.push({ site, query, url });
  }
  return links;
};

/** 商品の検索ワード(サイト別の上書きを除く。シートの検索ワード欄に出す値)。 */
export const itemQuery = (item: Item, genre: Genre | undefined): string => queryFor(item, genre, null);
