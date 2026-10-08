// P5-5d(issue #103 の Web 側。ADR-0318): 情報ページ(AboutScreen)の「データの扱い」節と「この端末のデータを削除」。
// recordClient / teamClient は fake(deleteDeviceData だけを持つ DeviceDataDeleter を props で注入。ADR-0318 §3)。
// 受け入れ条件(画面の振る舞い):
//   D1 節: 見出し「データの扱い」(h3)・説明4文(ADR-0209 §8 の3文 + 計算は影響されない旨)・ボタン「この端末のデータを削除」。
//      操作前は状態表示もダイアログも無く、1回も通信しない
//   D2 確認: ボタンでアプリ内ダイアログ(role=alertdialog・aria-modal。window.confirm は使わない)。名前「この端末のデータを削除」、
//      説明に確認文。初期フォーカスはキャンセル(破壊的な「削除する」にしない)。確認するまで通信しない
//   D3 キャンセル・Esc: ダイアログを閉じ、フォーカスを削除ボタンへ戻し、通信しない。Tab はダイアログ内で循環する
//   D4 実行: 確認後は「削除しています…」(role=status)。実行中はボタンを押せない(二重起動しない)。
//      record・team の両方が completed になってから「削除しました。」。partial の間は「まだ残っています。続けて削除します。」
//   D5 失敗: 失敗文言は role=alert。片方だけ消えたら「構築は削除済みです。」等を添える。completed の文言は出さない。
//      「もう一度削除する」は completed でない対象だけを再度呼び、再確認しない。成功で「削除しました。」
//   D6 上限: partial が続く対象は 20 回で止まり「まだ残っています。続けて削除します。」+ 再試行ボタン(無限ループしない)
//   D7 構築の再取得: team が completed になったら onTeamDataDeleted を呼ぶ(App が構築一覧を取り直す)。失敗した run では呼ばない
//   D8 ローカル状態: 端末 ID・計算モード等の localStorage は消さない・書き換えない(iOS と同じ。ADR-0318 §4)
//   D9 サーバーの message・code は画面に出さない(固定文言のみ)
// 文言はあえてリテラルで書く(i18n/ja.ts の deviceDataText の取り違えを検出するため)。

import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { DEVICE_ID_STORAGE_KEY } from "./api/clientIds";
import { AboutScreen } from "./AboutScreen";
import type { DeviceDataDeleter } from "./deviceData/deleteDeviceData";

const EXPLANATION = [
  "アカウントはありません。履歴・お気に入り・構築は、この端末に割り当てた番号でサーバーに保存しています。",
  "番号が変わると(ブラウザのサイトデータを消したとき)、前のデータは開けなくなります。元に戻す方法はありません。",
  "開けなくなったデータは自動的に消えます。計算の履歴は記録から90日、お気に入りと構築は最後に使った日から18か月です。",
  "削除するのはサーバーに保存したデータだけです。計算・逆算は、削除の成否にかかわらず使えます。",
] as const;
const CONFIRM = "履歴・お気に入り・構築をサーバーから削除します。元に戻せません。";
const FAILURE = "サーバーに届きませんでした。通信を確認してもう一度お試しください。";

type DeleteResult = Awaited<ReturnType<DeviceDataDeleter["deleteDeviceData"]>>;

function done(status: "completed" | "partial"): DeleteResult {
  return { ok: true, value: { status, purgedAt: "2026-10-02T01:00:00Z", deleted: {} as never } };
}
function failed(): DeleteResult {
  return { ok: false, error: { code: "upstream_unavailable", message: "SERVER-SIDE-MESSAGE" } };
}

interface Deferred {
  readonly resolve: (result: DeleteResult) => void;
}

/** 応答を手で返す fake(呼ばれた回数と、保留中の応答を持つ)。 */
function manual(): DeviceDataDeleter & { readonly pending: Deferred[]; readonly calls: () => number } {
  const pending: Deferred[] = [];
  const fn = vi.fn(
    () =>
      new Promise<DeleteResult>((resolve) => {
        pending.push({ resolve });
      }),
  );
  return { deleteDeviceData: fn, pending, calls: () => fn.mock.calls.length };
}

