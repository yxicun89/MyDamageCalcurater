// I-web-8 = F-09(ADR-0333 §2〜§5): お気に入りから計算画面へ入力を戻す(restoreRequest)と、
// 追加で「そのときの計算の入力」(calc)も保存すること。engine は fake、マスタは架空の例データ、RecordClient は fake。
// 確かめること:
//   R-1 mount 時に restoreRequest があれば入力を戻し、追加の操作なしで通常の calcBulk が走って結果が出る(role=status で「開きました」)
//   R-2 mount 済みの画面は token が変わったときだけ戻す(同じ token の再描画では戻さない=利用者の変更を消さない。
//       同じお気に入りでも token が進めばもう一度戻す)
//   R-3 古い結果を出さない: 戻す前の入力の応答が後から届いても表示しない(CompletedCalc)
//   R-4 戻せない項目(マスタに無い技・持ち物)は role=alert に日本語の見出し + ID で明示し、引けた部分は戻す。
//       持ち物が戻せないだけなら計算はそのまま走る
//   R-5 メガ種族の持ち物は固定を優先し、保存された持ち物と違えば通知する(ADR-0320)
//   R-6 calc の無い旧お気に入りは攻撃側だけを戻し(防御側はそのまま)、その旨を role=status で案内する
//   R-7 オンライン(種族は検索で都度引く)でも、保存された key を resolveSpecies で引いて戻す。引けなければ alert(防御側は戻す)
//   R-8 追加: 攻撃側・防御側・技が揃っていれば本文に calc を付け、label は「攻撃側→防御側(技)」、individual = calc.attacker。
//       戻した直後に追加すると、元の calc と同じ calc を送る(画面を通した往復)
// 架空のデータだけを使う(ADR-0002)。

import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeAll, describe, expect, test, vi } from "vitest";
import type { components } from "../api/openapi.gen";
import type { BulkRequest } from "../engine/types";
import type { FavoriteRestoreRequest } from "../favorites/favoriteCalc";
import { favoritesCalcText, favoritesRestoreText } from "../i18n/favorites";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpeciesResolution, MasterSpeciesSearch } from "../master/types";
import type { RecordClient, RecordResult } from "../record/recordClient";
import { bulkRow, createDeferredEngine, createFakeEngine, ok, type FakeEngine } from "../test/fakeEngine";
import { MEGA_FIRE, MEGA_FIRE_STONE, withMegaFixture } from "../test/megaMaster";
import { createFakeSpeciesSearch, limitedMaster } from "../test/onlineMaster";
import { CalcScreen } from "./CalcScreen";
import { moveTrigger, openMovePicker, selectedMoveId } from "../test/movePicker";

type Schemas = components["schemas"];
type Favorite = Schemas["Favorite"];
type CalcRequest = Schemas["CalcRequest"];

const ZERO = { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 } as const;
const NEUTRAL_ID = "example-nature-neutral-docile";

let master: MasterData;
beforeAll(async () => {
  master = await exampleMasterSource.load();
});

/** 条件をすべて既定から変えた計算(クライアントが保存する形。favoriteCalc.test.ts の FULL_STATE と同じ)。 */
const FULL_CALC: CalcRequest = {
  format: "single",
  attacker: {
    speciesKey: "9004-000",
    level: 50,
    natureId: "example-nature-spa",
    sp: { hp: 0, atk: 0, def: 0, spa: 32, spd: 0, spe: 0 },
    abilityId: "exampleabilityadapt",
    itemId: "exampleitempower",
    ranks: { atk: 0, def: 0, spa: 2, spd: 0, spe: 0 },
    status: "burn",
  },
  defender: {
    speciesKey: "9005-000",
    level: 50,
    natureId: NEUTRAL_ID,
    sp: ZERO,
    abilityId: "exampleabilitynone",
    itemId: "exampleitemspd",
    ranks: { atk: 0, def: 0, spa: 0, spd: -1, spe: 0 },
  },
  moveId: "examplemovethunder",
  field: {
    weather: "rain",
    terrain: "psychic",
    defenderScreens: { reflect: false, lightScreen: true, auroraVeil: false },
  },
  options: { critical: true },
};

