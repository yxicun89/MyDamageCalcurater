// P8-1c(ADR-0325): App のポケモン画像。manifest はアプリ起動時に1回だけ同一オリジンの /images/manifest.json から取る。
// 404・不正でも画面は従来どおり(エラー表示・alert を出さない = AC-X)。

import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { App } from "./App";
import { CALC_MODE_STORAGE_KEY } from "./app/calcMode";
import { createFakeEngine } from "./test/fakeEngine";

beforeEach(() => {
  window.history.replaceState(null, "", "/");
  window.localStorage.setItem(CALC_MODE_STORAGE_KEY, "offline");
});
afterEach(() => {
  window.localStorage.removeItem(CALC_MODE_STORAGE_KEY);
});

function fakeFetch(status: number, body: string): ReturnType<typeof vi.fn<typeof fetch>> {
  return vi.fn<typeof fetch>(() => Promise.resolve(new Response(body, { status })));
}

test("manifest が 404 でも alert を出さず画面が出る。取得は1回だけ・URL は /images/manifest.json", async () => {
  const imageFetch = fakeFetch(404, '{"error":"not_found"}');
  render(<App engine={createFakeEngine()} imageFetch={imageFetch} />);
  expect(await screen.findByRole("tablist", { name: "画面の切り替え" })).toBeInTheDocument();
  await waitFor(() => {
    expect(imageFetch).toHaveBeenCalledTimes(1);
  });
  expect(imageFetch.mock.calls[0]?.[0]).toBe("/images/manifest.json");
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});

test("画面を切り替えても manifest は取り直さない", async () => {
  const imageFetch = fakeFetch(404, "");
  render(<App engine={createFakeEngine()} imageFetch={imageFetch} />);
  const tablist = await screen.findByRole("tablist", { name: "画面の切り替え" });
  const tabs = tablist.querySelectorAll<HTMLElement>('[role="tab"]');
  for (const tab of Array.from(tabs)) {
    tab.click();
  }
  await waitFor(() => {
    expect(imageFetch).toHaveBeenCalledTimes(1);
  });
});

test("manifest が不正な JSON・HTML でも alert を出さない", async () => {
  const imageFetch = fakeFetch(200, "<!doctype html><html></html>");
  render(<App engine={createFakeEngine()} imageFetch={imageFetch} />);
  await screen.findByRole("tablist", { name: "画面の切り替え" });
  await waitFor(() => {
    expect(imageFetch).toHaveBeenCalledTimes(1);
  });
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});

test("fetch が例外でも alert を出さない", async () => {
  const imageFetch = vi.fn<typeof fetch>(() => Promise.reject(new TypeError("Failed to fetch")));
  render(<App engine={createFakeEngine()} imageFetch={imageFetch} />);
  await screen.findByRole("tablist", { name: "画面の切り替え" });
  await waitFor(() => {
    expect(imageFetch).toHaveBeenCalledTimes(1);
  });
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});