/** 呼ばれるたびに steps の次を即返す fake(使い切ったら最後を繰り返す)。 */
function scripted(steps: readonly DeleteResult[]): DeviceDataDeleter & { readonly calls: () => number } {
  const fn = vi.fn(() => {
    const step = steps[Math.min(fn.mock.calls.length - 1, steps.length - 1)];
    return Promise.resolve(step ?? done("completed"));
  });
  return { deleteDeviceData: fn, calls: () => fn.mock.calls.length };
}

function setup(record: DeviceDataDeleter, team: DeviceDataDeleter) {
  const onTeamDataDeleted = vi.fn();
  render(
    <AboutScreen
      backHref="/calc"
      onBack={() => undefined}
      focusOnMount={false}
      recordClient={record}
      teamClient={team}
      onTeamDataDeleted={onTeamDataDeleted}
    />,
  );
  return { onTeamDataDeleted, user: userEvent.setup() };
}

function deleteButton(): HTMLElement {
  return screen.getByRole("button", { name: "この端末のデータを削除" });
}
function dialog(): HTMLElement {
  return screen.getByRole("alertdialog", { name: "この端末のデータを削除" });
}
async function openDialog(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await user.click(deleteButton());
  dialog();
}
async function confirm(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await user.click(within(dialog()).getByRole("button", { name: "削除する" }));
}

beforeEach(() => {
  window.localStorage.clear();
});
afterEach(() => {
  vi.restoreAllMocks();
  window.localStorage.clear();
});

describe("D1 節の表示", () => {
  test("見出し(h3「データの扱い」)・説明4文・削除ボタン。操作前は状態表示もダイアログも無く、通信しない", () => {
    const record = scripted([done("completed")]);
    const team = scripted([done("completed")]);
    setup(record, team);
    expect(screen.getByRole("heading", { level: 3, name: "データの扱い" })).toBeInTheDocument();
    for (const sentence of EXPLANATION) {
      expect(screen.getByText(sentence)).toBeInTheDocument();
    }
    expect(deleteButton()).toBeEnabled();
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(screen.queryByRole("status")).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(record.calls() + team.calls()).toBe(0);
  });

  test("既存の非公式表示・データの出典・「計算に戻る」はそのまま残る", () => {
    setup(scripted([done("completed")]), scripted([done("completed")]));
    expect(screen.getByRole("heading", { level: 3, name: "非公式表示" })).toBeInTheDocument();
    expect(screen.getByRole("list", { name: "データの出典" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "計算に戻る" })).toBeInTheDocument();
  });
});

describe("D2 確認ダイアログ", () => {
  test("ボタンで alertdialog が開く。確認文が説明で、初期フォーカスはキャンセル。window.confirm は使わない。まだ通信しない", async () => {
    const confirmSpy = vi.spyOn(window, "confirm");
    const record = scripted([done("completed")]);
    const team = scripted([done("completed")]);
    const { user } = setup(record, team);
    await openDialog(user);
    const d = dialog();
    expect(d).toHaveAttribute("aria-modal", "true");
    expect(d).toHaveAccessibleDescription(CONFIRM);
    expect(within(d).getByRole("button", { name: "キャンセル" })).toHaveFocus();
    expect(within(d).getByRole("button", { name: "削除する" })).toBeInTheDocument();
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(record.calls() + team.calls()).toBe(0);
  });
});

