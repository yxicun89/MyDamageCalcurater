// issue #515・ADR-0320(docs/mega-evolution-spec.md §4-3): 逆算画面のメガシンカの持ち物固定。
// engine は fake。マスタは架空の例データに、架空のメガ種族・メガストーンを足したもの(test/megaMaster.ts)。
// 確かめること:
//   - 自分がメガ種族: 持ち物欄は disabled・メガストーンの名前・理由(見える文言と aria-describedby)。known.item がメガストーン
//   - メガでない種族に変えると固定が外れ、未選択に戻る
//   - 相手がメガ種族: 持ち物候補は探索せず、メガストーン1件に固定(itemCandidates = [メガストーン])。理由と名前を相手のカードに出す
//   - 相手がメガでない種族: 候補は今までどおり(メガストーンは候補に混ざらない)
//   - ストーンを引けないメガ種族の相手は、候補を探索せず [null]・理由を出す
//   - 観測した側(与えた / 受けた)を切り替えても整合する
//   - 検索で解決するマスタでも同じに動く

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { afterEach, beforeAll, describe, expect, test, vi } from "vitest";
import { firstDamagingMove } from "../domain/moves";
import { OBSERVATION_INPUT_DEBOUNCE_MS } from "../domain/observations";
import { reverseItemCandidates } from "../domain/reverseItems";
import type { Move, ReverseRequest } from "../engine/types";
import { calcScreenText, megaItemText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import { SPECIES_SEARCH_DEBOUNCE_MS } from "../master/onlineSource";
import type { MasterCapabilities, MasterData, MasterSpecies } from "../master/types";
import { createFakeEngine, type FakeEngine } from "../test/fakeEngine";
import {
  MEGA_FIRE,
  MEGA_FIRE_STONE,
  MEGA_FIRE_STONE_LABEL,
  MEGA_ORPHAN,
  MEGA_WATER,
  MEGA_WATER_STONE,
  MEGA_WATER_STONE_LABEL,
  withMegaFixture,
} from "../test/megaMaster";
import { createFakeSpeciesSearch, limitedMaster } from "../test/onlineMaster";
import { ReverseScreen } from "./ReverseScreen";

let example: MasterData;
let master: MasterData;

beforeAll(async () => {
  example = await exampleMasterSource.load();
  master = withMegaFixture(example);
});

afterEach(() => {
  vi.useRealTimers();
});

function normalSpecies(index: number): MasterSpecies {
  const species = example.species[index];
  if (species === undefined) {
    throw new Error(`例データに ${index} 番目の種族が無い`);
  }
  return species;
}

function firstMoveOf(species: MasterSpecies): Move {
  const move = firstDamagingMove(species, master.moves);
  if (move === undefined) {
    throw new Error(`${species.key} がダメージ技を覚えない`);
  }
  return move;
}

const sideGroup = () => screen.getByRole("radiogroup", { name: "観測したダメージ" });
const mySpeciesSelect = () => screen.getByRole("combobox", { name: "自分のポケモン" });
const theirSpeciesSelect = () => screen.getByRole("combobox", { name: "相手のポケモン" });
const myItemSelect = () => screen.getByRole("combobox", { name: "自分の持ち物" });
const myCard = () => screen.getByRole("region", { name: "自分のポケモン" });
const theirCard = () => screen.getByRole("region", { name: "相手のポケモン" });
const observationInput = (n: number) => screen.getByRole("textbox", { name: `観測${String(n)}` });

function lastRequest(engine: FakeEngine): ReverseRequest {
  const request = engine.reverseRequests.at(-1);
  if (request === undefined) {
    throw new Error("calcReverse が呼ばれていない");
  }
  return request;
}

function flushObservationDebounce(): void {
  act(() => {
    vi.advanceTimersByTime(OBSERVATION_INPUT_DEBOUNCE_MS);
  });
}

async function typeObservation(user: UserEvent, n: number, text: string): Promise<void> {
  await user.type(observationInput(n), text);
  flushObservationDebounce();
}

function renderScreen(data: MasterData = master): { user: UserEvent; engine: FakeEngine } {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
  const engine = createFakeEngine();
  render(<ReverseScreen engine={engine} master={data} />);
  return { user, engine };
}

async function choosePair(user: UserEvent, mine: MasterSpecies, theirs: MasterSpecies): Promise<void> {
  await user.selectOptions(mySpeciesSelect(), mine.key);
  await user.selectOptions(theirSpeciesSelect(), theirs.key);
}

describe("自分がメガ種族(持ち物欄がある側)", () => {
  test("持ち物欄は disabled でメガストーンの名前が見え、理由が見える文言と aria-describedby で伝わる", async () => {
    const { user } = renderScreen();
    await user.selectOptions(mySpeciesSelect(), MEGA_FIRE.key);

    expect(myItemSelect()).toBeDisabled();
    expect(myItemSelect()).toHaveValue(MEGA_FIRE_STONE.id);
    expect(myItemSelect()).toHaveDisplayValue(MEGA_FIRE_STONE_LABEL);
    expect(within(myCard()).getByText(megaItemText.lockedReason)).toBeVisible();
    expect(myItemSelect()).toHaveAccessibleDescription(megaItemText.lockedReason);
  });

  test("要求の known.item にメガストーンが載る", async () => {
    const { user, engine } = renderScreen();
    await choosePair(user, MEGA_FIRE, normalSpecies(1));
    await typeObservation(user, 1, "45");

    await waitFor(() => {
      expect(engine.reverseRequests.length).toBeGreaterThan(0);
    });
    expect(lastRequest(engine).known.item).toEqual(MEGA_FIRE_STONE);
    expect(lastRequest(engine).known.species.isMega).toBe(true);
    expect(lastRequest(engine).known.species.requiredItemId).toBe(MEGA_FIRE_STONE.id);
  });

  test("メガでない種族に変えると固定が外れ、持ち物は未選択に戻る。要求の known.item は null", async () => {
    const { user, engine } = renderScreen();
    await choosePair(user, MEGA_FIRE, normalSpecies(1));
    await typeObservation(user, 1, "45");

    await user.selectOptions(mySpeciesSelect(), normalSpecies(0).key);

    expect(myItemSelect()).not.toBeDisabled();
    expect(myItemSelect()).toHaveValue("");
    expect(myItemSelect()).toHaveDisplayValue(calcScreenText.noItemOption);
    expect(within(myCard()).queryByText(megaItemText.lockedReason)).toBeNull();
    await waitFor(() => {
      expect(lastRequest(engine).known.item).toBeNull();
    });
  });

  test("メガストーンは単独の選択肢に出ない(メガでない種族のとき)", async () => {
    const { user } = renderScreen();
    await user.selectOptions(mySpeciesSelect(), normalSpecies(0).key);

    const names = within(myItemSelect())
      .getAllByRole("option")
      .map((option) => option.textContent);
    expect(names).toEqual([calcScreenText.noItemOption, ...example.items.map((item) => item.nameJa)]);
  });

  test("ストーンを引けないメガ種族は、固定せず空・disabled で、理由(引けないこと)を伝える", async () => {
    const { user } = renderScreen();
    await user.selectOptions(mySpeciesSelect(), MEGA_ORPHAN.key);

    expect(myItemSelect()).toBeDisabled();
    expect(myItemSelect()).toHaveValue("");
    expect(myItemSelect()).toHaveAccessibleDescription(megaItemText.missingReason);
  });
});

describe("相手がメガ種族(持ち物候補を探索しない)", () => {
  test("与えたダメージ: itemCandidates はメガストーン1件(null も他の候補も混ぜない)", async () => {
    const { user, engine } = renderScreen();
    await choosePair(user, normalSpecies(0), MEGA_WATER);
    await typeObservation(user, 1, "45");

    await waitFor(() => {
      expect(engine.reverseRequests.length).toBeGreaterThan(0);
    });
    expect(lastRequest(engine).itemCandidates).toEqual([MEGA_WATER_STONE]);
    expect(lastRequest(engine).side).toBe("defender");
    expect(lastRequest(engine).unknownSpecies.isMega).toBe(true);
    expect(lastRequest(engine).unknownSpecies.requiredItemId).toBe(MEGA_WATER_STONE.id);
  });

  test("受けたダメージ: 相手(攻撃側)がメガでも itemCandidates はメガストーン1件", async () => {
    const { user, engine } = renderScreen();
    await user.click(within(sideGroup()).getByRole("radio", { name: "受けたダメージ" }));
    await choosePair(user, normalSpecies(1), MEGA_FIRE);
    await typeObservation(user, 1, "45");

    await waitFor(() => {
      expect(engine.reverseRequests.length).toBeGreaterThan(0);
    });
    expect(lastRequest(engine).side).toBe("attacker");
    expect(lastRequest(engine).itemCandidates).toEqual([MEGA_FIRE_STONE]);
  });

  test("相手のカードに、固定の理由とメガストーンの名前を出す", async () => {
    const { user } = renderScreen();
    await user.selectOptions(theirSpeciesSelect(), MEGA_WATER.key);

    expect(within(theirCard()).getByText(megaItemText.lockedReason)).toBeVisible();
    expect(within(theirCard()).getByText(megaItemText.fixedItemName(MEGA_WATER_STONE_LABEL))).toBeVisible();
  });

  test("相手をメガでない種族に戻すと、候補は今までどおり(メガストーンは混ざらない)・固定の表示は消える", async () => {
    const mine = normalSpecies(0);
    const move = firstMoveOf(mine);
    const { user, engine } = renderScreen();
    await choosePair(user, mine, MEGA_WATER);
    await typeObservation(user, 1, "45");

    await user.selectOptions(theirSpeciesSelect(), normalSpecies(1).key);

    await waitFor(() => {
      expect(lastRequest(engine).itemCandidates).toEqual(
        reverseItemCandidates("defender", example.items, move).candidates,
      );
    });
    const ids = (lastRequest(engine).itemCandidates ?? []).map((item) => item?.id ?? null);
    expect(ids[0]).toBeNull();
    expect(ids).not.toContain(MEGA_FIRE_STONE.id);
    expect(ids).not.toContain(MEGA_WATER_STONE.id);
    expect(within(theirCard()).queryByText(megaItemText.lockedReason)).toBeNull();
  });

  test("メガ種族が両側にいない通常の組でも、候補にメガストーンは混ざらない(効果が防御を上げるものでも)", async () => {
    const mine = normalSpecies(0);
    const move = firstMoveOf(mine);
    const { user, engine } = renderScreen();
    await choosePair(user, mine, normalSpecies(1));
    await typeObservation(user, 1, "45");

    await waitFor(() => {
      expect(lastRequest(engine).itemCandidates).toEqual(
        reverseItemCandidates("defender", example.items, move).candidates,
      );
    });
  });

  test("ストーンを引けないメガ種族の相手は、探索せず [null]・理由(引けないこと)を出す", async () => {
    const { user, engine } = renderScreen();
    await choosePair(user, normalSpecies(0), MEGA_ORPHAN);
    await typeObservation(user, 1, "45");

    await waitFor(() => {
      expect(engine.reverseRequests.length).toBeGreaterThan(0);
    });
    expect(lastRequest(engine).itemCandidates).toEqual([null]);
    expect(within(theirCard()).getByText(megaItemText.missingReason)).toBeVisible();
  });
});

describe("観測した側(与えた / 受けた)の切り替え", () => {
  test("自分がメガのまま「受けたダメージ」に切り替えても、固定が保たれ known.item(防御側)がメガストーン", async () => {
    const { user, engine } = renderScreen();
    await choosePair(user, MEGA_FIRE, normalSpecies(1));
    await user.click(within(sideGroup()).getByRole("radio", { name: "受けたダメージ" }));

    expect(myItemSelect()).toBeDisabled();
    expect(myItemSelect()).toHaveValue(MEGA_FIRE_STONE.id);
    await typeObservation(user, 1, "45");
    await waitFor(() => {
      expect(engine.reverseRequests.length).toBeGreaterThan(0);
    });
    expect(lastRequest(engine).side).toBe("attacker");
    expect(lastRequest(engine).known.item).toEqual(MEGA_FIRE_STONE);
  });
});

describe("種族を検索で解決するマスタ(isMega は解決後の種族から読む)", () => {
  const SEARCH_ONLY: MasterCapabilities = { speciesList: false, moves: true, effects: true };

  test("検索でメガ種族を選ぶと、持ち物欄が固定される", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
    const search = createFakeSpeciesSearch({
      species: master.species,
      abilities: master.abilities,
      moves: master.moves,
    });
    render(
      <ReverseScreen
        engine={createFakeEngine()}
        master={limitedMaster(master, SEARCH_ONLY)}
        masterSearch={search}
      />,
    );

    await user.type(within(myCard()).getByRole("combobox", { name: "自分のポケモン" }), MEGA_FIRE.nameJa);
    act(() => {
      vi.advanceTimersByTime(SPECIES_SEARCH_DEBOUNCE_MS);
    });
    await user.click(await within(myCard()).findByRole("option", { name: MEGA_FIRE.nameJa }));

    const select = await within(myCard()).findByRole("combobox", { name: "自分の持ち物" });
    expect(select).toBeDisabled();
    expect(select).toHaveDisplayValue(MEGA_FIRE_STONE_LABEL);
    expect(select).toHaveAccessibleDescription(megaItemText.lockedReason);
  });
});
