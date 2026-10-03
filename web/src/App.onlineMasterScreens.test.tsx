// issue 276(ADR-0411): API 専用の画面(タイプバランス・判定)は、計算モードに関係なく常にオンラインのマスタを使う。
// 確かめること:
//   - オフライン(既定)のままタイプバランスを開いても、画面にはオンラインのマスタの種族が出る
//   - そのとき計算画面は引き続きオフラインのマスタ(ダメージ計算の実行場所の選択に従う)
//   - オンラインのマスタを読めないときは、日本語の案内と「再試行」を出す(オフラインの架空データに落とさない)
//   - ヘッダーの切替の名前は「ダメージ計算の実行場所」

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { App } from "./App";
import { CALC_MODE_STORAGE_KEY } from "./app/calcMode";
import { exampleMasterSource } from "./master/exampleSource";
import type { MasterData, MasterSource } from "./master/types";
import { createFakeEngine } from "./test/fakeEngine";

beforeEach(() => {
  window.history.replaceState(null, "", "/");
  // ADR-0313: 既定の計算モードはオンラインになった。このファイルは「オフライン選択中でも API 専用画面は
  // オンラインのマスタを使う」を確かめるので、保存済みモードをオフラインにして始める(テストの意図は不変)。
  window.localStorage.setItem(CALC_MODE_STORAGE_KEY, "offline");
});

afterEach(() => {
  vi.restoreAllMocks();
  window.history.replaceState(null, "", "/");
  window.localStorage.removeItem(CALC_MODE_STORAGE_KEY);
});

async function masters(): Promise<{ offline: MasterData; online: MasterData }> {
  const offline = await exampleMasterSource.load();
  return {
    offline,
    online: {
      ...offline,
      species: offline.species.map((species) => ({ ...species, nameJa: `オンライン${species.nameJa}` })),
    },
  };
}

function source(data: MasterData): { source: MasterSource; load: ReturnType<typeof vi.fn> } {
  const load = vi.fn(() => Promise.resolve(data));
  return { source: { load }, load };
}

async function memberSpeciesLabels(): Promise<string[]> {
  const group = await screen.findByRole("group", { name: "メンバー1" });
  return within(within(group).getByRole("combobox", { name: "ポケモン" }))
    .getAllByRole("option")
    .map((option) => option.textContent);
}

test("オフラインのままタイプバランスを開いても、オンラインのマスタの種族が出る", async () => {
  const user = userEvent.setup();
  const { offline, online } = await masters();
  const offlineSource = source(offline);
  const onlineSource = source(online);
  render(
    <App
      engine={createFakeEngine()}
      masterSources={() => ({ offline: offlineSource.source, online: onlineSource.source })}
    />,
  );
  await screen.findByRole("combobox", { name: "攻撃側のポケモン" });

  await user.click(screen.getByRole("tab", { name: "タイプバランス" }));

  const labels = await memberSpeciesLabels();
  expect(labels.some((label) => label.startsWith("オンライン"))).toBe(true);
  expect(labels).not.toContain(offline.species[0]?.nameJa);
  // 計算画面のマスタはオフラインのまま
  await user.click(screen.getByRole("tab", { name: "計算" }));
  const attacker = await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
  expect(within(attacker).getAllByRole("option")[0]?.textContent).toBe(offline.species[0]?.nameJa);
});

test("オンラインのマスタを読めないときは、日本語の案内と再試行を出し、英語の原因は出さない", async () => {
  const user = userEvent.setup();
  const { offline, online } = await masters();
  const onlineLoad = vi
    .fn<() => Promise<MasterData>>()
    .mockRejectedValueOnce(new Error("fetch failed: connection refused"))
    .mockResolvedValue(online);
  render(
    <App
      engine={createFakeEngine()}
      masterSources={() => ({ offline: source(offline).source, online: { load: onlineLoad } })}
    />,
  );
  await screen.findByRole("combobox", { name: "攻撃側のポケモン" });

  await user.click(screen.getByRole("tab", { name: "タイプバランス" }));

  const alert = await screen.findByRole("alert");
  expect(alert).toHaveTextContent("オンラインのマスタを読み込めませんでした");
  expect(alert).not.toHaveTextContent("connection refused");
  await user.click(within(alert).getByRole("button", { name: "再試行" }));
  await waitFor(async () => {
    expect((await memberSpeciesLabels()).some((label) => label.startsWith("オンライン"))).toBe(true);
  });
});

test("ヘッダーの切替は「ダメージ計算の実行場所」という名前", async () => {
  render(<App engine={createFakeEngine()} />);
  expect(
    await within(screen.getByRole("banner")).findByRole("radiogroup", { name: "ダメージ計算の実行場所" }),
  ).toBeInTheDocument();
});

test("読めなかった後に再試行を押すと、次の取得が成功して画面が出る(取得は計2回)", async () => {
  const user = userEvent.setup();
  const { offline, online } = await masters();
  const onlineLoad = vi
    .fn<() => Promise<MasterData>>()
    .mockRejectedValueOnce(new Error("boom"))
    .mockResolvedValue(online);
  render(
    <App
      engine={createFakeEngine()}
      masterSources={() => ({ offline: source(offline).source, online: { load: onlineLoad } })}
    />,
  );
  await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
  await user.click(screen.getByRole("tab", { name: "タイプバランス" }));
  const alert = await screen.findByRole("alert");

  await user.click(within(alert).getByRole("button", { name: "再試行" }));

  expect(await screen.findByRole("group", { name: "メンバー1" })).toBeInTheDocument();
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  expect(onlineLoad).toHaveBeenCalledTimes(2);
});

test("取得の完了前にタブを離れても、後から解決して状態更新や警告が出ない", async () => {
  const user = userEvent.setup();
  const { offline, online } = await masters();
  let resolveOnline: (data: MasterData) => void = () => undefined;
  const onlineLoad = vi.fn(
    () =>
      new Promise<MasterData>((resolve) => {
        resolveOnline = resolve;
      }),
  );
  const errorSpy = vi.spyOn(console, "error").mockImplementation(() => undefined);
  render(
    <App
      engine={createFakeEngine()}
      masterSources={() => ({ offline: source(offline).source, online: { load: onlineLoad } })}
    />,
  );
  await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
  await user.click(screen.getByRole("tab", { name: "タイプバランス" }));
  await waitFor(() => {
    expect(onlineLoad).toHaveBeenCalledTimes(1);
  });
  await user.click(screen.getByRole("tab", { name: "計算" }));

  await act(async () => {
    resolveOnline(online);
    await Promise.resolve();
  });

  expect(errorSpy).not.toHaveBeenCalled();
  expect(screen.queryByRole("group", { name: "メンバー1" })).not.toBeInTheDocument();
});

test("判定の画面も包まれていて、オフラインのままでもオンラインのマスタの種族が渡る", async () => {
  const user = userEvent.setup();
  const { offline, online } = await masters();
  render(
    <App
      engine={createFakeEngine()}
      masterSources={() => ({ offline: source(offline).source, online: source(online).source })}
    />,
  );
  await screen.findByRole("combobox", { name: "攻撃側のポケモン" });

  await user.click(screen.getByRole("tab", { name: "判定" }));

  const region = await screen.findByRole("region", { name: "自分のポケモン" });
  const labels = within(within(region).getByRole("combobox", { name: "ポケモン" }))
    .getAllByRole("option")
    .map((option) => option.textContent);
  expect(labels.some((label) => label.startsWith("オンライン"))).toBe(true);
  expect(labels).not.toContain(offline.species[0]?.nameJa);
});
