// CB-6 段階 C(ADR-0359 追記): 連続技の節の C1(素早さの種族の例)・C3(a11y の微調整)・C2(素早さ画面からの受け口)。
// engine は fake(calcCombo を持つ)、speed API は fake の SpeedClient。
// 確かめること:
//   C1 相手の素早さが不明のとき、最後の手の自分の素早さで reach(raw)を呼び、準速の「抜かれる / 抜ける / 同速」を族で出す
//      相手の素早さが分かるとき・speedClient が無い(オフライン)・reach が失敗・トリックルーム中は出さず、連続技の結果は壊れない
//      古い応答は捨てる(自分の素早さが変わった後に届いた前の応答は出さない)
//   C3 手を外した・閉じたあとのフォーカス、防御側の振りが消えたときの role=status
//   C2 openRequest(素早さ画面から)を受けたら攻撃側・防御側を入れ、連続技の節を広げる

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { ComboOpenRequest } from "../app/comboOpenRequest";
import type { Move } from "../engine/types";
import { comboScreenText } from "../i18n/combo";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { comboResultFor, createComboEngine, type ComboFakeEngine } from "../test/fakeCombo";
import { bulkResultFor, createFakeEngine, ok } from "../test/fakeEngine";
import {
  BIRD,
  FISH,
  createFakeSpeedClient,
  lastOf,
  reachResponse,
  type FakeSpeedClient,
} from "../test/fakeSpeedClient";
import { chooseMove } from "../test/movePicker";
import type { components } from "../speed/speed.gen";
import { CalcScreen } from "./CalcScreen";

type Schemas = components["schemas"];

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
let third: MasterSpecies;

beforeAll(async () => {
  const base = await exampleMasterSource.load();
  const [first, second, other] = base.species;
  if (first === undefined || second === undefined || other === undefined) {
    throw new Error("例データが足りない");
  }
  attacker = { ...first, learnset: [TACKLE, GROWL, DANCE] };
  defender = { ...second, learnset: [WATERBLAST, TACKLE, GROWL] };
  third = other;
  master = {
    ...base,
    species: [attacker, defender, third, ...base.species.slice(3)],
    moves: [...base.moves, danceMove],
  };
});

const section = () => screen.getByRole("region", { name: "連続技" });

async function addStep(user: UserEvent, id: string): Promise<void> {
  await user.click(within(section()).getByRole("combobox", { name: "追加の技" }));
  const listbox = screen.getByRole("listbox", { name: "追加の技" });
  const row = listbox.querySelector(`[role="option"][data-move-id="${id}"]`);
  if (row === null) {
    throw new Error(`追加の技 ${id} の行が無い`);
  }
  await user.click(row);
}

interface StartOptions {
  readonly engine?: ComboFakeEngine;
  readonly speedClient?: FakeSpeedClient | undefined;
  readonly withSpeedClient?: boolean;
}

async function start(options: StartOptions = {}) {
  const engine = options.engine ?? createComboEngine();
  const speedClient =
    options.withSpeedClient === false ? undefined : (options.speedClient ?? createFakeSpeedClient());
  const user = userEvent.setup();
  render(
    <CalcScreen engine={engine} master={master} {...(speedClient === undefined ? {} : { speedClient })} />,
  );
  await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), attacker.key);
  await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), defender.key);
  await chooseMove(user, TACKLE);
  await screen.findByRole("region", { name: "連続技" });
  return { user, engine, speedClient };
}

async function startWithTwoSteps(options: StartOptions = {}) {
  const started = await start(options);
  await started.user.click(within(section()).getByRole("button", { name: "次の技" }));
  await addStep(started.user, DANCE);
  await within(section()).findByText(/素早さ 132 → 198/);
  return started;
}

async function resolveReach(client: FakeSpeedClient, value: Schemas["ReachResponse"], index?: number) {
  await waitFor(() => {
    expect(client.reachCalls.length).toBeGreaterThan(index ?? 0);
  });
  const call = index === undefined ? lastOf(client.reachCalls, "reach") : client.reachCalls[index];
  await act(async () => {
    call?.resolve({ ok: true, value });
    await Promise.resolve();
  });
}

const exampleList = () =>
  within(section()).queryByRole("list", { name: comboScreenText.speedExample.listLabel });

