// P5-3c(ADR-0327): App の組み込み。お気に入りタブ(登録ファイル favorites/favorites.screen.tsx)と、
// 計算画面での追加 → お気に入りタブの一覧に出る流れ。
// 確かめること(受け入れ条件 AC-8):
//   - タブ「お気に入り」が「構築」の後ろ(order 650)に出る。URL /favorites で開ける
//   - オフライン(選択中)では /api/record/ に一切触れず、タブは案内(role=status)を出す。計算画面にボタンも出ない
//   - オンラインで計算画面から追加すると POST /api/record/favorites(端末 ID・セッション ID 付き)を呼び、
//     お気に入りタブの一覧に出る。タブを先に開いて空だった場合も、追加後に開き直すと(ADR-0308 の保持で古いまま残らず)新しい一覧が出る
//   - 一覧の削除(2段階)で DELETE が呼ばれ、行が消える
//   - record が 503 でも、他のタブ(計算)は壊れない
// 架空の key だけを使う(ADR-0002)。

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { App } from "./App";
import { CALC_MODE_STORAGE_KEY } from "./app/calcMode";
import type { components } from "./api/openapi.gen";
import { favoritesCalcText, favoritesScreenText } from "./i18n/favorites";
import { exampleMasterSource } from "./master/exampleSource";
import { createFakeEngine } from "./test/fakeEngine";

type Favorite = components["schemas"]["Favorite"];

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

/** record の fake(お気に入りだけ)。その他の URL は 404 にする。 */
function installRecordBackend(options: { readonly down?: boolean } = {}) {
  const store: Favorite[] = [];
  const calls: { method: string; url: string; headers: Headers; body: unknown }[] = [];
  const fetchMock = vi.fn<typeof fetch>((input, init) => {
    const url = typeof input === "string" ? input : "";
    const method = init?.method ?? "GET";
    if (!url.includes("/api/record/")) {
      return Promise.resolve(json(404, { code: "not_found", message: "no" }));
    }
    calls.push({
      method,
      url,
      headers: new Headers(init?.headers),
      body: typeof init?.body === "string" ? JSON.parse(init.body) : undefined,
    });
    if (options.down === true) {
      return Promise.resolve(json(503, { code: "upstream_unavailable", message: "届きません" }));
    }
    if (url.endsWith("/api/record/favorites") && method === "GET") {
      return Promise.resolve(json(200, store));
    }
    if (url.endsWith("/api/record/favorites") && method === "POST") {
      const body = JSON.parse(init?.body as string) as { label?: string; individual: Favorite["individual"] };
      const created: Favorite = {
        id: String(store.length + 1),
        label: body.label ?? null,
        individual: body.individual,
        createdAt: "2026-10-04T01:00:00Z",
        updatedAt: "2026-10-04T01:00:00Z",
      };
      store.unshift(created);
      return Promise.resolve(json(201, created));
    }
    const match = /\/api\/record\/favorites\/([^/]+)$/.exec(url);
    if (match !== null && method === "DELETE") {
      const index = store.findIndex((favorite) => favorite.id === match[1]);
      if (index < 0) {
        return Promise.resolve(json(404, { code: "not_found", message: "no" }));
      }
      store.splice(index, 1);
      return Promise.resolve(new Response(null, { status: 204 }));
    }
    // frequent-opponents など
    return Promise.resolve(json(200, []));
  });
  vi.stubGlobal("fetch", fetchMock);
  return { store, calls };
}

async function renderApp() {
  const user = userEvent.setup();
  render(<App engine={createFakeEngine()} masterSource={exampleMasterSource} />);
  await screen.findByRole("combobox", { name: "攻撃側のポケモン" });
  return user;
}

const favoritesTab = () => screen.getByRole("tab", { name: favoritesScreenText.tabLabel });

async function addFromCalc(user: ReturnType<typeof userEvent.setup>) {
  const master = await exampleMasterSource.load();
  const species = master.species[1];
  if (species === undefined) {
    throw new Error("例データに種族が無い");
  }
  await user.selectOptions(screen.getByRole("combobox", { name: "攻撃側のポケモン" }), species.key);
  await user.click(await screen.findByRole("button", { name: favoritesCalcText.addLabel }));
  await screen.findByText(favoritesCalcText.addedNotice);
  return species;
}

