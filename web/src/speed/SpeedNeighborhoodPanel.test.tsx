// G-04(Web): 「自分の周り」パネルの画面テスト(ADR-0609)。
//   P1 パネルは常時表示(折りたたみ・details なし)で、表のスクロール領域の外にある
//   P2 表をどこまでスクロールしても、パネルの中身は変わらず見える(スクロール不要)
//   P3 自分(「自分」バッジ)を中央に、前後の直近 3 段と、端の合計行が出る
//   P4 同速の段があれば同速の面々を出し、境界の注記は出さない
//   P5 トリックルームでは見出しが「先に動く側/後に動く側」になり、並びが逆になる(ADR-0607)
//   P6 自分が未決定の間は案内の文だけを出し、表の読み直しでも壊れない
//   P7 リスト構造・名前(a11y)。色だけに頼らず矢印・バッジの文字を持つ
// 高さ・仮想スクロールの定数は speedWindow.ts が正(ADR-0608)。ここでは壊さないことも確かめる。

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test } from "vitest";
import { speedScreenText } from "../i18n/ja";
import { SpeedNeighborhood } from "./SpeedNeighborhoodPanel";
import { SpeedScreen } from "./SpeedScreen";
import type { components } from "./speed.gen";
import type { SpeedClient } from "./speedClient";

type Schemas = components["schemas"];

const BIRD: Schemas["SpeedPokemon"] = {
  pokemonId: "9001-000",
  nameJa: "テストカソウドリ",
  types: ["fire", "flying"],
  baseSpeed: 100,
};

const TIER_COUNT = 300;

function makeTable(ascending: boolean): Schemas["TableResponse"] {
  const speeds = Array.from({ length: TIER_COUNT }, (_, i) => 3000 - i * 10);
  if (ascending) {
    speeds.reverse();
  }
  return {
    regulationId: "example",
    presets: ["uninvested", "neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"],
    tiers: speeds.map((speed) => ({ speed, entries: [{ ...BIRD, preset: "max" as const }] })),
  };
}

function makeClient(ownSpeed: number): SpeedClient {
  return {
    pokemon: () => Promise.resolve({ ok: true, value: { regulationId: "example", pokemon: [BIRD] } }),
    table: (_presets, field) => Promise.resolve({ ok: true, value: makeTable(field?.trickRoom === true) }),
    position: () =>
      Promise.resolve({
        ok: true,
        value: { speed: ownSpeed, faster: 0, slower: 0, tie: [], pokemon: BIRD },
      }),
  };
}

async function renderScreen(ownSpeed: number | null) {
  const user = userEvent.setup();
  render(<SpeedScreen speedClient={makeClient(ownSpeed ?? 0)} />);
  await screen.findAllByTestId("speed-tier");
  if (ownSpeed !== null) {
    const select = within(screen.getByRole("region", { name: speedScreenText.selfRegionLabel })).getByRole(
      "combobox",
    );
    await user.selectOptions(select, BIRD.pokemonId);
    await screen.findByTestId("speed-marker");
  }
  return { user };
}

function panel(): HTMLElement {
  return screen.getByRole("region", { name: speedScreenText.neighborhoodRegionLabel });
}

function speedsOf(testId: string): string[] {
  return within(panel())
    .queryAllByTestId(testId)
    .map((el) => el.getAttribute("data-speed") ?? "");
}

describe("P1 常時表示・スクロール領域の外", () => {
  test("自分が決まるとパネルが出る。details(折りたたみ)の中ではない", async () => {
    await renderScreen(2555);
    expect(panel()).toBeVisible();
    expect(panel().closest("details")).toBeNull();
    expect(panel().querySelector("details")).toBeNull();
  });

  test("表のスクロール領域(speed-viewport)の外にある", async () => {
    await renderScreen(2555);
    expect(screen.getByTestId("speed-viewport").contains(panel())).toBe(false);
  });
});

describe("P2 表のスクロール位置に関係なく同じ中身が見える", () => {
  test("表を末尾までスクロールしても、先頭に戻しても、パネルの前後は変わらない", async () => {
    await renderScreen(2555);
    const beforeScroll = speedsOf("neighbor-before");
    expect(beforeScroll.length).toBe(3);

    act(() => {
      fireEvent.scroll(screen.getByTestId("speed-viewport"), { target: { scrollTop: 100000 } });
    });
    expect(panel()).toBeVisible();
    expect(speedsOf("neighbor-before")).toEqual(beforeScroll);

    act(() => {
      fireEvent.scroll(screen.getByTestId("speed-viewport"), { target: { scrollTop: 0 } });
    });
    expect(speedsOf("neighbor-before")).toEqual(beforeScroll);
  });

  test("既存の仮想スクロールは残る(全段は DOM に出ない)", async () => {
    await renderScreen(2555);
    expect(within(screen.getByTestId("speed-viewport")).getAllByTestId("speed-tier").length).toBeLessThan(40);
  });
});

