// I-speed-4b(ADR-0610 §4・Web 画面の具体): 素早さ画面の新しい構成。SpeedClient は fake(props で注入)。
// engine(WASM)・pokedex のマスタは使わない(ADR-0604 §5)。素早さの値・区間はすべて speed-svc が決め、Web は計算し直さない。
// 確かめること(受け入れ条件 W1〜W12。docs/adr/0610 の「Web 画面の具体」と対応):
//   W1  表・周辺パネルが画面に無く、table()・position() を呼ばない
//   W2  マウント時に pokemon() を 1 回呼ぶ。自分と相手を選ぶまで matchup() は呼ばない
//   W3  自分と相手を選ぶと matchup() を呼ぶ。本文は入力どおり(既定の想定は 4 つを表の順で明示・省略値は送らない)
//   W4  想定のチップ(既定 4 つ on)。+1・+2 を足せる。最後の 1 つは外せない
//   W5  想定ごとのカード: 見出し(相手の素早さ)・一言見出し・性格ごとの最低ライン・同速の別表示
//   W6  どう振っても上を取れないときの表示(同速だけがある場合は同速を併記)
//   W7  トリックルーム: matchup に trickRoom を送る。「下を取る」の読み替えと上限 SP、下降補正の行はこのときだけ出す
//   W8  「今の振り」: 入れると self.current を送り、行動順のバッジ(文字つき)と reach() が出る。入れないと reach は呼ばない
//   W9  「これ以上」: 6 プリセットごとの族の区切り・該当数・境界の例。トリックルーム中は読み替え(reach は呼び直さない)
//   W10 読み込み中・エラー(code から日本語。matchup と reach と pokemon は互いに独立)・古い応答の無視
//   W11 連続技へのリンクは onOpenCombo があるときだけ。無ければ出さない(無効表示もしない)
//   W12 a11y・見た目: 見出しの階層、ランクのメーター(slider)、ポケモンカード、色だけに頼らない、常時アニメーションなし

import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { readFileSync } from "node:fs";
import { describe, expect, test, vi } from "vitest";
import { speedPresetText, speedScreenText } from "../i18n/ja";
import { uiText } from "../i18n/ui";
import {
  BIRD,
  FISH,
  createFakeSpeedClient,
  lastOf,
  matchupResponse,
  pokemonListResponse,
  reachResponse,
  trickRoomMatchupResponse,
  unreachableMatchupResponse,
  type FakeSpeedClient,
} from "../test/fakeSpeedClient";
import { localPath } from "../test/localPath";
import { SpeedScreen, type SpeedComboRequest } from "./SpeedScreen";
import type { components } from "./speed.gen";

type Schemas = components["schemas"];

// ---- 描画と、よく使う操作 ----

interface Rendered {
  readonly user: UserEvent;
  readonly client: FakeSpeedClient;
}

/** 描画し、pokemon() の応答まで返す(一覧がそろった状態から始める)。 */
async function renderLoaded(onOpenCombo?: (request: SpeedComboRequest) => void): Promise<Rendered> {
  const user = userEvent.setup();
  const client = createFakeSpeedClient();
  render(<SpeedScreen speedClient={client} {...(onOpenCombo === undefined ? {} : { onOpenCombo })} />);
  await act(async () => {
    lastOf(client.pokemonCalls, "pokemon").resolve({ ok: true, value: pokemonListResponse });
    await Promise.resolve();
  });
  return { user, client };
}

function selfRegion(): HTMLElement {
  return screen.getByRole("region", { name: speedScreenText.selfRegionLabel });
}
function opponentRegion(): HTMLElement {
  return screen.getByRole("region", { name: speedScreenText.opponentRegionLabel });
}
/** 出力 1 の領域。名前は見出しと同じで、トリックルーム中は読み替えた見出しになる。 */
function matchupRegion(): HTMLElement {
  return (
    screen.queryByRole("region", { name: speedScreenText.matchupHeading }) ??
    screen.getByRole("region", { name: speedScreenText.matchupHeadingTrickRoom })
  );
}
function reachRegion(): HTMLElement {
  return screen.getByRole("region", { name: speedScreenText.reachHeading });
}

async function chooseSelf({ user }: Rendered, pokemon: Schemas["SpeedPokemon"] = BIRD): Promise<void> {
  await user.selectOptions(
    screen.getByRole("combobox", { name: speedScreenText.selfPokemonLabel }),
    pokemon.pokemonId,
  );
}
async function chooseOpponent({ user }: Rendered, pokemon: Schemas["SpeedPokemon"] = FISH): Promise<void> {
  await user.selectOptions(
    screen.getByRole("combobox", { name: speedScreenText.opponentPokemonLabel }),
    pokemon.pokemonId,
  );
}
async function chooseBoth(rendered: Rendered): Promise<void> {
  await chooseSelf(rendered);
  await chooseOpponent(rendered);
}

/** 「今の振りも入れる」をオンにして SP・性格を入れる。 */
async function enterCurrent({ user }: Rendered, sp: number, natureLabel: string): Promise<void> {
  await user.click(within(selfRegion()).getByRole("checkbox", { name: speedScreenText.currentToggleLabel }));
  fireEvent.change(within(selfRegion()).getByRole("spinbutton", { name: speedScreenText.spLabel }), {
    target: { value: String(sp) },
  });
  await user.click(within(selfRegion()).getByRole("radio", { name: natureLabel }));
}

