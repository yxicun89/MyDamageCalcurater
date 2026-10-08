// I-web-8 = F-09(ADR-0333 §3): App の組み込み。お気に入りタブの「計算に使う」→ 計算タブに切り替わり、入力が戻って
// 通常の calcBulk が走る(画面間の受け渡しは App の復元要求〈token + お気に入り〉。ScreenEnvironment 経由)。
// 確かめること:
//   - 計算タブを一度も開いていなくても(URL /favorites から始めても)、押すと計算タブ(URL /calc)に移り、入力が戻って結果が出る
//   - 計算タブを開いて入力を変えたあとでも(ADR-0308 で mount したまま)、押すと入力が置き換わる。同じお気に入りを2回使っても毎回戻る
//   - calc の無い旧お気に入りは攻撃側だけを戻す(防御側はそのまま)
// 架空の key だけを使う(ADR-0002)。

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { App } from "./App";
import { CALC_MODE_STORAGE_KEY } from "./app/calcMode";
import type { components } from "./api/openapi.gen";
import type { BulkRequest } from "./engine/types";
import { favoritesRestoreText, favoritesScreenText } from "./i18n/favorites";
import { exampleMasterSource } from "./master/exampleSource";
import { createFakeEngine, type FakeEngine } from "./test/fakeEngine";

type Favorite = components["schemas"]["Favorite"];

const ZERO = { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 } as const;
const NEUTRAL_ID = "example-nature-neutral-docile";
const SPA_SP = { hp: 0, atk: 0, def: 0, spa: 32, spd: 0, spe: 0 } as const;