describe("D3 キャンセル・Esc・フォーカス", () => {
  test("キャンセルで閉じ、フォーカスは削除ボタンへ戻り、通信しない", async () => {
    const record = scripted([done("completed")]);
    const team = scripted([done("completed")]);
    const { user } = setup(record, team);
    await openDialog(user);
    await user.click(within(dialog()).getByRole("button", { name: "キャンセル" }));
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(deleteButton()).toHaveFocus();
    expect(screen.queryByRole("status")).toBeNull();
    expect(record.calls() + team.calls()).toBe(0);
  });

  test("Esc でも同じ(閉じる・フォーカスを戻す・通信しない)", async () => {
    const record = scripted([done("completed")]);
    const team = scripted([done("completed")]);
    const { user } = setup(record, team);
    await openDialog(user);
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(deleteButton()).toHaveFocus();
    expect(record.calls() + team.calls()).toBe(0);
  });

  test("Tab / Shift+Tab はダイアログの中で循環する(キャンセル ⇄ 削除する)", async () => {
    const { user } = setup(scripted([done("completed")]), scripted([done("completed")]));
    await openDialog(user);
    const cancel = within(dialog()).getByRole("button", { name: "キャンセル" });
    const ok = within(dialog()).getByRole("button", { name: "削除する" });
    expect(cancel).toHaveFocus();
    await user.tab();
    expect(ok).toHaveFocus();
    await user.tab();
    expect(cancel).toHaveFocus();
    await user.tab({ shift: true });
    expect(ok).toHaveFocus();
  });
});

describe("D4 実行", () => {
  test("確認後は「削除しています…」(role=status)・ボタン無効。両方 completed になるまで「削除しました。」を出さない", async () => {
    const record = manual();
    const team = manual();
    const { user, onTeamDataDeleted } = setup(record, team);
    await openDialog(user);
    await confirm(user);

    expect(screen.queryByRole("alertdialog")).toBeNull();
    await waitFor(() => {
      expect(record.calls()).toBe(1);
    });
    expect(screen.getByRole("status")).toHaveTextContent("削除しています…");
    expect(deleteButton()).toBeDisabled();

    // record だけ完了しても、team が終わるまで完了を出さない。
    await act(async () => {
      record.pending[0]?.resolve(done("completed"));
      await Promise.resolve();
    });
    await waitFor(() => {
      expect(team.calls()).toBe(1);
    });
    expect(screen.queryByText("削除しました。")).toBeNull();
    expect(onTeamDataDeleted).not.toHaveBeenCalled();

    await act(async () => {
      team.pending[0]?.resolve(done("completed"));
      await Promise.resolve();
    });
    expect(await screen.findByText("削除しました。")).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("削除しました。");
    expect(deleteButton()).toBeEnabled();
    expect(record.calls()).toBe(1);
    expect(team.calls()).toBe(1);
  });

  test("実行中にボタンを押しても二重に起動しない(ダイアログも開かない)", async () => {
    const record = manual();
    const team = manual();
    const { user } = setup(record, team);
    await openDialog(user);
    await confirm(user);
    await waitFor(() => {
      expect(record.calls()).toBe(1);
    });
    await user.click(deleteButton());
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(record.calls()).toBe(1);
  });

  test("partial の間は同じ要求を繰り返し、「まだ残っています。続けて削除します。」を出し、completed で「削除しました。」", async () => {
    const record = scripted([done("partial"), done("partial"), done("completed")]);
    const team = scripted([done("completed")]);
    const { user } = setup(record, team);
    await openDialog(user);
    await confirm(user);
    expect(await screen.findByText("削除しました。")).toBeInTheDocument();
    expect(record.calls()).toBe(3);
    expect(team.calls()).toBe(1);
    expect(screen.queryByText("まだ残っています。続けて削除します。")).toBeNull();
  });

  test("途中で partial を受けた直後は「まだ残っています。続けて削除します。」が status に出る", async () => {
    const record = manual();
    const team = manual();
    const { user } = setup(record, team);
    await openDialog(user);
    await confirm(user);
    await waitFor(() => {
      expect(record.calls()).toBe(1);
    });
    await act(async () => {
      record.pending[0]?.resolve(done("partial"));
      await Promise.resolve();
    });
    await waitFor(() => {
      expect(record.calls()).toBe(2);
    });
    expect(screen.getByRole("status")).toHaveTextContent("まだ残っています。続けて削除します。");
  });
});

