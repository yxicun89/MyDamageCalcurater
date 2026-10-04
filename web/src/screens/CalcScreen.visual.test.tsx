// F-12(I-web-6、ADR-0331 §3): 計算画面への共通部品の適用。
// カードは選択中の種族の最初のタイプで色が変わる(--card-type。.ui-card--typed がふち・上端の帯に使う)。
// 既存のクラス(calc-card など)・アクセシブルな名前・DOM の役割は変えない。ホロ(--holo-x/y)のインライン style とも共存する。

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { afterEach, beforeAll, describe, expect, test, vi } from "vitest";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { createFakeEngine } from "../test/fakeEngine";
import { installFakeAnimationFrame } from "../test/animationFrame";
import { fakeMatchMedia } from "../test/matchMedia";
import { CalcScreen } from "./CalcScreen";

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

afterEach(() => {
  vi.restoreAllMocks();
});

const attackerCard = () => screen.getByRole("region", { name: "攻撃側" });
const defenderCard = () => screen.getByRole("region", { name: "防御側" });

function renderScreen(): UserEvent {
  const user = userEvent.setup();
  render(<CalcScreen engine={createFakeEngine()} master={master} />);
  return user;
}

async function choosePair(user: UserEvent, attacker: MasterSpecies, defender: MasterSpecies): Promise<void> {
  await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
  await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), defender.key);
}

/** 2体のうち、最初のタイプが違う組(色が変わったことを区別できる組)。 */
function pairWithDifferentPrimaryTypes(): [MasterSpecies, MasterSpecies] {
  for (const a of master.species) {
    const b = master.species.find((candidate) => candidate.types[0] !== a.types[0]);
    if (b !== undefined) {
      return [a, b];
    }
  }
  throw new Error("例データに最初のタイプが違う2体が無い");
}

describe("カード", () => {
  test("攻撃側・防御側のカードは ui-card(既存の calc-card も残す)", () => {
    renderScreen();
    for (const card of [attackerCard(), defenderCard()]) {
      expect(card).toHaveClass("ui-card", "calc-card");
    }
  });

  test("種族が未選択の間は --card-type を置かない(ブランド色の既定のまま)", () => {
    renderScreen();
    for (const card of [attackerCard(), defenderCard()]) {
      expect(card).toHaveClass("ui-card--typed");
      expect(card.style.getPropertyValue("--card-type")).toBe("");
    }
  });

  test("種族を選ぶと、そのカードの --card-type が最初のタイプの色になる", async () => {
    const user = renderScreen();
    const [attacker, defender] = pairWithDifferentPrimaryTypes();
    await choosePair(user, attacker, defender);
    expect(attackerCard().style.getPropertyValue("--card-type")).toBe(
      `var(--type-${attacker.types[0] ?? ""}, var(--brand-primary))`,
    );
    expect(defenderCard().style.getPropertyValue("--card-type")).toBe(
      `var(--type-${defender.types[0] ?? ""}, var(--brand-primary))`,
    );
  });

  test("攻守入れ替えのあとは、入れ替わった種族のタイプ色になる", async () => {
    const user = renderScreen();
    const [attacker, defender] = pairWithDifferentPrimaryTypes();
    await choosePair(user, attacker, defender);
    await user.click(screen.getByRole("button", { name: "攻守入れ替え" }));
    expect(attackerCard().style.getPropertyValue("--card-type")).toBe(
      `var(--type-${defender.types[0] ?? ""}, var(--brand-primary))`,
    );
  });

  test("ホロ(--holo-x/y)のインライン style とタイプ色の変数が共存する", async () => {
    vi.stubGlobal("matchMedia", fakeMatchMedia(false));
    const raf = installFakeAnimationFrame();
    try {
      const user = renderScreen();
      const [attacker, defender] = pairWithDifferentPrimaryTypes();
      await choosePair(user, attacker, defender);
      const card = attackerCard();
      const rect = { x: 100, y: 200, left: 100, top: 200, width: 200, height: 100, right: 300, bottom: 300 };
      vi.spyOn(card, "getBoundingClientRect").mockReturnValue({ ...rect, toJSON: () => rect });
      fireEvent.pointerMove(card, { pointerType: "mouse", clientX: 150, clientY: 275 });
      act(() => {
        raf.flush();
      });
      expect(card).toHaveClass("is-holo");
      expect(card.style.getPropertyValue("--holo-x").trim()).toBe("25%");
      expect(card.style.getPropertyValue("--card-type")).toBe(
        `var(--type-${attacker.types[0] ?? ""}, var(--brand-primary))`,
      );
    } finally {
      vi.unstubAllGlobals();
    }
  });

  test("タイプのバッジは ui-badge(既存の calc-card__type とタイプ色の塗りは残す)", async () => {
    const user = renderScreen();
    const [attacker, defender] = pairWithDifferentPrimaryTypes();
    await choosePair(user, attacker, defender);
    const badges = within(attackerCard()).getAllByRole("listitem");
    expect(badges.length).toBeGreaterThan(0);
    for (const badge of badges) {
      expect(badge).toHaveClass("ui-badge", "calc-card__type");
    }
  });

  test("画像が無ければタイプ色エンブレムのまま(画像は必須にしない。P8-1c)", async () => {
    const user = renderScreen();
    const [attacker, defender] = pairWithDifferentPrimaryTypes();
    await choosePair(user, attacker, defender);
    expect(within(attackerCard()).getByTestId("type-emblem")).toBeInTheDocument();
  });
});

describe("ボタン・結果", () => {
  test("攻守入れ替えは ui-button ui-button--secondary。名前は変えず、アイコンは装飾", () => {
    renderScreen();
    const swap = screen.getByRole("button", { name: "攻守入れ替え" });
    expect(swap).toHaveClass("ui-button", "ui-button--secondary", "calc-screen__swap");
    const icon = swap.querySelector("svg");
    expect(icon).toHaveClass("ui-icon");
    expect(icon).toHaveAttribute("aria-hidden", "true");
  });

  test("「詳細」は ui-button ui-button--secondary", () => {
    renderScreen();
    expect(screen.getByRole("button", { name: "詳細" })).toHaveClass("ui-button", "ui-button--secondary");
  });

  test("計算結果の一覧は ui-rows(ゼブラ・行ホバー)。確定数は ui-badge", async () => {
    const user = renderScreen();
    const [attacker, defender] = pairWithDifferentPrimaryTypes();
    await choosePair(user, attacker, defender);
    const list = await screen.findByRole("list", { name: "計算結果" });
    expect(list).toHaveClass("ui-rows", "calc-results__list");
    for (const row of within(list).getAllByRole("listitem")) {
      expect(row.querySelector(".calc-results__ko")).toHaveClass("ui-badge");
    }
  });
});
