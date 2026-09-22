// P4-10: URL で画面を切り替える(ルーターのライブラリは入れない。ADR-0300 §1)。
// 画面 ID ↔ パスの区切り・タブの表示名・文書のタイトルの対応を1か所(この表)に置く。
// 画面を増やすときは、SCREEN_ROUTES に1件・表示名を i18n/ja.ts に1件・画面のコンポーネントを
// app/screens.tsx の SCREEN_COMPONENTS に1件足す(App.tsx は触らない。足し忘れは型エラーになる)。
// パスは Vite の BASE_URL(import.meta.env.BASE_URL、末尾は "/")からの相対として扱う。

import { appText } from "../i18n/ja";

/** 画面 ID・URL の区切り・タブの表示名の1エントリ。 */
export interface ScreenRoute {
  readonly id: string;
  /** base からの相対パスの1セグメント目(先頭・末尾の "/" は含まない)。 */
  readonly segment: string;
  /** タブの表示名(appText の語をそのまま使う)。 */
  readonly label: string;
}

/**
 * 画面の定義順(タブの並び順・ロービング tabIndex の移動順もこの順)。
 * 画面を追加するときはここに1件足す(P4-12 のタイプバランスなど)。
 */
export const SCREEN_ROUTES = [
  { id: "calc", segment: "calc", label: appText.calcTabLabel },
  { id: "reverse", segment: "reverse", label: appText.reverseTabLabel },
] as const satisfies readonly ScreenRoute[];

/** 画面 ID(SCREEN_ROUTES から導出する。手で union を書かない)。 */
export type ScreenId = (typeof SCREEN_ROUTES)[number]["id"];

/** 既定の画面(未知のパス・"/" のとき)。 */
export const DEFAULT_SCREEN: ScreenId = "calc";

function findRoute(id: ScreenId): ScreenRoute {
  const route = SCREEN_ROUTES.find((candidate) => candidate.id === id);
  if (route === undefined) {
    // SCREEN_ROUTES が ScreenId を過不足なく覆っている限り起きない(型で保証)。
    throw new Error(`unknown screen id: ${id}`);
  }
  return route;
}

/**
 * base(import.meta.env.BASE_URL、末尾は "/")からの相対で pathname を読み、画面 ID を返す。
 * 次のときは null(未知のパス。呼び出し側が既定の画面へ置き換える):
 * ルート直下("/" や base そのもの)・空文字・未知のセグメント・深いパス・base の外・大文字小文字違い。
 * セグメントの後ろの "/" 1つだけは同じ画面として許す("/reverse/" も "/reverse" と同じ)。
 */
export function screenFromPath(pathname: string, base: string): ScreenId | null {
  if (!pathname.startsWith(base)) {
    return null;
  }
  const rest = pathname.slice(base.length);
  const trimmed = rest.endsWith("/") ? rest.slice(0, -1) : rest;
  if (trimmed === "" || trimmed.includes("/")) {
    return null;
  }
  const route = SCREEN_ROUTES.find((candidate) => candidate.segment === trimmed);
  return route === undefined ? null : route.id;
}

/** 画面のタブの表示名。 */
export function screenLabel(id: ScreenId): string {
  return findRoute(id).label;
}

/** 画面 ID から、base(末尾 "/")付きのパスを作る。screenFromPath の逆。 */
export function pathForScreen(id: ScreenId, base: string): string {
  return `${base}${findRoute(id).segment}`;
}

/** 文書のタイトル(「<画面名> | pokecalc」)。 */
export function documentTitle(id: ScreenId): string {
  return `${findRoute(id).label} | ${appText.siteTitle}`;
}
