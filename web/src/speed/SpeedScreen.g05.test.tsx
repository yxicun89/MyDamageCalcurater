// I-web-13e(G-05、ADR-0339): 素早さ画面の見た目の作り直し(docs/design.md「見た目の作り直し方針(G-05)」→「画面ごとの方向」)。
// 確かめること:
//   - 領域の見出し(h2)はアイコン + 短い名前。自分はポケモンカード(画像かエンブレム + 名前 h3 + タイプバッジ)
//   - 入力の方法・調整・性格補正は区切りボタン(radiogroup + radio。矢印キーで動く)。スカーフ・追い風・まひ・絞り込みはチップ
//   - 素早さ SP とランクは増減ボタン + 数値欄(範囲外は従来どおり誤りとして出し、送らない)
//   - 表の自分の段は枠 + 「自分」のバッジ(アイコン付き)。同速は ui-badge。高さの定数(ADR-0608)は変えない
//   - 自分の周りは小さなカード(自分のカードは「自分」のバッジ)で、表のスクロール領域の外のまま
// 読み上げ用の名前・testid・構造は既存のテスト(SpeedScreen*.test.tsx)が守る。ここは新しい部品のクラス・役割だけを見る。

import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { readFileSync } from "node:fs";
import { describe, expect, test } from "vitest";
import { speedScreenText } from "../i18n/ja";
import { localPath } from "../test/localPath";
import { SpeedScreen } from "./SpeedScreen";
import type { components } from "./speed.gen";
import type { SpeedClient } from "./speedClient";
import { ENTRY_HEIGHT, ROW_GAP } from "./speedWindow";

type Schemas = components["schemas"];

const BIRD: Schemas["SpeedPokemon"] = {
  pokemonId: "9001-000",
  nameJa: "テストカソウドリ",
  types: ["fire", "flying"],
  baseSpeed: 100,
};
const MOLE: Schemas["SpeedPokemon"] = {
  pokemonId: "9002-000",
  nameJa: "テストモグラ",
  types: ["ground"],
  baseSpeed: 40,
};

const OWN_SPEED = 150;

const tableResponse: Schemas["TableResponse"] = {
  regulationId: "example",
  presets: ["uninvested", "neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"],
  tiers: [
    { speed: 200, entries: [{ ...BIRD, preset: "max" }] },
    {
      speed: OWN_SPEED,
      entries: [
        { ...MOLE, preset: "max" },
        { ...BIRD, preset: "neutral-max" },
      ],
    },
    { speed: 100, entries: [{ ...MOLE, preset: "uninvested" }] },
  ],
};

function makeClient(positions: Schemas["PositionRequest"][] = []): SpeedClient {
  return {
    pokemon: () => Promise.resolve({ ok: true, value: { regulationId: "example", pokemon: [BIRD, MOLE] } }),
    table: () => Promise.resolve({ ok: true, value: tableResponse }),
    position: (request) => {
      positions.push(request);
      return Promise.resolve({
        ok: true,
        value: { speed: OWN_SPEED, faster: 1, slower: 1, tie: [{ ...MOLE, preset: "max" }], pokemon: BIRD },
      });
    },
  };
}

const tableRegion = () => screen.getByRole("region", { name: speedScreenText.tableRegionLabel });
const selfRegion = () => screen.getByRole("region", { name: speedScreenText.selfRegionLabel });

async function renderScreen(positions: Schemas["PositionRequest"][] = []) {
  const user = userEvent.setup();
  render(<SpeedScreen speedClient={makeClient(positions)} />);
  await screen.findAllByTestId("speed-tier");
  return { user };
}

async function chooseBird(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await user.selectOptions(
    within(selfRegion()).getByRole("combobox", { name: speedScreenText.pokemonLabel }),
    BIRD.pokemonId,
  );
  await screen.findByTestId("speed-marker");
}

describe("領域の見出し", () => {
  test("表・自分はそれぞれ h2(アイコン + 短い名前)", async () => {
    await renderScreen();
    for (const [region, name] of [
      [tableRegion(), speedScreenText.tableRegionLabel],
      [selfRegion(), speedScreenText.selfRegionLabel],
    ] as const) {
      const heading = within(region).getByRole("heading", { level: 2, name });
      expect(heading.querySelector("svg")).not.toBeNull();
    }
  });
});

