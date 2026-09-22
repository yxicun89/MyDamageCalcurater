// P4-8: 計算画面の演出(docs/design.md「動き」、CLAUDE.md「常時動くアニメーションを入れない。演出は操作時のみ」)。
// jsdom は実際のアニメーションを動かさないので、演出の「きっかけ」と「終わり」(状態クラス・CSS 変数)を確かめる。
// 見た目(keyframes・秒数・イージング)は CSS 側で持ち、web/src/styles/motion.test.ts で静的に確かめる。
// 確かめること:
//   - 確定数が変わった瞬間: その行の確定数バッジに is-pulsing(最初の結果・変わらない再計算では付けない)。animationend で外す
//   - 攻守入れ替え: 左右のカードに is-swapping。animationend か(animationend が来ないときの)タイマーで外す
//   - ホロ: ポインタが乗っているカード1枚だけに is-holo と --holo-x / --holo-y(カード内の位置、0〜100%)
//   - OS の「視差効果を減らす」では上のどれも付けない(演出を始めない)

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { afterEach, beforeAll, describe, expect, test, vi } from "vitest";
import type { BulkRow, CalcResult } from "../engine/types";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { fakeMatchMedia } from "../test/matchMedia";
import { bulkRow, createFakeEngine, ok, type FakeEngine } from "../test/fakeEngine";
import { CalcScreen } from "./CalcScreen";

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

function speciesAt(index: number): MasterSpecies {
  const species = master.species[index];
  if (species === undefined) {
    throw new Error(`例データに ${String(index)} 番目の種族が無い`);
  }
  return species;
}

function firstItemId(): string {
  const item = master.items[0];
  if (item === undefined) {
    throw new Error("例データに持ち物が無い");
  }
  return item.id;
}

function guaranteedKO(hits: number): CalcResult["ko"] {
  return { hits, guaranteed: true, chancePercent: 0, displayChancePercent: 100 };
}

const attackerSpeciesSelect = () => screen.getByRole("combobox", { name: "攻撃側のポケモン" });
const defenderSpeciesSelect = () => screen.getByRole("combobox", { name: "防御側のポケモン" });
const defenderItemSelect = () => screen.getByRole("combobox", { name: "防御側の持ち物" });
const attackerCard = () => screen.getByRole("region", { name: "攻撃側" });
const defenderCard = () => screen.getByRole("region", { name: "防御側" });
const swapButton = () => screen.getByRole("button", { name: "攻守入れ替え" });

function setReducedMotion(reduce: boolean): void {
  vi.stubGlobal("matchMedia", fakeMatchMedia(reduce));
}

/**
 * 行を差し替えられる fake。rows() は calcBulk が呼ばれるたびに読むので、テストの途中で次の応答を変えられる。
 * presetLabel に呼び出し回数を入れ、新しい応答が表示されたことを待てるようにする。
 */
function engineWithRows(rows: () => readonly BulkRow[]): FakeEngine {
  let calls = 0;
  return createFakeEngine((request) => {
    calls += 1;
    return ok({
      defenderSpeciesKey: request.defenderSpecies.key,
      rows: rows().map((row) => ({ ...row, presetLabel: `${row.presetLabel}${String(calls)}` })),
    });
  });
}

function renderScreen(engine: FakeEngine = createFakeEngine()): UserEvent {
  const user = userEvent.setup();
  render(<CalcScreen engine={engine} master={master} />);
  return user;
}

async function choosePair(user: UserEvent): Promise<void> {
  await user.selectOptions(attackerSpeciesSelect(), speciesAt(0).key);
  await user.selectOptions(defenderSpeciesSelect(), speciesAt(1).key);
}

/** presetLabel(呼び出し回数つき)で行を見つけ、その行の確定数バッジ(確定数の文字を持つ要素)を返す。 */
async function koBadge(rowLabel: string, koText: string): Promise<HTMLElement> {
  const label = await screen.findByText(rowLabel);
  const row = label.closest("li");
  if (row === null) {
    throw new Error(`${rowLabel} の行が無い`);
  }
  return within(row).getByText(koText);
}

