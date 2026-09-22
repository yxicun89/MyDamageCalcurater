// P4-8: 計算画面の演出(docs/design.md「動き」、CLAUDE.md「常時動くアニメーションを入れない。演出は操作時のみ」)。
// jsdom は実際のアニメーションを動かさないので、演出の「きっかけ」と「終わり」(状態クラス・CSS 変数)を確かめる。
// 見た目(keyframes・秒数・イージング)は CSS 側で持ち、web/src/styles/motion.test.ts で静的に確かめる。
// 確かめること:
//   - 確定数が変わった瞬間: その行の確定数バッジに is-pulsing(最初の結果・変わらない再計算では付けない)。animationend か
//     (animationend が来ないときの)最大待ちのタイマーで外す(P4-9)
//   - 攻守入れ替え: 左右のカードに is-swapping。animationend か(animationend が来ないときの)タイマーで外す
//   - ホロ: ポインタが乗っているカード1枚だけに is-holo と --holo-x / --holo-y(カード内の位置、0〜100%)。
//     P4-9: 反映は requestAnimationFrame で1フレーム1回に間引く(離れた・画面が消えたら予約を取り消す)。
//     pointerType が mouse / pen のときだけ(touch では出さない)
//   - OS の「視差効果を減らす」では上のどれも付けない(演出を始めない)

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { afterEach, beforeAll, beforeEach, describe, expect, test, vi } from "vitest";
import type { BulkRow, CalcResult } from "../engine/types";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { installFakeAnimationFrame, type FakeAnimationFrame } from "../test/animationFrame";
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
  // P4-9: 位置の反映は requestAnimationFrame で1フレームに1回へ間引く。フレームは fake で進める。
  let raf: FakeAnimationFrame;

  beforeEach(() => {
    raf = installFakeAnimationFrame();
  });

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

  /** マウスのポインタ移動(P4-9: ホロは pointerType が mouse / pen のときだけ)。 */
  function mouseMove(target: HTMLElement, clientX: number, clientY: number): void {
    fireEvent.pointerMove(target, { pointerType: "mouse", clientX, clientY });
  }

  /** 1フレーム進める(予約された位置の反映を実行する)。 */
  function nextFrame(): void {
    act(() => {
      raf.flush();
    });
  }

  test("操作する前はどちらのカードにもホロが無い(常時動かさない)", () => {
    renderScreen();
    nextFrame();
    for (const card of [attackerCard(), defenderCard()]) {
      expect(card).not.toHaveClass("is-holo");
      expect(holo(card)).toEqual({ x: "", y: "" });
    }
    expect(raf.request).not.toHaveBeenCalled();
  });

  test("ポインタを動かしたカードだけに is-holo と、カード内の位置(%)の --holo-x / --holo-y を付ける", () => {
    renderScreen();
    const card = attackerCard();
    placeCard(card);
    mouseMove(card, 150, 275);
    nextFrame();

    expect(card).toHaveClass("is-holo");
    expect(holo(card)).toEqual({ x: "25%", y: "75%" });
    expect(defenderCard()).not.toHaveClass("is-holo");
    expect(holo(defenderCard())).toEqual({ x: "", y: "" });
  });

  test("ペン(pointerType pen)でもホロを付ける", () => {
    renderScreen();
    const card = attackerCard();
    placeCard(card);
    fireEvent.pointerMove(card, { pointerType: "pen", clientX: 150, clientY: 275 });
    nextFrame();

    expect(card).toHaveClass("is-holo");
    expect(holo(card)).toEqual({ x: "25%", y: "75%" });
  });

  test("タッチ(pointerType touch)ではホロを付けない(フレームも予約しない)", () => {
    renderScreen();
    const card = attackerCard();
    placeCard(card);
    fireEvent.pointerMove(card, { pointerType: "touch", clientX: 150, clientY: 275 });
    nextFrame();

    expect(card).not.toHaveClass("is-holo");
    expect(holo(card)).toEqual({ x: "", y: "" });
    expect(raf.request).not.toHaveBeenCalled();
  });

  test("マウスで光っているカードでも、タッチの移動では位置を動かさない", () => {
    renderScreen();
    const card = attackerCard();
    placeCard(card);
    mouseMove(card, 150, 275);
    nextFrame();
    fireEvent.pointerMove(card, { pointerType: "touch", clientX: 250, clientY: 225 });
    nextFrame();

    expect(holo(card)).toEqual({ x: "25%", y: "75%" });
  });

  test("カードの中の子要素(セレクトなど)の上で動かしても、カード基準の位置になる", () => {
    renderScreen();
    const card = attackerCard();
    placeCard(card);
    mouseMove(attackerSpeciesSelect(), 200, 250);
    nextFrame();
    expect(holo(card)).toEqual({ x: "50%", y: "50%" });
  });

  test("カードの外にはみ出した位置は 0%〜100% に収める", () => {
    renderScreen();
    const card = attackerCard();
    placeCard(card);
    mouseMove(card, 50, 400);
    nextFrame();
    expect(holo(card)).toEqual({ x: "0%", y: "100%" });
  });

  test("1フレームの間の移動はまとめて1回だけ反映し、位置は最後の移動のもの", () => {
    renderScreen();
    const card = attackerCard();
    placeCard(card);
    mouseMove(card, 150, 275);
    mouseMove(card, 200, 250);
    mouseMove(card, 250, 225);

    // フレームが来るまでは位置を書かない(pointermove ごとに書かない)
    expect(holo(card)).toEqual({ x: "", y: "" });
    // 予約するフレームは1つだけ
    expect(raf.request).toHaveBeenCalledTimes(1);

    nextFrame();
    expect(card).toHaveClass("is-holo");
    expect(holo(card)).toEqual({ x: "75%", y: "25%" });
  });

  test("フレームごとに位置を更新する(次のフレームでは次の移動の位置)", () => {
    renderScreen();
    const card = attackerCard();
    placeCard(card);
    mouseMove(card, 150, 275);
    nextFrame();
    mouseMove(card, 250, 225);
    nextFrame();

    expect(holo(card)).toEqual({ x: "75%", y: "25%" });
    expect(raf.request).toHaveBeenCalledTimes(2);
  });

  test("ポインタが離れたらホロを外す", () => {
    renderScreen();
    const card = attackerCard();
    placeCard(card);
    mouseMove(card, 150, 275);
    nextFrame();
    fireEvent.pointerLeave(card, { pointerType: "mouse" });
    nextFrame();
    expect(card).not.toHaveClass("is-holo");
    expect(holo(card)).toEqual({ x: "", y: "" });
  });

  test("フレームを待っている間に離れたら、その予約を取り消し、ホロは出さない", () => {
    renderScreen();
    const card = attackerCard();
    placeCard(card);
    mouseMove(card, 150, 275);
    const [pendingId] = raf.pendingIds();
    expect(pendingId).toBeDefined();

    fireEvent.pointerLeave(card, { pointerType: "mouse" });
    expect(raf.cancel).toHaveBeenCalledWith(pendingId);
    expect(raf.pendingIds()).toEqual([]);

    nextFrame();
    expect(card).not.toHaveClass("is-holo");
    expect(holo(card)).toEqual({ x: "", y: "" });
  });

  test("フレームを待っている間に画面が消えたら、その予約を取り消す", () => {
    const { unmount } = render(<CalcScreen engine={createFakeEngine()} master={master} />);
    const card = attackerCard();
    placeCard(card);
    mouseMove(card, 150, 275);
    const [pendingId] = raf.pendingIds();
    expect(pendingId).toBeDefined();

    unmount();
    expect(raf.cancel).toHaveBeenCalledWith(pendingId);
    expect(raf.pendingIds()).toEqual([]);
  });

  test("もう一方のカードに移ったら、前のカードのホロは外す(同時に光るのは1枚だけ)", () => {
    renderScreen();
    const attacker = attackerCard();
    const defender = defenderCard();
    placeCard(attacker);
    placeCard(defender);
    mouseMove(attacker, 150, 275);
    nextFrame();
    // pointerleave を取りこぼしても1枚だけになること(leave は送らない)
    mouseMove(defender, 250, 225);
    nextFrame();

    expect(defender).toHaveClass("is-holo");
    expect(holo(defender)).toEqual({ x: "75%", y: "25%" });
    expect(attacker).not.toHaveClass("is-holo");
    expect(holo(attacker)).toEqual({ x: "", y: "" });
  });

  test("同じフレームの間に2枚のカードを動いても、光るのは最後に動いたカード1枚だけ", () => {
    renderScreen();
    const attacker = attackerCard();
    const defender = defenderCard();
    placeCard(attacker);
    placeCard(defender);
    mouseMove(attacker, 150, 275);
    mouseMove(defender, 250, 225);
    nextFrame();

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
    mouseMove(card, 150, 275);
    nextFrame();
    expect(card).not.toHaveClass("is-holo");
    expect(holo(card)).toEqual({ x: "", y: "" });
  });
});

