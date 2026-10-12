// I-web-32 の critic 指摘: 攻撃側の個体が素早さの SP・ランクを 0 以外で持っていても、既定は個体そのまま
// (連続技を開いただけで 0 に上書きしない)。表示は個体の値。入力を変えると上書きし、openSpeed なしの要求で既定に戻る。

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { Individual, Move } from "../engine/types";
import { comboScreenText } from "../i18n/combo";
import { buildIndividual } from "../domain/requests";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import { createComboEngine } from "../test/fakeCombo";
import { ComboSection, type OpenSpeed } from "./ComboSection";

const attackMove: Move = {
  id: "atk",
  nameJa: "テストこうげき",
  type: "normal",
  category: "physical",
  power: 80,
  priority: 0,
};
const danceMove: Move = {
  id: "dance",
  nameJa: "テストまい",
  type: "dragon",
  category: "status",
  power: 0,
  priority: 0,
  effect: { chance: 100, target: "self", stages: { atk: 1, spe: 1 } },
};

let master: MasterData;
let attacker: Individual;
let defender: Individual;

beforeAll(async () => {
  master = await exampleMasterSource.load();
  const [first, second] = master.species;
  if (first === undefined || second === undefined) {
    throw new Error("例データが足りない");
  }
  attacker = {
    ...buildIndividual(first, {
      sp: { hp: 0, atk: 20, def: 0, spa: 0, spd: 0, spe: 10 },
      nature: { plus: "", minus: "" },
      item: null,
      ability: { id: "", nameJa: "", effect: null },
      ranks: { atk: 0, def: 0, spa: 0, spd: 0, spe: 1 },
    }),
  };
  defender = buildIndividual(second, {
    sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
    nature: { plus: "", minus: "" },
    item: null,
    ability: { id: "", nameJa: "", effect: null },
  });
});

function renderSection(
  engine: ReturnType<typeof createComboEngine>,
  openSpeed: OpenSpeed | null,
  token: number,
) {
  return (
    <ComboSection
      engine={engine}
      typeChart={master.typeChart}
      types={master.typeChart.types}
      attacker={attacker}
      mainMove={attackMove}
      mainHits={null}
      mainCritical={false}
      onMainCriticalChange={() => undefined}
      extraCandidates={[danceMove]}
      opponentCandidates={[]}
      defenders={[{ key: "d", label: "相手", individual: defender }]}
      field={undefined}
      moves={[attackMove, danceMove]}
      items={[]}
      abilities={[]}
      openToken={token}
      openSpeed={openSpeed}
    />
  );
}

const group = () => screen.getByRole("group", { name: comboScreenText.selfSpeed.legend });

async function addDance(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  const region = screen.getByRole("region", { name: "連続技" });
  await user.click(within(region).getByRole("combobox", { name: "追加の技" }));
  const row = screen
    .getByRole("listbox", { name: "追加の技" })
    .querySelector('[role="option"][data-move-id="dance"]');
  if (row === null) {
    throw new Error("行が無い");
  }
  await user.click(row);
}

describe("既定は個体そのまま", () => {
  test("個体の素早さ SP 10・ランク +1 を要求にそのまま載せ、入力欄にも個体の値を出す", async () => {
    const engine = createComboEngine();
    const user = userEvent.setup();
    render(renderSection(engine, null, 1));
    await addDance(user);
    await waitFor(() => {
      expect(engine.comboRequests.length).toBeGreaterThan(0);
    });
    for (const request of engine.comboRequests) {
      expect(request.attacker.sp.spe).toBe(10);
      expect(request.attacker.ranks?.spe).toBe(1);
    }
    expect(within(group()).getByRole("spinbutton", { name: comboScreenText.selfSpeed.spLabel })).toHaveValue(
      10,
    );
    expect(within(group()).getByRole("slider")).toHaveAttribute("aria-valuenow", "1");
  });

  test("SP 入力は上限(66 − 他の SP = 46 → 32)に丸めて表示と要求を揃え、入力した値は個体より優先する", async () => {
    const engine = createComboEngine();
    const user = userEvent.setup();
    render(renderSection(engine, null, 1));
    await addDance(user);
    const sp = within(group()).getByRole("spinbutton", { name: comboScreenText.selfSpeed.spLabel });
    await user.clear(sp);
    await user.type(sp, "99");
    expect(sp).toHaveValue(32);
    await waitFor(() => {
      expect(engine.comboRequests.at(-1)?.attacker.sp.spe).toBe(32);
    });
  });

  test("speed を持つ要求で開いた後、speed なしの要求で開き直すと既定(個体の値・案内なし)に戻る", () => {
    const engine = createComboEngine();
    const { rerender } = render(renderSection(engine, { sp: 5, nature: "plus", rank: 2, scarf: true }, 1));
    expect(within(group()).getByRole("spinbutton", { name: comboScreenText.selfSpeed.spLabel })).toHaveValue(
      5,
    );
    expect(within(group()).getByText(comboScreenText.selfSpeed.scarfIgnored)).toBeInTheDocument();
    rerender(renderSection(engine, null, 2));
    expect(within(group()).getByRole("spinbutton", { name: comboScreenText.selfSpeed.spLabel })).toHaveValue(
      10,
    );
    expect(
      within(group()).getByRole("combobox", { name: comboScreenText.selfSpeed.natureLabel }),
    ).toHaveValue("keep");
    expect(within(group()).getByRole("slider")).toHaveAttribute("aria-valuenow", "1");
    expect(within(group()).queryByText(comboScreenText.selfSpeed.scarfIgnored)).toBeNull();
  });
});
