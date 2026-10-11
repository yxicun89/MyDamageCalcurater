// I-web-8 = F-09(ADR-0333): お気に入りに「そのときの計算の入力」(calc = API の CalcRequest)を保存し、
// 一覧から選んだら計算画面に戻すための純粋関数。
// 確かめること(ADR-0333 の受け入れ条件):
//   AC-1 保存の形: 画面の状態 → CalcRequest。既定の条件はキーごと省く(conditionRequestParts と同じ考え方)/
//        防御側は「種族の無振り個体」(SP 0・無補正の性格)に、防御側の持ち物・特性(おまかせは省く)・B/D ランクだけを載せる /
//        入力が揃っていない・攻撃側の入力が不正・性格を決められないときは null(calc を付けない)
//   AC-2 往復: 状態 → calc → 状態 が一致する(サーバーが既定値を補った形〈Favorite.calc〉から戻しても一致する)。
//        性格が一意に決まらない入力(A特化だけ等)は、表示は変わりうるが「もう一度保存した calc」は一致する
//   AC-3 戻せない項目: マスタ・レギュレーションの変更で引けない種族・技・持ち物・特性・性格は、空(既定)に戻して
//        issues に報告する(引けた部分は戻す)。Web の画面で表せない項目(ダブル・H/B/D/S の SP・テラス・やけど以外の状態異常・
//        攻撃側の壁・防御側の育成)は反映せず、ignored として報告する
//   AC-4 メガ: 攻撃側・防御側がメガ種族なら持ち物は固定(ADR-0320)。保存された持ち物が固定と違えば固定を採り、megaItem で報告する
//   AC-5 旧お気に入り(calc なし): individual から攻撃側だけを戻す
//   AC-6 作成本文: label は「攻撃側→防御側(技)」(揃っていなければ攻撃側の名前。30 コードポイントで切る)/
//        calc があれば individual は calc.attacker と同じ / 保存内容は 4096 バイト以内(超えるなら calc を付けない)
// 架空のデータだけを使う(ADR-0002)。

import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { DEFAULT_ATTACKER_STAT_INPUTS, type AttackerStatInputs } from "../domain/attackerStatInputs";
import { DEFAULT_CALC_CONDITIONS, type CalcConditions } from "../domain/calcConditions";
import { megaStoneItemIds } from "../domain/mega";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import { MEGA_FIRE, MEGA_FIRE_STONE, withMegaFixture } from "../test/megaMaster";
import { MAX_FAVORITE_LABEL_LENGTH, favoriteInputOf } from "./favoriteInput";
import {
  MAX_FAVORITE_SAVED_BYTES,
  favoriteCalcOf,
  favoriteLabelOf,
  restoreFavoriteAttacker,
  restoreFavoriteCalc,
  type FavoriteCalcState,
  type FavoriteRestoreLookup,
} from "./favoriteCalc";

type Schemas = components["schemas"];
type CalcRequest = Schemas["CalcRequest"];

const ZERO = { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 } as const;
/** 例データの無補正の性格のうち ID の昇順で最初(resolveNatureId の規則)。 */
const NEUTRAL_ID = "example-nature-neutral-docile";

let master: MasterData;
let megaMaster: MasterData;
beforeAll(async () => {
  master = await exampleMasterSource.load();
  megaMaster = withMegaFixture(master);
});

function lookupFor(
  data: MasterData,
  calc: Pick<CalcRequest, "attacker" | "defender">,
): FavoriteRestoreLookup {
  return {
    attackerSpecies: data.species.find((species) => species.key === calc.attacker.speciesKey) ?? null,
    defenderSpecies: data.species.find((species) => species.key === calc.defender.speciesKey) ?? null,
    moves: data.moves,
    items: data.items,
    abilities: data.abilities,
    natures: data.natures,
  };
}

function stateOf(overrides: Partial<FavoriteCalcState>): FavoriteCalcState {
  return {
    attackerKey: "9001-000",
    defenderKey: "9002-000",
    moveId: "examplemovetackle",
    attackerItemId: "",
    defenderItemId: "",
    attackerAbilityId: "exampleabilitynone",
    defenderAbilityId: "",
    attackerStatInputs: DEFAULT_ATTACKER_STAT_INPUTS,
    conditions: DEFAULT_CALC_CONDITIONS,
    ...overrides,
  };
}

