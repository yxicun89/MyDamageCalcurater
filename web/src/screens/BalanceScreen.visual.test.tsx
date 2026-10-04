// F-12 PR-2(I-web-12、ADR-0336 §2): タイプバランス画面への共通部品の適用。
// メンバー・仮想敵の枠(fieldset)は ui-card ui-card--typed(--card-type は選んだ種族の最初のタイプ)、
// 追加・削除ボタンは ui-button、計算中・エラーは ui-notice、結果の表は ui-table(ゼブラ)。
// 既存のクラス(balance-*)・アクセシブルな名前(group「メンバーn」・table の aria-label)・role・テキストは変えない。

import { act, render, screen, within } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../api/balance.gen";
import type { BalanceClient } from "../api/balanceClient";
import { balanceScreenText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData, MasterSpecies } from "../master/types";
import { BalanceScreen } from "./BalanceScreen";

type Schemas = components["schemas"];

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

const never = () => new Promise<never>(() => undefined);

function createClient(overrides: Partial<BalanceClient> = {}): BalanceClient {
  return { analyze: never, coverage: never, threats: never, recommendations: never, ...overrides };
}

function renderScreen(client: BalanceClient = createClient()): UserEvent {
  const user = userEvent.setup();
  render(<BalanceScreen master={master} client={client} />);
  return user;
}

const memberGroup = (n: number) => screen.getByRole("group", { name: balanceScreenText.memberGroupLabel(n) });

const typeVar = (typeId: string | undefined) => `var(--type-${typeId ?? ""}, var(--brand-primary))`;

async function selectSpecies(user: UserEvent, n: number, species: MasterSpecies): Promise<void> {
  await user.selectOptions(within(memberGroup(n)).getByRole("combobox", { name: "ポケモン" }), species.key);
}

function firstSpecies(): MasterSpecies {
  const species = master.species[0];
  if (species === undefined) {
    throw new Error("例データに種族が無い");
  }
  return species;
}

describe("メンバーの枠", () => {
  test("メンバー枠(group)は ui-card ui-card--typed(既存の balance-member も残す)。未選択なら --card-type なし", () => {
    renderScreen();
    expect(memberGroup(1)).toHaveClass("ui-card", "ui-card--typed", "balance-member");
    expect(memberGroup(1).style.getPropertyValue("--card-type")).toBe("");
  });

  test("種族を選ぶと、その枠の --card-type が最初のタイプの色になる", async () => {
    const user = renderScreen();
    const species = firstSpecies();
    await selectSpecies(user, 1, species);
    expect(memberGroup(1).style.getPropertyValue("--card-type")).toBe(typeVar(species.types[0]));
  });

  test("仮想敵の枠も ui-card ui-card--typed でタイプ色になる", async () => {
    const user = renderScreen();
    await user.click(screen.getByRole("button", { name: balanceScreenText.addThreatLabel }));
    const group = screen.getByRole("group", { name: "仮想敵1" });
    expect(group).toHaveClass("ui-card", "ui-card--typed");
    const species = firstSpecies();
    await user.selectOptions(within(group).getByRole("combobox", { name: "ポケモン" }), species.key);
    expect(group.style.getPropertyValue("--card-type")).toBe(typeVar(species.types[0]));
  });
});

describe("ボタン", () => {
  test("「メンバーを追加」「仮想敵を追加」は ui-button ui-button--secondary", () => {
    renderScreen();
    for (const name of [balanceScreenText.addMemberLabel, balanceScreenText.addThreatLabel]) {
      expect(screen.getByRole("button", { name })).toHaveClass("ui-button", "ui-button--secondary");
    }
  });

  test("メンバーの削除ボタンは ui-button ui-button--secondary(balance-member__remove も残す)", async () => {
    const user = renderScreen();
    await user.click(screen.getByRole("button", { name: balanceScreenText.addMemberLabel }));
    expect(screen.getByRole("button", { name: balanceScreenText.removeMemberLabel(2) })).toHaveClass(
      "ui-button",
      "ui-button--secondary",
      "balance-member__remove",
    );
  });
});

describe("案内・結果の表", () => {
  test("計算中の案内は ui-notice ui-notice--loading", async () => {
    const user = renderScreen();
    await selectSpecies(user, 1, firstSpecies());
    const loading = screen.getByText(balanceScreenText.loadingNotice);
    expect(loading).toHaveClass("ui-notice", "ui-notice--loading");
  });

  test("失敗は role=alert のまま ui-notice ui-notice--error", async () => {
    const user = renderScreen(
      createClient({
        analyze: () => Promise.resolve({ ok: false, error: { code: "internal", message: "x" } }),
      }),
    );
    await selectSpecies(user, 1, firstSpecies());
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveClass("ui-notice", "ui-notice--error");
  });

  test("防御相性・チームの集計の表は ui-table(aria-label はそのまま)", async () => {
    const response: Schemas["AnalyzeResponse"] = {
      members: [
        {
          pokemonId: firstSpecies().key,
          types: ["normal"],
          defense: [
            { attackType: "fire", multiplier: "1", category: "neutral", source: "type", effect: "none" },
          ],
        },
      ],
      teamSummary: [{ attackType: "fire", weak: 0, quadWeak: 0, resist: 0, immune: 0, neutral: 1 }],
    };
    const user = renderScreen(
      createClient({ analyze: () => Promise.resolve({ ok: true, value: response }) }),
    );
    await act(async () => {
      await selectSpecies(user, 1, firstSpecies());
    });
    for (const name of [balanceScreenText.defenseTableLabel, balanceScreenText.teamSummaryTableLabel]) {
      expect(await screen.findByRole("table", { name })).toHaveClass("ui-table");
    }
  });
});
