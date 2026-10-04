// I-web-4(ADR-0328): 逆算画面でも、メガストーンの表示名はマスタの nameJa(日本語として使えるとき)。
// 英語名・空のときだけ「{基本種名}のメガストーン」。自分側の持ち物欄の固定表示と、相手側の「持ち物: …」の行の両方。

import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeAll, describe, expect, test, vi } from "vitest";
import { OBSERVATION_INPUT_DEBOUNCE_MS } from "../domain/observations";
import { megaItemText, reverseScreenText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import { createFakeEngine } from "../test/fakeEngine";
import { MEGA_FIRE, MEGA_FIRE_STONE, withMegaFixture } from "../test/megaMaster";
import { ReverseScreen } from "./ReverseScreen";

let withStone: (name: string) => MasterData;

beforeAll(async () => {
  const example = await exampleMasterSource.load();
  withStone = (name) => {
    const data = withMegaFixture(example);
    return {
      ...data,
      items: data.items.map((item) => (item.id === MEGA_FIRE_STONE.id ? { ...item, nameJa: name } : item)),
    };
  };
});

const myCard = () => screen.getByRole("region", { name: "自分のポケモン" });
const theirCard = () => screen.getByRole("region", { name: "相手のポケモン" });

async function pickMega(master: MasterData) {
  const user = userEvent.setup();
  render(<ReverseScreen engine={createFakeEngine()} master={master} />);
  await user.selectOptions(screen.getByRole("combobox", { name: "自分のポケモン" }), MEGA_FIRE.key);
  await user.selectOptions(screen.getByRole("combobox", { name: "相手のポケモン" }), MEGA_FIRE.key);
  return within(myCard()).getByRole("combobox", { name: "自分の持ち物" });
}

describe("メガ種族の固定表示", () => {
  test("nameJa が日本語(全角Ｘ含む)ならそのまま(自分の持ち物欄・相手の「持ち物: …」)", async () => {
    const select = await pickMega(withStone("リザードナイトＸ"));
    expect(select).toHaveDisplayValue("リザードナイトＸ");
    expect(within(theirCard()).getByText(megaItemText.fixedItemName("リザードナイトＸ"))).toBeVisible();
  });

  test("英語名ならフォールバック(自分の持ち物欄・相手の「持ち物: …」)", async () => {
    const select = await pickMega(withStone("Barbaracite"));
    expect(select).toHaveDisplayValue("テストほのおのメガストーン");
    expect(
      within(theirCard()).getByText(megaItemText.fixedItemName("テストほのおのメガストーン")),
    ).toBeVisible();
    expect(screen.queryByText(/Barbaracite/)).toBeNull();
  });
});

afterEach(() => {
  vi.useRealTimers();
});

// ADR-0326 §4(ADR-0328 でも保つ): 英語名のストーンは逆算の候補の行にも出ない。
describe("英語名のメガストーンを画面に出さない", () => {
  test("相手がメガ種族なら、候補の行の持ち物は「{基本種名}のメガストーン」で、英語名はどこにも出ない", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
    render(<ReverseScreen engine={createFakeEngine()} master={withStone("Barbaracite")} />);
    await user.selectOptions(screen.getByRole("combobox", { name: "自分のポケモン" }), "9001-000");
    await user.selectOptions(screen.getByRole("combobox", { name: "相手のポケモン" }), MEGA_FIRE.key);
    await user.type(screen.getByRole("textbox", { name: "観測1" }), "45");
    act(() => {
      vi.advanceTimersByTime(OBSERVATION_INPUT_DEBOUNCE_MS);
    });

    const list = await screen.findByRole("list", { name: reverseScreenText.resultsListLabel });
    const rows = within(list).getAllByRole("listitem");
    expect(rows.length).toBeGreaterThan(0);
    for (const row of rows) {
      expect(within(row).getByText("テストほのおのメガストーン")).toBeVisible();
    }
    expect(document.body).not.toHaveTextContent("Barbaracite");
  });
});
