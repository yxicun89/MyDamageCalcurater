// ADR-0358(案 A)段 2: 計算画面の直近の入力を localStorage に 1 件保存し、再訪(mount)時に復元する。
//   L-1 攻撃側・防御側・技を選ぶと保存する(1 件を上書き)
//   L-2 mount 時に保存があれば、お気に入りの復元と同じ道で入力を戻し、そのまま計算が走る。案内(status / alert)は出さない
//   L-3 復元できない key(マスタに無い種族・技・持ち物)は安全に捨てる(落ちない・alert を出さない・入力は空のまま)
//   L-4 お気に入りからの復元要求(restoreRequest)があれば、そちらを優先し、保存した入力は使わない
//   L-5 壊れた保存値・使えないストレージでも画面は動く
// engine は fake、マスタは架空の例データ(ADR-0002)。

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { LAST_CALC_INPUT_STORAGE_KEY, loadLastCalcInput, saveLastCalcInput } from "../app/lastCalcInput";
import type { StorageLike } from "../api/storage";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import { createFakeEngine } from "../test/fakeEngine";
import { selectedMoveId } from "../test/movePicker";
import { CalcScreen } from "./CalcScreen";

type FavoriteInput = components["schemas"]["FavoriteInput"];

const ZERO = { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 } as const;
const NEUTRAL_ID = "example-nature-neutral-docile";

let master: MasterData;
beforeAll(async () => {
  master = await exampleMasterSource.load();
});

const attackerSelect = () => screen.getByRole("combobox", { name: "攻撃側のポケモン" });
const defenderSelect = () => screen.getByRole("combobox", { name: "防御側のポケモン" });

function savedInput(
  overrides: { attacker?: string; defender?: string; moveId?: string; itemId?: string } = {},
) {
  const attacker = {
    speciesKey: overrides.attacker ?? "9004-000",
    level: 50,
    natureId: "example-nature-spa",
    sp: { ...ZERO, spa: 32 },
    ...(overrides.itemId === undefined ? {} : { itemId: overrides.itemId }),
  };
  const input = {
    label: "保存",
    individual: attacker,
    calc: {
      format: "single",
      attacker,
      defender: { speciesKey: overrides.defender ?? "9005-000", level: 50, natureId: NEUTRAL_ID, sp: ZERO },
      moveId: overrides.moveId ?? "examplemovethunder",
    },
  } as unknown as FavoriteInput;
  saveLastCalcInput(input);
}

describe("L-1 保存", () => {
  test("攻撃側・防御側・技を選ぶと、直近の入力として保存される", async () => {
    const user = userEvent.setup();
    render(<CalcScreen engine={createFakeEngine()} master={master} />);
    expect(loadLastCalcInput()).toBeNull();

    await user.selectOptions(attackerSelect(), "9001-000");
    await user.selectOptions(defenderSelect(), "9002-000");

    await waitFor(() => {
      expect(loadLastCalcInput()?.calc?.defender.speciesKey).toBe("9002-000");
    });
    expect(loadLastCalcInput()?.calc?.attacker.speciesKey).toBe("9001-000");
    expect(loadLastCalcInput()?.individual.speciesKey).toBe("9001-000");
  });

  test("何も選んでいない間は保存しない", () => {
    render(<CalcScreen engine={createFakeEngine()} master={master} />);
    expect(window.localStorage.getItem(LAST_CALC_INPUT_STORAGE_KEY)).toBeNull();
  });
});

describe("L-2 復元", () => {
  test("再訪(mount)時に保存した入力が戻り、そのまま計算が走る。案内は出さない", async () => {
    savedInput({ itemId: "exampleitempower" });
    const engine = createFakeEngine();
    render(<CalcScreen engine={engine} master={master} />);

    await waitFor(() => {
      expect(attackerSelect()).toHaveValue("9004-000");
    });
    expect(defenderSelect()).toHaveValue("9005-000");
    expect(selectedMoveId()).toBe("examplemovethunder");
    expect(await screen.findByRole("list", { name: "計算結果" })).toBeInTheDocument();
    const bulk = engine.bulkRequests.at(-1);
    expect(bulk?.attacker.species.key).toBe("9004-000");
    expect(bulk?.attacker.item?.id).toBe("exampleitempower");
    expect(bulk?.attacker.sp).toEqual({ ...ZERO, spa: 32 });
    expect(screen.queryByRole("status")).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });
});

