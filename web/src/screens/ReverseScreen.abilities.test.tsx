// issue 272(ADR-0126・ADR-0311): 逆算画面の特性。engine は fake(ADR-0300 §8)。
// 確かめること(計算画面の CalcScreen.abilities.test.tsx と同じ規則を、逆算の語彙で):
//   - 「自分の特性」: 種族の特性から選ぶ。既定は先頭、種族を変えたら先頭に戻す、known.ability に入る
//   - 「相手の特性」: 「おまかせ(種族の全特性)」が既定 → unknownAbilities はスロット順で先頭3件まで。
//     個別選択はその1件(4件目も選べる)。相手の種族を変えたらおまかせに戻す。side(与えた/受けた)によらない
//   - 特性が1つも無いマスタではセレクトを出さず、従来どおり NO_ABILITY・unknownAbilities なし
//   - 候補: 特性で分かれた候補は特性名つき(React key が特性込みで重複しない)。まとめられた候補は特性名が分かる

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, describe, expect, test, vi } from "vitest";
import { OBSERVATION_INPUT_DEBOUNCE_MS } from "../domain/observations";
import { NO_ABILITY } from "../domain/requests";
import type { ReverseRequest } from "../engine/types";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import {
  abilityMasterFrom,
  noAbilityMasterFrom,
  quadAbilities,
  type AbilityMaster,
} from "../test/abilityMaster";
import { createFakeEngine, ok, reverseCandidate, type FakeEngine } from "../test/fakeEngine";
import { ReverseScreen } from "./ReverseScreen";

let base: MasterData;
let fixture: AbilityMaster;

beforeAll(async () => {
  base = await exampleMasterSource.load();
  fixture = abilityMasterFrom(base);
});

afterEach(() => {
  vi.restoreAllMocks();
});

afterAll(() => {
  vi.useRealTimers();
});

const mySpeciesSelect = () => screen.getByRole("combobox", { name: "自分のポケモン" });
const theirSpeciesSelect = () => screen.getByRole("combobox", { name: "相手のポケモン" });
const myAbilitySelect = () => screen.getByRole("combobox", { name: "自分の特性" });
const theirAbilitySelect = () => screen.getByRole("combobox", { name: "相手の特性" });
const sideGroup = () => screen.getByRole("radiogroup", { name: "観測したダメージ" });

function optionLabels(select: HTMLElement): string[] {
  return within(select)
    .getAllByRole("option")
    .map((option) => option.textContent.trim());
}

function optionValues(select: HTMLElement): string[] {
  return within(select)
    .getAllByRole("option")
    .map((option) => option.getAttribute("value") ?? "");
}

function abilityName(id: string): string {
  const found = fixture.master.abilities.find((ability) => ability.id === id);
  if (found === undefined) {
    throw new Error(`fixture に特性 ${id} が無い`);
  }
  return found.nameJa;
}

function renderScreen(
  master: MasterData,
  engine: FakeEngine = createFakeEngine(),
): { user: UserEvent; engine: FakeEngine } {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime.bind(vi) });
  render(<ReverseScreen engine={engine} master={master} />);
  return { user, engine };
}

async function choosePair(user: UserEvent, mine: MasterSpecies, theirs: MasterSpecies): Promise<void> {
  await user.selectOptions(mySpeciesSelect(), mine.key);
  await user.selectOptions(theirSpeciesSelect(), theirs.key);
}

/** 観測に 45 を打ってデバウンスを終わらせる(calcReverse が呼ばれるところまで進める)。 */
async function observe(user: UserEvent): Promise<void> {
  await user.type(screen.getByRole("textbox", { name: "観測1" }), "45");
  act(() => {
    vi.advanceTimersByTime(OBSERVATION_INPUT_DEBOUNCE_MS);
  });
}

async function lastRequest(
  engine: FakeEngine,
  until: (request: ReverseRequest) => boolean = () => true,
): Promise<ReverseRequest> {
  await waitFor(() => {
    const request = engine.reverseRequests.at(-1);
    expect(request !== undefined && until(request)).toBe(true);
  });
  const request = engine.reverseRequests.at(-1);
  if (request === undefined) {
    throw new Error("calcReverse が呼ばれていない");
  }
  return request;
}

