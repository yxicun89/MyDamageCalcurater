// P5-5d(ADR-0318 §6): 「この端末のデータを削除」の後、開いたままの構築一覧を古いまま残さない。
// TeamScreen は optional の props `reloadToken`(数値)を受け取り、**マウント後に値が変わったら** list() を呼び直す。
//   - 値が変わるまでは list() はマウント時の1回だけ(既存の AC-2 を保つ)
//   - 変わったら一覧を読み込み直す(その間、古い一覧を出し続けない = 読み込み中表示)。応答が空配列なら空の案内になる
//   - 取り直しの応答が古い list() の応答より先に着いても、古い応答で上書きしない(最新の呼び出しの応答だけ採る)
//   - 取り直しが失敗したら、古い一覧を残さずエラー表示(既存の失敗表示)
// 文言は teamScreenText(i18n/ja.ts)をそのまま参照する(ここは再取得の振る舞いの検証)。

import { act, render, screen } from "@testing-library/react";
import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../api/openapi.gen";
import { teamScreenText } from "../i18n/ja";
import { exampleMasterSource } from "../master/exampleSource";
import type { MasterData } from "../master/types";
import { TeamScreen } from "./TeamScreen";
import type { TeamClient, TeamResult } from "./teamClient";

type Team = components["schemas"]["Team"];

// PR-A2 から TeamScreen は master を受け取る(再取得の検証では使わない)。
let master: MasterData;
beforeAll(async () => {
  master = await exampleMasterSource.load();
});

const team: Team = {
  id: "33333333-3333-4333-8333-333333333333",
  name: "削除前の構築",
  members: [],
  createdAt: "2026-09-26T01:00:00Z",
  updatedAt: "2026-09-26T02:00:00Z",
};

function fakeClient() {
  const pending: ((result: TeamResult<Team[]>) => void)[] = [];
  const client = {
    list: () =>
      new Promise<TeamResult<Team[]>>((resolve) => {
        pending.push(resolve);
      }),
  } as unknown as TeamClient;
  return { client, pending };
}

async function settle(resolve: ((r: TeamResult<Team[]>) => void) | undefined, result: TeamResult<Team[]>) {
  await act(async () => {
    resolve?.(result);
    await Promise.resolve();
  });
}

describe("reloadToken", () => {
  test("値が変わらない再描画では list() を呼び直さない", async () => {
    const { client, pending } = fakeClient();
    const { rerender } = render(<TeamScreen teamClient={client} master={master} reloadToken={0} />);
    await settle(pending[0], { ok: true, value: [team] });
    rerender(<TeamScreen teamClient={client} master={master} reloadToken={0} />);
    expect(pending).toHaveLength(1);
  });

  test("値が変わると list() を呼び直し、空配列なら空の案内になる(古い構築は消える)", async () => {
    const { client, pending } = fakeClient();
    const { rerender } = render(<TeamScreen teamClient={client} master={master} reloadToken={0} />);
    await settle(pending[0], { ok: true, value: [team] });
    expect(screen.getByText("削除前の構築")).toBeInTheDocument();

    rerender(<TeamScreen teamClient={client} master={master} reloadToken={1} />);
    expect(pending).toHaveLength(2);
    expect(screen.queryByText("削除前の構築")).toBeNull();
    expect(screen.getByText(teamScreenText.loadingNotice)).toBeInTheDocument();

    await settle(pending[1], { ok: true, value: [] });
    expect(screen.getByText(teamScreenText.emptyNotice)).toBeInTheDocument();
  });

  test("古い list() の応答が後から着いても、最新の応答を上書きしない", async () => {
    const { client, pending } = fakeClient();
    const { rerender } = render(<TeamScreen teamClient={client} master={master} reloadToken={0} />);
    rerender(<TeamScreen teamClient={client} master={master} reloadToken={1} />);
    expect(pending).toHaveLength(2);
    await settle(pending[1], { ok: true, value: [] });
    await settle(pending[0], { ok: true, value: [team] });
    expect(screen.queryByText("削除前の構築")).toBeNull();
    expect(screen.getByText(teamScreenText.emptyNotice)).toBeInTheDocument();
  });

  test("取り直しが失敗したら、古い一覧を残さず失敗表示(role=alert)", async () => {
    const { client, pending } = fakeClient();
    const { rerender } = render(<TeamScreen teamClient={client} master={master} reloadToken={0} />);
    await settle(pending[0], { ok: true, value: [team] });
    rerender(<TeamScreen teamClient={client} master={master} reloadToken={1} />);
    await settle(pending[1], { ok: false, error: { code: "team_unavailable", message: "つながりません" } });
    expect(screen.queryByText("削除前の構築")).toBeNull();
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });
});