describe("P3 自分を中央に、前後の直近 3 段と合計行", () => {
  test("自分 2555(境界): 速い側 2580,2570,2560 / 遅い側 2550,2540,2530", async () => {
    await renderScreen(2555);
    expect(speedsOf("neighbor-before")).toEqual(["2580", "2570", "2560"]);
    expect(speedsOf("neighbor-after")).toEqual(["2550", "2540", "2530"]);
    const self = within(panel()).getByTestId("neighbor-self");
    expect(self).toHaveTextContent(speedScreenText.neighborhoodSelfBadge);
    expect(self).toHaveTextContent("2555");
    expect(self).toHaveTextContent(speedScreenText.neighborhoodBoundaryLabel);
  });

  test("各段に実数値と代表のポケモン名が出る", async () => {
    await renderScreen(2555);
    const first = within(panel()).getAllByTestId("neighbor-before")[0];
    expect(first).toHaveTextContent("2580");
    expect(first).toHaveTextContent(BIRD.nameJa);
  });

  test("端の合計行: 速い側 45 体・遅い側 255 体(通常の場の語)", async () => {
    await renderScreen(2555);
    expect(within(panel()).getByTestId("neighbor-total-before")).toHaveTextContent(
      speedScreenText.neighborhoodFasterTotal(45),
    );
    expect(within(panel()).getByTestId("neighbor-total-after")).toHaveTextContent(
      speedScreenText.neighborhoodSlowerTotal(255),
    );
  });
});

describe("P4 同速の段", () => {
  test("自分 2550: 自分の段に同速の面々が入り、境界の注記は出ない", async () => {
    await renderScreen(2550);
    const self = within(panel()).getByTestId("neighbor-self");
    expect(self).toHaveTextContent(BIRD.nameJa);
    expect(self).toHaveTextContent(speedScreenText.neighborhoodTieLabel);
    expect(self).not.toHaveTextContent(speedScreenText.neighborhoodBoundaryLabel);
    expect(speedsOf("neighbor-before")).toEqual(["2580", "2570", "2560"]);
    expect(speedsOf("neighbor-after")).toEqual(["2540", "2530", "2520"]);
  });
});

describe("P5 トリックルーム", () => {
  test("見出しが先に動く側/後に動く側になり、先に動く側(遅い実数値)が上に並ぶ", async () => {
    const { user } = await renderScreen(2555);
    await user.click(screen.getByRole("checkbox", { name: speedScreenText.trickRoomLabel }));
    await screen.findAllByTestId("speed-tier");
    expect(
      await within(panel()).findByRole("list", { name: speedScreenText.neighborhoodBeforeListLabel }),
    ).toBeInTheDocument();
    expect(
      within(panel()).getByRole("list", { name: speedScreenText.neighborhoodAfterListLabel }),
    ).toBeInTheDocument();
    expect(
      within(panel()).queryByRole("list", { name: speedScreenText.neighborhoodFasterListLabel }),
    ).toBeNull();
    expect(speedsOf("neighbor-before")).toEqual(["2530", "2540", "2550"]);
    expect(speedsOf("neighbor-after")).toEqual(["2560", "2570", "2580"]);
    expect(within(panel()).getByTestId("neighbor-total-before")).toHaveTextContent(
      speedScreenText.neighborhoodBeforeTotal(255),
    );
  });

  test("通常の場では速い側/遅い側の語", async () => {
    await renderScreen(2555);
    expect(
      within(panel()).getByRole("list", { name: speedScreenText.neighborhoodFasterListLabel }),
    ).toBeInTheDocument();
    expect(
      within(panel()).getByRole("list", { name: speedScreenText.neighborhoodSlowerListLabel }),
    ).toBeInTheDocument();
  });
});

describe("P6 自分が未決定", () => {
  test("案内の文だけが出て、前後の段は出ない", async () => {
    await renderScreen(null);
    expect(panel()).toHaveTextContent(speedScreenText.neighborhoodEmpty);
    expect(within(panel()).queryAllByTestId("neighbor-before")).toHaveLength(0);
    expect(within(panel()).queryByTestId("neighbor-self")).toBeNull();
  });
});

describe("P7 a11y・色だけに頼らない", () => {
  test("見出しがあり、前後は名前付きのリスト。矢印の文字を持つ", async () => {
    await renderScreen(2555);
    expect(
      within(panel()).getByRole("heading", { name: speedScreenText.neighborhoodHeading }),
    ).toBeInTheDocument();
    const faster = within(panel()).getByRole("list", { name: speedScreenText.neighborhoodFasterListLabel });
    expect(within(faster).getAllByRole("listitem").length).toBeGreaterThanOrEqual(3);
    expect(panel()).toHaveTextContent(speedScreenText.neighborhoodUpArrow);
    expect(panel()).toHaveTextContent(speedScreenText.neighborhoodDownArrow);
  });
});

describe("コンポーネント単体(props から)", () => {
  const tiers = [
    { speed: 200, entries: [{ nameJa: "ア" }, { nameJa: "イ" }] },
    { speed: 100, entries: [{ nameJa: "ウ" }] },
  ];

  test("先頭の端: 先に動く側が空のとき「いません」と合計 0 体", () => {
    render(<SpeedNeighborhood tiers={tiers} ownSpeed={300} trickRoom={false} />);
    expect(screen.getByTestId("neighbor-total-before")).toHaveTextContent(
      speedScreenText.neighborhoodFasterTotal(0),
    );
    expect(screen.queryAllByTestId("neighbor-before")).toHaveLength(0);
  });

  test("代表名の残りは「ほか n 体」", () => {
    render(<SpeedNeighborhood tiers={tiers} ownSpeed={150} trickRoom={false} />);
    const row = screen.getByTestId("neighbor-before");
    expect(row).toHaveTextContent("ア");
    expect(row).toHaveTextContent(speedScreenText.neighborhoodMore(1));
  });

  test("ownSpeed が null なら案内だけ", () => {
    render(<SpeedNeighborhood tiers={tiers} ownSpeed={null} trickRoom={false} />);
    expect(screen.getByText(speedScreenText.neighborhoodEmpty)).toBeInTheDocument();
  });
});