describe("自分の特性", () => {
  test("選択肢は種族の特性(日本語名)、既定は先頭で known.ability も先頭。選ぶと known.ability に入る", async () => {
    const { dual, single, master } = fixture;
    const { user, engine } = renderScreen(master);
    await choosePair(user, dual, single);

    expect(optionLabels(myAbilitySelect())).toEqual(dual.abilities.map(abilityName));
    expect(myAbilitySelect()).toHaveValue(dual.abilities[0]);
    await observe(user);
    expect((await lastRequest(engine)).known.ability.id).toBe(dual.abilities[0]);

    const second = dual.abilities[1] ?? "";
    await user.selectOptions(myAbilitySelect(), second);
    const request = await lastRequest(engine, (r) => r.known.ability.id === second);
    expect(request.known.ability).toEqual(master.abilities.find((ability) => ability.id === second));
  });

  test("自分の種族を変えると先頭に戻る", async () => {
    const { dual, quad, single, master } = fixture;
    const { user } = renderScreen(master);
    await choosePair(user, dual, single);
    await user.selectOptions(myAbilitySelect(), dual.abilities[1] ?? "");
    await user.selectOptions(mySpeciesSelect(), quad.key);
    expect(myAbilitySelect()).toHaveValue(quad.abilities[0]);
  });

  test("特性が1つの種族でもセレクトを出し(無効化しない)、選択肢は1つ", async () => {
    const { single, dual, master } = fixture;
    const { user } = renderScreen(master);
    await choosePair(user, single, dual);
    expect(myAbilitySelect()).toBeEnabled();
    expect(optionValues(myAbilitySelect())).toEqual([single.abilities[0]]);
  });

  test("受けたダメージ(自分 = 防御側)でも、選んだ特性が known.ability に入る", async () => {
    const { dual, single, master } = fixture;
    const { user, engine } = renderScreen(master);
    await user.click(within(sideGroup()).getByRole("radio", { name: "受けたダメージ" }));
    await choosePair(user, dual, single);
    const second = dual.abilities[1] ?? "";
    await user.selectOptions(myAbilitySelect(), second);
    await observe(user);

    const request = await lastRequest(engine, (r) => r.known.ability.id === second);
    expect(request.side).toBe("attacker");
  });
});

describe("相手の特性", () => {
  test("既定は「おまかせ(種族の全特性)」。選択肢は おまかせ + 種族の各特性、unknownAbilities は全特性をスロット順で", async () => {
    const { single, dual, master } = fixture;
    const { user, engine } = renderScreen(master);
    await choosePair(user, single, dual);

    expect(theirAbilitySelect()).toHaveValue("");
    const labels = optionLabels(theirAbilitySelect());
    expect(labels[0]).toMatch(/^おまかせ/);
    expect(labels.slice(1)).toEqual(dual.abilities.map(abilityName));
    await observe(user);
    const request = await lastRequest(engine);
    expect(request.unknownAbilities?.map((ability) => ability.id)).toEqual(dual.abilities);
  });

  test("おまかせは先頭 3 件まで(4特性の相手でも 3 件。4件目は落とす)。選択肢には 4 件目も出る", async () => {
    const { single, quad, master } = fixture;
    const { user, engine } = renderScreen(master);
    await choosePair(user, single, quad);
    await observe(user);

    const request = await lastRequest(engine);
    expect(request.unknownAbilities?.map((ability) => ability.id)).toEqual(quad.abilities.slice(0, 3));
    expect(optionValues(theirAbilitySelect())).toEqual(["", ...quad.abilities]);
  });

  test("個別選択はその1件だけ(4件目も選べる)", async () => {
    const { single, quad, master } = fixture;
    const { user, engine } = renderScreen(master);
    await choosePair(user, single, quad);
    const fourth = quad.abilities[3] ?? "";
    await user.selectOptions(theirAbilitySelect(), fourth);
    await observe(user);

    const request = await lastRequest(engine, (r) => r.unknownAbilities?.length === 1);
    expect(request.unknownAbilities?.map((ability) => ability.id)).toEqual([fourth]);
  });

  test("相手の種族を変えるとおまかせに戻る", async () => {
    const { single, dual, quad, master } = fixture;
    const { user, engine } = renderScreen(master);
    await choosePair(user, single, quad);
    await user.selectOptions(theirAbilitySelect(), quad.abilities[3] ?? "");
    await user.selectOptions(theirSpeciesSelect(), dual.key);
    expect(theirAbilitySelect()).toHaveValue("");
    await observe(user);

    const request = await lastRequest(engine, (r) => r.unknownSpecies.key === dual.key);
    expect(request.unknownAbilities?.map((ability) => ability.id)).toEqual(dual.abilities);
  });

  test("受けたダメージ(相手 = 攻撃側)でも相手の特性を unknownAbilities に渡す", async () => {
    const { single, quad, master } = fixture;
    const { user, engine } = renderScreen(master);
    await user.click(within(sideGroup()).getByRole("radio", { name: "受けたダメージ" }));
    await choosePair(user, single, quad);
    await user.selectOptions(theirAbilitySelect(), quad.abilities[1] ?? "");
    await observe(user);

    const request = await lastRequest(engine, (r) => r.unknownAbilities?.length === 1);
    expect(request.side).toBe("attacker");
    expect(request.unknownAbilities?.map((ability) => ability.id)).toEqual([quad.abilities[1]]);
  });
});