test("タブ「お気に入り」は「構築」の後ろに出る", async () => {
  installRecordBackend();
  await renderApp();
  const names = screen.getAllByRole("tab").map((tab) => tab.textContent);
  const team = names.indexOf("構築");
  expect(team).toBeGreaterThanOrEqual(0);
  expect(names[team + 1]).toBe(favoritesScreenText.tabLabel);
});

test("URL /favorites でお気に入りタブが開く", async () => {
  installRecordBackend();
  window.history.replaceState(null, "", "/favorites");
  render(<App engine={createFakeEngine()} masterSource={exampleMasterSource} />);
  expect(await screen.findByRole("region", { name: favoritesScreenText.regionLabel })).toBeVisible();
  expect(favoritesTab()).toHaveAttribute("aria-selected", "true");
});

test("オフライン(選択中)では /api/record に触れず、案内(status)を出し、計算画面にボタンも出ない", async () => {
  window.localStorage.setItem(CALC_MODE_STORAGE_KEY, "offline");
  const backend = installRecordBackend();
  const user = await renderApp();
  expect(screen.queryByRole("button", { name: favoritesCalcText.addLabel })).toBeNull();
  await user.click(favoritesTab());
  expect(await screen.findByText(favoritesScreenText.offlineNotice)).toBeVisible();
  expect(backend.calls).toHaveLength(0);
});

test("オンライン: 計算画面で追加 → お気に入りタブの一覧に出る(端末 ID・セッション ID 付き)→ 削除(2段階)で消える", async () => {
  const backend = installRecordBackend();
  const user = await renderApp();
  const species = await addFromCalc(user);

  const post = backend.calls.find((call) => call.method === "POST");
  expect(post?.headers.get("X-Device-Id")).toMatch(/^[0-9a-f-]{36}$/);
  expect(post?.headers.get("X-Session-Id")).toMatch(/^[0-9a-f-]{36}$/);

  await user.click(favoritesTab());
  const list = await screen.findByRole("list", { name: favoritesScreenText.listLabel });
  expect(within(list).getAllByRole("listitem")).toHaveLength(1);
  expect(list).toHaveTextContent(species.nameJa);

  await user.click(screen.getByRole("button", { name: favoritesScreenText.deleteLabel(species.nameJa) }));
  await user.click(
    screen.getByRole("button", { name: favoritesScreenText.deleteConfirmLabel(species.nameJa) }),
  );
  expect(await screen.findByText(favoritesScreenText.emptyNotice)).toBeInTheDocument();
  expect(backend.calls.some((call) => call.method === "DELETE" && call.url.endsWith("/favorites/1"))).toBe(
    true,
  );
});

test("お気に入りタブを先に開いて空でも、計算画面で追加してから開き直すと新しい一覧が出る(古いまま残らない)", async () => {
  installRecordBackend();
  const user = await renderApp();
  await user.click(favoritesTab());
  expect(await screen.findByText(favoritesScreenText.emptyNotice)).toBeInTheDocument();

  await user.click(screen.getByRole("tab", { name: "計算" }));
  await addFromCalc(user);
  await user.click(favoritesTab());
  await waitFor(() => {
    expect(screen.getByRole("list", { name: favoritesScreenText.listLabel })).toBeVisible();
  });
  expect(screen.queryByText(favoritesScreenText.emptyNotice)).toBeNull();
});

test("record が 503 でも、計算画面は壊れず、お気に入りタブだけが alert を出す", async () => {
  installRecordBackend({ down: true });
  const user = await renderApp();
  expect(screen.queryByRole("alert")).toBeNull();
  await user.click(favoritesTab());
  const alert = await screen.findByRole("alert");
  expect(alert).toHaveTextContent(favoritesScreenText.listErrorHeading);
  await user.click(screen.getByRole("tab", { name: "計算" }));
  expect(screen.getByRole("combobox", { name: "攻撃側のポケモン" })).toBeVisible();
});
