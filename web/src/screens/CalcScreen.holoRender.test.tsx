// P4-9: ホロの pointermove で計算画面全体を再レンダーしない(docs/plan.md P4-9、design.md「パフォーマンス予算」
// 計算結果の表示更新 100ms)。ホロの状態はカードの中に閉じ、結果一覧や技の選択は描き直さない。
// 描き直したかどうかは、結果一覧(ResultsList)と技の選択(MoveSelect)がレンダーのたびに呼ぶ表示用の関数
// (domain/format の formatPercentRange・formatMoveCategory)の呼び出し回数で確かめる(本体に計測用の口を足さない)。
// 中身は本物のまま、呼び出しを数えるだけの spy に差し替える。

import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeAll, beforeEach, describe, expect, test, vi } from "vitest";
import * as format from "../domain/format";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { installFakeAnimationFrame, type FakeAnimationFrame } from "../test/animationFrame";
import { createFakeEngine } from "../test/fakeEngine";
import { CalcScreen } from "./CalcScreen";

vi.mock("../domain/format", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../domain/format")>();
  return {
    ...actual,
    formatPercentRange: vi.fn(actual.formatPercentRange),
    formatMoveCategory: vi.fn(actual.formatMoveCategory),
  };
});

let master: MasterData;
let raf: FakeAnimationFrame;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

beforeEach(() => {
  raf = installFakeAnimationFrame();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function speciesAt(index: number): MasterSpecies {
  const species = master.species[index];
  if (species === undefined) {
    throw new Error(`例データに ${String(index)} 番目の種族が無い`);
  }
  return species;
}

const attackerCard = () => screen.getByRole("region", { name: "攻撃側" });
const defenderCard = () => screen.getByRole("region", { name: "防御側" });

function placeCard(card: HTMLElement): void {
  const rect = { x: 100, y: 200, left: 100, top: 200, width: 200, height: 100, right: 300, bottom: 300 };
  vi.spyOn(card, "getBoundingClientRect").mockReturnValue({ ...rect, toJSON: () => rect });
}

function mouseMove(target: HTMLElement, clientX: number, clientY: number): void {
  fireEvent.pointerMove(target, { pointerType: "mouse", clientX, clientY });
}

function nextFrame(): void {
  act(() => {
    raf.flush();
  });
}

/** 結果一覧と技の選択が、これまでに描かれた回数(表示用の関数の呼び出し回数)。 */
function renderCounts(): { results: number; moveSelect: number } {
  return {
    results: vi.mocked(format.formatPercentRange).mock.calls.length,
    moveSelect: vi.mocked(format.formatMoveCategory).mock.calls.length,
  };
}

/** 攻撃側・防御側を選んで結果一覧が出るまで進め、カードの位置を jsdom に教える。 */
async function renderWithResults(): Promise<void> {
  const user = userEvent.setup();
  render(<CalcScreen engine={createFakeEngine()} master={master} />);
  await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), speciesAt(0).key);
  await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), speciesAt(1).key);
  await screen.findByRole("list", { name: "計算結果" });
  placeCard(attackerCard());
  placeCard(defenderCard());
}

describe("ホロの pointermove で画面全体を描き直さない", () => {
  test("前提: 結果一覧と技の選択は、計算画面が描き直されるたびに表示用の関数を呼ぶ", async () => {
    await renderWithResults();
    const counts = renderCounts();
    expect(counts.results).toBeGreaterThan(0);
    expect(counts.moveSelect).toBeGreaterThan(0);
  });

  test("カードの上でポインタを動かしてホロを出しても、結果一覧と技の選択は描き直さない", async () => {
    await renderWithResults();
    const before = renderCounts();

    for (const [x, y] of [
      [150, 275],
      [200, 250],
      [250, 225],
    ] as const) {
      mouseMove(attackerCard(), x, y);
      nextFrame();
    }

    expect(attackerCard()).toHaveClass("is-holo");
    expect(renderCounts()).toEqual(before);
  });

  test("もう一方のカードへ移る・離れるときも、結果一覧と技の選択は描き直さない", async () => {
    await renderWithResults();
    const before = renderCounts();

    mouseMove(attackerCard(), 150, 275);
    nextFrame();
    mouseMove(defenderCard(), 250, 225);
    nextFrame();
    expect(defenderCard()).toHaveClass("is-holo");
    expect(attackerCard()).not.toHaveClass("is-holo");

    fireEvent.pointerLeave(defenderCard(), { pointerType: "mouse" });
    nextFrame();
    expect(defenderCard()).not.toHaveClass("is-holo");

    expect(renderCounts()).toEqual(before);
  });
});