async function resolveMatchup(
  client: FakeSpeedClient,
  value: Schemas["MatchupResponse"],
  index?: number,
): Promise<void> {
  const call = index === undefined ? lastOf(client.matchupCalls, "matchup") : client.matchupCalls[index];
  await act(async () => {
    call?.resolve({ ok: true, value });
    await Promise.resolve();
  });
}
async function resolveReach(client: FakeSpeedClient, value: Schemas["ReachResponse"]): Promise<void> {
  await act(async () => {
    lastOf(client.reachCalls, "reach").resolve({ ok: true, value });
    await Promise.resolve();
  });
}

function cards(): HTMLElement[] {
  return within(matchupRegion()).getAllByTestId("speed-matchup-card");
}
function cardFor(preset: Schemas["PresetId"]): HTMLElement {
  const found = cards().find((card) => card.getAttribute("data-preset") === preset);
  if (found === undefined) {
    throw new Error(`${preset} のカードが無い`);
  }
  return found;
}
function natureRow(card: HTMLElement, nature: Schemas["NatureId"]): HTMLElement {
  const found = within(card)
    .queryAllByTestId("speed-nature-line")
    .find((row) => row.getAttribute("data-nature") === nature);
  if (found === undefined) {
    throw new Error(`${nature} の行が無い`);
  }
  return found;
}
function queryNatureRow(card: HTMLElement, nature: Schemas["NatureId"]): HTMLElement | undefined {
  return within(card)
    .queryAllByTestId("speed-nature-line")
    .find((row) => row.getAttribute("data-nature") === nature);
}
function reachRow(preset: Schemas["PresetId"]): HTMLElement {
  const found = within(reachRegion())
    .getAllByTestId("speed-reach-line")
    .find((row) => row.getAttribute("data-preset") === preset);
  if (found === undefined) {
    throw new Error(`${preset} の「これ以上」の行が無い`);
  }
  return found;
}

const DEFAULT_ASSUMPTIONS = [
  { preset: "uninvested" },
  { preset: "neutral-max" },
  { preset: "max" },
  { preset: "max-scarf" },
] as const;

// ---- W1・W2: 構成と最初の呼び出し ----

describe("W1 表と周辺パネルは画面に無い(ADR-0610 §4)", () => {
  test("表の領域・仮想スクロール・段・自分の周りが出ず、table()・position() を呼ばない", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    await enterCurrent(rendered, 20, speedScreenText.natureLabel.plus);
    await resolveMatchup(rendered.client, matchupResponse);
    await resolveReach(rendered.client, reachResponse);

    expect(screen.queryByRole("region", { name: "素早さの表" })).not.toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "自分の周り" })).not.toBeInTheDocument();
    expect(screen.queryByTestId("speed-viewport")).not.toBeInTheDocument();
    expect(screen.queryByTestId("speed-tier")).not.toBeInTheDocument();
    expect(screen.queryByTestId("speed-marker")).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "自分の周り" })).not.toBeInTheDocument();
    expect(rendered.client.tableCalls).toHaveLength(0);
    expect(rendered.client.positionCalls).toHaveLength(0);
  });
});

describe("W2 最初の呼び出し", () => {
  test("マウント時に pokemon() を 1 回だけ呼び、読み込み中を出す。matchup・reach はまだ呼ばない", () => {
    const client = createFakeSpeedClient();
    render(<SpeedScreen speedClient={client} />);

    expect(client.pokemonCalls).toHaveLength(1);
    expect(client.matchupCalls).toHaveLength(0);
    expect(client.reachCalls).toHaveLength(0);
    expect(screen.getByText(speedScreenText.pokemonListLoadingNotice)).toBeInTheDocument();
  });

  test("一覧がそろうと、自分・相手の選択肢に名前が並び、使い方の一言を出す(カードはまだ無い)", async () => {
    await renderLoaded();

    const self = within(selfRegion()).getByRole("combobox", { name: speedScreenText.selfPokemonLabel });
    expect(within(self).getByRole("option", { name: speedScreenText.unselectedOption })).toBeInTheDocument();
    expect(within(self).getByRole("option", { name: BIRD.nameJa })).toBeInTheDocument();
    expect(
      within(opponentRegion()).getByRole("combobox", { name: speedScreenText.opponentPokemonLabel }),
    ).toBeInTheDocument();
    expect(screen.getByText(speedScreenText.introNotice)).toBeInTheDocument();
    expect(screen.queryByTestId("speed-matchup-card")).not.toBeInTheDocument();
  });

  test("自分だけ・相手だけでは matchup() を呼ばない", async () => {
    const rendered = await renderLoaded();
    await chooseSelf(rendered);
    expect(rendered.client.matchupCalls).toHaveLength(0);
    await rendered.user.selectOptions(
      screen.getByRole("combobox", { name: speedScreenText.selfPokemonLabel }),
      speedScreenText.unselectedOption,
    );
    await chooseOpponent(rendered);
    expect(rendered.client.matchupCalls).toHaveLength(0);
  });
});

// ---- W3・W4: matchup() の本文と想定のチップ ----

