// ADR-0326(ADR-0175 §4 のクライアント規約): 持ち物を役割で絞る共通ドメイン(domain/itemRoles.ts)。
// 確かめること:
//   - itemsForRole: attacker / defender / either / any の各欄で、役割に合う持ち物だけをマスタの順のまま返す
//   - メガストーンはどの欄にも出ない(isMegaStone、無ければ種族から導いた stoneIds で判別)。any(構築)でも出ない
//   - roles が無い持ち物(古いサーバー・古いキャッシュ・例データ)は役割で絞らない(効果から再導出しない)
//   - 「持ち物なし」は含めない(欄が先頭に足す)
//   - itemAfterRoleChange: 役割が変わったとき、合わない持ち物は未選択に戻し外した持ち物を返す
//   - megaStoneLabel: ストーンの nameJa が日本語ならそのまま、使えなければ「{基本種名}のメガストーン」、基本種名も無ければ「メガストーン」(ADR-0328)
//   - toEngineItem: roles・isMegaStone を落として engine の Item の形にする(境界は未知のフィールドを拒否する)
//   - 画面ごとの役割: 逆算の自分は観測した側の反対、判定・調整は either、構築は any

import { describe, expect, test } from "vitest";
import type { Item } from "../engine/types";
import type { MasterItem, MasterSpecies } from "../master/types";
import {
  BERRY_ITEM,
  BOTH_ITEM,
  DEF_ITEM,
  EXPECTED_ANY_ITEMS,
  EXPECTED_ATTACKER_ITEMS,
  EXPECTED_DEFENDER_ITEMS,
  EXPECTED_EITHER_ITEMS,
  NO_ROLE_ITEM,
  POWER_ITEM,
  ROLE_ITEMS,
  ROLE_MEGA_FIRE_STONE,
  UNRESOLVED_MEGA_STONE,
  withoutRoleFields,
} from "../test/itemRolesMaster";
import { MEGA_FIRE, MEGA_FIRE_STONE, MEGA_FIRE_STONE_LABEL, MEGA_WATER_STONE } from "../test/megaMaster";
import {
  ADJUST_ITEM_ROLE_FILTER,
  JUDGE_ITEM_ROLE_FILTER,
  TEAM_ITEM_ROLE_FILTER,
  isMegaStoneItem,
  itemAfterRoleChange,
  itemMatchesRole,
  itemsForRole,
  megaStoneLabel,
  reverseMyItemRole,
  toEngineItem,
  type ItemRoleFilter,
} from "./itemRoles";

const ids = (items: readonly MasterItem[]): string[] => items.map((item) => item.id);

/** baseSpeciesNameJa のキーを持たない種族(古いサーバーの応答)。 */
function withoutBaseSpeciesNameJa(species: MasterSpecies): MasterSpecies {
  const copy: { -readonly [K in keyof MasterSpecies]?: MasterSpecies[K] } = { ...species };
  delete copy.baseSpeciesNameJa;
  return copy as MasterSpecies;
}

describe("itemsForRole: 役割に合う持ち物だけをマスタの順のまま返す", () => {
  test.each<[ItemRoleFilter, readonly MasterItem[]]>([
    ["attacker", EXPECTED_ATTACKER_ITEMS],
    ["defender", EXPECTED_DEFENDER_ITEMS],
    ["either", EXPECTED_EITHER_ITEMS],
    ["any", EXPECTED_ANY_ITEMS],
  ])("%s の欄", (filter, expected) => {
    expect(ids(itemsForRole(ROLE_ITEMS, filter))).toEqual(ids(expected));
  });

  test("実体を保つ(写し替えない。要求に載せる前に toEngineItem で形を戻す)", () => {
    const result = itemsForRole(ROLE_ITEMS, "attacker");
    expect(result[0]).toBe(POWER_ITEM);
  });

  test("役割の無い持ち物(roles が空。回復のきのみの類)は攻撃・防御・either の欄に出ず、any(構築)には出る", () => {
    for (const filter of ["attacker", "defender", "either"] as const) {
      expect(ids(itemsForRole(ROLE_ITEMS, filter))).not.toContain(NO_ROLE_ITEM.id);
    }
    expect(ids(itemsForRole(ROLE_ITEMS, "any"))).toContain(NO_ROLE_ITEM.id);
  });

  test("両方の役割を持つ持ち物は攻撃側・防御側の両方の欄に出る", () => {
    expect(ids(itemsForRole(ROLE_ITEMS, "attacker"))).toContain(BOTH_ITEM.id);
    expect(ids(itemsForRole(ROLE_ITEMS, "defender"))).toContain(BOTH_ITEM.id);
  });

  test("空の入力は空", () => {
    expect(itemsForRole([], "attacker")).toEqual([]);
  });
});