function favoriteOf(
  calc: CalcRequest | undefined,
  label = "テストでんき→テストいわはがね(テストかみなり)",
): Favorite {
  return {
    id: "41",
    label,
    individual: calc?.attacker ?? {
      speciesKey: "9004-000",
      level: 50,
      natureId: "example-nature-spa",
      sp: { hp: 0, atk: 0, def: 0, spa: 32, spd: 0, spe: 0 },
      itemId: "exampleitempower",
      abilityId: "exampleabilityadapt",
    },
    ...(calc === undefined ? {} : { calc }),
    createdAt: "2026-10-04T01:00:00Z",
    updatedAt: "2026-10-04T01:00:00Z",
  };
}

function request(token: number, favorite: Favorite): FavoriteRestoreRequest {
  return { token, favorite };
}

const attackerSelect = () => screen.getByRole("combobox", { name: "攻撃側のポケモン" });
const defenderSelect = () => screen.getByRole("combobox", { name: "防御側のポケモン" });

function lastBulk(engine: FakeEngine): BulkRequest {
  const last = engine.bulkRequests.at(-1);
  if (last === undefined) {
    throw new Error("calcBulk が呼ばれていない");
  }
  return last;
}

interface FakeRecord extends RecordClient {
  readonly createMock: ReturnType<typeof vi.fn<RecordClient["createFavorite"]>>;
}

