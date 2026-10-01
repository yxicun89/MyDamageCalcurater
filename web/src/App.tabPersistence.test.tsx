// issue #218(ADR-0308): タブを切り替えても各画面の入力が消えないこと。
//
// 今の App.tsx は `SCREEN_COMPONENTS[tab]` で選択中の画面だけを描くので、タブを離れるたびに
// その画面が unmount され、種族・技・持ち物・プリセット・観測がすべて初期値に戻る。
// ADR-0308 の決定:
//   1. 一度でも選ばれたタブの画面だけを mount し、以後 unmount しない(未訪問の画面は mount しない。
//      SpeedScreen はマウント時に speed API を2本呼ぶため〈ADR-0604 §2〉、全画面の先読みはしない)
//   2. 非選択の画面はネイティブの `hidden` 属性で隠す(DOM には残るが、アクセシビリティツリーからは外れる。
//      既存の `queryByRole(...).toBeNull()` がそのまま成立し続ける)。`role="tabpanel"` は1つのまま
//   3. マスタが入れ替わったら(計算モードの切り替え)、マスタを使う画面は作り直す(入力は初期化される)
//   4. 保持はセッション内だけ(リロードで初期状態。sessionStorage/localStorage に入力を書かない)
//
// このファイルのテストのうち「保持」を確かめるものは実装前は red、
// 「壊していないこと」を確かめるもの(未訪問の画面の通信が無い・マスタ入れ替えで初期化される・
// リロードで初期化される・tabpanel が1つ)は実装前から green の回帰ガード。

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { App } from "./App";
import { teamScreenText } from "./i18n/ja";
import { exampleMasterSource } from "./master/exampleSource";
import type { MasterData, MasterSource } from "./master/types";
import { createFakeEngine } from "./test/fakeEngine";

beforeEach(() => {
  window.history.replaceState(null, "", "/");
  window.localStorage.clear();
  window.sessionStorage.clear();
});

afterEach(() => {
  vi.restoreAllMocks();
  window.history.replaceState(null, "", "/");
  window.localStorage.clear();
  window.sessionStorage.clear();
});

/** 例データから、テストで選ぶ種族2体と持ち物1つを取り出す。 */
async function examplePicks(): Promise<{
  master: MasterData;
  attackerKey: string;
  defenderKey: string;
  itemId: string;
}> {
  const master = await exampleMasterSource.load();
  const [attacker, defender] = master.species;
  const [item] = master.items;
  if (attacker === undefined || defender === undefined || item === undefined) {
    throw new Error("例データに種族2体と持ち物1つ以上が要る");
  }
  return { master, attackerKey: attacker.key, defenderKey: defender.key, itemId: item.id };
}

/** ブラウザの戻る・進むと同じく、URL を変えてから popstate を送る(App.routing.test.tsx と同じ形)。 */
function simulatePopState(path: string): void {
  act(() => {
    window.history.replaceState(null, "", path);
    window.dispatchEvent(new PopStateEvent("popstate", { state: null }));
  });
}

function calcCombobox(name: string): HTMLElement {
  return screen.getByRole("combobox", { name });
}

function tabButton(name: string): HTMLElement {
  return screen.getByRole("tab", { name });
}

/**
 * select の value(選択肢が1つも無い select は `toHaveValue("")` が undefined になるので、
 * 値そのものを見る)。
 */
function selectValue(name: string): string {
  const element = calcCombobox(name);
  if (!(element instanceof HTMLSelectElement)) {
    throw new Error(`「${name}」は select ではない`);
  }
  return element.value;
}

/** fetch の呼び出しのうち speed API(api/speed/…)のものの件数(App.test.tsx の urlOf と同じ形)。 */
function speedCallCount(calls: readonly (readonly [string | URL | Request, ...unknown[]])[]): number {
  const urlOf = (input: string | URL | Request): string =>
    input instanceof Request ? input.url : input instanceof URL ? input.href : input;
  return calls.filter(([input]) => urlOf(input).includes("api/speed/")).length;
}

