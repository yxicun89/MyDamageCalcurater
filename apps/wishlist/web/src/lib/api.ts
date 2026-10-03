import type {
  ApiErrorBody,
  Genre,
  GenreCreate,
  GenreUpdate,
  Item,
  ItemDraft,
  ItemEstimates,
  ItemFields,
  ItemUpdate,
  Listing,
  PriceHistory,
  Site,
  SiteCreate,
  SiteUpdate,
} from "../api/types";

/** API の code に、通信できなかったときの "network" を足したもの */
export type ApiErrorCode = ApiErrorBody["code"] | "network";

export class ApiError extends Error {
  readonly code: ApiErrorCode;
  /** HTTP ステータス。通信できなかったときは 0 */
  readonly status: number;
  constructor(code: ApiErrorCode, message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
  }
}

export interface ApiClientOptions {
  /** 末尾 / の有無は問わない(内部で正規化する) */
  baseUrl: string;
  token: string;
  /** 省略時は呼び出し時点の globalThis.fetch(vi.stubGlobal で差し替えられるように、生成時に捕まえない) */
  fetch?: typeof fetch;
}

export type ImageFile = Blob;

export interface ApiClient {
  listItems(genreId?: number): Promise<Item[]>;
  createItemWithImage(fields: ItemFields, image: ImageFile): Promise<Item>;
  createItemFromImageUrl(fields: ItemFields & { image_url: string }): Promise<Item>;
  draftFromUrl(url: string, genreId?: number): Promise<ItemDraft>;
  updateItem(id: number, patch: ItemUpdate): Promise<Item>;
  deleteItem(id: number): Promise<void>;
  replaceItemImage(id: number, image: ImageFile): Promise<Item>;
  getEstimates(id: number): Promise<ItemEstimates>;
  /** POST .../estimates/refresh(202)。本文は現時点のキャッシュ(GET と同じ形) */
  refreshEstimates(id: number): Promise<ItemEstimates>;
  /** GET .../listings。参考外を含む。siteId を渡すとそのサイトだけ */
  listListings(id: number, siteId?: number): Promise<Listing[]>;
  /** GET .../price-history(フェーズ4-2)。days を省くとクエリを付けない(サーバーの既定 90 日) */
  getPriceHistory(id: number, days?: number): Promise<PriceHistory>;
  listGenres(): Promise<Genre[]>;
  createGenre(body: GenreCreate): Promise<Genre>;
  updateGenre(id: number, patch: GenreUpdate): Promise<Genre>;
  listSites(): Promise<Site[]>;
  createSite(body: SiteCreate): Promise<Site>;
  updateSite(id: number, patch: SiteUpdate): Promise<Site>;
}

const CODE_BY_STATUS: Record<number, ApiErrorBody["code"]> = {
  400: "bad_request",
  401: "unauthorized",
  404: "not_found",
  422: "unprocessable",
  501: "not_implemented",
  502: "bad_gateway",
};

const ERROR_CODES: readonly string[] = [
  "bad_request",
  "unauthorized",
  "not_found",
  "unprocessable",
  "bad_gateway",
  "not_implemented",
  "internal",
];

/** 末尾に / を補う。 */
export const normalizeBaseUrl = (baseUrl: string): string =>
  baseUrl.endsWith("/") ? baseUrl : `${baseUrl}/`;

/** 既定のベース URL。document.baseURI(= index.html のある場所。クラスタでは .../wishlist/)から、クエリとハッシュを除いたディレクトリ。 */
export const defaultBaseUrl = (baseURI: string = document.baseURI): string => new URL("./", baseURI).href;

/** Item.image_url(`images/<name>`。先頭 / なし)をベース URL 基準で解決する。 */
export const resolveImageUrl = (imageUrl: string, baseUrl: string): string =>
  new URL(imageUrl, normalizeBaseUrl(baseUrl)).href;

