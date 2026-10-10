// @vitest-environment node
// G-01(ADR-0341): 並びの切り替え機能(ADR-0335 の習得順・五十音順・チップ・記憶)はユーザー決定(2026-10-11)で廃止した。
// 部品・フック・保存・文言・旧ドメイン関数が残っていないこと(死んだコードを残さない)。
// 旧テスト(App.moveSort / CalcScreen.moveSort / ReverseScreen.moveSort / moveSortStorage / domain/moveSort)は
// 機能の廃止により削除する(テストを弱めて通すのではなく、対象の機能自体を消すため。ADR-0341 に記録)。

import { existsSync } from "node:fs";
import { describe, expect, test } from "vitest";

const gone = [
  "screens/MoveSortControls.tsx",
  "app/useMoveSort.ts",
  "app/moveSortStorage.ts",
  "i18n/moveSort.ts",
  "domain/moveSort.ts",
  "App.moveSort.test.tsx",
  "screens/CalcScreen.moveSort.test.tsx",
  "screens/ReverseScreen.moveSort.test.tsx",
  "app/moveSortStorage.test.ts",
  "domain/moveSort.test.ts",
];

describe("並びの切り替えの廃止", () => {
  test.each(gone)("%s は存在しない", (path) => {
    expect(existsSync(new URL(`./${path}`, import.meta.url))).toBe(false);
  });

  test.each(["screens/MovePicker.tsx", "screens/MovePicker.css", "domain/moveOrder.ts"])(
    "%s がある",
    (path) => {
      expect(existsSync(new URL(`./${path}`, import.meta.url))).toBe(true);
    },
  );
});
