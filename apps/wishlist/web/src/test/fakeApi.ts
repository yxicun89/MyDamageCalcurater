import { vi } from "vitest";
import type { Genre, Item, Site } from "../api/types";
import { NOW, makeItem } from "./factories";
import { urlOf } from "./net";

export interface Call {
  method: string;
  /** 前置・クエリを除いた `/api/...` 部分 */
  path: string;
  search: string;
  headers: Headers;
  /** JSON ボディ(JSON でないときは undefined) */
  json?: unknown;
  form?: FormData;
}

export interface FakeApi {
  items: Item[];
  genres: Genre[];
  sites: Site[];
  calls: Call[];
  /** true の間はすべての fetch が TypeError で失敗する(Mac が落ちている状態) */
  offline: boolean;
  /** POST /api/items/from-url が返す下書き */
  draft: { name: string; image_url: string | null; source_url: string; genre_id: number | null };
  /** path の接頭辞が合う呼び出しのうち method 一致のもの */
  callsTo(method: string, pathPrefix: string): Call[];
}

const jsonRes = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
const errRes = (status: number, code: string) => jsonRes({ code, message: code }, status);

/**
 * 契約テスト用の最小のバックエンド。globalThis.fetch を差し替える。認証(Bearer が "test-token")も検査する。
 * 受け付けるのは /api/... のみ(画像は画面側が <img src> で読むだけで fetch しない)。
 */
export function installFakeApi(init: { items?: Item[]; genres?: Genre[]; sites?: Site[] } = {}): FakeApi {
  const api: FakeApi = {
    items: init.items ?? [],
    genres: init.genres ?? [],
    sites: init.sites ?? [],
    calls: [],
    offline: false,
    draft: {
      name: "下書きの名前",
      image_url: "https://img.example/og.png",
      source_url: "https://shop.example/p/1",
      genre_id: null,
    },
    callsTo: (method, prefix) => api.calls.filter((c) => c.method === method && c.path.startsWith(prefix)),
  };
  let nextId = 1000;

  const route = (input: RequestInfo | URL, reqInit: RequestInit): Response => {
    const url = new URL(urlOf(input), "http://localhost/");
    const path = url.pathname.slice(url.pathname.indexOf("/api/"));
    const method = (reqInit.method ?? "GET").toUpperCase();
    const headers = new Headers(reqInit.headers);
    const call: Call = { method, path, search: url.search, headers };
    if (typeof reqInit.body === "string") call.json = JSON.parse(reqInit.body);
    if (reqInit.body instanceof FormData) call.form = reqInit.body;
    api.calls.push(call);

    if (headers.get("Authorization") !== "Bearer test-token") return errRes(401, "unauthorized");
    const body = call.json as Record<string, unknown> | undefined;

    if (path === "/api/items" && method === "GET") {
      const g = url.searchParams.get("genre_id");
      return jsonRes({ items: g ? api.items.filter((i) => i.genre_id === Number(g)) : api.items });
    }
    if (path === "/api/items" && method === "POST") {
      const f = call.form;
      const fields = f
        ? { genre_id: Number(f.get("genre_id")), name: f.get("name") as string }
        : (body as { genre_id: number; name: string });
      const item = makeItem({ ...fields, id: nextId++ });
      api.items.push(item);
      return jsonRes(item, 201);
    }
    if (path === "/api/items/from-url" && method === "POST") return jsonRes(api.draft);
    let m = /^\/api\/items\/(\d+)$/.exec(path);
    if (m) {
      const id = Number(m[1]);
      const idx = api.items.findIndex((i) => i.id === id);
      const cur = api.items[idx];
      if (!cur) return errRes(404, "not_found");
      if (method === "DELETE") {
        api.items.splice(idx, 1);
        return new Response(null, { status: 204 });
      }
      if (method === "PATCH") {
        const next = { ...cur, ...body, updated_at: NOW };
        api.items[idx] = next;
        return jsonRes(next);
      }
      if (method === "GET") return jsonRes(cur);
    }
    m = /^\/api\/items\/(\d+)\/image$/.exec(path);
    if (m && method === "PUT") {
      const id = Number(m[1]);
      const idx = api.items.findIndex((i) => i.id === id);
      const cur = api.items[idx];
      if (!cur) return errRes(404, "not_found");
      const next = { ...cur, image_url: "images/11111111-1111-4111-8111-111111111111.png" };
      api.items[idx] = next;
      return jsonRes(next);
    }
    m = /^\/api\/items\/(\d+)\/estimates$/.exec(path);
    if (m && method === "GET") return jsonRes({ item_id: Number(m[1]), sites: [], refreshing: false });
    if (path === "/api/genres" && method === "GET") return jsonRes({ genres: api.genres });
    if (path === "/api/genres" && method === "POST") {
      const g = { sort_order: 0, site_ids: [], ...body, id: nextId++ } as unknown as Genre;
      api.genres.push(g);
      return jsonRes(g, 201);
    }
    m = /^\/api\/genres\/(\d+)$/.exec(path);
    if (m && method === "PATCH") {
      const id = Number(m[1]);
      const idx = api.genres.findIndex((g) => g.id === id);
      const cur = api.genres[idx];
      if (!cur) return errRes(404, "not_found");
      const next = { ...cur, ...body };
      api.genres[idx] = next;
      return jsonRes(next);
    }
    if (path === "/api/sites" && method === "GET") return jsonRes({ sites: api.sites });
    if (path === "/api/sites" && method === "POST") {
      const s = { fetch_type: "link_only", is_reference: false, ...body, id: nextId++ } as unknown as Site;
      api.sites.push(s);
      return jsonRes(s, 201);
    }
    m = /^\/api\/sites\/(\d+)$/.exec(path);
    if (m && method === "PATCH") {
      const id = Number(m[1]);
      const idx = api.sites.findIndex((x) => x.id === id);
      const cur = api.sites[idx];
      if (!cur) return errRes(404, "not_found");
      const next = { ...cur, ...body };
      api.sites[idx] = next;
      return jsonRes(next);
    }
    return errRes(404, "not_found");
  };

  const handler = (input: RequestInfo | URL, reqInit: RequestInit = {}): Promise<Response> =>
    api.offline ? Promise.reject(new TypeError("Failed to fetch")) : Promise.resolve(route(input, reqInit));
  vi.stubGlobal("fetch", vi.fn(handler));
  return api;
}

/** 設定画面で保存済みの状態にする(キー・形式は src/lib/settings.ts と同じ契約)。 */
export function seedSettings(token = "test-token"): void {
  localStorage.setItem("wishlist.settings", JSON.stringify({ apiBaseUrl: null, token }));
}
