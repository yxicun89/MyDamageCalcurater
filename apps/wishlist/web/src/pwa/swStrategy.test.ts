import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import { describe, expect, it } from "vitest";
import { webPath } from "../test/paths";

// public/sw-strategy.js は classic script(importScripts で sw.js に読み込む)。vm で読んで self.wishlistSw を取り出す。
type Strategy = "ignore" | "network-only" | "cache-first" | "network-first-shell" | "stale-while-revalidate";
interface Sw {
  classify(req: { url: string; method: string; mode: string }, scopeUrl: string): Strategy;
  shouldCache(res: { status: number; type: string }): boolean;
  cacheNames: { shell: string; images: string };
}
function load(): Sw {
  const self: { wishlistSw?: Sw } = {};
  runInNewContext(readFileSync(webPath("public", "sw-strategy.js"), "utf8"), { self });
  if (!self.wishlistSw) throw new Error("self.wishlistSw が未定義");
  return self.wishlistSw;
}

const SCOPE = "https://h.example/wishlist/";
const req = (url: string, over: Partial<{ method: string; mode: string }> = {}) => ({
  url,
  method: "GET",
  mode: "no-cors",
  ...over,
});

// AC-PWA-03: どのリクエストをどの戦略にするか。
describe("sw-strategy classify", () => {
  const sw = load;
  it.each([
    ["https://h.example/wishlist/api/items", {}, "network-only"],
    ["https://h.example/wishlist/api/items/3/estimates", {}, "network-only"],
    ["https://h.example/wishlist/healthz", {}, "network-only"],
    ["https://h.example/wishlist/sw.js", {}, "network-only"],
    ["https://h.example/wishlist/images/00000000-0000-4000-8000-000000000001.png", {}, "cache-first"],
    ["https://h.example/wishlist/", { mode: "navigate" }, "network-first-shell"],
    ["https://h.example/wishlist/index.html", { mode: "navigate" }, "network-first-shell"],
    ["https://h.example/wishlist/assets/index-abc.js", {}, "stale-while-revalidate"],
    ["https://h.example/wishlist/manifest.webmanifest", {}, "stale-while-revalidate"],
    ["https://h.example/wishlist/api/items", { method: "POST" }, "ignore"],
    ["https://h.example/wishlist/images/x.png", { method: "PUT" }, "ignore"],
    ["https://other.example/wishlist/images/x.png", {}, "ignore"],
    ["https://h.example/other/app.js", {}, "ignore"],
  ] as [string, Partial<{ method: string; mode: string }>, Strategy][])("%s %j -> %s", (url, over, want) => {
    expect(sw().classify(req(url, over), SCOPE)).toBe(want);
  });
  it("API はどの経路でもキャッシュ対象にならない(network-only 以外を返さない)", () => {
    for (const p of ["api/items", "api/genres", "api/sites", "api/items/1/listings?site_id=2"]) {
      expect(sw().classify(req(SCOPE + p), SCOPE)).toBe("network-only");
    }
  });
  it("shouldCache は 200 の同一オリジン応答だけ", () => {
    expect(sw().shouldCache({ status: 200, type: "basic" })).toBe(true);
    expect(sw().shouldCache({ status: 404, type: "basic" })).toBe(false);
    expect(sw().shouldCache({ status: 500, type: "basic" })).toBe(false);
    expect(sw().shouldCache({ status: 0, type: "opaque" })).toBe(false);
  });
  it("キャッシュ名は shell と images で別", () => {
    const { shell, images } = sw().cacheNames;
    expect(shell).toBeTruthy();
    expect(images).toBeTruthy();
    expect(shell).not.toBe(images);
  });
});

describe("sw.js", () => {
  const src = () => readFileSync(webPath("public", "sw.js"), "utf8");
  it("sw-strategy.js を読み込み、install・activate・fetch を扱う", () => {
    expect(src()).toContain("sw-strategy.js");
    for (const ev of ["install", "activate", "fetch"]) expect(src()).toContain(`"${ev}"`);
  });
  it("API(/api/)をキャッシュに入れる処理を直接持たない(戦略は classify に任せる)", () => {
    expect(src()).not.toMatch(/["'`]\/?api\//);
  });
});
