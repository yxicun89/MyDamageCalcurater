// P5-5c: 計算画面の「よく計算する相手」チップ(ADR-0317、docs/design.md「画面: ダメージ計算」)。
// engine は fake、マスタは架空の例データ。RecordClient は fake(本物の通信はしない)。
// 確かめること(受け入れ条件 AC-2〜AC-6):
//   - 取得に成功し1件以上あるとき、名前「よく計算する相手」の group に、サーバーの順で button が並ぶ(名前 = 種族の日本語名)
//   - 取得は画面の表示時に1回だけ(選択・入力のたびに呼ばない)。アンマウントで abort する
//   - チップを押すと防御側にその種族がセットされ、(攻撃側・技が揃っていれば)計算が走る。キーボード(Enter)でも押せる
//   - 失敗・0件・マスタで引けない key だけ・prop 省略では group を出さず、alert も出さない
//   - 引けない key は個別に落とし、引けるものだけ出す
//   - 取得が終わらない(pending)・失敗しても、計算は普通に成功して結果が出る(計算は record に依存しない)
//   - 検索マスタ(speciesList: false)では resolveSpecies で名前を解決し、押すと防御側に入る
// 架空のデータだけを使う(ADR-0002)。

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test, vi } from "vitest";
import { firstDamagingMove } from "../domain/moves";
import { exampleMasterSource } from "../master/exampleSource";
import { ONLINE_MASTER_CAPABILITIES } from "../master/onlineSource";
import type { MasterData, MasterSpecies } from "../master/types";
import type { RecordClient, RecordResult } from "../record/recordClient";
import type { components } from "../api/openapi.gen";
import { createFakeEngine, type FakeEngine } from "../test/fakeEngine";
import { createFakeSpeciesSearch, limitedMaster } from "../test/onlineMaster";
import { CalcScreen } from "./CalcScreen";

// 全体を並列で流すと遅い環境で既定の 5 秒を超えることがあるため、このファイルだけ余裕を持たせる。
vi.setConfig({ testTimeout: 15000 });

type Opponent = components["schemas"]["FrequentOpponent"];

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

function speciesAt(index: number): MasterSpecies {
  const species = master.species[index];
  if (species === undefined) {
    throw new Error(`例データに ${index} 番目の種族が無い`);
  }
  return species;
}

function opponent(key: string, score: number): Opponent {
  return { speciesKey: key, score, count: 1, lastCalculatedAt: "2026-10-01T00:00:00Z" };
}

interface FakeRecord extends RecordClient {
  readonly listMock: ReturnType<typeof vi.fn<RecordClient["listFrequentOpponents"]>>;
}

function fakeRecord(result: () => Promise<RecordResult<Opponent[]>>): FakeRecord {
  const listMock = vi.fn<RecordClient["listFrequentOpponents"]>(() => result());
  return {
    listFrequentOpponents: listMock,
    deleteDeviceData: () => Promise.reject(new Error("このテストでは使わない")),
    listFavorites: () => Promise.reject(new Error("このテストでは使わない")),
    createFavorite: () => Promise.reject(new Error("このテストでは使わない")),
    deleteFavorite: () => Promise.reject(new Error("このテストでは使わない")),
    listMock,
  };
}

function okRecord(...opponents: Opponent[]): FakeRecord {
  return fakeRecord(() => Promise.resolve({ ok: true, value: opponents }));
}

const GROUP_NAME = "よく計算する相手";
const chipGroup = () => screen.queryByRole("group", { name: GROUP_NAME });
const attackerSelect = () => screen.getByRole("combobox", { name: "攻撃側のポケモン" });
const defenderSelect = () => screen.getByRole("combobox", { name: "防御側のポケモン" });

function renderScreen(recordClient: RecordClient | undefined, engine: FakeEngine = createFakeEngine()) {
  const user = userEvent.setup();
  render(<CalcScreen engine={engine} master={master} recordClient={recordClient} />);
  return { user, engine };
}