describe("D5 失敗と再試行", () => {
  test("record だけ失敗: role=alert に失敗文言 + 「構築は削除済みです。」。completed の文言は出さない。team は進む", async () => {
    const record = scripted([failed()]);
    const team = scripted([done("completed")]);
    const { user } = setup(record, team);
    await openDialog(user);
    await confirm(user);
    const alert = await screen.findByRole("alert");
    expect(within(alert).getByText(FAILURE)).toBeInTheDocument();
    expect(within(alert).getByText("構築は削除済みです。")).toBeInTheDocument();
    expect(screen.queryByText("削除しました。")).toBeNull();
    expect(record.calls()).toBe(1);
    expect(team.calls()).toBe(1);
    expect(screen.getByRole("button", { name: "もう一度削除する" })).toBeInTheDocument();
    expect(deleteButton()).toBeEnabled();
  });

  test("再試行は completed でない対象(record)だけを呼び、確認ダイアログは出さず、成功で「削除しました。」(alert は消える)", async () => {
    const record = scripted([failed(), done("completed")]);
    const team = scripted([done("completed")]);
    const { user } = setup(record, team);
    await openDialog(user);
    await confirm(user);
    await screen.findByRole("alert");

    await user.click(screen.getByRole("button", { name: "もう一度削除する" }));
    expect(await screen.findByText("削除しました。")).toBeInTheDocument();
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByRole("button", { name: "もう一度削除する" })).toBeNull();
    expect(record.calls()).toBe(2);
    expect(team.calls()).toBe(1);
  });

  test("team だけ失敗: 「履歴・お気に入りは削除済みです。」を添える。再試行は team だけ呼ぶ", async () => {
    const record = scripted([done("completed")]);
    const team = scripted([failed(), done("completed")]);
    const { user } = setup(record, team);
    await openDialog(user);
    await confirm(user);
    const alert = await screen.findByRole("alert");
    expect(within(alert).getByText("履歴・お気に入りは削除済みです。")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "もう一度削除する" }));
    expect(await screen.findByText("削除しました。")).toBeInTheDocument();
    expect(record.calls()).toBe(1);
    expect(team.calls()).toBe(2);
  });

  test("両方失敗: 失敗文言だけ(「削除済み」は出さない)。API 不達・オフラインでも黙って成功にしない", async () => {
    const { user } = setup(scripted([failed()]), scripted([failed()]));
    await openDialog(user);
    await confirm(user);
    const alert = await screen.findByRole("alert");
    expect(within(alert).getByText(FAILURE)).toBeInTheDocument();
    expect(screen.queryByText(/削除済みです/)).toBeNull();
    expect(screen.queryByText("削除しました。")).toBeNull();
  });

  test("D9 サーバーの message・code は画面に出さない", async () => {
    const { user } = setup(scripted([failed()]), scripted([failed()]));
    await openDialog(user);
    await confirm(user);
    await screen.findByRole("alert");
    expect(screen.queryByText(/SERVER-SIDE-MESSAGE/)).toBeNull();
    expect(screen.queryByText(/upstream_unavailable/)).toBeNull();
  });
});

describe("D6 上限", () => {
  test("partial が続き続ける対象は 20 回で止まり、「まだ残っています。続けて削除します。」+ 再試行ボタン。team は完了している", async () => {
    const record = scripted([done("partial")]);
    const team = scripted([done("completed")]);
    const { user } = setup(record, team);
    await openDialog(user);
    await confirm(user);
    expect(await screen.findByText("まだ残っています。続けて削除します。")).toBeInTheDocument();
    expect(record.calls()).toBe(20);
    expect(team.calls()).toBe(1);
    expect(screen.getByRole("button", { name: "もう一度削除する" })).toBeInTheDocument();
    expect(screen.queryByText("削除しました。")).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });
});

