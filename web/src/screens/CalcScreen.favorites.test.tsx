// P5-3c(ADR-0327): 計算画面の「攻撃側をお気に入りに追加」(ピン留め。作成は計算画面だけ)。
// engine は fake、マスタは架空の例データ、RecordClient は fake。
// 確かめること(受け入れ条件 AC-6〜AC-7):
//   AC-6 追加: recordClient があるときだけボタンを出す(省略 = オフラインは出さない)/ 攻撃側の種族が決まるまで disabled /
//        押すと createFavorite を1回呼び、本文は {label: 種族の日本語名, individual: {speciesKey, level: 50, natureId, sp(6キー・各0〜32・合計66以下)}}
//        (id・createdAt などのサーバー決定項目を含めない)/ 持ち物を選んでいれば itemId を含む /
//        201(新規)は role=status「お気に入りに追加しました」/ 200(同じ内容)は「すでにお気に入りに入っています」/
//        送信中は二重に呼ばない / 失敗(上限の 400 invalid_input など)は role=alert に見出し+サーバー message
//   AC-7 計算は独立(絶対ルール5): createFavorite が pending・失敗・reject でも計算結果は普通に出る(alert は追加の失敗だけ)
// 架空のデータだけを使う(ADR-0002)。

import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeAll, describe, expect, test, vi } from "vitest";
import { resolveNatureId } from "../api/apiEngine";
import type { components } from "../api/openapi.gen";
import { NEUTRAL_NATURE } from "../domain/requests";
import { favoritesCalcText } from "../i18n/favorites";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import type { RecordClient, RecordResult } from "../record/recordClient";
import { MEGA_FIRE, MEGA_FIRE_STONE, withMegaFixture } from "../test/megaMaster";
import { createFakeEngine } from "../test/fakeEngine";
import { CalcScreen } from "./CalcScreen";

type Favorite = components["schemas"]["Favorite"];
type FavoriteInput = components["schemas"]["FavoriteInput"];
type CreateValue = { favorite: Favorite; created: boolean };

let master: MasterData;
beforeAll(async () => {
  master = await exampleMasterSource.load();
});

interface FakeRecord extends RecordClient {
  readonly createMock: ReturnType<typeof vi.fn<RecordClient["createFavorite"]>>;
}

function fakeRecord(create: () => Promise<RecordResult<CreateValue>>): FakeRecord {
  const createMock = vi.fn<RecordClient["createFavorite"]>(() => create());
  const unused = () => Promise.reject(new Error("このテストでは使わない"));
  return {
    createMock,
    createFavorite: createMock,
    listFrequentOpponents: () => Promise.resolve({ ok: true, value: [] }),
    deleteDeviceData: unused,
    listFavorites: unused,
    deleteFavorite: unused,
    listCalcHistory: unused,
  };
}

function savedFavorite(input: FavoriteInput): Favorite {
  return {
    id: "1",
    label: input.label ?? null,
    individual: input.individual,
    createdAt: "2026-10-04T01:00:00Z",
    updatedAt: "2026-10-04T01:00:00Z",
  };
}

function okRecord(created: boolean): FakeRecord {
  const record: FakeRecord = fakeRecord(() => Promise.reject(new Error("unreachable")));
  record.createMock.mockImplementation((input) =>
    Promise.resolve({ ok: true, value: { favorite: savedFavorite(input), created } }),
  );
  return record;
}

const addButton = () => screen.queryByRole("button", { name: favoritesCalcText.addLabel });
const attackerSelect = () => screen.getByRole("combobox", { name: "攻撃側のポケモン" });
const defenderSelect = () => screen.getByRole("combobox", { name: "防御側のポケモン" });

function speciesAt(index: number) {
  const species = master.species[index];
  if (species === undefined) {
    throw new Error(`例データに ${index} 番目の種族が無い`);
  }
  return species;
}

function renderScreen(recordClient: RecordClient | undefined) {
  const user = userEvent.setup();
  const engine = createFakeEngine();
  render(<CalcScreen engine={engine} master={master} recordClient={recordClient} />);
  return { user, engine };
}

