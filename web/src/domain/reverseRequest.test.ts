// P4-4: 逆算リクエストの組み立て(純粋関数。ADR-0011 §3 calcReverse、ADR-0010 §R、ADR-0300 §7)。
// 境界は未知のフィールドを拒否する(unknown_field)ので、画面のための learnset を渡さない。
// maxCandidates は渡さない(engine は 2 × 持ち物候補数を全部返す。候補は高々十数件で切る必要がない)。

import { describe, expect, test } from "vitest";
import type { Individual, Item, Move, Observation, TypeChart } from "../engine/types";
import type { MasterSpecies } from "../master/types";
import { NEUTRAL_NATURE, NO_ABILITY, ZERO_SP, buildReverseRequest, toEngineSpecies } from "./requests";

// 境界の reverseRequest が受け付けるフィールド(engine/wasmapi/requests.go の json タグ。契約から書いた期待値)。
const reverseRequestFields = [
  "critical",
  "field",
  "format",
  "itemCandidates",
  "known",
  "maxCandidates",
  "move",
  "observations",
  "side",
  "typeChart",
  "unknownSpecies",
];

const typeChart: TypeChart = { types: ["fire", "water"], effectiveness: { fire: { water: 1 } } };

const mine: MasterSpecies = {
  key: "mine",
  dexNo: 9101,
  form: 0,
  nameJa: "じぶん",
  types: ["fire"],
  baseStats: { hp: 80, atk: 100, def: 70, spa: 90, spd: 70, spe: 95 },
  abilities: [],
  learnset: ["m1"],
};

const theirs: MasterSpecies = { ...mine, key: "theirs", dexNo: 9102, nameJa: "あいて", learnset: ["m2"] };

const move: Move = { id: "m1", nameJa: "わざ", type: "fire", category: "physical", power: 80, priority: 0 };

const known: Individual = {
  species: toEngineSpecies(mine),
  level: 50,
  nature: NEUTRAL_NATURE,
  ability: NO_ABILITY,
  item: null,
  sp: ZERO_SP,
};

const berry: Item = { id: "berry", nameJa: "きのみ", effect: { resistBerryType: "fire" } };

describe("buildReverseRequest", () => {
  test("side・known・unknownSpecies(learnset なし)・技・相性表・持ち物候補・観測をそのまま入れる", () => {
    const observations: Observation[] = [{ percent: 45 }, { percent: 52 }];
    const request = buildReverseRequest({
      side: "defender",
      known,
      unknownSpecies: theirs,
      move,
      typeChart,
      itemCandidates: [null, berry],
      observations,
    });
    expect(request).toEqual({
      format: "single",
      side: "defender",
      known,
      unknownSpecies: toEngineSpecies(theirs),
      move,
      typeChart,
      itemCandidates: [null, berry],
      observations,
    });
    expect(request.unknownSpecies).not.toHaveProperty("learnset");
    expect(request.itemCandidates?.[1]).toBe(berry);
  });

  test("maxCandidates は渡さない(engine が全候補を返す)", () => {
    const request = buildReverseRequest({
      side: "attacker",
      known,
      unknownSpecies: theirs,
      move,
      typeChart,
      itemCandidates: [null],
      observations: [{ damage: 60 }],
    });
    expect(request).not.toHaveProperty("maxCandidates");
    expect(request.side).toBe("attacker");
    expect(request.observations).toEqual([{ damage: 60 }]);
  });

  test("境界が受け付けるフィールドだけを持つ", () => {
    const request = buildReverseRequest({
      side: "defender",
      known,
      unknownSpecies: theirs,
      move,
      typeChart,
      itemCandidates: [null],
      observations: [{ percent: 1 }],
    });
    for (const key of Object.keys(request)) {
      expect(reverseRequestFields).toContain(key);
    }
  });
});
