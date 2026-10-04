// ポケモン画像の manifest(ADR-0808 の契約、ADR-0325)を読む純粋関数。
// manifest は { version: 1, images: { "<種族キー>": { thumb, detail } } }。画像の URL は "/images/" + 相対パス。
// 取れない・不正・version 違い・キー無し・危険なパスは「画像なし」(= タイプ色エンブレム)。エラーにはしない。

/** 画像の配信パス(同一オリジン。CSP の img-src 'self' に収まる)。 */
export const POKEMON_IMAGES_BASE_PATH = "/images/";
export const POKEMON_IMAGES_MANIFEST_PATH = `${POKEMON_IMAGES_BASE_PATH}manifest.json`;

export type PokemonImageSize = "thumb" | "detail";

interface ImageEntry {
  readonly thumb: string;
  readonly detail: string;
}

/** 検証済みの manifest。キーはプロトタイプを持たない Map で引く。 */
export interface PokemonImageManifest {
  readonly images: ReadonlyMap<string, ImageEntry>;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/**
 * `.webp` で終わる安全な相対パスだけ true(.. ・絶対・スキーム・// ・バックスラッシュ・クエリ・フラグメント・空は不可)。
 * `%` も不可: `%2e%2e` は URL の仕様で `..` と同じに扱われ、`/images/` の外へ出られる。正規のパスは
 * 内容 hash 付きの ASCII ファイル名(変換ツール tools/assets が作る。ADR-0808)で、パーセントエンコードは要らない。
 */
function isSafeRelativeWebpPath(path: unknown): path is string {
  if (typeof path !== "string" || path === "" || !path.endsWith(".webp")) {
    return false;
  }
  if (/[\\?#:%]/.test(path) || path.startsWith("/")) {
    return false;
  }
  return path.split("/").every((segment) => segment !== "" && segment !== "." && segment !== "..");
}

/** 不正な形は null。不正なエントリだけを落とし、他のキーは残す。 */
export function parsePokemonImageManifest(input: unknown): PokemonImageManifest | null {
  if (!isRecord(input) || input["version"] !== 1) {
    return null;
  }
  const raw = input["images"];
  if (!isRecord(raw)) {
    return null;
  }
  const images = new Map<string, ImageEntry>();
  for (const [key, entry] of Object.entries(raw)) {
    if (
      isRecord(entry) &&
      isSafeRelativeWebpPath(entry["thumb"]) &&
      isSafeRelativeWebpPath(entry["detail"])
    ) {
      images.set(key, { thumb: entry["thumb"], detail: entry["detail"] });
    }
  }
  return { images };
}

/** 種族キーの画像 URL。manifest が無い・キーが無いときは null(エンブレム)。 */
export function pokemonImageUrl(
  manifest: PokemonImageManifest | null,
  key: string,
  size: PokemonImageSize,
): string | null {
  const entry = manifest?.images.get(key);
  return entry === undefined ? null : `${POKEMON_IMAGES_BASE_PATH}${entry[size]}`;
}

/** manifest を取得する。失敗(HTTP エラー・不正な JSON・HTML・ネットワーク失敗)はすべて null。 */
export async function fetchPokemonImageManifest(
  fetchImpl: typeof fetch,
): Promise<PokemonImageManifest | null> {
  try {
    const response = await fetchImpl(POKEMON_IMAGES_MANIFEST_PATH, { method: "GET" });
    if (!response.ok) {
      return null;
    }
    return parsePokemonImageManifest(await response.json());
  } catch {
    return null;
  }
}
