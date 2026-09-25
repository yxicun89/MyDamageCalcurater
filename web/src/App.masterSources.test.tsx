// P4-16: 計算モードごとのマスタの取得口(ADR-0301 §4「オンラインのときは API からマスタを読む」、ADR-0304 §追記)。
// 確かめること:
//   - masterSources を渡すと、選ばれているモードのマスタだけを読む(もう一方は読まない)
//   - モードを切り替えるとマスタを読み込み直し、読み込みが終わるまで古いモードのマスタで画面を出さない
//   - オンラインのマスタが読めなければ role=alert(自動でオフラインのマスタに戻さない。ADR-0301 §4)
//   - masterSources を渡さない既存の使い方(masterSource 単数・省略)では、モードを切り替えても読み直さない
//   - masterSources には App が持つ端末 ID・セッション ID(ADR-0301 §3)がそのまま渡る
//
// issue #308: マスタが読めないときの立て直し。失敗しても画面から抜け出せることを確かめる。
//   - 失敗の表示に原因(Error の message)と「再試行」を出し、タブ一覧は残す
//   - マスタを使わない「素早さ」の画面は、失敗中でもタブを選んで使える(app/screens.tsx の ScreenProps)
//   - 「再試行」で同じ取得口を読み直し、成功すればその場で画面が出る(リロード不要)
//   - オンラインのときだけ「オフラインに切り替える」を出す(自動では切り替えない。ADR-0301 §4)
//   - 保存済みのモードがオンラインのまま起動して失敗しても、同じ手段で抜けられる(calcMode.ts)

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
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
  vi.restoreAllMocks();
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

/**
 * issue #308: 呼ばれるたびに、渡した順で失敗(Error)か成功(MasterData)を返す MasterSource。
 * 用意した分を使い切ったあとは最後の指定を繰り返す(再試行を何度押しても破綻しないように)。
 */
function scriptedSource(steps: readonly (MasterData | Error)[]): {
  source: MasterSource;
  load: ReturnType<typeof vi.fn>;
} {
  let calls = 0;
  const load = vi.fn(() => {
    const step = steps[Math.min(calls, steps.length - 1)];
    calls += 1;
    if (step === undefined) {
      throw new Error("scriptedSource に手順が1つも渡っていない");
    }
    return step instanceof Error ? Promise.reject(step) : Promise.resolve(step);
  });
  return { source: { load }, load };
}

/** マスタの読み込みに失敗したときの案内にある操作(issue #308)。 */
function retryButton(): HTMLElement {
  return screen.getByRole("button", { name: "再試行" });
}

function switchToOfflineButton(): HTMLElement {
  return screen.getByRole("button", { name: "オフラインに切り替える" });
}

