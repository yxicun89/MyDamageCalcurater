// P4-16: 計算モードごとのマスタの取得口(ADR-0301 §4「オンラインのときは API からマスタを読む」、ADR-0304 §追記)。
// 確かめること:
//   - masterSources を渡すと、選ばれているモードのマスタだけを読む(もう一方は読まない)
//   - モードを切り替えるとマスタを読み込み直し、読み込みが終わるまで古いモードのマスタで画面を出さない
//   - オンラインのマスタが読めなければ role=alert(自動でオフラインのマスタに戻さない。ADR-0301 §4)
//   - masterSources を渡さない既存の使い方(masterSource 単数・省略)では、モードを切り替えても読み直さない
//   - masterSources には App が持つ端末 ID・セッション ID(ADR-0301 §3)がそのまま渡る

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { App } from "./App";
import type { ClientIds } from "./api/clientIds";
import { CALC_MODE_STORAGE_KEY } from "./app/calcMode";
import { exampleMasterSource } from "./master/exampleSource";
import type { MasterData, MasterSource, MasterSources } from "./master/types";
import { createFakeEngine } from "./test/fakeEngine";

/** RFC 4122 v4 の小文字表記(clientIds.ts が作る形)。 */
const UUID_V4_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

beforeEach(() => {
  window.history.replaceState(null, "", "/");
  window.localStorage.removeItem(CALC_MODE_STORAGE_KEY);
});

afterEach(() => {
  window.history.replaceState(null, "", "/");
  window.localStorage.removeItem(CALC_MODE_STORAGE_KEY);
});

/** オフラインのマスタ(架空の例データ)と、種族名だけを変えた「オンラインの」マスタ。 */
async function masters(): Promise<{ offline: MasterData; online: MasterData }> {
  const offline = await exampleMasterSource.load();
  const online: MasterData = {
    ...offline,
    species: offline.species.map((species) => ({ ...species, nameJa: `オンライン${species.nameJa}` })),
  };
  return { offline, online };
}

/** load() の呼び出し回数を数え、解決を保留できる MasterSource。 */
function countingSource(data: MasterData): { source: MasterSource; load: ReturnType<typeof vi.fn> } {
  const load = vi.fn(() => Promise.resolve(data));
  return { source: { load }, load };
}

/** あとから解決・棄却できる MasterSource(読み込み中の見た目を確かめる)。 */
function deferredSource(): {
  source: MasterSource;
  resolve(data: MasterData): void;
  reject(error: Error): void;
} {
  let resolve: (data: MasterData) => void = () => undefined;
  let reject: (error: Error) => void = () => undefined;
  const promise = new Promise<MasterData>((onResolve, onReject) => {
    resolve = onResolve;
    reject = onReject;
  });
  // App がまだ読んでいない段階(実装前)でも unhandled rejection にしないための空の購読。
  // load() が返すのは元の promise なので、App 側のエラー処理の検査には影響しない。
  promise.catch(() => undefined);
  return { source: { load: () => promise }, resolve, reject };
}

/** 計算モードのラジオ。 */
function modeRadios(): { offline: HTMLElement; online: HTMLElement } {
  const group = screen.getByRole("radiogroup", { name: "計算モード" });
  return {
    offline: within(group).getByRole("radio", { name: "オフライン(WASM)" }),
    online: within(group).getByRole("radio", { name: "オンライン(API)" }),
  };
}

/** 攻撃側のポケモンのセレクトに出ている選択肢の表示名。 */
async function attackerOptionLabels(): Promise<string[]> {
  const select = await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
  return within(select)
    .getAllByRole("option")
    .map((option) => option.textContent);
}

test("既定(オフライン)では offline のマスタだけを読む", async () => {
  const { offline, online } = await masters();
  const offlineSource = countingSource(offline);
  const onlineSource = countingSource(online);
  render(
    <App
      engine={createFakeEngine()}
      masterSources={() => ({ offline: offlineSource.source, online: onlineSource.source })}
    />,
  );

  expect((await attackerOptionLabels())[0]).toBe(offline.species[0]?.nameJa);
  expect(offlineSource.load).toHaveBeenCalledTimes(1);
  expect(onlineSource.load).not.toHaveBeenCalled();
});

