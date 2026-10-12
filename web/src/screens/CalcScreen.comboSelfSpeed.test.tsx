// I-web-32(ADR-0359 追記): 連続技の「自分の素早さ」入力。
//   既定は今の挙動(要求の攻撃側が素早さについて変わらない)
//   SP・性格補正・ランクを変えると要求の攻撃側に載り、結果が引き直される
//   素早さ画面から渡る speed(SP・性格・ランク)を入力に反映する。スカーフは反映できない旨を案内する
//   素早さの例(reach)は反映後の自分の素早さで引く

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { ComboOpenRequest } from "../app/comboOpenRequest";
import type { Move } from "../engine/types";
import { comboScreenText } from "../i18n/combo";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { comboResultFor, createComboEngine, type ComboFakeEngine } from "../test/fakeCombo";
import { ok } from "../test/fakeEngine";
import { createFakeSpeedClient } from "../test/fakeSpeedClient";
import { chooseMove } from "../test/movePicker";
import { CalcScreen } from "./CalcScreen";

const TACKLE = "examplemovetackle";
const GROWL = "examplemovegrowl";
const DANCE = "examplemovedance";
const WATERBLAST = "examplemovewaterblast";

const danceMove: Move = {
  id: DANCE,
  nameJa: "テストりゅうのまい",
  type: "dragon",
  category: "status",
  power: 0,
  priority: 0,
  effect: { chance: 100, target: "self", stages: { atk: 1, spe: 1 } },
};

let master: MasterData;
let attacker: MasterSpecies;
let defender: MasterSpecies;

beforeAll(async () => {
  const base = await exampleMasterSource.load();
  const [first, second] = base.species;
  if (first === undefined || second === undefined) {
    throw new Error("例データが足りない");
  }
  attacker = { ...first, learnset: [TACKLE, GROWL, DANCE] };
  defender = { ...second, learnset: [WATERBLAST, TACKLE, GROWL] };
  master = {
    ...base,
    species: [attacker, defender, ...base.species.slice(2)],
    moves: [...base.moves, danceMove],
  };
});

const section = () => screen.getByRole("region", { name: "連続技" });
const speedGroup = () => within(section()).getByRole("group", { name: comboScreenText.selfSpeed.legend });
const lastRequest = (engine: ComboFakeEngine) => {
  const request = engine.comboRequests.at(-1);
  if (request === undefined) {
    throw new Error("連続技の要求が無い");
  }
  return request;
};

async function addDance(user: UserEvent): Promise<void> {
  await user.click(within(section()).getByRole("button", { name: "次の技" }));
  await user.click(within(section()).getByRole("combobox", { name: "追加の技" }));
  const row = screen
    .getByRole("listbox", { name: "追加の技" })
    .querySelector(`[role="option"][data-move-id="${DANCE}"]`);
  if (row === null) {
    throw new Error("追加の技の行が無い");
  }
  await user.click(row);
  await within(section()).findByText(/素早さ 132 → 198/);
}

async function start(engine: ComboFakeEngine = createComboEngine()) {
  const user = userEvent.setup();
  const speedClient = createFakeSpeedClient();
  render(<CalcScreen engine={engine} master={master} speedClient={speedClient} />);
  await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
  await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), defender.key);
  await chooseMove(user, TACKLE);
  await screen.findByRole("region", { name: "連続技" });
  await addDance(user);
  return { user, engine, speedClient };
}

describe("既定", () => {
  test("SP 0・性格そのまま・ランク 0 で、要求の攻撃側は素早さについて上書きされない", async () => {
    const { engine } = await start();
    const request = lastRequest(engine);
    expect(request.attacker.sp.spe).toBe(0);
    expect(request.attacker.ranks?.spe ?? 0).toBe(0);
    expect(
      within(speedGroup()).getByRole("spinbutton", { name: comboScreenText.selfSpeed.spLabel }),
    ).toHaveValue(0);
    expect(
      within(speedGroup()).getByRole("combobox", { name: comboScreenText.selfSpeed.natureLabel }),
    ).toHaveValue("keep");
    expect(within(speedGroup()).getByRole("slider")).toHaveAttribute("aria-valuenow", "0");
  });
});

