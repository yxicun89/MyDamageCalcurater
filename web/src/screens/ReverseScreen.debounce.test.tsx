// P4-18(issue 113): 逆算で「入力の途中の値」で計算を始めない・先行の計算を取り消す(ADR-0300 §11)。
// 確かめること:
//   - 観測の数値を打っている間は計算を始めず、打ち終わって 200ms(OBSERVATION_INPUT_DEBOUNCE_MS)で1回だけ始める
//     (「45」と打つ途中の「4」では計算しない)
//   - 表示(計算中)と入力の検証は待たずに今までどおり
//   - 確定した操作(種族・持ち物の選択、単位の切り替え、行の追加・削除)は待たずに計算する
//   - 待っている途中の確定操作では、待っていた値も含めて1回だけ計算する(待機中のタイマーを残さない)
//   - 画面は最新の計算1つだけを持ち、新しい入力・画面の破棄で先行の計算を abort する
//   - abort した先行の計算の応答は、エラー表示にも最新の結果の上書きにもならない
//
// 時間はすべて fake timer で進める(壁時計に依存しない)。実時間では1ms も進めないので、待ちを含む口
// (waitFor / findBy* / userEvent の非同期ラッパー)は使わず、同期の fireEvent と act だけで操作し、
// タイマーとマイクロタスクはテストが明示的に進める。「素早く打つ」= 待ちを進めずに続けて入力する。

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeAll, describe, expect, test, vi } from "vitest";
import { OBSERVATION_INPUT_DEBOUNCE_MS } from "../domain/observations";
import { REQUEST_ABORTED_CODE, type Item, type ReverseResult } from "../engine/types";
import { calcScreenText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import {
  createDeferredReverseEngine,
  createFakeEngine,
  engineError,
  ok,
  reverseCandidate,
  type FakeEngine,
  type PendingReverse,
} from "../test/fakeEngine";
import { ReverseScreen } from "./ReverseScreen";

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

afterEach(() => {
  vi.useRealTimers();
});

function speciesAt(index: number): MasterSpecies {
  const species = master.species[index];
  if (species === undefined) {
    throw new Error(`例データに ${String(index)} 番目の種族が無い`);
  }
  return species;
}

function firstItem(): Item {
  const [item] = master.items;
  if (item === undefined) {
    throw new Error("例データに持ち物が無い");
  }
  return item;
}

const mySpeciesSelect = () => screen.getByRole("combobox", { name: "自分のポケモン" });
const theirSpeciesSelect = () => screen.getByRole("combobox", { name: "相手のポケモン" });
const myItemSelect = () => screen.getByRole("combobox", { name: "自分の持ち物" });
const observationInput = (n: number) => screen.getByRole("textbox", { name: `観測${String(n)}` });
const unitGroup = (n: number) => screen.getByRole("radiogroup", { name: `観測${String(n)}の単位` });
const addObservationButton = () => screen.getByRole("button", { name: "観測を追加" });

interface Rendered {
  readonly engine: FakeEngine;
  readonly unmount: () => void;
}

function renderScreen(engine: FakeEngine = createFakeEngine()): Rendered {
  vi.useFakeTimers();
  const { unmount } = render(<ReverseScreen engine={engine} master={master} />);
  return { engine, unmount };
}

/** セレクトで値を選ぶ(確定した操作)。 */
function select(element: HTMLElement, value: string): void {
  fireEvent.change(element, { target: { value } });
}

/**
 * 観測の欄に value を入れる(その時点の欄の中身そのもの)。
 * 「4」→「45」と続けて呼べば、利用者が1文字ずつ打ったのと同じ onChange の並びになる。
 */
function typeObservation(n: number, value: string): void {
  fireEvent.change(observationInput(n), { target: { value } });
}

/** タイマーを ms 進め、そのとき解決した Promise(応答の .then)も実行しきる。 */
async function advance(ms: number): Promise<void> {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}

/** タイマーは進めずに、解決済みの Promise(応答の .then)だけ実行しきる。 */
async function settle(): Promise<void> {
  await act(async () => {
    await Promise.resolve();
  });
}

/** 観測のデバウンスの待ちを終わらせる(trailing debounce の発火)。 */
function flushObservationDebounce(): Promise<void> {
  return advance(OBSERVATION_INPUT_DEBOUNCE_MS);
}

function choosePair(): void {
  select(mySpeciesSelect(), speciesAt(0).key);
  select(theirSpeciesSelect(), speciesAt(1).key);
}

const singleCandidateResult: ReverseResult = {
  side: "defender",
  stat: "def",
  assumedHpSp: 32,
  exactCount: 1,
  candidates: [reverseCandidate({ ranges: [{ min: 20, max: 23 }] })],
};

/** 取り消されていない(= 画面がまだ待っている)呼び出しだけを取り出す。 */
function liveCalls(pending: readonly PendingReverse[]): PendingReverse[] {
  return pending.filter((entry) => entry.signal?.aborted !== true);
}

describe("観測の数値を打っている間は計算を始めない(200ms の trailing debounce)", () => {
  test("素早く「4」→「45」と打つと、確定値の 45 だけで1回計算する", async () => {
    const { engine } = renderScreen();
    choosePair();

    typeObservation(1, "4");
    expect(engine.reverseRequests).toHaveLength(0);
    typeObservation(1, "45");
    expect(engine.reverseRequests).toHaveLength(0);

    await flushObservationDebounce();
    expect(engine.reverseRequests).toHaveLength(1);
    expect(engine.reverseRequests[0]?.observations).toEqual([{ percent: 45 }]);
  });

  test(`打ち終わって ${String(OBSERVATION_INPUT_DEBOUNCE_MS)}ms たつまでは計算を始めない`, async () => {
    const { engine } = renderScreen();
    choosePair();

    typeObservation(1, "45");
    await advance(OBSERVATION_INPUT_DEBOUNCE_MS - 1);
    expect(engine.reverseRequests).toHaveLength(0);

    await advance(1);
    expect(engine.reverseRequests).toHaveLength(1);
  });

  test("待っている途中でもう1文字打ったら、待ち直して1回だけ計算する", async () => {
    const { engine } = renderScreen();
    choosePair();

    typeObservation(1, "4");
    await advance(OBSERVATION_INPUT_DEBOUNCE_MS - 1);
    typeObservation(1, "45");
    await advance(OBSERVATION_INPUT_DEBOUNCE_MS - 1);
    expect(engine.reverseRequests).toHaveLength(0);

    await advance(1);
    expect(engine.reverseRequests).toHaveLength(1);
    expect(engine.reverseRequests[0]?.observations).toEqual([{ percent: 45 }]);
  });

  test("表示と入力の検証は待たない(打った直後に「計算中」、不正はすぐ知らせる)", async () => {
    const { engine } = renderScreen();
    choosePair();

    typeObservation(1, "45");
    // 計算はまだ始めていないが、利用者には「計算中」をすぐ見せる(入力してから 200ms 何も起きない画面にしない)。
    expect(screen.getByText("計算中")).toBeInTheDocument();
    expect(engine.reverseRequests).toHaveLength(0);

    // 「456」は 1〜100 の外なので不正。待たずに(その場で)知らせる。
    typeObservation(1, "456");
    expect(observationInput(1)).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByText("1〜100 の整数で入力してください")).toBeInTheDocument();
    await flushObservationDebounce();
    expect(engine.reverseRequests).toHaveLength(0);
  });

  test("画面が消えるときは待機中のタイマーを片付け、計算を始めない", async () => {
    const { engine, unmount } = renderScreen();
    choosePair();
    typeObservation(1, "45");

    unmount();
    expect(vi.getTimerCount()).toBe(0);
    await flushObservationDebounce();
    expect(engine.reverseRequests).toHaveLength(0);
  });

  test("前の結果が出ている状態で新しい値を打つと、デバウンスの間は前の結果を出さず「計算中」にする", async () => {
    renderScreen();
    choosePair();
    typeObservation(1, "45");
    await flushObservationDebounce();
    await settle();
    // 前の入力(45)の結果が出ている(「計算中」ではない)ことを確認してから、新しい値を打つ。
    expect(screen.queryByText(calcScreenText.loadingNotice)).not.toBeInTheDocument();

    typeObservation(1, "50");
    // requestRows にはまだ「45」時点の値のままなので、observations との不一致(debouncePending)で
    // 「計算中」に戻る(前の結果〈45 の候補〉をそのまま出し続けない)。
    expect(screen.getByText(calcScreenText.loadingNotice)).toBeInTheDocument();
  });
});