/** fetch の呼び出しのうち構築 API(api/team/…)のものの件数(P5-5 PR-A1。ADR-0309 §4)。 */
function teamCallCount(calls: readonly (readonly [string | URL | Request, ...unknown[]])[]): number {
  const urlOf = (input: string | URL | Request): string =>
    input instanceof Request ? input.url : input instanceof URL ? input.href : input;
  return calls.filter(([input]) => urlOf(input).includes("api/team/")).length;
}

describe("issue #218 タブを往復しても入力が残る(同じマスタで開いている間)", () => {
  test("計算タブの種族・持ち物・技は、逆算タブへ行って戻っても残る", async () => {
    const { attackerKey, defenderKey, itemId } = await examplePicks();
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);

    await user.selectOptions(await screen.findByRole("combobox", { name: "攻撃側のポケモン" }), attackerKey);
    await user.selectOptions(calcCombobox("防御側のポケモン"), defenderKey);
    await user.selectOptions(calcCombobox("攻撃側の持ち物"), itemId);
    // 攻撃側を選ぶと技が自動で選ばれる(CalcScreen の resolveMoveId)。その値も往復後に残ること。
    const moveIdBefore = selectValue("技");
    expect(moveIdBefore).not.toBe("");

    await user.click(tabButton("逆算"));
    // 既存の保証(App.test.tsx): 別のタブの入力欄はアクセシビリティツリーから取れない。
    expect(screen.queryByRole("combobox", { name: "攻撃側のポケモン" })).toBeNull();
    await screen.findByRole("combobox", { name: "自分のポケモン" });

    await user.click(tabButton("計算"));

    expect(await screen.findByRole("combobox", { name: "攻撃側のポケモン" })).toHaveValue(attackerKey);
    expect(calcCombobox("防御側のポケモン")).toHaveValue(defenderKey);
    expect(calcCombobox("攻撃側の持ち物")).toHaveValue(itemId);
    expect(calcCombobox("技")).toHaveValue(moveIdBefore);
  });

  test("逆算タブの種族と観測は、計算タブへ行って戻っても残る", async () => {
    const { attackerKey, defenderKey } = await examplePicks();
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });

    await user.click(tabButton("逆算"));
    await user.selectOptions(await screen.findByRole("combobox", { name: "自分のポケモン" }), attackerKey);
    await user.selectOptions(calcCombobox("相手のポケモン"), defenderKey);
    await user.type(screen.getByRole("textbox", { name: "観測1" }), "45");

    await user.click(tabButton("計算"));
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
    await user.click(tabButton("逆算"));

    expect(await screen.findByRole("combobox", { name: "自分のポケモン" })).toHaveValue(attackerKey);
    expect(calcCombobox("相手のポケモン")).toHaveValue(defenderKey);
    expect(screen.getByRole("textbox", { name: "観測1" })).toHaveValue("45");
  });

  test("3つ以上のタブを回っても、それぞれの入力がそのまま残る", async () => {
    const { attackerKey, defenderKey } = await examplePicks();
    // 素早さの画面は speed-svc に届かないが、画面は壊れない(ADR-0604 §4)。
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("Failed to fetch"));
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);

    await user.selectOptions(await screen.findByRole("combobox", { name: "攻撃側のポケモン" }), attackerKey);
    await user.click(tabButton("逆算"));
    await user.selectOptions(await screen.findByRole("combobox", { name: "自分のポケモン" }), defenderKey);
    await user.click(tabButton("素早さ"));
    await screen.findByRole("region", { name: "自分のポケモン" });
    await user.click(tabButton("判定"));
    await user.click(tabButton("計算"));

    expect(await screen.findByRole("combobox", { name: "攻撃側のポケモン" })).toHaveValue(attackerKey);
    await user.click(tabButton("逆算"));
    expect(await screen.findByRole("combobox", { name: "自分のポケモン" })).toHaveValue(defenderKey);
  });

  test("ブラウザの戻る・進む(popstate)でも入力が残る", async () => {
    const { attackerKey, defenderKey } = await examplePicks();
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);

    await user.selectOptions(await screen.findByRole("combobox", { name: "攻撃側のポケモン" }), attackerKey);
    await user.selectOptions(calcCombobox("防御側のポケモン"), defenderKey);

    // タブのクリックで /reverse を pushState(P4-10)。
    await user.click(tabButton("逆算"));
    expect(window.location.pathname).toBe("/reverse");
    await user.type(await screen.findByRole("textbox", { name: "観測1" }), "45");

    // 戻る → 計算タブ。入力は消えていない。
    simulatePopState("/calc");
    expect(tabButton("計算")).toHaveAttribute("aria-selected", "true");
    expect(await screen.findByRole("combobox", { name: "攻撃側のポケモン" })).toHaveValue(attackerKey);
    expect(calcCombobox("防御側のポケモン")).toHaveValue(defenderKey);

    // 進む → 逆算タブ。こちらの入力も消えていない。
    simulatePopState("/reverse");
    expect(tabButton("逆算")).toHaveAttribute("aria-selected", "true");
    expect(await screen.findByRole("textbox", { name: "観測1" })).toHaveValue("45");
  });
});

