import { describe, expect, it, vi } from "vitest";
import { ApiError, createApiClient, defaultBaseUrl, normalizeBaseUrl, resolveImageUrl } from "./api";
import { makeItem } from "../test/factories";
import { bodyText, urlOf } from "../test/net";

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}
function setup(res: Response | (() => Promise<Response>) = json({})) {
  const fetchMock = vi.fn<typeof fetch>(() => (typeof res === "function" ? res() : Promise.resolve(res)));
  const client = createApiClient({ baseUrl: "https://h.example/wishlist", token: "tok", fetch: fetchMock });
  const last = () => {
    const [input, init] = fetchMock.mock.calls.at(-1) ?? [];
    return { url: urlOf(input ?? ""), init: init ?? {}, headers: new Headers(init?.headers) };
  };
  return { client, fetchMock, last };
}

// AC-API-01〜06
describe("createApiClient", () => {
  it("AC-API-01 ベース URL(末尾 / なしでも)基準で組み立て、Bearer を付ける", async () => {
    const { client, last } = setup(json({ items: [makeItem({ id: 1 })] }));
    const items = await client.listItems();
    expect(items).toHaveLength(1);
    expect(last().url).toBe("https://h.example/wishlist/api/items");
    expect(last().headers.get("Authorization")).toBe("Bearer tok");
    expect(last().init.method ?? "GET").toBe("GET");
  });
  it("AC-API-01 genre_id をクエリに付ける", async () => {
    const { client, last } = setup(json({ items: [] }));
    await client.listItems(3);
    expect(last().url).toBe("https://h.example/wishlist/api/items?genre_id=3");
  });
  it("AC-API-01 ジャンル・サイトは配列だけを返す", async () => {
    expect(await setup(json({ genres: [] })).client.listGenres()).toEqual([]);
    expect(await setup(json({ sites: [] })).client.listSites()).toEqual([]);
  });
  it("AC-API-02 JSON ボディは Content-Type: application/json", async () => {
    const { client, last } = setup(json(makeItem({ id: 1 })));
    await client.updateItem(1, { query_override: null });
    expect(last().url).toBe("https://h.example/wishlist/api/items/1");
    expect(last().init.method).toBe("PATCH");
    expect(last().headers.get("Content-Type")).toBe("application/json");
    expect(JSON.parse(bodyText(last().init))).toEqual({ query_override: null });
  });
  it("AC-API-03 画像つき登録は multipart(Content-Type は手で付けない)。未指定の項目は含めない", async () => {
    const { client, last } = setup(json(makeItem({ id: 1 }), 201));
    const file = new File(["x"], "a.png", { type: "image/png" });
    await client.createItemWithImage({ genre_id: 2, name: "グリス", min_price: 0 }, file);
    expect(last().init.method).toBe("POST");
    expect(last().headers.has("Content-Type")).toBe(false);
    const body = last().init.body as FormData;
    expect(body).toBeInstanceOf(FormData);
    expect(body.get("genre_id")).toBe("2");
    expect(body.get("name")).toBe("グリス");
    expect(body.get("min_price")).toBe("0");
    expect(body.has("option_text")).toBe(false);
    expect((body.get("image") as File).name).toBe("a.png");
  });
  it("AC-API-03 画像差し替えは PUT /api/items/:id/image の multipart", async () => {
    const { client, last } = setup(json(makeItem({ id: 4 })));
    await client.replaceItemImage(4, new File(["x"], "b.png", { type: "image/png" }));
    expect(last().url).toBe("https://h.example/wishlist/api/items/4/image");
    expect(last().init.method).toBe("PUT");
    expect((last().init.body as FormData).get("image")).toBeInstanceOf(File);
  });
  it("AC-API-03 from-url と image_url 登録は JSON", async () => {
    const { client, last } = setup(
      json({ name: "n", source_url: "https://x", image_url: null, genre_id: null }),
    );
    await client.draftFromUrl("https://x", 2);
    expect(last().url).toBe("https://h.example/wishlist/api/items/from-url");
    expect(JSON.parse(bodyText(last().init))).toEqual({ url: "https://x", genre_id: 2 });
    await client.draftFromUrl("https://x");
    expect(JSON.parse(bodyText(last().init))).toEqual({ url: "https://x" });
    const s2 = setup(json(makeItem({ id: 1 }), 201));
    await s2.client.createItemFromImageUrl({ genre_id: 1, name: "n", image_url: "https://i/x.png" });
    expect(s2.last().url).toBe("https://h.example/wishlist/api/items");
    expect(s2.last().headers.get("Content-Type")).toBe("application/json");
  });
  it("AC-API-04 DELETE は 204 でも解決する", async () => {
    const { client, last } = setup(new Response(null, { status: 204 }));
    await expect(client.deleteItem(7)).resolves.toBeUndefined();
    expect(last().init.method).toBe("DELETE");
    expect(last().url).toBe("https://h.example/wishlist/api/items/7");
  });
  it("AC-API-05 エラー応答 {code,message} を ApiError にする", async () => {
    const { client } = setup(json({ code: "unprocessable", message: "重複" }, 422));
    const err = await client
      .createGenre({ name: "x", query_template: "{name}", sort_order: 0 })
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ code: "unprocessable", message: "重複", status: 422 });
  });
  it("AC-API-05 JSON でないエラー応答はステータスから code を決める", async () => {
    const { client } = setup(new Response("<html>bad gateway</html>", { status: 502 }));
    await expect(client.listItems()).rejects.toMatchObject({ code: "bad_gateway", status: 502 });
    const s2 = setup(new Response("nope", { status: 401 }));
    await expect(s2.client.listItems()).rejects.toMatchObject({ code: "unauthorized", status: 401 });
  });
  it("AC-API-06 通信できないときは code=network・status=0", async () => {
    const { client } = setup(() => Promise.reject(new TypeError("Failed to fetch")));
    await expect(client.listItems()).rejects.toMatchObject({ code: "network", status: 0 });
  });
  it("fetch を渡さないときは呼び出し時点の globalThis.fetch を使う", async () => {
    const client = createApiClient({ baseUrl: "https://h.example/", token: "t" });
    const spy = vi.fn().mockResolvedValue(json({ items: [] }));
    vi.stubGlobal("fetch", spy);
    await client.listItems();
    expect(spy).toHaveBeenCalledOnce();
  });
});

