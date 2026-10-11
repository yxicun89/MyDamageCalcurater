// ADR-0350: 構築の技の選択肢(MovePicker に渡す Move の一覧)。
import { describe, expect, test } from "vitest";
import type { Move } from "../engine/types";
import type { MasterSpecies } from "../master/types";
import { moveOptions } from "./teamMemberOptions";

const POOL: readonly Move[] = [
  { id: "m-hit", nameJa: "ひっとう", type: "normal", category: "physical", power: 40, priority: 0 },
  { id: "m-growl", nameJa: "なきごえ", type: "normal", category: "status", power: 0, priority: 0 },
  { id: "m-other", nameJa: "よそのわざ", type: "fire", category: "special", power: 80, priority: 0 },
];
const SPECIES = { learnset: ["m-hit", "m-growl", "m-missing"] } as unknown as MasterSpecies;

describe("moveOptions", () => {
  test("learnset の技を Move のまま(タイプ・分類・威力つき)返し、変化技も含む。マスタに無い ID は除く", () => {
    expect(moveOptions(SPECIES, POOL, null).map((move) => move.id)).toEqual(["m-hit", "m-growl"]);
    expect(moveOptions(SPECIES, POOL, null)[1]?.category).toBe("status");
  });

  test("現在値が learnset 外でも pool にあれば、その Move を足す", () => {
    expect(moveOptions(SPECIES, POOL, "m-other").at(-1)).toEqual(POOL[2]);
  });

  test("現在値が pool にも無ければ、ID を名前にしたタイプ無し・変化技の代役を足す(壊さず開ける)", () => {
    expect(moveOptions(SPECIES, POOL, "gone").at(-1)).toMatchObject({
      id: "gone",
      nameJa: "gone",
      type: "",
      category: "status",
    });
  });

  test("種族が無いときは現在値だけ", () => {
    expect(moveOptions(null, POOL, "m-hit").map((move) => move.id)).toEqual(["m-hit"]);
  });
});