describe("確定数が変わった瞬間にバッジが弾む", () => {
  test("最初の結果では弾まない(表示されただけでは動かさない)", async () => {
    const user = renderScreen(engineWithRows(() => [bulkRow({ presetLabel: "行", ko: guaranteedKO(2) })]));
    await choosePair(user);
    expect(await koBadge("行1", "確定2発")).not.toHaveClass("is-pulsing");
  });

  test("入力を変えて同じ行の確定数が変わったら、その行のバッジに is-pulsing を付け、animationend で外す", async () => {
    let ko = guaranteedKO(2);
    const user = renderScreen(engineWithRows(() => [bulkRow({ presetLabel: "行", ko })]));
    await choosePair(user);
    await koBadge("行1", "確定2発");

    ko = guaranteedKO(1);
    await user.selectOptions(defenderItemSelect(), firstItemId());
    const badge = await koBadge("行2", "確定1発");
    expect(badge).toHaveClass("is-pulsing");

    fireEvent.animationEnd(badge);
    expect(badge).not.toHaveClass("is-pulsing");
  });

  test("計算し直しても確定数が変わらなければ弾まない", async () => {
    const user = renderScreen(engineWithRows(() => [bulkRow({ presetLabel: "行", ko: guaranteedKO(2) })]));
    await choosePair(user);
    await koBadge("行1", "確定2発");

    await user.selectOptions(defenderItemSelect(), firstItemId());
    expect(await koBadge("行2", "確定2発")).not.toHaveClass("is-pulsing");
  });

  test("弾むのは確定数が変わった行だけ(同じ preset・持ち物の行どうしで比べる)", async () => {
    let hpRowKO = guaranteedKO(3);
    const user = renderScreen(
      engineWithRows(() => [
        bulkRow({ preset: "none", presetLabel: "無振り", ko: guaranteedKO(2) }),
        bulkRow({ preset: "hp", presetLabel: "H振り", ko: hpRowKO }),
      ]),
    );
    await choosePair(user);
    await koBadge("H振り1", "確定3発");

    hpRowKO = guaranteedKO(2);
    await user.selectOptions(defenderItemSelect(), firstItemId());
    expect(await koBadge("H振り2", "確定2発")).toHaveClass("is-pulsing");
    expect(await koBadge("無振り2", "確定2発")).not.toHaveClass("is-pulsing");
  });

  test("OS の「視差効果を減らす」では、確定数が変わっても弾まない", async () => {
    setReducedMotion(true);
    let ko = guaranteedKO(2);
    const user = renderScreen(engineWithRows(() => [bulkRow({ presetLabel: "行", ko })]));
    await choosePair(user);
    await koBadge("行1", "確定2発");

    ko = guaranteedKO(1);
    await user.selectOptions(defenderItemSelect(), firstItemId());
    expect(await koBadge("行2", "確定1発")).not.toHaveClass("is-pulsing");
  });
});

describe("攻守入れ替えでカードが入れ替わる", () => {
  test("操作する前はどちらのカードにも is-swapping が無い", () => {
    renderScreen();
    expect(attackerCard()).not.toHaveClass("is-swapping");
    expect(defenderCard()).not.toHaveClass("is-swapping");
  });

  test("攻守入れ替えを押すと左右のカードに is-swapping を付け、それぞれの animationend で外す", async () => {
    const user = renderScreen();
    await user.click(swapButton());
    expect(attackerCard()).toHaveClass("is-swapping");
    expect(defenderCard()).toHaveClass("is-swapping");

    fireEvent.animationEnd(attackerCard());
    fireEvent.animationEnd(defenderCard());
    expect(attackerCard()).not.toHaveClass("is-swapping");
    expect(defenderCard()).not.toHaveClass("is-swapping");
  });

  test("animationend が来なくても(タブが裏にある等)、1秒以内に is-swapping を外す", () => {
    vi.useFakeTimers();
    renderScreen();
    fireEvent.click(swapButton());
    expect(attackerCard()).toHaveClass("is-swapping");

    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(attackerCard()).not.toHaveClass("is-swapping");
    expect(defenderCard()).not.toHaveClass("is-swapping");
  });

  test("OS の「視差効果を減らす」では is-swapping を付けない(入れ替え自体はする)", async () => {
    setReducedMotion(true);
    const user = renderScreen();
    await choosePair(user);
    await user.click(swapButton());
    expect(attackerSpeciesSelect()).toHaveValue(speciesAt(1).key);
    expect(attackerCard()).not.toHaveClass("is-swapping");
    expect(defenderCard()).not.toHaveClass("is-swapping");
  });
});

