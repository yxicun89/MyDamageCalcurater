// P4-1: デザイントークンはアプリの入口で1回だけ読み込む。

import { readFileSync } from "node:fs";
import { expect, test } from "vitest";
import { localPath } from "../test/localPath";

test("main.tsx が styles/tokens.css を読み込む", () => {
  const main = readFileSync(localPath("../main.tsx", import.meta.url), "utf8");
  expect(main).toMatch(/^import\s+["']\.\/styles\/tokens\.css["'];?\s*$/m);
});