function calcOf(state: FavoriteCalcState, category: "physical" | "special" = "physical"): CalcRequest {
  const calc = favoriteCalcOf({ state, natures: master.natures, moveCategory: category });
  if (calc === null) {
    throw new Error("calc を作れなかった");
  }
  return calc;
}

/** サーバーが保存時に補う既定値(api/openapi.yaml の Favorite.calc の説明。ADR-0228)。 */
function withServerDefaults(calc: CalcRequest): CalcRequest {
  const individual = (value: Schemas["Individual"]): Schemas["Individual"] => ({
    ...value,
    level: 50,
    ranks: { atk: 0, def: 0, spa: 0, spd: 0, spe: 0, ...value.ranks },
    status: value.status ?? "none",
  });
  const noScreens = { reflect: false, lightScreen: false, auroraVeil: false };
  return {
    format: calc.format,
    attacker: individual(calc.attacker),
    defender: individual(calc.defender),
    moveId: calc.moveId,
    field: {
      weather: calc.field?.weather ?? "none",
      terrain: calc.field?.terrain ?? "none",
      attackerScreens: { ...noScreens, ...calc.field?.attackerScreens },
      defenderScreens: { ...noScreens, ...calc.field?.defenderScreens },
    },
    options: { critical: calc.options?.critical ?? false },
  };
}

function utf8Bytes(value: unknown): number {
  return new TextEncoder().encode(JSON.stringify(value)).length;
}

const FULL_CONDITIONS: CalcConditions = {
  critical: true,
  burned: true,
  weather: "rain",
  terrain: "psychic",
  defenderScreens: { reflect: false, lightScreen: true, auroraVeil: false },
  ranks: { atk: 0, spa: 2 },
  defenderRanks: { def: 0, spd: -1 },
};

const SPECIAL_UP: AttackerStatInputs = {
  atk: { spText: "0", modifier: "down" },
  spa: { spText: "32", modifier: "up" },
};

/** 条件をすべて既定から変えた状態(特殊技・特性・持ち物・防御側の特性の個別選択)。 */
const FULL_STATE = (): FavoriteCalcState =>
  stateOf({
    attackerKey: "9004-000",
    defenderKey: "9005-000",
    moveId: "examplemovethunder",
    attackerItemId: "exampleitempower",
    defenderItemId: "exampleitemspd",
    attackerAbilityId: "exampleabilityadapt",
    defenderAbilityId: "exampleabilitynone",
    attackerStatInputs: SPECIAL_UP,
    conditions: FULL_CONDITIONS,
  });

