// F-12(I-web-6、ADR-0331 §2): タブ(画面)ごとのアイコン。画面 ID → アイコン名の対応は app/screenIcons.ts に1か所。
// 画面の登録ファイル(ADR-0323)は変えない(調整画面〈web/src/adjust/〉はこの PR で触らないため)。
// 対応表に無い画面(後から足された画面)は既定のアイコン("screen")になり、タブ列は壊れない。

import { describe, expect, test } from "vitest";
import { ICON_NAMES } from "../ui/Icon";
import { SCREENS } from "./screens";
import { screenIconName } from "./screenIcons";

describe("screenIconName", () => {
  test.each([
    ["calc", "calc"],
    ["reverse", "reverse"],
    ["balance", "balance"],
    ["speed", "speed"],
    ["judge", "judge"],
    ["team", "team"],
    ["favorites", "favorites"],
    ["adjust", "adjust"],
  ])("%s → %s", (screenId, icon) => {
    expect(screenIconName(screenId)).toBe(icon);
  });

  test("対応表に無い画面は既定の screen", () => {
    expect(screenIconName("future-screen")).toBe("screen");
  });

  test("登録済みのすべての画面のアイコンが Icon に存在する", () => {
    for (const screen of SCREENS) {
      expect(ICON_NAMES).toContain(screenIconName(screen.id));
    }
  });
});
