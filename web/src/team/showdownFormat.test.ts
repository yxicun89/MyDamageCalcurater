// P5-5 Showdown 形式の変換部(判定レーン担当)。仕様の正は docs/adr/0310-showdown-format.md(実装時に追加)。
// 純粋なテキスト変換(ADR-0213 §4)。team-svc にも engine にも置かない。名前→ID はマスタから引く(ハードコードしない)。
//
// 受け入れ条件(AC):
//  AC-1 parseShowdownTeam は例外を投げず {members, issues} を返す。空入力・空白だけは issue(empty_input)、
//       7体目以降は members に入れず issue(too_many_members)。パーティは空行区切り。
//  AC-2 1体の書式: `Nickname (Species) (F) @ Item` / `Ability:` / `Level:` / `Tera Type:` / `EVs:` /
//       `<Nature> Nature` / `IVs:` / `- Move`(最大4)。性別 (F)(M) は読んで捨てる。
//  AC-3 名前解決はマスタの nameEn(あれば)・nameJa で引く(英語は大小文字を無視)。日本語名も取り込む。
//       解決できない項目だけを issue(unresolved_name)にして他は取り込む。推測で補わない。
//       持ち物・特性・技・テラスタイプの未解決は、その項目だけ省く(技は詰める)。種族・性格が未解決のメンバーは
//       members に入れない(TeamMember の必須項目)。
//  AC-4 `EVs:` の数値は SP としてそのまま読む(範囲 0..32・合計 <=66)。範囲外・合計超過は丸めず
//       sp_out_of_range / sp_total_exceeded(severity error)で、そのメンバーを members に入れない。
//       32 超(EV らしい値)は加えて ev_like_value(severity warning)。EVs 行が無ければ全 0。
//  AC-5 Level が 50 以外・IVs が 31 以外は取り込まず警告(level_not_50 / iv_not_31。severity warning)。
//  AC-6 issue は何体目(memberIndex。0 始まり。全体は null)のどの項目(field)かと理由コード(code)を持つ。
//       技が5つ以上は too_many_moves、同一技の重複は duplicate_move、解釈できない行は malformed_line。
//  AC-7 exportShowdownTeam は解決済みの TeamMember からテキストを作る。名前はマスタから引き、
//       名前が無い ID は ID をそのまま出さず、その項目だけ省いて issue(missing_name)。
//       SP はそのまま `EVs:` に出し(0 は省く)、Level・IVs 行は出さない。
//  AC-8 ラウンドトリップ: export → parse で同じ TeamMember の列に戻る(性質テスト)。
// 架空の名前だけを使う(実 Pokémon データ禁止。CLAUDE.md ドメイン規約・ADR-0002)。

import { describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import {
  exportShowdownTeam,
  parseShowdownTeam,
  type ShowdownIssue,
  type ShowdownMaster,
} from "./showdownFormat";

type TeamMember = components["schemas"]["TeamMember"];

const MASTER: ShowdownMaster = {
  species: [
    { key: "9001-000", nameJa: "テストモン", nameEn: "Testmon" },
    { key: "9002-000", nameJa: "ダミーラ" },
    { key: "9003-001", nameJa: "サンプリン", nameEn: "Samplin-Alt" },
  ],
  moves: [
    { id: "move-a", nameJa: "ためしうち", nameEn: "Trial Strike" },
    { id: "move-b", nameJa: "ひなたぼうし", nameEn: "Sunny Hat" },
    { id: "move-c", nameJa: "まぼろしのかぜ" },
    { id: "move-d", nameJa: "かりのたて", nameEn: "Mock Shield" },
    { id: "move-e", nameJa: "ごばんめ", nameEn: "Fifth One" },
  ],
  items: [
    { id: "item-a", nameJa: "ためしのたま", nameEn: "Trial Orb" },
    { id: "item-b", nameJa: "かりのおまもり" },
  ],
  abilities: [
    { id: "abil-a", nameJa: "ぎじのうでまえ", nameEn: "Mock Skill" },
    { id: "abil-b", nameJa: "ひかげぐらし" },
  ],
  natures: [
    { id: "nat-a", nameJa: "ためしがち", nameEn: "Testy" },
    { id: "nat-b", nameJa: "のんびりや" },
  ],
};

const ZERO = { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 };

const FULL_TEXT = [
  "Boss (Testmon) (F) @ Trial Orb",
  "Ability: Mock Skill",
  "Level: 50",
  "Tera Type: Fire",
  "EVs: 2 HP / 32 Atk / 32 Spe",
  "Testy Nature",
  "IVs: 31 HP / 31 Atk",
  "- Trial Strike",
  "- Sunny Hat",
].join("\n");

const FULL_MEMBER: TeamMember = {
  speciesKey: "9001-000",
  nickname: "Boss",
  moveIds: ["move-a", "move-b"],
  itemId: "item-a",
  abilityId: "abil-a",
  natureId: "nat-a",
  sp: { ...ZERO, hp: 2, atk: 32, spe: 32 },
  teraType: "fire",
};

const BARE = (species: string): string => `${species}\nTesty Nature`;

function codes(issues: readonly ShowdownIssue[]): string[] {
  return issues.map((i) => i.code);
}

describe("parseShowdownTeam: 1体の書式 (AC-2)", () => {
  test("全項目を TeamMember に読む(性別・Level 50・IVs 31 は問題なし)", () => {
    const r = parseShowdownTeam(FULL_TEXT, MASTER);
    expect(r.issues).toEqual([]);
    expect(r.members).toEqual([FULL_MEMBER]);
  });

  test("ニックネームなし・持ち物なし・特性なし・テラスなしは null / 空で取り込む", () => {
    const r = parseShowdownTeam(BARE("Testmon"), MASTER);
    expect(r.issues).toEqual([]);
    expect(r.members).toEqual([
      {
        speciesKey: "9001-000",
        nickname: null,
        moveIds: [],
        itemId: null,
        abilityId: null,
        natureId: "nat-a",
        sp: ZERO,
        teraType: null,
      },
    ]);
  });

  test("空行区切りで複数体(CRLF も可)", () => {
    const text = [BARE("Testmon"), "", BARE("Samplin-Alt")].join("\r\n");
    const r = parseShowdownTeam(text, MASTER);
    expect(r.members.map((m) => m.speciesKey)).toEqual(["9001-000", "9003-001"]);
  });

  test("EVs の項目名は HP/Atk/Def/SpA/SpD/Spe で、書いていない項目は 0", () => {
    const r = parseShowdownTeam(
      `Testmon\nEVs: 1 HP / 2 Atk / 3 Def / 4 SpA / 5 SpD / 6 Spe\nTesty Nature`,
      MASTER,
    );
    expect(r.members[0]?.sp).toEqual({ hp: 1, atk: 2, def: 3, spa: 4, spd: 5, spe: 6 });
  });
});

describe("parseShowdownTeam: 全体の扱い (AC-1・AC-6)", () => {
  test.each([
    ["", "空"],
    ["  \n \n", "空白だけ"],
  ])("%s は例外にせず empty_input", (text) => {
    const r = parseShowdownTeam(text, MASTER);
    expect(r.members).toEqual([]);
    expect(r.issues).toEqual([
      expect.objectContaining({ code: "empty_input", memberIndex: null, severity: "error" }),
    ]);
  });

  test("7体目以降は members に入れず too_many_members(最大6体)", () => {
    const text = Array.from({ length: 8 }, () => BARE("Testmon")).join("\n\n");
    const r = parseShowdownTeam(text, MASTER);
    expect(r.members).toHaveLength(6);
    expect(r.issues).toEqual([expect.objectContaining({ code: "too_many_members", memberIndex: null })]);
  });

  test("解釈できない行は malformed_line(何体目か・その行を値に持ち、他の項目は取り込む)", () => {
    const r = parseShowdownTeam(`Testmon\n???\nTesty Nature\n- Trial Strike`, MASTER);
    expect(r.issues).toEqual([
      expect.objectContaining({ code: "malformed_line", memberIndex: 0, value: "???" }),
    ]);
    expect(r.members[0]?.moveIds).toEqual(["move-a"]);
  });

  test("技は5つ以上で too_many_moves(先頭4つを残す)、同一技は duplicate_move(1つにする)", () => {
    const five = parseShowdownTeam(
      `${BARE("Testmon")}\n- Trial Strike\n- Sunny Hat\n- Mock Shield\n- Fifth One\n- Mock Skill`,
      MASTER,
    );
    expect(codes(five.issues)).toContain("too_many_moves");
    expect(five.members[0]?.moveIds).toEqual(["move-a", "move-b", "move-d", "move-e"]);

    const dup = parseShowdownTeam(`${BARE("Testmon")}\n- Trial Strike\n- Trial Strike`, MASTER);
    expect(codes(dup.issues)).toEqual(["duplicate_move"]);
    expect(dup.members[0]?.moveIds).toEqual(["move-a"]);
  });

  test("例外を投げない(乱雑な入力でも {members, issues})", () => {
    for (const text of ["@", "()", "- ", "EVs:", "Level: x", "\u0000\n\n\n(", "Nature"]) {
      const r = parseShowdownTeam(text, MASTER);
      expect(Array.isArray(r.members)).toBe(true);
      expect(Array.isArray(r.issues)).toBe(true);
    }
  });
});

describe("parseShowdownTeam: 名前の解決 (AC-3)", () => {
  test("日本語名で取り込める(英語名が無い項目は日本語名だけ)", () => {
    const text = [
      "ダミーラ @ かりのおまもり",
      "特性: ひかげぐらし",
      "のんびりや Nature",
      "- まぼろしのかぜ",
    ].join("\n");
    const r = parseShowdownTeam(text.replace("特性:", "Ability:"), MASTER);
    expect(r.issues).toEqual([]);
    expect(r.members).toEqual([
      expect.objectContaining({
        speciesKey: "9002-000",
        itemId: "item-b",
        abilityId: "abil-b",
        natureId: "nat-b",
        moveIds: ["move-c"],
      }),
    ]);
  });

  test("英語名は大小文字・前後の空白を無視する", () => {
    const r = parseShowdownTeam(`  testmon  @ trial ORB\nTESTY nature`, MASTER);
    expect(r.issues).toEqual([]);
    expect(r.members[0]).toEqual(expect.objectContaining({ speciesKey: "9001-000", itemId: "item-a" }));
  });

  test("未解決の持ち物・特性・技・テラスは、その項目だけ省いて unresolved_name(他は取り込む)", () => {
    const text = [
      "Testmon @ Nonexistent Orb",
      "Ability: Nonexistent Skill",
      "Tera Type: Nonexistent",
      "Testy Nature",
      "- Trial Strike",
      "- Nonexistent Move",
      "- Sunny Hat",
    ].join("\n");
    const r = parseShowdownTeam(text, MASTER);
    expect(r.members).toEqual([
      expect.objectContaining({
        speciesKey: "9001-000",
        itemId: null,
        abilityId: null,
        teraType: null,
        moveIds: ["move-a", "move-b"],
      }),
    ]);
    expect(r.issues.map((i) => [i.code, i.field, i.memberIndex, i.value])).toEqual([
      ["unresolved_name", "item", 0, "Nonexistent Orb"],
      ["unresolved_name", "ability", 0, "Nonexistent Skill"],
      ["unresolved_name", "teraType", 0, "Nonexistent"],
      ["unresolved_name", "move", 0, "Nonexistent Move"],
    ]);
  });

  test("種族が未解決のメンバーは members に入れず、他のメンバーは取り込む(何体目かが分かる)", () => {
    const r = parseShowdownTeam([BARE("Testmon"), BARE("Nonexistentmon")].join("\n\n"), MASTER);
    expect(r.members.map((m) => m.speciesKey)).toEqual(["9001-000"]);
    expect(r.issues).toEqual([
      expect.objectContaining({
        code: "unresolved_name",
        field: "species",
        memberIndex: 1,
        severity: "error",
      }),
    ]);
  });

  test("性格が未解決・性格の行が無いメンバーは members に入れない(natureId は必須。推測で補わない)", () => {
    const unresolved = parseShowdownTeam("Testmon\nNonexistent Nature", MASTER);
    expect(unresolved.members).toEqual([]);
    expect(unresolved.issues).toEqual([
      expect.objectContaining({ code: "unresolved_name", field: "nature", memberIndex: 0 }),
    ]);
    const missing = parseShowdownTeam("Testmon", MASTER);
    expect(missing.members).toEqual([]);
    expect(missing.issues).toEqual([
      expect.objectContaining({ code: "missing_nature", field: "nature", memberIndex: 0 }),
    ]);
  });

  test("名前リストを持たない(マスタが空なら何も解決しない。コードに名前を持たない)", () => {
    const empty: ShowdownMaster = { species: [], moves: [], items: [], abilities: [], natures: [] };
    const r = parseShowdownTeam(FULL_TEXT, empty);
    expect(r.members).toEqual([]);
    expect(codes(r.issues)).toContain("unresolved_name");
  });
});

describe("parseShowdownTeam: SP・Level・IVs (AC-4・AC-5)", () => {
  test("33 以上は丸めず sp_out_of_range(error)+ ev_like_value(warning)で、メンバーを入れない", () => {
    const r = parseShowdownTeam(`Testmon\nEVs: 252 Atk\nTesty Nature`, MASTER);
    expect(r.members).toEqual([]);
    expect(r.issues).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ code: "sp_out_of_range", field: "sp", memberIndex: 0, severity: "error" }),
        expect.objectContaining({ code: "ev_like_value", field: "sp", memberIndex: 0, severity: "warning" }),
      ]),
    );
  });

  test("負数・小数は sp_out_of_range", () => {
    for (const v of ["-1 Atk", "1.5 Atk"]) {
      const r = parseShowdownTeam(`Testmon\nEVs: ${v}\nTesty Nature`, MASTER);
      expect(r.members).toEqual([]);
      expect(codes(r.issues)).toContain("sp_out_of_range");
    }
  });

  test("合計 66 は通り、67 は sp_total_exceeded(丸めない)", () => {
    const ok = parseShowdownTeam(`Testmon\nEVs: 32 HP / 32 Atk / 2 Spe\nTesty Nature`, MASTER);
    expect(ok.issues).toEqual([]);
    expect(ok.members).toHaveLength(1);
    const over = parseShowdownTeam(`Testmon\nEVs: 32 HP / 32 Atk / 3 Spe\nTesty Nature`, MASTER);
    expect(over.members).toEqual([]);
    expect(over.issues).toEqual([
      expect.objectContaining({ code: "sp_total_exceeded", field: "sp", memberIndex: 0, severity: "error" }),
    ]);
  });

  test("Level が 50 以外・IVs が 31 以外は取り込まず警告(メンバーは取り込む)", () => {
    const r = parseShowdownTeam(`Testmon\nLevel: 100\nIVs: 0 Atk\nTesty Nature`, MASTER);
    expect(r.members).toHaveLength(1);
    expect(r.issues).toEqual([
      expect.objectContaining({ code: "level_not_50", field: "level", memberIndex: 0, severity: "warning" }),
      expect.objectContaining({ code: "iv_not_31", field: "ivs", memberIndex: 0, severity: "warning" }),
    ]);
  });
});