// P5-5 PR-A1(ADR-0309 §4): 新しい「構築」のタブも、ADR-0308 の決まりに乗る
// (訪れるまで mount しない = 構築 API を呼ばない / 往復しても入力が消えない)。
describe("issue #218 構築のタブも同じ決まりに乗る(P5-5 PR-A1)", () => {
  test("構築のタブを開くまで構築 API を呼ばない", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("Failed to fetch"));
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });

    await user.click(tabButton("逆算"));
    await screen.findByRole("combobox", { name: "自分のポケモン" });

    expect(teamCallCount(fetchMock.mock.calls)).toBe(0);
  });

  test("構築名の入力は、計算タブへ行って戻っても残り、構築 API を呼び直さない", async () => {
    // team-svc は居ないので一覧は失敗するが、新規作成の入力は先に使える(ADR-0309 §4)。
    const fetchMock = vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("Failed to fetch"));
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });

    await user.click(tabButton("構築"));
    const nameField = await screen.findByRole("textbox", { name: teamScreenText.nameLabel });
    await user.type(nameField, "テスト構築C");
    await waitFor(() => {
      expect(teamCallCount(fetchMock.mock.calls)).toBeGreaterThan(0);
    });
    const callsAfterFirstVisit = teamCallCount(fetchMock.mock.calls);

    await user.click(tabButton("計算"));
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
    await user.click(tabButton("構築"));

    expect(await screen.findByRole("textbox", { name: teamScreenText.nameLabel })).toHaveValue("テスト構築C");
    expect(teamCallCount(fetchMock.mock.calls)).toBe(callsAfterFirstVisit);
  });
});

describe("issue #218 隠し方(ADR-0308 決定2: hidden 属性・tabpanel は1つ)", () => {
  test("非選択の画面は DOM に残るが、アクセシビリティツリーからは外れる", async () => {
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });

    await user.click(tabButton("逆算"));
    await screen.findByRole("combobox", { name: "自分のポケモン" });
    await user.click(tabButton("計算"));
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });

    // 逆算の入力欄は「取れない」(既存のアサーションと同じ)が、DOM には残っている
    // (= unmount されていない。これが入力が消えない理由)。
    expect(screen.queryByRole("combobox", { name: "自分のポケモン" })).toBeNull();
    expect(screen.getByRole("combobox", { name: "自分のポケモン", hidden: true })).toBeInTheDocument();
  });

  test("role=tabpanel は1つのままで、すべてのタブの aria-controls がそれを指す", async () => {
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
    await user.click(tabButton("逆算"));
    await screen.findByRole("combobox", { name: "自分のポケモン" });

    // 隠れたパネルを増やしていない(hidden: true でも1つ)。
    expect(screen.getAllByRole("tabpanel", { hidden: true })).toHaveLength(1);
    const panel = screen.getByRole("tabpanel");
    const list = screen.getByRole("tablist", { name: "画面の切り替え" });
    for (const tab of within(list).getAllByRole("tab")) {
      expect(tab).toHaveAttribute("aria-controls", panel.id);
    }
    // aria-labelledby は選択中のタブを指す(配線は今までどおり)。
    expect(panel).toHaveAttribute("aria-labelledby", tabButton("逆算").id);
  });
});

