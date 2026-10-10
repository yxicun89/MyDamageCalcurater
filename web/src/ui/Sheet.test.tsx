// G-05(ADR-0339): シート(狭い幅は下から出るシート・広い幅は面)。「このアプリについて」の確認ダイアログと同じ作法:
// 開いたら中へフォーカス・Esc で閉じる・Tab は中で回る・閉じたら開いたボタンへ戻す。

import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { afterEach, describe, expect, test, vi } from "vitest";
import { fakeMatchMedia } from "../test/matchMedia";
import { Sheet } from "./Sheet";

function Harness({ onClose = vi.fn() }: { onClose?: () => void }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button
        type="button"
        onClick={() => {
          setOpen(true);
        }}
      >
        技
      </button>
      <Sheet
        open={open}
        title="技を選ぶ"
        onClose={() => {
          setOpen(false);
          onClose();
        }}
      >
        <input aria-label="検索" />
        <button type="button">でんじほう</button>
      </Sheet>
    </>
  );
}

describe("Sheet", () => {
  test("閉じているあいだは何も出ない", () => {
    render(<Harness />);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  test("開くと dialog(aria-modal・名前つき)が出て、中の最初の入力へフォーカスが移る", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "技" }));
    const dialog = screen.getByRole("dialog", { name: "技を選ぶ" });
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(screen.getByRole("textbox", { name: "検索" })).toHaveFocus();
  });

  test("閉じるボタンはアイコンだけで名前「閉じる」を持ち、押すと閉じて開いたボタンへフォーカスが戻る", async () => {
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<Harness onClose={onClose} />);
    await user.click(screen.getByRole("button", { name: "技" }));
    await user.click(screen.getByRole("button", { name: "閉じる" }));
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByRole("button", { name: "技" })).toHaveFocus();
  });

  test("Esc で閉じて、開いたボタンへ戻る", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "技" }));
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByRole("button", { name: "技" })).toHaveFocus();
  });

  test("Tab / Shift+Tab は中で回る(外へ出ない)", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "技" }));
    // 順: 閉じる → 検索 → でんじほう → (回って)閉じる
    expect(screen.getByRole("textbox", { name: "検索" })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { name: "でんじほう" })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { name: "閉じる" })).toHaveFocus();
    await user.tab({ shift: true });
    expect(screen.getByRole("button", { name: "でんじほう" })).toHaveFocus();
  });

  test("幕(scrim)を押すと閉じる。シートの中を押しても閉じない", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "技" }));
    await user.click(screen.getByRole("dialog"));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    const scrim = document.querySelector(".ui-sheet__scrim");
    expect(scrim).not.toBeNull();
    await user.click(scrim as Element);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  test("見出しは dialog の名前になる(aria-labelledby)", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "技" }));
    expect(screen.getByRole("heading", { name: "技を選ぶ" })).toBeInTheDocument();
  });

  describe("出入りの動き(操作のときだけ。時間は --duration-sheet)", () => {
    afterEach(() => {
      vi.useRealTimers();
      vi.unstubAllGlobals();
      document.documentElement.style.removeProperty("--duration-sheet");
    });

    test("開くと、次のフレームで is-shown が付く(transition の行き先)", async () => {
      const user = userEvent.setup();
      render(<Harness />);
      await user.click(screen.getByRole("button", { name: "技" }));
      await vi.waitFor(() => {
        expect(document.querySelector(".ui-sheet__scrim")).toHaveClass("is-shown");
      });
    });

    test("時間が 0 でないとき、閉じても出入りの時間だけ DOM に残ってから消える", async () => {
      document.documentElement.style.setProperty("--duration-sheet", "0.25s");
      const user = userEvent.setup();
      render(<Harness />);
      await user.click(screen.getByRole("button", { name: "技" }));
      vi.useFakeTimers({ shouldAdvanceTime: true });
      await user.keyboard("{Escape}");
      expect(screen.queryByRole("dialog")).not.toBeNull();
      expect(document.querySelector(".ui-sheet__scrim")).not.toHaveClass("is-shown");
      await act(async () => {
        await vi.advanceTimersByTimeAsync(300);
      });
      expect(screen.queryByRole("dialog")).toBeNull();
    });

    test("「視差効果を減らす」では待たずにすぐ消える", async () => {
      document.documentElement.style.setProperty("--duration-sheet", "0.25s");
      vi.stubGlobal("matchMedia", fakeMatchMedia(true));
      const user = userEvent.setup();
      render(<Harness />);
      await user.click(screen.getByRole("button", { name: "技" }));
      await user.keyboard("{Escape}");
      expect(screen.queryByRole("dialog")).toBeNull();
    });
  });

  describe("モーダルとしての作法", () => {
    test("開いている間は body のスクロールを止め、背景の兄弟に inert を付け、閉じたら戻す", async () => {
      document.body.style.overflow = "scroll";
      const user = userEvent.setup();
      const { container } = render(<Harness />);
      await user.click(screen.getByRole("button", { name: "技" }));
      expect(document.body.style.overflow).toBe("hidden");
      expect(container).toHaveAttribute("inert");
      await user.keyboard("{Escape}");
      expect(document.body.style.overflow).toBe("scroll");
      expect(container).not.toHaveAttribute("inert");
      document.body.style.overflow = "";
    });

    test("シートが重なっても、一番上だけ閉じたあいだは止めたまま、全部閉じたら戻る。Esc は一番上だけ閉じる", async () => {
      function Two() {
        const [a, setA] = useState(true);
        const [b, setB] = useState(true);
        return (
          <>
            <Sheet
              open={a}
              title="下"
              onClose={() => {
                setA(false);
              }}
            >
              <button type="button">下の中</button>
            </Sheet>
            <Sheet
              open={b}
              title="上"
              onClose={() => {
                setB(false);
              }}
            >
              <button type="button">上の中</button>
            </Sheet>
          </>
        );
      }
      const user = userEvent.setup();
      render(<Two />);
      expect(document.body.style.overflow).toBe("hidden");
      await user.keyboard("{Escape}");
      expect(screen.queryByRole("dialog", { name: "上" })).toBeNull();
      expect(screen.getByRole("dialog", { name: "下" })).toBeInTheDocument();
      expect(document.body.style.overflow).toBe("hidden");
      await user.keyboard("{Escape}");
      expect(screen.queryByRole("dialog")).toBeNull();
      expect(document.body.style.overflow).toBe("");
      expect(document.querySelector("[inert]")).toBeNull();
    });

    test("パネルの中で押して幕の上で離しても閉じない。幕の上で押して離したときは閉じる", async () => {
      const user = userEvent.setup();
      render(<Harness />);
      await user.click(screen.getByRole("button", { name: "技" }));
      const scrim = document.querySelector(".ui-sheet__scrim") as Element;
      await user.pointer([
        { keys: "[MouseLeft>]", target: screen.getByRole("dialog") },
        { keys: "[/MouseLeft]", target: scrim },
      ]);
      expect(screen.getByRole("dialog")).toBeInTheDocument();
      await user.pointer([{ keys: "[MouseLeft]", target: scrim }]);
      expect(screen.queryByRole("dialog")).toBeNull();
    });

    test("フォーカスが外へ出ていても Esc で閉じる", async () => {
      const user = userEvent.setup();
      render(<Harness />);
      await user.click(screen.getByRole("button", { name: "技" }));
      (document.activeElement as HTMLElement).blur();
      await user.keyboard("{Escape}");
      expect(screen.queryByRole("dialog")).toBeNull();
    });
  });
});
