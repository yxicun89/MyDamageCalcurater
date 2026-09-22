// P4-8: 演出の共通部品(web/src/ui/motion.ts)。design.md「動き」: OS の「視差効果を減らす」で全演出を無効化する。
// 演出を始めるかどうかは、操作(クリック・結果の到着・ポインタ移動)のたびに prefersReducedMotion() で決める。

import { afterEach, describe, expect, test, vi } from "vitest";
import { REDUCED_MOTION_MEDIA_QUERY, fakeMatchMedia } from "../test/matchMedia";
import { REDUCED_MOTION_QUERY, prefersReducedMotion } from "./motion";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("prefersReducedMotion", () => {
  test("問い合わせは (prefers-reduced-motion: reduce)", () => {
    expect(REDUCED_MOTION_QUERY).toBe(REDUCED_MOTION_MEDIA_QUERY);
  });

  test("OS が「視差効果を減らす」なら true", () => {
    const matchMedia = fakeMatchMedia(true);
    vi.stubGlobal("matchMedia", matchMedia);
    expect(prefersReducedMotion()).toBe(true);
    expect(matchMedia.queries).toContain(REDUCED_MOTION_MEDIA_QUERY);
  });

  test("OS が「視差効果を減らす」でなければ false", () => {
    vi.stubGlobal("matchMedia", fakeMatchMedia(false));
    expect(prefersReducedMotion()).toBe(false);
  });

  test("matchMedia が無い環境(jsdom・古いブラウザ)では false を返し、例外を投げない", () => {
    vi.stubGlobal("matchMedia", undefined);
    expect(prefersReducedMotion()).toBe(false);
  });

  test("毎回問い合わせ直す(途中で OS の設定を変えても次の操作から従う)", () => {
    vi.stubGlobal("matchMedia", fakeMatchMedia(false));
    expect(prefersReducedMotion()).toBe(false);
    vi.stubGlobal("matchMedia", fakeMatchMedia(true));
    expect(prefersReducedMotion()).toBe(true);
  });
});
