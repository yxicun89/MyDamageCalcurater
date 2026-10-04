// F-06(Web): 素早さ表の仮想スクロールと、右の位置マーカーの高さ揃え(ADR-0608)。
// 確かめること:
//   V1 段が多くても DOM に出す段は周辺だけ(全行を出さない)。a11y: aria-setsize(総数)・aria-posinset(元の順)
//   V2 スクロールすると表示する段が入れ替わる(総数は変わらない)
//   V3 自分の位置マーカーは、左の該当行(同速の段・境界)と同じ高さに置かれ、スクロールに追従する
//   V4 見えない位置にいるときは端に張り付き(data-state)、「自分の位置へ移動」ボタンで中央に寄せられる
//   V5 トリックルーム(昇順の表)でも成立する
//   V6 スクロール領域はキーボードで操作でき(tabindex=0)、名前を持つ
// 高さの定数(ENTRY_HEIGHT 等)は speedWindow.ts が正。jsdom はレイアウトを持たないので、
// ビューポートの高さは仮の値(DEFAULT_VIEWPORT_HEIGHT)になる。

import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test } from "vitest";
import { speedScreenText } from "../i18n/ja";
import { SpeedScreen } from "./SpeedScreen";
import type { components } from "./speed.gen";
import type { SpeedClient } from "./speedClient";
import {
  DEFAULT_VIEWPORT_HEIGHT,
  ROW_GAP,
  BOUNDARY_SLOT_HEIGHT,
  scrollTopToReveal,
  tierSlotHeight,
} from "./speedWindow";

type Schemas = components["schemas"];

const BIRD: Schemas["SpeedPokemon"] = {
  pokemonId: "9001-000",
  nameJa: "テストカソウドリ",
  types: ["fire", "flying"],
  baseSpeed: 100,
};

const TIER_COUNT = 300;

/** 1行の段を TIER_COUNT 個。降順(速い順)は 3000, 2990, ...、昇順(トリックルーム)はその逆。 */
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

function viewport(): HTMLElement {
  return screen.getByTestId("speed-viewport");
}

function renderedTiers(): HTMLElement[] {
  return within(viewport()).queryAllByTestId("speed-tier");
}

/** 表が出るまで待つ。 */
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

function scrollTo(top: number): void {
  act(() => {
    fireEvent.scroll(viewport(), { target: { scrollTop: top } });
  });
}

describe("V1 周辺の段だけ描画する", () => {
  test("300段のうち DOM に出るのは周辺だけで、先頭から始まる", async () => {
    await renderScreen(null);
    const rows = renderedTiers();
    expect(rows.length).toBeGreaterThan(0);
    expect(rows.length).toBeLessThan(40);
    expect(rows[0]).toHaveAttribute("data-speed", "3000");
  });

  test("各段が総数(aria-setsize)と元の順(aria-posinset)を持つ", async () => {
    await renderScreen(null);
    const rows = renderedTiers();
    rows.forEach((row, index) => {
      expect(row).toHaveAttribute("aria-setsize", String(TIER_COUNT));
      expect(row).toHaveAttribute("aria-posinset", String(index + 1));
    });
  });

  test("リストの意味(ul/li)を保つ", async () => {
    await renderScreen(null);
    expect(
      within(viewport()).getByRole("list", { name: speedScreenText.tiersListLabel }),
    ).toBeInTheDocument();
    expect(within(viewport()).getAllByRole("listitem").length).toBeGreaterThan(0);
  });
});

describe("V2 スクロールで表示する段が入れ替わる", () => {
  test("途中までスクロールすると、先頭の段は消え、その位置の段が出る。総数は変わらない", async () => {
    await renderScreen(null);
    const slot = tierSlotHeight(1);
    scrollTo(slot * 150);
    const rows = renderedTiers();
    expect(rows.length).toBeLessThan(40);
    const speeds = rows.map((row) => row.getAttribute("data-speed"));
    expect(speeds).not.toContain("3000");
    expect(speeds).toContain(String(3000 - 150 * 10));
    const posinsets = rows.map((row) => Number(row.getAttribute("aria-posinset")));
    expect(posinsets).toContain(151);
    expect(rows[0]).toHaveAttribute("aria-setsize", String(TIER_COUNT));
  });

  test("末尾までスクロールすると最後の段が出る", async () => {
    await renderScreen(null);
    scrollTo(tierSlotHeight(1) * TIER_COUNT);
    const speeds = renderedTiers().map((row) => row.getAttribute("data-speed"));
    expect(speeds).toContain(String(3000 - (TIER_COUNT - 1) * 10));
  });
});