function okRecord(): FakeRecord {
  const createMock = vi.fn<RecordClient["createFavorite"]>((input) =>
    Promise.resolve<RecordResult<{ favorite: Favorite; created: boolean }>>({
      ok: true,
      value: {
        favorite: {
          id: "1",
          label: input.label ?? null,
          individual: input.individual,
          ...(input.calc === undefined ? {} : { calc: input.calc }),
          createdAt: "2026-10-04T01:00:00Z",
          updatedAt: "2026-10-04T01:00:00Z",
        },
        created: true,
      },
    }),
  );
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

describe("R-1 mount 時の復元", () => {
  test("入力全体が戻り、そのまま calcBulk が走って結果が出る。「開きました」を role=status で出す", async () => {
    const engine = createFakeEngine();
    const favorite = favoriteOf(FULL_CALC);
    render(<CalcScreen engine={engine} master={master} restoreRequest={request(1, favorite)} />);

    expect(await screen.findByRole("list", { name: "計算結果" })).toBeInTheDocument();
    const bulk = lastBulk(engine);
    expect(bulk.attacker.species.key).toBe("9004-000");
    expect(bulk.attacker.item?.id).toBe("exampleitempower");
    expect(bulk.attacker.ability.id).toBe("exampleabilityadapt");
    expect(bulk.attacker.sp).toEqual({ hp: 0, atk: 0, def: 0, spa: 32, spd: 0, spe: 0 });
    expect(bulk.attacker.nature).toEqual({ plus: "spa", minus: "atk" });
    expect(bulk.attacker.ranks).toEqual({ atk: 0, def: 0, spa: 2, spd: 0, spe: 0 });
    expect(bulk.attacker.status).toBe("burn");
    expect(bulk.defenderSpecies.key).toBe("9005-000");
    expect(bulk.move.id).toBe("examplemovethunder");
    expect(bulk.critical).toBe(true);
    expect(bulk.field).toEqual({
      weather: "rain",
      terrain: "psychic",
      defenderScreens: { reflect: false, lightScreen: true, auroraVeil: false },
    });
    expect(bulk.defenderOverride?.ranks).toEqual({ atk: 0, def: 0, spa: 0, spd: -1, spe: 0 });
    expect(bulk.defenderAbilities?.map((ability) => ability.id)).toEqual(["exampleabilitynone"]);
    expect(bulk.itemVariants?.map((item) => item?.id)).toEqual(["exampleitemspd"]);

    expect(attackerSelect()).toHaveValue("9004-000");
    expect(defenderSelect()).toHaveValue("9005-000");
    expect(selectedMoveId()).toBe("examplemovethunder");
    expect(
      await screen.findByText(favoritesRestoreText.restoredNotice(favorite.label ?? "")),
    ).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });
});

describe("R-2 mount 済みの画面は token の変化で戻す", () => {
  test("利用者が選んだ入力を、新しい token の要求で置き換える。同じ token の再描画では戻さない。token が進めばもう一度戻す", async () => {
    const user = userEvent.setup();
    const engine = createFakeEngine();
    const favorite = favoriteOf(FULL_CALC);
    const view = render(<CalcScreen engine={engine} master={master} />);
    await user.selectOptions(attackerSelect(), "9001-000");
    await user.selectOptions(defenderSelect(), "9002-000");
    await waitFor(() => {
      expect(lastBulk(engine).attacker.species.key).toBe("9001-000");
    });

    view.rerender(<CalcScreen engine={engine} master={master} restoreRequest={request(1, favorite)} />);
    await waitFor(() => {
      expect(lastBulk(engine).attacker.species.key).toBe("9004-000");
    });
    expect(lastBulk(engine).defenderSpecies.key).toBe("9005-000");

    // 戻したあとに利用者が変えた入力は、同じ token の再描画(別オブジェクト)で消さない
    await user.selectOptions(defenderSelect(), "9002-000");
    view.rerender(<CalcScreen engine={engine} master={master} restoreRequest={request(1, favorite)} />);
    await waitFor(() => {
      expect(lastBulk(engine).defenderSpecies.key).toBe("9002-000");
    });
    expect(defenderSelect()).toHaveValue("9002-000");

    // 同じお気に入りでも token が進めばもう一度戻す
    view.rerender(<CalcScreen engine={engine} master={master} restoreRequest={request(2, favorite)} />);
    await waitFor(() => {
      expect(lastBulk(engine).defenderSpecies.key).toBe("9005-000");
    });
  });
});

describe("R-3 古い結果を出さない", () => {
  test("戻す前の入力の応答が後から届いても表示せず、戻した入力の応答だけを出す", async () => {
    const user = userEvent.setup();
    const { engine, pending } = createDeferredEngine();
    const favorite = favoriteOf(FULL_CALC);
    const view = render(<CalcScreen engine={engine} master={master} />);
    await user.selectOptions(attackerSelect(), "9001-000");
    await user.selectOptions(defenderSelect(), "9002-000");
    await waitFor(() => {
      expect(pending.length).toBeGreaterThan(0);
    });
    const before = pending.length;

    view.rerender(<CalcScreen engine={engine} master={master} restoreRequest={request(1, favorite)} />);
    await waitFor(() => {
      expect(pending.length).toBeGreaterThan(before);
    });
    const older = pending[before - 1];
    const newer = pending.at(-1);
    expect(newer?.request.attacker.species.key).toBe("9004-000");

    await act(async () => {
      older?.resolve(ok({ defenderSpeciesKey: "9002-000", rows: [bulkRow({ presetLabel: "古い結果" })] }));
      await Promise.resolve();
    });
    expect(screen.queryByText("古い結果")).toBeNull();

    await act(async () => {
      newer?.resolve(ok({ defenderSpeciesKey: "9005-000", rows: [bulkRow({ presetLabel: "新しい結果" })] }));
      await Promise.resolve();
    });
    expect(screen.getByText("新しい結果")).toBeInTheDocument();
    expect(screen.queryByText("古い結果")).toBeNull();
  });
});

describe("R-4 戻せない項目の通知", () => {
  test("マスタに無い技: 種族・持ち物は戻し、技は未選択のまま。role=alert に見出しと技の ID を出す", async () => {
    const engine = createFakeEngine();
    const calc: CalcRequest = { ...FULL_CALC, moveId: "examplemovegone" };
    render(<CalcScreen engine={engine} master={master} restoreRequest={request(1, favoriteOf(calc))} />);

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(favoritesRestoreText.unresolvedHeading);
    expect(alert).toHaveTextContent("examplemovegone");
    expect(attackerSelect()).toHaveValue("9004-000");
    expect(defenderSelect()).toHaveValue("9005-000");
    expect(selectedMoveId()).toBe("");
    expect(engine.bulkRequests).toHaveLength(0);
  });

  test("マスタに無い持ち物だけなら、持ち物なしで計算はそのまま走り、alert に持ち物の ID を出す", async () => {
    const engine = createFakeEngine();
    const calc: CalcRequest = {
      ...FULL_CALC,
      attacker: { ...FULL_CALC.attacker, itemId: "exampleitemgone" },
    };
    render(<CalcScreen engine={engine} master={master} restoreRequest={request(1, favoriteOf(calc))} />);

    expect(await screen.findByRole("list", { name: "計算結果" })).toBeInTheDocument();
    expect(lastBulk(engine).attacker.item).toBeNull();
    expect(lastBulk(engine).move.id).toBe("examplemovethunder");
    expect(screen.getByRole("alert")).toHaveTextContent("exampleitemgone");
  });
});

describe("R-5 メガ種族の持ち物は固定を優先(ADR-0320)", () => {
  test("保存された持ち物がメガストーンと違えば、メガストーンで計算し、その旨を知らせる", async () => {
    const engine = createFakeEngine();
    const calc: CalcRequest = {
      format: "single",
      attacker: {
        speciesKey: MEGA_FIRE.key,
        level: 50,
        natureId: NEUTRAL_ID,
        sp: ZERO,
        itemId: "exampleitempower",
      },
      defender: { speciesKey: "9002-000", level: 50, natureId: NEUTRAL_ID, sp: ZERO },
      moveId: "examplemovetackle",
    };
    render(
      <CalcScreen
        engine={engine}
        master={withMegaFixture(master)}
        restoreRequest={request(1, favoriteOf(calc))}
      />,
    );
    await waitFor(() => {
      expect(lastBulk(engine).attacker.item?.id).toBe(MEGA_FIRE_STONE.id);
    });
    expect(screen.getByText(favoritesRestoreText.megaItemNotice)).toBeInTheDocument();
  });
});

describe("R-6 calc の無い旧お気に入り", () => {
  test("攻撃側だけを戻し(防御側はそのまま)、技は攻撃側の技に合わせ、その旨を role=status で案内する", async () => {
    const user = userEvent.setup();
    const engine = createFakeEngine();
    const legacy = favoriteOf(undefined, "テストでんき");
    const view = render(<CalcScreen engine={engine} master={master} />);
    await user.selectOptions(attackerSelect(), "9001-000");
    await user.selectOptions(defenderSelect(), "9002-000");
    await waitFor(() => {
      expect(engine.bulkRequests.length).toBeGreaterThan(0);
    });

    view.rerender(<CalcScreen engine={engine} master={master} restoreRequest={request(1, legacy)} />);
    await waitFor(() => {
      expect(lastBulk(engine).attacker.species.key).toBe("9004-000");
    });
    const bulk = lastBulk(engine);
    expect(bulk.defenderSpecies.key).toBe("9002-000");
    expect(bulk.move.id).toBe("examplemovethunder");
    expect(bulk.attacker.item?.id).toBe("exampleitempower");
    expect(bulk.attacker.ability.id).toBe("exampleabilityadapt");
    expect(bulk.attacker.sp.spa).toBe(32);
    expect(bulk.attacker.nature).toEqual({ plus: "spa", minus: "atk" });
    expect(screen.getByText(favoritesRestoreText.attackerOnlyNotice("テストでんき"))).toBeInTheDocument();
  });
});

describe("R-7 オンライン(種族は検索で都度引く)", () => {
  function onlineSetup(failKey?: string): {
    online: MasterData;
    search: MasterSpeciesSearch & { resolvedKeys: readonly string[] };
  } {
    const online = limitedMaster(master, { speciesList: false, moves: false, effects: true });
    const base = createFakeSpeciesSearch({
      species: master.species,
      abilities: master.abilities,
      moves: master.moves,
    });
    const search = {
      ...base,
      resolvedKeys: base.resolvedKeys,
      resolveSpecies(key: string, signal?: AbortSignal): Promise<MasterSpeciesResolution> {
        if (key === failKey) {
          return Promise.reject(new Error("見つからない"));
        }
        return base.resolveSpecies(key, signal);
      },
    };
    return { online, search };
  }

  test("保存された攻撃側・防御側の key を resolveSpecies で引いてから戻し、計算が走る", async () => {
    const engine = createFakeEngine();
    const { online, search } = onlineSetup();
    render(
      <CalcScreen
        engine={engine}
        master={online}
        masterSearch={search}
        restoreRequest={request(1, favoriteOf(FULL_CALC))}
      />,
    );
    expect(await screen.findByRole("list", { name: "計算結果" })).toBeInTheDocument();
    expect(search.resolvedKeys).toEqual(expect.arrayContaining(["9004-000", "9005-000"]));
    expect(lastBulk(engine).attacker.species.key).toBe("9004-000");
    expect(lastBulk(engine).move.id).toBe("examplemovethunder");
    expect(attackerSelect()).toHaveValue("テストでんき");
    expect(defenderSelect()).toHaveValue("テストいわはがね");
  });

  test("攻撃側の key が引けなければ alert に出し、防御側は戻す(画面は壊れない)", async () => {
    const engine = createFakeEngine();
    const { online, search } = onlineSetup("9004-000");
    render(
      <CalcScreen
        engine={engine}
        master={online}
        masterSearch={search}
        restoreRequest={request(1, favoriteOf(FULL_CALC))}
      />,
    );
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(favoritesRestoreText.unresolvedHeading);
    expect(alert).toHaveTextContent("9004-000");
    await waitFor(() => {
      expect(defenderSelect()).toHaveValue("テストいわはがね");
    });
    expect(engine.bulkRequests).toHaveLength(0);
  });
});

describe("R-8 追加は calc を付けて保存する", () => {
  test("攻撃側・防御側・技が揃っていれば calc を付け、label は「攻撃側→防御側(技)」、individual = calc.attacker", async () => {
    const user = userEvent.setup();
    const record = okRecord();
    render(<CalcScreen engine={createFakeEngine()} master={master} recordClient={record} />);
    await user.selectOptions(attackerSelect(), "9001-000");
    await user.selectOptions(defenderSelect(), "9002-000");
    await user.click(await screen.findByRole("button", { name: favoritesCalcText.addLabel }));

    expect(record.createMock).toHaveBeenCalledTimes(1);
    const body = record.createMock.mock.calls[0]?.[0];
    expect(body?.label).toBe("テストほのお→テストみず(テストたいあたり)");
    expect(body?.calc).toEqual({
      format: "single",
      attacker: {
        speciesKey: "9001-000",
        level: 50,
        natureId: NEUTRAL_ID,
        sp: ZERO,
        abilityId: "exampleabilitynone",
      },
      defender: { speciesKey: "9002-000", level: 50, natureId: NEUTRAL_ID, sp: ZERO },
      moveId: "examplemovetackle",
    });
    expect(body?.individual).toEqual(body?.calc?.attacker);
  });

  test("戻した直後に追加すると、元のお気に入りと同じ calc を送る(画面を通した往復)", async () => {
    const user = userEvent.setup();
    const record = okRecord();
    render(
      <CalcScreen
        engine={createFakeEngine()}
        master={master}
        recordClient={record}
        restoreRequest={request(1, favoriteOf(FULL_CALC))}
      />,
    );
    await screen.findByRole("list", { name: "計算結果" });
    await user.click(await screen.findByRole("button", { name: favoritesCalcText.addLabel }));
    expect(record.createMock.mock.calls[0]?.[0].calc).toEqual(FULL_CALC);
  });
});

describe("R-9 技を戻せなかったときの未選択の選択肢", () => {
  test("disabled の「技を選んでください」が選ばれている(別の技を選んだように見えない)", async () => {
    const calc: CalcRequest = { ...FULL_CALC, moveId: "examplemovegone" };
    render(
      <CalcScreen
        engine={createFakeEngine()}
        master={master}
        restoreRequest={request(1, favoriteOf(calc))}
      />,
    );
    await screen.findByRole("alert");
    // 技ピッカー: 何も選ばれておらず(data-value が空)、トリガーは「技を選んでください」を出し、
    // 開くと先頭の「技を選んでください」の行は disabled(選べない)。
    expect(selectedMoveId()).toBe("");
    expect(moveTrigger()).toHaveTextContent(favoritesRestoreText.moveUnselectedOption);
    await openMovePicker(userEvent.setup());
    const option = screen.getByRole("option", { name: favoritesRestoreText.moveUnselectedOption });
    expect(option).toHaveAttribute("aria-disabled", "true");
  });
});

describe("R-10 案内は利用者が入力を変えたら消える", () => {
  test("戻せなかった項目の alert・開いた旨の status は、入力を変えると消える", async () => {
    const user = userEvent.setup();
    const calc: CalcRequest = { ...FULL_CALC, moveId: "examplemovegone" };
    const favorite = favoriteOf(calc);
    render(<CalcScreen engine={createFakeEngine()} master={master} restoreRequest={request(1, favorite)} />);
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    await user.selectOptions(defenderSelect(), "9002-000");
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByText(favoritesRestoreText.restoredNotice(favorite.label ?? ""))).toBeNull();
  });
});

