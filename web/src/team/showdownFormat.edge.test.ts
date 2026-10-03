// showdownFormat の追加ケース(ADR-0310 追記。critic 指摘: nickname の記号・括弧入りの名前・同名衝突・入力サイズ上限)。
// 架空の名前だけを使う。

import { describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { exampleMasterSource } from "../master/exampleSource";
import {
  exportShowdownTeam,
  parseShowdownTeam,
  toShowdownMaster,
  type ShowdownMaster,
} from "./showdownFormat";

type TeamMember = components["schemas"]["TeamMember"];
const ZERO = { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 };

const MASTER: ShowdownMaster = {
  species: [
    { key: "9001-000", nameJa: "テストモン", nameEn: "Testmon" },
    { key: "9004-000", nameJa: "Mon (X)", nameEn: "Mon (X)" },
    { key: "9005-000", nameJa: "ダブリ", nameEn: "Dupe" },
    { key: "9006-000", nameJa: "Dupe" },
  ],
  moves: [{ id: "move-a", nameJa: "ためしうち", nameEn: "Trial Strike" }],
  items: [{ id: "item-p", nameJa: "かっこ (小)", nameEn: "Orb (S)" }],
  abilities: [{ id: "abil-p", nameJa: "とくせい (A)", nameEn: "Skill (A)" }],
  natures: [{ id: "nat-a", nameJa: "ためしがち", nameEn: "Testy" }],
};
const base = (over: Partial<TeamMember>): TeamMember => ({
  speciesKey: "9001-000",
  nickname: null,
  moveIds: [],
  itemId: null,
  abilityId: null,
  natureId: "nat-a",
  sp: ZERO,
  teraType: null,
  ...over,
});

describe("nickname に記号を含む", () => {
  test.each(["a@b", "a(b)", "(x)", "a)b(", "x @ y", "@"])("%s は export → parse で戻る", (nick) => {
    for (const itemId of [null, "item-p"]) {
      const m = base({ nickname: nick, itemId });
      const out = exportShowdownTeam([m], MASTER);
      expect(out.issues).toEqual([]);
      const back = parseShowdownTeam(out.text, MASTER);
      expect(back.issues).toEqual([]);
      expect(back.members).toEqual([m]);
    }
  });

  test("性質テスト: 記号入りの nickname を持つ 200 パーティ", () => {
    let s = 7;
    const rng = (): number => {
      s = (s * 1664525 + 1013904223) % 4294967296;
      return s / 4294967296;
    };
    const chars = ["a", "b", "@", "(", ")", " ", "Z", "-"];
    for (let i = 0; i < 200; i++) {
      const team = Array.from({ length: 1 + Math.floor(rng() * 6) }, () => {
        let nick = "";
        const n = 1 + Math.floor(rng() * 6);
        for (let j = 0; j < n; j++) nick += chars[Math.floor(rng() * chars.length)] ?? "a";
        nick = nick.trim() === "" ? "n" : nick.trim();
        return base({
          nickname: nick,
          itemId: rng() < 0.5 ? "item-p" : null,
          abilityId: rng() < 0.5 ? "abil-p" : null,
        });
      });
      const back = parseShowdownTeam(exportShowdownTeam(team, MASTER).text, MASTER);
      expect(back.issues).toEqual([]);
      expect(back.members).toEqual(team);
    }
  });
});

describe("括弧を含む名前", () => {
  test("種族・持ち物・特性が括弧を含んでも解決できる", () => {
    const text = "Mon (X) @ Orb (S)\nAbility: Skill (A)\nTesty Nature";
    const r = parseShowdownTeam(text, MASTER);
    expect(r.issues).toEqual([]);
    expect(r.members[0]).toEqual(
      expect.objectContaining({
        speciesKey: "9004-000",
        itemId: "item-p",
        abilityId: "abil-p",
        nickname: null,
      }),
    );
  });

  test("括弧入りの種族にニックネームも付けられる", () => {
    const r = parseShowdownTeam("Boss (Mon (X))\nTesty Nature", MASTER);
    expect(r.issues).toEqual([]);
    expect(r.members[0]).toEqual(expect.objectContaining({ speciesKey: "9004-000", nickname: "Boss" }));
  });
});

describe("名前の衝突 (ambiguous_name)", () => {
  test("parse: 同名が複数 ID に付くときは先勝ちで ambiguous_name(warning)", () => {
    const r = parseShowdownTeam("Dupe\nTesty Nature", MASTER);
    expect(r.members[0]?.speciesKey).toBe("9005-000");
    expect(r.issues).toEqual([
      expect.objectContaining({
        code: "ambiguous_name",
        field: "species",
        memberIndex: 0,
        severity: "warning",
      }),
    ]);
  });

  test("export: 後ろの ID は戻すと別の ID になるので ambiguous_name", () => {
    const r = exportShowdownTeam([base({ speciesKey: "9006-000" })], MASTER);
    expect(r.issues).toEqual([expect.objectContaining({ code: "ambiguous_name", field: "species" })]);
    expect(r.text).toContain("Dupe");
  });
});

describe("入力サイズの上限", () => {
  test("全体が 100KB を超えると input_too_large で何も取り込まない", () => {
    const r = parseShowdownTeam("a".repeat(100_001), MASTER);
    expect(r.members).toEqual([]);
    expect(r.issues).toEqual([expect.objectContaining({ code: "input_too_large", memberIndex: null })]);
  });

  test("1行が 1000 文字を超えると malformed_line(空白だらけでも速い)", () => {
    const t0 = Date.now();
    const r = parseShowdownTeam(
      `Testmon\nTesty Nature\n${"x ".repeat(2000)}Nature\n- ${" ".repeat(30000)}x`,
      MASTER,
    );
    expect(Date.now() - t0).toBeLessThan(200);
    expect(r.members).toHaveLength(1);
    expect(r.issues.filter((i) => i.code === "malformed_line")).toHaveLength(2);
  });
});

describe("その他", () => {
  test("EVs 行が複数なら duplicate_line(先の行が勝つ)", () => {
    const r = parseShowdownTeam("Testmon\nEVs: 1 HP\nEVs: 2 HP\nTesty Nature", MASTER);
    expect(r.members[0]?.sp.hp).toBe(1);
    expect(r.issues).toEqual([expect.objectContaining({ code: "duplicate_line", field: "evs" })]);
  });

  test("24 コードポイント超の nickname は export でも省いて警告", () => {
    const r = exportShowdownTeam([base({ nickname: "あ".repeat(25) })], MASTER);
    expect(r.text).toBe("Testmon\nTesty Nature");
    expect(r.issues).toEqual([expect.objectContaining({ code: "nickname_too_long" })]);
  });

  test("SP 0 の明示は問題なし・マスタが空なら export は何も出さず missing_name", () => {
    expect(parseShowdownTeam("Testmon\nEVs: 0 HP\nTesty Nature", MASTER).issues).toEqual([]);
    const empty: ShowdownMaster = { species: [], moves: [], items: [], abilities: [], natures: [] };
    const r = exportShowdownTeam([base({})], empty);
    expect(r.text).toBe("");
    expect(r.issues.map((i) => i.code)).toEqual(["missing_name", "missing_name"]);
  });

  test("toShowdownMaster は MasterData をそのまま通す", async () => {
    const data = await exampleMasterSource.load();
    expect(toShowdownMaster(data).species).toBe(data.species);
  });
});
