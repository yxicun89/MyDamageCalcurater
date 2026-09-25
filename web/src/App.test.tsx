import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { App } from "./App";
import { CALC_MODE_STORAGE_KEY } from "./app/calcMode";
import { exampleMasterSource } from "./master/exampleSource";
import type { MasterData, MasterSource } from "./master/types";
import { createFakeEngine, engineError } from "./test/fakeEngine";

// P4-10: タブの選択が URL(History API)と連動するようになったため(App.routing.test.tsx が詳細を確かめる)、
// このファイルの各テストは既定の「計算」タブ(パス "/")から始まる前提を置く。テストの間で URL が
// 漏れないよう、前後で "/" に戻す。
beforeEach(() => {
  window.history.replaceState(null, "", "/");
});

afterEach(() => {
  window.history.replaceState(null, "", "/");
});

test("アプリが描画される", () => {
  render(<App />);
  expect(screen.getByRole("main")).toBeInTheDocument();
});

// P4-2: App はヘッダーと計算画面を出す。マスタは MasterSource(既定は架空の例データ。ADR-0300 §3)から、
// 計算は CalcEngine(既定は browserWasmLoader の WASM 実装。ADR-0300 §2)から受け取り、テストでは差し替える。
describe("P4-2 計算画面の組み込み", () => {
  function deferredMasterSource(): {
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
    return { source: { load: () => promise }, resolve, reject };
  }

  test("ヘッダー(banner)を出す", () => {
    render(<App engine={createFakeEngine()} masterSource={deferredMasterSource().source} />);
    expect(screen.getByRole("banner")).toBeInTheDocument();
  });

  test("マスタの読み込み中は「読み込み中」を出し、読み込めたら計算画面を出す", async () => {
    const master = deferredMasterSource();
    render(<App engine={createFakeEngine()} masterSource={master.source} />);
    expect(screen.getByText(/読み込み中/)).toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "攻撃側のポケモン" })).toBeNull();

    master.resolve(await exampleMasterSource.load());
    expect(await screen.findByRole("combobox", { name: "攻撃側のポケモン" })).toBeInTheDocument();
    expect(screen.queryByText(/読み込み中/)).toBeNull();
  });

  test("マスタを読み込めなければ role=alert で知らせる", async () => {
    const master = deferredMasterSource();
    render(<App engine={createFakeEngine()} masterSource={master.source} />);
    master.reject(new Error("テストの読み込み失敗"));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });

  test("既定のマスタは架空の例データ(props なしでも計算画面が出る)", async () => {
    render(<App />);
    expect(await screen.findByRole("combobox", { name: "攻撃側のポケモン" })).toBeInTheDocument();
  });

  test("既定の engine(WASM)でも、描画〜マスタ読み込みの間は engine.wasm を読まない(計算するまで遅らせる。ADR-0300 §2)", async () => {
    // fetch(/engine.wasm の取得)と script 要素の追加(/wasm_exec.js の読み込み)のどちらも
    // 計算を始めるまで起きないはず。実際に監視して確かめる(読み込みタイミングの実装が壊れたら検出する)。
    const fetchSpy = vi.spyOn(globalThis, "fetch");
    const headAppendSpy = vi.spyOn(document.head, "append");
    const headAppendChildSpy = vi.spyOn(document.head, "appendChild");

    render(<App />);
    expect(await screen.findByRole("combobox", { name: "攻撃側のポケモン" })).toBeInTheDocument();

    expect(fetchSpy).not.toHaveBeenCalled();
    expect(headAppendSpy).not.toHaveBeenCalled();
    expect(headAppendChildSpy).not.toHaveBeenCalled();
  });

  test("engine を渡すと CalcScreen に引き継がれ、攻撃側・防御側を選ぶと fake の calcBulk が呼ばれる", async () => {
    const engine = createFakeEngine();
    const master = await exampleMasterSource.load();
    const [attacker, defender] = master.species;
    if (attacker === undefined || defender === undefined) {
      throw new Error("例データに種族が2つ以上要る");
    }
    const user = userEvent.setup();
    render(<App engine={engine} />);

    await user.selectOptions(await screen.findByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
    await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), defender.key);

    await waitFor(() => {
      expect(engine.bulkRequests).toHaveLength(1);
    });
    expect(engine.bulkRequests[0]?.defenderSpecies.key).toBe(defender.key);
  });
});