describe("C1 素早さの種族の例", () => {
  test("相手の素早さが不明なら、最後の手の素早さ(raw)で reach を呼び、準速の抜かれる / 抜けるを族で出す", async () => {
    const { speedClient } = await startWithTwoSteps();
    if (speedClient === undefined) {
      throw new Error("fake が無い");
    }
    await waitFor(() => {
      expect(speedClient.reachCalls).toHaveLength(1);
    });
    expect(speedClient.reachCalls[0]?.args).toEqual({ mode: "raw", value: 198 });
    await resolveReach(speedClient, reachResponse);
    const list = await within(section()).findByRole("list", { name: comboScreenText.speedExample.listLabel });
    const items = within(list)
      .getAllByRole("listitem")
      .map((item) => item.textContent);
    expect(items).toEqual([
      `準速 118 族以上には抜かれる(例: ${BIRD.nameJa})`,
      `準速 117 族までは抜ける(例: ${FISH.nameJa})`,
    ]);
  });

  test("同速の族があれば「同速」の行も出す(最大 3 行。範囲は 〜 でつなぐ)", async () => {
    const { speedClient } = await startWithTwoSteps();
    if (speedClient === undefined) {
      throw new Error("fake が無い");
    }
    const neutralMax = reachResponse.lines.find((line) => line.preset === "neutral-max");
    if (neutralMax === undefined) {
      throw new Error("fixture が無い");
    }
    await resolveReach(speedClient, {
      ...reachResponse,
      lines: [
        {
          ...neutralMax,
          slower: { min: 1, max: 115 },
          tie: { min: 116, max: 117 },
          faster: { min: 118, max: 255 },
        },
      ],
    });
    const list = await within(section()).findByRole("list", { name: comboScreenText.speedExample.listLabel });
    expect(within(list).getAllByRole("listitem")).toHaveLength(3);
    expect(within(list).getByText("準速 116〜117 族とは同速")).toBeInTheDocument();
  });

  test("相手の素早さが分かるとき(from_defender)は reach を呼ばず、例も出さない", async () => {
    const engine = createComboEngine((request) => ok(comboResultFor(request, { opponentSpeedKnown: true })));
    const { speedClient } = await startWithTwoSteps({ engine });
    expect(await within(section()).findByText("相手の素早さ 169")).toBeInTheDocument();
    expect(speedClient?.reachCalls).toHaveLength(0);
    expect(exampleList()).toBeNull();
  });

  test("speedClient が無い(オフライン)ときは例を出さず、連続技の結果はそのまま出る", async () => {
    await startWithTwoSteps({ withSpeedClient: false });
    expect(exampleList()).toBeNull();
    expect(
      within(section()).getByText("197 以下を抜く / 198 は同速 / 199 以上は相手が先"),
    ).toBeInTheDocument();
  });

  test("reach が失敗しても例の行も alert も出さず、連続技の結果はそのまま出る", async () => {
    const { speedClient } = await startWithTwoSteps();
    if (speedClient === undefined) {
      throw new Error("fake が無い");
    }
    await waitFor(() => {
      expect(speedClient.reachCalls).toHaveLength(1);
    });
    await act(async () => {
      lastOf(speedClient.reachCalls, "reach").resolve({
        ok: false,
        error: { code: "speed_unavailable", message: "x" },
      });
      await Promise.resolve();
    });
    expect(exampleList()).toBeNull();
    expect(within(section()).queryByRole("alert")).toBeNull();
    expect(within(section()).getByText("素早さ 132 → 198")).toBeInTheDocument();
  });

  test("トリックルーム中は(読み方が逆になるので)例を出さず、reach も呼ばない", async () => {
    const { user, speedClient } = await startWithTwoSteps();
    if (speedClient === undefined) {
      throw new Error("fake が無い");
    }
    await resolveReach(speedClient, reachResponse);
    await within(section()).findByRole("list", { name: comboScreenText.speedExample.listLabel });
    await user.click(within(section()).getByRole("button", { name: "素早さの場" }));
    await user.click(within(section()).getByRole("checkbox", { name: "トリックルーム" }));
    await waitFor(() => {
      expect(exampleList()).toBeNull();
    });
  });

  test("手を外して結果が消えたあとに前の応答が届いても出さない", async () => {
    const { user, speedClient } = await startWithTwoSteps();
    if (speedClient === undefined) {
      throw new Error("fake が無い");
    }
    await waitFor(() => {
      expect(speedClient.reachCalls).toHaveLength(1);
    });
    // 前の値(198)の応答が出る前に、手を外して素早さを変える(1 手 + 相手の技なし = 計算しないので結果ごと消える)。
    await user.click(within(section()).getByRole("button", { name: "テストりゅうのまいを外す" }));
    await resolveReach(speedClient, reachResponse, 0);
    expect(exampleList()).toBeNull();
  });
});