describe("よく計算する相手のチップ(表示)", () => {
  test("成功して1件以上あれば、サーバーの順で button が並ぶ(名前は種族の日本語名)", async () => {
    const [first, second] = [speciesAt(1), speciesAt(0)];
    renderScreen(okRecord(opponent(first.key, 3), opponent(second.key, 1)));

    const group = await screen.findByRole("group", { name: GROUP_NAME });
    const chips = within(group).getAllByRole("button");
    expect(chips.map((chip) => chip.textContent)).toEqual([first.nameJa, second.nameJa]);
  });

  test("取得は画面の表示時に1回だけで、選択を変えても呼び直さない", async () => {
    const record = okRecord(opponent(speciesAt(0).key, 1));
    const { user } = renderScreen(record);
    await screen.findByRole("group", { name: GROUP_NAME });

    await user.selectOptions(attackerSelect(), speciesAt(1).key);
    await user.selectOptions(defenderSelect(), speciesAt(0).key);
    expect(record.listMock).toHaveBeenCalledTimes(1);
  });

  test("アンマウントで取得を abort する(取得した signal が aborted になる)", async () => {
    const record = fakeRecord(() => new Promise(() => undefined));
    const view = render(<CalcScreen engine={createFakeEngine()} master={master} recordClient={record} />);
    await waitFor(() => {
      expect(record.listMock).toHaveBeenCalledTimes(1);
    });
    const signal = record.listMock.mock.calls[0]?.[0];
    expect(signal).toBeInstanceOf(AbortSignal);
    view.unmount();
    expect(signal?.aborted).toBe(true);
  });

  test("マスタで引けない key は個別に落とし、引けるものだけ出す", async () => {
    renderScreen(okRecord(opponent("0000-unknown", 5), opponent(speciesAt(0).key, 1)));
    const group = await screen.findByRole("group", { name: GROUP_NAME });
    expect(
      within(group)
        .getAllByRole("button")
        .map((chip) => chip.textContent),
    ).toEqual([speciesAt(0).nameJa]);
  });
});

describe("よく計算する相手のチップ(黙って非表示)", () => {
  async function expectSilentlyHidden(record: FakeRecord): Promise<void> {
    await waitFor(() => {
      expect(record.listMock).toHaveBeenCalledTimes(1);
    });
    // 応答を処理する時間を与えてから、何も出ていないことを確かめる。
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(chipGroup()).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByText(/記録の API/)).toBeNull();
  }

  test("0件なら group を出さない(エラーも出さない)", async () => {
    const record = okRecord();
    renderScreen(record);
    await expectSilentlyHidden(record);
  });

  test("失敗(ok: false)なら group も alert も出さない", async () => {
    const record = fakeRecord(() =>
      Promise.resolve({ ok: false, error: { code: "store_unavailable", message: "記録を読めません" } }),
    );
    renderScreen(record);
    await expectSilentlyHidden(record);
    expect(screen.queryByText("記録を読めません")).toBeNull();
  });

  test("client が reject しても(想定外)group も alert も出さない", async () => {
    const record = fakeRecord(() => Promise.reject(new Error("boom")));
    renderScreen(record);
    await expectSilentlyHidden(record);
  });

  test("すべての key がマスタで引けないなら group を出さない", async () => {
    const record = okRecord(opponent("0000-unknown", 5));
    renderScreen(record);
    await expectSilentlyHidden(record);
  });

  test("recordClient を渡さなければ(オフライン相当)group を出さず、通信もしない", () => {
    renderScreen(undefined);
    expect(chipGroup()).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });
});

describe("よく計算する相手のチップ(操作)", () => {
  test("チップを押すと防御側にその種族がセットされる(攻撃側は変わらない)", async () => {
    const attacker = speciesAt(0);
    const target = speciesAt(1);
    const { user } = renderScreen(okRecord(opponent(target.key, 2)));
    await user.selectOptions(attackerSelect(), attacker.key);

    const group = await screen.findByRole("group", { name: GROUP_NAME });
    await user.click(within(group).getByRole("button", { name: target.nameJa }));

    expect(defenderSelect()).toHaveValue(target.key);
    expect(attackerSelect()).toHaveValue(attacker.key);
  });

  test("攻撃側・技が揃っていれば、押した相手で計算が走る(防御側がそのキーで送られる)", async () => {
    const attacker = speciesAt(0);
    const target = speciesAt(1);
    const { user, engine } = renderScreen(okRecord(opponent(target.key, 2)));
    await user.selectOptions(attackerSelect(), attacker.key);
    expect(firstDamagingMove(attacker, master.moves)).toBeDefined();

    const group = await screen.findByRole("group", { name: GROUP_NAME });
    await user.click(within(group).getByRole("button", { name: target.nameJa }));

    await waitFor(() => {
      expect(engine.bulkRequests.length).toBeGreaterThan(0);
    });
    expect(engine.bulkRequests.at(-1)?.defenderSpecies.key).toBe(target.key);
  });

  test("キーボード(Tab で到達し Enter)で押せる", async () => {
    const target = speciesAt(1);
    const { user } = renderScreen(okRecord(opponent(target.key, 2)));
    const group = await screen.findByRole("group", { name: GROUP_NAME });
    const chip = within(group).getByRole("button", { name: target.nameJa });

    chip.focus();
    expect(chip).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(defenderSelect()).toHaveValue(target.key);
  });

  test("チップは type=button(フォーム送信などの副作用を持たない)", async () => {
    renderScreen(okRecord(opponent(speciesAt(0).key, 1)));
    const group = await screen.findByRole("group", { name: GROUP_NAME });
    for (const chip of within(group).getAllByRole("button")) {
      expect(chip).toHaveAttribute("type", "button");
    }
  });
});

