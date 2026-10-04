// P5-5d(ADR-0318 §6): App 全体での「この端末のデータを削除」。fetch を差し替えて、実際の recordClient / teamClient 経由で
// DELETE /api/record/device-data・DELETE /api/team/device-data が(ヘッダー付きで)呼ばれることと、
// 削除後に開いたままの構築一覧が空へ取り直されることを確かめる。
//   - 情報ページ(/about)にだけ「データの扱い」節がある(計算・逆算などのタブには出さない)
//   - 削除は両方の DELETE を呼び、完了後に「削除しました。」
//   - 構築タブを先に開いて一覧を読み込んでおいても、削除後に戻ると一覧は空(古い構築が残らない。サーバー側は fake が空にする)
//   - 削除しても計算は影響されない: 通信が全て失敗しても、計算画面は使える(絶対ルール5)
//   - 端末 ID は削除の前後で同じ(再生成しない)
// 文言はあえてリテラル。jsdom の window.history を使う(App.about.test.tsx と同じ流儀)。

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { App } from "./App";
import { DEVICE_ID_STORAGE_KEY } from "./api/clientIds";
import { createFakeEngine } from "./test/fakeEngine";

const DEVICE_ID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

interface Server {
  teams: object[];
  readonly calls: { method: string; path: string; headers: Record<string, string> }[];
}

function installServer(options: { down?: boolean } = {}): Server {
  const server: Server = {
    teams: [
      {
        id: "33333333-3333-4333-8333-333333333333",
        name: "削除前の構築",
        members: [],
        createdAt: "2026-09-26T01:00:00Z",
        updatedAt: "2026-09-26T02:00:00Z",
      },
    ],
    calls: [],
  };
  vi.stubGlobal(
    "fetch",
    vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = new URL(
        typeof input === "string" ? input : input instanceof URL ? input.href : input.url,
        "http://localhost/",
      );
      const method = init?.method ?? "GET";
      server.calls.push({
        method,
        path: url.pathname,
        headers: Object.fromEntries(new Headers(init?.headers).entries()),
      });
      if (options.down === true) {
        return Promise.reject(new TypeError("Failed to fetch"));
      }
      if (method === "GET" && url.pathname === "/api/team/teams") {
        return Promise.resolve(jsonResponse(200, server.teams));
      }
      if (method === "DELETE" && url.pathname === "/api/record/device-data") {
        return Promise.resolve(
          jsonResponse(200, {
            status: "completed",
            purgedAt: "2026-10-02T01:00:00Z",
            deleted: { calcEvents: 0, aggregates: 0, favorites: 0 },
          }),
        );
      }
      if (method === "DELETE" && url.pathname === "/api/team/device-data") {
        server.teams = [];
        return Promise.resolve(
          jsonResponse(200, {
            status: "completed",
            purgedAt: "2026-10-02T01:00:00Z",
            deleted: { teams: 1, teamMembers: 0 },
          }),
        );
      }
      return Promise.resolve(jsonResponse(404, { code: "not_found", message: "none" }));
    }),
  );
  return server;
}

function setPath(path: string): void {
  window.history.replaceState(null, "", path);
}

async function deleteAllFromAbout(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await user.click(within(screen.getByRole("contentinfo")).getByRole("link", { name: "このアプリについて" }));
  await user.click(await screen.findByRole("button", { name: "この端末のデータを削除" }));
  await user.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: "削除する" }));
}

beforeEach(() => {
  window.localStorage.clear();
  window.localStorage.setItem(DEVICE_ID_STORAGE_KEY, DEVICE_ID);
  setPath("/");
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  window.localStorage.clear();
  setPath("/");
});

describe("情報ページの「データの扱い」", () => {
  test("/about にだけある(計算画面には出さない)", async () => {
    installServer();
    setPath("/about");
    render(<App engine={createFakeEngine()} />);
    expect(await screen.findByRole("heading", { level: 3, name: "データの扱い" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "この端末のデータを削除" })).toBeInTheDocument();
  });

  test("計算画面に削除ボタンは無い", async () => {
    installServer();
    setPath("/calc");
    render(<App engine={createFakeEngine()} />);
    await screen.findByRole("tablist", { name: "画面の切り替え" });
    expect(screen.queryByRole("button", { name: "この端末のデータを削除" })).toBeNull();
  });
});

describe("削除の実行", () => {
  test("record と team の両方の DELETE を、端末 ID・セッション ID 付きで呼び、「削除しました。」。端末 ID は変わらない", async () => {
    const server = installServer();
    setPath("/about");
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await user.click(await screen.findByRole("button", { name: "この端末のデータを削除" }));
    await user.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: "削除する" }));
    expect(await screen.findByText("削除しました。")).toBeInTheDocument();

    const deletes = server.calls.filter((call) => call.method === "DELETE");
    expect(deletes.map((call) => call.path).sort()).toEqual([
      "/api/record/device-data",
      "/api/team/device-data",
    ]);
    for (const call of deletes) {
      expect(call.headers["x-device-id"]).toBe(DEVICE_ID);
      expect(call.headers["x-session-id"]).toMatch(/^[0-9a-f-]{36}$/);
    }
    expect(window.localStorage.getItem(DEVICE_ID_STORAGE_KEY)).toBe(DEVICE_ID);
  });

  test("構築タブで一覧を読み込んでおいても、削除後に戻ると一覧は空(古い構築を残さない)", async () => {
    const server = installServer();
    setPath("/team");
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    expect(await screen.findByText("削除前の構築")).toBeInTheDocument();
    const listCallsBefore = server.calls.filter((call) => call.method === "GET").length;

    await deleteAllFromAbout(user);
    await screen.findByText("削除しました。");
    await user.click(screen.getByRole("link", { name: "計算に戻る" }));
    await user.click(screen.getByRole("tab", { name: "構築" }));

    await waitFor(() => {
      expect(screen.queryByText("削除前の構築")).toBeNull();
    });
    expect(
      await screen.findByText(
        "まだ構築がありません。「新しい構築」を押すと、ポケモンを6体まで選んで構築を作れます",
      ),
    ).toBeInTheDocument();
    expect(server.calls.filter((call) => call.method === "GET").length).toBeGreaterThan(listCallsBefore);
  });
});

describe("API に届かないとき", () => {
  test("通信が全て失敗しても黙って成功にせず、失敗文言を出す。その後も計算画面は使える(絶対ルール5)", async () => {
    installServer({ down: true });
    setPath("/about");
    const user = userEvent.setup();
    render(<App engine={createFakeEngine()} />);
    await user.click(await screen.findByRole("button", { name: "この端末のデータを削除" }));
    await user.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: "削除する" }));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("サーバーに届きませんでした。通信を確認してもう一度お試しください。");
    expect(screen.queryByText("削除しました。")).toBeNull();
    expect(window.localStorage.getItem(DEVICE_ID_STORAGE_KEY)).toBe(DEVICE_ID);

    await user.click(screen.getByRole("link", { name: "計算に戻る" }));
    expect(await screen.findByRole("combobox", { name: "攻撃側のポケモン" })).toBeInTheDocument();
  });
});
