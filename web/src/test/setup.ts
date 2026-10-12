import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

afterEach(() => {
  cleanup();
  // ADR-0358: 直近の計算入力(localStorage)がテスト間で復元されないよう、1 件ごとに消す(node 環境では何もしない)。
  try {
    (globalThis as { localStorage?: Storage }).localStorage?.clear();
  } catch {
    // ストレージが使えない環境では何もしない。
  }
});
