// G-05(ADR-0339): 区切りボタン。ラジオの作法(radiogroup + radio、矢印キーで動かす)。選択中は色だけで示さない。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, test, vi } from "vitest";
import { SegmentedControl, type SegmentedOption } from "./SegmentedControl";

const OPTIONS: readonly SegmentedOption<"up" | "none" | "down">[] = [
  { value: "up", label: "上昇" },
  { value: "none", label: "補正なし" },
  { value: "down", label: "下降" },
];

function Harness({ initial = "none" as "up" | "none" | "down", onChange = vi.fn() }) {
  const [value, setValue] = useState(initial);
  return (
    <SegmentedControl
      label="性格補正"
      options={OPTIONS}
      value={value}
      onChange={(next) => {
        setValue(next);
        onChange(next);
      }}
    />
  );
}

describe("SegmentedControl", () => {
  test("radiogroup に名前があり、選択肢は radio。選択中だけ aria-checked=true", () => {
    render(<Harness />);
    expect(screen.getByRole("radiogroup", { name: "性格補正" })).toBeInTheDocument();
    const radios = screen.getAllByRole("radio");
    expect(radios.map((radio) => radio.textContent)).toEqual(["上昇", "補正なし", "下降"]);
    expect(radios.map((radio) => radio.getAttribute("aria-checked"))).toEqual(["false", "true", "false"]);
  });

  test("選択中は塗りのクラスに加えてチェックの印(色だけに頼らない)が付く", () => {
    render(<Harness />);
    const selected = screen.getByRole("radio", { name: "補正なし" });
    expect(selected).toHaveClass("ui-segmented__option--selected");
    expect(selected.querySelector("svg")).not.toBeNull();
    expect(screen.getByRole("radio", { name: "上昇" }).querySelector("svg")).toBeNull();
  });

  test("tabindex は選択中だけ 0(roving)", () => {
    render(<Harness />);
    expect(screen.getAllByRole("radio").map((radio) => radio.getAttribute("tabindex"))).toEqual([
      "-1",
      "0",
      "-1",
    ]);
  });

  test("矢印キーで動かすと選択とフォーカスが一緒に動く(端で回る)", async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<Harness onChange={onChange} />);
    screen.getByRole("radio", { name: "補正なし" }).focus();
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("radio", { name: "下降" })).toHaveFocus();
    expect(onChange).toHaveBeenLastCalledWith("down");
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("radio", { name: "上昇" })).toHaveFocus();
    await user.keyboard("{ArrowLeft}");
    expect(screen.getByRole("radio", { name: "下降" })).toHaveFocus();
    await user.keyboard("{ArrowUp}");
    expect(screen.getByRole("radio", { name: "補正なし" })).toHaveFocus();
    await user.keyboard("{Home}");
    expect(screen.getByRole("radio", { name: "上昇" })).toHaveFocus();
    await user.keyboard("{End}");
    expect(screen.getByRole("radio", { name: "下降" })).toHaveFocus();
  });

  test("押すと選ぶ", async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<Harness onChange={onChange} />);
    await user.click(screen.getByRole("radio", { name: "上昇" }));
    expect(onChange).toHaveBeenCalledWith("up");
    expect(screen.getByRole("radio", { name: "上昇" })).toHaveAttribute("aria-checked", "true");
  });

  test("disabled の選択肢は選べない・矢印で飛ばす", async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(
      <SegmentedControl
        label="性格補正"
        options={[
          { value: "up", label: "上昇" },
          { value: "none", label: "補正なし", disabled: true },
          { value: "down", label: "下降" },
        ]}
        value="up"
        onChange={onChange}
      />,
    );
    screen.getByRole("radio", { name: "上昇" }).focus();
    await user.keyboard("{ArrowRight}");
    expect(onChange).toHaveBeenCalledWith("down");
    await user.click(screen.getByRole("radio", { name: "補正なし" }));
    expect(onChange).toHaveBeenCalledTimes(1);
  });

  test("選択肢は 2〜4 個だけ", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => undefined);
    expect(() =>
      render(
        <SegmentedControl label="x" options={[{ value: "a", label: "A" }]} value="a" onChange={vi.fn()} />,
      ),
    ).toThrow();
    spy.mockRestore();
  });

  test("アイコンつきの選択肢のアイコンは装飾", () => {
    render(
      <SegmentedControl
        label="単位"
        options={[
          { value: "a", label: "割合", icon: "calc" },
          { value: "b", label: "HP", icon: "speed" },
        ]}
        value="a"
        onChange={vi.fn()}
      />,
    );
    for (const svg of document.querySelectorAll("svg")) {
      expect(svg).toHaveAttribute("aria-hidden", "true");
    }
  });
});
