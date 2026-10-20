// import の向きの検査(web/eslint.boundaries.js・ADR-0811・architecture-adoption.md §5.2)の設定テスト。
// 規則が効くこと(例外外の新しい違反が失敗する)と、凍結した例外が今も存在することを確かめる。
import { existsSync } from "node:fs";
import { resolve } from "node:path";
import { Linter } from "eslint";
import tseslint from "typescript-eslint";
import { describe, expect, it } from "vitest";
import {
  buildImportBoundaryConfigs,
  importBoundaryConfigs,
  KNOWN_VIOLATIONS,
} from "../../eslint.boundaries.js";

const webRoot = resolve(import.meta.dirname, "../..");

function restricted(filename: string, code: string, configs = importBoundaryConfigs): string[] {
  const linter = new Linter({ configType: "flat", cwd: webRoot });
  const messages = linter.verify(code, [{ languageOptions: { parser: tseslint.parser } }, ...configs], {
    filename: resolve(webRoot, filename),
  });
  return messages.filter((m) => m.ruleId === "no-restricted-imports").map((m) => m.message);
}

describe("import の向きの検査", () => {
  it.each([
    ["機能 → 機能", "src/adjust/New.ts", 'import { x } from "../speed/speedClient";'],
    ["機能 → screens", "src/speed/New.tsx", 'import { x } from "../screens/ToggleChip";'],
    ["機能 → app", "src/team/New.ts", 'import { x } from "../app/storage";'],
    ["共有 → 機能", "src/ui/New.tsx", 'import { x } from "../favorites/favoriteCalc";'],
    ["共有 → app", "src/master/New.ts", 'import { x } from "../app/routes";'],
    ["共有 → screens", "src/domain/New.ts", 'import { x } from "../screens/MovePicker";'],
    ["domain → api/*.gen", "src/domain/New.ts", 'import type { t } from "../api/balance.gen";'],
  ])("例外外の違反を検出する: %s", (_name, file, code) => {
    expect(restricted(file, code)).toHaveLength(1);
  });

  it.each([
    ["機能 → shared", "src/adjust/New.ts", 'import { x } from "../domain/requests";'],
    ["機能自身の中", "src/team/New.ts", 'import { x } from "./teamSlots";'],
    ["shared → shared", "src/api/New.ts", 'import { x } from "../domain/requests";'],
    ["i18n 内の ./speed(自身のファイル)", "src/i18n/New.ts", 'import { x } from "./speed";'],
    ["screens(登録)→ app", "src/speed/speed.screen.tsx", 'import { x } from "../app/routes";'],
    ["api → api/*.gen", "src/api/New.ts", 'import type { t } from "./balance.gen";'],
  ])("許す向きは通す: %s", (_name, file, code) => {
    expect(restricted(file, code)).toHaveLength(0);
  });

  it("凍結した例外は規則を外した形でだけ許す(別の違反は検出する)", () => {
    // W1・W2 で例外一覧は空になったため、仕組みは架空の例外で確かめる。
    const configs = buildImportBoundaryConfigs([
      { wave: "W1", file: "src/adjust/AdjustScreen.tsx", allow: ["screens"] },
    ]);
    // src/adjust/AdjustScreen.tsx は screens だけ許す。app は禁止のまま。
    expect(
      restricted("src/adjust/AdjustScreen.tsx", 'import { x } from "../screens/MovePicker";', configs),
    ).toHaveLength(0);
    expect(
      restricted("src/adjust/AdjustScreen.tsx", 'import { x } from "../app/storage";', configs),
    ).toHaveLength(1);
    // 例外の無い実際の設定では、同じ import を検出する。
    expect(
      restricted("src/adjust/AdjustScreen.tsx", 'import { x } from "../screens/MovePicker";'),
    ).toHaveLength(1);
  });

  it("既知の違反(W1〜W5)は解消済みで、例外一覧は空", () => {
    expect(KNOWN_VIOLATIONS).toEqual([]);
  });

  it("凍結した例外のファイルは実在する(解消したら一覧から消す)", () => {
    for (const { file } of KNOWN_VIOLATIONS) {
      expect(existsSync(resolve(webRoot, file)), file).toBe(true);
    }
  });
});