// P4-4: 計算と逆算の切り替え(ADR-0300 §7)。タブ(role=tab)「計算」「逆算」で切り替え、既定は計算。
// どちらの画面も App が持つ同じ engine・master を使う。
describe("P4-4 計算・逆算の切り替え", () => {
  test("既定は「計算」タブで、計算画面を出す", async () => {
    render(<App engine={createFakeEngine()} />);
    const calcTab = await screen.findByRole("tab", { name: "計算" });
    expect(calcTab).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tab", { name: "逆算" })).toHaveAttribute("aria-selected", "false");
    expect(screen.getByRole("tablist")).toBeInTheDocument();
    expect(await screen.findByRole("combobox", { name: "攻撃側のポケモン" })).toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "自分のポケモン" })).toBeNull();
  });

  test("「逆算」タブで逆算画面に切り替わり、「計算」タブで戻る", async () => {
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("combobox", { name: "攻撃側のポケモン" });

    await user.click(screen.getByRole("tab", { name: "逆算" }));
    expect(screen.getByRole("tab", { name: "逆算" })).toHaveAttribute("aria-selected", "true");
    expect(await screen.findByRole("combobox", { name: "自分のポケモン" })).toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "攻撃側のポケモン" })).toBeNull();

    await user.click(screen.getByRole("tab", { name: "計算" }));
    expect(await screen.findByRole("combobox", { name: "攻撃側のポケモン" })).toBeInTheDocument();
  });

  test("逆算画面も App に渡した engine を使う(観測を入れると fake の calcReverse が呼ばれる)", async () => {
    const engine = createFakeEngine();
    const master = await exampleMasterSource.load();
    const [mine, theirs] = master.species;
    if (mine === undefined || theirs === undefined) {
      throw new Error("例データに種族が2つ以上要る");
    }
    const user = userEvent.setup();
    render(<App engine={engine} />);
    await user.click(await screen.findByRole("tab", { name: "逆算" }));
    await user.selectOptions(await screen.findByRole("combobox", { name: "自分のポケモン" }), mine.key);
    await user.selectOptions(screen.getByRole("combobox", { name: "相手のポケモン" }), theirs.key);
    await user.type(screen.getByRole("textbox", { name: "観測1" }), "45");

    await waitFor(() => {
      expect(engine.reverseRequests.length).toBeGreaterThan(0);
    });
    expect(engine.reverseRequests.at(-1)?.unknownSpecies.key).toBe(theirs.key);
  });
});

