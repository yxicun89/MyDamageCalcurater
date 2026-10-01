// issue 272(ADR-0126・ADR-0311): 計算画面の特性。engine は fake(ADR-0300 §8)。
// 確かめること:
//   - 攻撃側の「特性」セレクト: 選択肢は種族の特性(日本語名)、既定は先頭、種族を変えたら先頭に戻す、
//     選んだ特性が attacker.ability に入る。特性が1つの種族でも無効化せず選択肢1つで出す
//   - 防御側の「特性」セレクト: 「おまかせ(種族の全特性)」が既定 → defenderAbilities はスロット順に先頭3件まで
//     (4件目は落とす)。個別選択はその1件だけ(4件目も選べる)。防御側の種族を変えたらおまかせに戻す
//   - 特性が1つも無いマスタではセレクトを出さず、従来どおり NO_ABILITY・defenderAbilities なし
//   - 結果: 特性ごとに結果が分かれた行は特性名つきで別行(React key が特性込みで重複しない)。
//     まとめられた行(abilityIds が複数)はまとめた特性が分かる。特性が1つだけの種族は従来どおり名前を出さない
// 例データの種族は特性が1〜2件なので、3件以上・4件の境界は src/test/abilityMaster.ts の fixture で足す。

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { afterEach, beforeAll, describe, expect, test, vi } from "vitest";
import { NO_ABILITY } from "../domain/requests";
import type { BulkRequest } from "../engine/types";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import {
  abilityMasterFrom,
  noAbilityMasterFrom,
  quadAbilities,
  type AbilityMaster,
} from "../test/abilityMaster";
import { bulkRow, createFakeEngine, ok, physicalPresetRows, type FakeEngine } from "../test/fakeEngine";
import { CalcScreen } from "./CalcScreen";

let base: MasterData;
let fixture: AbilityMaster;

beforeAll(async () => {
  base = await exampleMasterSource.load();
  fixture = abilityMasterFrom(base);
});

afterEach(() => {
  vi.restoreAllMocks();
});

const attackerAbilitySelect = () => screen.getByRole("combobox", { name: "攻撃側の特性" });
const defenderAbilitySelect = () => screen.getByRole("combobox", { name: "防御側の特性" });
const attackerSpeciesSelect = () => screen.getByRole("combobox", { name: "攻撃側のポケモン" });
const defenderSpeciesSelect = () => screen.getByRole("combobox", { name: "防御側のポケモン" });

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

function renderScreen(
  master: MasterData,
  engine: FakeEngine = createFakeEngine(),
): { user: UserEvent; engine: FakeEngine } {
  const user = userEvent.setup();
  render(<CalcScreen engine={engine} master={master} />);
  return { user, engine };
}

async function choosePair(user: UserEvent, attacker: MasterSpecies, defender: MasterSpecies): Promise<void> {
  await user.selectOptions(attackerSpeciesSelect(), attacker.key);
  await user.selectOptions(defenderSpeciesSelect(), defender.key);
}

async function lastRequest(
  engine: FakeEngine,
  until?: (request: BulkRequest) => boolean,
): Promise<BulkRequest> {
  await waitFor(() => {
    const request = engine.bulkRequests.at(-1);
    expect(request).toBeDefined();
    if (request !== undefined && until !== undefined) {
      expect(until(request)).toBe(true);
    }
  });
  const request = engine.bulkRequests.at(-1);
  if (request === undefined) {
    throw new Error("calcBulk が呼ばれていない");
  }
  return request;
}

function abilityName(id: string): string {
  const found = fixture.master.abilities.find((ability) => ability.id === id);
  if (found === undefined) {
    throw new Error(`fixture に特性 ${id} が無い`);
  }
  return found.nameJa;
}

