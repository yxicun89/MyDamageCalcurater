// ADR-0332 §1(F-08 / I-web-7): 構築の表示名の導出(team/teamName.ts の teamDisplayNames。実装は後続)。
// 構築名の入力は廃止し、サーバーが既定名「名称未設定」を入れる(ADR-0229)。画面はそれを「構築 N」と出す。
// 確かめること:
//   N-1 既定名の構築は「構築 N」。N は既定名の構築だけを作成日時の古い順に数えた 1 始まりの番号(同時刻は id の昇順)
//   N-2 名前付きの古い構築はその名前のまま(番号を消費しない)
//   N-3 新しい構築を先頭に足しても、既存の構築の番号は変わらない
//   N-4 既定名との一致は完全一致だけ(サーバーに保存された名前は変えない)
// 架空の ID・構築名だけを使う(ADR-0002)。

import { describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { teamScreenText } from "../i18n/team";
import { SERVER_DEFAULT_TEAM_NAME, teamDisplayNames } from "./teamName";

type Schemas = components["schemas"];

function team(id: string, name: string, createdAt: string): Schemas["Team"] {
  return { id, name, members: [], createdAt, updatedAt: createdAt };
}

const OLDEST = team("11111111-1111-4111-8111-111111111111", "名称未設定", "2026-10-01T09:00:00Z");
const MIDDLE = team("22222222-2222-4222-8222-222222222222", "名称未設定", "2026-10-02T09:00:00Z");
const NEWEST = team("33333333-3333-4333-8333-333333333333", "名称未設定", "2026-10-03T09:00:00Z");
const NAMED = team("44444444-4444-4444-8444-444444444444", "テスト構築A", "2026-09-30T09:00:00Z");

describe("teamDisplayNames", () => {
  test("サーバーの既定名は ADR-0229 の「名称未設定」", () => {
    expect(SERVER_DEFAULT_TEAM_NAME).toBe("名称未設定");
  });

  test("N-1 既定名の構築は作成の古い順に「構築 1」「構築 2」…(一覧の並び〈新しい順〉とは独立)", () => {
    const names = teamDisplayNames([NEWEST, MIDDLE, OLDEST]);
    expect(names.get(OLDEST.id)).toBe(teamScreenText.untitledTeamName(1));
    expect(names.get(MIDDLE.id)).toBe(teamScreenText.untitledTeamName(2));
    expect(names.get(NEWEST.id)).toBe(teamScreenText.untitledTeamName(3));
  });

  test("N-1 同じ作成日時は id の昇順", () => {
    const later = team("99999999-9999-4999-8999-999999999999", "名称未設定", OLDEST.createdAt);
    const names = teamDisplayNames([later, OLDEST]);
    expect(names.get(OLDEST.id)).toBe(teamScreenText.untitledTeamName(1));
    expect(names.get(later.id)).toBe(teamScreenText.untitledTeamName(2));
  });

  test("N-2 名前付きの古い構築はその名前のまま、番号を消費しない", () => {
    const names = teamDisplayNames([MIDDLE, NAMED, OLDEST]);
    expect(names.get(NAMED.id)).toBe("テスト構築A");
    expect(names.get(OLDEST.id)).toBe(teamScreenText.untitledTeamName(1));
    expect(names.get(MIDDLE.id)).toBe(teamScreenText.untitledTeamName(2));
  });

  test("N-3 新しい構築を先頭に足しても、既存の構築の番号は変わらない", () => {
    const before = teamDisplayNames([MIDDLE, OLDEST]);
    const after = teamDisplayNames([NEWEST, MIDDLE, OLDEST]);
    expect(after.get(OLDEST.id)).toBe(before.get(OLDEST.id));
    expect(after.get(MIDDLE.id)).toBe(before.get(MIDDLE.id));
  });

  test("N-4 既定名を含むだけの名前・前後に空白のある名前は、保存された名前のまま", () => {
    const similar = team("55555555-5555-4555-8555-555555555555", "テスト名称未設定", OLDEST.createdAt);
    const spaced = team("66666666-6666-4666-8666-666666666666", " 名称未設定 ", OLDEST.createdAt);
    const names = teamDisplayNames([similar, spaced]);
    expect(names.get(similar.id)).toBe("テスト名称未設定");
    expect(names.get(spaced.id)).toBe(" 名称未設定 ");
  });

  test("すべての構築に表示名がある(空の一覧は空)", () => {
    expect(teamDisplayNames([]).size).toBe(0);
    const teams = [NEWEST, NAMED, MIDDLE, OLDEST];
    const names = teamDisplayNames(teams);
    expect(teams.every((candidate) => names.has(candidate.id))).toBe(true);
    expect([...names.values()]).not.toContain(SERVER_DEFAULT_TEAM_NAME);
  });

  test("N-5 名前付きの古い構築を Web で保存して既定名になると、作成日時順で番号が割り当たり、後ろの構築の番号がずれる", () => {
    const before = teamDisplayNames([MIDDLE, NAMED, OLDEST]);
    expect(before.get(MIDDLE.id)).toBe(teamScreenText.untitledTeamName(2));
    // NAMED(作成が最も古い)が保存で既定名に置き換わる。
    const saved = team(NAMED.id, "名称未設定", NAMED.createdAt);
    const after = teamDisplayNames([MIDDLE, saved, OLDEST]);
    expect(after.get(saved.id)).toBe(teamScreenText.untitledTeamName(1));
    expect(after.get(OLDEST.id)).toBe(teamScreenText.untitledTeamName(2));
    expect(after.get(MIDDLE.id)).toBe(teamScreenText.untitledTeamName(3));
  });
});
