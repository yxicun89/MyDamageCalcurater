import { describe, expect, it } from "vitest";
import { formatAliasGroup, parseAliasGroups, parseAliasLine } from "./aliases";

// AC-A14(docs/phase4-spec.md): 設定画面の別名グループは 1 行 = 1 グループ・カンマ区切り。
describe("parseAliasLine", () => {
  it.each([
    [
      "S.H.Figuarts, SHフィギュアーツ, フィギュアーツ",
      ["S.H.Figuarts", "SHフィギュアーツ", "フィギュアーツ"],
    ],
    ["HG,ハイグレード", ["HG", "ハイグレード"]],
    ["HG，ハイグレード、エイチジー", ["HG", "ハイグレード", "エイチジー"]],
    ["  HG ,　ハイグレード　", ["HG", "ハイグレード"]],
    ["HG,, ,ハイグレード,", ["HG", "ハイグレード"]],
    ["マスター グレード, MG", ["マスター グレード", "MG"]],
    ["", []],
    [" , 、", []],
  ])("%j", (line, want) => {
    expect(parseAliasLine(line)).toEqual(want);
  });
});

describe("formatAliasGroup", () => {
  it("「, 」でつなぐ(parseAliasLine で元に戻る)", () => {
    const g = ["S.H.Figuarts", "SHフィギュアーツ", "フィギュアーツ"];
    expect(formatAliasGroup(g)).toBe("S.H.Figuarts, SHフィギュアーツ, フィギュアーツ");
    expect(parseAliasLine(formatAliasGroup(g))).toEqual(g);
  });
});

describe("parseAliasGroups", () => {
  it("空の行は除き、順を保つ", () => {
    expect(parseAliasGroups(["HG, ハイグレード", "  ", "MG, マスターグレード"])).toEqual({
      groups: [
        ["HG", "ハイグレード"],
        ["MG", "マスターグレード"],
      ],
      invalidRow: null,
    });
  });
  it("1 語だけの行は invalidRow(1 始まり。空の行も数える)", () => {
    expect(parseAliasGroups(["HG, ハイグレード", "", "MG"]).invalidRow).toBe(3);
    expect(parseAliasGroups(["RG,", "MG, マスターグレード"]).invalidRow).toBe(1);
  });
  it("行が無ければ空", () => {
    expect(parseAliasGroups([])).toEqual({ groups: [], invalidRow: null });
  });
});
