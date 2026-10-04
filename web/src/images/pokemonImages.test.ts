// P8-1c(ADR-0325): ポケモン画像の manifest(ADR-0808)を読む純粋関数。
// 契約: manifest は { version: 1, images: { "<種族キー>": { thumb, detail } } }。画像の URL は "/images/" + 相対パス。
// manifest が取れない・不正・version 違い・キー無し・危険なパスは「画像なし」= タイプ色エンブレム(エラーにしない)。

import { describe, expect, test, vi } from "vitest";
import {
  POKEMON_IMAGES_BASE_PATH,
  POKEMON_IMAGES_MANIFEST_PATH,
  fetchPokemonImageManifest,
  parsePokemonImageManifest,
  pokemonImageUrl,
} from "./pokemonImages";

const VALID = {
  version: 1,
  images: {
    "0445-000": { thumb: "thumb/0445-000.ab12cd34.webp", detail: "detail/0445-000.ef567890.webp" },
  },
};

describe("定数", () => {
  test("manifest は同一オリジンの /images/manifest.json(CSP img-src 'self' に収まる)", () => {
    expect(POKEMON_IMAGES_BASE_PATH).toBe("/images/");
    expect(POKEMON_IMAGES_MANIFEST_PATH).toBe("/images/manifest.json");
  });
});

describe("parsePokemonImageManifest", () => {
  test("正しい manifest を読み、URL を /images/ + 相対パスで組み立てる", () => {
    const manifest = parsePokemonImageManifest(VALID);
    expect(manifest).not.toBeNull();
    expect(pokemonImageUrl(manifest, "0445-000", "thumb")).toBe("/images/thumb/0445-000.ab12cd34.webp");
    expect(pokemonImageUrl(manifest, "0445-000", "detail")).toBe("/images/detail/0445-000.ef567890.webp");
  });

  test("画像が0件の manifest は有効(全部エンブレム)", () => {
    const manifest = parsePokemonImageManifest({ version: 1, images: {} });
    expect(manifest).not.toBeNull();
    expect(pokemonImageUrl(manifest, "0445-000", "thumb")).toBeNull();
  });

  test.each<[string, unknown]>([
    ["null", null],
    ["配列", []],
    ["文字列", "manifest"],
    ["version が無い", { images: {} }],
    ["version が 2", { version: 2, images: {} }],
    ["version が文字列の '1'", { version: "1", images: {} }],
    ["images が無い", { version: 1 }],
    ["images が配列", { version: 1, images: [] }],
    ["images が null", { version: 1, images: null }],
  ])("不正な形(%s)は null(画像なし)", (_name, input) => {
    expect(parsePokemonImageManifest(input)).toBeNull();
  });

  test("不正なエントリだけを落とし、他のキーは使える(manifest 全体は捨てない)", () => {
    const manifest = parsePokemonImageManifest({
      version: 1,
      images: {
        ...VALID.images,
        "0001-000": { thumb: 1, detail: "detail/0001-000.aaaaaaaa.webp" },
        "0002-000": { thumb: "thumb/0002-000.aaaaaaaa.webp" },
        "0003-000": null,
      },
    });
    expect(pokemonImageUrl(manifest, "0445-000", "thumb")).toBe("/images/thumb/0445-000.ab12cd34.webp");
    for (const key of ["0001-000", "0002-000", "0003-000"]) {
      expect(pokemonImageUrl(manifest, key, "thumb")).toBeNull();
      expect(pokemonImageUrl(manifest, key, "detail")).toBeNull();
    }
  });

  test.each<[string, string]>([
    ["親ディレクトリ", "../secret.webp"],
    ["途中の ..", "thumb/../../x.webp"],
    ["パーセントエンコードの ..(小文字)", "%2e%2e/x.webp"],
    ["パーセントエンコードの ..(大文字)", "%2E%2E/x.webp"],
    ["途中のパーセントエンコードの ..", "a/%2e%2e/b.webp"],
    ["絶対パス", "/etc/passwd"],
    ["絶対 URL(https)", "https://evil.example/x.webp"],
    ["スキーム相対 URL", "//evil.example/x.webp"],
    ["data: URL", "data:image/webp;base64,AAAA"],
    ["javascript: スキーム", "javascript:alert(1)"],
    ["バックスラッシュ", "thumb\\x.webp"],
    ["クエリ・フラグメント", "thumb/x.webp?x=1#y"],
    ["空文字", ""],
    ["webp 以外", "thumb/x.svg"],
  ])("危険・不正な相対パス(%s)のエントリは不採用 = エンブレム", (_name, path) => {
    const manifest = parsePokemonImageManifest({
      version: 1,
      images: { "0445-000": { thumb: path, detail: "detail/0445-000.ef567890.webp" } },
    });
    expect(pokemonImageUrl(manifest, "0445-000", "thumb")).toBeNull();
    expect(pokemonImageUrl(manifest, "0445-000", "detail")).toBeNull();
  });
});

describe("pokemonImageUrl", () => {
  test("manifest が null(未取得・失敗)なら null", () => {
    expect(pokemonImageUrl(null, "0445-000", "thumb")).toBeNull();
  });

  test("manifest に無いキーは null(エンブレム)。Object のプロトタイプのキーも引かない", () => {
    const manifest = parsePokemonImageManifest(VALID);
    expect(pokemonImageUrl(manifest, "9999-000", "thumb")).toBeNull();
    expect(pokemonImageUrl(manifest, "constructor", "thumb")).toBeNull();
    expect(pokemonImageUrl(manifest, "__proto__", "thumb")).toBeNull();
  });
});

describe("fetchPokemonImageManifest", () => {
  function response(status: number, body: string): Response {
    return new Response(body, { status });
  }

  test("同一オリジンの /images/manifest.json を GET し、読めた manifest を返す", async () => {
    const fetchMock = vi.fn<typeof fetch>(() => Promise.resolve(response(200, JSON.stringify(VALID))));
    const manifest = await fetchPokemonImageManifest(fetchMock);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/images/manifest.json");
    // <img> と違い fetch は X-Device-Id を付けられるが、画像側が課さないので付けない(契約: ADR-0808)。
    const init = fetchMock.mock.calls[0]?.[1];
    expect(JSON.stringify(init?.headers ?? {})).not.toMatch(/device/i);
    expect(pokemonImageUrl(manifest, "0445-000", "thumb")).toBe("/images/thumb/0445-000.ab12cd34.webp");
  });

  test.each<[string, () => Promise<Response>]>([
    ["404", () => Promise.resolve(response(404, '{"error":"not_found"}'))],
    ["500", () => Promise.resolve(response(500, ""))],
    ["不正な JSON", () => Promise.resolve(response(200, "{not json"))],
    [
      "HTML(SPA のフォールバックが返る環境)",
      () => Promise.resolve(response(200, "<!doctype html><html></html>")),
    ],
    ["version 違い", () => Promise.resolve(response(200, JSON.stringify({ version: 2, images: {} })))],
    ["ネットワーク失敗", () => Promise.reject(new TypeError("Failed to fetch"))],
  ])("%s は例外を投げず null(画像なし)", async (_name, impl) => {
    const fetchMock = vi.fn<typeof fetch>(impl);
    await expect(fetchPokemonImageManifest(fetchMock)).resolves.toBeNull();
  });
});