describe("C3 フォーカスと通知", () => {
  test("追加の手を外したあと、フォーカスは「追加の技」のピッカーに移る", async () => {
    const { user } = await startWithTwoSteps();
    await user.click(within(section()).getByRole("button", { name: "テストりゅうのまいを外す" }));
    await waitFor(() => {
      expect(within(section()).getByRole("combobox", { name: "追加の技" })).toHaveFocus();
    });
  });

  test("「閉じる」のあと、フォーカスは「次の技」ボタンに移る", async () => {
    const { user } = await start();
    await user.click(within(section()).getByRole("button", { name: "次の技" }));
    await user.click(within(section()).getByRole("button", { name: "閉じる" }));
    await waitFor(() => {
      expect(within(section()).getByRole("button", { name: "次の技" })).toHaveFocus();
    });
  });

  test("選んでいた相手の振りが無くなって先頭に戻ったとき、role=status で知らせる", async () => {
    const engine = createComboEngine(
      undefined,
      createFakeEngine((request) => {
        const bulk = bulkResultFor(request);
        return ok(
          request.defenderSpecies.key === defender.key ? bulk : { ...bulk, rows: bulk.rows.slice(0, 1) },
        );
      }),
    );
    const { user } = await startWithTwoSteps({ engine });
    const select = within(section()).getByRole("combobox", { name: "相手の振り" });
    const second = within(select).getAllByRole("option")[1];
    if (second === undefined) {
      throw new Error("2 行目が無い");
    }
    await user.selectOptions(select, second);
    expect(within(section()).queryByText(comboScreenText.defenderReset)).toBeNull();
    await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), third.key);
    const notice = await within(section()).findByText(comboScreenText.defenderReset);
    expect(notice).toHaveAttribute("role", "status");
  });
});

describe("C2 素早さ画面からの要求(comboRequest)", () => {
  const request = (token: number, patch: Partial<ComboOpenRequest> = {}): ComboOpenRequest => ({
    token,
    attackerPokemonId: attacker.key,
    defenderPokemonId: defender.key,
    ...patch,
  });

  test("攻撃側・防御側を入れ、連続技の節を広げる(追加の技のピッカーが出る)", async () => {
    render(<CalcScreen engine={createComboEngine()} master={master} comboRequest={request(1)} />);
    await waitFor(() => {
      expect(screen.getByRole("combobox", { name: "攻撃側のポケモン" })).toHaveValue(attacker.key);
      // 防御側は攻撃側の反映を 1 フレーム待ってから入る(ADR-0359 追記)。
      expect(screen.getByRole("combobox", { name: "防御側のポケモン" })).toHaveValue(defender.key);
    });
    expect(await screen.findByRole("combobox", { name: "追加の技" })).toBeInTheDocument();
  });

  test("token が変わったときだけ入れ直す(同じ token の再描画では利用者の変更を消さない)", async () => {
    const engine = createComboEngine();
    const { rerender } = render(<CalcScreen engine={engine} master={master} comboRequest={request(1)} />);
    const user = userEvent.setup();
    await waitFor(() => {
      expect(screen.getByRole("combobox", { name: "防御側のポケモン" })).toHaveValue(defender.key);
    });
    await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), third.key);
    rerender(<CalcScreen engine={engine} master={master} comboRequest={request(1)} />);
    expect(screen.getByRole("combobox", { name: "防御側のポケモン" })).toHaveValue(third.key);
    rerender(
      <CalcScreen
        engine={engine}
        master={master}
        comboRequest={request(2, { attackerPokemonId: defender.key, defenderPokemonId: attacker.key })}
      />,
    );
    await waitFor(() => {
      expect(screen.getByRole("combobox", { name: "攻撃側のポケモン" })).toHaveValue(defender.key);
      expect(screen.getByRole("combobox", { name: "防御側のポケモン" })).toHaveValue(attacker.key);
    });
  });

  test("マスタに無いポケモン ID は(オフラインで引けないので)そのポケモンだけ入れず、画面は壊れない", async () => {
    render(
      <CalcScreen
        engine={createComboEngine()}
        master={master}
        comboRequest={request(1, { defenderPokemonId: "no-such-pokemon" })}
      />,
    );
    await waitFor(() => {
      expect(screen.getByRole("combobox", { name: "攻撃側のポケモン" })).toHaveValue(attacker.key);
    });
    expect(screen.getByRole("combobox", { name: "防御側のポケモン" })).toHaveValue("");
  });
});
