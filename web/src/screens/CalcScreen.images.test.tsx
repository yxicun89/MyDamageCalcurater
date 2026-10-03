// P8-1c(ADR-0325): 計算画面の攻撃側・防御側カードのポケモン画像。manifest にキーがあれば <img>(thumb)、
// 無ければ従来のタイプ色エンブレム(画像なしで成立 = AC-X)。画像は装飾(alt 空)で、名前は見出しで読める。

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import { PokemonImagesProvider } from "../images/PokemonImagesContext";
import { parsePokemonImageManifest } from "../images/pokemonImages";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import { createFakeEngine } from "../test/fakeEngine";
import { CalcScreen } from "./CalcScreen";

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

const attackerCard = () => screen.getByRole("region", { name: "攻撃側" });
const defenderCard = () => screen.getByRole("region", { name: "防御側" });

async function renderWith(
  imageKeys: readonly string[],
): Promise<{ attackerKey: string; defenderKey: string }> {
  const attacker = master.species[0];
  const defender = master.species[1];
  if (attacker === undefined || defender === undefined) {
    throw new Error("例データに種族が足りない");
  }
  const manifest = parsePokemonImageManifest({
    version: 1,
    images: Object.fromEntries(
      imageKeys.map((key) => [
        key,
        { thumb: `thumb/${key}.aaaaaaaa.webp`, detail: `detail/${key}.bbbbbbbb.webp` },
      ]),
    ),
  });
  const user = userEvent.setup();
  render(
    <PokemonImagesProvider manifest={manifest}>
      <CalcScreen engine={createFakeEngine()} master={master} />
    </PokemonImagesProvider>,
  );
  await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
  await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), defender.key);
  return { attackerKey: attacker.key, defenderKey: defender.key };
}

describe("カードのポケモン画像", () => {
  test("manifest にあるキーのカードは thumb の <img>(lazy・alt 空)を出し、エンブレムは出さない", async () => {
    const { attackerKey, defenderKey } = await renderWith([
      master.species[0]?.key ?? "",
      master.species[1]?.key ?? "",
    ]);
    for (const [card, key] of [
      [attackerCard(), attackerKey],
      [defenderCard(), defenderKey],
    ] as const) {
      const img = card.querySelector("img");
      expect(img?.getAttribute("src")).toBe(`/images/thumb/${key}.aaaaaaaa.webp`);
      expect(img?.getAttribute("loading")).toBe("lazy");
      expect(img?.getAttribute("alt")).toBe("");
      expect(within(card).queryByTestId("type-emblem")).not.toBeInTheDocument();
    }
  });

  test("manifest に無い側だけエンブレムのまま(攻撃側は画像・防御側はエンブレム)", async () => {
    await renderWith([master.species[0]?.key ?? ""]);
    expect(attackerCard().querySelector("img")).not.toBeNull();
    expect(within(attackerCard()).queryByTestId("type-emblem")).not.toBeInTheDocument();
    expect(defenderCard().querySelector("img")).toBeNull();
    expect(within(defenderCard()).getByTestId("type-emblem")).toBeInTheDocument();
  });

  test("画像があっても名前の見出し・タイプ表示は変わらない(画像は読み上げ対象でない)", async () => {
    await renderWith([master.species[0]?.key ?? ""]);
    const species = master.species[0];
    expect(within(attackerCard()).getByRole("heading", { name: species?.nameJa })).toBeInTheDocument();
    expect(within(attackerCard()).queryByRole("img")).not.toBeInTheDocument();
  });
});
