import { describe, expect, it, vi } from "vitest";
import { ApiError } from "./api";
import { cacheKey, fetchWithCache, loadCache, saveCache } from "./cache";
import { makeGenre, makeItem } from "../test/factories";

// AC-LIB-07
describe("cache", () => {
  it("保存して読み戻せる(items・genres・sites を別々に)", () => {
    const items = [makeItem({ id: 1 })];
    expect(saveCache("items", items)).toBe(true);
    expect(loadCache("items")).toEqual(items);
    expect(loadCache("genres")).toBeNull();
  });
  it("壊れた値は null", () => {
    localStorage.setItem(cacheKey("genres"), "{oops");
    expect(loadCache("genres")).toBeNull();
  });
  it("localStorage が例外を投げても落ちない", () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("quota");
    });
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("denied");
    });
    expect(saveCache("items", [])).toBe(false);
    expect(loadCache("items")).toBeNull();
  });

  describe("fetchWithCache", () => {
    it("成功したら保存して stale=false", async () => {
      const genres = [makeGenre({ id: 1 })];
      const r = await fetchWithCache("genres", () => Promise.resolve(genres));
      expect(r).toEqual({ data: genres, stale: false });
      expect(loadCache("genres")).toEqual(genres);
    });
    it("失敗したら保存済みを stale=true で返す(通信失敗・API エラーとも)", async () => {
      const genres = [makeGenre({ id: 1 })];
      saveCache("genres", genres);
      const net = await fetchWithCache("genres", () => Promise.reject(new ApiError("network", "x", 0)));
      expect(net).toEqual({ data: genres, stale: true });
      const srv = await fetchWithCache("genres", () => Promise.reject(new ApiError("internal", "x", 500)));
      expect(srv).toEqual({ data: genres, stale: true });
    });
    it("保存済みも無いときは元の例外を投げる", async () => {
      const err = new ApiError("unauthorized", "no", 401);
      await expect(fetchWithCache("sites", () => Promise.reject(err))).rejects.toBe(err);
    });
    it("失敗しても保存済みを上書きしない", async () => {
      const genres = [makeGenre({ id: 1 })];
      saveCache("genres", genres);
      await fetchWithCache("genres", () => Promise.reject(new Error("x")));
      expect(loadCache("genres")).toEqual(genres);
    });
  });
});
