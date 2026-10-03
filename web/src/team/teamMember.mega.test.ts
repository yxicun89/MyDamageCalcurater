// issue #515・ADR-0320 PR-B: 構築メンバーのメガ種族の持ち物の整合(純粋関数)。画面なしで境界を固定する。
//   AC-M1 changeSpecies: メガへ → 持ち物はストーン / メガから非メガ → null / 非メガどうし → 持ち物を保つ /
//         ストーンを引けないメガ → null。種族の変更のほかの規則(特性・技)は変わらない
//   AC-M2 correctMegaItem(古い保存データの補正): メガ種族に requiredItemId 以外(null 含む)→ ストーンへ直し通知の種類を返す。
//         すでにストーン・非メガ・種族未解決は変えない(非メガに持たせたストーンも直さない)。ストーンが引けない(missing)は
//         持ち物を空にして通知する(すでに空なら通知しない)。他の欄は変えない
// 架空のマスタ(例データ + test/megaMaster.ts)だけを使う。

import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { selectableAbilities } from "../domain/requests";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import {
  MEGA_FIRE,
  MEGA_FIRE_STONE,
  MEGA_ORPHAN,
  MEGA_WATER,
  MEGA_WATER_STONE,
  withMegaFixture,
} from "../test/megaMaster";
import { changeSpecies, correctMegaItem, memberToDraft } from "./teamMember";

type Schemas = components["schemas"];

let master: MasterData;
let normal: MasterSpecies;
let otherNormal: MasterSpecies;

beforeAll(async () => {
  master = withMegaFixture(await exampleMasterSource.load());
  const plain = master.species.filter((species) => species.isMega !== true);
  const [first, second] = plain;
  if (first === undefined || second === undefined) {
    throw new Error("例データに非メガの種族が2件無い");
  }
  normal = first;
  otherNormal = second;
});

function member(speciesKey: string, itemId: string | null): Schemas["TeamMember"] {
  return {
    speciesKey,
    moveIds: [],
    itemId,
    abilityId: null,
    natureId: "example-nature-atk",
    sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
    teraType: null,
  };
}

function change(
  draftItemId: string | null,
  previous: MasterSpecies | null,
  next: MasterSpecies,
): string | null {
  const draft = { ...memberToDraft(member(previous?.key ?? normal.key, draftItemId)) };
  return changeSpecies(draft, next, selectableAbilities(next, master.abilities), {
    previous,
    items: master.items,
  }).itemId;
}

describe("AC-M1 種族を変えたときの持ち物(changeSpecies)", () => {
  test("非メガ → メガ: 持ち物はストーンになる(元の持ち物は置き換わる)", () => {
    expect(change("exampleitemdef", normal, MEGA_FIRE)).toBe(MEGA_FIRE_STONE.id);
    expect(change(null, normal, MEGA_WATER)).toBe(MEGA_WATER_STONE.id);
  });

  test("メガ → 別のメガ: 新しいストーンになる", () => {
    expect(change(MEGA_FIRE_STONE.id, MEGA_FIRE, MEGA_WATER)).toBe(MEGA_WATER_STONE.id);
  });

  test("メガ → 非メガ: 持ち物は null(ストーンを残さない)", () => {
    expect(change(MEGA_FIRE_STONE.id, MEGA_FIRE, normal)).toBeNull();
  });

  test("非メガ → 非メガ: 持ち物を保つ。未選択(previous = null)から非メガでも保つ", () => {
    expect(change("exampleitemdef", normal, otherNormal)).toBe("exampleitemdef");
    expect(change("exampleitemdef", null, normal)).toBe("exampleitemdef");
  });

  test("ストーンを引けないメガ: 持ち物は null(別の持ち物を残さない)", () => {
    expect(change("exampleitemdef", normal, MEGA_ORPHAN)).toBeNull();
  });

  test("持ち物以外(特性・技・性格・SP)の規則は変わらない。options を省くと持ち物は従来どおり保つ", () => {
    const draft = memberToDraft({ ...member(normal.key, "exampleitemdef"), moveIds: ["examplemovetackle"] });
    const withOptions = changeSpecies(draft, MEGA_FIRE, selectableAbilities(MEGA_FIRE, master.abilities), {
      previous: normal,
      items: master.items,
    });
    const without = changeSpecies(draft, MEGA_FIRE, selectableAbilities(MEGA_FIRE, master.abilities));
    expect({ ...withOptions, itemId: null }).toEqual({ ...without, itemId: null });
    expect(without.itemId).toBe("exampleitemdef");
  });
});

