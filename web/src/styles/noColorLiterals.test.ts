// P4-1: 色の値は CSS(tokens.css の CSS 変数)にだけ置く。TypeScript 側に色を書かない(ADR-0016 §4)。

import { readFileSync, readdirSync } from "node:fs";
import { localPath } from "../test/localPath";
import { expect, test } from "vitest";

const srcDir = localPath("../", import.meta.url);

function productionSources(): string[] {
  return readdirSync(srcDir, { recursive: true, encoding: "utf8" })
    .map((path) => path.replace(/\\/g, "/"))
    .filter((path) => /\.(ts|tsx)$/.test(path))
    .filter((path) => !/\.test\.(ts|tsx)$/.test(path))
    .filter((path) => !path.startsWith("test/"));
}

test("走査対象に main.tsx が含まれる(走査の前提の確認)", () => {
  expect(productionSources()).toContain("main.tsx");
});

test("web/src の本体コード(テスト以外)に 16 進の色リテラルが無い", () => {
  const hexColor = /#[0-9a-fA-F]{3,8}\b/;
  const offenders = productionSources().flatMap((path) =>
    readFileSync(`${srcDir}${path}`, "utf8")
      .split("\n")
      .flatMap((line, index) => (hexColor.test(line) ? [`${path}:${index + 1}: ${line.trim()}`] : [])),
  );
  expect(offenders).toEqual([]);
});
