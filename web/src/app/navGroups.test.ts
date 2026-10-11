// ADR-0356: ナビの区分(上位のタブ)のまとめ方と、実際の登録の区分。

import { describe, expect, test } from "vitest";
import { NAV_GROUPS, buildNavTabs, type NavGroup } from "./navGroups";
import { SCREEN_ROUTES } from "./routes";

function group(id: string, order: number): NavGroup {
  return { id, label: id, order, icon: "screen" };
}

describe("buildNavTabs", () => {
  test("区分の order の昇順に並べ、区分の中の画面は渡された順のまま", () => {
    const damage = group("damage", 100);
    const speed = group("speed", 300);
    const team = group("team", 200);
    const tabs = buildNavTabs([
      { id: "calc", group: damage },
      { id: "speed", group: speed },
      { id: "team", group: team },
      { id: "reverse", group: damage },
    ]);
    expect(tabs.map((tab) => [tab.group.id, tab.screenIds])).toEqual([
      ["damage", ["calc", "reverse"]],
      ["team", ["team"]],
      ["speed", ["speed"]],
    ]);
  });

  test("同じ order の区分は先に出た順", () => {
    const tabs = buildNavTabs([
      { id: "a", group: group("a", 100) },
      { id: "b", group: group("b", 100) },
    ]);
    expect(tabs.map((tab) => tab.group.id)).toEqual(["a", "b"]);
  });
});

describe("実際の登録の区分(ADR-0356)", () => {
  test("上位は ダメージ・構築・素早さ・記録 の 4 つで、ダメージの中は 計算・逆算・調整", () => {
    const tabs = buildNavTabs(SCREEN_ROUTES);
    expect(tabs.map((tab) => [tab.group.id, tab.screenIds])).toEqual([
      ["damage", ["calc", "reverse", "adjust"]],
      ["team", ["team"]],
      ["speed", ["speed"]],
      ["records", ["favorites"]],
    ]);
    expect(tabs.map((tab) => tab.group.label)).toEqual(["ダメージ", "構築", "素早さ", "記録"]);
  });

  test("区分のアイコンは区分どうし・下位の画面のアイコンと重ならない(形でも分かる)", () => {
    const icons = Object.values(NAV_GROUPS).map((navGroup) => navGroup.icon);
    expect(new Set(icons).size).toBe(icons.length);
    expect(icons).not.toContain("calc");
    expect(icons).not.toContain("reverse");
    expect(icons).not.toContain("adjust");
  });
});
