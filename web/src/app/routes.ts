// P4-10: URL で画面を切り替える(ルーターのライブラリは入れない。ADR-0300 §1)。
// 画面 ID ↔ パスの区切り・タブの表示名・文書のタイトルの対応は、各画面の登録ファイル(`*.screen.tsx`)が正で、
// app/screens.tsx が集めた SCREENS からこの表を導く(ADR-0323。画面を足すときこのファイルは触らない)。
// パスは Vite の BASE_URL(import.meta.env.BASE_URL、末尾は "/")からの相対として扱う。

import { aboutText, appText } from "../i18n/ja";
import { ABOUT_SEGMENT, SCREENS } from "./screens";

/** 画面 ID・URL の区切り・タブの表示名の1エントリ。 */
export interface ScreenRoute {
  readonly id: string;
  /** base からの相対パスの1セグメント目(先頭・末尾の "/" は含まない)。 */
  readonly segment: string;
  /** タブの表示名(文言資源の語をそのまま使う)。 */
  readonly label: string;
  /**
   * issue 308(ADR-0304 追記6): この画面がマスタ(MasterData)を使うか。false の画面は、マスタの
   * 読み込みに失敗していてもタブを選んで使える。
   */
  readonly usesMaster: boolean;
}

/**
 * 画面 ID(登録ファイルの id。ADR-0323 で登録ファイルからの導出に変えたため文字列。
 * 重複・欠落は app/screens.tsx の buildScreenRegistry と app/screenRegistry.test.ts が守る)。
 */
export type ScreenId = string;

/** 画面の定義順(タブの並び順・ロービング tabIndex の移動順もこの順。登録ファイルの order の昇順)。 */
export const SCREEN_ROUTES: readonly ScreenRoute[] = SCREENS.map(({ id, segment, label, usesMaster }) => ({
  id,
  segment,
  label,
  usesMaster,
}));

/** 既定の画面(未知のパス・"/" のとき)。 */
export const DEFAULT_SCREEN: ScreenId = "calc";

function findRoute(id: ScreenId): ScreenRoute {
  const route = SCREEN_ROUTES.find((candidate) => candidate.id === id);
  if (route === undefined) {
    // 未知の画面 ID(登録ファイルに無い id)。呼び出し側は SCREEN_ROUTES の id だけを渡す。
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

/** issue 308: この画面がマスタ(MasterData)を使うか。 */
export function screenUsesMaster(id: ScreenId): boolean {
  return findRoute(id).usesMaster;
}

/** issue 308: id がマスタを使わない画面か。 */
export function isMasterlessScreen(id: ScreenId): boolean {
  return !screenUsesMaster(id);
}

/** 画面 ID から、base(末尾 "/")付きのパスを作る。screenFromPath の逆。 */
export function pathForScreen(id: ScreenId, base: string): string {
  return `${base}${findRoute(id).segment}`;
}

/** 文書のタイトル(「<画面名> | pokecalc」)。 */
export function documentTitle(id: ScreenId): string {
  return `${findRoute(id).label} | ${appText.siteTitle}`;
}

// 「このアプリについて」(ADR-0314)。タブ(SCREEN_ROUTES)ではない情報ページなので、segment(ABOUT_SEGMENT)は
// 画面の登録とは別に app/screens.tsx が持つ(登録ファイルがこの segment を使うと検証で弾く)。
// パスの読み方は screenFromPath と同じ(末尾の "/" 1つは同じ画面。大文字小文字違い・深いパスは別物)。

/** pathname が情報ページ(/about)か。 */
export function isAboutPath(pathname: string, base: string): boolean {
  if (!pathname.startsWith(base)) {
    return false;
  }
  const rest = pathname.slice(base.length);
  return (rest.endsWith("/") ? rest.slice(0, -1) : rest) === ABOUT_SEGMENT;
}

/** 情報ページの、base(末尾 "/")付きのパス。 */
export function pathForAbout(base: string): string {
  return `${base}${ABOUT_SEGMENT}`;
}

/** 情報ページの文書のタイトル。 */
export function aboutDocumentTitle(): string {
  return `${aboutText.pageHeading} | ${appText.siteTitle}`;
}