describe("W3 matchup() の本文", () => {
  test("自分と相手を選ぶと 1 回呼ぶ。既定は想定 4 つを表の順で明示し、false・0 の欄は送らない", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);

    expect(rendered.client.matchupCalls).toHaveLength(1);
    expect(lastOf(rendered.client.matchupCalls, "matchup").args).toEqual({
      self: { pokemonId: BIRD.pokemonId },
      opponent: { pokemonId: FISH.pokemonId, assumptions: DEFAULT_ASSUMPTIONS },
    });
  });

  test("ランク・スカーフ・追い風・まひ(自分と相手)・トリックルームを入れると、本文にその欄だけを足す", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    const self = within(selfRegion());
    const opponent = within(opponentRegion());

    await rendered.user.click(
      self.getByRole("button", { name: `${speedScreenText.rankLabel}${uiText.rankIncreaseSuffix}` }),
    );
    await rendered.user.click(
      self.getByRole("button", { name: `${speedScreenText.rankLabel}${uiText.rankIncreaseSuffix}` }),
    );
    await rendered.user.click(self.getByRole("checkbox", { name: speedScreenText.scarfLabel }));
    await rendered.user.click(self.getByRole("checkbox", { name: speedScreenText.selfTailwindLabel }));
    await rendered.user.click(self.getByRole("checkbox", { name: speedScreenText.selfParalysisLabel }));
    await rendered.user.click(
      opponent.getByRole("checkbox", { name: speedScreenText.opponentTailwindLabel }),
    );
    await rendered.user.click(
      opponent.getByRole("checkbox", { name: speedScreenText.opponentParalysisLabel }),
    );
    await rendered.user.click(screen.getByRole("checkbox", { name: speedScreenText.trickRoomLabel }));

    expect(lastOf(rendered.client.matchupCalls, "matchup").args).toEqual({
      self: { pokemonId: BIRD.pokemonId, rank: 2, scarf: true, tailwind: true, paralysis: true },
      opponent: {
        pokemonId: FISH.pokemonId,
        assumptions: DEFAULT_ASSUMPTIONS,
        tailwind: true,
        paralysis: true,
      },
      trickRoom: true,
    });
  });

  test("入力を変えるたびに呼び直す(同じ入力では呼び直さない)", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    expect(rendered.client.matchupCalls).toHaveLength(1);
    await rendered.user.click(
      within(selfRegion()).getByRole("checkbox", { name: speedScreenText.scarfLabel }),
    );
    expect(rendered.client.matchupCalls).toHaveLength(2);
    await chooseOpponent(rendered, FISH);
    expect(rendered.client.matchupCalls).toHaveLength(2);
  });
});

describe("W4 相手の想定のチップ", () => {
  test("既定は 無振り・準速・最速・最速スカーフ の 4 つが on、+1・+2 は off", async () => {
    await renderLoaded();
    const group = within(opponentRegion()).getByRole("group", { name: speedScreenText.assumptionGroupLabel });
    for (const preset of ["uninvested", "neutral-max", "max", "max-scarf"] as const) {
      expect(within(group).getByRole("checkbox", { name: speedPresetText[preset] })).toBeChecked();
    }
    for (const preset of ["max-plus1", "max-plus2"] as const) {
      expect(within(group).getByRole("checkbox", { name: speedPresetText[preset] })).not.toBeChecked();
    }
  });

  test("+1 を足すと表の順に並べて送り、無振りを外すと送らない", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    const group = within(opponentRegion()).getByRole("group", { name: speedScreenText.assumptionGroupLabel });

    await rendered.user.click(within(group).getByRole("checkbox", { name: speedPresetText["max-plus1"] }));
    await rendered.user.click(within(group).getByRole("checkbox", { name: speedPresetText.uninvested }));

    expect(lastOf(rendered.client.matchupCalls, "matchup").args.opponent.assumptions).toEqual([
      { preset: "neutral-max" },
      { preset: "max" },
      { preset: "max-scarf" },
      { preset: "max-plus1" },
    ]);
  });

  test("最後の 1 つは外せない(案内を出し、assumptions は空にならない)", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    const group = within(opponentRegion()).getByRole("group", { name: speedScreenText.assumptionGroupLabel });
    for (const preset of ["uninvested", "neutral-max", "max"] as const) {
      await rendered.user.click(within(group).getByRole("checkbox", { name: speedPresetText[preset] }));
    }
    const last = within(group).getByRole("checkbox", { name: speedPresetText["max-scarf"] });
    await rendered.user.click(last);

    expect(last).toBeChecked();
    expect(screen.getByText(speedScreenText.assumptionMinimumNotice)).toBeInTheDocument();
    expect(lastOf(rendered.client.matchupCalls, "matchup").args.opponent.assumptions).toEqual([
      { preset: "max-scarf" },
    ]);
  });
});

// ---- W5・W6: 出力 1 ----

