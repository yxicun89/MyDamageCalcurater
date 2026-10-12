// CB-6 段階 C2(ADR-0359 追記): 素早さ画面の「この相手で連続技を計算」→ 計算タブ。
// App の受け渡し(連続技を開く要求〈token + 攻撃側・防御側のポケモン ID〉。ScreenEnvironment 経由)を確かめる。
//   - 押すと計算タブ(URL /calc)に移り、攻撃側 = 自分・防御側 = 相手が入って、連続技の節が広がる
//   - 計算タブを開いて入力を変えたあとでも、もう一度押せば置き換わる
// 架空のポケモン ID(9001-000 など)だけを使う(ADR-0002)。

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { App } from "./App";
import { CALC_MODE_STORAGE_KEY } from "./app/calcMode";
import { speedScreenText } from "./i18n/ja";
import { exampleMasterSource } from "./master/exampleSource";
import { createFakeEngine } from "./test/fakeEngine";
import { pokemonListResponse } from "./test/fakeSpeedClient";
import { openTab, screenTab } from "./test/navTabs";

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

beforeEach(() => {
  window.history.replaceState(null, "", "/speed");
  window.localStorage.removeItem(CALC_MODE_STORAGE_KEY);
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>((input) => {
      const url = typeof input === "string" ? input : "";
      if (url.endsWith("/api/speed/v1/pokemon")) {
        return Promise.resolve(json(200, pokemonListResponse));
      }
      return Promise.resolve(json(404, { code: "not_found", message: "no" }));
    }),
  );
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  window.history.replaceState(null, "", "/");
  window.localStorage.removeItem(CALC_MODE_STORAGE_KEY);
});

async function chooseBoth(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  const self = await screen.findByRole("combobox", { name: speedScreenText.selfPokemonLabel });
  await within(self).findByRole("option", { name: /テストカソウドリ/ });
  await user.selectOptions(self, "9001-000");
  await user.selectOptions(
    screen.getByRole("combobox", { name: speedScreenText.opponentPokemonLabel }),
    "9002-000",
  );
}

test("「この相手で連続技を計算」で計算タブ(/calc)に移り、攻撃側・防御側が入って連続技の節が広がる", async () => {
  const user = userEvent.setup();
  render(<App engine={createFakeEngine()} masterSource={exampleMasterSource} />);
  await chooseBoth(user);
  await user.click(screen.getByRole("button", { name: speedScreenText.comboLinkLabel }));

  await waitFor(() => {
    expect(screenTab("計算")).toHaveAttribute("aria-selected", "true");
  });
  expect(window.location.pathname).toBe("/calc");
  await waitFor(() => {
    expect(screen.getByRole("combobox", { name: "攻撃側のポケモン" })).toHaveValue("9001-000");
  });
  expect(screen.getByRole("combobox", { name: "防御側のポケモン" })).toHaveValue("9002-000");
  const region = await screen.findByRole("region", { name: "連続技" });
  expect(await within(region).findByRole("combobox", { name: "追加の技" })).toBeInTheDocument();
});

test("計算タブで入力を変えたあとでも、もう一度押すと攻撃側・防御側が置き換わる", async () => {
  const user = userEvent.setup();
  render(<App engine={createFakeEngine()} masterSource={exampleMasterSource} />);
  await chooseBoth(user);
  await user.click(screen.getByRole("button", { name: speedScreenText.comboLinkLabel }));
  await waitFor(() => {
    expect(screen.getByRole("combobox", { name: "防御側のポケモン" })).toHaveValue("9002-000");
  });
  await user.selectOptions(screen.getByRole("combobox", { name: "防御側のポケモン" }), "9003-000");

  await openTab(user, "素早さ");
  await user.click(screen.getByRole("button", { name: speedScreenText.comboLinkLabel }));
  await waitFor(() => {
    expect(screen.getByRole("combobox", { name: "防御側のポケモン" })).toHaveValue("9002-000");
  });
});