// AC-API-07
describe("画像・ベース URL の解決", () => {
  it("resolveImageUrl はベース URL 基準(前置 /wishlist が抜けない)", () => {
    expect(resolveImageUrl("images/a.png", "https://h.example/wishlist/")).toBe(
      "https://h.example/wishlist/images/a.png",
    );
    expect(resolveImageUrl("images/a.png", "https://h.example/wishlist")).toBe(
      "https://h.example/wishlist/images/a.png",
    );
  });
  it("normalizeBaseUrl は末尾 / を補う(あれば足さない)", () => {
    expect(normalizeBaseUrl("https://h.example/wishlist")).toBe("https://h.example/wishlist/");
    expect(normalizeBaseUrl("https://h.example/wishlist/")).toBe("https://h.example/wishlist/");
  });
  it("defaultBaseUrl は baseURI のディレクトリ(クエリ・ハッシュ・ファイル名を除く)", () => {
    expect(defaultBaseUrl("https://h.example/wishlist/")).toBe("https://h.example/wishlist/");
    expect(defaultBaseUrl("https://h.example/wishlist/?a=1#b")).toBe("https://h.example/wishlist/");
    expect(defaultBaseUrl("https://h.example/wishlist/index.html")).toBe("https://h.example/wishlist/");
  });
  it("defaultBaseUrl は引数なしで document.baseURI を使う", () => {
    expect(defaultBaseUrl()).toBe(new URL("./", document.baseURI).href);
  });
});

// AC-API-08: フェーズ3(更新・出品一覧)
describe("createApiClient(フェーズ3)", () => {
  const estimates = { item_id: 5, sites: [], refreshing: true };
  it("AC-API-08 refreshEstimates は POST .../estimates/refresh(202 でも本文を返す。Bearer 付き)", async () => {
    const { client, last } = setup(json(estimates, 202));
    expect(await client.refreshEstimates(5)).toEqual(estimates);
    expect(last().url).toBe("https://h.example/wishlist/api/items/5/estimates/refresh");
    expect(last().init.method).toBe("POST");
    expect(last().headers.get("Authorization")).toBe("Bearer tok");
  });
  it("AC-API-08 listListings は GET .../listings の配列だけを返し、siteId があれば site_id を付ける", async () => {
    const { client, last } = setup(json({ listings: [] }));
    expect(await client.listListings(5)).toEqual([]);
    expect(last().url).toBe("https://h.example/wishlist/api/items/5/listings");
    expect(last().init.method ?? "GET").toBe("GET");
    await client.listListings(5, 2);
    expect(last().url).toBe("https://h.example/wishlist/api/items/5/listings?site_id=2");
  });
  it("AC-API-08 失敗は ApiError(通信失敗は network)", async () => {
    const { client } = setup(json({ code: "not_found", message: "x" }, 404));
    await expect(client.refreshEstimates(5)).rejects.toMatchObject({ code: "not_found", status: 404 });
    const down = setup(() => Promise.reject(new TypeError("Failed to fetch")));
    await expect(down.client.listListings(5)).rejects.toMatchObject({ code: "network", status: 0 });
  });
});