describe("メガストーンはどの欄にも出ない", () => {
  test.each<ItemRoleFilter>(["attacker", "defender", "either", "any"])("%s の欄", (filter) => {
    const result = ids(itemsForRole(ROLE_ITEMS, filter));
    expect(result).not.toContain(ROLE_MEGA_FIRE_STONE.id);
    // まだ解決していないメガ種族のストーン(種族から導く集合では判別できない)も isMegaStone で外れる。
    expect(result).not.toContain(UNRESOLVED_MEGA_STONE.id);
  });

  test("roles に役割があっても isMegaStone が true なら外す(any でも)", () => {
    const odd: MasterItem = { ...POWER_ITEM, id: "examplemegastoneodd", isMegaStone: true };
    for (const filter of ["attacker", "either", "any"] as const) {
      expect(ids(itemsForRole([odd, POWER_ITEM], filter))).toEqual([POWER_ITEM.id]);
    }
  });

  test("isMegaStone が無い持ち物(古いサーバー・例データ)は、種族から導いた stoneIds で外す", () => {
    const withRoles: readonly MasterItem[] = [DEF_ITEM, POWER_ITEM, MEGA_FIRE_STONE, MEGA_WATER_STONE];
    const legacy: MasterItem[] = withRoles.map(withoutRoleFields);
    const stoneIds = new Set([MEGA_FIRE_STONE.id, MEGA_WATER_STONE.id]);
    expect(ids(itemsForRole(legacy, "any", stoneIds))).toEqual([DEF_ITEM.id, POWER_ITEM.id]);
    expect(ids(itemsForRole(legacy, "attacker", stoneIds))).toEqual([DEF_ITEM.id, POWER_ITEM.id]);
  });

  test("isMegaStone が false と明示されていれば stoneIds より優先する(サーバーの判定が正)", () => {
    expect(isMegaStoneItem(POWER_ITEM, new Set([POWER_ITEM.id]))).toBe(false);
    expect(isMegaStoneItem(ROLE_MEGA_FIRE_STONE, new Set())).toBe(true);
    const legacyStone: MasterItem = { id: MEGA_FIRE_STONE.id, nameJa: MEGA_FIRE_STONE.nameJa, effect: null };
    expect(isMegaStoneItem(legacyStone, new Set([MEGA_FIRE_STONE.id]))).toBe(true);
    expect(isMegaStoneItem(legacyStone, new Set())).toBe(false);
  });
});

describe("roles が無い持ち物は役割で絞らない(効果から再導出しない)", () => {
  // 効果は攻撃側の補正(damageMod)だが、roles が無いので防御側の欄にも出る(効果を読まないことの確認)。
  const legacyPower: MasterItem = {
    id: "exampleitempower",
    nameJa: "テストちからのたま",
    effect: { damageMod: 5324 },
  };
  const legacyNone: MasterItem = { id: "exampleitemheal", nameJa: "テストかいふくのみ", effect: null };

  test.each<ItemRoleFilter>(["attacker", "defender", "either", "any"])("%s の欄でも全件のまま", (filter) => {
    expect(ids(itemsForRole([legacyPower, legacyNone], filter))).toEqual([legacyPower.id, legacyNone.id]);
  });

  test("itemMatchesRole は roles が無ければ常に true", () => {
    expect(itemMatchesRole(legacyPower, "defender")).toBe(true);
    expect(itemMatchesRole(legacyNone, "attacker")).toBe(true);
  });

  test("roles が空配列なのは「役割が無い」で、「分からない」ではない(絞る)", () => {
    expect(itemMatchesRole(NO_ROLE_ITEM, "attacker")).toBe(false);
    expect(itemMatchesRole(NO_ROLE_ITEM, "either")).toBe(false);
    expect(itemMatchesRole(NO_ROLE_ITEM, "any")).toBe(true);
  });
});

describe("itemMatchesRole のテーブル", () => {
  test.each<[MasterItem, ItemRoleFilter, boolean]>([
    [POWER_ITEM, "attacker", true],
    [POWER_ITEM, "defender", false],
    [POWER_ITEM, "either", true],
    [DEF_ITEM, "attacker", false],
    [DEF_ITEM, "defender", true],
    [BERRY_ITEM, "defender", true],
    [BOTH_ITEM, "attacker", true],
    [BOTH_ITEM, "defender", true],
    [NO_ROLE_ITEM, "defender", false],
    [NO_ROLE_ITEM, "any", true],
  ])("%o を %s の欄に: %s", (item, filter, expected) => {
    expect(itemMatchesRole(item, filter)).toBe(expected);
  });
});