describe("V3 位置マーカーは左の該当行と同じ高さ", () => {
  // 自分 2555: 降順で 2560(i=44)と 2550(i=45)の間 → 境界は tier 45 の前(top = 45 * 段の高さ)。
  const OWN = 2555;
  const anchorTop = 45 * tierSlotHeight(1);

  test("自分が決まると右のマーカーが出る(位置を出す前は出ない)", async () => {
    await renderScreen(null);
    expect(screen.queryByTestId("speed-marker")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: speedScreenText.jumpToSelfLabel })).not.toBeInTheDocument();
  });

  test("スクロール位置に応じて、左の境界の行と同じ高さにマーカーが置かれる", async () => {
    await renderScreen(OWN);
    scrollTo(anchorTop - 100);
    const marker = screen.getByTestId("speed-marker");
    expect(marker).toHaveAttribute("data-state", "visible");
    expect(marker.style.top).toBe("100px");
    expect(marker.style.height).toBe(`${String(BOUNDARY_SLOT_HEIGHT - ROW_GAP)}px`);
    // 左には境界の行が描画されている
    expect(within(viewport()).getByTestId("speed-boundary")).toBeInTheDocument();

    scrollTo(anchorTop - 60);
    expect(screen.getByTestId("speed-marker").style.top).toBe("60px");
  });

  test("同速の段があれば、その段の行と同じ高さ・同じ高さの箱になる", async () => {
    await renderScreen(2550);
    const tieTop = 45 * tierSlotHeight(1);
    scrollTo(tieTop - 80);
    const marker = screen.getByTestId("speed-marker");
    expect(marker.style.top).toBe("80px");
    expect(marker.style.height).toBe(`${String(tierSlotHeight(1) - ROW_GAP)}px`);
    expect(within(viewport()).queryByTestId("speed-boundary")).not.toBeInTheDocument();
    expect(viewport().querySelector('[data-self="tie"]')).toHaveAttribute("data-speed", "2550");
  });
});

describe("V4 見えない位置の扱いと、自分の位置への移動", () => {
  const OWN = 2555;
  const anchorTop = 45 * tierSlotHeight(1);
  const anchorHeight = BOUNDARY_SLOT_HEIGHT;

  test("先頭にいて自分が下に隠れていれば、下端に張り付き data-state=below", async () => {
    await renderScreen(OWN);
    const marker = screen.getByTestId("speed-marker");
    expect(marker).toHaveAttribute("data-state", "below");
    expect(marker.style.top).toBe(`${String(DEFAULT_VIEWPORT_HEIGHT - (anchorHeight - ROW_GAP))}px`);
  });

  test("通り過ぎたら上端に張り付き data-state=above", async () => {
    await renderScreen(OWN);
    scrollTo(anchorTop + 1000);
    const marker = screen.getByTestId("speed-marker");
    expect(marker).toHaveAttribute("data-state", "above");
    expect(marker.style.top).toBe("0px");
  });

  test("「自分の位置へ移動」でスクロールが動き、自分の行が見える位置に来る", async () => {
    const { user } = await renderScreen(OWN);
    await user.click(screen.getByRole("button", { name: speedScreenText.jumpToSelfLabel }));
    const total = TIER_COUNT * tierSlotHeight(1) + BOUNDARY_SLOT_HEIGHT;
    const expected = scrollTopToReveal(
      { top: anchorTop, height: anchorHeight },
      DEFAULT_VIEWPORT_HEIGHT,
      total,
    );
    await waitFor(() => {
      expect(viewport().scrollTop).toBe(expected);
    });
    expect(screen.getByTestId("speed-marker")).toHaveAttribute("data-state", "visible");
    expect(within(viewport()).getByTestId("speed-boundary")).toBeInTheDocument();
  });
});

describe("V5 トリックルーム(昇順の表)", () => {
  test("昇順でも、自分より速い最初の段の前に境界が入り、マーカーが同じ高さに来る", async () => {
    const { user } = await renderScreen(2555);
    await user.click(
      within(screen.getByRole("group", { name: speedScreenText.fieldGroupLabel })).getByRole("checkbox", {
        name: speedScreenText.trickRoomLabel,
      }),
    );
    // 昇順: 10, 20, ... 。2555 より速い最初の段は 2560(昇順の index 255)。境界はその前。
    const boundaryTop = 255 * tierSlotHeight(1);
    await waitFor(() => {
      expect(screen.getAllByTestId("speed-tier")[0]).toHaveAttribute("data-speed", "10");
    });
    scrollTo(boundaryTop - 40);
    const boundary = within(viewport()).getByTestId("speed-boundary");
    expect(boundary).toHaveAttribute("data-after-speed", "2550");
    expect(boundary).toHaveAttribute("data-before-speed", "2560");
    const marker = screen.getByTestId("speed-marker");
    expect(marker).toHaveAttribute("data-state", "visible");
    expect(marker.style.top).toBe("40px");
  });
});

describe("V6 キーボードと支援技術", () => {
  test("スクロール領域はフォーカスでき、名前を持つ", async () => {
    await renderScreen(null);
    expect(viewport()).toHaveAttribute("tabindex", "0");
    expect(viewport()).toHaveAccessibleName(speedScreenText.viewportLabel);
  });

  test("自分の位置への移動ボタンはキーボードで押せる(button)", async () => {
    const { user } = await renderScreen(2555);
    const button = screen.getByRole("button", { name: speedScreenText.jumpToSelfLabel });
    button.focus();
    await user.keyboard("{Enter}");
    await waitFor(() => {
      expect(screen.getByTestId("speed-marker")).toHaveAttribute("data-state", "visible");
    });
  });

  test("マーカーは見た目の補助(読み上げは右の結果と境界の行が担う)なので aria-hidden", async () => {
    await renderScreen(2555);
    expect(screen.getByTestId("speed-marker")).toHaveAttribute("aria-hidden", "true");
  });
});