describe("AC-1 保存の形(状態 → CalcRequest)", () => {
  test("既定の条件・無振りなら、既定のキー(field・options・ranks・status・itemId・防御側の abilityId)を作らない", () => {
    expect(calcOf(stateOf({}))).toEqual({
      format: "single",
      attacker: {
        speciesKey: "9001-000",
        level: 50,
        natureId: NEUTRAL_ID,
        sp: ZERO,
        abilityId: "exampleabilitynone",
      },
      defender: { speciesKey: "9002-000", level: 50, natureId: NEUTRAL_ID, sp: ZERO },
      moveId: "examplemovetackle",
    });
  });

  test("条件をすべて変えた状態は、攻撃側の A/C ランク・やけど、防御側の B/D ランク・持ち物・特性、場・急所に載る", () => {
    expect(calcOf(FULL_STATE(), "special")).toEqual({
      format: "single",
      attacker: {
        speciesKey: "9004-000",
        level: 50,
        natureId: "example-nature-spa",
        sp: { hp: 0, atk: 0, def: 0, spa: 32, spd: 0, spe: 0 },
        abilityId: "exampleabilityadapt",
        itemId: "exampleitempower",
        ranks: { atk: 0, def: 0, spa: 2, spd: 0, spe: 0 },
        status: "burn",
      },
      defender: {
        speciesKey: "9005-000",
        level: 50,
        natureId: NEUTRAL_ID,
        sp: ZERO,
        abilityId: "exampleabilitynone",
        itemId: "exampleitemspd",
        ranks: { atk: 0, def: 0, spa: 0, spd: -1, spe: 0 },
      },
      moveId: "examplemovethunder",
      field: {
        weather: "rain",
        terrain: "psychic",
        defenderScreens: { reflect: false, lightScreen: true, auroraVeil: false },
      },
      options: { critical: true },
    });
  });

  test('防御側の特性が「おまかせ」("")なら defender.abilityId を作らない', () => {
    const calc = calcOf({ ...FULL_STATE(), defenderAbilityId: "" }, "special");
    expect("abilityId" in calc.defender).toBe(false);
  });

  test("A 上昇・C 下降の物理技は、+A/−C の性格 ID と A・C の SP をそのまま載せる", () => {
    const calc = calcOf(
      stateOf({
        moveId: "examplemovefirepunch",
        attackerStatInputs: { atk: { spText: "20", modifier: "up" }, spa: { spText: "5", modifier: "down" } },
      }),
    );
    expect(calc.attacker.natureId).toBe("example-nature-atk");
    expect(calc.attacker.sp).toEqual({ hp: 0, atk: 20, def: 0, spa: 5, spd: 0, spe: 0 });
  });

  test.each<[string, Partial<FavoriteCalcState>]>([
    ["防御側が未選択", { defenderKey: "" }],
    ["攻撃側が未選択", { attackerKey: "" }],
    ["技が未選択", { moveId: "" }],
    [
      "攻撃の SP が範囲外(33)",
      { attackerStatInputs: { ...DEFAULT_ATTACKER_STAT_INPUTS, atk: { spText: "33", modifier: "neutral" } } },
    ],
    [
      "性格を決められない(A・C とも上昇)",
      { attackerStatInputs: { atk: { spText: "0", modifier: "up" }, spa: { spText: "0", modifier: "up" } } },
    ],
  ])("%s なら null(calc を付けない)", (_name, overrides) => {
    expect(
      favoriteCalcOf({ state: stateOf(overrides), natures: master.natures, moveCategory: "physical" }),
    ).toBeNull();
  });
});

describe("AC-2 往復(状態 → calc → 状態)", () => {
  const cases: ReadonlyArray<[string, () => FavoriteCalcState, "physical" | "special"]> = [
    ["既定の条件・無振り", () => stateOf({}), "physical"],
    ["条件をすべて変えた状態", FULL_STATE, "special"],
    [
      "A 上昇・C 下降・SP 20/5",
      () =>
        stateOf({
          moveId: "examplemovefirepunch",
          attackerItemId: "exampleitemfireberry",
          attackerStatInputs: {
            atk: { spText: "20", modifier: "up" },
            spa: { spText: "5", modifier: "down" },
          },
          conditions: { ...DEFAULT_CALC_CONDITIONS, weather: "sun", ranks: { atk: -6, spa: 0 } },
        }),
      "physical",
    ],
  ];

  test.each(cases)("%s: 保存した calc から戻すと同じ状態になり、issues は空", (_name, state, category) => {
    const calc = calcOf(state(), category);
    const restored = restoreFavoriteCalc(calc, lookupFor(master, calc));
    expect(restored.state).toEqual(state());
    expect(restored.issues).toEqual([]);
  });

  test.each(cases)(
    "%s: サーバーが既定値を補った形(Favorite.calc)から戻しても同じ状態になり、issues は空",
    (_name, state, category) => {
      const calc = withServerDefaults(calcOf(state(), category));
      const restored = restoreFavoriteCalc(calc, lookupFor(master, calc));
      expect(restored.state).toEqual(state());
      expect(restored.issues).toEqual([]);
    },
  );

  test("性格が一意に決まらない入力(A特化・C は補正なし)でも、戻してもう一度保存した calc は最初の calc と一致する", () => {
    const state = stateOf({
      moveId: "examplemovefirepunch",
      attackerStatInputs: {
        atk: { spText: "32", modifier: "up" },
        spa: { spText: "0", modifier: "neutral" },
      },
    });
    const first = calcOf(state);
    const restored = restoreFavoriteCalc(first, lookupFor(master, first));
    expect(restored.issues).toEqual([]);
    expect(calcOf(restored.state)).toEqual(first);
  });
});