describe("確定した操作は待たずに計算する", () => {
  test("持ち物を選んだら、待たずに計算し直す", async () => {
    const { engine } = renderScreen();
    choosePair();
    typeObservation(1, "45");
    await flushObservationDebounce();
    expect(engine.reverseRequests).toHaveLength(1);

    select(myItemSelect(), firstItem().id);
    await settle();
    expect(engine.reverseRequests).toHaveLength(2);
    expect(engine.reverseRequests[1]?.known.item?.id).toBe(firstItem().id);
  });

  test("観測の単位(%/HP)を切り替えたら、待たずに計算し直す", async () => {
    const { engine } = renderScreen();
    choosePair();
    typeObservation(1, "45");
    await flushObservationDebounce();

    fireEvent.click(within(unitGroup(1)).getByRole("radio", { name: "HP" }));
    await settle();
    expect(engine.reverseRequests).toHaveLength(2);
    expect(engine.reverseRequests[1]?.observations).toEqual([{ damage: 45 }]);
  });

  test("観測の行を削除したら、待たずに残りの観測で計算し直す", async () => {
    const { engine } = renderScreen();
    choosePair();
    typeObservation(1, "45");
    await flushObservationDebounce();
    fireEvent.click(addObservationButton());
    typeObservation(2, "50");
    await flushObservationDebounce();
    const before = engine.reverseRequests.length;

    fireEvent.click(screen.getByRole("button", { name: "観測2を削除" }));
    await settle();
    expect(engine.reverseRequests).toHaveLength(before + 1);
    expect(engine.reverseRequests.at(-1)?.observations).toEqual([{ percent: 45 }]);
  });

  test("待機中に確定した操作をしたら、待っていた値も含めて1回だけ計算する", async () => {
    const { engine } = renderScreen();
    choosePair();
    typeObservation(1, "45");
    await flushObservationDebounce();
    expect(engine.reverseRequests).toHaveLength(1);

    // 「50」を打った直後(まだ待機中)に持ち物を選ぶ。
    typeObservation(1, "");
    typeObservation(1, "50");
    select(myItemSelect(), firstItem().id);
    await settle();
    expect(engine.reverseRequests).toHaveLength(2);
    expect(engine.reverseRequests[1]?.observations).toEqual([{ percent: 50 }]);
    expect(engine.reverseRequests[1]?.known.item?.id).toBe(firstItem().id);

    // 待機中だったタイマーは残っていない(同じ入力でもう1回計算しない)。
    await flushObservationDebounce();
    expect(engine.reverseRequests).toHaveLength(2);
  });
});

