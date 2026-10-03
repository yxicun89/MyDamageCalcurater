// P5-5e(ADR-0321): parse の結果(ParseResult)から「作成する構築のメンバー」を決める純粋関数。テスト先行
// (実装は web/src/team/showdownImportPlan.ts)。
//
// planShowdownImport(result: ParseResult, master: MasterData): ImportPlan
//   ImportPlan = { members: TeamMember[]; issues: ShowdownIssue[]; notes: ImportNote[]; canCreate: boolean }
//   ImportNote = { kind: "mega_item_fixed" | "mega_item_unavailable"; memberIndex: number; speciesKey: string }
//   (memberIndex は plan.members の添字。落とされたメンバーがあると parse の issue の memberIndex〈入力の何体目か〉とはずれる)
//
// 受け入れ条件(AC):
//  P-1 parse の issues はそのまま(並び・内容を変えず)plan.issues に載せる。members は最大6(parse の上限に従い、plan では切らない)。
//  P-2 canCreate は「members が1体以上」。0体(空入力・全て落ちた)は false。severity が warning のみでも members があれば true。
//      error の issue があっても、取り込める members が残っていれば true(取り込める分だけ作る)。
//  P-3 メガ種族(master.species の isMega)は megaItemLock で持ち物を requiredItemId に直す。別の持ち物・null だったときは
//      notes に mega_item_fixed。すでにストーンなら変えず note も出さない。
//  P-4 メガ種族でストーンがマスタに無い(megaItemLock が missing)ときは itemId を null にして mega_item_unavailable(黙って壊さない)。
//  P-5 非メガ種族は持ち物を変えない(メガストーンを持たせていても直さない。ADR-0320 AC-5 と同じ)。
//  P-6 種族がマスタに無い(未解決の key)は変えない。入力の members を書き換えない(新しい配列・オブジェクトを返す)。
//  P-7 members の他の項目(nickname・技・SP・性格・特性・テラス)は変えない。

import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import {
  MEGA_FIRE,
  MEGA_FIRE_STONE,
  MEGA_ORPHAN,
  MEGA_WATER,
  MEGA_WATER_STONE,
  withMegaFixture,
} from "../test/megaMaster";
import type { ParseResult, ShowdownIssue } from "./showdownFormat";
import { planShowdownImport } from "./showdownImportPlan";

type TeamMember = components["schemas"]["TeamMember"];

let master: MasterData;
beforeAll(async () => {
  master = withMegaFixture(await exampleMasterSource.load());
});

function member(speciesKey: string, itemId: string | null, extra: Partial<TeamMember> = {}): TeamMember {
  return {
    speciesKey,
    nickname: "ニック",
    moveIds: ["examplemovetackle", "examplemovefirepunch"],
    itemId,
    abilityId: "exampleabilitynone",
    natureId: "example-nature-atk",
    sp: { hp: 2, atk: 32, def: 0, spa: 0, spd: 0, spe: 32 },
    teraType: "fire",
    ...extra,
  };
}

const warn: ShowdownIssue = {
  severity: "warning",
  code: "level_not_50",
  memberIndex: 0,
  field: "level",
  value: "100",
};
const err: ShowdownIssue = {
  severity: "error",
  code: "unresolved_name",
  memberIndex: 1,
  field: "species",
  value: "ふめい",
};

describe("P-1・P-2 issues と canCreate", () => {
  test("issues はそのまま、warning だけでも members があれば canCreate", () => {
    const result: ParseResult = { members: [member("9001-000", "exampleitemdef")], issues: [warn] };
    const plan = planShowdownImport(result, master);
    expect(plan.issues).toEqual([warn]);
    expect(plan.members).toHaveLength(1);
    expect(plan.canCreate).toBe(true);
    expect(plan.notes).toEqual([]);
  });

  test("error があっても取り込める members が残れば canCreate(その分だけ作る)", () => {
    const plan = planShowdownImport({ members: [member("9001-000", null)], issues: [err] }, master);
    expect(plan.canCreate).toBe(true);
    expect(plan.issues).toEqual([err]);
  });

  test("members が0体は canCreate=false(issues は保つ)", () => {
    const empty: ShowdownIssue = { severity: "error", code: "empty_input", memberIndex: null, field: null };
    const plan = planShowdownImport({ members: [], issues: [empty] }, master);
    expect(plan.canCreate).toBe(false);
    expect(plan.members).toEqual([]);
    expect(plan.issues).toEqual([empty]);
  });
});

describe("P-3〜P-5 メガの持ち物補正", () => {
  test("メガ種族に別の持ち物・null → ストーンに直し mega_item_fixed。すでにストーンなら変えず note なし", () => {
    const plan = planShowdownImport(
      {
        members: [
          member(MEGA_FIRE.key, "exampleitemdef"),
          member(MEGA_WATER.key, null),
          member(MEGA_FIRE.key, MEGA_FIRE_STONE.id),
        ],
        issues: [],
      },
      master,
    );
    expect(plan.members.map((m) => m.itemId)).toEqual([
      MEGA_FIRE_STONE.id,
      MEGA_WATER_STONE.id,
      MEGA_FIRE_STONE.id,
    ]);
    expect(plan.notes).toEqual([
      { kind: "mega_item_fixed", memberIndex: 0, speciesKey: MEGA_FIRE.key },
      { kind: "mega_item_fixed", memberIndex: 1, speciesKey: MEGA_WATER.key },
    ]);
  });

  test("ストーンがマスタに無いメガは itemId を null にして mega_item_unavailable", () => {
    const plan = planShowdownImport(
      { members: [member(MEGA_ORPHAN.key, "exampleitemdef")], issues: [] },
      master,
    );
    expect(plan.members[0]?.itemId).toBeNull();
    expect(plan.notes).toEqual([
      { kind: "mega_item_unavailable", memberIndex: 0, speciesKey: MEGA_ORPHAN.key },
    ]);
  });

  test("非メガは持ち物を変えない(メガストーンでも)", () => {
    const plan = planShowdownImport(
      { members: [member("9001-000", MEGA_FIRE_STONE.id)], issues: [] },
      master,
    );
    expect(plan.members[0]?.itemId).toBe(MEGA_FIRE_STONE.id);
    expect(plan.notes).toEqual([]);
  });
});

describe("P-6・P-7 純粋さと他項目の保持", () => {
  test("未解決の種族 key は変えず、入力を書き換えない。他の項目は保つ", () => {
    const input = [member("0000-000", "exampleitemdef"), member(MEGA_FIRE.key, "exampleitemdef")];
    const snapshot = structuredClone(input);
    const plan = planShowdownImport({ members: input, issues: [] }, master);
    expect(input).toEqual(snapshot);
    expect(plan.members[0]).toEqual(snapshot[0]);
    expect(plan.members[1]).toEqual({ ...snapshot[1], itemId: MEGA_FIRE_STONE.id });
    expect(plan.members).not.toBe(input);
  });
});
