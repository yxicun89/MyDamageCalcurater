// issue #515・ADR-0320: 例データ→ calc-svc の共通マスタのスナップショット(MasterExport)で、メガ種族の
// isMega・requiredItemId を false・null 固定にせず MasterSpecies のとおり出す(E2E の calc-svc がメガを検証できる)。

import { beforeAll, expect, test } from "vitest";
import { MEGA_FIRE, MEGA_FIRE_STONE, withMegaFixture } from "../test/megaMaster";
import { exampleMasterSource } from "./exampleSource";
import { toCalcSnapshot } from "./exportSnapshot";
import type { MasterData } from "./types";

let master: MasterData;

beforeAll(async () => {
  master = withMegaFixture(await exampleMasterSource.load());
});

test("メガ種族は isMega=true・requiredItemId=メガストーンの ID、基本種は false・null", () => {
  const snapshot = toCalcSnapshot(master);
  const mega = snapshot.species.find((species) => species.key === MEGA_FIRE.key);
  expect(mega?.isMega).toBe(true);
  expect(mega?.requiredItemId).toBe(MEGA_FIRE_STONE.id);
  const base = snapshot.species.find((species) => species.key === "9001-000");
  expect(base?.isMega).toBe(false);
  expect(base?.requiredItemId).toBeNull();
});
