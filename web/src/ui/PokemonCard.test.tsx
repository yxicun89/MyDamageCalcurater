// G-05(ADR-0339): ポケモンカード。画像かエンブレム + 名前 + タイプバッジ + 持ち物の印。押すとピッカーが開く(onClick)。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test, vi } from "vitest";
import { PokemonCard } from "./PokemonCard";

describe("PokemonCard", () => {
  test("名前・タイプバッジ(タイプ名の文字つき)・持ち物を出し、画像が無ければエンブレムで成立する", () => {
    render(<PokemonCard speciesKey="25-000" name="ピカチュウ" types={["electric"]} itemName="でんきだま" />);
    expect(screen.getByText("ピカチュウ")).toBeInTheDocument();
    expect(screen.getByText("でんき")).toHaveClass("ui-badge");
    expect(screen.getByText("でんきだま")).toBeInTheDocument();
    expect(screen.getByTestId("pokemon-icon-emblem")).toBeInTheDocument();
  });

  test("onClick が無ければ押せない(カード = div)。ある場合は button で、中の文字が名前になる", async () => {
    const { rerender } = render(<PokemonCard speciesKey="25-000" name="ピカチュウ" types={[]} />);
    expect(screen.queryByRole("button")).toBeNull();
    const onClick = vi.fn();
    rerender(<PokemonCard speciesKey="25-000" name="ピカチュウ" types={["electric"]} onClick={onClick} />);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: /ピカチュウ/ }));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  test("持ち物が無ければ持ち物の印は出さない。small は寸法違いのクラス", () => {
    const { container } = render(<PokemonCard speciesKey="25-000" name="ピカチュウ" types={[]} small />);
    expect(container.querySelector(".ui-pokemon-card__item")).toBeNull();
    expect(container.firstElementChild).toHaveClass("ui-pokemon-card", "ui-pokemon-card--small");
  });

  test("selected は aria-pressed ではなく印で示す(カード自体は選択状態を持たない)", () => {
    render(<PokemonCard speciesKey="25-000" name="ピカチュウ" types={[]} onClick={vi.fn()} />);
    expect(screen.getByRole("button")).not.toHaveAttribute("aria-pressed");
  });

  test("画像は装飾(alt 空)で、アイコンは名前に入らない", () => {
    render(<PokemonCard speciesKey="25-000" name="ピカチュウ" types={["electric"]} onClick={vi.fn()} />);
    expect(screen.getByRole("button").textContent).toBe("ピカチュウでんき");
  });
});