describe("W5 想定ごとのカード(上を取る最低ライン)", () => {
  test("応答を待つ間は読み込み中。応答が来ると想定の順にカードを出し、見出しは h3 で相手の素早さを言う", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    expect(within(matchupRegion()).getByText(speedScreenText.matchupLoadingNotice)).toBeInTheDocument();

    await resolveMatchup(rendered.client, matchupResponse);

    expect(within(matchupRegion()).queryByText(speedScreenText.matchupLoadingNotice)).not.toBeInTheDocument();
    const all = cards();
    expect(all.map((card) => card.getAttribute("data-preset"))).toEqual([
      "uninvested",
      "neutral-max",
      "max",
      "max-scarf",
    ]);
    const heading = within(cardFor("max")).getByRole("heading", { level: 3 });
    expect(heading).toHaveTextContent(speedScreenText.cardHeading("max", 145));
    // 領域の見出しは h2。
    expect(
      within(matchupRegion()).getByRole("heading", { level: 2, name: speedScreenText.matchupHeading }),
    ).toBeInTheDocument();
  });

  test("一言見出し: 補正なし 0 → 無振りで抜ける / 補正なしで足りる → 準速で抜ける / 上昇だけ → 最速なら S○ 以上", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    await resolveMatchup(rendered.client, matchupResponse);

    expect(within(cardFor("uninvested")).getByTestId("speed-headline")).toHaveTextContent(
      speedScreenText.headlineUninvested,
    );
    expect(within(cardFor("neutral-max")).getByTestId("speed-headline")).toHaveTextContent(
      speedScreenText.headlineNeutralMax,
    );
    expect(within(cardFor("max")).getByTestId("speed-headline")).toHaveTextContent(
      speedScreenText.headlinePlusOnly(12),
    );
  });

  test("性格ごとの最低ライン: 補正なし・上昇を出し、下降は通常の場では出さない(ADR-0610 §6 Q5)", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    await resolveMatchup(rendered.client, matchupResponse);

    const neutralMax = cardFor("neutral-max");
    expect(natureRow(neutralMax, "neutral")).toHaveTextContent(speedScreenText.natureLabel.neutral);
    expect(natureRow(neutralMax, "neutral")).toHaveTextContent(speedScreenText.movesFirstMin(20));
    expect(natureRow(neutralMax, "plus")).toHaveTextContent(speedScreenText.natureLabel.plus);
    expect(natureRow(neutralMax, "plus")).toHaveTextContent(speedScreenText.movesFirstMin(0));
    expect(queryNatureRow(neutralMax, "minus")).toBeUndefined();

    const max = cardFor("max");
    expect(natureRow(max, "neutral")).toHaveTextContent(speedScreenText.cannotMoveFirst);
    expect(natureRow(max, "plus")).toHaveTextContent(speedScreenText.movesFirstMin(12));
  });

  test("同速は上を取るに含めず、別に併記する(S11 で同速)。同速が無い行には出さない", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    await resolveMatchup(rendered.client, matchupResponse);

    const plus = natureRow(cardFor("max"), "plus");
    expect(plus).toHaveTextContent(speedScreenText.movesFirstMin(12));
    expect(plus).toHaveTextContent(speedScreenText.tieAt(11, 11));
    expect(natureRow(cardFor("neutral-max"), "plus")).not.toHaveTextContent(speedScreenText.tieLabel);
  });
});

describe("W6 どう振っても上を取れない", () => {
  test("どの性格でも届かず同速もないカードは「どう振っても上を取れません」と各行の「上を取れません」を出す", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    await resolveMatchup(rendered.client, unreachableMatchupResponse);

    const card = cardFor("max-scarf");
    expect(within(card).getByTestId("speed-headline")).toHaveTextContent(speedScreenText.unreachableNotice);
    expect(natureRow(card, "neutral")).toHaveTextContent(speedScreenText.cannotMoveFirst);
    expect(natureRow(card, "plus")).toHaveTextContent(speedScreenText.cannotMoveFirst);
    expect(within(card).queryByText(new RegExp(speedScreenText.tieLabel))).not.toBeInTheDocument();
  });

  test("上を取れなくても同速だけある場合は、届かない表示のまま同速の SP を併記する", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    await resolveMatchup(rendered.client, matchupResponse);

    const card = cardFor("max-scarf");
    expect(within(card).getByTestId("speed-headline")).toHaveTextContent(speedScreenText.unreachableNotice);
    expect(natureRow(card, "plus")).toHaveTextContent(speedScreenText.tieAt(32, 32));
    expect(natureRow(card, "plus")).toHaveTextContent(speedScreenText.cannotMoveFirst);
  });
});

// ---- W7: トリックルーム ----

describe("W7 トリックルーム", () => {
  test("オンにすると trickRoom: true で matchup() を呼び直し、見出しと読み方を切り替える", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    await resolveMatchup(rendered.client, matchupResponse);
    expect(
      within(matchupRegion()).queryByText(speedScreenText.matchupHeadingTrickRoom),
    ).not.toBeInTheDocument();

    await rendered.user.click(screen.getByRole("checkbox", { name: speedScreenText.trickRoomLabel }));
    expect(lastOf(rendered.client.matchupCalls, "matchup").args.trickRoom).toBe(true);
    await resolveMatchup(rendered.client, trickRoomMatchupResponse);

    expect(screen.getByRole("region", { name: speedScreenText.matchupHeadingTrickRoom })).toBeInTheDocument();
    const card = cardFor("max");
    // 一言見出しは補正なしの上限 SP(S28 まで振っても先に動ける)。
    expect(within(card).getByTestId("speed-headline")).toHaveTextContent(
      speedScreenText.headlineTrickRoom(28),
    );
    expect(natureRow(card, "neutral")).toHaveTextContent(speedScreenText.movesFirstMax(28));
    expect(natureRow(card, "neutral")).toHaveTextContent(speedScreenText.tieAt(29, 29));
    expect(natureRow(card, "plus")).toHaveTextContent(speedScreenText.movesFirstMax(20));
  });

  test("トリックルーム中だけ下降補正の行を出す", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    await rendered.user.click(screen.getByRole("checkbox", { name: speedScreenText.trickRoomLabel }));
    await resolveMatchup(rendered.client, trickRoomMatchupResponse);

    const card = cardFor("max");
    expect(natureRow(card, "minus")).toHaveTextContent(speedScreenText.natureLabel.minus);
    expect(natureRow(card, "minus")).toHaveTextContent(speedScreenText.movesFirstMax(32));
  });

  test("トリックルーム中に先に動けないときは「どう振っても先に動けません」", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    await rendered.user.click(screen.getByRole("checkbox", { name: speedScreenText.trickRoomLabel }));
    await resolveMatchup(rendered.client, { ...unreachableMatchupResponse, trickRoom: true });

    const card = cardFor("max-scarf");
    expect(within(card).getByTestId("speed-headline")).toHaveTextContent(
      speedScreenText.unreachableTrickRoomNotice,
    );
    expect(natureRow(card, "plus")).toHaveTextContent(speedScreenText.cannotMoveFirstTrickRoom);
  });
});

