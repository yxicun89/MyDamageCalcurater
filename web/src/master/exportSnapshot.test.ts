// P4-5: Web の例データを calc-svc のマスタのスナップショット(services/calc/README.md の暫定スキーマ
// schemaVersion 1)に書き出す(ADR-0301 §5)。calc-svc をその出力で起動すれば、Web の例データの ID が
// そのまま API に通る(オンラインの動作確認・P4-6 の E2E)。
// 形の規則: calc-svc のローダーは未知のフィールドをエラーにするので、learnset(画面だけの追加フィールド)と
// typeChart(CALC_TYPECHART_PATH で別に渡す)は書かない。

import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { beforeAll, describe, expect, test } from "vitest";
import { localPath } from "../test/localPath";
import { exampleMasterSource } from "./exampleSource";
import { toCalcSnapshot } from "./exportSnapshot";
import type { MasterData } from "./types";

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

function sortedKeys(value: object): string[] {
  return Object.keys(value).sort();
}

describe("toCalcSnapshot(services/calc/README.md のスキーマ schemaVersion 1)", () => {
  test("最上位は schemaVersion 1 と species・moves・items・abilities・natures だけ(typeChart は書かない)", () => {
    const snapshot = toCalcSnapshot(master);
    expect(snapshot.schemaVersion).toBe(1);
    expect(sortedKeys(snapshot)).toEqual(
      ["abilities", "items", "moves", "natures", "schemaVersion", "species"].sort(),
    );
  });

  test("種族は key・dexNo・form・nameJa・types・baseStats・abilities だけ(learnset は書かない)", () => {
    const snapshot = toCalcSnapshot(master);
    expect(snapshot.species).toHaveLength(master.species.length);
    snapshot.species.forEach((species, index) => {
      const source = master.species[index];
      expect(sortedKeys(species)).toEqual(
        ["abilities", "baseStats", "dexNo", "form", "key", "nameJa", "types"].sort(),
      );
      expect(species).toEqual({
        key: source?.key,
        dexNo: source?.dexNo,
        form: source?.form,
        nameJa: source?.nameJa,
        types: source?.types,
        baseStats: source?.baseStats,
        abilities: source?.abilities,
      });
    });
  });

  test("技は id・nameJa・type・category・power・priority", () => {
    const snapshot = toCalcSnapshot(master);
    expect(snapshot.moves).toEqual(master.moves);
    for (const move of snapshot.moves) {
      expect(sortedKeys(move)).toEqual(["category", "id", "nameJa", "power", "priority", "type"].sort());
    }
  });

  test.each(["items", "abilities"] as const)(
    "%s は id・nameJa・effect(効果は DTO の形のまま、無ければ null)",
    (name) => {
      const snapshot = toCalcSnapshot(master);
      expect(snapshot[name]).toEqual(master[name]);
      for (const entry of snapshot[name]) {
        expect(sortedKeys(entry)).toEqual(["effect", "id", "nameJa"].sort());
      }
    },
  );

  test("性格は id・nameJa・plus・minus(無補正は plus・minus とも null)", () => {
    const snapshot = toCalcSnapshot(master);
    expect(snapshot.natures).toEqual(master.natures);
    for (const nature of snapshot.natures) {
      expect(sortedKeys(nature)).toEqual(["id", "minus", "nameJa", "plus"].sort());
    }
  });

  test("例データのすべての ID が同じ順で書き出され、JSON にしても変わらない", () => {
    const snapshot = toCalcSnapshot(master);
    expect(snapshot.species.map((entry) => entry.key)).toEqual(master.species.map((entry) => entry.key));
    expect(snapshot.moves.map((entry) => entry.id)).toEqual(master.moves.map((entry) => entry.id));
    expect(snapshot.items.map((entry) => entry.id)).toEqual(master.items.map((entry) => entry.id));
    expect(snapshot.abilities.map((entry) => entry.id)).toEqual(master.abilities.map((entry) => entry.id));
    expect(snapshot.natures.map((entry) => entry.id)).toEqual(master.natures.map((entry) => entry.id));
    expect(JSON.parse(JSON.stringify(snapshot))).toEqual(snapshot);
  });

  test("入力のマスタを書き換えない", () => {
    const before = JSON.stringify(master);
    toCalcSnapshot(master);
    expect(JSON.stringify(master)).toBe(before);
  });
});

describe("web/scripts/export-example-master.mjs", () => {
  test("引数のパスに toCalcSnapshot(例データ) の JSON を書く", () => {
    const webRoot = localPath("../../", import.meta.url);
    const outDir = mkdtempSync(join(tmpdir(), "pokecalc-export-"));
    const outPath = join(outDir, "master.json");
    try {
      execFileSync(process.execPath, [join(webRoot, "scripts", "export-example-master.mjs"), outPath], {
        cwd: webRoot,
        stdio: "pipe",
        timeout: 60_000,
      });
      const written = JSON.parse(readFileSync(outPath, "utf8")) as unknown;
      expect(written).toEqual(toCalcSnapshot(master));
    } finally {
      rmSync(outDir, { recursive: true, force: true });
    }
  }, 90_000);
});