test("オンラインに切り替えるとオンラインのマスタを読み、その種族が画面に出る", async () => {
  const user = userEvent.setup();
  const { offline, online } = await masters();
  const offlineSource = countingSource(offline);
  const onlineSource = countingSource(online);
  render(
    <App
      engine={createFakeEngine()}
      masterSources={() => ({ offline: offlineSource.source, online: onlineSource.source })}
    />,
  );
  await attackerOptionLabels();

  await user.click(modeRadios().online);

  await waitFor(() => {
    expect(onlineSource.load).toHaveBeenCalledTimes(1);
  });
  const labels = await attackerOptionLabels();
  expect(labels[0]).toBe(online.species[0]?.nameJa);
  expect(labels[0]?.startsWith("オンライン")).toBe(true);
});

test("切り替え後のマスタを読み込むまでは、前のモードのマスタで画面を出さない", async () => {
  const user = userEvent.setup();
  const { offline, online } = await masters();
  const onlinePending = deferredSource();
  render(
    <App
      engine={createFakeEngine()}
      masterSources={() => ({
        offline: { load: () => Promise.resolve(offline) },
        online: onlinePending.source,
      })}
    />,
  );
  await attackerOptionLabels();

  await user.click(modeRadios().online);

  await waitFor(() => {
    expect(screen.getByText(/読み込み中/)).toBeInTheDocument();
  });
  expect(screen.queryByRole("combobox", { name: "攻撃側のポケモン" })).toBeNull();

  onlinePending.resolve(online);
  expect((await attackerOptionLabels())[0]).toBe(online.species[0]?.nameJa);
});

test("オンラインのマスタを読めなければ role=alert を出す(オフラインのマスタに戻さない)", async () => {
  const user = userEvent.setup();
  const { offline } = await masters();
  const onlinePending = deferredSource();
  render(
    <App
      engine={createFakeEngine()}
      masterSources={() => ({
        offline: { load: () => Promise.resolve(offline) },
        online: onlinePending.source,
      })}
    />,
  );
  await attackerOptionLabels();

  await user.click(modeRadios().online);
  onlinePending.reject(new Error("テストの読み込み失敗"));

  expect(await screen.findByRole("alert")).toBeInTheDocument();
  expect(screen.queryByRole("combobox", { name: "攻撃側のポケモン" })).toBeNull();
});

test("オフラインに戻すと offline のマスタを読み直す", async () => {
  const user = userEvent.setup();
  const { offline, online } = await masters();
  const offlineSource = countingSource(offline);
  const onlineSource = countingSource(online);
  render(
    <App
      engine={createFakeEngine()}
      masterSources={() => ({ offline: offlineSource.source, online: onlineSource.source })}
    />,
  );
  await attackerOptionLabels();

  await user.click(modeRadios().online);
  await waitFor(() => {
    expect(onlineSource.load).toHaveBeenCalledTimes(1);
  });

  await user.click(modeRadios().offline);
  await waitFor(() => {
    expect(offlineSource.load).toHaveBeenCalledTimes(2);
  });
  expect((await attackerOptionLabels())[0]).toBe(offline.species[0]?.nameJa);
});

test("masterSource(単数)だけを渡した既存の使い方では、モードを切り替えてもマスタを読み直さない", async () => {
  const user = userEvent.setup();
  const { offline } = await masters();
  const single = countingSource(offline);
  render(<App engine={createFakeEngine()} masterSource={single.source} />);
  await attackerOptionLabels();

  await user.click(modeRadios().online);
  await waitFor(() => {
    expect(modeRadios().online).toBeChecked();
  });

  expect(single.load).toHaveBeenCalledTimes(1);
  expect((await attackerOptionLabels())[0]).toBe(offline.species[0]?.nameJa);
});

test("masterSources には App の端末 ID・セッション ID(UUID v4・互いに異なる)が渡る", async () => {
  const { offline, online } = await masters();
  const received: ClientIds[] = [];
  const build = (ids: ClientIds): MasterSources => {
    received.push(ids);
    return {
      offline: { load: () => Promise.resolve(offline) },
      online: { load: () => Promise.resolve(online) },
    };
  };
  render(<App engine={createFakeEngine()} masterSources={build} />);
  await attackerOptionLabels();

  const ids = received[0];
  if (ids === undefined) {
    throw new Error("masterSources が呼ばれていない");
  }
  expect(ids.deviceId).toMatch(UUID_V4_PATTERN);
  expect(ids.sessionId).toMatch(UUID_V4_PATTERN);
  expect(ids.deviceId).not.toBe(ids.sessionId);
});
