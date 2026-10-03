// P5-5c: App の組み込み(ADR-0317 §2)。「よく計算する相手」は計算モードがオンラインのときだけ取得・表示する。
// 確かめること(受け入れ条件 AC-7):
//   - オフライン(既定)では /api/record に一切触れず、チップも出ない(offline.spec.ts の前提)
//   - オンラインに切り替えると、/api/record/frequent-opponents を端末 ID・セッション ID 付きで1回呼び、チップが出る
//   - オンラインでも record が 503 なら、チップも alert も出ない(計算画面は普通に使える)

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { App } from "./App";
import { CALC_MODE_STORAGE_KEY } from "./app/calcMode";
import { exampleMasterSource } from "./master/exampleSource";
import { createFakeEngine } from "./test/fakeEngine";

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

function urlText(input: unknown): string {
  return typeof input === "string" ? input : "";
}

function recordCalls(fetchMock: ReturnType<typeof vi.fn<typeof fetch>>) {
  return fetchMock.mock.calls.filter(([url]) => urlText(url).includes("/api/record/"));
}

test("オフライン(選択中)では record に触れず、チップも出ない", async () => {
  // ADR-0313: 既定の計算モードはオンラインになったので、オフラインを保存済みにして始める。
  window.localStorage.setItem(CALC_MODE_STORAGE_KEY, "offline");
  const fetchMock = vi.fn<typeof fetch>(() => Promise.resolve(new Response("[]", { status: 200 })));
  vi.stubGlobal("fetch", fetchMock);
  render(<App engine={createFakeEngine()} masterSource={exampleMasterSource} />);
  await screen.findByRole("combobox", { name: "攻撃側のポケモン" });

  expect(recordCalls(fetchMock)).toHaveLength(0);
  expect(screen.queryByRole("group", { name: "よく計算する相手" })).toBeNull();
});

test("オンラインに切り替えると取得してチップを出す(端末 ID・セッション ID 付き)", async () => {
  const user = userEvent.setup();
  const master = await exampleMasterSource.load();
  const target = master.species[1];
  if (target === undefined) {
    throw new Error("例データに種族が無い");
  }
  const fetchMock = vi.fn<typeof fetch>(() =>
    Promise.resolve(
      new Response(
        JSON.stringify([
          { speciesKey: target.key, score: 2, count: 2, lastCalculatedAt: "2026-10-01T00:00:00Z" },
        ]),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    ),
  );
  vi.stubGlobal("fetch", fetchMock);
  render(<App engine={createFakeEngine()} masterSource={exampleMasterSource} />);
  await screen.findByRole("combobox", { name: "攻撃側のポケモン" });

  await user.click(screen.getByText("オンライン(API)"));

  const group = await screen.findByRole("group", { name: "よく計算する相手" });
  expect(group).toHaveTextContent(target.nameJa);
  const calls = recordCalls(fetchMock);
  expect(calls).toHaveLength(1);
  const headers = new Headers(calls[0]?.[1]?.headers);
  expect(headers.get("X-Device-Id")).toMatch(/^[0-9a-f-]{36}$/);
  expect(headers.get("X-Session-Id")).toMatch(/^[0-9a-f-]{36}$/);
});

test("オンラインでも record が 503 なら、チップも alert も出ない", async () => {
  const user = userEvent.setup();
  const fetchMock = vi.fn<typeof fetch>(() =>
    Promise.resolve(
      new Response(JSON.stringify({ code: "upstream_unavailable", message: "届きません" }), {
        status: 503,
        headers: { "Content-Type": "application/json" },
      }),
    ),
  );
  vi.stubGlobal("fetch", fetchMock);
  render(<App engine={createFakeEngine()} masterSource={exampleMasterSource} />);
  await screen.findByRole("combobox", { name: "攻撃側のポケモン" });

  await user.click(screen.getByText("オンライン(API)"));

  await waitFor(() => {
    expect(recordCalls(fetchMock)).toHaveLength(1);
  });
  await new Promise((resolve) => setTimeout(resolve, 0));
  expect(screen.queryByRole("group", { name: "よく計算する相手" })).toBeNull();
  expect(screen.queryByRole("alert")).toBeNull();
});
