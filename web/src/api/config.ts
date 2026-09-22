// P4-5: API の基点 URL(ADR-0301 §4)。環境変数 VITE_API_BASE_URL をここ1か所で読む
// (コーディング規約 §2「設定は1か所で読み込む」)。既定は同じオリジン "/"。
// 末尾のスラッシュを1つに揃え、`${apiBaseUrl()}api/calc` の形でそのまま連結できるようにする。

/** VITE_API_BASE_URL 未設定・空のときの既定値(同じオリジン)。 */
const DEFAULT_BASE_URL = "/";

/**
 * API の基点 URL。末尾はスラッシュ1つに揃える(`apiEngine.ts` はここに `api/calc` などを続けて連結する)。
 * gateway が `/api` と Web の両方を配る想定で、既定は同じオリジン(requirements.md §4)。
 */
export function apiBaseUrl(): string {
  const trimmed = (import.meta.env.VITE_API_BASE_URL ?? "").trim();
  if (trimmed === "") {
    return DEFAULT_BASE_URL;
  }
  return `${trimmed.replace(/\/+$/, "")}/`;
}
