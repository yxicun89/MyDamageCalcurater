import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test, vi } from "vitest";
import { App } from "./App";
import { exampleMasterSource } from "./master/exampleSource";
import type { MasterData, MasterSource } from "./master/types";
import { createFakeEngine } from "./test/fakeEngine";

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

    calcTab.focus();
    await user.keyboard("{End}");
    expect(reverseTab).toHaveAttribute("aria-selected", "true");
    expect(reverseTab).toHaveFocus();

    await user.keyboard("{Home}");
    expect(calcTab).toHaveAttribute("aria-selected", "true");
    expect(calcTab).toHaveFocus();
  });
});