describe("AC-M2 古い保存データの補正(correctMegaItem)", () => {
  function correct(species: MasterSpecies | null, itemId: string | null) {
    return correctMegaItem(memberToDraft(member(species?.key ?? normal.key, itemId)), species, master.items);
  }

  test("メガ種族に別の持ち物: ストーンへ直し、fixed(直した先のストーン)を返す", () => {
    const result = correct(MEGA_FIRE, "exampleitemdef");
    expect(result.draft.itemId).toBe(MEGA_FIRE_STONE.id);
    expect(result.correction).toEqual({ kind: "fixed", item: MEGA_FIRE_STONE });
  });

  test("メガ種族の持ち物が null(持ち物なし)でもストーンへ直す", () => {
    const result = correct(MEGA_WATER, null);
    expect(result.draft.itemId).toBe(MEGA_WATER_STONE.id);
    expect(result.correction).toEqual({ kind: "fixed", item: MEGA_WATER_STONE });
  });

  test("メガ種族に別のメガのストーンを持たせていた場合も直す", () => {
    const result = correct(MEGA_FIRE, MEGA_WATER_STONE.id);
    expect(result.draft.itemId).toBe(MEGA_FIRE_STONE.id);
    expect(result.correction?.kind).toBe("fixed");
  });

  test("すでにストーンなら変えない(correction は null。draft は同じ値)", () => {
    const input = memberToDraft(member(MEGA_FIRE.key, MEGA_FIRE_STONE.id));
    const result = correctMegaItem(input, MEGA_FIRE, master.items);
    expect(result.draft).toEqual(input);
    expect(result.correction).toBeNull();
  });

  test("非メガは変えない(非メガに持たせたメガストーンも直さない)", () => {
    for (const itemId of ["exampleitemdef", null, MEGA_FIRE_STONE.id]) {
      const input = memberToDraft(member(normal.key, itemId));
      const result = correctMegaItem(input, normal, master.items);
      expect(result.draft).toEqual(input);
      expect(result.correction).toBeNull();
    }
  });

  test("種族が引けていない(null)ときは変えない", () => {
    const input = memberToDraft(member("9999-000", "exampleitemdef"));
    const result = correctMegaItem(input, null, master.items);
    expect(result.draft).toEqual(input);
    expect(result.correction).toBeNull();
  });

  test("ストーンをマスタから引けないメガ: 別の持ち物は空にし cleared を返す。すでに空なら通知しない", () => {
    const cleared = correct(MEGA_ORPHAN, "exampleitemdef");
    expect(cleared.draft.itemId).toBeNull();
    expect(cleared.correction).toEqual({ kind: "cleared" });

    const already = correct(MEGA_ORPHAN, null);
    expect(already.draft.itemId).toBeNull();
    expect(already.correction).toBeNull();
  });

  test("持ち物以外の欄(性格・SP・技・特性・ニックネーム)は変えない", () => {
    const base = memberToDraft({
      ...member(MEGA_FIRE.key, "exampleitemdef"),
      nickname: "愛称",
      teraType: "fire",
    });
    const result = correctMegaItem(base, MEGA_FIRE, master.items);
    expect({ ...result.draft, itemId: null }).toEqual({ ...base, itemId: null });
  });
});