describe("計算は record に依存しない(絶対ルール5)", () => {
  async function pickPairAndExpectRows(user: UserEvent, engine: FakeEngine): Promise<void> {
    await user.selectOptions(attackerSelect(), speciesAt(0).key);
    await user.selectOptions(defenderSelect(), speciesAt(1).key);
    await waitFor(() => {
      expect(engine.bulkRequests.length).toBeGreaterThan(0);
    });
    expect(await screen.findAllByRole("listitem")).not.toHaveLength(0);
  }

  test("取得が終わらない(pending)間も、計算して結果が出る", async () => {
    const record = fakeRecord(() => new Promise(() => undefined));
    const { user, engine } = renderScreen(record);
    await pickPairAndExpectRows(user, engine);
    expect(chipGroup()).toBeNull();
  });

  test("取得が失敗しても、計算して結果が出て、alert は出ない", async () => {
    const record = fakeRecord(() =>
      Promise.resolve({ ok: false, error: { code: "upstream_unavailable", message: "届きません" } }),
    );
    const { user, engine } = renderScreen(record);
    await pickPairAndExpectRows(user, engine);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  test("チップがあっても、計算のリクエストは recordClient の有無で変わらない", async () => {
    const withRecord = createFakeEngine();
    const withoutRecord = createFakeEngine();
    const first = renderScreen(okRecord(opponent(speciesAt(2).key, 1)), withRecord);
    await first.user.selectOptions(attackerSelect(), speciesAt(0).key);
    await first.user.selectOptions(defenderSelect(), speciesAt(1).key);
    await waitFor(() => {
      expect(withRecord.bulkRequests.length).toBeGreaterThan(0);
    });
    document.body.innerHTML = "";

    const second = renderScreen(undefined, withoutRecord);
    await second.user.selectOptions(attackerSelect(), speciesAt(0).key);
    await second.user.selectOptions(defenderSelect(), speciesAt(1).key);
    await waitFor(() => {
      expect(withoutRecord.bulkRequests.length).toBeGreaterThan(0);
    });
    expect(withRecord.bulkRequests.at(-1)).toEqual(withoutRecord.bulkRequests.at(-1));
  });
});

describe("検索マスタ(speciesList: false。オンライン)", () => {
  function renderSearch(record: RecordClient) {
    const limited = limitedMaster(master, ONLINE_MASTER_CAPABILITIES);
    const search = createFakeSpeciesSearch({ species: master.species, abilities: master.abilities });
    const user = userEvent.setup();
    const engine = createFakeEngine();
    render(<CalcScreen engine={engine} master={limited} masterSearch={search} recordClient={record} />);
    return { user, engine, search };
  }

  test("key を resolveSpecies で名前に解決してチップに出し、押すと防御側の検索欄にその名前が入る", async () => {
    const target = speciesAt(1);
    const { user, search } = renderSearch(okRecord(opponent(target.key, 2)));

    const group = await screen.findByRole("group", { name: GROUP_NAME });
    expect(search.resolvedKeys).toContain(target.key);
    await user.click(within(group).getByRole("button", { name: target.nameJa }));

    await waitFor(() => {
      expect(screen.getByRole("combobox", { name: "防御側のポケモン" })).toHaveValue(target.nameJa);
    });
  });

  test("チップ押下後に攻守入れ替えを押すと、攻撃側・防御側の検索欄の値が入れ替わる", async () => {
    const attacker = speciesAt(0);
    const target = speciesAt(1);
    const { user } = renderSearch(okRecord(opponent(target.key, 2)));
    await user.type(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), attacker.nameJa);
    await user.click(await screen.findByRole("option", { name: attacker.nameJa }));
    const group = await screen.findByRole("group", { name: GROUP_NAME });
    await user.click(within(group).getByRole("button", { name: target.nameJa }));
    await waitFor(() => {
      expect(screen.getByRole("combobox", { name: "防御側のポケモン" })).toHaveValue(target.nameJa);
    });

    await user.click(screen.getByRole("button", { name: "攻守入れ替え" }));

    await waitFor(() => {
      expect(screen.getByRole("combobox", { name: "攻撃側のポケモン" })).toHaveValue(target.nameJa);
    });
    expect(screen.getByRole("combobox", { name: "防御側のポケモン" })).toHaveValue(attacker.nameJa);
  });

  test("resolveSpecies が失敗した key は黙って落とす(他のチップは出る)", async () => {
    const good = speciesAt(0);
    renderSearch(okRecord(opponent("0000-unknown", 5), opponent(good.key, 1)));
    const group = await screen.findByRole("group", { name: GROUP_NAME });
    expect(
      within(group)
        .getAllByRole("button")
        .map((chip) => chip.textContent),
    ).toEqual([good.nameJa]);
    expect(screen.queryByRole("alert")).toBeNull();
  });
});