// P4-4 critic 指摘: タブは WAI-ARIA Authoring Practices の Tabs パターン(automatic activation)に従う。
// aria-controls/aria-labelledby の配線、ロービング tabIndex、矢印キー・Home/End の操作を確かめる。
describe("P4-4 タブの ARIA 配線とキーボード操作", () => {
  test("tab の aria-controls が tabpanel の id と一致し、tabpanel の aria-labelledby が選択中の tab の id と一致する", async () => {
    render(<App engine={createFakeEngine()} />);
    const calcTab = await screen.findByRole("tab", { name: "計算" });
    const reverseTab = screen.getByRole("tab", { name: "逆算" });
    const panel = screen.getByRole("tabpanel");

    expect(calcTab).toHaveAttribute("aria-controls", panel.id);
    expect(reverseTab).toHaveAttribute("aria-controls", panel.id);
    expect(panel).toHaveAttribute("aria-labelledby", calcTab.id);

    await userEvent.setup().click(reverseTab);
    expect(screen.getByRole("tabpanel")).toHaveAttribute("aria-labelledby", reverseTab.id);
  });

  test("選択中のタブだけ tabindex 0、他は -1(ロービング tabIndex)", async () => {
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    const calcTab = await screen.findByRole("tab", { name: "計算" });
    const reverseTab = screen.getByRole("tab", { name: "逆算" });

    expect(calcTab).toHaveAttribute("tabindex", "0");
    expect(reverseTab).toHaveAttribute("tabindex", "-1");

    await user.click(reverseTab);
    expect(reverseTab).toHaveAttribute("tabindex", "0");
    expect(calcTab).toHaveAttribute("tabindex", "-1");
  });

  test("ArrowRight/ArrowLeft でタブの選択とフォーカスの両方が移動する", async () => {
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    const calcTab = await screen.findByRole("tab", { name: "計算" });
    const reverseTab = screen.getByRole("tab", { name: "逆算" });

    calcTab.focus();
    await user.keyboard("{ArrowRight}");
    expect(reverseTab).toHaveAttribute("aria-selected", "true");
    expect(reverseTab).toHaveFocus();
    expect(await screen.findByRole("combobox", { name: "自分のポケモン" })).toBeInTheDocument();

    await user.keyboard("{ArrowLeft}");
    expect(calcTab).toHaveAttribute("aria-selected", "true");
    expect(calcTab).toHaveFocus();
    expect(await screen.findByRole("combobox", { name: "攻撃側のポケモン" })).toBeInTheDocument();
  });

  test("Home/End で最初/最後のタブへ選択とフォーカスが移動する", async () => {
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    const calcTab = await screen.findByRole("tab", { name: "計算" });
    const reverseTab = screen.getByRole("tab", { name: "逆算" });

    // JD5: 最後のタブは判定(ADR-0705 §1)。素早さ・タイプバランスは End からそれぞれ1・2つ手前。
    const balanceTab = screen.getByRole("tab", { name: "タイプバランス" });
    const speedTab = screen.getByRole("tab", { name: "素早さ" });
    const judgeTab = screen.getByRole("tab", { name: "判定" });
    calcTab.focus();
    await user.keyboard("{End}");
    expect(judgeTab).toHaveAttribute("aria-selected", "true");
    expect(judgeTab).toHaveFocus();

    await user.keyboard("{ArrowLeft}");
    expect(speedTab).toHaveAttribute("aria-selected", "true");
    expect(speedTab).toHaveFocus();

    await user.keyboard("{ArrowLeft}");
    expect(balanceTab).toHaveAttribute("aria-selected", "true");
    expect(balanceTab).toHaveFocus();

    await user.keyboard("{ArrowLeft}");
    expect(reverseTab).toHaveAttribute("aria-selected", "true");
    expect(reverseTab).toHaveFocus();

    await user.keyboard("{Home}");
    expect(calcTab).toHaveAttribute("aria-selected", "true");
    expect(calcTab).toHaveFocus();
  });
});