describe("攻撃側の特性", () => {
  test("選択肢は種族の特性(日本語名、スロット順)で、既定は先頭。calcBulk の attacker.ability も先頭", async () => {
    const { dual, single } = fixture;
    const { user, engine } = renderScreen(fixture.master);
    await choosePair(user, dual, single);

    expect(optionLabels(attackerAbilitySelect())).toEqual(dual.abilities.map(abilityName));
    expect(attackerAbilitySelect()).toHaveValue(dual.abilities[0]);
    const request = await lastRequest(engine);
    expect(request.attacker.ability.id).toBe(dual.abilities[0]);
  });

  test("特性を選ぶと attacker.ability に入る(マスタの実体のまま)", async () => {
    const { dual, single, master } = fixture;
    const { user, engine } = renderScreen(master);
    await choosePair(user, dual, single);
    const second = dual.abilities[1] ?? "";
    await user.selectOptions(attackerAbilitySelect(), second);

    const request = await lastRequest(engine, (r) => r.attacker.ability.id === second);
    expect(request.attacker.ability).toEqual(master.abilities.find((ability) => ability.id === second));
  });

  test("攻撃側の種族を変えると、特性は新しい種族の先頭に戻る(古い選択を引き継がない)", async () => {
    const { dual, quad, single, master } = fixture;
    const { user, engine } = renderScreen(master);
    await choosePair(user, dual, single);
    await user.selectOptions(attackerAbilitySelect(), dual.abilities[1] ?? "");

    await user.selectOptions(attackerSpeciesSelect(), quad.key);
    expect(attackerAbilitySelect()).toHaveValue(quad.abilities[0]);
    const request = await lastRequest(engine, (r) => r.attacker.species.key === quad.key);
    expect(request.attacker.ability.id).toBe(quad.abilities[0]);

    // 同じ種族を選び直しても戻る(別の種族へ変えたあと元の種族に戻したとき)
    await user.selectOptions(attackerSpeciesSelect(), dual.key);
    expect(attackerAbilitySelect()).toHaveValue(dual.abilities[0]);
  });

  test("特性が1つの種族でもセレクトを出し(無効化しない)、選択肢は1つ", async () => {
    const { single, dual, master } = fixture;
    const { user } = renderScreen(master);
    await choosePair(user, single, dual);
    expect(attackerAbilitySelect()).toBeEnabled();
    expect(optionValues(attackerAbilitySelect())).toEqual([single.abilities[0]]);
  });

  test("4特性の種族は4つとも選べる(選択肢は切り詰めない)", async () => {
    const { quad, single, master } = fixture;
    const { user } = renderScreen(master);
    await choosePair(user, quad, single);
    expect(optionLabels(attackerAbilitySelect())).toEqual(quadAbilities.map((ability) => ability.nameJa));
  });
});

describe("防御側の特性", () => {
  test("既定は「おまかせ(種族の全特性)」。選択肢は おまかせ + 種族の各特性、calcBulk に全特性をスロット順で渡す", async () => {
    const { single, dual, master } = fixture;
    const { user, engine } = renderScreen(master);
    await choosePair(user, single, dual);

    const select = defenderAbilitySelect();
    expect(select).toHaveValue("");
    const labels = optionLabels(select);
    expect(labels[0]).toMatch(/^おまかせ/);
    expect(labels.slice(1)).toEqual(dual.abilities.map(abilityName));
    const request = await lastRequest(engine);
    expect(request.defenderAbilities?.map((ability) => ability.id)).toEqual(dual.abilities);
  });

  test("おまかせは先頭 3 件まで: 4特性の種族でも defenderAbilities は 3 件(4件目は落とす)", async () => {
    const { single, quad, master } = fixture;
    const { user, engine } = renderScreen(master);
    await choosePair(user, single, quad);

    const request = await lastRequest(engine);
    expect(request.defenderAbilities?.map((ability) => ability.id)).toEqual(quad.abilities.slice(0, 3));
    // 選択肢には4件目も出す(個別なら選べる)
    expect(optionValues(defenderAbilitySelect())).toEqual(["", ...quad.abilities]);
  });

  test("個別選択はその1件だけ。4件目も選べる", async () => {
    const { single, quad, master } = fixture;
    const { user, engine } = renderScreen(master);
    await choosePair(user, single, quad);
    const fourth = quad.abilities[3] ?? "";
    await user.selectOptions(defenderAbilitySelect(), fourth);

    const request = await lastRequest(engine, (r) => r.defenderAbilities?.length === 1);
    expect(request.defenderAbilities?.map((ability) => ability.id)).toEqual([fourth]);
  });

  test("個別選択からおまかせに戻すと、再び全特性(先頭3件)を渡す", async () => {
    const { single, quad, master } = fixture;
    const { user, engine } = renderScreen(master);
    await choosePair(user, single, quad);
    await user.selectOptions(defenderAbilitySelect(), quad.abilities[1] ?? "");
    await user.selectOptions(defenderAbilitySelect(), "");

    const request = await lastRequest(engine, (r) => r.defenderAbilities?.length === 3);
    expect(request.defenderAbilities?.map((ability) => ability.id)).toEqual(quad.abilities.slice(0, 3));
  });

  test("防御側の種族を変えると、おまかせに戻る(古い個別選択が残らない)", async () => {
    const { single, dual, quad, master } = fixture;
    const { user, engine } = renderScreen(master);
    await choosePair(user, single, quad);
    await user.selectOptions(defenderAbilitySelect(), quad.abilities[3] ?? "");

    await user.selectOptions(defenderSpeciesSelect(), dual.key);
    expect(defenderAbilitySelect()).toHaveValue("");
    const request = await lastRequest(engine, (r) => r.defenderSpecies.key === dual.key);
    expect(request.defenderAbilities?.map((ability) => ability.id)).toEqual(dual.abilities);
  });

  test("特性が1つの防御側は選択肢が おまかせ + 1つ で、defenderAbilities も1件", async () => {
    const { single, master } = fixture;
    const { user, engine } = renderScreen(master);
    await choosePair(user, fixture.dual, single);
    expect(defenderAbilitySelect()).toBeEnabled();
    expect(optionValues(defenderAbilitySelect())).toEqual(["", single.abilities[0]]);
    const request = await lastRequest(engine);
    expect(request.defenderAbilities?.map((ability) => ability.id)).toEqual(single.abilities);
  });

  test("攻守入れ替えで、特性の選択(攻撃側 / 防御側)も入れ替わった種族に合わせて既定に戻る", async () => {
    const { single, quad, master } = fixture;
    const { user, engine } = renderScreen(master);
    await choosePair(user, single, quad);
    await user.selectOptions(defenderAbilitySelect(), quad.abilities[2] ?? "");

    await user.click(screen.getByRole("button", { name: "攻守入れ替え" }));
    // 入れ替え後の攻撃側 = quad(先頭)、防御側 = single(おまかせ)
    expect(attackerAbilitySelect()).toHaveValue(quad.abilities[0]);
    expect(defenderAbilitySelect()).toHaveValue("");
    const request = await lastRequest(engine, (r) => r.attacker.species.key === quad.key);
    expect(request.attacker.ability.id).toBe(quad.abilities[0]);
    expect(request.defenderAbilities?.map((ability) => ability.id)).toEqual(single.abilities);
  });
});

