// issue #515・ADR-0320(docs/mega-evolution-spec.md §4-3): メガシンカの持ち物固定の共通ドメイン(純粋関数)。
// 計算・逆算(PR-A)と、構築の編集・判定(PR-B)が同じ関数を使う。名前・ID の直書きはせず、
// マスタの isMega / requiredItemId だけから導く。
// 確かめること:
//   - isMegaSpecies: isMega が true の種族だけ(省略・null は false)
//   - megaStoneItemIds: いずれかのメガ種族の requiredItemId に現れる ID の集合
//   - selectableItems: その集合の持ち物を単独の選択肢から外す(順序・実体は保つ)
//   - megaItemLock: none(メガでない)/ locked(メガ+マスタにストーンあり)/ missing(メガだがストーンを引けない)
//   - itemIdAfterSpeciesChange: 種族を変えたときの持ち物 ID(メガ→固定、メガ→非メガは未選択に戻す)

import { describe, expect, test } from "vitest";
import type { Item } from "../engine/types";
import type { MasterSpecies } from "../master/types";
import {
  MEGA_FIRE,
  MEGA_FIRE_STONE,
  MEGA_ORPHAN,
  MEGA_WATER,
  MEGA_WATER_STONE,
  MISSING_STONE_ID,
} from "../test/megaMaster";
import { exampleSpecies } from "../master/example/species";
import { exampleItems } from "../master/example/items";
import {
  itemIdAfterSpeciesChange,
  isMegaSpecies,
  megaItemLock,
  megaStoneItemIds,
  selectableItems,
} from "./mega";

function required<T>(value: T | undefined, what: string): T {
  if (value === undefined) {
    throw new Error(`例データに ${what} が無い`);
  }
  return value;
}

const normal: MasterSpecies = required(exampleSpecies[0], "種族 0");
const normalItem: Item = required(exampleItems[0], "持ち物 0");
const allItems: readonly Item[] = [...exampleItems, MEGA_FIRE_STONE, MEGA_WATER_STONE];

describe("isMegaSpecies", () => {
  test("isMega が true の種族だけ true", () => {
    expect(isMegaSpecies(MEGA_FIRE)).toBe(true);
    expect(isMegaSpecies({ ...normal, isMega: false, requiredItemId: null })).toBe(false);
  });

  test("isMega を持たない種族・null は false(公開 API・古いキャッシュが返さない間の互換)", () => {
    expect(isMegaSpecies(normal)).toBe(false);
    expect(isMegaSpecies(null)).toBe(false);
  });
});

describe("megaStoneItemIds", () => {
  test("メガ種族の requiredItemId を集める(重複は1つ。基本種・null は入らない)", () => {
    const ids = megaStoneItemIds([normal, MEGA_FIRE, MEGA_WATER, { ...MEGA_FIRE, key: "9101-002", form: 2 }]);
    expect([...ids].sort()).toEqual([MEGA_FIRE_STONE.id, MEGA_WATER_STONE.id].sort());
  });

  test("isMega が false の種族の requiredItemId は数えない", () => {
    const ids = megaStoneItemIds([{ ...normal, isMega: false, requiredItemId: "examplestray" }]);
    expect(ids.size).toBe(0);
  });

  test("isMega が true でも requiredItemId が null・省略なら入れない", () => {
    expect(
      megaStoneItemIds([
        { ...MEGA_FIRE, requiredItemId: null },
        { ...MEGA_WATER, requiredItemId: undefined },
      ]).size,
    ).toBe(0);
  });

  test("マスタの持ち物に無い ID も、メガ種族が要求していれば集合に入る(持ち物の有無は見ない)", () => {
    expect(megaStoneItemIds([MEGA_ORPHAN]).has(MISSING_STONE_ID)).toBe(true);
  });

  test("種族が空なら空集合", () => {
    expect(megaStoneItemIds([]).size).toBe(0);
  });
});

describe("selectableItems", () => {
  test("メガストーンの集合に入る持ち物を外し、残りは順序と実体(参照)を保つ", () => {
    const stoneIds = megaStoneItemIds([MEGA_FIRE, MEGA_WATER]);
    const selectable = selectableItems(allItems, stoneIds);
    expect(selectable).toEqual(exampleItems);
    selectable.forEach((item, index) => {
      expect(item).toBe(exampleItems[index]);
    });
  });

  test("集合が空なら全件(例データだけのマスタは今までどおり)", () => {
    expect(selectableItems(exampleItems, new Set())).toEqual(exampleItems);
  });
});

describe("megaItemLock", () => {
  test("メガでない種族・未選択(null)は none", () => {
    expect(megaItemLock(normal, allItems)).toEqual({ kind: "none" });
    expect(megaItemLock(null, allItems)).toEqual({ kind: "none" });
  });

  test("メガ種族でストーンがマスタにあれば locked(持ち物の実体をそのまま返す)", () => {
    const lock = megaItemLock(MEGA_FIRE, allItems);
    expect(lock.kind).toBe("locked");
    if (lock.kind === "locked") {
      expect(lock.item).toBe(MEGA_FIRE_STONE);
    }
  });

  test("メガ種族でもストーンがマスタの持ち物に無ければ missing(黙って別の持ち物にしない)", () => {
    expect(megaItemLock(MEGA_ORPHAN, allItems)).toEqual({ kind: "missing" });
  });

  test("メガ種族で requiredItemId が null・省略なら missing", () => {
    expect(megaItemLock({ ...MEGA_FIRE, requiredItemId: null }, allItems)).toEqual({ kind: "missing" });
    expect(megaItemLock({ ...MEGA_FIRE, requiredItemId: undefined }, allItems)).toEqual({ kind: "missing" });
  });
});

describe("itemIdAfterSpeciesChange", () => {
  const change = (
    previous: MasterSpecies | null,
    next: MasterSpecies | null,
    currentItemId: string,
  ): string => itemIdAfterSpeciesChange({ previous, next, items: allItems, currentItemId });

  test("メガ種族を選ぶと、直前の持ち物にかかわらずメガストーンの ID になる", () => {
    expect(change(null, MEGA_FIRE, "")).toBe(MEGA_FIRE_STONE.id);
    expect(change(normal, MEGA_FIRE, normalItem.id)).toBe(MEGA_FIRE_STONE.id);
  });

  test("メガ種族から別のメガ種族へ変えると、新しいメガストーンの ID になる", () => {
    expect(change(MEGA_FIRE, MEGA_WATER, MEGA_FIRE_STONE.id)).toBe(MEGA_WATER_STONE.id);
  });

  test("メガ種族からメガでない種族(または未選択)へ変えると未選択(空文字)に戻る。メガストーンを残さない", () => {
    expect(change(MEGA_FIRE, normal, MEGA_FIRE_STONE.id)).toBe("");
    expect(change(MEGA_FIRE, null, MEGA_FIRE_STONE.id)).toBe("");
  });

  test("メガでない種族どうし(・未選択からの選択)では、選んでいる持ち物を保つ", () => {
    const other = required(exampleSpecies[1], "種族 1");
    expect(change(normal, other, normalItem.id)).toBe(normalItem.id);
    expect(change(null, normal, normalItem.id)).toBe(normalItem.id);
  });

  test("ストーンを引けないメガ種族(missing)は空文字(固定せず空にする)", () => {
    expect(change(normal, MEGA_ORPHAN, normalItem.id)).toBe("");
  });

  test("ストーンを引けないメガ種族からメガでない種族へ変えても空文字", () => {
    expect(change(MEGA_ORPHAN, normal, "")).toBe("");
  });
});
