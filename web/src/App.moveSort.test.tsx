// I-web-9 = F-02(ADR-0335 §4・ADR-0308): 技の並びは計算タブと逆算タブで共有し、タブを往復しても保たれる。
// どちらのタブで変えても、もう一方のタブ(すでに mount 済みで隠れているほう)に反映される。
// 例データの 9001-000 テストほのお(習得順 たいあたり・かえんパンチ → 五十音順 かえんパンチ・たいあたり)を使う。

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, test } from "vitest";
import { App } from "./App";
import { moveSortText } from "./i18n/moveSort";
import { createFakeEngine } from "./test/fakeEngine";

beforeEach(() => {
  window.history.replaceState(null, "", "/");
  localStorage.clear();
  sessionStorage.clear();
});
afterEach(() => {
  window.history.replaceState(null, "", "/");
  localStorage.clear();
  sessionStorage.clear();
});

const sortRadio = (label: string) =>
  within(screen.getByRole("radiogroup", { name: moveSortText.groupLabel })).getByRole("radio", {
    name: label,
  });

describe("タブをまたぐ技の並び", () => {
  test("計算タブで五十音順にして逆算タブへ行くと、逆算タブも五十音順で、戻っても保たれる", async () => {
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
    await user.click(sortRadio(moveSortText.options.kana));

    await user.click(screen.getByRole("tab", { name: "逆算" }));
    await screen.findByRole("combobox", { name: "自分のポケモン" });
    expect(sortRadio(moveSortText.options.kana)).toBeChecked();

    await user.click(screen.getByRole("tab", { name: "計算" }));
    expect(sortRadio(moveSortText.options.kana)).toBeChecked();
  });

  test("逆算タブで変えた並びが、mount 済みの計算タブにも反映される", async () => {
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
    await user.click(screen.getByRole("tab", { name: "逆算" }));
    await screen.findByRole("combobox", { name: "自分のポケモン" });
    await user.click(sortRadio(moveSortText.options.type));

    await user.click(screen.getByRole("tab", { name: "計算" }));
    expect(sortRadio(moveSortText.options.type)).toBeChecked();
  });
});