/** タブ一覧に出ている画面名(issue #308: 失敗中も消えないことを確かめる)。 */
function tabLabels(): string[] {
  const list = screen.getByRole("tablist", { name: "画面の切り替え" });
  return within(list)
    .getAllByRole("tab")
    .map((tab) => tab.textContent);
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

test("オンラインのマスタを読めなければ role=alert・原因・立て直しの操作を出す(オフラインのマスタに戻さない)", async () => {
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

  const alert = await screen.findByRole("alert");
  expect(alert).toBeInTheDocument();
  // 自動でオフラインのマスタに戻さない(ADR-0301 §4)。計算画面は出さないまま。
  expect(screen.queryByRole("combobox", { name: "攻撃側のポケモン" })).toBeNull();
  expect(modeRadios().online).toBeChecked();
  // issue #308: 原因(Error の message)を握りつぶさず、次の一手を画面に出す。
  expect(within(alert).getByText(/テストの読み込み失敗/)).toBeInTheDocument();
  expect(retryButton()).toBeInTheDocument();
  expect(switchToOfflineButton()).toBeInTheDocument();
});

// issue #308: マスタが読めないとき、タブ一覧ごと消えて1行のエラーだけになり、再試行も案内も無かった。
// 失敗しても「タブは残る」「マスタを使わない画面は使える」「その場で読み直せる」ことを確かめる。
describe("issue #308 マスタの読み込みに失敗したときの立て直し", () => {
  /** オンラインのマスタが1回目だけ失敗する App を描画し、失敗の表示が出るまで待つ。 */
  async function renderOnlineFailure(
    steps: readonly (MasterData | Error)[],
  ): Promise<{ user: ReturnType<typeof userEvent.setup>; online: ReturnType<typeof scriptedSource> }> {
    const user = userEvent.setup();
    const { offline } = await masters();
    const online = scriptedSource(steps);
    render(
      <App
        engine={createFakeEngine()}
        masterSources={() => ({ offline: { load: () => Promise.resolve(offline) }, online: online.source })}
      />,
    );
    await attackerOptionLabels();
    await user.click(modeRadios().online);
    await screen.findByRole("alert");
    return { user, online };
  }

  test("失敗してもタブ一覧は消えない(5つの画面すべてが選べる)", async () => {
    await renderOnlineFailure([new Error("テストの読み込み失敗")]);

    expect(tabLabels()).toEqual(["計算", "逆算", "タイプバランス", "素早さ", "判定"]);
  });

  test("失敗中でも、マスタを使わない「素早さ」の画面はタブを選んで使える", async () => {
    // speed-svc は居ないので通信は失敗させる(画面はそれでも壊れない。ADR-0604 §4)。
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("Failed to fetch"));
    const { user } = await renderOnlineFailure([new Error("テストの読み込み失敗")]);

    await user.click(screen.getByRole("tab", { name: "素早さ" }));

    expect(await screen.findByRole("region", { name: "自分のポケモン" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "素早さ" })).toHaveAttribute("aria-selected", "true");
    expect(window.location.pathname).toBe("/speed");

    // マスタを使う画面へ戻ると、また失敗の案内が出る(黙って古いマスタで描かない)。
    await user.click(screen.getByRole("tab", { name: "計算" }));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "攻撃側のポケモン" })).toBeNull();
  });

  test("「再試行」で同じ取得口を読み直し、成功すればその場で画面が出る", async () => {
    const { online: onlineMaster } = await masters();
    const { user, online } = await renderOnlineFailure([new Error("テストの読み込み失敗"), onlineMaster]);
    expect(online.load).toHaveBeenCalledTimes(1);

    await user.click(retryButton());

    await waitFor(() => {
      expect(online.load).toHaveBeenCalledTimes(2);
    });
    expect((await attackerOptionLabels())[0]).toBe(onlineMaster.species[0]?.nameJa);
    // 読み直しただけで、モードはオンラインのまま(オフラインへ落ちない)。
    expect(modeRadios().online).toBeChecked();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  test("「再試行」がまた失敗しても案内は残り、もう一度押せる", async () => {
    const { user, online } = await renderOnlineFailure([new Error("テストの読み込み失敗")]);

    await user.click(retryButton());

    await waitFor(() => {
      expect(online.load).toHaveBeenCalledTimes(2);
    });
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(retryButton()).toBeInTheDocument();
    expect(tabLabels()).toHaveLength(5);
  });

  test("「オフラインに切り替える」でオフラインのマスタに戻り、選択も保存される", async () => {
    const { offline } = await masters();
    const { user } = await renderOnlineFailure([new Error("テストの読み込み失敗")]);

    await user.click(switchToOfflineButton());

    expect((await attackerOptionLabels())[0]).toBe(offline.species[0]?.nameJa);
    expect(modeRadios().offline).toBeChecked();
    expect(window.localStorage.getItem(CALC_MODE_STORAGE_KEY)).toBe("offline");
  });

  test("保存済みのモードがオンラインのまま起動して失敗しても、タブと再試行で抜けられる", async () => {
    // リロードしても同じ失敗画面から抜けられない、という issue #308 の再現条件。
    window.localStorage.setItem(CALC_MODE_STORAGE_KEY, "online");
    const user = userEvent.setup();
    const { offline, online: onlineMaster } = await masters();
    const online = scriptedSource([new Error("テストの読み込み失敗"), onlineMaster]);
    render(
      <App
        engine={createFakeEngine()}
        masterSources={() => ({ offline: { load: () => Promise.resolve(offline) }, online: online.source })}
      />,
    );

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(tabLabels()).toHaveLength(5);

    await user.click(retryButton());

    expect((await attackerOptionLabels())[0]).toBe(onlineMaster.species[0]?.nameJa);
  });

  test("オフラインのマスタが読めないときは「オフラインに切り替える」を出さない(再試行だけ)", async () => {
    const offlineSource = scriptedSource([new Error("テストの読み込み失敗")]);
    render(
      <App
        engine={createFakeEngine()}
        masterSources={() => ({
          offline: offlineSource.source,
          online: { load: () => Promise.reject(new Error("呼ばれない")) },
        })}
      />,
    );

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(retryButton()).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "オフラインに切り替える" })).toBeNull();
    expect(tabLabels()).toHaveLength(5);
  });
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
