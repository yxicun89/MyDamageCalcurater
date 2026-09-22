// P4-16b(ADR-0304 A-10): 画面に種族の検索口を渡す経路。App は今選ばれているマスタの取得口が
// 検索付き(isSearchableMasterSource)のときだけ、その search を画面へ渡す。
// 確かめること:
//   - オンラインの取得口が検索付きなら、画面の検索欄からその取得口の searchSpecies が呼ばれる
//   - 検索付きでない取得口(オフラインの例データ)では検索欄を出さない(今までどおりドロップダウン)

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { App } from "./App";
import { CALC_MODE_STORAGE_KEY } from "./app/calcMode";
import { exampleMasterSource } from "./master/exampleSource";
import { SPECIES_SEARCH_DEBOUNCE_MS } from "./master/onlineSource";
import type { MasterCapabilities, MasterData, SearchableMasterSource } from "./master/types";
import { createFakeEngine } from "./test/fakeEngine";
import { createFakeSpeciesSearch, limitedMaster, type FakeSpeciesSearch } from "./test/onlineMaster";

/** 種族だけ検索で引くマスタ(技はある = P4-17 の形。検索の経路だけをここで見る)。 */
const SEARCH_ONLY: MasterCapabilities = { speciesList: false, moves: true, effects: true };

beforeEach(() => {
  window.history.replaceState(null, "", "/");
  window.localStorage.removeItem(CALC_MODE_STORAGE_KEY);
});

afterEach(() => {
  window.history.replaceState(null, "", "/");
  window.localStorage.removeItem(CALC_MODE_STORAGE_KEY);
  vi.useRealTimers();
});

function modeRadios(): { offline: HTMLElement; online: HTMLElement } {
  const group = screen.getByRole("radiogroup", { name: "計算モード" });
  return {
    offline: within(group).getByRole("radio", { name: "オフライン(WASM)" }),
    online: within(group).getByRole("radio", { name: "オンライン(API)" }),
  };
}

/** 例データを元にした、検索付きのオンラインの取得口。 */
async function onlineSearchableSource(): Promise<{
  source: SearchableMasterSource;
  search: FakeSpeciesSearch;
  offline: MasterData;
}> {
  const offline = await exampleMasterSource.load();
  const search = createFakeSpeciesSearch({ species: offline.species, abilities: offline.abilities });
  const online = limitedMaster(offline, SEARCH_ONLY);
  return { source: { load: () => Promise.resolve(online), search }, search, offline };
}

test("オンラインの取得口が検索付きなら、画面の検索欄からその取得口を引く", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
  const { source, search, offline } = await onlineSearchableSource();
  render(
    <App
      engine={createFakeEngine()}
      masterSources={() => ({ offline: { load: () => Promise.resolve(offline) }, online: source })}
    />,
  );
  await screen.findByRole("combobox", { name: "攻撃側のポケモン" });

  await user.click(modeRadios().online);

  const input = await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
  await user.type(input, "テスト");
  act(() => {
    vi.advanceTimersByTime(SPECIES_SEARCH_DEBOUNCE_MS);
  });

  await waitFor(() => {
    expect(search.searchCalls.map((call) => call.query)).toEqual(["テスト"]);
  });
});

test("検索付きでない取得口では検索欄を出さない(今までどおりドロップダウン)", async () => {
  const user = userEvent.setup();
  const offline = await exampleMasterSource.load();
  render(<App engine={createFakeEngine()} masterSource={{ load: () => Promise.resolve(offline) }} />);

  const select = await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
  for (const species of offline.species) {
    expect(within(select).getByRole("option", { name: species.nameJa })).toHaveValue(species.key);
  }
  // 種族の一覧があるマスタなので、モードを切り替えても検索欄にはならない。
  await user.click(modeRadios().online);
  expect(
    within(await screen.findByRole("combobox", { name: "攻撃側のポケモン" })).getAllByRole("option"),
  ).not.toHaveLength(0);
});