describe("自分のポケモンカード", () => {
  test("未選択の間はカードを出さず、選ぶと名前(h3)・タイプバッジ・エンブレムが出る", async () => {
    const { user } = await renderScreen();
    expect(selfRegion().querySelector(".ui-pokemon-card")).toBeNull();
    await chooseBird(user);
    const card = selfRegion().querySelector(".ui-pokemon-card");
    expect(card).not.toBeNull();
    expect(within(selfRegion()).getByRole("heading", { level: 3, name: BIRD.nameJa })).toBeInTheDocument();
    expect(card?.querySelectorAll(".ui-badge")).toHaveLength(BIRD.types.length);
    expect(card?.querySelector('[data-testid="pokemon-icon-emblem"]')).not.toBeNull();
  });
});

describe("区切りボタンとチップ", () => {
  test("入力の方法は区切りボタン。矢印キーで選択が動き、その入力に切り替わる", async () => {
    const { user } = await renderScreen();
    const group = within(selfRegion()).getByRole("radiogroup", { name: speedScreenText.modeGroupLabel });
    expect(group).toHaveClass("ui-segmented");
    const preset = within(group).getByRole("radio", { name: speedScreenText.modeLabel.preset });
    expect(preset).toHaveClass("ui-segmented__option--selected");
    preset.focus();
    await user.keyboard("{ArrowRight}");
    expect(within(group).getByRole("radio", { name: speedScreenText.modeLabel.custom })).toBeChecked();
    expect(
      within(selfRegion()).getByRole("spinbutton", { name: speedScreenText.spLabel }),
    ).toBeInTheDocument();
  });

  test("調整(preset)・性格補正(custom)も区切りボタン", async () => {
    const { user } = await renderScreen();
    expect(
      within(selfRegion()).getByRole("radiogroup", { name: speedScreenText.presetGroupLabel }),
    ).toHaveClass("ui-segmented");
    await user.click(within(selfRegion()).getByRole("radio", { name: speedScreenText.modeLabel.custom }));
    expect(
      within(selfRegion()).getByRole("radiogroup", { name: speedScreenText.natureGroupLabel }),
    ).toHaveClass("ui-segmented");
  });

  test("スカーフ・追い風・まひ・絞り込み・場の状態はチップ(ネイティブの checkbox を包む)", async () => {
    await renderScreen();
    const names = [
      speedScreenText.scarfLabel,
      speedScreenText.selfTailwindLabel,
      speedScreenText.paralysisLabel,
      speedScreenText.tableTailwindLabel,
      speedScreenText.trickRoomLabel,
    ];
    for (const name of names) {
      const checkbox = screen.getByRole("checkbox", { name });
      expect(checkbox.closest("label")).toHaveClass("ui-chip");
    }
    const checked = screen.getByRole("checkbox", { name: speedScreenText.scarfLabel });
    expect(checked.closest("label")).not.toHaveClass("ui-chip--selected");
    await userEvent.click(checked);
    expect(checked.closest("label")).toHaveClass("ui-chip--selected");
  });
});

describe("増減ボタン", () => {
  test("素早さ SP は − / + と数値欄。押すと 1 ずつ変わり、上限・下限では動かない", async () => {
    const positions: Schemas["PositionRequest"][] = [];
    const { user } = await renderScreen(positions);
    await user.click(within(selfRegion()).getByRole("radio", { name: speedScreenText.modeLabel.custom }));
    await chooseBird(user);
    const input = within(selfRegion()).getByRole("spinbutton", { name: speedScreenText.spLabel });
    const plus = within(selfRegion()).getByRole("button", { name: `${speedScreenText.spLabel}を増やす` });
    const minus = within(selfRegion()).getByRole("button", { name: `${speedScreenText.spLabel}を減らす` });
    expect(minus).toHaveAttribute("aria-disabled", "true");
    await user.click(plus);
    expect(input).toHaveValue(1);
    await waitFor(() => {
      expect(positions.at(-1)).toMatchObject({ mode: "custom", sp: 1 });
    });
    fireEvent.change(input, { target: { value: "32" } });
    expect(plus).toHaveAttribute("aria-disabled", "true");
    await user.click(plus);
    expect(input).toHaveValue(32);
  });

  test("ランクも増減ボタン。範囲外の入力は従来どおり誤りとして出し、送らない", async () => {
    const positions: Schemas["PositionRequest"][] = [];
    const { user } = await renderScreen(positions);
    await user.click(within(selfRegion()).getByRole("radio", { name: speedScreenText.modeLabel.custom }));
    await chooseBird(user);
    const input = within(selfRegion()).getByRole("spinbutton", { name: speedScreenText.rankLabel });
    await user.click(
      within(selfRegion()).getByRole("button", { name: `${speedScreenText.rankLabel}を減らす` }),
    );
    expect(input).toHaveValue(-1);
    const before = positions.length;
    fireEvent.change(input, { target: { value: "7" } });
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(within(selfRegion()).getByRole("alert")).toBeInTheDocument();
    // 範囲外の値から押すと、範囲に収めた値になって誤りが消える。
    await user.click(
      within(selfRegion()).getByRole("button", { name: `${speedScreenText.rankLabel}を増やす` }),
    );
    expect(input).toHaveValue(6);
    expect(input).not.toHaveAttribute("aria-invalid", "true");
    expect(positions.slice(before).every((request) => request.mode !== "custom" || request.rank !== 7)).toBe(
      true,
    );
  });
});

