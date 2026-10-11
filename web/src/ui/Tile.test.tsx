// G-05(ADR-0339): タイル(押せる四角)。選べる状態は aria-pressed + 印、空き枠は「+」と文字。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test, vi } from "vitest";
import { Tile } from "./Tile";

describe("Tile", () => {
  test("押せるボタンで、中身が名前になる", async () => {
    const onClick = vi.fn();
    const user = userEvent.setup();
    render(<Tile onClick={onClick}>ピカチュウ</Tile>);
    await user.click(screen.getByRole("button", { name: "ピカチュウ" }));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  test("selected を渡すと aria-pressed と印(チェック)が付く。渡さなければ aria-pressed なし", () => {
    const { rerender } = render(<Tile onClick={vi.fn()}>A</Tile>);
    expect(screen.getByRole("button")).not.toHaveAttribute("aria-pressed");
    rerender(
      <Tile onClick={vi.fn()} selected>
        A
      </Tile>,
    );
    const button = screen.getByRole("button", { name: "A" });
    expect(button).toHaveAttribute("aria-pressed", "true");
    expect(button).toHaveClass("ui-tile--selected");
    expect(button.querySelector("svg")).not.toBeNull();
    rerender(
      <Tile onClick={vi.fn()} selected={false}>
        A
      </Tile>,
    );
    expect(screen.getByRole("button")).toHaveAttribute("aria-pressed", "false");
  });

  test("空き枠(empty)は「+」のアイコンと見える名前を持つ", () => {
    render(<Tile onClick={vi.fn()} empty label="追加" />);
    const button = screen.getByRole("button", { name: "追加" });
    expect(button).toHaveClass("ui-tile", "ui-tile--empty");
    expect(button.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
  });

  test("disabled", () => {
    render(
      <Tile onClick={vi.fn()} disabled>
        A
      </Tile>,
    );
    expect(screen.getByRole("button")).toBeDisabled();
  });
});
