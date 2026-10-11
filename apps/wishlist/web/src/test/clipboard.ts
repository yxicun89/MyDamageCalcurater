import { vi } from "vitest";

/** navigator.clipboard.readText を差し替える(jsdom には無い)。後始末は test/setup.ts が navigator.clipboard を消す。 */
export function stubClipboard(impl: () => Promise<string>) {
  const readText = vi.fn(impl);
  Object.defineProperty(navigator, "clipboard", { value: { readText }, configurable: true });
  return readText;
}