describe("表の自分の段と境界", () => {
  test("同速の段は枠 + 「自分」のバッジ(アイコン付き)と同速のバッジ。読み上げの語は残す", async () => {
    const { user } = await renderScreen();
    await chooseBird(user);
    const row = within(tableRegion())
      .getAllByTestId("speed-tier")
      .find((el) => el.getAttribute("data-self") === "tie");
    expect(row).toBeDefined();
    const badge = row?.querySelector(".speed-badge--self");
    expect(badge).not.toBeNull();
    expect(badge).toHaveTextContent(speedScreenText.neighborhoodSelfBadge);
    expect(badge?.querySelector("svg")).not.toBeNull();
    expect(row).toHaveTextContent(speedScreenText.selfTierLabel);
    expect(row?.querySelector(".speed-tier__tie")).toHaveClass("ui-badge");
  });

  test("同速が無いときの境界にもアイコンが付く", async () => {
    const client = makeClient();
    client.position = () =>
      Promise.resolve({
        ok: true,
        value: { speed: 170, faster: 1, slower: 2, tie: [], pokemon: BIRD },
      });
    render(<SpeedScreen speedClient={client} />);
    await screen.findAllByTestId("speed-tier");
    await userEvent.selectOptions(
      within(selfRegion()).getByRole("combobox", { name: speedScreenText.pokemonLabel }),
      BIRD.pokemonId,
    );
    const boundary = await screen.findByTestId("speed-boundary");
    expect(boundary.querySelector("svg")).not.toBeNull();
    expect(boundary).toHaveTextContent(speedScreenText.selfBoundaryLabel);
  });

  test("高さの定数(ADR-0608)と行の絶対配置は変えない", async () => {
    await renderScreen();
    expect(ENTRY_HEIGHT).toBe(24);
    expect(ROW_GAP).toBe(8);
    const row = screen.getAllByTestId("speed-tier")[0];
    expect(row?.style.height).not.toBe("");
    const css = readFileSync(localPath("./SpeedScreen.css", import.meta.url), "utf8");
    const rule = /\.speed-tier\s*\{[^}]*\}/.exec(css)?.[0] ?? "";
    expect(rule).toContain("position: absolute");
    expect(rule).not.toMatch(/(?<![-\w])(height|min-height|flex-wrap)\s*:/);
    expect(css).not.toMatch(/animation\s*:/);
  });
});

describe("自分の周りの小さなカード", () => {
  test("前後の段・同速の段が小さなカードで、自分のカードは「自分」のバッジ(アイコン付き)", async () => {
    const { user } = await renderScreen();
    await chooseBird(user);
    const panel = screen.getByRole("region", { name: speedScreenText.neighborhoodRegionLabel });
    expect(panel.closest(".speed-table__viewport")).toBeNull();
    expect(within(panel).getByTestId("neighbor-before")).toHaveClass("speed-neighbor__card");
    expect(within(panel).getByTestId("neighbor-after")).toHaveClass("speed-neighbor__card");
    const self = within(panel).getByTestId("neighbor-self");
    expect(self).toHaveClass("speed-neighbor__card", "speed-neighbor__card--self");
    const badge = self.querySelector(".speed-badge--self");
    expect(badge).toHaveTextContent(speedScreenText.neighborhoodSelfBadge);
    expect(badge?.querySelector("svg")).not.toBeNull();
    // 近傍のカードにも画像かエンブレムが付く。
    expect(
      within(within(panel).getByTestId("neighbor-before")).getByTestId("pokemon-icon-emblem"),
    ).toBeInTheDocument();
  });

  test("自分が決まるまでは 1 行の空の状態だけ", async () => {
    await renderScreen();
    const panel = screen.getByRole("region", { name: speedScreenText.neighborhoodRegionLabel });
    expect(within(panel).getByText(speedScreenText.neighborhoodEmpty)).toBeInTheDocument();
    expect(panel.querySelectorAll("p")).toHaveLength(1);
  });
});