describe("確定数バッジの弾みは animationend が来なくても外す(P4-9)", () => {
  // animationend が来ない(タブが裏にある等)ときの最大待ち。CSS の --duration-pulse(0.4秒)を上回り、1秒以内。
  const PULSE_CSS_DURATION_MS = 400;
  const MAX_WAIT_UPPER_BOUND_MS = 1000;

  /** 本物の時間を進めつつ(応答の Promise・findBy が進むように)、タイマーはテストからも進められる fake にする。 */
  function renderWithFakeTimers(engine: FakeEngine): { user: UserEvent; unmount: () => void } {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
    const { unmount } = render(<CalcScreen engine={engine} master={master} />);
    return { user, unmount };
  }

  function advance(ms: number): void {
    act(() => {
      vi.advanceTimersByTime(ms);
    });
  }

  test("弾み始めてから1秒たっても animationend が来なければ is-pulsing を外す", async () => {
    let ko = guaranteedKO(2);
    const { user } = renderWithFakeTimers(engineWithRows(() => [bulkRow({ presetLabel: "行", ko })]));
    await choosePair(user);
    await koBadge("行1", "確定2発");

    ko = guaranteedKO(1);
    await user.selectOptions(defenderItemSelect(), firstItemId());
    const badge = await koBadge("行2", "確定1発");
    expect(badge).toHaveClass("is-pulsing");

    advance(MAX_WAIT_UPPER_BOUND_MS);
    expect(badge).not.toHaveClass("is-pulsing");
  });

  test("CSS の弾み(0.4秒)が終わる前には外さない", async () => {
    let ko = guaranteedKO(2);
    const { user } = renderWithFakeTimers(engineWithRows(() => [bulkRow({ presetLabel: "行", ko })]));
    await choosePair(user);
    await koBadge("行1", "確定2発");

    ko = guaranteedKO(1);
    await user.selectOptions(defenderItemSelect(), firstItemId());
    const badge = await koBadge("行2", "確定1発");
    advance(PULSE_CSS_DURATION_MS - 1);
    expect(badge).toHaveClass("is-pulsing");
  });

  // 前の弾みのタイマーが、次の弾みを途中で打ち切らないこと(弾み直すたびにタイマーを掛け直す)。
  // 最大待ち(0.4秒超〜1秒)のどの値でも、前のタイマーが次の弾みの 0.4秒の間に発火する間隔を2つ選ぶ。
  test.each([300, 650])(
    "%i ms 後に確定数がまた変わったら、前のタイマーでは外さない(次の弾みの 0.4秒の間は付けたまま)",
    async (retriggerAfterMs) => {
      let ko = guaranteedKO(2);
      const { user } = renderWithFakeTimers(engineWithRows(() => [bulkRow({ presetLabel: "行", ko })]));
      await choosePair(user);
      await koBadge("行1", "確定2発");

      ko = guaranteedKO(1);
      await user.selectOptions(defenderItemSelect(), firstItemId());
      expect(await koBadge("行2", "確定1発")).toHaveClass("is-pulsing");

      advance(retriggerAfterMs);
      ko = guaranteedKO(2);
      await user.selectOptions(defenderItemSelect(), "");
      const badge = await koBadge("行3", "確定2発");
      expect(badge).toHaveClass("is-pulsing");

      advance(PULSE_CSS_DURATION_MS - 1);
      expect(badge).toHaveClass("is-pulsing");
      advance(MAX_WAIT_UPPER_BOUND_MS);
      expect(badge).not.toHaveClass("is-pulsing");
    },
  );

  test("弾んでいる間に画面が消えたら、最大待ちのタイマーも片付ける(回しっぱなしにしない)", async () => {
    let ko = guaranteedKO(2);
    const { user, unmount } = renderWithFakeTimers(
      engineWithRows(() => [bulkRow({ presetLabel: "行", ko })]),
    );
    await choosePair(user);
    await koBadge("行1", "確定2発");
    const timersBeforePulse = vi.getTimerCount();

    ko = guaranteedKO(1);
    await user.selectOptions(defenderItemSelect(), firstItemId());
    expect(await koBadge("行2", "確定1発")).toHaveClass("is-pulsing");
    expect(vi.getTimerCount()).toBeGreaterThan(timersBeforePulse);

    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });

  test("OS の「視差効果を減らす」では弾まないので、タイマーも掛けない", async () => {
    setReducedMotion(true);
    let ko = guaranteedKO(2);
    const { user } = renderWithFakeTimers(engineWithRows(() => [bulkRow({ presetLabel: "行", ko })]));
    await choosePair(user);
    await koBadge("行1", "確定2発");
    const timersBefore = vi.getTimerCount();

    ko = guaranteedKO(1);
    await user.selectOptions(defenderItemSelect(), firstItemId());
    expect(await koBadge("行2", "確定1発")).not.toHaveClass("is-pulsing");
    expect(vi.getTimerCount()).toBe(timersBefore);
  });
});
