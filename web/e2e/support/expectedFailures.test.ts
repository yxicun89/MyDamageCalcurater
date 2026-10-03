// e2e-k3d の失敗収集の許容が、/images/manifest.json の 404 だけに狭く保たれていること。

import { describe, expect, test } from "vitest";
import { isCountedResponseFailure } from "./expectedFailures";

describe("isCountedResponseFailure", () => {
  test("成功・リダイレクトは失敗に数えない", () => {
    expect(isCountedResponseFailure(200, "/api/calc/bulk")).toBe(false);
    expect(isCountedResponseFailure(304, "/static/a.js")).toBe(false);
  });

  test("/images/manifest.json の 404 だけは許容する(画像なし = エンブレム。ADR-0807)", () => {
    expect(isCountedResponseFailure(404, "/images/manifest.json")).toBe(false);
  });

  test.each<[number, string]>([
    [500, "/images/manifest.json"],
    [503, "/images/manifest.json"],
    [403, "/images/manifest.json"],
    [400, "/images/manifest.json"],
    [404, "/images/thumb/0445-000.ab12cd34.webp"],
    [404, "/images/manifest.json.bak"],
    [404, "/images/"],
    [404, "/images/sub/manifest.json"],
    [404, "/api/pokedex/species"],
    [404, "/api/images/manifest.json"],
    [404, "/images/Manifest.json"],
    [500, "/api/calc/bulk"],
  ])("ほかは従来どおり失敗に数える: %i %s", (status, pathname) => {
    expect(isCountedResponseFailure(status, pathname)).toBe(true);
  });
});