// P4-5: 計算モードの切り替え(ADR-0301 §4)。ヘッダーに「オフライン(WASM)/ オンライン(API)」の
// radiogroup を置き、既定はオフライン。選択は localStorage に覚える。App は engines(offline・online)を
// 受け取れ(テストで差し替える)、選択中のモードの engine だけで計算する。自動のフォールバックはしない。
describe("P4-5 計算モード(オフライン / オンライン)の切り替え", () => {
  beforeEach(() => {
    localStorage.clear();
  });
  afterEach(() => {
    vi.restoreAllMocks();
    localStorage.clear();
  });

  async function exampleSpeciesPair() {
    const master = await exampleMasterSource.load();
    const [attacker, defender, other] = master.species;
    if (attacker === undefined || defender === undefined || other === undefined) {
      throw new Error("例データに種族が3つ以上要る");
    }
    return { attacker, defender, other };
  }

  function modeRadios() {
    const group = within(screen.getByRole("banner")).getByRole("radiogroup", { name: "計算モード" });
    return {
      group,
      offline: within(group).getByRole("radio", { name: "オフライン(WASM)" }),
      online: within(group).getByRole("radio", { name: "オンライン(API)" }),
    };
  }

  test("ヘッダーに「計算モード」の radiogroup があり、既定はオフライン(WASM)", () => {
    render(<App engines={{ offline: createFakeEngine(), online: createFakeEngine() }} />);
    const { offline, online } = modeRadios();
    expect(offline).toBeChecked();
    expect(online).not.toBeChecked();
  });

  test("engines を渡さず保存値がオンラインなら、既定の API 実装が /api/calc/bulk に POST する(WASM は使わない)", async () => {
    localStorage.setItem(CALC_MODE_STORAGE_KEY, "online");
    // 応答の中身はこのテストでは見ない(配線だけを見る)。エラーの封筒を返しておく。
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ code: "internal", message: "テスト" }), {
        status: 500,
        headers: { "Content-Type": "application/json" },
      }),
    );
    const { attacker, defender } = await exampleSpeciesPair();
    const user = userEvent.setup();
    render(<App />);

    await user.selectOptions(await screen.findByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
    await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), defender.key);

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalled();
    });
    const urlOf = (input: string | URL | Request): string =>
      input instanceof Request ? input.url : input instanceof URL ? input.href : input;
    const bulkCall = fetchMock.mock.calls.find(([input]) => urlOf(input).endsWith("api/calc/bulk"));
    if (bulkCall === undefined) {
      throw new Error("api/calc/bulk への POST が無い");
    }
    const init = bulkCall[1];
    expect(init?.method).toBe("POST");
    const bodyText = init?.body;
    if (typeof bodyText !== "string") {
      throw new Error("本文は JSON 文字列で送る");
    }
    expect(JSON.parse(bodyText)).toMatchObject({ defenderSpeciesKey: defender.key });
    // WASM(engine.wasm / wasm_exec.js)は読まない。
    expect(fetchMock.mock.calls.some(([input]) => urlOf(input).includes("engine.wasm"))).toBe(false);
  });

  test("既定(オフライン)では offline の engine で計算し、online の engine には触れない", async () => {
    const offlineEngine = createFakeEngine();
    const onlineEngine = createFakeEngine();
    const { attacker, defender } = await exampleSpeciesPair();
    const user = userEvent.setup();
    render(<App engines={{ offline: offlineEngine, online: onlineEngine }} />);

    await user.selectOptions(await screen.findByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
    await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), defender.key);

    await waitFor(() => {
      expect(offlineEngine.bulkRequests.length).toBeGreaterThan(0);
    });
    expect(onlineEngine.bulkRequests).toHaveLength(0);
    expect(onlineEngine.calcRequests).toHaveLength(0);
  });

  test("オンラインに切り替えると、以後の計算は online の engine に送り、offline の engine は呼ばない", async () => {
    const offlineEngine = createFakeEngine();
    const onlineEngine = createFakeEngine();
    const { attacker, defender, other } = await exampleSpeciesPair();
    const user = userEvent.setup();
    render(<App engines={{ offline: offlineEngine, online: onlineEngine }} />);

    await user.selectOptions(await screen.findByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
    await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), defender.key);
    await waitFor(() => {
      expect(offlineEngine.bulkRequests.length).toBeGreaterThan(0);
    });
    const offlineCountBeforeSwitch = offlineEngine.bulkRequests.length;

    await user.click(modeRadios().online);
    expect(modeRadios().online).toBeChecked();
    expect(modeRadios().offline).not.toBeChecked();

    await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), other.key);
    await waitFor(() => {
      expect(onlineEngine.bulkRequests.at(-1)?.defenderSpecies.key).toBe(other.key);
    });
    expect(offlineEngine.bulkRequests).toHaveLength(offlineCountBeforeSwitch);
  });

  test("オンラインからオフラインに戻すと、以後の計算は offline の engine に送る", async () => {
    localStorage.setItem(CALC_MODE_STORAGE_KEY, "online");
    const offlineEngine = createFakeEngine();
    const onlineEngine = createFakeEngine();
    const { attacker, defender, other } = await exampleSpeciesPair();
    const user = userEvent.setup();
    render(<App engines={{ offline: offlineEngine, online: onlineEngine }} />);

    await user.selectOptions(await screen.findByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
    await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), defender.key);
    await waitFor(() => {
      expect(onlineEngine.bulkRequests.length).toBeGreaterThan(0);
    });
    expect(offlineEngine.bulkRequests).toHaveLength(0);
    const onlineCountBeforeSwitch = onlineEngine.bulkRequests.length;

    await user.click(modeRadios().offline);
    await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), other.key);
    await waitFor(() => {
      expect(offlineEngine.bulkRequests.at(-1)?.defenderSpecies.key).toBe(other.key);
    });
    expect(onlineEngine.bulkRequests).toHaveLength(onlineCountBeforeSwitch);
  });

  test("逆算画面も選択中のモードの engine を使う", async () => {
    const offlineEngine = createFakeEngine();
    const onlineEngine = createFakeEngine();
    const { attacker, defender } = await exampleSpeciesPair();
    const user = userEvent.setup();
    render(<App engines={{ offline: offlineEngine, online: onlineEngine }} />);

    await user.click(modeRadios().online);
    await user.click(await screen.findByRole("tab", { name: "逆算" }));
    await user.selectOptions(await screen.findByRole("combobox", { name: "自分のポケモン" }), attacker.key);
    await user.selectOptions(screen.getByRole("combobox", { name: "相手のポケモン" }), defender.key);
    await user.type(screen.getByRole("textbox", { name: "観測1" }), "45");

    await waitFor(() => {
      expect(onlineEngine.reverseRequests.length).toBeGreaterThan(0);
    });
    expect(offlineEngine.reverseRequests).toHaveLength(0);
  });

  test("選んだモードは localStorage に保存され、アプリを開き直しても保たれる", async () => {
    const engines = { offline: createFakeEngine(), online: createFakeEngine() };
    const user = userEvent.setup();
    const { unmount } = render(<App engines={engines} />);

    await user.click(modeRadios().online);
    expect(localStorage.getItem(CALC_MODE_STORAGE_KEY)).toBe("online");
    unmount();

    render(<App engines={engines} />);
    expect(modeRadios().online).toBeChecked();

    await user.click(modeRadios().offline);
    expect(localStorage.getItem(CALC_MODE_STORAGE_KEY)).toBe("offline");
  });

  test("保存値が壊れていてもオフラインで開く", () => {
    localStorage.setItem(CALC_MODE_STORAGE_KEY, "broken");
    render(<App engines={{ offline: createFakeEngine(), online: createFakeEngine() }} />);
    expect(modeRadios().offline).toBeChecked();
  });

  test("オンラインで API に届かなくても、自動でオフラインに切り替えない(エラーを出す)", async () => {
    localStorage.setItem(CALC_MODE_STORAGE_KEY, "online");
    const offlineEngine = createFakeEngine();
    const onlineEngine = createFakeEngine(() => engineError("engine_unavailable", "API に届かない"));
    const { attacker, defender } = await exampleSpeciesPair();
    const user = userEvent.setup();
    render(<App engines={{ offline: offlineEngine, online: onlineEngine }} />);

    await user.selectOptions(await screen.findByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
    await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), defender.key);

    await waitFor(() => {
      expect(onlineEngine.bulkRequests.length).toBeGreaterThan(0);
    });
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(offlineEngine.bulkRequests).toHaveLength(0);
    expect(modeRadios().online).toBeChecked();
  });

  test("既定の engines でも、オンラインで描画〜マスタ読み込みの間は engine.wasm も API も読まない", async () => {
    localStorage.setItem(CALC_MODE_STORAGE_KEY, "online");
    const fetchSpy = vi.spyOn(globalThis, "fetch");
    const headAppendSpy = vi.spyOn(document.head, "append");
    const headAppendChildSpy = vi.spyOn(document.head, "appendChild");

    render(<App />);
    expect(await screen.findByRole("combobox", { name: "攻撃側のポケモン" })).toBeInTheDocument();

    expect(fetchSpy).not.toHaveBeenCalled();
    expect(headAppendSpy).not.toHaveBeenCalled();
    expect(headAppendChildSpy).not.toHaveBeenCalled();
  });
});