describe("特性の無いマスタ", () => {
  test("特性セレクトを出さず、従来どおり NO_ABILITY・defenderAbilities なしで計算する", async () => {
    const master = noAbilityMasterFrom(base);
    const [first, second] = master.species;
    if (first === undefined || second === undefined) {
      throw new Error("例データに種族が無い");
    }
    const { user, engine } = renderScreen(master);
    await choosePair(user, first, second);

    expect(screen.queryByRole("combobox", { name: "攻撃側の特性" })).toBeNull();
    expect(screen.queryByRole("combobox", { name: "防御側の特性" })).toBeNull();
    const request = await lastRequest(engine);
    expect(request.attacker.ability).toEqual(NO_ABILITY);
    expect(request).not.toHaveProperty("defenderAbilities");
  });
});

describe("結果の特性の表示", () => {
  async function resultItems(): Promise<HTMLElement[]> {
    const list = await screen.findByRole("list", { name: "計算結果" });
    return within(list).getAllByRole("listitem");
  }

  // 例データの特性(効果なし / てきおう相当)。dual の特性の並びは [adapt, none]。
  const none = "exampleabilitynone";
  const adapt = "exampleabilityadapt";

  test("結果が特性で分かれた行は、特性名つきの別行になる(React key の重複警告も出ない)", async () => {
    const { single, dual, master } = fixture;
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const engine = createFakeEngine(() =>
      ok({
        defenderSpeciesKey: dual.key,
        rows: physicalPresetRows
          .slice(0, 2)
          .flatMap((preset) => [
            bulkRow({ ...preset, abilityIds: [adapt], maxPercent: 0, minPercent: 0 }),
            bulkRow({ ...preset, abilityIds: [none] }),
          ]),
      }),
    );
    const { user } = renderScreen(master, engine);
    await choosePair(user, single, dual);

    const items = await resultItems();
    expect(items).toHaveLength(4);
    const names = items.map((item) =>
      [abilityName(adapt), abilityName(none)].filter((name) => within(item).queryByText(name) !== null),
    );
    expect(names).toEqual([
      [abilityName(adapt)],
      [abilityName(none)],
      [abilityName(adapt)],
      [abilityName(none)],
    ]);
    const duplicateKey = consoleError.mock.calls.some((call) => String(call[0]).includes("same key"));
    expect(duplicateKey).toBe(false);
  });

  test("まとめられた行(abilityIds が複数)は、まとめた特性の名前が分かる", async () => {
    const { single, dual, master } = fixture;
    const engine = createFakeEngine(() =>
      ok({
        defenderSpeciesKey: dual.key,
        rows: physicalPresetRows.map((preset) => bulkRow({ ...preset, abilityIds: [adapt, none] })),
      }),
    );
    const { user } = renderScreen(master, engine);
    await choosePair(user, single, dual);

    const items = await resultItems();
    expect(items).toHaveLength(physicalPresetRows.length);
    for (const item of items) {
      expect(within(item).getByText(new RegExp(abilityName(adapt)))).toBeInTheDocument();
      expect(within(item).getByText(new RegExp(abilityName(none)))).toBeInTheDocument();
    }
  });

  test("特性が1つの防御側は従来どおり: 特性名を行に出さない", async () => {
    const { single, master } = fixture;
    const only = single.abilities[0] ?? "";
    const engine = createFakeEngine(() =>
      ok({
        defenderSpeciesKey: single.key,
        rows: physicalPresetRows.map((preset) => bulkRow({ ...preset, abilityIds: [only] })),
      }),
    );
    const { user } = renderScreen(master, engine);
    await choosePair(user, fixture.dual, single);

    const items = await resultItems();
    for (const item of items) {
      expect(within(item).queryByText(new RegExp(abilityName(only)))).toBeNull();
    }
  });

  test("abilityIds の無い応答(特性を送らなかった / 古い応答)は従来どおり、特性名を出さない", async () => {
    const { single, dual, master } = fixture;
    const { user } = renderScreen(master);
    await choosePair(user, single, dual);
    const items = await resultItems();
    for (const item of items) {
      for (const ability of master.abilities) {
        expect(within(item).queryByText(ability.nameJa)).toBeNull();
      }
    }
  });
});