describe("特性の無いマスタ", () => {
  test("特性セレクトを出さず、従来どおり NO_ABILITY・unknownAbilities なしで逆算する", async () => {
    const master = noAbilityMasterFrom(base);
    const [first, second] = master.species;
    if (first === undefined || second === undefined) {
      throw new Error("例データに種族が無い");
    }
    const { user, engine } = renderScreen(master);
    await choosePair(user, first, second);
    await observe(user);

    expect(screen.queryByRole("combobox", { name: "自分の特性" })).toBeNull();
    expect(screen.queryByRole("combobox", { name: "相手の特性" })).toBeNull();
    const request = await lastRequest(engine);
    expect(request.known.ability).toEqual(NO_ABILITY);
    expect(request).not.toHaveProperty("unknownAbilities");
  });
});

describe("候補の特性の表示", () => {
  const none = "exampleabilitynone";
  const adapt = "exampleabilityadapt";

  async function candidateCards(): Promise<HTMLElement[]> {
    const list = await screen.findByRole("list", { name: "推定結果" });
    return within(list).getAllByRole("listitem");
  }

  function engineReturning(abilityIdsPerCandidate: ReadonlyArray<readonly string[] | undefined>): FakeEngine {
    return createFakeEngine(undefined, (request) => {
      const candidates = abilityIdsPerCandidate.map((abilityIds) =>
        reverseCandidate(
          abilityIds === undefined ? {} : { abilityId: abilityIds[0] ?? "", abilityIds: [...abilityIds] },
        ),
      );
      return ok({
        side: request.side,
        stat: "def",
        assumedHpSp: 32,
        exactCount: candidates.length,
        candidates,
      });
    });
  }

  test("特性で分かれた候補は特性名つきの別カードになる(React key の重複警告も出ない)", async () => {
    const { single, dual, master } = fixture;
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const { user } = renderScreen(master, engineReturning([[adapt], [none], [adapt], [none]]));
    await choosePair(user, single, dual);
    await observe(user);

    const cards = await candidateCards();
    expect(cards).toHaveLength(4);
    const shown = cards.map((card) =>
      [adapt, none].filter((id) => within(card).queryByText(abilityName(id)) !== null),
    );
    expect(shown).toEqual([[adapt], [none], [adapt], [none]]);
    expect(consoleError.mock.calls.some((call) => String(call[0]).includes("same key"))).toBe(false);
  });

  test("まとめられた候補(abilityIds が複数)は、まとめた特性の名前が分かる", async () => {
    const { single, dual, master } = fixture;
    const { user } = renderScreen(
      master,
      engineReturning([
        [adapt, none],
        [adapt, none],
      ]),
    );
    await choosePair(user, single, dual);
    await observe(user);

    for (const card of await candidateCards()) {
      expect(within(card).getByText(new RegExp(abilityName(adapt)))).toBeInTheDocument();
      expect(within(card).getByText(new RegExp(abilityName(none)))).toBeInTheDocument();
    }
  });

  test("特性が1つの相手は従来どおり: 特性名を候補に出さない", async () => {
    const { dual, single, master } = fixture;
    const only = single.abilities[0] ?? "";
    const { user } = renderScreen(master, engineReturning([[only], [only]]));
    await choosePair(user, dual, single);
    await observe(user);

    for (const card of await candidateCards()) {
      expect(within(card).queryByText(new RegExp(abilityName(only)))).toBeNull();
    }
  });

  test("特性が複数の種族の候補は、1件でも特性名を出す(どの特性での結果か分かる)", async () => {
    const { single, quad, master } = fixture;
    const { user } = renderScreen(master, engineReturning([[quad.abilities[0] ?? ""]]));
    await choosePair(user, single, quad);
    await observe(user);
    const cards = await candidateCards();
    expect(within(cards[0] as HTMLElement).getByText(quadAbilities[0]?.nameJa ?? "")).toBeInTheDocument();
  });
});