describe("AC-2b 対戦の状態(battleState。ADR-0144 §3)の往復", () => {
  test("指定した値だけが calc に入り、戻すと同じ状態になる(省略は入れない)", () => {
    const battleState = { defenderCurrentHp: 50, hits: 3 };
    const calc = calcOf({ ...FULL_STATE(), battleState }, "special");
    expect(calc.battleState).toEqual(battleState);
    const restored = restoreFavoriteCalc(calc, lookupFor(master, calc));
    expect(restored.state.battleState).toEqual(battleState);
    expect(restored.issues).toEqual([]);
  });

  test("状態が無いときは calc にキーを作らず、戻した状態にも無い", () => {
    const calc = calcOf(FULL_STATE(), "special");
    expect(calc).not.toHaveProperty("battleState");
    expect(restoreFavoriteCalc(calc, lookupFor(master, calc)).state).not.toHaveProperty("battleState");
  });

  test("空のオブジェクトは無いものとして戻す", () => {
    const calc = { ...calcOf(FULL_STATE(), "special"), battleState: {} };
    expect(restoreFavoriteCalc(calc, lookupFor(master, calc)).state).not.toHaveProperty("battleState");
  });
});

describe("AC-3 戻せない項目は既定に戻して報告する(引けた部分は戻す)", () => {
  function restoreWith(mutate: (calc: CalcRequest) => CalcRequest, data: () => MasterData = () => master) {
    const calc = mutate(calcOf(FULL_STATE(), "special"));
    return restoreFavoriteCalc(calc, lookupFor(data(), calc));
  }

  test("マスタに無い技: moveId は空、issue {kind: move}。種族・持ち物・条件は戻す", () => {
    const restored = restoreWith((calc) => ({ ...calc, moveId: "examplemovegone" }));
    expect(restored.state.moveId).toBe("");
    expect(restored.state.attackerKey).toBe("9004-000");
    expect(restored.state.defenderKey).toBe("9005-000");
    expect(restored.state.attackerItemId).toBe("exampleitempower");
    expect(restored.state.conditions).toEqual(FULL_CONDITIONS);
    expect(restored.issues).toEqual([{ kind: "move", id: "examplemovegone" }]);
  });

  test("マスタにはあるが攻撃側が覚えない技(レギュレーションの変更)も issue {kind: move}", () => {
    const restored = restoreWith((calc) => ({ ...calc, moveId: "examplemovetackle" }));
    expect(restored.state.moveId).toBe("");
    expect(restored.issues).toEqual([{ kind: "move", id: "examplemovetackle" }]);
  });

  test("攻撃側の種族が引けない: attackerKey・技・攻撃側の特性は空、issue は種族だけ。持ち物・SP・防御側・条件は戻す", () => {
    const restored = restoreWith((calc) => ({
      ...calc,
      attacker: { ...calc.attacker, speciesKey: "9999-000" },
    }));
    expect(restored.state.attackerKey).toBe("");
    expect(restored.state.moveId).toBe("");
    expect(restored.state.attackerAbilityId).toBe("");
    // 持ち物・SP・性格は種族に依存しないので戻す(あとで攻撃側を選び直したときに引き継がれる。ADR-0329 §6)
    expect(restored.state.attackerItemId).toBe("exampleitempower");
    expect(restored.state.attackerStatInputs).toEqual(SPECIAL_UP);
    expect(restored.state.defenderKey).toBe("9005-000");
    expect(restored.state.defenderItemId).toBe("exampleitemspd");
    expect(restored.state.conditions).toEqual(FULL_CONDITIONS);
    expect(restored.issues).toEqual([{ kind: "species", side: "attacker", id: "9999-000" }]);
  });

  test("防御側の種族が引けない: defenderKey・防御側の特性は空、issue は種族だけ。攻撃側・技は戻す", () => {
    const restored = restoreWith((calc) => ({
      ...calc,
      defender: { ...calc.defender, speciesKey: "9998-000" },
    }));
    expect(restored.state.defenderKey).toBe("");
    expect(restored.state.defenderAbilityId).toBe("");
    expect(restored.state.attackerKey).toBe("9004-000");
    expect(restored.state.moveId).toBe("examplemovethunder");
    expect(restored.issues).toEqual([{ kind: "species", side: "defender", id: "9998-000" }]);
  });

  test.each<["attacker" | "defender"]>([["attacker"], ["defender"]])(
    "%s の持ち物が引けない: 持ち物なしにして issue {kind: item}",
    (side) => {
      const restored = restoreWith((calc) => ({
        ...calc,
        [side]: { ...calc[side], itemId: "exampleitemgone" },
      }));
      expect(side === "attacker" ? restored.state.attackerItemId : restored.state.defenderItemId).toBe("");
      expect(restored.issues).toEqual([{ kind: "item", side, id: "exampleitemgone" }]);
    },
  );

  test('攻撃側の特性が種族の特性に無い: 既定("" = 種族の先頭)にして issue {kind: ability}', () => {
    const restored = restoreWith((calc) => ({
      ...calc,
      attacker: { ...calc.attacker, abilityId: "exampleabilitygone" },
    }));
    expect(restored.state.attackerAbilityId).toBe("");
    expect(restored.issues).toEqual([{ kind: "ability", side: "attacker", id: "exampleabilitygone" }]);
  });

  test('防御側の特性が種族の特性に無い: おまかせ("")にして issue {kind: ability}', () => {
    const restored = restoreWith((calc) => ({
      ...calc,
      defender: { ...calc.defender, abilityId: "exampleabilitygone" },
    }));
    expect(restored.state.defenderAbilityId).toBe("");
    expect(restored.issues).toEqual([{ kind: "ability", side: "defender", id: "exampleabilitygone" }]);
  });

  test("攻撃側の性格が引けない: 補正は両方「補正なし」、SP はそのまま戻し、issue {kind: nature}", () => {
    const restored = restoreWith((calc) => ({
      ...calc,
      attacker: { ...calc.attacker, natureId: "example-nature-gone" },
    }));
    expect(restored.state.attackerStatInputs).toEqual({
      atk: { spText: "0", modifier: "neutral" },
      spa: { spText: "32", modifier: "neutral" },
    });
    expect(restored.issues).toEqual([{ kind: "nature", id: "example-nature-gone" }]);
  });

  test.each<[string, (calc: CalcRequest) => CalcRequest, string]>([
    ["ダブル", (calc) => ({ ...calc, format: "double" }), "format"],
    [
      "攻撃側の H/B/D/S の SP",
      (calc) => ({ ...calc, attacker: { ...calc.attacker, sp: { ...calc.attacker.sp, spe: 32 } } }),
      "attackerSp",
    ],
    [
      "攻撃側のテラスタイプ",
      (calc) => ({ ...calc, attacker: { ...calc.attacker, teraType: "fire" } }),
      "attackerTeraType",
    ],
    [
      "やけど以外の状態異常",
      (calc) => ({ ...calc, attacker: { ...calc.attacker, status: "paralysis" } }),
      "attackerStatus",
    ],
    [
      "攻撃側の A/C 以外のランク",
      (calc) => ({
        ...calc,
        attacker: { ...calc.attacker, ranks: { atk: 0, def: 0, spa: 2, spd: 0, spe: 1 } },
      }),
      "attackerRanks",
    ],
    [
      "攻撃側の壁",
      (calc) => ({
        ...calc,
        field: { ...calc.field, attackerScreens: { reflect: true, lightScreen: false, auroraVeil: false } },
      }),
      "attackerScreens",
    ],
    [
      "防御側の SP",
      (calc) => ({ ...calc, defender: { ...calc.defender, sp: { ...ZERO, hp: 32 } } }),
      "defenderBuild",
    ],
    [
      "防御側の性格(補正あり)",
      (calc) => ({ ...calc, defender: { ...calc.defender, natureId: "example-nature-def" } }),
      "defenderBuild",
    ],
  ])("この画面で表せない %s は反映せず、issue {kind: ignored, field: %s}", (_name, mutate, field) => {
    const restored = restoreWith(mutate);
    expect(restored.issues).toEqual([{ kind: "ignored", field }]);
    // 表せる部分はそのまま戻す
    expect(restored.state.attackerKey).toBe("9004-000");
    expect(restored.state.moveId).toBe("examplemovethunder");
  });
});

