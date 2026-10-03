// issue 515・ADR-0324: 種族の名前検索の規則(pokedex-svc の SearchSpecies と同じ結果にする)。
// 規則: nameJa が q で始まる、または、メガ種族で nameJa が「メガ + q」で始まる。前後の空白は呼び出し側で除く。
// 確かめること(架空の名前): メガの名前・「メガ」・基本種名(基本種とメガの両方)・非メガは「メガ + q」で当たらない・並び。

import { describe, expect, test } from "vitest";
import { MEGA_FIRE, MEGA_WATER } from "../test/megaMaster";
import type { MasterSpecies } from "./types";
import { exampleSpecies } from "./example/species";
import { MEGA_NAME_PREFIX, matchesSpeciesName, searchSpeciesByName } from "./speciesNameMatch";

function exampleSpeciesByKey(key: string): MasterSpecies {
  const found = exampleSpecies.find((species) => species.key === key);
  if (found === undefined) {
    throw new Error(`例データに ${key} が無い`);
  }
  return found;
}

const BASE_FIRE = exampleSpeciesByKey("9001-000");
const BASE_WATER = exampleSpeciesByKey("9002-000");
const population = [MEGA_WATER, MEGA_FIRE, ...exampleSpecies];

describe("matchesSpeciesName", () => {
  test("接頭辞は『メガ』", () => {
    expect(MEGA_NAME_PREFIX).toBe("メガ");
  });

  test.each([
    ["メガの名前そのもの", MEGA_FIRE, "メガテストほのお", true],
    ["「メガ」だけ", MEGA_FIRE, "メガ", true],
    ["基本種名(メガを除いた名前)でメガも当たる", MEGA_FIRE, "テストほのお", true],
    ["基本種名の途中まででも当たる", MEGA_FIRE, "テスト", true],
    ["別の基本種名では当たらない", MEGA_FIRE, "テストみず", false],
    ["メガの名前の途中(前方でない)は当たらない", MEGA_FIRE, "ガテスト", false],
  ])("メガ種族: %s", (_name, species, query, want) => {
    expect(matchesSpeciesName(species, query)).toBe(want);
  });

  test("メガでない種族は、名前の前方一致だけで当たる(『メガ + q』では当たらない)", () => {
    expect(matchesSpeciesName(BASE_FIRE, "テストほのお")).toBe(true);
    expect(matchesSpeciesName(BASE_FIRE, "メガ")).toBe(false);
    expect(matchesSpeciesName(BASE_FIRE, "メガテストほのお")).toBe(false);
  });

  test("空の q は全件", () => {
    expect(matchesSpeciesName(MEGA_FIRE, "")).toBe(true);
    expect(matchesSpeciesName(BASE_FIRE, "")).toBe(true);
  });
});

describe("searchSpeciesByName", () => {
  test("メガの名前で検索するとメガ種族が返る", () => {
    expect(searchSpeciesByName(population, "メガテストほのお").map((s) => s.key)).toEqual([MEGA_FIRE.key]);
  });

  test("「メガ」で全メガ種族が返る(図鑑番号・フォーム番号の昇順)", () => {
    expect(searchSpeciesByName(population, "メガ").map((s) => s.key)).toEqual([
      MEGA_FIRE.key,
      MEGA_WATER.key,
    ]);
  });

  test("基本種名で、基本種とメガの両方が返る(基本種が先)", () => {
    expect(searchSpeciesByName(population, "テストみず").map((s) => s.key)).toEqual([
      BASE_WATER.key,
      MEGA_WATER.key,
    ]);
  });

  test("一致なしは空", () => {
    expect(searchSpeciesByName(population, "メガテストくさくさ")).toEqual([]);
  });
});
