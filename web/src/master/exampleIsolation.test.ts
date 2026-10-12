// ADR-0358 段 4: 架空の例データ(master/example。「テスト…」の名前)が利用者の画面に出ないことを守る。
// 例データは単体テストの fixture と、App の既定の取得口(テストが masterSource を渡さないとき)だけ。
// 本番の入口(main.tsx)は使わず、明示の開発用の入口(dev:example 等)も作らない。

import { readFileSync, readdirSync, statSync } from "node:fs";
import path from "node:path";
import { describe, expect, test } from "vitest";
import { exampleAbilities } from "./example/abilities";
import { exampleItems } from "./example/items";
import { exampleMoves } from "./example/moves";
import { exampleSpecies } from "./example/species";
import { localPath } from "../test/localPath";

const srcDir = localPath("../", import.meta.url);
const webDir = localPath("../../", import.meta.url);

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const full = path.join(dir, name);
    return statSync(full).isDirectory() ? sourceFiles(full) : [full];
  });
}

/** テストでも例データでもない、利用者に届く側のソース(.ts / .tsx)。 */
function productionFiles(): string[] {
  return sourceFiles(srcDir).filter((file) => {
    const rel = path.relative(srcDir, file).split(path.sep).join("/");
    return (
      /\.(ts|tsx)$/.test(rel) &&
      !/\.test\.(ts|tsx)$/.test(rel) &&
      !rel.startsWith("test/") &&
      !rel.startsWith("master/example/") &&
      rel !== "master/exampleSource.ts"
    );
  });
}

describe("例データは利用者の画面に出ない", () => {
  test("例データの名前は、画面の文言資源(i18n)に書かれていない", () => {
    const names = [
      ...exampleSpecies.map((entry) => entry.nameJa),
      ...exampleMoves.map((entry) => entry.nameJa),
      ...exampleItems.map((entry) => entry.nameJa),
      ...exampleAbilities.map((entry) => entry.nameJa),
    ].filter((name) => name !== "");
    expect(names.length).toBeGreaterThan(0);
    const i18n = productionFiles().filter((file) => file.includes(`${path.sep}i18n${path.sep}`));
    expect(i18n.length).toBeGreaterThan(0);
    for (const file of i18n) {
      const text = readFileSync(file, "utf8");
      for (const name of names) {
        expect(text.includes(name), `${path.basename(file)} に例データの名前「${name}」がある`).toBe(false);
      }
    }
  });

  test("例データを読み込むのは App.tsx の既定の取得口だけ(それ以外の本番コードは読まない)", () => {
    const importers = productionFiles()
      .filter((file) => /from\s+["'][^"']*(master\/example|exampleSource)/.test(readFileSync(file, "utf8")))
      .map((file) => path.relative(srcDir, file).split(path.sep).join("/"));
    expect(importers).toEqual(["App.tsx"]);
  });

  test("本番の入口 main.tsx は例データを使わず、保存済みマスタの取得口を App に渡す", () => {
    const main = readFileSync(path.join(srcDir, "main.tsx"), "utf8");
    expect(main).not.toMatch(/example/i);
    expect(main).toMatch(/masterSources=/);
  });

  test("例データだけを出す開発用の入口(dev:example)は作っていない", () => {
    const pkg = JSON.parse(readFileSync(path.join(webDir, "package.json"), "utf8")) as {
      scripts: Record<string, string>;
    };
    expect(Object.keys(pkg.scripts).filter((name) => /example/i.test(name))).toEqual([]);
  });
});