// ---- W8: 今の振り ----

describe("W8 今の振り", () => {
  test("既定では SP・性格の入力は出ず、reach() は呼ばない。「これ以上」の欄は入れ方を案内する", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);

    expect(
      within(selfRegion()).getByRole("checkbox", { name: speedScreenText.currentToggleLabel }),
    ).not.toBeChecked();
    expect(
      within(selfRegion()).queryByRole("spinbutton", { name: speedScreenText.spLabel }),
    ).not.toBeInTheDocument();
    expect(rendered.client.reachCalls).toHaveLength(0);
    expect(within(reachRegion()).getByText(speedScreenText.reachNeedsCurrent)).toBeInTheDocument();
  });

  test("入れると SP・性格の入力が出て、matchup の self.current に載り、reach() を custom の本文で呼ぶ", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    await enterCurrent(rendered, 20, speedScreenText.natureLabel.plus);

    expect(lastOf(rendered.client.matchupCalls, "matchup").args.self).toEqual({
      pokemonId: BIRD.pokemonId,
      current: { sp: 20, nature: "plus" },
    });
    expect(lastOf(rendered.client.reachCalls, "reach").args).toEqual({
      mode: "custom",
      pokemonId: BIRD.pokemonId,
      sp: 20,
      nature: "plus",
      rank: 0,
      scarf: false,
    });
  });

  test("reach の本文には自分のランク・スカーフ・追い風・まひと、相手側の追い風(tableTailwind)を載せる", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    await enterCurrent(rendered, 32, speedScreenText.natureLabel.neutral);
    const self = within(selfRegion());
    await rendered.user.click(
      self.getByRole("button", { name: `${speedScreenText.rankLabel}${uiText.rankIncreaseSuffix}` }),
    );
    await rendered.user.click(self.getByRole("checkbox", { name: speedScreenText.scarfLabel }));
    await rendered.user.click(self.getByRole("checkbox", { name: speedScreenText.selfTailwindLabel }));
    await rendered.user.click(self.getByRole("checkbox", { name: speedScreenText.selfParalysisLabel }));
    await rendered.user.click(
      within(opponentRegion()).getByRole("checkbox", { name: speedScreenText.opponentTailwindLabel }),
    );

    expect(lastOf(rendered.client.reachCalls, "reach").args).toEqual({
      mode: "custom",
      pokemonId: BIRD.pokemonId,
      sp: 32,
      nature: "neutral",
      rank: 1,
      scarf: true,
      tailwind: true,
      paralysis: true,
      tableTailwind: true,
    });
  });

  test("相手を選ばなくても、今の振りを入れれば reach() を呼ぶ(matchup は呼ばない)", async () => {
    const rendered = await renderLoaded();
    await chooseSelf(rendered);
    await enterCurrent(rendered, 10, speedScreenText.natureLabel.minus);

    expect(rendered.client.matchupCalls).toHaveLength(0);
    expect(lastOf(rendered.client.reachCalls, "reach").args).toMatchObject({ sp: 10, nature: "minus" });
  });

  test("行動順のバッジは文字で言う(先に動く・同速・後に動く)。自分の素早さも出す", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    await enterCurrent(rendered, 20, speedScreenText.natureLabel.plus);
    await resolveMatchup(rendered.client, {
      ...matchupResponse,
      selfSpeed: 167,
      lines: matchupResponse.lines.map((line, index) => {
        const order = (["self_first", "speed_tie", "opponent_first"] as const)[index];
        return order === undefined ? line : { ...line, current: order };
      }),
    });

    expect(within(cardFor("uninvested")).getByTestId("speed-current-order")).toHaveTextContent(
      speedScreenText.currentOrderLabel.self_first,
    );
    expect(within(cardFor("neutral-max")).getByTestId("speed-current-order")).toHaveTextContent(
      speedScreenText.currentOrderLabel.speed_tie,
    );
    expect(within(cardFor("max")).getByTestId("speed-current-order")).toHaveTextContent(
      speedScreenText.currentOrderLabel.opponent_first,
    );
    // current が無い想定のカードにはバッジを出さない。
    expect(within(cardFor("max-scarf")).queryByTestId("speed-current-order")).not.toBeInTheDocument();
    expect(within(matchupRegion()).getByText(speedScreenText.selfSpeedLabel(167))).toBeInTheDocument();
  });

  test("今の振りを入れない間はバッジも自分の素早さも出さない", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    await resolveMatchup(rendered.client, matchupResponse);
    expect(screen.queryByTestId("speed-current-order")).not.toBeInTheDocument();
    expect(screen.queryByText(/自分の素早さ/)).not.toBeInTheDocument();
  });
});

// ---- W9: 出力 2 ----