describe("AC-6 追加ボタン", () => {
  test("recordClient が無い(オフライン)ときはボタンを出さない", () => {
    renderScreen(undefined);
    expect(addButton()).toBeNull();
  });

  test("攻撃側の種族が決まるまで disabled、決まると押せる", async () => {
    const { user } = renderScreen(okRecord(true));
    expect(await screen.findByRole("button", { name: favoritesCalcText.addLabel })).toBeDisabled();
    await user.selectOptions(attackerSelect(), speciesAt(0).key);
    expect(addButton()).toBeEnabled();
  });

  test("押すと createFavorite を1回呼ぶ。本文は label=種族名・individual(Lv50・性格・SP)で、サーバー決定項目を含まない", async () => {
    const record = okRecord(true);
    const { user } = renderScreen(record);
    const species = speciesAt(1);
    await user.selectOptions(attackerSelect(), species.key);
    await user.click(await screen.findByRole("button", { name: favoritesCalcText.addLabel }));

    expect(record.createMock).toHaveBeenCalledTimes(1);
    const body = record.createMock.mock.calls[0]?.[0];
    expect(body?.label).toBe(species.nameJa);
    expect(body?.individual.speciesKey).toBe(species.key);
    expect(body?.individual.level).toBe(50);
    expect(body?.individual.natureId).not.toBe("");
    const sp = body?.individual.sp;
    expect(Object.keys(sp ?? {}).sort()).toEqual(["atk", "def", "hp", "spa", "spd", "spe"]);
    const values = Object.values(sp ?? {});
    for (const value of values) {
      expect(value).toBeGreaterThanOrEqual(0);
      expect(value).toBeLessThanOrEqual(32);
    }
    expect(values.reduce((sum, value) => sum + value, 0)).toBeLessThanOrEqual(66);
    expect(Object.keys(body ?? {}).sort()).toEqual(["individual", "label"]);
  });

  test("攻撃側の持ち物を選んでいれば itemId を含む", async () => {
    const record = okRecord(true);
    const { user } = renderScreen(record);
    await user.selectOptions(attackerSelect(), speciesAt(0).key);
    const itemSelect = screen.getByRole("combobox", { name: "攻撃側の持ち物" });
    const option = Array.from(itemSelect.querySelectorAll("option")).find((o) => o.value !== "");
    if (option === undefined) {
      throw new Error("持ち物の選択肢が無い");
    }
    await user.selectOptions(itemSelect, option.value);
    await user.click(await screen.findByRole("button", { name: favoritesCalcText.addLabel }));
    expect(record.createMock.mock.calls[0]?.[0].individual.itemId).toBe(option.value);
  });

  test("メガ種族を攻撃側に選ぶと、固定された requiredItemId が itemId に入る", async () => {
    const record = okRecord(true);
    const user = userEvent.setup();
    render(<CalcScreen engine={createFakeEngine()} master={withMegaFixture(master)} recordClient={record} />);
    await user.selectOptions(attackerSelect(), MEGA_FIRE.key);
    await user.click(await screen.findByRole("button", { name: favoritesCalcText.addLabel }));
    expect(record.createMock.mock.calls[0]?.[0].individual.itemId).toBe(MEGA_FIRE_STONE.id);
  });

  test("攻撃側を別の個体に切り替えたら、前の個体への結果表示は消える", async () => {
    const { user } = renderScreen(okRecord(true));
    await user.selectOptions(attackerSelect(), speciesAt(0).key);
    await user.click(await screen.findByRole("button", { name: favoritesCalcText.addLabel }));
    expect(await screen.findByText(favoritesCalcText.addedNotice)).toBeInTheDocument();
    await user.selectOptions(attackerSelect(), speciesAt(1).key);
    expect(screen.queryByText(favoritesCalcText.addedNotice)).toBeNull();
  });

  test("201 は「お気に入りに追加しました」を role=status に出す(alert は出さない)", async () => {
    const { user } = renderScreen(okRecord(true));
    await user.selectOptions(attackerSelect(), speciesAt(0).key);
    await user.click(await screen.findByRole("button", { name: favoritesCalcText.addLabel }));
    expect(await screen.findByText(favoritesCalcText.addedNotice)).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(favoritesCalcText.addedNotice);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  test("200(同じ内容)は「すでにお気に入りに入っています」(エラーにしない)", async () => {
    const { user } = renderScreen(okRecord(false));
    await user.selectOptions(attackerSelect(), speciesAt(0).key);
    await user.click(await screen.findByRole("button", { name: favoritesCalcText.addLabel }));
    expect(await screen.findByText(favoritesCalcText.alreadyNotice)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  test("送信中は二重に呼ばない(disabled)", async () => {
    let resolve: ((r: RecordResult<CreateValue>) => void) | undefined;
    const record = fakeRecord(
      () =>
        new Promise((r) => {
          resolve = r;
        }),
    );
    const { user } = renderScreen(record);
    await user.selectOptions(attackerSelect(), speciesAt(0).key);
    const button = await screen.findByRole("button", { name: favoritesCalcText.addLabel });
    await user.click(button);
    await user.click(button);
    expect(record.createMock).toHaveBeenCalledTimes(1);
    expect(button).toBeDisabled();
    await act(async () => {
      resolve?.({ ok: false, error: { code: "x", message: "y" } });
      await Promise.resolve();
    });
    expect(button).toBeEnabled();
  });

  test("上限などの失敗は role=alert に見出し+サーバー message、ボタンは再度押せる", async () => {
    const record = fakeRecord(() =>
      Promise.resolve({
        ok: false,
        error: { code: "invalid_input", message: "お気に入りは100件までです" },
      }),
    );
    const { user } = renderScreen(record);
    await user.selectOptions(attackerSelect(), speciesAt(0).key);
    await user.click(await screen.findByRole("button", { name: favoritesCalcText.addLabel }));
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(favoritesCalcText.addErrorHeading);
    expect(alert).toHaveTextContent("お気に入りは100件までです");
    expect(addButton()).toBeEnabled();
  });

  test("アンマウント後に応答が届いても setState しない(console.error なし)", async () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => undefined);
    let resolve: ((r: RecordResult<CreateValue>) => void) | undefined;
    const record = fakeRecord(
      () =>
        new Promise((r) => {
          resolve = r;
        }),
    );
    const user = userEvent.setup();
    const view = render(<CalcScreen engine={createFakeEngine()} master={master} recordClient={record} />);
    await user.selectOptions(attackerSelect(), speciesAt(0).key);
    await user.click(await screen.findByRole("button", { name: favoritesCalcText.addLabel }));
    view.unmount();
    await act(async () => {
      resolve?.({ ok: false, error: { code: "x", message: "y" } });
      await Promise.resolve();
    });
    expect(errorSpy).not.toHaveBeenCalled();
    errorSpy.mockRestore();
  });
});

describe("AC-7 計算は追加の成否に依存しない(絶対ルール5)", () => {
  test.each([
    ["pending", () => new Promise<RecordResult<CreateValue>>(() => undefined)],
    [
      "失敗",
      () => Promise.resolve<RecordResult<CreateValue>>({ ok: false, error: { code: "x", message: "y" } }),
    ],
    ["reject", () => Promise.reject<RecordResult<CreateValue>>(new Error("boom"))],
  ])("createFavorite が %s でも、計算結果は普通に出る", async (_name, create) => {
    const record = fakeRecord(create);
    const { user, engine } = renderScreen(record);
    await user.selectOptions(attackerSelect(), speciesAt(0).key);
    await user.selectOptions(defenderSelect(), speciesAt(1).key);
    await user.click(await screen.findByRole("button", { name: favoritesCalcText.addLabel }));
    await waitFor(() => {
      expect(engine.bulkRequests.length).toBeGreaterThan(0);
    });
    expect(screen.getByRole("list", { name: "計算結果" })).toBeInTheDocument();
  });
});

// ADR-0329 §7: お気に入りの攻撃側の SP・性格は、計算画面の「攻撃」「特攻」の入力から解決したものにする。
describe("お気に入りの SP・性格は攻撃・特攻の入力から決まる(ADR-0329 §7)", () => {
  async function typeSp(user: ReturnType<typeof userEvent.setup>, name: string, text: string) {
    const box = screen.getByRole("textbox", { name });
    await user.clear(box);
    await user.click(box);
    await user.paste(text);
  }

  test("攻撃 SP 20・特攻 SP 12 は individual.sp に載り、性格は無補正の natureId", async () => {
    const record = okRecord(true);
    const { user } = renderScreen(record);
    await user.selectOptions(attackerSelect(), speciesAt(0).key);
    await typeSp(user, "攻撃のSP", "20");
    await typeSp(user, "特攻のSP", "12");
    await user.click(await screen.findByRole("button", { name: favoritesCalcText.addLabel }));

    const individual = record.createMock.mock.calls[0]?.[0].individual;
    expect(individual?.sp).toEqual({ hp: 0, atk: 20, def: 0, spa: 12, spd: 0, spe: 0 });
    expect(individual?.natureId).toBe(resolveNatureId(master.natures, NEUTRAL_NATURE));
  });

  test("攻撃 SP が範囲外(33)のあいだは追加できない", async () => {
    const { user } = renderScreen(okRecord(true));
    await user.selectOptions(attackerSelect(), speciesAt(0).key);
    await typeSp(user, "攻撃のSP", "33");
    expect(await screen.findByRole("button", { name: favoritesCalcText.addLabel })).toBeDisabled();
  });
});
