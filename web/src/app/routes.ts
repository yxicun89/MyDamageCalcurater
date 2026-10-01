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
  /**
   * issue 308(ADR-0304 追記6): この画面がマスタ(MasterData)を使うか。false の画面は、マスタの
   * 読み込みに失敗していてもタブを選んで使える(app/screens.tsx の ScreenProps.master は使わない画面だけ
   * 省略できる)。画面を追加するときは必ずどちらかを書く(省略できないので足し忘れは型エラーになる)。
   */
  readonly usesMaster: boolean;
}

/**
 * 画面の定義順(タブの並び順・ロービング tabIndex の移動順もこの順)。
 * 画面を追加するときはここに1件足す(P4-12 のタイプバランスなど)。
 */
export const SCREEN_ROUTES = [
  { id: "calc", segment: "calc", label: appText.calcTabLabel, usesMaster: true },
  { id: "reverse", segment: "reverse", label: appText.reverseTabLabel, usesMaster: true },
  // P4-12a(ADR-0303 §2): タイプバランス。
  { id: "balance", segment: "balance", label: appText.balanceTabLabel, usesMaster: true },
  // SP3(ADR-0604 §2): 素早さ比較。engine(WASM)・master(pokedex のマスタ)のどちらも使わない
  // (ADR-0604 §5)。issue 308: マスタの読み込みに失敗していても、このタブだけは使える。
  { id: "speed", segment: "speed", label: appText.speedTabLabel, usesMaster: false },
  // JD5(ADR-0705 §1): 判定(抜けて倒せるか・返り討ちに遭うか)。
  { id: "judge", segment: "judge", label: appText.judgeTabLabel, usesMaster: true },
  // P5-5 PR-A1(ADR-0309 §1): 構築ビルダー(一覧・新規作成・名前変更・削除)。PR-A1 自体はマスタ不要だが、
  // PR-A2 のメンバー編集で種族・技・持ち物・特性の名前解決にマスタが要るため、最初から usesMaster: true。
  { id: "team", segment: "team", label: appText.teamTabLabel, usesMaster: true },
] as const satisfies readonly ScreenRoute[];

/** 画面 ID(SCREEN_ROUTES から導出する。手で union を書かない)。 */
export type ScreenId = (typeof SCREEN_ROUTES)[number]["id"];

/**
 * issue 308(ADR-0304 追記6): マスタを使わない画面 ID だけの union(usesMaster: false の行から導出する。
 * 手で union を書かない)。app/screens.tsx の `MASTERLESS_SCREEN_COMPONENTS` が `Record<MasterlessScreenId, ...>`
 * なので、usesMaster: false の画面を足したのに対応するコンポーネントを登録し忘れると型エラーになる。
 */
export type MasterlessScreenId = Extract<
  (typeof SCREEN_ROUTES)[number],
  { readonly usesMaster: false }
>["id"];

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

/** issue 308: この画面がマスタ(MasterData)を使うか。 */
export function screenUsesMaster(id: ScreenId): boolean {
  return findRoute(id).usesMaster;
}

/**
 * issue 308: id がマスタを使わない画面か(型ガード)。App.tsx はこれで `tab: ScreenId` を
 * `MasterlessScreenId` に絞り込み、`MASTERLESS_SCREEN_COMPONENTS[tab]` を型キャスト無しで引く。
 */
export function isMasterlessScreen(id: ScreenId): id is MasterlessScreenId {
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