describe("ホロ効果は選択中のカード1枚だけ、ポインタ位置に連動", () => {
  /** カードの位置を jsdom に教える(jsdom はレイアウトしないので getBoundingClientRect は 0 を返す)。 */
  function placeCard(card: HTMLElement): void {
    const rect = { x: 100, y: 200, left: 100, top: 200, width: 200, height: 100, right: 300, bottom: 300 };
    vi.spyOn(card, "getBoundingClientRect").mockReturnValue({ ...rect, toJSON: () => rect });
  }

  function holo(card: HTMLElement): { x: string; y: string } {
    return {
      x: card.style.getPropertyValue("--holo-x").trim(),
      y: card.style.getPropertyValue("--holo-y").trim(),
    };
  }

  test("操作する前はどちらのカードにもホロが無い(常時動かさない)", () => {
    renderScreen();
    for (const card of [attackerCard(), defenderCard()]) {
      expect(card).not.toHaveClass("is-holo");
      expect(holo(card)).toEqual({ x: "", y: "" });
    }
  });

  test("ポインタを動かしたカードだけに is-holo と、カード内の位置(%)の --holo-x / --holo-y を付ける", () => {
    renderScreen();
    const card = attackerCard();
    placeCard(card);
    fireEvent.pointerMove(card, { clientX: 150, clientY: 275 });

    expect(card).toHaveClass("is-holo");
    expect(holo(card)).toEqual({ x: "25%", y: "75%" });
    expect(defenderCard()).not.toHaveClass("is-holo");
    expect(holo(defenderCard())).toEqual({ x: "", y: "" });
  });

  test("カードの中の子要素(セレクトなど)の上で動かしても、カード基準の位置になる", () => {
    renderScreen();
    const card = attackerCard();
    placeCard(card);
    fireEvent.pointerMove(attackerSpeciesSelect(), { clientX: 200, clientY: 250 });
    expect(holo(card)).toEqual({ x: "50%", y: "50%" });
  });

  test("カードの外にはみ出した位置は 0%〜100% に収める", () => {
    renderScreen();
    const card = attackerCard();
    placeCard(card);
    fireEvent.pointerMove(card, { clientX: 50, clientY: 400 });
    expect(holo(card)).toEqual({ x: "0%", y: "100%" });
  });

  test("ポインタが離れたらホロを外す", () => {
    renderScreen();
    const card = attackerCard();
    placeCard(card);
    fireEvent.pointerMove(card, { clientX: 150, clientY: 275 });
    fireEvent.pointerLeave(card);
    expect(card).not.toHaveClass("is-holo");
    expect(holo(card)).toEqual({ x: "", y: "" });
  });

  test("もう一方のカードに移ったら、前のカードのホロは外す(同時に光るのは1枚だけ)", () => {
    renderScreen();
    const attacker = attackerCard();
    const defender = defenderCard();
    placeCard(attacker);
    placeCard(defender);
    fireEvent.pointerMove(attacker, { clientX: 150, clientY: 275 });
    // pointerleave を取りこぼしても1枚だけになること(leave は送らない)
    fireEvent.pointerMove(defender, { clientX: 250, clientY: 225 });

    expect(defender).toHaveClass("is-holo");
    expect(holo(defender)).toEqual({ x: "75%", y: "25%" });
    expect(attacker).not.toHaveClass("is-holo");
    expect(holo(attacker)).toEqual({ x: "", y: "" });
  });

  test("OS の「視差効果を減らす」ではホロを付けない", () => {
    setReducedMotion(true);
    renderScreen();
    const card = attackerCard();
    placeCard(card);
    fireEvent.pointerMove(card, { clientX: 150, clientY: 275 });
    expect(card).not.toHaveClass("is-holo");
    expect(holo(card)).toEqual({ x: "", y: "" });
  });
});