describe("入力を変える", () => {
  test("SP・性格補正・ランクが要求の攻撃側に載り、そのたびに結果を引き直す", async () => {
    const { user, engine } = await start();
    const before = engine.comboRequests.length;
    const sp = within(speedGroup()).getByRole("spinbutton", { name: comboScreenText.selfSpeed.spLabel });
    await user.clear(sp);
    await user.type(sp, "32");
    await waitFor(() => {
      expect(lastRequest(engine).attacker.sp.spe).toBe(32);
    });
    await user.selectOptions(
      within(speedGroup()).getByRole("combobox", { name: comboScreenText.selfSpeed.natureLabel }),
      "plus",
    );
    await waitFor(() => {
      expect(lastRequest(engine).attacker.nature.plus).toBe("spe");
    });
    await user.click(
      within(speedGroup()).getByRole("button", { name: comboScreenText.selfSpeed.rankUpLabel }),
    );
    await waitFor(() => {
      expect(lastRequest(engine).attacker.ranks?.spe).toBe(1);
    });
    expect(engine.comboRequests.length).toBeGreaterThan(before);
  });

  test("反映後の素早さ(lastSelfSpeed)で素早さの例(reach)を引く", async () => {
    const engine = createComboEngine((request) =>
      ok(
        comboResultFor(request, {
          firstSelfSpeed: request.attacker.sp.spe === 32 ? 150 : 132,
          lastSelfSpeed: request.attacker.sp.spe === 32 ? 225 : 198,
        }),
      ),
    );
    const { user, speedClient } = await start(engine);
    const sp = within(speedGroup()).getByRole("spinbutton", { name: comboScreenText.selfSpeed.spLabel });
    await user.clear(sp);
    await user.type(sp, "32");
    await waitFor(() => {
      expect(speedClient.reachCalls.at(-1)?.args).toEqual({ mode: "raw", value: 225 });
    });
  });
});

describe("素早さ画面からの要求", () => {
  const request = (speed: ComboOpenRequest["speed"]): ComboOpenRequest => ({
    token: 1,
    attackerPokemonId: attacker.key,
    defenderPokemonId: defender.key,
    ...(speed === undefined ? {} : { speed }),
  });

  test("speed の SP・性格・ランクが入力に反映され、要求の攻撃側に載る", async () => {
    const engine = createComboEngine();
    render(
      <CalcScreen
        engine={engine}
        master={master}
        comboRequest={request({ sp: 20, nature: "plus", rank: 2, scarf: false })}
      />,
    );
    const group = await screen.findByRole("group", { name: comboScreenText.selfSpeed.legend });
    expect(within(group).getByRole("spinbutton", { name: comboScreenText.selfSpeed.spLabel })).toHaveValue(
      20,
    );
    expect(within(group).getByRole("combobox", { name: comboScreenText.selfSpeed.natureLabel })).toHaveValue(
      "plus",
    );
    expect(within(group).getByRole("slider")).toHaveAttribute("aria-valuenow", "2");
    expect(within(group).queryByText(comboScreenText.selfSpeed.scarfIgnored)).toBeNull();
  });

  test("scarf を渡されたら、反映できない旨を案内する(入力を触ると消える)", async () => {
    const user = userEvent.setup();
    render(
      <CalcScreen
        engine={createComboEngine()}
        master={master}
        comboRequest={request({ sp: 0, nature: "neutral", rank: 0, scarf: true })}
      />,
    );
    const group = await screen.findByRole("group", { name: comboScreenText.selfSpeed.legend });
    expect(within(group).getByText(comboScreenText.selfSpeed.scarfIgnored)).toBeInTheDocument();
    await user.click(within(group).getByRole("button", { name: comboScreenText.selfSpeed.rankUpLabel }));
    expect(within(group).queryByText(comboScreenText.selfSpeed.scarfIgnored)).toBeNull();
  });

  test("speed が無い要求は入力を既定のままにする", async () => {
    render(<CalcScreen engine={createComboEngine()} master={master} comboRequest={request(undefined)} />);
    const group = await screen.findByRole("group", { name: comboScreenText.selfSpeed.legend });
    expect(within(group).getByRole("spinbutton", { name: comboScreenText.selfSpeed.spLabel })).toHaveValue(0);
  });
});
