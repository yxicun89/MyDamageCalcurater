// I-web-13c(G-05、ADR-0339): 逆算画面の見た目の作り直し(docs/design.md「見た目の作り直し方針(G-05)」→「画面ごとの方向」)。
// 確かめること:
//   - 自分・相手はポケモンカード(画像かエンブレム + 名前 h3 + タイプバッジ + 持ち物の印)
//   - ダメージの側(与えた/受けた)・ダメージの単位(%/HP)は区切りボタン(radiogroup + radio。矢印キーで動く)
//   - 自分の調整はチップ(ネイティブの radio を包む)
//   - 候補はカード(型の名前・「ほぼ合う候補」「参考」はバッジ、予測%の数字とバー)
//   - 説明の文を出し続けない(待機の状態の段落は単位の 1 行だけ。種族検索の補足は「説明」ボタンの奥)
// 読み上げ用の名前・構造は既存のテスト(ReverseScreen.*.test.tsx)が守る。ここは新しい部品のクラス・役割だけを見る。

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { readFileSync } from "node:fs";
import { afterEach, beforeAll, describe, expect, test, vi } from "vitest";
import { OBSERVATION_INPUT_DEBOUNCE_MS } from "../domain/observations";
import { masterOnlineText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { createFakeEngine, ok, reverseResultFor } from "../test/fakeEngine";
import { localPath } from "../test/localPath";
import { MEGA_FIRE, MEGA_FIRE_STONE_LABEL, withMegaFixture } from "../test/megaMaster";
import { ReverseScreen } from "./ReverseScreen";

let master: MasterData;
let megaMaster: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
  megaMaster = withMegaFixture(master);
});

afterEach(() => {
  vi.useRealTimers();
});

const myCard = () => screen.getByRole("region", { name: "自分のポケモン" });
const theirCard = () => screen.getByRole("region", { name: "相手のポケモン" });

function species(index: number): MasterSpecies {
  const found = master.species[index];
  if (found === undefined) {
    throw new Error(`例データに ${String(index)} 番目の種族が無い`);
  }
  return found;
}

function renderScreen(data: MasterData = master): { user: UserEvent; container: HTMLElement } {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
  const { container } = render(<ReverseScreen engine={createFakeEngine()} master={data} />);
  return { user, container };
}

async function choosePair(user: UserEvent): Promise<void> {
  await user.selectOptions(screen.getByRole("combobox", { name: "自分のポケモン" }), species(0).key);
  await user.selectOptions(screen.getByRole("combobox", { name: "相手のポケモン" }), species(1).key);
}

async function showCandidates(user: UserEvent): Promise<HTMLElement[]> {
  await choosePair(user);
  await user.type(screen.getByRole("textbox", { name: "ダメージ1" }), "45");
  act(() => {
    vi.advanceTimersByTime(OBSERVATION_INPUT_DEBOUNCE_MS);
  });
  const list = await screen.findByRole("list", { name: "考えられる振り方" });
  return within(list).getAllByRole("listitem");
}

describe("ポケモンカード", () => {
  test("種族を選ぶと、カードに画像かエンブレム・名前(h3)・タイプバッジが出る", async () => {
    const { user } = renderScreen();
    await choosePair(user);
    for (const [card, found] of [
      [myCard(), species(0)],
      [theirCard(), species(1)],
    ] as const) {
      const pokemonCard = card.querySelector(".ui-pokemon-card");
      expect(pokemonCard).not.toBeNull();
      expect(within(card).getByTestId("pokemon-icon-emblem")).toBeInTheDocument();
      expect(within(card).getByRole("heading", { level: 3, name: found.nameJa })).toBeInTheDocument();
      expect(pokemonCard?.querySelectorAll(".ui-badge")).toHaveLength(found.types.length);
    }
  });

  test("種族が未選択の間はカードを出さない(選ぶ欄だけ)", () => {
    renderScreen();
    expect(myCard().querySelector(".ui-pokemon-card")).toBeNull();
    expect(theirCard().querySelector(".ui-pokemon-card")).toBeNull();
  });

  test("自分の持ち物を選ぶとカードに持ち物の印が出る。「なし」なら出さない", async () => {
    const { user } = renderScreen();
    await choosePair(user);
    expect(myCard().querySelector(".ui-pokemon-card__item")).toBeNull();
    const select = screen.getByRole("combobox", { name: "自分の持ち物" });
    const option = within(select)
      .getAllByRole("option")
      .find((candidate) => candidate.getAttribute("value") !== "");
    if (option === undefined) {
      throw new Error("持ち物の選択肢が無い");
    }
    await user.selectOptions(select, option.getAttribute("value") ?? "");
    expect(myCard().querySelector(".ui-pokemon-card__item")).toHaveTextContent(option.textContent);
  });

  test("自分がメガ種族ならメガストーンが固定の持ち物としてカードの印に出る", async () => {
    const { user } = renderScreen(megaMaster);
    await user.selectOptions(screen.getByRole("combobox", { name: "自分のポケモン" }), MEGA_FIRE.key);
    expect(myCard().querySelector(".ui-pokemon-card__item")).toHaveTextContent(MEGA_FIRE_STONE_LABEL);
  });
});

