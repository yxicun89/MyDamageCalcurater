import { describe, expect, it } from "vitest";
import { itemQuery, resolveSiteLinks } from "./links";
import { makeGenre, makeItem, makeSite } from "../test/factories";

const mercari = makeSite({
  id: 1,
  name: "メルカリ",
  search_url_template: "https://jp.mercari.com/search?keyword={q}",
});
const amazon = makeSite({ id: 2, name: "Amazon", search_url_template: "https://www.amazon.co.jp/s?k={q}" });
const other = makeSite({ id: 3, name: "その他" });
const sites = [mercari, amazon, other];
const genre = makeGenre({ id: 1, query_template: "S.H.Figuarts {name}", site_ids: [2, 1] });

describe("itemQuery", () => {
  it("テンプレートから作る", () => {
    expect(itemQuery(makeItem({ id: 1, name: "グリス" }), genre)).toBe("S.H.Figuarts グリス");
  });
  it("query_override が優先", () => {
    expect(itemQuery(makeItem({ id: 1, name: "グリス", query_override: "グリス 上書き" }), genre)).toBe(
      "グリス 上書き",
    );
  });
  it("ジャンルが無い(未取得)ときは name と option を空白でつなぐ", () => {
    expect(itemQuery(makeItem({ id: 1, name: "A", option_text: "B" }), undefined)).toBe("A B");
  });
});

// AC-SHEET-04/05: サイト行の並び・除外・検索ワードの優先順位。
describe("resolveSiteLinks", () => {
  it("ジャンルの site_ids の順に並べ、検索 URL を作る", () => {
    const links = resolveSiteLinks(makeItem({ id: 1, name: "グリス" }), genre, sites);
    expect(links.map((l) => l.site.name)).toEqual(["Amazon", "メルカリ"]);
    expect(links[1]?.url).toBe(
      "https://jp.mercari.com/search?keyword=S.H.Figuarts%20%E3%82%B0%E3%83%AA%E3%82%B9",
    );
  });
  it("http(s) 以外の検索 URL になるサイトは出さない", () => {
    const bad = makeSite({ id: 9, name: "悪い", search_url_template: "javascript:alert('{q}')" });
    const g = { ...genre, site_ids: [9, ...genre.site_ids] };
    const links = resolveSiteLinks(makeItem({ id: 1, name: "グリス" }), g, [...sites, bad]);
    expect(links.map((l) => l.site.id)).not.toContain(9);
    expect(links.length).toBe(genre.site_ids.length);
  });
  it("enabled=false のサイトは出さない", () => {
    const item = makeItem({ id: 1, name: "グリス", site_overrides: [{ site_id: 2, enabled: false }] });
    expect(resolveSiteLinks(item, genre, sites).map((l) => l.site.id)).toEqual([1]);
  });
  it("サイト別 query が最優先(query_override より上)", () => {
    const item = makeItem({
      id: 1,
      name: "グリス",
      query_override: "上書き",
      site_overrides: [{ site_id: 1, query: "メルカリ用", enabled: true }],
    });
    const links = resolveSiteLinks(item, genre, sites);
    expect(links.find((l) => l.site.id === 1)?.query).toBe("メルカリ用");
    expect(links.find((l) => l.site.id === 2)?.query).toBe("上書き");
  });
  it("sites に無い ID は読み飛ばす", () => {
    const g = makeGenre({ id: 1, site_ids: [99, 1] });
    expect(resolveSiteLinks(makeItem({ id: 1 }), g, sites).map((l) => l.site.id)).toEqual([1]);
  });
  it("ジャンルが無いときは空", () => {
    expect(resolveSiteLinks(makeItem({ id: 1 }), undefined, sites)).toEqual([]);
  });
});
