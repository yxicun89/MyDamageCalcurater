// ADR-0357: 能力ランクのメーター。13 段階・矢印キー・タップ / ドラッグ・読み上げ・範囲の端。

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, test, vi } from "vitest";
import { RankMeter, signedRank } from "./RankMeter";

function Harness({ initial = 0, onChange = vi.fn() }: { initial?: number; onChange?: (v: number) => void }) {
  const [value, setValue] = useState(initial);
  return (
    <RankMeter
      label="攻撃側のランク"
      statName="攻撃"
      value={value}
      onChange={(next) => {
        setValue(next);
        onChange(next);
      }}
    />
  );
}

const slider = () => screen.getByRole("slider", { name: "攻撃側のランク" });

describe("RankMeter", () => {
  test("13 段階のスライダー: -6..+6、読み上げは「攻撃 +2」、0 は「攻撃 ±0」", () => {
    const { rerender } = render(
      <RankMeter label="攻撃側のランク" statName="攻撃" value={2} onChange={vi.fn()} />,
    );
    expect(slider()).toHaveAttribute("aria-valuemin", "-6");
    expect(slider()).toHaveAttribute("aria-valuemax", "6");
    expect(slider()).toHaveAttribute("aria-valuenow", "2");
    expect(slider()).toHaveAttribute("aria-valuetext", "攻撃 +2");
    expect(slider().querySelectorAll(".ui-rank__segment")).toHaveLength(13);
    rerender(<RankMeter label="攻撃側のランク" statName="攻撃" value={-3} onChange={vi.fn()} />);
    expect(slider()).toHaveAttribute("aria-valuetext", "攻撃 -3");
    rerender(<RankMeter label="攻撃側のランク" statName="攻撃" value={0} onChange={vi.fn()} />);
    expect(slider()).toHaveAttribute("aria-valuetext", "攻撃 ±0");
  });

  test("段の色: 右の上昇は up、左の下降は down、中央は zero(目印)", () => {
    const states = (value: number) => {
      const { container, unmount } = render(
        <RankMeter label="攻撃側のランク" statName="攻撃" value={value} onChange={vi.fn()} />,
      );
      const result = [...container.querySelectorAll(".ui-rank__segment")].map((node) =>
        node.getAttribute("data-state"),
      );
      unmount();
      return result;
    };
    expect(states(2)).toEqual([
      ...Array<string>(6).fill("off"),
      "zero",
      "up",
      "up",
      "off",
      "off",
      "off",
      "off",
    ]);
    expect(states(-1)).toEqual([
      ...Array<string>(5).fill("off"),
      "down",
      "zero",
      ...Array<string>(6).fill("off"),
    ]);
    expect(states(0).filter((state) => state === "zero")).toHaveLength(1);
  });

  test("矢印キー(左右・上下)で 1 ずつ、PageUp / PageDown で 3、Home / End で両端。端を越えない", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);
    slider().focus();
    await user.keyboard("{ArrowRight}");
    expect(onChange).toHaveBeenLastCalledWith(1);
    await user.keyboard("{ArrowUp}");
    expect(onChange).toHaveBeenLastCalledWith(2);
    await user.keyboard("{ArrowLeft}{ArrowDown}{ArrowDown}");
    expect(onChange).toHaveBeenLastCalledWith(-1);
    await user.keyboard("{PageUp}");
    expect(onChange).toHaveBeenLastCalledWith(2);
    await user.keyboard("{PageDown}{PageDown}");
    expect(onChange).toHaveBeenLastCalledWith(-4);
    await user.keyboard("{End}");
    expect(slider()).toHaveAttribute("aria-valuenow", "6");
    onChange.mockClear();
    await user.keyboard("{ArrowRight}{PageUp}");
    expect(onChange).not.toHaveBeenCalled();
    await user.keyboard("{Home}");
    expect(slider()).toHaveAttribute("aria-valuenow", "-6");
  });

  test("− / + ボタンで 1 ずつ。端では押せない", async () => {
    const user = userEvent.setup();
    render(<Harness initial={5} />);
    await user.click(screen.getByRole("button", { name: "攻撃側のランクを上げる" }));
    expect(slider()).toHaveAttribute("aria-valuenow", "6");
    expect(screen.getByRole("button", { name: "攻撃側のランクを上げる" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "攻撃側のランクを下げる" }));
    expect(slider()).toHaveAttribute("aria-valuenow", "5");
  });

  test("タップとドラッグ: 横位置から最も近い段へ(両端は ±6、中央は 0)", () => {
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);
    // 幅 1300px を 13 段で等分(1 段 = 100px)。
    slider().getBoundingClientRect = () => ({ left: 0, width: 1300 }) as DOMRect;
    fireEvent.pointerDown(slider(), { clientX: 1250, pointerId: 1 });
    expect(onChange).toHaveBeenLastCalledWith(6);
    fireEvent.pointerMove(slider(), { clientX: 650, pointerId: 1 });
    expect(onChange).toHaveBeenLastCalledWith(0);
    fireEvent.pointerMove(slider(), { clientX: 150, pointerId: 1 });
    expect(onChange).toHaveBeenLastCalledWith(-5);
    fireEvent.pointerUp(slider(), { pointerId: 1 });
    onChange.mockClear();
    fireEvent.pointerMove(slider(), { clientX: 1000, pointerId: 1 });
    expect(onChange).not.toHaveBeenCalled();
  });

  test("disabled ではキー・ポインタ・ボタンのどれでも動かない", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<RankMeter label="攻撃側のランク" statName="攻撃" value={0} onChange={onChange} disabled />);
    slider().getBoundingClientRect = () => ({ left: 0, width: 1300 }) as DOMRect;
    fireEvent.keyDown(slider(), { key: "ArrowRight" });
    fireEvent.pointerDown(slider(), { clientX: 1250, pointerId: 1 });
    await user.click(screen.getByRole("button", { name: "攻撃側のランクを上げる" }));
    expect(onChange).not.toHaveBeenCalled();
    expect(slider()).toHaveAttribute("aria-disabled", "true");
  });

  test("マウスの右クリックでは動かない。名前と見える値は上書きできる。誤りの印と説明を付けられる", () => {
    const onChange = vi.fn();
    render(
      <RankMeter
        label="防御側のランク"
        statName="防御"
        value={1}
        valueLabel="B +1"
        decreaseLabel="下へ"
        increaseLabel="上へ"
        invalid
        describedBy="err"
        onChange={onChange}
      />,
    );
    const meter = screen.getByRole("slider", { name: "防御側のランク" });
    meter.getBoundingClientRect = () => ({ left: 0, width: 1300 }) as DOMRect;
    fireEvent.pointerDown(meter, { clientX: 1250, pointerId: 1, pointerType: "mouse", button: 2 });
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByText("B +1")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "下へ" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "上へ" })).toBeInTheDocument();
    expect(meter).toHaveAttribute("aria-invalid", "true");
    expect(meter).toHaveAttribute("aria-describedby", "err");
  });

  test("signedRank の表記", () => {
    expect([signedRank(3), signedRank(-2), signedRank(0)]).toEqual(["+3", "-2", "±0"]);
  });
});