describe("itemAfterRoleChange: 欄の役割が変わったときの持ち物", () => {
  test("合う持ち物はそのまま(外さない)", () => {
    expect(itemAfterRoleChange({ items: ROLE_ITEMS, role: "defender", currentItemId: BOTH_ITEM.id })).toEqual(
      {
        itemId: BOTH_ITEM.id,
        dropped: null,
      },
    );
  });

  test("合わない持ち物は未選択に戻し、外した持ち物を返す", () => {
    expect(
      itemAfterRoleChange({ items: ROLE_ITEMS, role: "defender", currentItemId: POWER_ITEM.id }),
    ).toEqual({
      itemId: "",
      dropped: POWER_ITEM,
    });
    expect(itemAfterRoleChange({ items: ROLE_ITEMS, role: "attacker", currentItemId: DEF_ITEM.id })).toEqual({
      itemId: "",
      dropped: DEF_ITEM,
    });
  });

  test("未選択は未選択のまま(通知しない)", () => {
    expect(itemAfterRoleChange({ items: ROLE_ITEMS, role: "attacker", currentItemId: "" })).toEqual({
      itemId: "",
      dropped: null,
    });
  });

  test("roles が無い持ち物は外さない(役割が分からないので絞らない)", () => {
    const legacy: MasterItem = {
      id: "exampleitempower",
      nameJa: "テストちからのたま",
      effect: { damageMod: 5324 },
    };
    expect(itemAfterRoleChange({ items: [legacy], role: "defender", currentItemId: legacy.id })).toEqual({
      itemId: legacy.id,
      dropped: null,
    });
  });

  test("メガストーン(固定で持っていた)は外さず、そのまま返す(固定は種族から毎回導く)", () => {
    expect(
      itemAfterRoleChange({ items: ROLE_ITEMS, role: "attacker", currentItemId: ROLE_MEGA_FIRE_STONE.id }),
    ).toEqual({ itemId: ROLE_MEGA_FIRE_STONE.id, dropped: null });
  });
});

describe("megaStoneLabel: 固定中の表示", () => {
  test("ストーンの nameJa が日本語ならそのまま、英語名などのときは「{基本種名}のメガストーン」", () => {
    expect(megaStoneLabel(MEGA_FIRE, MEGA_FIRE_STONE.nameJa)).toBe(MEGA_FIRE_STONE_LABEL);
    expect(megaStoneLabel(MEGA_FIRE, "Examplite F")).toBe(`${MEGA_FIRE.baseSpeciesNameJa}専用のメガストーン`);
  });

  test.each<[string, MasterSpecies]>([
    ["null", { ...MEGA_FIRE, baseSpeciesNameJa: null }],
    ["省略", withoutBaseSpeciesNameJa(MEGA_FIRE)],
    ["空白だけ", { ...MEGA_FIRE, baseSpeciesNameJa: "  " }],
  ])("基本種名が %s なら「メガストーン」だけ(名前を推測しない)", (_label, species) => {
    expect(megaStoneLabel(species)).toBe("メガストーン");
  });

  test("ユーザー報告の例: ルカリオのメガストーン", () => {
    expect(megaStoneLabel({ ...MEGA_FIRE, baseSpeciesNameJa: "ルカリオ" })).toBe(
      "ルカリオ専用のメガストーン",
    );
  });
});

describe("toEngineItem: engine の Item の形に戻す", () => {
  test("roles・isMegaStone を落とし、id・nameJa・effect だけにする", () => {
    const engineItem: Item = toEngineItem(BOTH_ITEM);
    expect(Object.keys(engineItem).sort()).toEqual(["effect", "id", "nameJa"]);
    expect(engineItem).toEqual({ id: BOTH_ITEM.id, nameJa: BOTH_ITEM.nameJa, effect: BOTH_ITEM.effect });
  });

  test("項目の無い持ち物(例データ)は同じ実体を返す(既存の要求の組み立ての挙動を変えない)", () => {
    expect(toEngineItem(MEGA_FIRE_STONE)).toBe(MEGA_FIRE_STONE);
  });
});

describe("画面ごとの欄の役割", () => {
  test("逆算の自分: 与えたダメージ(side defender)なら攻撃側、受けたダメージ(side attacker)なら防御側", () => {
    expect(reverseMyItemRole("defender")).toBe("attacker");
    expect(reverseMyItemRole("attacker")).toBe("defender");
  });

  test("判定・調整は攻撃・防御のどちらか(either)、構築は絞らない(any)", () => {
    expect(JUDGE_ITEM_ROLE_FILTER).toBe("either");
    expect(ADJUST_ITEM_ROLE_FILTER).toBe("either");
    expect(TEAM_ITEM_ROLE_FILTER).toBe("any");
  });
});
