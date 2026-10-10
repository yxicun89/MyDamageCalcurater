// G-06(ADR-0343): 一覧・選択肢の小さなポケモン表示。manifest にあれば <img>(thumb・lazy・alt 空・寸法つき)、
// 無い・manifest 無し・読み込み失敗ならタイプ色のエンブレム。console にエラーを出さない。

import { fireEvent, render } from "@testing-library/react";
import { describe, expect, test, vi } from "vitest";
import { PokemonIcon } from "./PokemonIcon";
import { PokemonImagesProvider } from "./PokemonImagesContext";
import { parsePokemonImageManifest } from "./pokemonImages";

const MANIFEST = parsePokemonImageManifest({
  version: 1,
  images: { "9001-000": { thumb: "thumb/9001-000.aaaaaaaa.webp", detail: "detail/9001-000.bbbbbbbb.webp" } },
});

describe("PokemonIcon", () => {
  test("manifest にあれば thumb の <img>(lazy・alt 空・width/height つき)で、エンブレムは出さない", () => {
    const { container } = render(
      <PokemonImagesProvider manifest={MANIFEST}>
        <PokemonIcon speciesKey="9001-000" typeId="fire" />
      </PokemonImagesProvider>,
    );
    const img = container.querySelector("img");
    expect(img?.getAttribute("src")).toBe("/images/thumb/9001-000.aaaaaaaa.webp");
    expect(img?.getAttribute("loading")).toBe("lazy");
    expect(img?.getAttribute("alt")).toBe("");
    expect(img?.getAttribute("width")).not.toBeNull();
    expect(img?.getAttribute("height")).not.toBeNull();
    expect(img?.classList.contains("pokemon-icon")).toBe(true);
    expect(container.querySelector("[data-testid='pokemon-icon-emblem']")).toBeNull();
  });

  test("Provider が無い・キーが無い・manifest が null のときはタイプ色のエンブレム", () => {
    const bare = render(<PokemonIcon speciesKey="9001-000" typeId="fire" />);
    const emblem = bare.getByTestId("pokemon-icon-emblem");
    expect(emblem.style.backgroundColor).toBe("var(--type-fire, var(--text-secondary))");
    bare.unmount();

    const noKey = render(
      <PokemonImagesProvider manifest={MANIFEST}>
        <PokemonIcon speciesKey="9999-000" typeId="water" />
      </PokemonImagesProvider>,
    );
    expect(noKey.container.querySelector("img")).toBeNull();
    expect(noKey.getByTestId("pokemon-icon-emblem")).toBeInTheDocument();
    noKey.unmount();

    const none = render(
      <PokemonImagesProvider manifest={null}>
        <PokemonIcon speciesKey="9001-000" />
      </PokemonImagesProvider>,
    );
    // タイプが分からなくても中立色のエンブレムで成立する。
    expect(none.getByTestId("pokemon-icon-emblem").style.backgroundColor).toBe("var(--text-secondary)");
  });

  test("不正なタイプ ID は CSS に埋め込まず中立色にする", () => {
    const { getByTestId } = render(<PokemonIcon speciesKey="9001-000" typeId="fire); x:(" />);
    expect(getByTestId("pokemon-icon-emblem").style.backgroundColor).toBe("var(--text-secondary)");
  });

  test("読み込み失敗(404 等)でエンブレムに切り替わり、console にエラーを出さない", () => {
    const error = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const { container, getByTestId } = render(
      <PokemonImagesProvider manifest={MANIFEST}>
        <PokemonIcon speciesKey="9001-000" typeId="fire" />
      </PokemonImagesProvider>,
    );
    const img = container.querySelector("img");
    expect(img).not.toBeNull();
    if (img !== null) {
      fireEvent.error(img);
    }
    expect(container.querySelector("img")).toBeNull();
    expect(getByTestId("pokemon-icon-emblem")).toBeInTheDocument();
    expect(error).not.toHaveBeenCalled();
    error.mockRestore();
  });
});
