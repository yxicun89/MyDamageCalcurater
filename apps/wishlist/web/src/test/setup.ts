import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach, vi } from "vitest";

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  // stubClipboard(test/clipboard.ts)が足した navigator.clipboard を戻す。
  Reflect.deleteProperty(navigator, "clipboard");
  try {
    localStorage.clear();
  } catch {
    // jsdom では起きない。
  }
});
