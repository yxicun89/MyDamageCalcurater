// issue 272(ADR-0126・ADR-0311): 特性の選択肢と、防御側・相手側の特性の候補の組み立て(純粋関数)。
//   - selectableAbilities: 種族の特性をスロット順に、マスタで解決できたものだけ返す
//   - defenderAbilityCandidates: おまかせ(null)= 先頭 MAX_ABILITY_CANDIDATES(3)件、個別選択 = その1件
//   - buildBulkRequest / buildReverseRequest: 候補が空・未指定なら defenderAbilities / unknownAbilities を送らない
// 例データの種族は特性が1〜2件なので、3件・4件の境界は src/test/abilityMaster.ts で足す。

import { beforeAll, describe, expect, test } from "vitest";
import { exampleMasterSource } from "../master/exampleSource";
import { abilityMasterFrom, quadAbilities, type AbilityMaster } from "../test/abilityMaster";
import { MAX_ABILITY_CANDIDATES } from "./requestLimits";
import {
  NEUTRAL_NATURE,
  ZERO_SP,
  buildBulkRequest,
  buildIndividual,
  buildReverseRequest,
  defaultAbility,
  defenderAbilityCandidates,
  selectableAbilities,
} from "./requests";
import { firstDamagingMove } from "./moves";

let fixture: AbilityMaster;

beforeAll(async () => {
  fixture = abilityMasterFrom(await exampleMasterSource.load());
});

describe("MAX_ABILITY_CANDIDATES", () => {
  test("engine.MaxAbilityCandidates と同じ 3(ADR-0126 §1。通常特性2つ + 隠れ特性1つ)", () => {
    expect(MAX_ABILITY_CANDIDATES).toBe(3);
  });
});

describe("selectableAbilities", () => {
  test("同じ ID が複数スロットにあっても初出だけ返す(engine は候補の重複を拒否する)", () => {
    const { master, dual } = fixture;
    const [first] = dual.abilities;
    const duplicated = { ...dual, abilities: [...dual.abilities, first ?? ""] };
    const ids = selectableAbilities(duplicated, master.abilities).map((ability) => ability.id);
    expect(ids).toEqual(dual.abilities);
  });

  test("種族の特性をスロット順のまま、マスタの実体で返す(マスタ全体の順ではない)", () => {
    const { master, dual } = fixture;
    const ids = selectableAbilities(dual, master.abilities).map((ability) => ability.id);
    expect(ids).toEqual(dual.abilities);
  });

  test("4件の種族は4件とも返す(切り詰めるのは防御側の候補だけで、選択肢は落とさない)", () => {
    const { master, quad } = fixture;
    expect(selectableAbilities(quad, master.abilities)).toEqual(quadAbilities);
  });

  test("マスタで解決できない特性は黙って捨てる(先頭が解決できなくても残りは返す)", () => {
    const { master, dual } = fixture;
    const partial = master.abilities.filter((ability) => ability.id !== dual.abilities[0]);
    expect(selectableAbilities(dual, partial).map((ability) => ability.id)).toEqual([dual.abilities[1]]);
  });

  test("1件も解決できなければ空(画面は特性セレクトを出さない)", () => {
    const { dual } = fixture;
    expect(selectableAbilities(dual, [])).toEqual([]);
  });

  test("先頭は defaultAbility と同じ(攻撃側の既定は先頭)", () => {
    const { master, dual, quad } = fixture;
    for (const species of [dual, quad]) {
      expect(selectableAbilities(species, master.abilities)[0]).toEqual(
        defaultAbility(species, master.abilities),
      );
    }
  });
});

describe("defenderAbilityCandidates", () => {
  test("おまかせ(null)は種族の特性をスロット順に全部(3件以下のとき)", () => {
    const { master, single, dual } = fixture;
    expect(defenderAbilityCandidates(single, master.abilities, null).map((a) => a.id)).toEqual(
      single.abilities,
    );
    expect(defenderAbilityCandidates(dual, master.abilities, null).map((a) => a.id)).toEqual(dual.abilities);
  });

  test("おまかせは先頭 3 件まで。4件目は落とす(engine の上限を超えて常に失敗するのを防ぐ。ADR-0214)", () => {
    const { master, quad } = fixture;
    const candidates = defenderAbilityCandidates(quad, master.abilities, null);
    expect(candidates).toHaveLength(MAX_ABILITY_CANDIDATES);
    expect(candidates.map((a) => a.id)).toEqual(quad.abilities.slice(0, 3));
  });

  test("個別選択はその1件だけ(4件目も選べる)", () => {
    const { master, quad } = fixture;
    const fourth = quad.abilities[3] ?? "";
    expect(defenderAbilityCandidates(quad, master.abilities, fourth).map((a) => a.id)).toEqual([fourth]);
  });

  test("種族が持たない ID の個別選択は空(古い選択が残っても engine に不正な ID を渡さない)", () => {
    const { master, dual, quad } = fixture;
    expect(defenderAbilityCandidates(dual, master.abilities, quad.abilities[3] ?? "")).toEqual([]);
  });

  test("特性が1つも解決できないときは空(NO_ABILITY を候補に入れない)", () => {
    const { dual } = fixture;
    expect(defenderAbilityCandidates(dual, [], null)).toEqual([]);
  });
});

describe("リクエストへの載せ方", () => {
  function bulkInput() {
    const { master, dual } = fixture;
    const attackerSpecies = master.species[0];
    if (attackerSpecies === undefined) {
      throw new Error("例データに種族が無い");
    }
    const move = firstDamagingMove(attackerSpecies, master.moves);
    if (move === undefined) {
      throw new Error("ダメージ技が無い");
    }
    const attacker = buildIndividual(attackerSpecies, {
      sp: ZERO_SP,
      nature: NEUTRAL_NATURE,
      item: null,
      ability: defaultAbility(attackerSpecies, master.abilities),
    });
    return { attacker, defenderSpecies: dual, move, typeChart: master.typeChart };
  }

  test("buildBulkRequest: defenderAbilities を渡すとそのまま載せる(WASM の calcBulk.defenderAbilities)", () => {
    const { master, dual } = fixture;
    const input = bulkInput();
    const defenderAbilities = selectableAbilities(dual, master.abilities);
    expect(buildBulkRequest({ ...input, defenderAbilities }).defenderAbilities).toEqual(defenderAbilities);
  });

  test("buildBulkRequest: 未指定・空配列なら defenderAbilities を送らない(従来とバイト同じ。境界は未指定を拒否しないが、送らないのが後方互換)", () => {
    const input = bulkInput();
    expect(buildBulkRequest(input)).not.toHaveProperty("defenderAbilities");
    expect(buildBulkRequest({ ...input, defenderAbilities: [] })).not.toHaveProperty("defenderAbilities");
  });

  test("buildReverseRequest: unknownAbilities を渡すとそのまま載せ、未指定・空配列なら送らない", () => {
    const { master, dual } = fixture;
    const { attacker, move, typeChart } = bulkInput();
    const input = {
      side: "defender" as const,
      known: attacker,
      unknownSpecies: dual,
      move,
      typeChart,
      itemCandidates: [null],
      observations: [{ percent: 45 }],
    };
    const unknownAbilities = selectableAbilities(dual, master.abilities);
    expect(buildReverseRequest({ ...input, unknownAbilities }).unknownAbilities).toEqual(unknownAbilities);
    expect(buildReverseRequest(input)).not.toHaveProperty("unknownAbilities");
    expect(buildReverseRequest({ ...input, unknownAbilities: [] })).not.toHaveProperty("unknownAbilities");
  });
});
