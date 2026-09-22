// P4-16: MasterCapabilities の既定と読み出し(ADR-0304 §追記)。
// 確かめること:
//   - capabilities を省いた MasterData は「全部使える」(オフライン相当)とみなす
//     = P4-16 より前に書かれたマスタ(exampleMasterSource・各画面テストの fixture)の挙動が変わらない
//   - capabilities を持つ MasterData はその値をそのまま返す(既定で上書きしない)
//   - isSearchableMasterSource は search を持つ取得口だけを真とする(型の絞り込みも効く)

import { expect, test } from "vitest";
import type { TypeChart } from "../engine/types";
import { FULL_MASTER_CAPABILITIES, isSearchableMasterSource, masterCapabilities } from "./capabilities";
import { exampleMasterSource } from "./exampleSource";
import type {
  MasterCapabilities,
  MasterData,
  MasterSource,
  MasterSpeciesSearch,
  SearchableMasterSource,
} from "./types";

const emptyTypeChart: TypeChart = { types: [], effectiveness: {} };

/** capabilities 以外が空のマスタ(既定の補完だけを確かめるための最小の fixture)。 */
function masterWith(capabilities?: MasterCapabilities): MasterData {
  const base = {
    species: [],
    moves: [],
    items: [],
    abilities: [],
    natures: [],
    typeChart: emptyTypeChart,
  } as const;
  return capabilities === undefined ? base : { ...base, capabilities };
}

test("FULL_MASTER_CAPABILITIES はすべて true(オフライン相当)", () => {
  expect(FULL_MASTER_CAPABILITIES).toEqual({ speciesList: true, moves: true, effects: true });
});

test("capabilities を省いたマスタは「全部使える」とみなす(既存のマスタの挙動を変えない)", () => {
  expect(masterCapabilities(masterWith())).toEqual(FULL_MASTER_CAPABILITIES);
});

test("架空の例データ(オフライン)は全部使えると報告する", async () => {
  const master = await exampleMasterSource.load();
  expect(masterCapabilities(master)).toEqual(FULL_MASTER_CAPABILITIES);
});

test("capabilities を持つマスタはその値をそのまま返す", () => {
  const capabilities: MasterCapabilities = { speciesList: false, moves: false, effects: false };
  expect(masterCapabilities(masterWith(capabilities))).toEqual(capabilities);
});

test("一部だけ false の capabilities も、既定で上書きしない", () => {
  const capabilities: MasterCapabilities = { speciesList: true, moves: false, effects: true };
  expect(masterCapabilities(masterWith(capabilities))).toEqual(capabilities);
});

test("isSearchableMasterSource は search を持たない取得口に false を返す", () => {
  expect(isSearchableMasterSource(exampleMasterSource)).toBe(false);
});

test("isSearchableMasterSource は search を持つ取得口に true を返し、search を型として取り出せる", () => {
  const search: MasterSpeciesSearch = {
    searchSpecies: () => Promise.resolve([]),
    resolveSpecies: () => Promise.reject(new Error("テストでは呼ばない")),
  };
  const source: SearchableMasterSource = { load: () => Promise.resolve(masterWith()), search };
  // MasterSource として受け取ったものを絞り込めること(画面はこの形で使う)。
  const asMasterSource: MasterSource = source;
  expect(isSearchableMasterSource(asMasterSource)).toBe(true);
  if (!isSearchableMasterSource(asMasterSource)) {
    throw new Error("絞り込みに失敗した");
  }
  expect(asMasterSource.search).toBe(search);
});