describe("R-11 オンラインで解決待ちの間に状況が変わったとき、古い解決を捨てる", () => {
  function deferredSearch() {
    const online = limitedMaster(master, { speciesList: false, moves: false, effects: true });
    const base = createFakeSpeciesSearch({
      species: master.species,
      abilities: master.abilities,
      moves: master.moves,
    });
    const pending: { key: string; release: () => void }[] = [];
    const search: MasterSpeciesSearch = {
      ...base,
      resolveSpecies(key: string, signal?: AbortSignal): Promise<MasterSpeciesResolution> {
        return new Promise((resolve, reject) => {
          pending.push({
            key,
            release: () => {
              base.resolveSpecies(key, signal).then(resolve, reject);
            },
          });
        });
      },
    };
    return { online, search, pending };
  }

  const SECOND: CalcRequest = {
    format: "single",
    attacker: { speciesKey: "9001-000", level: 50, natureId: NEUTRAL_ID, sp: ZERO },
    defender: { speciesKey: "9002-000", level: 50, natureId: NEUTRAL_ID, sp: ZERO },
    moveId: "examplemovetackle",
  };

  test("先の要求の解決が後から届いても、新しい要求の入力・案内を上書きしない", async () => {
    const engine = createFakeEngine();
    const { online, search, pending } = deferredSearch();
    const first = favoriteOf(FULL_CALC, "最初のお気に入り");
    const second = favoriteOf(SECOND, "次のお気に入り");
    const view = render(
      <CalcScreen engine={engine} master={online} masterSearch={search} restoreRequest={request(1, first)} />,
    );
    await waitFor(() => {
      expect(pending).toHaveLength(2);
    });
    view.rerender(
      <CalcScreen
        engine={engine}
        master={online}
        masterSearch={search}
        restoreRequest={request(2, second)}
      />,
    );
    await waitFor(() => {
      expect(pending).toHaveLength(4);
    });
    const [oldOnes, newOnes] = [pending.slice(0, 2), pending.slice(2)];
    await act(async () => {
      newOnes.forEach((entry) => {
        entry.release();
      });
      await Promise.resolve();
    });
    expect(
      await screen.findByText(favoritesRestoreText.restoredNotice("次のお気に入り")),
    ).toBeInTheDocument();
    await act(async () => {
      oldOnes.forEach((entry) => {
        entry.release();
      });
      await new Promise((resolve) => setTimeout(resolve, 20));
    });
    expect(screen.getByText(favoritesRestoreText.restoredNotice("次のお気に入り"))).toBeInTheDocument();
    expect(screen.queryByText(favoritesRestoreText.restoredNotice("最初のお気に入り"))).toBeNull();
    expect(attackerSelect()).toHaveValue("テストほのお");
  });

  test("解決待ちの間にアンマウントされたら、解決が届いても何もしない", async () => {
    const errors = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const engine = createFakeEngine();
    const { online, search, pending } = deferredSearch();
    const view = render(
      <CalcScreen
        engine={engine}
        master={online}
        masterSearch={search}
        restoreRequest={request(1, favoriteOf(FULL_CALC))}
      />,
    );
    await waitFor(() => {
      expect(pending).toHaveLength(2);
    });
    view.unmount();
    await act(async () => {
      pending.forEach((entry) => {
        entry.release();
      });
      await new Promise((resolve) => setTimeout(resolve, 20));
    });
    expect(engine.bulkRequests).toHaveLength(0);
    expect(errors).not.toHaveBeenCalled();
    errors.mockRestore();
  });
});