describe("exportShowdownTeam (AC-7)", () => {
  test("TeamMember を Showdown テキストにする(Level・IVs は出さず、EVs: に SP をそのまま出す)", () => {
    const r = exportShowdownTeam([FULL_MEMBER], MASTER);
    expect(r.issues).toEqual([]);
    expect(r.text).toBe(
      [
        "Boss (Testmon) @ Trial Orb",
        "Ability: Mock Skill",
        "Tera Type: Fire",
        "EVs: 2 HP / 32 Atk / 32 Spe",
        "Testy Nature",
        "- Trial Strike",
        "- Sunny Hat",
      ].join("\n"),
    );
  });

  test("パーティは空行1つで区切る。0 の SP は出さず、全 0 なら EVs 行も出さない", () => {
    const bare: TeamMember = {
      speciesKey: "9003-001",
      natureId: "nat-a",
      moveIds: [],
      sp: ZERO,
    };
    const r = exportShowdownTeam([bare, bare], MASTER);
    expect(r.text).toBe(["Samplin-Alt\nTesty Nature", "Samplin-Alt\nTesty Nature"].join("\n\n"));
  });

  test("nameEn が無ければ日本語名を出す(名前はマスタから引く)", () => {
    const m: TeamMember = {
      speciesKey: "9002-000",
      natureId: "nat-b",
      sp: ZERO,
      itemId: "item-b",
      moveIds: ["move-c"],
    };
    const r = exportShowdownTeam([m], MASTER);
    expect(r.text).toBe("ダミーラ @ かりのおまもり\nのんびりや Nature\n- まぼろしのかぜ");
  });

  test("名前が無い ID は ID を出さず、その項目だけ省いて missing_name", () => {
    const m: TeamMember = {
      speciesKey: "9001-000",
      natureId: "nat-a",
      sp: ZERO,
      itemId: "item-unknown",
      abilityId: "abil-unknown",
      moveIds: ["move-a", "move-unknown"],
    };
    const r = exportShowdownTeam([m], MASTER);
    expect(r.text).toBe("Testmon\nTesty Nature\n- Trial Strike");
    expect(r.text).not.toContain("unknown");
    expect(r.issues.map((i) => [i.code, i.field, i.memberIndex, i.value])).toEqual([
      ["missing_name", "item", 0, "item-unknown"],
      ["missing_name", "ability", 0, "abil-unknown"],
      ["missing_name", "move", 0, "move-unknown"],
    ]);
  });

  test("種族・性格の名前が無いメンバーは出力に含めず missing_name(error)", () => {
    const good: TeamMember = { speciesKey: "9001-000", natureId: "nat-a", moveIds: [], sp: ZERO };
    const bad: TeamMember = { speciesKey: "0000-000", natureId: "nat-a", moveIds: [], sp: ZERO };
    const r = exportShowdownTeam([bad, good], MASTER);
    expect(r.text).toBe("Testmon\nTesty Nature");
    expect(r.issues).toEqual([
      expect.objectContaining({ code: "missing_name", field: "species", memberIndex: 0, severity: "error" }),
    ]);
  });

  test("空配列は空文字列(例外にしない)", () => {
    expect(exportShowdownTeam([], MASTER)).toEqual({ text: "", issues: [] });
  });
});