describe("W9 これ以上(境界のリスト)", () => {
  async function withReach(): Promise<Rendered> {
    const rendered = await renderLoaded();
    await chooseSelf(rendered);
    await enterCurrent(rendered, 20, speedScreenText.natureLabel.plus);
    await resolveReach(rendered.client, reachResponse);
    return rendered;
  }

  test("6 プリセットを表の順に 1 行ずつ出す(h2 の見出しは「これ以上」)", async () => {
    await withReach();
    const rows = within(reachRegion()).getAllByTestId("speed-reach-line");
    expect(rows.map((row) => row.getAttribute("data-preset"))).toEqual([
      "uninvested",
      "neutral-max",
      "max",
      "max-scarf",
      "max-plus1",
      "max-plus2",
    ]);
    expect(
      within(reachRegion()).getByRole("heading", { level: 2, name: speedScreenText.reachHeading }),
    ).toBeInTheDocument();
    expect(within(reachRegion()).getByText(speedScreenText.reachIntro)).toBeInTheDocument();
  });

  test("最速: 102 族まで抜ける・103 族と同速・104 族以上に抜かれる。数と境界の例を添える", async () => {
    await withReach();
    const row = reachRow("max");
    expect(within(row).getByTestId("speed-reach-slower")).toHaveTextContent(
      speedScreenText.reachSlower("max", 102),
    );
    expect(within(row).getByTestId("speed-reach-slower")).toHaveTextContent(speedScreenText.reachCount(40));
    expect(within(row).getByTestId("speed-reach-slower")).toHaveTextContent(
      speedScreenText.reachExample(BIRD.nameJa),
    );
    expect(within(row).getByTestId("speed-reach-tie")).toHaveTextContent(
      speedScreenText.reachTie("max", 103, 103),
    );
    expect(within(row).getByTestId("speed-reach-tie")).toHaveTextContent(speedScreenText.reachCount(1));
    expect(within(row).getByTestId("speed-reach-faster")).toHaveTextContent(
      speedScreenText.reachFaster("max", 104),
    );
    expect(within(row).getByTestId("speed-reach-faster")).toHaveTextContent(speedScreenText.reachCount(2));
    expect(within(row).getByTestId("speed-reach-faster")).toHaveTextContent(
      speedScreenText.reachExample(FISH.nameJa),
    );
  });

  test("同速の族が無い行(準速)は同速を出さない", async () => {
    await withReach();
    const row = reachRow("neutral-max");
    expect(within(row).getByTestId("speed-reach-slower")).toHaveTextContent(
      speedScreenText.reachSlower("neutral-max", 117),
    );
    expect(within(row).getByTestId("speed-reach-faster")).toHaveTextContent(
      speedScreenText.reachFaster("neutral-max", 118),
    );
    expect(within(row).queryByTestId("speed-reach-tie")).not.toBeInTheDocument();
  });

  test("全員抜ける・全員に抜かれる行は「なし」の文を出し、空の区間の文を作らない", async () => {
    await withReach();
    const allSlower = reachRow("max-scarf");
    expect(within(allSlower).getByTestId("speed-reach-slower")).toHaveTextContent(
      speedScreenText.reachSlower("max-scarf", 255),
    );
    expect(within(allSlower).getByTestId("speed-reach-faster")).toHaveTextContent(
      speedScreenText.reachNoneFaster,
    );

    const allFaster = reachRow("max-plus1");
    expect(within(allFaster).getByTestId("speed-reach-slower")).toHaveTextContent(
      speedScreenText.reachNoneSlower,
    );
    expect(within(allFaster).getByTestId("speed-reach-faster")).toHaveTextContent(
      speedScreenText.reachFaster("max-plus1", 1),
    );
  });

  test("トリックルーム中は読み方だけを切り替える(先に動ける・後に動く)。reach() は呼び直さない", async () => {
    const rendered = await withReach();
    const calls = rendered.client.reachCalls.length;

    await rendered.user.click(screen.getByRole("checkbox", { name: speedScreenText.trickRoomLabel }));

    expect(rendered.client.reachCalls).toHaveLength(calls);
    const row = reachRow("max");
    expect(within(row).getByTestId("speed-reach-slower")).toHaveTextContent(
      speedScreenText.reachSlowerTrickRoom("max", 102),
    );
    expect(within(row).getByTestId("speed-reach-faster")).toHaveTextContent(
      speedScreenText.reachFasterTrickRoom("max", 104),
    );
    expect(within(reachRow("max-scarf")).getByTestId("speed-reach-faster")).toHaveTextContent(
      speedScreenText.reachNoneFasterTrickRoom,
    );
  });
});

// ---- W10: 読み込み中・エラー・古い応答 ----

