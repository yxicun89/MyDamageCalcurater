// P5-5e(ADR-0321): Showdown の取り込み・書き出しに使う「名前引き用のマスタ」を作る非同期関数。テスト先行(実装は web/src/team/showdownMaster.ts)。
// 背景: 実際のマスタは種族の全件一覧を持たない(オンライン・キャッシュ済みオフラインとも capabilities.speciesList が false。
//       特性・技の全件も無い)。parseShowdownTeam / exportShowdownTeam は全件の名前表を前提にするので、
//       画面は MasterSpeciesSearch(searchSpecies / resolveSpecies)で必要な種族だけ引いてマスタに足してから渡す。
//
// 受け入れ条件(AC):
//  M-1 candidateSpeciesNames(text): 各メンバー(空行区切り)の1行目から、種族名の候補(行全体・@ の前・括弧の中・括弧の前)を
//      重複なしで返す。2行目以降(Ability: や - 技)は見ない。
//  M-2 resolveMasterForImport: 一覧のあるマスタ(speciesList が true)はそのまま返し、検索しない。
//  M-3 一覧の無いマスタは、候補ごとに searchSpecies して nameJa が完全一致する種族だけ resolveSpecies し、
//      その種族・特性・技をマスタに足す(元のマスタは書き換えない・同じ種族を二重に足さない)。
//  M-4 前方一致だけの候補(完全一致でない)は resolve しない。検索・解決の失敗は握りつぶし(例外にしない)、その種族は足さない
//      (後段の parse が unresolved_name を出す)。masterSearch が無ければマスタをそのまま返す。
//  M-5 resolveMasterForExport: メンバーの speciesKey がマスタに無いときだけ resolveSpecies で引いて足す(重複しない)。失敗は握りつぶす。
//  M-6 結果のマスタを parseShowdownTeam / exportShowdownTeam に渡すと、一覧の無いマスタでも取り込み・書き出しができる。
// 架空データのみ(例データ+ test/megaMaster.ts)。

import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import { MEGA_FIRE, withMegaFixture } from "../test/megaMaster";
import { createFakeSpeciesSearch, limitedMaster } from "../test/onlineMaster";
import { exportShowdownTeam, parseShowdownTeam } from "./showdownFormat";
import { candidateSpeciesNames, resolveMasterForExport, resolveMasterForImport } from "./showdownMaster";

type TeamMember = components["schemas"]["TeamMember"];

let full: MasterData;
let online: MasterData;

beforeAll(async () => {
  full = withMegaFixture(await exampleMasterSource.load());
  online = limitedMaster(full, { speciesList: false, moves: false, effects: false });
});

function searchOf(master: MasterData) {
  return createFakeSpeciesSearch({
    species: master.species,
    abilities: master.abilities,
    moves: master.moves,
  });
}

const BLOCK = (first: string) =>
  [
    first,
    "Ability: テストむこう",
    "EVs: 2 HP / 32 Atk / 32 Spe",
    "テストいじっぱり Nature",
    "- テストたいあたり",
  ].join("\n");

describe("M-1 candidateSpeciesNames", () => {
  test("1行目だけから、行全体・@ の前・括弧の中・括弧の前を重複なしで返す", () => {
    const text = [
      "テストほのお @ テストぼうぎょだま",
      "Ability: テストむこう",
      "- テストたいあたり",
      "",
      "ニック (テストみず) (F) @ テストとくぼうだま",
      "- テストなきごえ",
    ].join("\n");
    const names = candidateSpeciesNames(text);
    expect(names).toEqual(expect.arrayContaining(["テストほのお", "テストみず", "ニック"]));
    expect(new Set(names).size).toBe(names.length);
    expect(names).not.toContain("テストたいあたり");
    expect(names).not.toContain("テストむこう");
  });

  test("空・空白だけの入力は空配列、候補数は有限(1メンバーあたり多くても8)", () => {
    expect(candidateSpeciesNames("")).toEqual([]);
    expect(candidateSpeciesNames("  \n\n ")).toEqual([]);
    const crazy = `${"(a) ".repeat(50)}@ x`;
    expect(candidateSpeciesNames(crazy).length).toBeLessThanOrEqual(8 + 1);
  });
});