describe("ラウンドトリップ (AC-8)", () => {
  // 外部の乱数を使わない決定的な疑似乱数(線形合同法)で、有効な TeamMember を多数作る。
  function makeRng(seed: number): () => number {
    let s = seed;
    return () => {
      s = (s * 1664525 + 1013904223) % 4294967296;
      return s / 4294967296;
    };
  }

  const TYPES = ["fire", "water", "grass", "fairy", "steel"] as const;

  function randomMember(rng: () => number): TeamMember {
    const pick = <T>(xs: readonly T[]): T => xs[Math.floor(rng() * xs.length)] as T;
    const maybe = <T>(v: T): T | null => (rng() < 0.5 ? v : null);
    const stats = ["hp", "atk", "def", "spa", "spd", "spe"] as const;
    const sp = { ...ZERO };
    let budget = 66;
    for (const k of stats) {
      const v = Math.min(budget, Math.floor(rng() * 33));
      sp[k] = v;
      budget -= v;
    }
    const moves = MASTER.moves
      .map((m) => m.id)
      .filter(() => rng() < 0.5)
      .slice(0, 4);
    return {
      speciesKey: pick(MASTER.species).key,
      nickname: maybe(`Nick${Math.floor(rng() * 100)}`),
      moveIds: moves,
      itemId: maybe(pick(MASTER.items).id),
      abilityId: maybe(pick(MASTER.abilities).id),
      natureId: pick(MASTER.natures).id,
      sp,
      teraType: maybe(pick(TYPES)),
    };
  }

  test("export → parse で同じメンバーの列に戻り、issue は出ない(200 パーティ)", () => {
    const rng = makeRng(20261003);
    for (let i = 0; i < 200; i++) {
      const size = 1 + Math.floor(rng() * 6);
      const team = Array.from({ length: size }, () => randomMember(rng));
      const out = exportShowdownTeam(team, MASTER);
      expect(out.issues).toEqual([]);
      const back = parseShowdownTeam(out.text, MASTER);
      expect(back.issues).toEqual([]);
      expect(back.members).toEqual(team);
    }
  });

  test("日本語名しか無い項目でもラウンドトリップする", () => {
    const m: TeamMember = {
      speciesKey: "9002-000",
      nickname: null,
      moveIds: ["move-c"],
      itemId: "item-b",
      abilityId: "abil-b",
      natureId: "nat-b",
      sp: { ...ZERO, spd: 32 },
      teraType: null,
    };
    const back = parseShowdownTeam(exportShowdownTeam([m], MASTER).text, MASTER);
    expect(back.members).toEqual([m]);
  });
});