// issue #218(ADR-0308): タブを切り替えても入力が消えないこと。ここには最小の往復1件だけ置き
// (上の「別タブの入力欄は取れない」を守る describe と同じファイルで回帰を見るため)、
// 戻る/進む・遅延マウント・マスタ入れ替え・リロードの扱いは App.tabPersistence.test.tsx で確かめる。
describe("issue #218 タブの往復で入力が消えない", () => {
  test("計算タブで選んだ攻撃側・防御側は、逆算タブへ行って戻っても残る", async () => {
    const master = await exampleMasterSource.load();
    const [attacker, defender] = master.species;
    if (attacker === undefined || defender === undefined) {
      throw new Error("例データに種族が2つ以上要る");
    }
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);

    await user.selectOptions(await screen.findByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
    await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), defender.key);

    await user.click(screen.getByRole("tab", { name: "逆算" }));
    // 逆算タブを出している間、計算画面の入力欄はアクセシビリティツリーから取れない(既存の保証)。
    expect(screen.queryByRole("combobox", { name: "攻撃側のポケモン" })).toBeNull();
    await screen.findByRole("combobox", { name: "自分のポケモン" });

    await user.click(screen.getByRole("tab", { name: "計算" }));

    expect(await screen.findByRole("combobox", { name: "攻撃側のポケモン" })).toHaveValue(attacker.key);
    expect(screen.getByRole("combobox", { name: "防御側のポケモン" })).toHaveValue(defender.key);
  });
});
