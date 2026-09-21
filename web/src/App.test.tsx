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

// P4-2: App はヘッダーと計算画面を出す。マスタは MasterSource(既定は架空の例データ。ADR-0016 §3)から、
// 計算は CalcEngine(既定は browserWasmLoader の WASM 実装。ADR-0016 §2)から受け取り、テストでは差し替える。
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

  test("既定の engine(WASM)でも、描画〜マスタ読み込みの間は engine.wasm を読まない(計算するまで遅らせる。ADR-0016 §2)", async () => {
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
