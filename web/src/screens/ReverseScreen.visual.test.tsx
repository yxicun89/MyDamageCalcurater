// F-12 PR-2(I-web-12、ADR-0336 §2): 逆算画面への共通部品の適用。
// 自分/相手のカードは ui-card ui-card--typed(--card-type は選んだ種族の最初のタイプ)、
// 観測側・自分の調整の選択は ui-chip(選択中だけ ui-chip--selected)、ボタンは ui-button、
// 案内・エラーは ui-notice、推定結果の一覧は ui-rows。
// 既存のクラス(reverse-*)・アクセシブルな名前・role・テキストは変えない(クラスの追加と包みだけ)。

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { afterEach, beforeAll, describe, expect, test, vi } from "vitest";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { createFakeEngine, engineError, ok, reverseResultFor } from "../test/fakeEngine";
import { ReverseScreen } from "./ReverseScreen";

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

afterEach(() => {
  vi.useRealTimers();
});

const myCard = () => screen.getByRole("region", { name: "自分のポケモン" });
const theirCard = () => screen.getByRole("region", { name: "相手のポケモン" });

function renderScreen(engine = createFakeEngine()): UserEvent {
  const user = userEvent.setup();
  render(<ReverseScreen engine={engine} master={master} />);
  return user;
}

function pairWithDifferentPrimaryTypes(): [MasterSpecies, MasterSpecies] {
  for (const a of master.species) {
    const b = master.species.find((candidate) => candidate.types[0] !== a.types[0]);
    if (b !== undefined) {
      return [a, b];
    }
  }
  throw new Error("例データに最初のタイプが違う2体が無い");
}

async function choosePair(user: UserEvent, mine: MasterSpecies, theirs: MasterSpecies): Promise<void> {
  await user.selectOptions(screen.getByRole("combobox", { name: "自分のポケモン" }), mine.key);
  await user.selectOptions(screen.getByRole("combobox", { name: "相手のポケモン" }), theirs.key);
}

const typeVar = (typeId: string | undefined) => `var(--type-${typeId ?? ""}, var(--brand-primary))`;

describe("カード", () => {
  test("自分・相手のカードは ui-card ui-card--typed(既存の reverse-card も残す)", () => {
    renderScreen();
    for (const card of [myCard(), theirCard()]) {
      expect(card).toHaveClass("ui-card", "ui-card--typed", "reverse-card");
    }
  });

  test("種族が未選択の間は --card-type を置かない", () => {
    renderScreen();
    for (const card of [myCard(), theirCard()]) {
      expect(card.style.getPropertyValue("--card-type")).toBe("");
    }
  });

  test("種族を選ぶと、そのカードの --card-type が最初のタイプの色になる", async () => {
    const user = renderScreen();
    const [mine, theirs] = pairWithDifferentPrimaryTypes();
    await choosePair(user, mine, theirs);
    expect(myCard().style.getPropertyValue("--card-type")).toBe(typeVar(mine.types[0]));
    expect(theirCard().style.getPropertyValue("--card-type")).toBe(typeVar(theirs.types[0]));
  });
});

describe("選択肢はチップ", () => {
  test("観測したダメージの側(ラジオ)は ui-chip。選択中だけ ui-chip--selected、切り替えに追従する", async () => {
    const user = renderScreen();
    const group = screen.getByRole("radiogroup", { name: "観測したダメージ" });
    for (const radio of within(group).getAllByRole("radio")) {
      const option = radio.closest("label");
      expect(option).toHaveClass("ui-chip", "reverse-side__option");
      expect(option?.classList.contains("ui-chip--selected")).toBe((radio as HTMLInputElement).checked);
    }
    await user.click(within(group).getByRole("radio", { name: "受けたダメージ" }));
    expect(within(group).getByRole("radio", { name: "受けたダメージ" }).closest("label")).toHaveClass(
      "ui-chip--selected",
    );
    expect(within(group).getByRole("radio", { name: "与えたダメージ" }).closest("label")).not.toHaveClass(
      "ui-chip--selected",
    );
  });

  test("自分の調整(ラジオ)も ui-chip(既存の reverse-preset__option も残す)", async () => {
    const user = renderScreen();
    const [mine, theirs] = pairWithDifferentPrimaryTypes();
    await choosePair(user, mine, theirs);
    const group = screen.getByRole("radiogroup", { name: "自分の調整" });
    for (const radio of within(group).getAllByRole("radio")) {
      const option = radio.closest("label");
      expect(option).toHaveClass("ui-chip", "reverse-preset__option");
      expect(option?.classList.contains("ui-chip--selected")).toBe((radio as HTMLInputElement).checked);
    }
  });

  test("観測の単位(ラジオ)は ui-chip。選択中だけ ui-chip--selected", () => {
    renderScreen();
    const group = screen.getByRole("radiogroup", { name: "観測1の単位" });
    for (const radio of within(group).getAllByRole("radio")) {
      const option = radio.closest("label");
      expect(option).toHaveClass("ui-chip");
      expect(option?.classList.contains("ui-chip--selected")).toBe((radio as HTMLInputElement).checked);
    }
  });
});

describe("ボタン", () => {
  test("「観測を追加」は ui-button ui-button--secondary、追加した観測の削除も ui-button--secondary", async () => {
    const user = renderScreen();
    const add = screen.getByRole("button", { name: "観測を追加" });
    expect(add).toHaveClass("ui-button", "ui-button--secondary", "reverse-observations__add");
    await user.click(add);
    expect(screen.getByRole("button", { name: "観測2を削除" })).toHaveClass(
      "ui-button",
      "ui-button--secondary",
    );
  });
});

describe("案内・結果", () => {
  test("エラーは role=alert のまま ui-notice ui-notice--error", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
    render(
      <ReverseScreen
        engine={createFakeEngine(undefined, () => engineError("internal", "テストの失敗"))}
        master={master}
      />,
    );
    const [mine, theirs] = pairWithDifferentPrimaryTypes();
    await choosePair(user, mine, theirs);
    await user.type(screen.getByRole("textbox", { name: "観測1" }), "45");
    vi.advanceTimersByTime(1000);
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveClass("ui-notice", "ui-notice--error");
  });

  test("推定結果の一覧(list「推定結果」)は ui-rows(既存の reverse-results__list も残す)", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
    render(
      <ReverseScreen
        engine={createFakeEngine(undefined, (request) => ok(reverseResultFor(request)))}
        master={master}
      />,
    );
    const [mine, theirs] = pairWithDifferentPrimaryTypes();
    await choosePair(user, mine, theirs);
    await user.type(screen.getByRole("textbox", { name: "観測1" }), "45");
    vi.advanceTimersByTime(1000);
    const list = await screen.findByRole("list", { name: "推定結果" });
    await waitFor(() => {
      expect(list).toHaveClass("ui-rows", "reverse-results__list");
    });
  });
});
