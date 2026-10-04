// 画面の登録(ADR-0323)。各レーンのディレクトリにある `*.screen.tsx`(登録ファイル)を import.meta.glob で集め、
// 検証して order の昇順に並べる。画面を足すレーンは、自分のディレクトリに登録ファイルを置くだけでよい
// (このファイル・App.tsx・app/routes.ts・i18n/ja.ts は触らない)。

import type { RegisteredScreen, ScreenClientDeps, ScreenInstance } from "./screenDefinition";

/** 情報ページ(ADR-0314)の segment。タブではないので画面の登録とは別に持つ(app/routes.ts が使う)。 */
export const ABOUT_SEGMENT = "about";

/** タブ以外が使う、画面に割り当てられない segment。 */
export const RESERVED_SEGMENTS: readonly string[] = [ABOUT_SEGMENT];

const SEGMENT_PATTERN = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

/**
 * 登録ファイルの既定エクスポートが defineScreen の結果の形か(id・segment・label が文字列、order が数値、
 * usesMaster が真偽値、instantiate が関数。hidden は省略可で、あるなら真偽値)。React コンポーネントをそのまま既定エクスポートにした誤りなどを弾く。
 */
function isRegisteredScreen(value: unknown): value is RegisteredScreen {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  return (
    "id" in value &&
    typeof value.id === "string" &&
    "segment" in value &&
    typeof value.segment === "string" &&
    "label" in value &&
    typeof value.label === "string" &&
    "order" in value &&
    typeof value.order === "number" &&
    "usesMaster" in value &&
    typeof value.usesMaster === "boolean" &&
    (!("hidden" in value) || typeof value.hidden === "boolean") &&
    "instantiate" in value &&
    typeof value.instantiate === "function"
  );
}

/**
 * 登録ファイル(パス → 既定エクスポート)を検証して order の昇順に並べる。次のときは例外を投げる:
 * 既定エクスポートが無い・defineScreen の結果の形でない・id/segment/order の重複・segment の形が不正・予約済みの segment・空の表示名・order が有限の数でない。
 */
export function buildScreenRegistry(modules: Readonly<Record<string, unknown>>): readonly RegisteredScreen[] {
  const screens: RegisteredScreen[] = [];
  const seenIds = new Map<string, string>();
  const seenSegments = new Map<string, string>();
  const seenOrders = new Map<number, string>();
  for (const [path, screen] of Object.entries(modules)) {
    if (screen === undefined) {
      throw new Error(`screen registry: ${path} に既定エクスポート(defineScreen)がありません`);
    }
    if (!isRegisteredScreen(screen)) {
      throw new Error(
        `screen registry: ${path} の既定エクスポートが defineScreen の結果ではありません(コンポーネントを直接エクスポートしていないか確認)`,
      );
    }
    const duplicateId = seenIds.get(screen.id);
    if (duplicateId !== undefined) {
      throw new Error(`screen registry: id "${screen.id}" が重複しています(${duplicateId} と ${path})`);
    }
    const duplicateSegment = seenSegments.get(screen.segment);
    if (duplicateSegment !== undefined) {
      throw new Error(
        `screen registry: segment "${screen.segment}" が重複しています(${duplicateSegment} と ${path})`,
      );
    }
    if (!Number.isFinite(screen.order)) {
      throw new Error(`screen registry: ${path} の order が有限の数ではありません`);
    }
    const duplicateOrder = seenOrders.get(screen.order);
    if (duplicateOrder !== undefined) {
      throw new Error(
        `screen registry: order ${String(screen.order)} が重複しています(${duplicateOrder} と ${path})`,
      );
    }
    if (!SEGMENT_PATTERN.test(screen.segment)) {
      throw new Error(`screen registry: ${path} の segment "${screen.segment}" の形が不正です`);
    }
    if (RESERVED_SEGMENTS.includes(screen.segment)) {
      throw new Error(`screen registry: ${path} の segment "${screen.segment}" は予約済みです`);
    }
    if (screen.id === "" || screen.label === "") {
      throw new Error(`screen registry: ${path} の id または表示名が空です`);
    }
    seenIds.set(screen.id, path);
    seenSegments.set(screen.segment, path);
    seenOrders.set(screen.order, path);
    screens.push(screen);
  }
  return screens.sort((a, b) => a.order - b.order);
}

/**
 * 登録ファイル(eager: 遅延読み込みにはしない。バンドルの形は今までと同じ)。`*.screen.tsx` は本番に取り込まれてタブになるので、
 * テスト用の fixture にはこの名前を使わない(念のため `*.test.screen.tsx` は除外する)。
 */
const registeredModules = import.meta.glob<unknown>(["../**/*.screen.tsx", "!../**/*.test.screen.tsx"], {
  eager: true,
  import: "default",
});

/** 登録された画面(タブの並び順)。 */
export const SCREENS: readonly RegisteredScreen[] = buildScreenRegistry(registeredModules);

/** 非表示(hidden。ADR-0330)でない画面だけを、順序を保って返す。タブ・URL・マウントの対象はこの画面だけ。 */
export function visibleScreens(screens: readonly RegisteredScreen[]): readonly RegisteredScreen[] {
  return screens.filter((screen) => !screen.hidden);
}

/**
 * 表示する画面(hidden を除く)のクライアントを作り、描画の口を並び順で返す(App のマウント時に1回だけ呼ぶ)。
 * hidden の画面は createClient も呼ばず、描画の口も返さない。
 */
export function instantiateScreens(
  screens: readonly RegisteredScreen[],
  deps: ScreenClientDeps,
): readonly ScreenInstance[] {
  return visibleScreens(screens).map((screen) => screen.instantiate(deps));
}
