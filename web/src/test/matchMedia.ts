// テスト専用: window.matchMedia の fake(P4-8「動き」)。jsdom は matchMedia を実装していないため、
// OS の「視差効果を減らす」(prefers-reduced-motion: reduce)の有無をテストから切り替えるのに使う。
// vi.stubGlobal("matchMedia", fakeMatchMedia(true)) のように渡し、afterEach で vi.unstubAllGlobals() する。

/** OS の「視差効果を減らす」の問い合わせ(design.md「動き」)。本体の定数と同じ文字列であることは motion.test.ts で確かめる。 */
export const REDUCED_MOTION_MEDIA_QUERY = "(prefers-reduced-motion: reduce)";

/**
 * reduce が true のときだけ prefers-reduced-motion: reduce に一致する matchMedia。
 * それ以外の問い合わせ(prefers-color-scheme など)には一致しない。問い合わせた文字列は queries に残す。
 */
export function fakeMatchMedia(reduce: boolean): ((query: string) => MediaQueryList) & {
  readonly queries: string[];
} {
  const queries: string[] = [];
  const matchMedia = (query: string): MediaQueryList => {
    queries.push(query);
    return {
      matches: reduce && query === REDUCED_MOTION_MEDIA_QUERY,
      media: query,
      onchange: null,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      addListener: () => undefined,
      removeListener: () => undefined,
      dispatchEvent: () => false,
    };
  };
  return Object.assign(matchMedia, { queries });
}
