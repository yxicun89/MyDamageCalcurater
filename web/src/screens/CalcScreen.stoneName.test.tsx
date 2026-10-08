// I-web-4(ADR-0328): メガストーンの表示名はマスタの nameJa(日本語として使えるとき)。使えないとき(英語名・空)だけ
// 従来の「{基本種名}のメガストーン」。計算画面の持ち物欄の固定表示で確かめる(純粋関数は domain/megaStoneName.test.ts)。

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import { calcScreenText } from "../i18n/ja";
import { bulkRow, createFakeEngine, ok } from "../test/fakeEngine";
import {
  MEGA_FIRE,
  MEGA_FIRE_STONE,
  MEGA_WATER,
  MEGA_WATER_STONE,
  withMegaFixture,
} from "../test/megaMaster";
import { CalcScreen } from "./CalcScreen";

let withStones: (fireName: string, waterName: string) => MasterData;

beforeAll(async () => {
  const example = await exampleMasterSource.load();
  withStones = (fireName, waterName) => {
    const data = withMegaFixture(example);
    return {
      ...data,
      items: data.items.map((item) =>
        item.id === MEGA_FIRE_STONE.id
          ? { ...item, nameJa: fireName }
          : item.id === MEGA_WATER_STONE.id
            ? { ...item, nameJa: waterName }
            : item,
      ),
    };
  };
});

const attackerSpeciesSelect = () => screen.getByRole("combobox", { name: "攻撃側のポケモン" });
const attackerItemSelect = () => screen.getByRole("combobox", { name: "攻撃側の持ち物" });
const defenderSpeciesSelect = () => screen.getByRole("combobox", { name: "防御側のポケモン" });
const defenderItemSelect = () => screen.getByRole("combobox", { name: "防御側の持ち物" });

describe("メガストーンの固定表示", () => {
  test("マスタの nameJa が日本語(全角Ｘ含む)ならそのまま出る", async () => {
    const user = userEvent.setup();
    render(
      <CalcScreen engine={createFakeEngine()} master={withStones("リザードナイトＸ", "ルカリオナイト")} />,
    );
    await user.selectOptions(attackerSpeciesSelect(), MEGA_FIRE.key);
    await user.selectOptions(defenderSpeciesSelect(), MEGA_WATER.key);

    expect(attackerItemSelect()).toHaveDisplayValue("リザードナイトＸ");
    expect(defenderItemSelect()).toHaveDisplayValue("ルカリオナイト");
  });

  test("英語名・空は従来のフォールバック「{基本種名}のメガストーン」", async () => {
    const user = userEvent.setup();
    render(<CalcScreen engine={createFakeEngine()} master={withStones("Barbaracite", "")} />);
    await user.selectOptions(attackerSpeciesSelect(), MEGA_FIRE.key);
    await user.selectOptions(defenderSpeciesSelect(), MEGA_WATER.key);

    expect(attackerItemSelect()).toHaveDisplayValue("テストほのお専用のメガストーン");
    expect(defenderItemSelect()).toHaveDisplayValue("テストみず専用のメガストーン");
  });
});

// ADR-0326 §4(ADR-0328 でも保つ): 英語名のストーンは画面のどこにも出ない(結果の行・未対応の印)。
describe("英語名のメガストーンを画面に出さない", () => {
  const ENGLISH = "Barbaracite";

  test("結果の各行の持ち物は「{基本種名}のメガストーン」で、英語名はどこにも出ない", async () => {
    const user = userEvent.setup();
    render(<CalcScreen engine={createFakeEngine()} master={withStones(ENGLISH, "")} />);
    await user.selectOptions(attackerSpeciesSelect(), "9001-000");
    await user.selectOptions(defenderSpeciesSelect(), MEGA_FIRE.key);

    const list = await screen.findByRole("list", { name: calcScreenText.resultsListLabel });
    const rows = within(list).getAllByRole("listitem");
    expect(rows.length).toBeGreaterThan(0);
    for (const row of rows) {
      expect(within(row).getByText("テストほのお専用のメガストーン")).toBeVisible();
    }
    expect(document.body).not.toHaveTextContent(ENGLISH);
  });

  test("未対応の印の持ち物名も英語名を出さない", async () => {
    const user = userEvent.setup();
    const engine = createFakeEngine((request) =>
      ok({
        defenderSpeciesKey: request.defenderSpecies.key,
        rows: [
          bulkRow({
            itemId: MEGA_FIRE_STONE.id,
            unsupported: [{ target: "defender_item", reason: "unsupported_effect", id: MEGA_FIRE_STONE.id }],
          }),
        ],
      }),
    );
    render(<CalcScreen engine={engine} master={withStones(ENGLISH, "")} />);
    await user.selectOptions(attackerSpeciesSelect(), "9001-000");
    await user.selectOptions(defenderSpeciesSelect(), MEGA_FIRE.key);

    const matches = await screen.findAllByText(/テストほのお専用のメガストーン/);
    expect(matches.some((element) => element.closest('[role="status"]') !== null)).toBe(true);
    expect(document.body).not.toHaveTextContent(ENGLISH);
  });
});