describe("AC-3b 持ち物は画面の持ち物欄と同じ判定(役割・メガストーン除外)で戻す", () => {
  test("役割に合わない持ち物は戻さず(持ち物なし)、issue {kind: item} を報告する", () => {
    const roleMaster: MasterData = {
      ...master,
      items: master.items.map((item) =>
        item.id === "exampleitempower" ? { ...item, roles: ["defender" as const] } : item,
      ),
    };
    const calc = calcOf(FULL_STATE(), "special");
    const restored = restoreFavoriteCalc(calc, lookupFor(roleMaster, calc));
    expect(restored.state.attackerItemId).toBe("");
    expect(restored.issues).toEqual([{ kind: "item", side: "attacker", id: "exampleitempower" }]);
  });

  test("メガでない種族に保存されたメガストーンは戻さず、issue {kind: item} を報告する", () => {
    const base = calcOf(stateOf({}));
    const calc: CalcRequest = { ...base, attacker: { ...base.attacker, itemId: MEGA_FIRE_STONE.id } };
    // 画面と同じく、メガストーンの判別集合は全種族から導いて渡す
    const restored = restoreFavoriteCalc(calc, {
      ...lookupFor(megaMaster, calc),
      stoneIds: megaStoneItemIds(megaMaster.species),
    });
    expect(restored.state.attackerItemId).toBe("");
    expect(restored.issues).toEqual([{ kind: "item", side: "attacker", id: MEGA_FIRE_STONE.id }]);
  });
});

