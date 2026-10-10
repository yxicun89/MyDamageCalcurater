// G-05(ADR-0339): 増減ボタン(SP 0〜32 の − / + と数値欄)。数値欄は残す。名前は用語集の語。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, test, vi } from "vitest";
import { Stepper } from "./Stepper";

function Harness({ initial = 10, onChange = vi.fn() }: { initial?: number; onChange?: (v: number) => void }) {
  const [value, setValue] = useState(initial);
  return (
    <Stepper
      label="攻撃の能力ポイント"
      value={value}
      min={0}
      max={32}
      onChange={(next) => {
        setValue(next);
        onChange(next);
      }}
    />
  );
}

describe("Stepper", () => {
  test("group の名前・数値欄・− / + に読み上げ用の名前がある", () => {
    render(<Harness />);
    expect(screen.getByRole("group", { name: "攻撃の能力ポイント" })).toBeInTheDocument();
    expect(screen.getByRole("spinbutton", { name: "攻撃の能力ポイント" })).toHaveValue(10);
    expect(screen.getByRole("button", { name: "攻撃の能力ポイントを減らす" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "攻撃の能力ポイントを増やす" })).toBeInTheDocument();
  });

  test("数値欄に min / max / step を渡す", () => {
    render(<Harness />);
    const input = screen.getByRole("spinbutton");
    expect(input).toHaveAttribute("min", "0");
    expect(input).toHaveAttribute("max", "32");
    expect(input).toHaveAttribute("step", "1");
  });

  test("+ / − で 1 ずつ動く", async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<Harness onChange={onChange} />);
    await user.click(screen.getByRole("button", { name: "攻撃の能力ポイントを増やす" }));
    expect(onChange).toHaveBeenLastCalledWith(11);
    await user.click(screen.getByRole("button", { name: "攻撃の能力ポイントを減らす" }));
    await user.click(screen.getByRole("button", { name: "攻撃の能力ポイントを減らす" }));
    expect(onChange).toHaveBeenLastCalledWith(9);
  });

  test("上限・下限では aria-disabled で、押しても通知せずフォーカスは残る", async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    const { rerender } = render(<Stepper label="SP" value={32} min={0} max={32} onChange={onChange} />);
    const plus = screen.getByRole("button", { name: "SPを増やす" });
    expect(plus).toHaveAttribute("aria-disabled", "true");
    expect(plus).toBeEnabled();
    expect(screen.getByRole("button", { name: "SPを減らす" })).not.toHaveAttribute("aria-disabled");
    await user.click(plus);
    expect(onChange).not.toHaveBeenCalled();
    expect(plus).toHaveFocus();
    rerender(<Stepper label="SP" value={0} min={0} max={32} onChange={onChange} />);
    expect(screen.getByRole("button", { name: "SPを減らす" })).toHaveAttribute("aria-disabled", "true");
  });

  test("押して端に着いても、押したボタンのフォーカスは残る", async () => {
    const user = userEvent.setup();
    render(<Harness initial={31} />);
    const plus = screen.getByRole("button", { name: "攻撃の能力ポイントを増やす" });
    await user.click(plus);
    expect(plus).toHaveFocus();
    expect(plus).toHaveAttribute("aria-disabled", "true");
  });

  test("範囲に入る数は打ちながら通知し、範囲外は blur で範囲に収める。空のまま離れると元の値に戻す", async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<Harness onChange={onChange} />);
    const input = screen.getByRole("spinbutton");
    await user.clear(input);
    expect(onChange).not.toHaveBeenCalled();
    await user.type(input, "99");
    expect(onChange).toHaveBeenLastCalledWith(9);
    expect(input).toHaveValue(99);
    await user.tab();
    expect(onChange).toHaveBeenLastCalledWith(32);
    expect(input).toHaveValue(32);
    await user.clear(input);
    await user.tab();
    expect(input).toHaveValue(32);
  });

  test("Enter でも範囲に収める", async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<Harness onChange={onChange} />);
    const input = screen.getByRole("spinbutton");
    await user.clear(input);
    await user.type(input, "50{Enter}");
    expect(onChange).toHaveBeenLastCalledWith(32);
    expect(input).toHaveValue(32);
  });

  test("min が 0 でない欄でも、途中の桁(範囲外)を打てる", async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<Stepper label="実数値" value={100} min={100} max={200} onChange={onChange} />);
    const input = screen.getByRole("spinbutton");
    await user.clear(input);
    await user.type(input, "15");
    expect(input).toHaveValue(15);
    expect(onChange).not.toHaveBeenCalled();
    await user.type(input, "0");
    expect(onChange).toHaveBeenLastCalledWith(150);
  });

  test("親が value を外から変えたら、打っている途中の文字を捨てて新しい値を出す", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<Stepper label="SP" value={10} min={0} max={32} onChange={vi.fn()} />);
    const input = screen.getByRole("spinbutton");
    await user.clear(input);
    await user.type(input, "99");
    expect(input).toHaveValue(99);
    rerender(<Stepper label="SP" value={20} min={0} max={32} onChange={vi.fn()} />);
    expect(input).toHaveValue(20);
  });

  test("disabled なら全部押せない", () => {
    render(<Stepper label="SP" value={5} min={0} max={32} disabled onChange={vi.fn()} />);
    expect(screen.getByRole("spinbutton")).toBeDisabled();
    expect(screen.getByRole("button", { name: "SPを増やす" })).toBeDisabled();
  });

  test("アイコンは装飾で、名前は文字(− / + の記号は隠す)", () => {
    render(<Harness />);
    for (const svg of document.querySelectorAll("svg")) {
      expect(svg).toHaveAttribute("aria-hidden", "true");
    }
  });
});