describe("issue #218 未訪問の画面は mount しない(ADR-0308 決定1)", () => {
  test("素早さのタブを開くまで speed API を呼ばない(全画面を先に mount しない)", async () => {
    const { attackerKey } = await examplePicks();
    const fetchMock = vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("Failed to fetch"));
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);

    await user.selectOptions(await screen.findByRole("combobox", { name: "攻撃側のポケモン" }), attackerKey);
    await user.click(tabButton("逆算"));
    await screen.findByRole("combobox", { name: "自分のポケモン" });

    // SpeedScreen はマウント時に pokemon() と table() を呼ぶ(ADR-0604 §2)。まだ開いていないので0件。
    expect(speedCallCount(fetchMock.mock.calls)).toBe(0);
  });

  test("素早さのタブを離れて戻っても、speed API を呼び直さない(mount したまま保つ)", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("Failed to fetch"));
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });

    await user.click(tabButton("素早さ"));
    await screen.findByRole("region", { name: "自分のポケモン" });
    await waitFor(() => {
      expect(speedCallCount(fetchMock.mock.calls)).toBeGreaterThan(0);
    });
    const callsAfterFirstVisit = speedCallCount(fetchMock.mock.calls);

    await user.click(tabButton("計算"));
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
    await user.click(tabButton("素早さ"));
    await screen.findByRole("region", { name: "自分のポケモン" });

    expect(speedCallCount(fetchMock.mock.calls)).toBe(callsAfterFirstVisit);
  });
});

