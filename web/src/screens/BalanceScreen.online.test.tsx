// P4-16b: タイプバランスの画面を、機能が欠けたマスタ(オンライン相当)でどう見せるか(ADR-0304 A-9)。
// この画面の4つの診断のうち技に依存しないのは防御相性(analyze)だけで、技が空のまま threats /
// recommendations を呼ぶと「与える倍率は全部ゼロ」「攻撃範囲の穴は18タイプ全部」という誤解を招く結果が返る。
// そこで speciesList と moves が両方そろわないマスタでは、画面ごと「使えない」と案内して API を呼ばない。
// 確かめること:
//   - speciesList か moves が false: 案内を出し、入力は残すが全部 disabled、balance API を1本も呼ばない
//   - effects だけ false: balance API は ID しか送らないので、今までどおり動く
//   - capabilities を省いた既存のマスタでは今までどおり

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { BalanceClient } from "../api/balanceClient";
import { learnsetMoves } from "../domain/moves";
import type { Move } from "../engine/types";
import { masterOnlineText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import { ONLINE_MASTER_CAPABILITIES } from "../master/onlineSource";
import type { MasterCapabilities, MasterData, MasterSpecies } from "../master/types";
import { limitedMaster } from "../test/onlineMaster";
import { BalanceScreen } from "./BalanceScreen";

let example: MasterData;

beforeAll(async () => {
  example = await exampleMasterSource.load();
});

const NO_SPECIES_LIST: MasterCapabilities = { speciesList: false, moves: true, effects: true };
const NO_MOVES: MasterCapabilities = { speciesList: true, moves: false, effects: true };
const NO_EFFECTS: MasterCapabilities = { speciesList: true, moves: true, effects: false };

function speciesAt(index: number): MasterSpecies {
  const species = example.species[index];
  if (species === undefined) {
    throw new Error(`例データに ${index} 番目の種族が無い`);
  }
  return species;
}

/** 攻撃技を1つ以上覚える種族と、その技(coverage は攻撃技を選んだメンバーがいないと呼ばれない)。 */
function speciesWithDamagingMove(): { species: MasterSpecies; move: Move } {
  for (const species of example.species) {
    const move = learnsetMoves(species, example.moves).find((candidate) => candidate.category !== "status");
    if (move !== undefined) {
      return { species, move };
    }
  }
  throw new Error("例データに攻撃技を覚える種族が無い");
}

/** 呼び出しの有無だけを見る fake(応答は返さない = 画面は計算中のまま)。 */
interface CountingBalanceClient extends BalanceClient {
  readonly calls: string[];
}

function countingClient(): CountingBalanceClient {
  const calls: string[] = [];
  const never = <T,>(name: string): Promise<T> => {
    calls.push(name);
    return new Promise<T>(() => undefined);
  };
  return {
    calls,
    analyze: () => never("analyze"),
    coverage: () => never("coverage"),
    threats: () => never("threats"),
    recommendations: () => never("recommendations"),
  };
}

function renderScreen(master: MasterData): {
  user: UserEvent;
  client: CountingBalanceClient;
  rerenderMaster: (next: MasterData) => void;
} {
  const user = userEvent.setup();
  const client = countingClient();
  const rendered = render(<BalanceScreen master={master} client={client} />);
  return {
    user,
    client,
    rerenderMaster: (next) => {
      rendered.rerender(<BalanceScreen master={next} client={client} />);
    },
  };
}

const memberGroup = (n: number) => screen.getByRole("group", { name: `メンバー${String(n)}` });
const threatGroup = (n: number) => screen.getByRole("group", { name: `仮想敵${String(n)}` });

describe.each([
  ["ポケモンの一覧が無い(speciesList: false)", NO_SPECIES_LIST],
  ["技が無い(moves: false)", NO_MOVES],
  ["オンラインのマスタ(両方とも無い)", ONLINE_MASTER_CAPABILITIES],
])("%s とき、画面ごと使えないことを案内する(ADR-0304 A-9)", (_name, capabilities) => {
  test("使えない旨の案内を出す", () => {
    renderScreen(limitedMaster(example, capabilities));
    expect(screen.getByText(masterOnlineText.balanceUnavailable)).toBeInTheDocument();
  });

  test("パーティ・仮想敵の入力は残すが、全部 disabled にする", () => {
    renderScreen(limitedMaster(example, capabilities));
    for (const group of [memberGroup(1), threatGroup(1)]) {
      expect(within(group).getByRole("combobox", { name: "ポケモン" })).toBeDisabled();
      expect(within(group).getByRole("combobox", { name: "特性" })).toBeDisabled();
      for (const slot of [1, 2, 3, 4]) {
        expect(within(group).getByRole("combobox", { name: `技${String(slot)}` })).toBeDisabled();
      }
    }
    expect(screen.getByRole("button", { name: "メンバーを追加" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "仮想敵を追加" })).toBeDisabled();
  });

  test("balance API を1本も呼ばない(誤解を招く診断を出さない)", async () => {
    const { client } = renderScreen(limitedMaster(example, capabilities));
    expect(screen.getByText(masterOnlineText.balanceUnavailable)).toBeInTheDocument();
    // 応答を待つ画面ではないので、effect が動く猶予だけ置いてから確かめる。
    await act(async () => {
      await Promise.resolve();
    });
    expect(client.calls).toEqual([]);
  });
});

describe("持ち物・特性の効果データだけが無いマスタ(effects: false)", () => {
  test("balance API は ID しか送らないので、今までどおり動く", async () => {
    const master = limitedMaster(example, NO_EFFECTS);
    const { user, client } = renderScreen(master);
    expect(screen.queryByText(masterOnlineText.balanceUnavailable)).toBeNull();

    const select = within(memberGroup(1)).getByRole("combobox", { name: "ポケモン" });
    expect(select).not.toBeDisabled();
    await user.selectOptions(select, speciesAt(0).key);

    await waitFor(() => {
      expect(client.calls).toContain("analyze");
    });
  });
});

describe("capabilities を省いたマスタ(オフライン相当)は今までどおり", () => {
  test("案内を出さず、メンバーを選べば analyze を呼ぶ", async () => {
    const { user, client } = renderScreen(example);
    expect(screen.queryByText(masterOnlineText.balanceUnavailable)).toBeNull();

    await user.selectOptions(
      within(memberGroup(1)).getByRole("combobox", { name: "ポケモン" }),
      speciesAt(0).key,
    );

    await waitFor(() => {
      expect(client.calls).toContain("analyze");
    });
  });
});

describe("A-9 のガードが実際に効いていること(critic 指摘の回帰ガード)", () => {
  test("メンバーを選んだ状態でオンラインの capabilities に切り替わると、analyze を呼び直さない", async () => {
    // 先に使えるマスタで選ばせ、analyze が呼ばれることを確認する(入力が空だから呼ばれない、という
    // 見せかけの緑を避けるため)。member.speciesKey は画面のローカル state なので、master が変わっても
    // 選択は残る = balanceAvailable のガード以外に呼び出しを止めるものが無い状態を作れる。
    const { user, client, rerenderMaster } = renderScreen(example);
    await user.selectOptions(
      within(memberGroup(1)).getByRole("combobox", { name: "ポケモン" }),
      speciesAt(0).key,
    );
    await waitFor(() => {
      expect(client.calls).toContain("analyze");
    });
    const callsBeforeSwitch = client.calls.length;

    rerenderMaster(limitedMaster(example, ONLINE_MASTER_CAPABILITIES));

    expect(screen.getByText(masterOnlineText.balanceUnavailable)).toBeInTheDocument();
    await act(async () => {
      await Promise.resolve();
    });
    expect(client.calls.length).toBe(callsBeforeSwitch);
  });

  test("技・仮想敵まで選んだ状態で切り替わっても、4つの診断すべてを呼び直さない(P4-16c(6))", async () => {
    // 上のテストはポケモンしか選ばないので、coverage(攻撃技が要る)・threats(仮想敵が要る)は
    // ガードが無くても呼ばれない = ガード削除を検知できない。ここでは4つとも実際に呼ばれた状態を作り、
    // capabilities が切り替わったあとに1本も増えないことを確かめる(critic 指摘の回帰ガード)。
    const { user, client, rerenderMaster } = renderScreen(example);
    const { species, move } = speciesWithDamagingMove();

    await user.selectOptions(within(memberGroup(1)).getByRole("combobox", { name: "ポケモン" }), species.key);
    await user.selectOptions(within(memberGroup(1)).getByRole("combobox", { name: "技1" }), move.id);
    await user.selectOptions(
      within(threatGroup(1)).getByRole("combobox", { name: "ポケモン" }),
      speciesAt(1).key,
    );

    await waitFor(() => {
      expect(client.calls).toEqual(
        expect.arrayContaining(["analyze", "coverage", "threats", "recommendations"]),
      );
    });
    const callsBeforeSwitch = [...client.calls];

    rerenderMaster(limitedMaster(example, ONLINE_MASTER_CAPABILITIES));

    expect(screen.getByText(masterOnlineText.balanceUnavailable)).toBeInTheDocument();
    await act(async () => {
      await Promise.resolve();
    });
    expect(client.calls).toEqual(callsBeforeSwitch);
  });
});

describe("P4-17 の後もオンラインでは無効のまま(ADR-0304 A-13 の回帰ガード)", () => {
  // P4-17 で計算画面・逆算画面の技は戻ったが、それは「種族を選ぶと、その種族の learnset が解決される」
  // 経路であって、技の**全件一覧**は今も取れない。この画面は全件一覧(各枠の技を選ぶ UI)を前提にしており、
  // 技検索 UI は P4-17b の積み残しなので、ここを「有効なのに技が1つも選べない」状態にしてはいけない。
  test("ONLINE_MASTER_CAPABILITIES.moves は false のまま(全件一覧は取れない)", () => {
    expect(ONLINE_MASTER_CAPABILITIES.moves).toBe(false);
  });

  test("オンラインのマスタでは、案内を出したまま技の枠も disabled にする", () => {
    const master = limitedMaster(example, ONLINE_MASTER_CAPABILITIES);
    renderScreen(master);

    expect(master.moves).toEqual([]);
    expect(screen.getByText(masterOnlineText.balanceUnavailable)).toBeInTheDocument();
    for (const slot of [1, 2, 3, 4]) {
      expect(within(memberGroup(1)).getByRole("combobox", { name: `技${String(slot)}` })).toBeDisabled();
    }
  });
});
