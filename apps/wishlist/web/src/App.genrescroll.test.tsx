import { screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { openHome } from "./test/render";

// phase6-spec 6-3: 選んだジャンルのボタンが、横に流れる列の端で見切れない。
// 選択が変わったとき、新しく選ばれたボタンを scrollIntoView で列の中に入れる(初回表示では呼ばない)。

type ScrollCall = { el: Element; arg: unknown };

/** jsdom には scrollIntoView が無いので足す。ジャンルのボタンに対する呼び出しだけ記録する。 */
function stubScrollIntoView(): ScrollCall[] {
  const calls: ScrollCall[] = [];
  Object.defineProperty(Element.prototype, "scrollIntoView", {
    configurable: true,
    writable: true,
    value: vi.fn(function (this: Element, arg?: unknown) {
      calls.push({ el: this, arg });
    }),
  });
  return calls;
}

/** prefers-reduced-motion: reduce が効いているかを差し替える(jsdom に matchMedia は無い)。 */
function stubReducedMotion(reduce: boolean) {
  vi.stubGlobal(
    "matchMedia",
    vi.fn((query: string) => ({
      matches: reduce && query.includes("prefers-reduced-motion: reduce"),
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  );
}

afterEach(() => {
  Reflect.deleteProperty(Element.prototype, "scrollIntoView");
});

const genreCalls = (calls: ScrollCall[]) => calls.filter((c) => c.el.classList.contains("genre-button"));

describe("ホームのジャンル: 選んだボタンを列の中に入れる", () => {
  it("AC-6-3-1 初回表示では scrollIntoView を呼ばない(読み込みでページが跳ねない)", async () => {
    const calls = stubScrollIntoView();
    stubReducedMotion(false);
    await openHome();
    expect(genreCalls(calls)).toHaveLength(0);
  });

  it("AC-6-3-2 ジャンルを押すと、押したボタンが inline/block nearest・smooth で scrollIntoView される", async () => {
    const calls = stubScrollIntoView();
    stubReducedMotion(false);
    const { user } = await openHome();
    const nav = screen.getByRole("navigation", { name: "ジャンル" });
    const button = within(nav).getByRole("button", { name: "デュエマ" });
    await user.click(button);
    const got = genreCalls(calls);
    expect(got).toHaveLength(1);
    expect(got[0]?.el).toBe(button);
    expect(got[0]?.arg).toEqual({ block: "nearest", inline: "nearest", behavior: "smooth" });
  });

  it("AC-6-3-2 prefers-reduced-motion: reduce のときは behavior が auto", async () => {
    const calls = stubScrollIntoView();
    stubReducedMotion(true);
    const { user } = await openHome();
    const nav = screen.getByRole("navigation", { name: "ジャンル" });
    const button = within(nav).getByRole("button", { name: "S.H.Figuarts" });
    await user.click(button);
    const got = genreCalls(calls);
    expect(got).toHaveLength(1);
    expect(got[0]?.el).toBe(button);
    expect(got[0]?.arg).toEqual({ block: "nearest", inline: "nearest", behavior: "auto" });
  });
});