describe("D7 構築の再取得の合図", () => {
  test("両方成功で onTeamDataDeleted を1回呼ぶ", async () => {
    const { user, onTeamDataDeleted } = setup(scripted([done("completed")]), scripted([done("completed")]));
    await openDialog(user);
    await confirm(user);
    await screen.findByText("削除しました。");
    expect(onTeamDataDeleted).toHaveBeenCalledTimes(1);
  });

  test("record が失敗でも team が completed になったら呼ぶ(構築一覧は空になっているため)。再試行(record だけ)では呼ばない", async () => {
    const { user, onTeamDataDeleted } = setup(
      scripted([failed(), done("completed")]),
      scripted([done("completed")]),
    );
    await openDialog(user);
    await confirm(user);
    await screen.findByRole("alert");
    expect(onTeamDataDeleted).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole("button", { name: "もう一度削除する" }));
    await screen.findByText("削除しました。");
    expect(onTeamDataDeleted).toHaveBeenCalledTimes(1);
  });

  test("team が失敗したら呼ばない", async () => {
    const { user, onTeamDataDeleted } = setup(scripted([done("completed")]), scripted([failed()]));
    await openDialog(user);
    await confirm(user);
    await screen.findByRole("alert");
    expect(onTeamDataDeleted).not.toHaveBeenCalled();
  });
});

describe("D8 ローカル状態は消さない", () => {
  test("端末 ID・計算モード等の localStorage は削除の前後で同じ(端末 ID を作り直さない)", async () => {
    const deviceId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
    window.localStorage.setItem(DEVICE_ID_STORAGE_KEY, deviceId);
    window.localStorage.setItem("pokecalc.calcMode", "api");
    const before = JSON.stringify(Object.fromEntries(Object.entries(window.localStorage)));
    const removeSpy = vi.spyOn(Storage.prototype, "removeItem");
    const clearSpy = vi.spyOn(Storage.prototype, "clear");
    const { user } = setup(scripted([done("completed")]), scripted([done("completed")]));
    await openDialog(user);
    await confirm(user);
    await screen.findByText("削除しました。");
    expect(removeSpy).not.toHaveBeenCalled();
    expect(clearSpy).not.toHaveBeenCalled();
    expect(window.localStorage.getItem(DEVICE_ID_STORAGE_KEY)).toBe(deviceId);
    expect(JSON.stringify(Object.fromEntries(Object.entries(window.localStorage)))).toBe(before);
  });
});

describe("追加: 中断と二重起動(critic 推奨)", () => {
  test("同一フレームの2回押しでも削除は1回だけ走る", async () => {
    const record = manual();
    const team = manual();
    const { user } = setup(record, team);
    await openDialog(user);
    const ok = within(dialog()).getByRole("button", { name: "削除する" });
    await act(async () => {
      ok.click();
      ok.click();
      await Promise.resolve();
    });
    await waitFor(() => {
      expect(record.calls()).toBe(1);
    });
    expect(record.calls()).toBe(1);
  });

  test("team の応答が completed で届く前に画面を離れても、completed なら onTeamDataDeleted を呼ぶ", async () => {
    const record = scripted([done("completed")]);
    const team = manual();
    const onTeamDataDeleted = vi.fn();
    const user = userEvent.setup();
    const view = render(
      <AboutScreen
        backHref="/calc"
        onBack={() => undefined}
        focusOnMount={false}
        recordClient={record}
        teamClient={team}
        onTeamDataDeleted={onTeamDataDeleted}
      />,
    );
    await openDialog(user);
    await confirm(user);
    await waitFor(() => {
      expect(team.calls()).toBe(1);
    });
    view.unmount();
    await act(async () => {
      team.pending[0]?.resolve(done("completed"));
      await Promise.resolve();
    });
    await waitFor(() => {
      expect(onTeamDataDeleted).toHaveBeenCalledTimes(1);
    });
  });
});
