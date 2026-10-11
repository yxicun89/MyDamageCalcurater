import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { App } from "./App";
import { installFakeApi } from "./test/fakeApi";
import { scenario } from "./test/scenario";

// AC-5-2a-*: トークンと接続先の設定を無くす(ユーザー決定 2026-10-11 / phase5-spec 5-1a・5-2「設定」。8080 は 127.0.0.1 だけで待ち、外からは Tailscale を通す)
describe("認証なし・設定なしの起動", () => {
  it("AC-5-2a-1 初回起動(localStorage が空)でも設定画面を挟まず、ホームが出て、API を Authorization 無しで呼ぶ", async () => {
    const api = installFakeApi(scenario());
    render(<App />);
    expect(await screen.findByRole("button", { name: "グリス" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "設定" })).not.toBeInTheDocument();
    for (const path of ["/api/items", "/api/genres", "/api/sites"])
      expect(api.callsTo("GET", path).length).toBeGreaterThan(0);
    for (const c of api.calls) expect(c.headers.has("Authorization")).toBe(false);
  });

  it("AC-5-2a-2 以前の版が保存した wishlist.settings(トークン・API の URL)は読まない。送信先はこのページと同じ場所", async () => {
    localStorage.setItem(
      "wishlist.settings",
      JSON.stringify({ apiBaseUrl: "https://old.example/wishlist/", token: "old-token" }),
    );
    const fetchSpy = installFakeApi(scenario());
    render(<App />);
    await screen.findByRole("button", { name: "グリス" });
    expect(fetchSpy.calls.length).toBeGreaterThan(0);
    for (const c of fetchSpy.calls) expect(c.headers.has("Authorization")).toBe(false);
    const urls = (globalThis.fetch as unknown as { mock: { calls: [unknown][] } }).mock.calls.map((a) =>
      String(a[0]),
    );
    for (const u of urls) expect(u.startsWith(new URL("./", document.baseURI).href)).toBe(true);
  });
});
