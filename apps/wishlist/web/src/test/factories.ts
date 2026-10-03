import type { Genre, Item, Site } from "../api/types";

export const NOW = "2026-10-03T00:00:00Z";
const UUID = (n: number) => `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;

export function makeItem(over: Partial<Item> & { id: number }): Item {
  return {
    genre_id: 1,
    name: `商品${String(over.id)}`,
    option_text: null,
    query_override: null,
    image_url: `images/${UUID(over.id)}.png`,
    source_url: null,
    min_price: null,
    sort_order: 0,
    site_overrides: [],
    created_at: NOW,
    updated_at: NOW,
    ...over,
  };
}

export function makeGenre(over: Partial<Genre> & { id: number }): Genre {
  return {
    name: `ジャンル${String(over.id)}`,
    query_template: "{name} {option}",
    sort_order: 0,
    site_ids: [],
    ...over,
  };
}

export function makeSite(over: Partial<Site> & { id: number }): Site {
  return {
    name: `サイト${String(over.id)}`,
    search_url_template: `https://site${String(over.id)}.example.com/s?q={q}`,
    fetch_type: "link_only",
    is_reference: false,
    ...over,
  };
}
