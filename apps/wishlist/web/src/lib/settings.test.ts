import { describe, expect, it, vi } from "vitest";
import { SETTINGS_KEY, loadSettings, saveSettings } from "./settings";

// AC-LIB-06
describe("settings", () => {
  it("何も保存が無いときの既定", () => {
    expect(loadSettings()).toEqual({ apiBaseUrl: null, token: "" });
  });
  it("保存して読み戻せる", () => {
    expect(saveSettings({ apiBaseUrl: "https://h.example/wishlist/", token: "tok" })).toBe(true);
    expect(loadSettings()).toEqual({ apiBaseUrl: "https://h.example/wishlist/", token: "tok" });
    expect(JSON.parse(localStorage.getItem(SETTINGS_KEY) ?? "null")).toEqual({
      apiBaseUrl: "https://h.example/wishlist/",
      token: "tok",
    });
  });
  it("壊れた JSON・型違いは既定に戻す", () => {
    localStorage.setItem(SETTINGS_KEY, "{oops");
    expect(loadSettings()).toEqual({ apiBaseUrl: null, token: "" });
    localStorage.setItem(SETTINGS_KEY, JSON.stringify({ apiBaseUrl: 5, token: {} }));
    expect(loadSettings()).toEqual({ apiBaseUrl: null, token: "" });
  });
  it("localStorage が例外を投げても落ちない", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("denied");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("denied");
    });
    expect(loadSettings()).toEqual({ apiBaseUrl: null, token: "" });
    expect(saveSettings({ apiBaseUrl: null, token: "x" })).toBe(false);
  });
});
