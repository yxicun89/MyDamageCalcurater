// P8-1c(ADR-0325)→ I-speed-4b(ADR-0610 §4): 素早さ画面のポケモンカードの画像。
// manifest にあれば <img>(thumb・lazy・alt 空)、無ければ従来のタイプ色エンブレム。画像は必須にしない。

import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test } from "vitest";
import { PokemonImagesProvider } from "../images/PokemonImagesContext";
import { parsePokemonImageManifest } from "../images/pokemonImages";
import { speedScreenText } from "../i18n/ja";
import { BIRD, FISH, createFakeSpeedClient, lastOf, pokemonListResponse } from "../test/fakeSpeedClient";
import { SpeedScreen } from "./SpeedScreen";

describe("素早さ画面のポケモンカードの画像", () => {
  test("manifest にある自分のカードは <img>、無い相手のカードはタイプ色エンブレム", async () => {
    const manifest = parsePokemonImageManifest({
      version: 1,
      images: {
        "9001-000": { thumb: "thumb/9001-000.aaaaaaaa.webp", detail: "detail/9001-000.bbbbbbbb.webp" },
      },
    });
    const user = userEvent.setup();
    const client = createFakeSpeedClient();
    render(
      <PokemonImagesProvider manifest={manifest}>
        <SpeedScreen speedClient={client} />
      </PokemonImagesProvider>,
    );
    await act(async () => {
      lastOf(client.pokemonCalls, "pokemon").resolve({ ok: true, value: pokemonListResponse });
      await Promise.resolve();
    });
    await user.selectOptions(
      screen.getByRole("combobox", { name: speedScreenText.selfPokemonLabel }),
      BIRD.pokemonId,
    );
    await user.selectOptions(
      screen.getByRole("combobox", { name: speedScreenText.opponentPokemonLabel }),
      FISH.pokemonId,
    );

    const self = within(screen.getByRole("region", { name: speedScreenText.selfRegionLabel }));
    const img = self
      .getByRole("heading", { level: 3, name: BIRD.nameJa })
      .closest(".ui-pokemon-card")
      ?.querySelector("img");
    expect(img?.getAttribute("src")).toBe("/images/thumb/9001-000.aaaaaaaa.webp");
    expect(img?.getAttribute("alt")).toBe("");
    expect(self.queryByTestId("pokemon-icon-emblem")).not.toBeInTheDocument();

    const opponent = within(screen.getByRole("region", { name: speedScreenText.opponentRegionLabel }));
    expect(opponent.getByTestId("pokemon-icon-emblem")).toBeInTheDocument();
    expect(opponent.queryByRole("img")).not.toBeInTheDocument();
  });
});