describe("AC-4 メガ種族の持ち物は固定(ADR-0320)", () => {
  function megaCalc(itemId: string | undefined): CalcRequest {
    const base = calcOf(stateOf({}));
    const attacker = { ...base.attacker, speciesKey: MEGA_FIRE.key };
    return { ...base, attacker: itemId === undefined ? attacker : { ...attacker, itemId } };
  }

  test("保存した持ち物が固定のメガストーンと同じなら、そのまま戻して issue なし", () => {
    const calc = megaCalc(MEGA_FIRE_STONE.id);
    const restored = restoreFavoriteCalc(calc, lookupFor(megaMaster, calc));
    expect(restored.state.attackerItemId).toBe(MEGA_FIRE_STONE.id);
    expect(restored.issues).toEqual([]);
  });

  test.each<[string, string | undefined]>([
    ["別の持ち物", "exampleitempower"],
    ["持ち物なし", undefined],
  ])("保存した持ち物が%sなら、固定のメガストーンを採り issue {kind: megaItem}", (_name, saved) => {
    const calc = megaCalc(saved);
    const restored = restoreFavoriteCalc(calc, lookupFor(megaMaster, calc));
    expect(restored.state.attackerItemId).toBe(MEGA_FIRE_STONE.id);
    expect(restored.issues).toEqual([
      { kind: "megaItem", side: "attacker", savedItemId: saved ?? "", lockedItemId: MEGA_FIRE_STONE.id },
    ]);
  });

  test("メガ種族の攻撃側を保存すると、attacker.itemId は固定のメガストーン", () => {
    const calc = favoriteCalcOf({
      state: stateOf({ attackerKey: MEGA_FIRE.key, attackerItemId: MEGA_FIRE_STONE.id }),
      natures: megaMaster.natures,
      moveCategory: "physical",
    });
    expect(calc?.attacker.itemId).toBe(MEGA_FIRE_STONE.id);
  });
});

