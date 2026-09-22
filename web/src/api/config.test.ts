// P4-5: API の基点 URL(ADR-0301 §4)。環境変数 VITE_API_BASE_URL を1か所で読む。既定は同じオリジン "/"。
// 末尾のスラッシュは1つに揃え、`${apiBaseUrl()}api/calc` の形で連結できるようにする。

import { afterEach, describe, expect, test, vi } from "vitest";
import { apiBaseUrl } from "./config";

afterEach(() => {
  vi.unstubAllEnvs();
});

describe("apiBaseUrl", () => {
  test("未設定なら同じオリジンの /", () => {
    vi.stubEnv("VITE_API_BASE_URL", undefined);
    expect(apiBaseUrl()).toBe("/");
  });

  test.each([
    ["", "/"],
    ["  ", "/"],
    ["/", "/"],
    ["http://localhost:8080", "http://localhost:8080/"],
    ["http://localhost:8080/", "http://localhost:8080/"],
    ["http://localhost:8080//", "http://localhost:8080/"],
    ["https://example.test/pokecalc", "https://example.test/pokecalc/"],
    [" https://example.test/pokecalc/ ", "https://example.test/pokecalc/"],
  ])("VITE_API_BASE_URL=%j は %j", (value, expected) => {
    vi.stubEnv("VITE_API_BASE_URL", value);
    expect(apiBaseUrl()).toBe(expected);
  });
});
