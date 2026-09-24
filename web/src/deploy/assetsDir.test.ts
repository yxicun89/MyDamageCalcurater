// @vitest-environment node
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, test } from "vitest";
import { baseConfig } from "../../vite.config";

// gateway が Web に転送しない先頭セグメント(services/gateway/internal/httpapi/routing.go の reservedFirstSegments)。
const routingGo = fileURLToPath(
  new URL("../../../services/gateway/internal/httpapi/routing.go", import.meta.url),
);

function gatewayReservedSegments(): string[] {
  const src = readFileSync(routingGo, "utf8");
  const block = /reservedFirstSegments\s*=\s*map\[string\]bool\{([^}]*)\}/.exec(src);
  expect(block, "routing.go に reservedFirstSegments が見つからない").not.toBeNull();
  return [...(block?.[1] ?? "").matchAll(/"([^"]+)"\s*:\s*true/g)].map((m) => m[1] ?? "");
}

describe("ビルド成果物の置き場所(issue #268)", () => {
  test("gateway の予約語を読み取れる", () => {
    expect(gatewayReservedSegments()).toContain("assets");
  });

  test("assetsDir の先頭セグメントが gateway の予約語と衝突しない(衝突すると :8080 で白画面)", () => {
    const assetsDir = baseConfig.build?.assetsDir ?? "assets";
    const first = assetsDir.replace(/^\/+/, "").split("/")[0];
    expect(gatewayReservedSegments()).not.toContain(first);
  });
});