describe("先行の計算を取り消す(AbortSignal)", () => {
  test("calcReverse には取り消しの signal を渡す(まだ取り消されていない)", async () => {
    const { engine } = renderScreen();
    choosePair();
    typeObservation(1, "45");
    await flushObservationDebounce();

    const signal = engine.reverseSignals.at(-1);
    expect(signal).toBeInstanceOf(AbortSignal);
    expect(signal?.aborted).toBe(false);
  });

  test("入力が変わったら、送信済みの先行の計算を abort して最新の1つだけを持つ", async () => {
    const { engine, pending } = createDeferredReverseEngine();
    renderScreen(engine);
    choosePair();
    typeObservation(1, "45");
    await flushObservationDebounce();
    expect(pending).toHaveLength(1);

    select(myItemSelect(), firstItem().id);
    await settle();
    expect(pending).toHaveLength(2);
    expect(pending[0]?.signal?.aborted).toBe(true);
    expect(pending[1]?.signal?.aborted).toBe(false);
    expect(liveCalls(pending)).toHaveLength(1);
  });

  test("画面が消えたら、進行中の計算を abort する", async () => {
    const { engine, pending } = createDeferredReverseEngine();
    const { unmount } = renderScreen(engine);
    choosePair();
    typeObservation(1, "45");
    await flushObservationDebounce();
    expect(pending).toHaveLength(1);

    unmount();
    expect(pending[0]?.signal?.aborted).toBe(true);
  });

  test.each([
    ["取り消しの封筒", REQUEST_ABORTED_CODE, "新しい入力で計算を取り消しました"],
    ["engine_unavailable の封筒", "engine_unavailable", "API に接続できません"],
  ] as const)(
    "abort した先行の計算が %s で返っても、エラーを出さず最新の計算の状態を変えない",
    async (_label, code, message) => {
      const { engine, pending } = createDeferredReverseEngine();
      renderScreen(engine);
      choosePair();
      typeObservation(1, "45");
      await flushObservationDebounce();
      select(myItemSelect(), firstItem().id);
      await settle();
      const [stale, fresh] = pending;
      if (stale === undefined || fresh === undefined) {
        throw new Error("先行と最新の2回の呼び出しが要る");
      }

      await act(async () => {
        stale.resolve(engineError(code, message));
        await Promise.resolve();
      });
      expect(screen.queryByRole("alert")).toBeNull();
      expect(screen.getByText("計算中")).toBeInTheDocument();

      await act(async () => {
        fresh.resolve(ok(singleCandidateResult));
        await Promise.resolve();
      });
      expect(screen.queryByRole("alert")).toBeNull();
      expect(screen.getByRole("list", { name: "推定結果" })).toBeInTheDocument();
      expect(screen.queryByText("計算中")).toBeNull();
    },
  );

  test("最新の計算が失敗したときは、今までどおりエラーを出す(取り消しと取り違えない)", async () => {
    const { engine, pending } = createDeferredReverseEngine();
    renderScreen(engine);
    choosePair();
    typeObservation(1, "45");
    await flushObservationDebounce();
    const latest = pending.at(-1);
    if (latest === undefined) {
      throw new Error("calcReverse が呼ばれていない");
    }

    await act(async () => {
      latest.resolve(engineError("engine_unavailable", "API に接続できません"));
      await Promise.resolve();
    });
    expect(screen.getByRole("alert")).toHaveTextContent("API に接続できません");
  });
});
