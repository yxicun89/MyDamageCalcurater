// ADR-0313(issue #210): App とキャッシュ付きのマスタ取得口の結合。
//   - 保存済みがオフライン + キャッシュ空 → 架空データを出さず、案内を role=alert に出す(「再試行」はある)
//   - オンラインで一度取得した後、オフラインに切り替えるとキャッシュのマスタで画面が出る
//   - 既定(オンライン)で API が落ちていてもオフラインへ自動で落とさない(従来のエラー+再試行)

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import typeChartData from "@typechart";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { App } from "./App";
import { CALC_MODE_STORAGE_KEY } from "./app/calcMode";
import { appText } from "./i18n/ja";
import { createCachedMasterSources } from "./master/cache/cachedSources";
import { exampleItems } from "./master/example/items";
import { exampleNatures } from "./master/example/natures";
import { ONLINE_MASTER_CAPABILITIES } from "./master/onlineSource";
import { typeChartFromData } from "./master/typeChart";
import type { SearchableMasterSource } from "./master/types";
import { createFakeEngine } from "./test/fakeEngine";
import { createMemoryMasterCacheStore } from "./test/memoryMasterCacheStore";

beforeEach(() => {
  window.history.replaceState(null, "", "/");
  window.localStorage.removeItem(CALC_MODE_STORAGE_KEY);
});
afterEach(() => {
  vi.restoreAllMocks();
  window.localStorage.removeItem(CALC_MODE_STORAGE_KEY);
});

function onlineFake(fail: boolean): SearchableMasterSource {
  return {
    load: () =>
      fail
        ? Promise.reject(new Error("API に届かない"))
        : Promise.resolve({
            species: [],
            moves: [],
            items: exampleItems,
            abilities: [],
            natures: exampleNatures,
            typeChart: typeChartFromData(typeChartData),
            capabilities: ONLINE_MASTER_CAPABILITIES,
          }),
    search: {
      searchSpecies: () => Promise.resolve([]),
      resolveSpecies: () => Promise.reject(new Error("not used")),
    },
  };
}

test("保存済みがオフライン・キャッシュ空: 架空データを出さず、案内を alert に出す", async () => {
  window.localStorage.setItem(CALC_MODE_STORAGE_KEY, "offline");
  const store = createMemoryMasterCacheStore();
  render(
    <App
      engine={createFakeEngine()}
      masterSources={() => createCachedMasterSources({ online: onlineFake(true), store })}
    />,
  );

  const alert = await screen.findByRole("alert");
  expect(within(alert).getByText(new RegExp(appText.masterCacheEmptyError))).toBeInTheDocument();
  expect(screen.queryByRole("combobox", { name: "攻撃側のポケモン" })).toBeNull();
  expect(screen.queryByText(/テストほのお|テストモン/)).toBeNull();
  expect(screen.getByRole("button", { name: "再試行" })).toBeInTheDocument();
});

test("既定(オンライン)で API 遮断・キャッシュ空: オフラインへ自動で落とさず、エラーと再試行を出す", async () => {
  const store = createMemoryMasterCacheStore();
  render(
    <App
      engine={createFakeEngine()}
      masterSources={() => createCachedMasterSources({ online: onlineFake(true), store })}
    />,
  );

  const alert = await screen.findByRole("alert");
  expect(within(alert).getByText(/API に届かない/)).toBeInTheDocument();
  expect(screen.getByRole("radio", { name: "オンライン(API)" })).toBeChecked();
  expect(screen.getByRole("button", { name: "再試行" })).toBeInTheDocument();
  expect(screen.queryByText(/テストほのお|テストモン/)).toBeNull();
});

test("オンラインで一度取得したあと、オフラインへ切り替えるとキャッシュのマスタで画面が出る", async () => {
  const user = userEvent.setup();
  const store = createMemoryMasterCacheStore();
  render(
    <App
      engine={createFakeEngine()}
      masterSources={() => createCachedMasterSources({ online: onlineFake(false), store })}
    />,
  );
  await screen.findByRole("tablist", { name: "画面の切り替え" });
  await waitFor(() => {
    expect(store.peek()).not.toBeNull();
  });

  await user.click(screen.getByRole("radio", { name: "オフライン(WASM)" }));

  await screen.findByRole("tablist", { name: "画面の切り替え" });
  expect(screen.queryByRole("alert")).toBeNull();
  expect(screen.getByRole("radio", { name: "オフライン(WASM)" })).toBeChecked();
});
