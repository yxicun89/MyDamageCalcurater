// P4-10: URL で画面を切り替える(ルーターのライブラリは入れない。ADR-0300 §1)。
// 画面 ID ↔ パスの区切り・タブの表示名・文書のタイトルの対応を1か所(routes.ts)に置き、タブはそこから描画する
// (P4-12 タイプバランスなど以後の画面は、この表に1件足すだけで同じ仕組みに乗る)。
// パスは Vite の BASE_URL(import.meta.env.BASE_URL、末尾は "/")からの相対。ここでは base を引数で渡して確かめる。

import { describe, expect, test } from "vitest";
import { appText } from "../i18n/ja";
import { DEFAULT_SCREEN, SCREEN_ROUTES, documentTitle, pathForScreen, screenFromPath } from "./routes";

describe("ルート表", () => {
  // P4-12a(ADR-0303 §2): タイプバランス(/balance)を逆算の後ろに足す。
  test("計算 → calc、逆算 → reverse、タイプバランス → balance の順に並び、表示名は appText の語", () => {
    expect(SCREEN_ROUTES.map((route) => [route.id, route.segment, route.label])).toEqual([
      ["calc", "calc", appText.calcTabLabel],
      ["reverse", "reverse", appText.reverseTabLabel],
      ["balance", "balance", appText.balanceTabLabel],
    ]);
    expect(appText.balanceTabLabel).toBe("タイプバランス");
  });

  test("既定の画面は計算", () => {
    expect(DEFAULT_SCREEN).toBe("calc");
  });

  test("画面 ID・パスの区切りはどちらも重複しない", () => {
    const ids = SCREEN_ROUTES.map((route) => route.id);
    const segments = SCREEN_ROUTES.map((route) => route.segment);
    expect(new Set(ids).size).toBe(ids.length);
    expect(new Set(segments).size).toBe(segments.length);
  });
});

describe("パス → 画面(screenFromPath)", () => {
  test.each([
    ["/calc", "/", "calc"],
    ["/reverse", "/", "reverse"],
    // 末尾の "/" は同じ画面として扱う。
    ["/reverse/", "/", "reverse"],
    // base が付くときは、base からの相対で読む。
    ["/app/reverse", "/app/", "reverse"],
    ["/app/calc", "/app/", "calc"],
    ["/balance", "/", "balance"],
    ["/app/balance", "/app/", "balance"],
  ] as const)("%s(base %s)は %s", (pathname, base, expected) => {
    expect(screenFromPath(pathname, base)).toBe(expected);
  });

  test.each([
    // ルート直下・未知のパス・深いパス・base の外・大文字違いは、どの画面でもない(呼び出し側が既定へ置き換える)。
    ["/", "/"],
    ["", "/"],
    ["/unknown", "/"],
    ["/calc/extra", "/"],
    ["/Reverse", "/"],
    ["/app/", "/app/"],
    ["/reverse", "/app/"],
    ["/application/reverse", "/app/"],
  ] as const)("%s(base %s)は null", (pathname, base) => {
    expect(screenFromPath(pathname, base)).toBeNull();
  });
});

describe("画面 → パス(pathForScreen)", () => {
  test.each([
    ["calc", "/", "/calc"],
    ["reverse", "/", "/reverse"],
    ["calc", "/app/", "/app/calc"],
    ["reverse", "/app/", "/app/reverse"],
  ] as const)("%s(base %s)は %s", (id, base, expected) => {
    expect(pathForScreen(id, base)).toBe(expected);
  });

  test("すべての画面で、作ったパスを読み戻すと同じ画面になる", () => {
    for (const base of ["/", "/app/"]) {
      for (const route of SCREEN_ROUTES) {
        expect(screenFromPath(pathForScreen(route.id, base), base)).toBe(route.id);
      }
    }
  });
});

describe("文書のタイトル(documentTitle)", () => {
  test.each([
    ["calc", "計算 | pokecalc"],
    ["reverse", "逆算 | pokecalc"],
    ["balance", "タイプバランス | pokecalc"],
  ] as const)("%s は「%s」", (id, expected) => {
    expect(documentTitle(id)).toBe(expected);
  });
});
