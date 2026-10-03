// P8-1c(ADR-0325): 素早さ画面の各行のポケモン画像。manifest にあれば <img>(thumb)、無ければ従来のタイプ色エンブレム。

import { render, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, test } from "vitest";
import { PokemonImagesProvider } from "../images/PokemonImagesContext";
import { parsePokemonImageManifest } from "../images/pokemonImages";
import { speedScreenText } from "../i18n/ja";
import { SpeedScreen } from "./SpeedScreen";
import type { components } from "./speed.gen";
import type { SpeedClient } from "./speedClient";

type Schemas = components["schemas"];

const BIRD: Schemas["SpeedPokemon"] = {
  pokemonId: "9001-000",
  nameJa: "テストカソウドリ",
  types: ["fire"],
  baseSpeed: 100,
};
const FISH: Schemas["SpeedPokemon"] = {
  pokemonId: "9002-000",
  nameJa: "テストカソウギョ",
  types: ["water"],
  baseSpeed: 80,
};

const client: SpeedClient = {
  pokemon: () => Promise.resolve({ ok: true, value: { regulationId: "example", pokemon: [BIRD, FISH] } }),
  table: () =>
    Promise.resolve({
      ok: true,
      value: {
        regulationId: "example",
        presets: ["max", "max-scarf"],
        tiers: [
          {
            speed: 200,
            entries: [
              { ...BIRD, preset: "max" },
              { ...FISH, preset: "max-scarf" },
            ],
          },
        ],
      },
    }),
  position: () => new Promise(() => undefined),
};

describe("素早さ画面の行の画像", () => {
  test("manifest にある行は <img>(thumb・lazy・alt 空)、無い行はタイプ色エンブレム", async () => {
    const manifest = parsePokemonImageManifest({
      version: 1,
      images: {
        "9001-000": { thumb: "thumb/9001-000.aaaaaaaa.webp", detail: "detail/9001-000.bbbbbbbb.webp" },
      },
    });
    render(
      <PokemonImagesProvider manifest={manifest}>
        <SpeedScreen speedClient={client} />
      </PokemonImagesProvider>,
    );
    const entries = await waitFor(() => {
      const found = screen.getAllByTestId("speed-entry");
      expect(found).toHaveLength(2);
      return found;
    });
    const [bird, fish] = entries as [HTMLElement, HTMLElement];
    const img = bird.querySelector("img");
    expect(img?.getAttribute("src")).toBe("/images/thumb/9001-000.aaaaaaaa.webp");
    expect(img?.getAttribute("loading")).toBe("lazy");
    expect(img?.getAttribute("alt")).toBe("");
    expect(within(bird).queryByTestId("type-emblem")).not.toBeInTheDocument();
    expect(fish.querySelector("img")).toBeNull();
    expect(within(fish).getByTestId("type-emblem")).toBeInTheDocument();
    expect(bird).toHaveTextContent(BIRD.nameJa);
    expect(screen.getByRole("region", { name: speedScreenText.tableRegionLabel })).toBeInTheDocument();
  });
});