const WITH_CALC: Favorite = {
  id: "51",
  label: "テストでんき→テストいわはがね(テストかみなり)",
  individual: { speciesKey: "9004-000", level: 50, natureId: "example-nature-spa", sp: SPA_SP },
  calc: {
    format: "single",
    attacker: {
      speciesKey: "9004-000",
      level: 50,
      natureId: "example-nature-spa",
      sp: SPA_SP,
      abilityId: "exampleabilityadapt",
      ranks: { atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
      status: "none",
    },
    defender: {
      speciesKey: "9005-000",
      level: 50,
      natureId: NEUTRAL_ID,
      sp: ZERO,
      ranks: { atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
      status: "none",
    },
    moveId: "examplemovethunder",
    field: {
      weather: "none",
      terrain: "none",
      attackerScreens: { reflect: false, lightScreen: false, auroraVeil: false },
      defenderScreens: { reflect: false, lightScreen: false, auroraVeil: false },
    },
    options: { critical: false },
  },
  createdAt: "2026-10-04T01:00:00Z",
  updatedAt: "2026-10-04T01:00:00Z",
};

const LEGACY: Favorite = {
  id: "50",
  label: "テストでんき",
  individual: { speciesKey: "9004-000", level: 50, natureId: "example-nature-spa", sp: SPA_SP },
  createdAt: "2026-10-03T01:00:00Z",
  updatedAt: "2026-10-03T01:00:00Z",
};

beforeEach(() => {
  window.history.replaceState(null, "", "/");
  window.localStorage.removeItem(CALC_MODE_STORAGE_KEY);
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  window.history.replaceState(null, "", "/");
  window.localStorage.removeItem(CALC_MODE_STORAGE_KEY);
});

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

/** record の fake(お気に入りの一覧だけ)。その他の record は空、record 以外は 404。 */
function installRecordBackend(favorites: Favorite[]) {
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>((input, init) => {
      const url = typeof input === "string" ? input : "";
      const method = init?.method ?? "GET";
      if (!url.includes("/api/record/")) {
        return Promise.resolve(json(404, { code: "not_found", message: "no" }));
      }
      if (url.endsWith("/api/record/favorites") && method === "GET") {
        return Promise.resolve(json(200, favorites));
      }
      return Promise.resolve(json(200, []));
    }),
  );
}

function renderApp(path: string): { user: ReturnType<typeof userEvent.setup>; engine: FakeEngine } {
  window.history.replaceState(null, "", path);
  const engine = createFakeEngine();
  render(<App engine={engine} masterSource={exampleMasterSource} />);
  return { user: userEvent.setup(), engine };
}

function lastBulk(engine: FakeEngine): BulkRequest | undefined {
  return engine.bulkRequests.at(-1);
}

const calcTab = () => screen.getByRole("tab", { name: "計算" });
const favoritesTab = () => screen.getByRole("tab", { name: favoritesScreenText.tabLabel });
const attackerSelect = () => screen.getByRole("combobox", { name: "攻撃側のポケモン" });
const defenderSelect = () => screen.getByRole("combobox", { name: "防御側のポケモン" });
const useButton = (favorite: Favorite) =>
  screen.findByRole("button", { name: favoritesScreenText.useLabel(favorite.label ?? "") });

test("URL /favorites から始めても、「計算に使う」で計算タブ(/calc)に移り、入力が戻って結果が出る", async () => {
  installRecordBackend([WITH_CALC]);
  const { user, engine } = renderApp("/favorites");
  await user.click(await useButton(WITH_CALC));

  await waitFor(() => {
    expect(calcTab()).toHaveAttribute("aria-selected", "true");
  });
  expect(window.location.pathname).toBe("/calc");
  expect(await screen.findByRole("list", { name: "計算結果" })).toBeVisible();
  expect(attackerSelect()).toHaveValue("9004-000");
  expect(defenderSelect()).toHaveValue("9005-000");
  expect(lastBulk(engine)?.attacker.species.key).toBe("9004-000");
  expect(lastBulk(engine)?.defenderSpecies.key).toBe("9005-000");
  expect(lastBulk(engine)?.move.id).toBe("examplemovethunder");
});

test("計算タブで入力を変えたあとでも、押すと入力が置き換わる。同じお気に入りを2回使っても毎回戻る", async () => {
  installRecordBackend([WITH_CALC]);
  const { user, engine } = renderApp("/calc");
  await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
  await user.selectOptions(attackerSelect(), "9001-000");
  await user.selectOptions(defenderSelect(), "9002-000");

  await user.click(favoritesTab());
  await user.click(await useButton(WITH_CALC));
  await waitFor(() => {
    expect(lastBulk(engine)?.attacker.species.key).toBe("9004-000");
  });
  expect(calcTab()).toHaveAttribute("aria-selected", "true");
  expect(defenderSelect()).toHaveValue("9005-000");

  await user.selectOptions(defenderSelect(), "9002-000");
  await waitFor(() => {
    expect(lastBulk(engine)?.defenderSpecies.key).toBe("9002-000");
  });
  await user.click(favoritesTab());
  await user.click(await useButton(WITH_CALC));
  await waitFor(() => {
    expect(lastBulk(engine)?.defenderSpecies.key).toBe("9005-000");
  });
  expect(defenderSelect()).toHaveValue("9005-000");
});

test("calc の無い旧お気に入りは攻撃側だけを戻し、防御側はそのまま", async () => {
  installRecordBackend([LEGACY]);
  const { user, engine } = renderApp("/calc");
  await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
  await user.selectOptions(attackerSelect(), "9001-000");
  await user.selectOptions(defenderSelect(), "9002-000");

  await user.click(favoritesTab());
  await user.click(await useButton(LEGACY));
  await waitFor(() => {
    expect(lastBulk(engine)?.attacker.species.key).toBe("9004-000");
  });
  expect(lastBulk(engine)?.defenderSpecies.key).toBe("9002-000");
  expect(attackerSelect()).toHaveValue("9004-000");
  expect(defenderSelect()).toHaveValue("9002-000");
});

test("「計算に使う」で計算タブへ移ると、フォーカスは復元の案内に移る(非表示のお気に入りタブに残らない)", async () => {
  installRecordBackend([WITH_CALC]);
  const { user } = renderApp("/favorites");
  await user.click(await useButton(WITH_CALC));
  const notice = await screen.findByText(favoritesRestoreText.restoredNotice(WITH_CALC.label ?? ""));
  await waitFor(() => {
    expect(document.activeElement).toBe(notice.closest("[role=status]"));
  });
});