describe("W10 読み込み中・エラー・古い応答", () => {
  test("reach の応答を待つ間は「これ以上」の欄に読み込み中を出す", async () => {
    const rendered = await renderLoaded();
    await chooseSelf(rendered);
    await enterCurrent(rendered, 20, speedScreenText.natureLabel.plus);
    expect(within(reachRegion()).getByText(speedScreenText.reachLoadingNotice)).toBeInTheDocument();
  });

  test.each([
    ["unknown_pokemon", "unknown pokemonId: 9999-000"],
    ["master_unavailable", "pokemon read model is not configured"],
    ["upstream_unavailable", "upstream unavailable"],
    ["speed_unavailable", "network"],
    ["invalid_request", "invalid matchup"],
  ] as const)(
    "matchup のエラー %s は role=alert で code から日本語にし、サーバーの英語は出さない",
    async (code, message) => {
      const rendered = await renderLoaded();
      await chooseBoth(rendered);
      await act(async () => {
        lastOf(rendered.client.matchupCalls, "matchup").resolve({ ok: false, error: { code, message } });
        await Promise.resolve();
      });

      const alert = within(matchupRegion()).getByRole("alert");
      expect(alert).toHaveTextContent(speedScreenText.errorByCode[code]);
      expect(alert).not.toHaveTextContent(message);
      expect(screen.queryByTestId("speed-matchup-card")).not.toBeInTheDocument();
    },
  );

  test("未知の code は汎用の文言にする", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    await act(async () => {
      lastOf(rendered.client.matchupCalls, "matchup").resolve({
        ok: false,
        error: { code: "something_new", message: "boom" },
      });
      await Promise.resolve();
    });
    expect(within(matchupRegion()).getByRole("alert")).toHaveTextContent(speedScreenText.errorFallback);
  });

  test("matchup のエラーは「これ以上」を消さず、reach のエラーはカードを消さない(互いに独立)", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    await enterCurrent(rendered, 20, speedScreenText.natureLabel.plus);
    await act(async () => {
      lastOf(rendered.client.matchupCalls, "matchup").resolve({
        ok: false,
        error: { code: "internal_error", message: "x" },
      });
      await Promise.resolve();
    });
    await resolveReach(rendered.client, reachResponse);
    expect(within(matchupRegion()).getByRole("alert")).toBeInTheDocument();
    expect(within(reachRegion()).getAllByTestId("speed-reach-line")).toHaveLength(6);

    // 入力を変えて両方呼び直し、今度は matchup が成功・reach が失敗する。
    await rendered.user.click(
      within(selfRegion()).getByRole("checkbox", { name: speedScreenText.scarfLabel }),
    );
    await resolveMatchup(rendered.client, matchupResponse);
    await act(async () => {
      lastOf(rendered.client.reachCalls, "reach").resolve({
        ok: false,
        error: { code: "speed_unavailable", message: "x" },
      });
      await Promise.resolve();
    });
    expect(cards()).toHaveLength(4);
    expect(within(reachRegion()).getByRole("alert")).toHaveTextContent(
      speedScreenText.errorByCode.speed_unavailable,
    );
  });

  test("pokemon() が失敗したら role=alert を出す(入力欄は壊れない)", async () => {
    const client = createFakeSpeedClient();
    render(<SpeedScreen speedClient={client} />);
    await act(async () => {
      lastOf(client.pokemonCalls, "pokemon").resolve({
        ok: false,
        error: { code: "master_unavailable", message: "x" },
      });
      await Promise.resolve();
    });
    expect(screen.getByRole("alert")).toHaveTextContent(speedScreenText.errorByCode.master_unavailable);
    expect(screen.getByRole("combobox", { name: speedScreenText.selfPokemonLabel })).toBeInTheDocument();
  });

  test("古い応答は無視する(後から届いた前の入力の結果で、今の表示を上書きしない)", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    await rendered.user.click(screen.getByRole("checkbox", { name: speedScreenText.trickRoomLabel }));
    expect(rendered.client.matchupCalls).toHaveLength(2);

    // 新しい(トリックルーム)応答が先に届き、古い(通常の場)応答が後から届く。
    await resolveMatchup(rendered.client, trickRoomMatchupResponse, 1);
    await resolveMatchup(rendered.client, matchupResponse, 0);

    expect(screen.getByRole("region", { name: speedScreenText.matchupHeadingTrickRoom })).toBeInTheDocument();
    expect(cards()).toHaveLength(1);
  });

  test("入力を空にしたら、前のカードは消える(相手を未選択に戻す)", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    await resolveMatchup(rendered.client, matchupResponse);
    await rendered.user.selectOptions(
      screen.getByRole("combobox", { name: speedScreenText.opponentPokemonLabel }),
      speedScreenText.unselectedOption,
    );
    await waitFor(() => {
      expect(screen.queryByTestId("speed-matchup-card")).not.toBeInTheDocument();
    });
  });
});

// ---- W11: 連続技へのリンク ----