describe("M-2・M-3 resolveMasterForImport", () => {
  test("一覧のあるマスタはそのまま返し、検索しない", async () => {
    const search = searchOf(full);
    const result = await resolveMasterForImport(BLOCK("テストほのお"), full, search);
    expect(result).toBe(full);
    expect(search.searchCalls).toHaveLength(0);
  });

  test("一覧の無いマスタは、完全一致の種族を解決して種族・特性・技を足す(元は不変・二重に足さない)", async () => {
    const search = searchOf(full);
    const before = structuredClone(online);
    const text = `${BLOCK("テストほのお @ テストぼうぎょだま")}\n\n${BLOCK("テストほのお")}`;
    const result = await resolveMasterForImport(text, online, search);
    expect(online).toEqual(before);
    expect(result.species.map((s) => s.key)).toEqual(["9001-000"]);
    expect(result.abilities.map((a) => a.id)).toContain("exampleabilitynone");
    expect(result.moves.map((m) => m.id)).toEqual(
      expect.arrayContaining(["examplemovetackle", "examplemovefirepunch"]),
    );
    expect(result.items).toBe(online.items);
    expect(result.natures).toBe(online.natures);
    expect(search.resolvedKeys.filter((k) => k === "9001-000")).toHaveLength(1);
  });

  test("メガ種族(isMega・requiredItemId)も解決して足す", async () => {
    const result = await resolveMasterForImport(BLOCK("メガテストほのお"), online, searchOf(full));
    expect(result.species.find((s) => s.key === MEGA_FIRE.key)).toMatchObject({ isMega: true });
  });
});

describe("M-4 失敗の扱い", () => {
  test("前方一致だけ(完全一致でない)の候補は resolve しない", async () => {
    const search = searchOf(full);
    const result = await resolveMasterForImport("テスト", online, search);
    expect(search.resolvedKeys).toEqual([]);
    expect(result.species).toEqual([]);
  });

  test("検索・解決が reject しても例外にせず、その種族は足さない", async () => {
    const failing = {
      searchSpecies: () => Promise.reject(new Error("net")),
      resolveSpecies: () => Promise.reject(new Error("net")),
    };
    const result = await resolveMasterForImport(BLOCK("テストほのお"), online, failing);
    expect(result.species).toEqual([]);
    const inner = searchOf(full);
    const half = {
      searchSpecies: (query: string) => inner.searchSpecies(query),
      resolveSpecies: () => Promise.reject(new Error("net")),
    };
    expect((await resolveMasterForImport(BLOCK("テストほのお"), online, half)).species).toEqual([]);
  });

  test("masterSearch が無ければマスタをそのまま返す", async () => {
    expect(await resolveMasterForImport(BLOCK("テストほのお"), online, undefined)).toBe(online);
  });
});

describe("M-5 resolveMasterForExport", () => {
  const member = (speciesKey: string): TeamMember => ({
    speciesKey,
    moveIds: ["examplemovetackle"],
    itemId: null,
    abilityId: "exampleabilitynone",
    natureId: "example-nature-atk",
    sp: { hp: 2, atk: 32, def: 0, spa: 0, spd: 0, spe: 32 },
    teraType: null,
  });

  test("マスタに無い種族だけ resolveSpecies で引く(同じ key は1回)。一覧のあるマスタは引かない", async () => {
    const search = searchOf(full);
    const result = await resolveMasterForExport(
      [member("9001-000"), member("9001-000"), member("9002-000")],
      online,
      search,
    );
    expect([...search.resolvedKeys].sort()).toEqual(["9001-000", "9002-000"]);
    expect(result.species.map((s) => s.key).sort()).toEqual(["9001-000", "9002-000"]);

    const search2 = searchOf(full);
    expect(await resolveMasterForExport([member("9001-000")], full, search2)).toBe(full);
    expect(search2.resolvedKeys).toEqual([]);
  });

  test("解決の失敗は握りつぶす(その種族は足さない)", async () => {
    const failing = {
      searchSpecies: () => Promise.resolve([]),
      resolveSpecies: () => Promise.reject(new Error("x")),
    };
    const result = await resolveMasterForExport([member("9001-000")], online, failing);
    expect(result.species).toEqual([]);
  });
});

describe("M-6 一覧の無いマスタでも取り込み・書き出しができる", () => {
  test("import → parse で種族・特性・技が解決される", async () => {
    const master = await resolveMasterForImport(
      BLOCK("テストほのお @ テストぼうぎょだま"),
      online,
      searchOf(full),
    );
    const parsed = parseShowdownTeam(BLOCK("テストほのお @ テストぼうぎょだま"), master);
    expect(parsed.issues).toEqual([]);
    expect(parsed.members[0]).toMatchObject({
      speciesKey: "9001-000",
      itemId: "exampleitemdef",
      abilityId: "exampleabilitynone",
      moveIds: ["examplemovetackle"],
    });
  });

  test("export で種族名が出る(ID を出さない)", async () => {
    const m: TeamMember = {
      speciesKey: "9001-000",
      moveIds: ["examplemovetackle"],
      itemId: null,
      abilityId: "exampleabilitynone",
      natureId: "example-nature-atk",
      sp: { hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 0 },
      teraType: null,
    };
    const master = await resolveMasterForExport([m], online, searchOf(full));
    const out = exportShowdownTeam([m], master);
    expect(out.text).toContain("テストほのお");
    expect(out.text).not.toContain("9001-000");
    expect(out.issues).toEqual([]);
  });
});