describe("L-2b 計算する場所の切り替え後は戻さない", () => {
  test("restoreLastInput=false の mount では、保存があっても戻さない(ページを開き直したときだけ戻す)", async () => {
    savedInput();
    render(<CalcScreen engine={createFakeEngine()} master={master} restoreLastInput={false} />);
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(attackerSelect()).toHaveValue("");
  });
});

describe("L-3 復元できない key は安全に捨てる", () => {
  test("マスタに無い種族: 落ちず、案内を出さず、入力は空のまま。保存は消さない", async () => {
    savedInput({ attacker: "9999-000", defender: "9998-000" });
    const before = window.localStorage.getItem(LAST_CALC_INPUT_STORAGE_KEY);
    render(<CalcScreen engine={createFakeEngine()} master={master} />);
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(attackerSelect()).toHaveValue("");
    expect(defenderSelect()).toHaveValue("");
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByRole("status")).toBeNull();
    expect(window.localStorage.getItem(LAST_CALC_INPUT_STORAGE_KEY)).toBe(before);
  });

  test("マスタに無い技・持ち物: 引けた種族は戻し、技と持ち物は戻さない", async () => {
    savedInput({ moveId: "examplemovegone", itemId: "exampleitemgone" });
    const engine = createFakeEngine();
    render(<CalcScreen engine={engine} master={master} />);
    await waitFor(() => {
      expect(attackerSelect()).toHaveValue("9004-000");
    });
    expect(defenderSelect()).toHaveValue("9005-000");
    expect(selectedMoveId()).not.toBe("examplemovegone");
    expect(screen.queryByRole("alert")).toBeNull();
  });
});

describe("L-4 お気に入りからの復元が優先", () => {
  test("restoreRequest がある mount では保存した入力を使わない", async () => {
    savedInput({ attacker: "9001-000", defender: "9002-000" });
    const favorite = {
      id: "1",
      label: "お気に入り",
      individual: { speciesKey: "9004-000", level: 50, natureId: "example-nature-spa", sp: ZERO },
      createdAt: "2026-10-04T01:00:00Z",
      updatedAt: "2026-10-04T01:00:00Z",
    } as components["schemas"]["Favorite"];
    render(
      <CalcScreen engine={createFakeEngine()} master={master} restoreRequest={{ token: 1, favorite }} />,
    );
    await waitFor(() => {
      expect(attackerSelect()).toHaveValue("9004-000");
    });
    expect(defenderSelect()).toHaveValue("");
  });
});

describe("L-5 壊れた値・使えないストレージ", () => {
  test("壊れた保存値は無視して空の画面で動く", () => {
    window.localStorage.setItem(LAST_CALC_INPUT_STORAGE_KEY, "{broken");
    render(<CalcScreen engine={createFakeEngine()} master={master} />);
    expect(attackerSelect()).toHaveValue("");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  test("ストレージが例外を投げても、選択・計算は動く", async () => {
    const throwing: StorageLike = {
      getItem() {
        throw new DOMException("denied", "SecurityError");
      },
      setItem() {
        throw new DOMException("denied", "QuotaExceededError");
      },
    };
    const user = userEvent.setup();
    render(<CalcScreen engine={createFakeEngine()} master={master} lastInputStorage={throwing} />);
    await user.selectOptions(attackerSelect(), "9001-000");
    await user.selectOptions(defenderSelect(), "9002-000");
    expect(await screen.findByRole("list", { name: "計算結果" })).toBeInTheDocument();
  });
});