describe("issue #218 異常系: マスタが入れ替わったら作り直す(ADR-0308 決定3)", () => {
  /** オフラインの例データと、種族の key ごと差し替えた「オンラインの」マスタ。 */
  async function swappedMasters(): Promise<{ offline: MasterData; online: MasterData }> {
    const offline = await exampleMasterSource.load();
    const online: MasterData = {
      ...offline,
      species: offline.species.map((species) => ({
        ...species,
        key: `online-${species.key}`,
        nameJa: `オンライン${species.nameJa}`,
      })),
    };
    return { offline, online };
  }

  function modeRadio(name: string): HTMLElement {
    const group = screen.getByRole("radiogroup", { name: "計算モード" });
    return within(group).getByRole("radio", { name });
  }

  test("計算モードを切り替えてマスタが入れ替わると、マスタに無い種族が選ばれたまま残らない", async () => {
    const { offline, online } = await swappedMasters();
    const offlineSource: MasterSource = { load: () => Promise.resolve(offline) };
    const onlineSource: MasterSource = { load: () => Promise.resolve(online) };
    const [item] = offline.items;
    const [attacker, defender] = offline.species;
    if (item === undefined || attacker === undefined || defender === undefined) {
      throw new Error("例データに種族2体と持ち物が要る");
    }
    const user = userEvent.setup();
    render(
      <App
        engine={createFakeEngine()}
        masterSources={() => ({ offline: offlineSource, online: onlineSource })}
      />,
    );

    await user.selectOptions(await screen.findByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
    await user.selectOptions(calcCombobox("攻撃側の持ち物"), item.id);
    // 攻撃側・防御側の両方を選び、切り替え前に実際に計算結果を出しておく(critic指摘: 結果を一度も
    // 出さずに queryByRole(...).toBeNull() を確かめても空振りのアサーションにしかならない)。
    await user.selectOptions(calcCombobox("防御側のポケモン"), defender.key);
    expect(calcCombobox("攻撃側のポケモン")).toHaveValue(attacker.key);
    await screen.findByRole("list", { name: "計算結果" });

    await user.click(modeRadio("オンライン(API)"));

    // 新しいマスタの種族が並ぶまで待つ(攻撃側の select の中だけを見る)。
    await waitFor(() => {
      expect(
        within(calcCombobox("攻撃側のポケモン")).getByRole("option", {
          name: `オンライン${attacker.nameJa}`,
        }),
      ).toBeInTheDocument();
    });
    // 古いマスタの key が選ばれたまま残っていない(ADR-0308 決定3: 画面ごと作り直す)。
    expect(calcCombobox("攻撃側のポケモン")).toHaveValue("");
    expect(calcCombobox("攻撃側の持ち物")).toHaveValue("");
    expect(selectValue("技")).toBe("");
    // 古いマスタで計算した結果(切り替え直前に実際に出していたもの)も残らない。
    expect(screen.queryByRole("list", { name: "計算結果" })).toBeNull();
  });

  test("オフラインに戻しても、前のマスタの入力を復活させない", async () => {
    const { offline, online } = await swappedMasters();
    const [attacker] = offline.species;
    if (attacker === undefined) {
      throw new Error("例データに種族が要る");
    }
    const user = userEvent.setup();
    render(
      <App
        engine={createFakeEngine()}
        masterSources={() => ({
          offline: { load: () => Promise.resolve(offline) },
          online: { load: () => Promise.resolve(online) },
        })}
      />,
    );

    await user.selectOptions(await screen.findByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
    await user.click(modeRadio("オンライン(API)"));
    await waitFor(() => {
      expect(
        within(calcCombobox("攻撃側のポケモン")).getByRole("option", {
          name: `オンライン${attacker.nameJa}`,
        }),
      ).toBeInTheDocument();
    });

    await user.click(modeRadio("オフライン(WASM)"));

    await waitFor(() => {
      expect(
        within(calcCombobox("攻撃側のポケモン")).getByRole("option", { name: attacker.nameJa }),
      ).toBeInTheDocument();
    });
    expect(calcCombobox("攻撃側のポケモン")).toHaveValue("");
  });

  test("素早さタブを訪問後にマスタが入れ替わっても、speed API を呼び直さない(hidden のまま再マウントしない)", async () => {
    // critic の実測(#218 FAIL 指摘): 素早さタブを一度開く→計算タブへ戻る→計算モードを切り替える、で
    // 呼び出し件数が2件→4件に増えていた。visitedTabs が App の state のままだと、マスタ再読み込みで
    // .app-tabs サブツリーが作り直されるとき、訪問済みの素早さも含めて全部 hidden のまま再マウントされ、
    // SpeedScreen のマウント時 speed API 呼び出しが再度走っていた。
    const { offline, online } = await swappedMasters();
    const fetchMock = vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("Failed to fetch"));
    const user = userEvent.setup();
    render(
      <App
        engine={createFakeEngine()}
        masterSources={() => ({
          offline: { load: () => Promise.resolve(offline) },
          online: { load: () => Promise.resolve(online) },
        })}
      />,
    );
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });

    // 素早さタブを一度開く(speed API が呼ばれる)。
    await user.click(tabButton("素早さ"));
    await screen.findByRole("region", { name: "自分のポケモン" });
    await waitFor(() => {
      expect(speedCallCount(fetchMock.mock.calls)).toBeGreaterThan(0);
    });
    const callsAfterSpeedVisit = speedCallCount(fetchMock.mock.calls);

    // 計算タブへ戻ってから、計算モードを切り替えてマスタを入れ替える。
    await user.click(tabButton("計算"));
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
    await user.click(modeRadio("オンライン(API)"));
    await waitFor(() => {
      expect(calcCombobox("攻撃側のポケモン")).toHaveValue("");
    });

    // 素早さは非選択(hidden)のまま。訪問済みタブは選択中のタブだけへリセットされ、素早さは
    // 再訪問するまで mount されないので、呼び出し件数は増えない。
    expect(speedCallCount(fetchMock.mock.calls)).toBe(callsAfterSpeedVisit);
  });
});

describe("issue #218 境界値: 保持はセッション内だけ(ADR-0308 決定4)", () => {
  test("読み込み直す(unmount して描き直す)と入力は初期状態に戻り、ストレージにも残らない", async () => {
    const { attackerKey } = await examplePicks();
    const user = userEvent.setup();
    const { unmount } = render(<App engine={createFakeEngine()} />);

    await user.selectOptions(await screen.findByRole("combobox", { name: "攻撃側のポケモン" }), attackerKey);
    expect(calcCombobox("攻撃側のポケモン")).toHaveValue(attackerKey);
    unmount();

    render(<App engine={createFakeEngine()} />);
    expect(await screen.findByRole("combobox", { name: "攻撃側のポケモン" })).toHaveValue("");

    // 入力はストレージに書かない(localStorage に入れるのは計算モードと端末 ID だけ。ADR-0308 決定4)。
    expect(window.sessionStorage.length).toBe(0);
    for (const key of Object.keys(window.localStorage)) {
      expect(window.localStorage.getItem(key)).not.toContain(attackerKey);
    }
  });
});