async function toApiError(res: Response): Promise<ApiError> {
  try {
    const body = (await res.clone().json()) as Partial<ApiErrorBody>;
    if (
      typeof body.code === "string" &&
      ERROR_CODES.includes(body.code) &&
      typeof body.message === "string"
    ) {
      return new ApiError(body.code, body.message, res.status);
    }
  } catch {
    // JSON でない応答(プロキシのエラーページなど)はステータスから決める。
  }
  return new ApiError(CODE_BY_STATUS[res.status] ?? "internal", `HTTP ${String(res.status)}`, res.status);
}

type Query = Record<string, string | number | undefined>;

export const createApiClient = (options: ApiClientOptions): ApiClient => {
  const base = normalizeBaseUrl(options.baseUrl);

  async function request<T>(
    method: string,
    path: string,
    body?: { json?: unknown; form?: FormData },
    query: Query = {},
  ): Promise<T> {
    const url = new URL(path, base);
    for (const [k, v] of Object.entries(query)) if (v !== undefined) url.searchParams.set(k, String(v));
    const headers = new Headers({ Authorization: `Bearer ${options.token}` });
    const init: RequestInit = { method, headers };
    if (body?.json !== undefined) {
      headers.set("Content-Type", "application/json");
      init.body = JSON.stringify(body.json);
    } else if (body?.form) {
      init.body = body.form; // Content-Type は boundary 付きでブラウザが付ける。
    }
    let res: Response;
    try {
      // 呼び出し時点の globalThis.fetch を使う(生成時に捕まえない)。
      res = await (options.fetch ?? globalThis.fetch)(url.href, init);
    } catch (e) {
      throw new ApiError("network", e instanceof Error ? e.message : "network error", 0);
    }
    if (!res.ok) throw await toApiError(res);
    if (res.status === 204) return undefined as T;
    // clone して読む(テストの fetch モックが同じ Response を使い回しても 2 回目が読めるように)。
    return (await res.clone().json()) as T;
  }

  const toForm = (fields: ItemFields, image: ImageFile): FormData => {
    const form = new FormData();
    for (const [k, v] of Object.entries<string | number | boolean | undefined>(fields))
      if (v !== undefined) form.append(k, String(v));
    form.append("image", image);
    return form;
  };

  return {
    listItems: async (genreId) =>
      (await request<{ items: Item[] }>("GET", "api/items", undefined, { genre_id: genreId })).items,
    createItemWithImage: (fields, image) => request("POST", "api/items", { form: toForm(fields, image) }),
    createItemFromImageUrl: (fields) => request("POST", "api/items", { json: fields }),
    draftFromUrl: (url, genreId) =>
      request("POST", "api/items/from-url", {
        json: genreId === undefined ? { url } : { url, genre_id: genreId },
      }),
    updateItem: (id, patch) => request("PATCH", `api/items/${String(id)}`, { json: patch }),
    deleteItem: (id) => request("DELETE", `api/items/${String(id)}`),
    replaceItemImage: (id, image) => {
      const form = new FormData();
      form.append("image", image);
      return request("PUT", `api/items/${String(id)}/image`, { form });
    },
    getEstimates: (id) => request("GET", `api/items/${String(id)}/estimates`),
    refreshEstimates: (id) => request("POST", `api/items/${String(id)}/estimates/refresh`),
    listListings: async (id, siteId) =>
      (
        await request<{ listings: Listing[] }>("GET", `api/items/${String(id)}/listings`, undefined, {
          site_id: siteId,
        })
      ).listings,
    getPriceHistory: (id, days) =>
      request("GET", `api/items/${String(id)}/price-history`, undefined, { days }),
    listGenres: async () => (await request<{ genres: Genre[] }>("GET", "api/genres")).genres,
    createGenre: (body) => request("POST", "api/genres", { json: body }),
    updateGenre: (id, patch) => request("PATCH", `api/genres/${String(id)}`, { json: patch }),
    listSites: async () => (await request<{ sites: Site[] }>("GET", "api/sites")).sites,
    createSite: (body) => request("POST", "api/sites", { json: body }),
    updateSite: (id, patch) => request("PATCH", `api/sites/${String(id)}`, { json: patch }),
  };
};