describe("AC-5 旧お気に入り(calc なし)は individual から攻撃側だけを戻す", () => {
  test("種族・持ち物・特性・A/C の SP と補正を戻す", () => {
    const individual: Schemas["Individual"] = {
      speciesKey: "9004-000",
      level: 50,
      natureId: "example-nature-spa",
      sp: { hp: 0, atk: 0, def: 0, spa: 32, spd: 0, spe: 0 },
      itemId: "exampleitempower",
      abilityId: "exampleabilityadapt",
    };
    const restored = restoreFavoriteAttacker(individual, {
      ...lookupFor(master, { attacker: individual, defender: individual }),
      defenderSpecies: null,
    });
    expect(restored).toEqual({
      attackerKey: "9004-000",
      attackerItemId: "exampleitempower",
      attackerAbilityId: "exampleabilityadapt",
      attackerStatInputs: SPECIAL_UP,
      issues: [],
    });
  });

  test("種族が引けなければ attackerKey は空で issue {kind: species, side: attacker}", () => {
    const individual: Schemas["Individual"] = {
      speciesKey: "9999-000",
      level: 50,
      natureId: NEUTRAL_ID,
      sp: ZERO,
    };
    const restored = restoreFavoriteAttacker(individual, {
      ...lookupFor(master, { attacker: individual, defender: individual }),
      defenderSpecies: null,
    });
    expect(restored.attackerKey).toBe("");
    expect(restored.issues).toEqual([{ kind: "species", side: "attacker", id: "9999-000" }]);
  });
});

describe("AC-6 作成本文(label・individual・サイズ)", () => {
  test("label は「攻撃側→防御側(技)」", () => {
    expect(
      favoriteLabelOf({
        attackerNameJa: "テストほのお",
        defenderNameJa: "テストみず",
        moveNameJa: "テストかえんパンチ",
      }),
    ).toBe("テストほのお→テストみず(テストかえんパンチ)");
  });

  test("防御側か技が無ければ、label は攻撃側の名前だけ(従来どおり)", () => {
    expect(favoriteLabelOf({ attackerNameJa: "テストほのお", defenderNameJa: null, moveNameJa: null })).toBe(
      "テストほのお",
    );
    expect(
      favoriteLabelOf({ attackerNameJa: "テストほのお", defenderNameJa: "テストみず", moveNameJa: null }),
    ).toBe("テストほのお");
  });

  test("calc を渡すと本文に calc が入り、individual は calc.attacker と同じ。label は 30 コードポイントで切る", () => {
    const calc = calcOf(FULL_STATE(), "special");
    const input = favoriteInputOf({
      label: "あ".repeat(MAX_FAVORITE_LABEL_LENGTH + 5),
      speciesKey: "9004-000",
      natureId: "example-nature-spa",
      sp: calc.attacker.sp,
      itemId: "exampleitempower",
      calc,
    });
    expect(input.calc).toEqual(calc);
    expect(input.individual).toEqual(calc.attacker);
    expect(Array.from(input.label ?? "")).toHaveLength(MAX_FAVORITE_LABEL_LENGTH);
  });

  test("calc が null なら従来の本文({label, individual})のまま", () => {
    const input = favoriteInputOf({
      label: "テストほのお",
      speciesKey: "9001-000",
      natureId: NEUTRAL_ID,
      sp: ZERO,
      itemId: null,
      calc: null,
    });
    expect(Object.keys(input).sort()).toEqual(["individual", "label"]);
  });

  test("保存内容の上限は 4096 バイト(ADR-0228 §3)", () => {
    expect(MAX_FAVORITE_SAVED_BYTES).toBe(4096);
  });

  test("最も大きくなる状態でも、サーバーが既定値を補った保存内容は 2KB 以下(上限に十分収まる)", () => {
    const calc = withServerDefaults(calcOf(FULL_STATE(), "special"));
    const label = "漢".repeat(MAX_FAVORITE_LABEL_LENGTH);
    expect(utf8Bytes({ label, individual: calc.attacker, calc })).toBeLessThanOrEqual(2048);
  });

  test("万一 4096 バイトを超える calc(異常に長い ID)は付けず、従来の本文で保存する(400 で追加を失敗させない)", () => {
    const calc = { ...calcOf(stateOf({})), moveId: "x".repeat(MAX_FAVORITE_SAVED_BYTES) };
    const input = favoriteInputOf({
      label: "テストほのお",
      speciesKey: "9001-000",
      natureId: NEUTRAL_ID,
      sp: ZERO,
      itemId: null,
      calc,
    });
    expect("calc" in input).toBe(false);
    expect(input.individual.speciesKey).toBe("9001-000");
    expect(utf8Bytes(input)).toBeLessThanOrEqual(MAX_FAVORITE_SAVED_BYTES);
  });
});
