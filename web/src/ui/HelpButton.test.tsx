// G-05(ADR-0339): 説明ボタン。情報のアイコン + 「説明」。開くのは 1 段だけ(開いたものを閉じるのも同じボタンと Esc)。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test } from "vitest";
import { HelpButton } from "./HelpButton";

describe("HelpButton", () => {
  test("名前は「説明」。閉じているあいだは本文が出ず、aria-expanded=false", () => {
    render(<HelpButton>SP は 0〜32 です。</HelpButton>);
    const button = screen.getByRole("button", { name: "説明" });
    expect(button).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("SP は 0〜32 です。")).toBeNull();
    expect(button.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
  });

  test("押すと本文が開き、aria-controls でつながる。もう一度押すと閉じる", async () => {
    const user = userEvent.setup();
    render(<HelpButton>SP は 0〜32 です。</HelpButton>);
    const button = screen.getByRole("button", { name: "説明" });
    await user.click(button);
    expect(button).toHaveAttribute("aria-expanded", "true");
    const body = screen.getByText("SP は 0〜32 です。");
    expect(button.getAttribute("aria-controls")).toBe(body.closest("[id]")?.id);
    await user.click(button);
    expect(screen.queryByText("SP は 0〜32 です。")).toBeNull();
  });

  test("Esc で閉じて、ボタンにフォーカスが残る", async () => {
    const user = userEvent.setup();
    render(<HelpButton>本文</HelpButton>);
    const button = screen.getByRole("button", { name: "説明" });
    await user.click(button);
    await user.keyboard("{Escape}");
    expect(screen.queryByText("本文")).toBeNull();
    expect(button).toHaveFocus();
  });

  test("label で名前を変えられる(見える文字が名前)", () => {
    render(<HelpButton label="SPとは">本文</HelpButton>);
    expect(screen.getByRole("button", { name: "SPとは" })).toBeInTheDocument();
  });
});
