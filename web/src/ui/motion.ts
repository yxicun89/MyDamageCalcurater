// P4-8: 演出の共通部品(docs/design.md「動き」、CLAUDE.md ドメイン規約「常時動くアニメーションを入れない。
// 演出は操作時のみ」)。演出を始めるかどうかは、操作(クリック・結果の到着・ポインタ移動)のたびに
// prefersReducedMotion() で決める(結果をキャッシュせず、毎回 OS に問い合わせ直す)。

/** OS の「視差効果を減らす」設定の問い合わせ文字列(styles/tokens.css の reduced-motion ブロックと同じ値)。 */
export const REDUCED_MOTION_QUERY = "(prefers-reduced-motion: reduce)";

/**
 * OS が「視差効果を減らす」設定かどうか。matchMedia が無い環境(jsdom・古いブラウザ)では、
 * 演出を諦める側(false 以外)に倒さず false を返し、例外も投げない。
 * 呼ぶたびに問い合わせ直す: 途中で OS の設定が変わっても、次の操作からその設定に従うため。
 */
export function prefersReducedMotion(): boolean {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return false;
  }
  return window.matchMedia(REDUCED_MOTION_QUERY).matches;
}