describe("区切りボタン・チップ", () => {
  test("ダメージの側は区切りボタン(radiogroup + radio)。矢印キーで選択が動く", async () => {
    const { user } = renderScreen();
    const group = screen.getByRole("radiogroup", { name: "どちらのダメージ" });
    expect(group).toHaveClass("ui-segmented");
    const dealt = within(group).getByRole("radio", { name: "与えたダメージ" });
    expect(dealt).toBeChecked();
    dealt.focus();
    await user.keyboard("{ArrowRight}");
    expect(within(group).getByRole("radio", { name: "受けたダメージ" })).toBeChecked();
  });

  test("ダメージの単位(%/HP)も区切りボタン。選択中は塗り + チェックの印", async () => {
    const { user } = renderScreen();
    const group = screen.getByRole("radiogroup", { name: "ダメージ1の単位" });
    expect(group).toHaveClass("ui-segmented");
    const percent = within(group).getByRole("radio", { name: "%" });
    expect(percent).toBeChecked();
    expect(percent).toHaveClass("ui-segmented__option--selected");
    await user.click(within(group).getByRole("radio", { name: "HP" }));
    expect(within(group).getByRole("radio", { name: "HP" })).toHaveClass("ui-segmented__option--selected");
    expect(percent).not.toHaveClass("ui-segmented__option--selected");
  });

  test("自分の調整はチップ(radio を包む label)。選択中だけ ui-chip--selected", async () => {
    const { user } = renderScreen();
    await choosePair(user);
    const group = screen.getByRole("radiogroup", { name: "自分の調整" });
    for (const radio of within(group).getAllByRole("radio")) {
      const chip = radio.closest("label");
      expect(chip).toHaveClass("ui-chip");
      expect(chip?.classList.contains("ui-chip--selected")).toBe((radio as HTMLInputElement).checked);
    }
  });
});

describe("候補はカード", () => {
  test("候補ごとに、型の名前はバッジ・予測%は数字・バー(装飾)が出る", async () => {
    const { user } = renderScreen();
    const rows = await showCandidates(user);
    expect(rows.length).toBeGreaterThan(0);
    for (const row of rows) {
      expect(row).toHaveClass("reverse-results__row");
      expect(row.querySelector(".reverse-results__percent-value")?.textContent).toMatch(/%/);
      const bar = row.querySelector("[data-testid='candidate-bar']");
      expect(bar).toHaveAttribute("aria-hidden", "true");
    }
  });

  test("型の名前(目安)・「ほぼ合う候補」・「参考」はバッジ(ui-badge)", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
    // 全件が不一致(exact: false)の応答にして「ほぼ合う候補」「参考」を出す。
    const engine = createFakeEngine(undefined, (request) => {
      const base = reverseResultFor(request);
      return ok({
        ...base,
        exactCount: 0,
        candidates: base.candidates.map((candidate) => ({ ...candidate, exact: false })),
      });
    });
    render(<ReverseScreen engine={engine} master={master} />);
    await choosePair(user);
    await user.type(screen.getByRole("textbox", { name: "ダメージ1" }), "45");
    act(() => {
      vi.advanceTimersByTime(OBSERVATION_INPUT_DEBOUNCE_MS);
    });
    const list = await screen.findByRole("list", { name: "考えられる振り方" });
    const first = within(list).getAllByRole("listitem")[0];
    if (first === undefined) {
      throw new Error("候補が無い");
    }
    expect(within(first).getByText("ほぼ合う候補")).toHaveClass("ui-badge");
    expect(within(first).getByText("参考")).toHaveClass("ui-badge");
  });

  test("CSS: 予測%の数字は結果の大きさ、補足(持ち物・特性)は caption", () => {
    const css = readFileSync(localPath("./ReverseScreen.css", import.meta.url), "utf8");
    const ruleOf = (selector: string): string => {
      const start = css.indexOf(`${selector} {`);
      expect(start, `${selector} の規則が無い`).toBeGreaterThanOrEqual(0);
      return css.slice(start, css.indexOf("}", start));
    };
    expect(ruleOf(".reverse-results__percent-value")).toContain("var(--font-size-result)");
    expect(ruleOf(".reverse-results__nature")).toContain("var(--font-size-heading)");
    expect(css).toMatch(/\.reverse-results__item[^{]*\{[^}]*var\(--font-size-caption\)/);
  });
});

describe("文で説明しない", () => {
  test("待機の状態の段落(<p>)は、ダメージ欄の単位の 1 行だけ", () => {
    const { container } = renderScreen();
    const paragraphs = [...container.querySelectorAll("p")];
    expect(paragraphs).toHaveLength(1);
    expect(paragraphs[0]).toHaveClass("reverse-observation__hint");
  });

  test("種族の検索欄の補足は「説明」ボタンの奥(開くまで出ない)", async () => {
    const user = userEvent.setup();
    const onlineMaster: MasterData = {
      ...master,
      capabilities: { speciesList: false, moves: true, effects: true },
    };
    render(<ReverseScreen engine={createFakeEngine()} master={onlineMaster} />);
    expect(screen.queryByText(masterOnlineText.speciesSearchHint)).toBeNull();
    await user.click(within(myCard()).getByRole("button", { name: "説明" }));
    await waitFor(() => {
      expect(screen.getByText(masterOnlineText.speciesSearchHint)).toBeInTheDocument();
    });
  });
});