describe("W11 連続技へのリンク(連続技の画面ができるまで出さない。画面レジストリ ADR-0323)", () => {
  test("onOpenCombo が無いときは、相手を選んでもリンクも無効表示も出さない", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    await resolveMatchup(rendered.client, matchupResponse);
    expect(screen.queryByText(speedScreenText.comboLinkLabel)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: speedScreenText.comboLinkLabel })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: speedScreenText.comboLinkLabel })).not.toBeInTheDocument();
  });

  test("onOpenCombo があり、相手を選んでいるときだけボタンを出す", async () => {
    const onOpenCombo = vi.fn();
    const rendered = await renderLoaded(onOpenCombo);
    await chooseSelf(rendered);
    expect(screen.queryByRole("button", { name: speedScreenText.comboLinkLabel })).not.toBeInTheDocument();
    await chooseOpponent(rendered);
    expect(screen.getByRole("button", { name: speedScreenText.comboLinkLabel })).toBeInTheDocument();
  });

  test("押すと、攻撃側 = 自分・防御側 = 相手のポケモンを渡す(今の振りが無ければ speed は渡さない)", async () => {
    const onOpenCombo = vi.fn();
    const rendered = await renderLoaded(onOpenCombo);
    await chooseBoth(rendered);
    await rendered.user.click(screen.getByRole("button", { name: speedScreenText.comboLinkLabel }));

    expect(onOpenCombo).toHaveBeenCalledTimes(1);
    expect(onOpenCombo).toHaveBeenCalledWith({
      attackerPokemonId: BIRD.pokemonId,
      defenderPokemonId: FISH.pokemonId,
    } satisfies SpeedComboRequest);
  });

  test("今の振りがあれば、素早さの SP・性格・ランク・スカーフも渡す(ADR-0610 §6 Q7)", async () => {
    const onOpenCombo = vi.fn();
    const rendered = await renderLoaded(onOpenCombo);
    await chooseBoth(rendered);
    await enterCurrent(rendered, 20, speedScreenText.natureLabel.plus);
    await rendered.user.click(
      within(selfRegion()).getByRole("checkbox", { name: speedScreenText.scarfLabel }),
    );
    await rendered.user.click(screen.getByRole("button", { name: speedScreenText.comboLinkLabel }));

    expect(onOpenCombo).toHaveBeenCalledWith({
      attackerPokemonId: BIRD.pokemonId,
      defenderPokemonId: FISH.pokemonId,
      speed: { sp: 20, nature: "plus", rank: 0, scarf: true },
    } satisfies SpeedComboRequest);
  });

  test("素早さ画面はダメージ・確定数を出さない", async () => {
    const rendered = await renderLoaded(vi.fn());
    await chooseBoth(rendered);
    await resolveMatchup(rendered.client, matchupResponse);
    expect(screen.queryByText(/ダメージ|確定/)).not.toBeInTheDocument();
  });
});

// ---- W12: a11y・見た目 ----

describe("W12 a11y と見た目の部品", () => {
  test("選んだポケモンはポケモンカード(名前は h3、タイプバッジつき)で出す", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);

    expect(within(selfRegion()).getByRole("heading", { level: 3, name: BIRD.nameJa })).toBeInTheDocument();
    expect(
      within(opponentRegion()).getByRole("heading", { level: 3, name: FISH.nameJa }),
    ).toBeInTheDocument();
    expect(
      within(selfRegion()).getByRole("heading", { level: 3, name: BIRD.nameJa }).closest(".ui-pokemon-card"),
    ).not.toBeNull();
    // 画像が無ければタイプ色のエンブレム。
    expect(within(selfRegion()).getByTestId("pokemon-icon-emblem")).toBeInTheDocument();
  });

  test("領域の見出しは h2(自分・相手・場・最低ライン・これ以上)。領域に名前がある", async () => {
    await renderLoaded();
    for (const name of [
      speedScreenText.selfRegionLabel,
      speedScreenText.opponentRegionLabel,
      speedScreenText.matchupHeading,
      speedScreenText.reachHeading,
    ]) {
      expect(screen.getByRole("heading", { level: 2, name })).toBeInTheDocument();
      expect(screen.getByRole("region", { name })).toBeInTheDocument();
    }
    expect(screen.getByRole("group", { name: speedScreenText.fieldGroupLabel })).toBeInTheDocument();
  });

  test("ランクは RankMeter(slider)。同名の group を二重に作らない(asGroup=false)", async () => {
    await renderLoaded();
    const slider = within(selfRegion()).getByRole("slider", { name: speedScreenText.rankLabel });
    expect(slider).toHaveAttribute("aria-valuenow", "0");
    expect(
      within(selfRegion()).queryByRole("group", { name: speedScreenText.rankLabel }),
    ).not.toBeInTheDocument();
  });

  test("SP は Stepper(増減ボタン + 数値欄。0〜32 に収める)、性格は区切りボタン(radiogroup)", async () => {
    const rendered = await renderLoaded();
    await rendered.user.click(
      within(selfRegion()).getByRole("checkbox", { name: speedScreenText.currentToggleLabel }),
    );
    const stepper = within(selfRegion()).getByRole("group", { name: speedScreenText.spLabel });
    expect(within(stepper).getByRole("spinbutton", { name: speedScreenText.spLabel })).toHaveAttribute(
      "max",
      "32",
    );
    expect(within(stepper).getByRole("spinbutton", { name: speedScreenText.spLabel })).toHaveAttribute(
      "min",
      "0",
    );
    const natureGroup = within(selfRegion()).getByRole("radiogroup", {
      name: speedScreenText.natureGroupLabel,
    });
    expect(
      within(natureGroup)
        .getAllByRole("radio")
        .map((radio) => radio.textContent),
    ).toEqual([
      speedScreenText.natureLabel.minus,
      speedScreenText.natureLabel.neutral,
      speedScreenText.natureLabel.plus,
    ]);
  });

  test("結果は色だけに頼らない: 性格の行・見出し・行動順はすべて文字を持つ", async () => {
    const rendered = await renderLoaded();
    await chooseBoth(rendered);
    await resolveMatchup(rendered.client, matchupResponse);
    for (const row of within(matchupRegion()).getAllByTestId("speed-nature-line")) {
      expect(row.textContent).toMatch(/上昇|補正なし|下降/);
      expect(row.textContent).toMatch(/S\d|振らなくても|上を取れません/);
    }
  });

  test("常時動くアニメーションを入れない(CSS に infinite が無い)", () => {
    const css = readFileSync(localPath("./SpeedScreen.css", import.meta.url), "utf8");
    expect(css).not.toMatch(/infinite/);
  });
});
