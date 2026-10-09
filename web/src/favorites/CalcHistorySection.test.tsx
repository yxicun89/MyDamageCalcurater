// Web: 計算履歴の一覧(ADR-0230「Web・iOS レーンへの依頼」・ADR-0338)。お気に入りの画面の第2の節 CalcHistorySection。
// RecordClient は fake(props で注入)。確かめること(受け入れ条件 AC-2〜AC-5):
//   AC-2 一覧: マウント時に listCalcHistory(undefined, AbortSignal) を1回だけ / 読み込み中・空・失敗・一覧の4状態 /
//        region「計算履歴」・list「計算履歴の一覧」/ 行は攻撃側・防御側の speciesKey・moveId・「min〜max%」・日時(生の ISO 文字列を出さない)/
//        サーバーの順のまま / 同じ内容の行が2つ並んでも両方出る(キーは配列の位置)
//   AC-3 続き: nextCursor が null でない間だけ「もっと見る」/ 押すと nextCursor を渡して次を末尾に足す / 連打で二重に呼ばない /
//        最後のページで消える / 続きの失敗は節の中の alert に出し、読み込み済みの行と「もっと見る」は残す(押し直せる)
//        400 は先頭から読み直す(cursor なしで呼び直し、一覧は新しい先頭ページに置き換わる。読み直しも 400 なら alert で止まり、繰り返さない)
//   AC-4 失敗の閉じ込め: 503 等は節の中だけの role=alert(見出し+サーバー message)。ポーリングしない(時間が経っても再取得しない)
//        recordClient が無い(オフライン)ときは何も出さず、API も呼ばない
//   AC-5 取り直し・後始末: reloadToken が変わったら先頭から取り直す(古い応答で上書きしない・古い取得は abort)/
//        アンマウント後に setState しない(console.error なし)・取得は abort する
//   AC-6 計算に使う: onUse を渡したときだけ各行に「この計算を使う」/ 押すと onUse に Favorite 形(calc は行の calc と同じ値)を1回渡す
// 架空の key だけを使う(ADR-0002)。

import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, test, vi } from "vitest";
import type { components } from "../api/openapi.gen";
import { calcHistoryText } from "../i18n/favorites";
import type { RecordClient, RecordResult } from "../record/recordClient";
import { CalcHistorySection } from "./CalcHistorySection";

type Schemas = components["schemas"];
type Entry = Schemas["CalcHistoryEntry"];
type Page = Schemas["CalcHistoryPage"];

const SP = { hp: 0, atk: 32, def: 0, spa: 0, spd: 0, spe: 0 } as const;

function entryOf(attacker: string, defender: string, moveId: string, min: number, max: number): Entry {
  return {
    occurredAt: "2026-10-09T03:00:00Z",
    calc: {
      format: "single",
      attacker: { speciesKey: attacker, level: 50, natureId: "fake-nature", sp: SP },
      defender: { speciesKey: defender, level: 50, natureId: "fake-nature", sp: SP },
      moveId,
    },
    result: { minPercent: min, maxPercent: max },
  };
}

const E1 = entryOf("9001-000", "9002-000", "fake-move-a", 41.2, 48.9);
const E2 = entryOf("9003-000", "9004-000", "fake-move-b", 10, 12.5);
const E3 = entryOf("9005-000", "9006-000", "fake-move-c", 100, 118.4);

interface Pending {
  readonly cursor: string | undefined;
  readonly signal: AbortSignal | undefined;
  resolve(result: RecordResult<Page>): void;
}

interface FakeRecord extends RecordClient {
  readonly calls: Pending[];
}

function createFakeRecord(): FakeRecord {
  const calls: Pending[] = [];
  const unused = () => Promise.reject(new Error("このテストでは使わない"));
  return {
    calls,
    listFrequentOpponents: unused,
    deleteDeviceData: unused,
    createFavorite: unused,
    listFavorites: unused,
    deleteFavorite: unused,
    listCalcHistory(cursor, signal) {
      return new Promise((resolve) => {
        calls.push({ cursor, signal, resolve });
      });
    },
  };
}

async function settle(resolve: () => void): Promise<void> {
  await act(async () => {
    resolve();
    await Promise.resolve();
  });
}

function last(record: FakeRecord): Pending {
  const call = record.calls.at(-1);
  if (call === undefined) {
    throw new Error("listCalcHistory が呼ばれていない");
  }
  return call;
}

const ok = (items: Entry[], nextCursor: string | null): RecordResult<Page> => ({
  ok: true,
  value: { items, nextCursor },
});
const fail = (code: string, message: string): RecordResult<Page> => ({
  ok: false,
  error: { code, message },
});

async function renderLoaded(
  items: Entry[],
  nextCursor: string | null,
  extra: { onUse?: (favorite: Schemas["Favorite"]) => void } = {},
) {
  const record = createFakeRecord();
  const view = render(<CalcHistorySection recordClient={record} {...extra} />);
  await settle(() => {
    last(record).resolve(ok(items, nextCursor));
  });
  return { record, view };
}

const rows = () =>
  within(screen.getByRole("list", { name: calcHistoryText.listLabel })).getAllByRole("listitem");
const moreButton = () => screen.queryByRole("button", { name: "もっと見る" });

afterEach(() => {
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe("AC-2 一覧", () => {
  test("マウント時に listCalcHistory を cursor なし・AbortSignal 付きで1回だけ呼び、読み込み中を出す", () => {
    const record = createFakeRecord();
    render(<CalcHistorySection recordClient={record} />);
    expect(record.calls).toHaveLength(1);
    expect(record.calls[0]?.cursor).toBeUndefined();
    expect(record.calls[0]?.signal).toBeInstanceOf(AbortSignal);
    expect(screen.getByRole("region", { name: "計算履歴" })).toBeInTheDocument();
    expect(screen.getByText(calcHistoryText.loadingNotice)).toBeInTheDocument();
    expect(screen.queryByRole("list", { name: calcHistoryText.listLabel })).toBeNull();
  });

  test("行に攻撃側・防御側の key・技・ダメージの幅・日時を出し、サーバーの順のまま並べる", async () => {
    await renderLoaded([E1, E2, E3], null);
    const texts = rows().map((row) => row.textContent);
    expect(texts).toHaveLength(3);
    expect(texts[0]).toContain("9001-000");
    expect(texts[0]).toContain("9002-000");
    expect(texts[0]).toContain("fake-move-a");
    expect(texts[0]).toContain("41.2〜48.9%");
    expect(texts[1]).toContain("9003-000");
    expect(texts[1]).toContain("10〜12.5%");
    expect(texts[2]).toContain("9005-000");
    expect(texts[2]).toContain("100〜118.4%");
    expect(screen.queryByText(calcHistoryText.loadingNotice)).toBeNull();
  });

  test("日時は読みやすい形にする(生の ISO 文字列をそのまま出さない)", async () => {
    await renderLoaded([E1], null);
    const text = rows()[0]?.textContent ?? "";
    expect(text).toContain("2026");
    expect(text).not.toContain("2026-10-09T03:00:00Z");
    expect(text).not.toContain("T03:00");
  });

  test("まったく同じ行が2つ並んでも両方出る(キーは配列の位置)。重複キーの警告も出ない", async () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => undefined);
    await renderLoaded([E1, E1], null);
    expect(rows()).toHaveLength(2);
    expect(errorSpy).not.toHaveBeenCalled();
  });

  test("0件なら空の案内(list を出さない・alert にしない・「もっと見る」なし)", async () => {
    await renderLoaded([], null);
    expect(screen.getByText(calcHistoryText.emptyNotice)).toBeInTheDocument();
    expect(screen.queryByRole("list", { name: calcHistoryText.listLabel })).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(moreButton()).toBeNull();
  });
});

describe("AC-3 続き(もっと見る)", () => {
  test("nextCursor が null なら「もっと見る」を出さない", async () => {
    await renderLoaded([E1], null);
    expect(moreButton()).toBeNull();
  });

  test("nextCursor があれば出し、押すとその値を加工せず渡して、次のページを末尾に足す。最後で消える", async () => {
    const user = userEvent.setup();
    const { record } = await renderLoaded([E1, E2], "cursor-2");
    expect(moreButton()).toBeInTheDocument();

    await user.click(moreButton() as HTMLElement);
    expect(record.calls).toHaveLength(2);
    expect(record.calls[1]?.cursor).toBe("cursor-2");
    expect(record.calls[1]?.signal).toBeInstanceOf(AbortSignal);
    // 読み込み済みの行は残る
    expect(rows()).toHaveLength(2);

    await settle(() => {
      last(record).resolve(ok([E3], "cursor-3"));
    });
    expect(rows().map((row) => row.textContent)).toEqual([
      expect.stringContaining("9001-000"),
      expect.stringContaining("9003-000"),
      expect.stringContaining("9005-000"),
    ]);
    expect(moreButton()).toBeInTheDocument();

    await user.click(moreButton() as HTMLElement);
    expect(record.calls[2]?.cursor).toBe("cursor-3");
    await settle(() => {
      last(record).resolve(ok([E1], null));
    });
    expect(rows()).toHaveLength(4);
    expect(moreButton()).toBeNull();
  });

  test("応答待ちの間は二重に呼ばない(連打しても1回)", async () => {
    const user = userEvent.setup();
    const { record } = await renderLoaded([E1], "cursor-2");
    const button = moreButton() as HTMLElement;
    await user.click(button);
    await user.click(button);
    expect(record.calls).toHaveLength(2);
  });

  test("続きの失敗は節の中の alert に出し、読み込み済みの行と「もっと見る」は残る。押し直せる", async () => {
    const user = userEvent.setup();
    const { record } = await renderLoaded([E1, E2], "cursor-2");
    await user.click(moreButton() as HTMLElement);
    await settle(() => {
      last(record).resolve(fail("store_unavailable", "記録を読めません"));
    });
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("記録を読めません");
    expect(within(screen.getByRole("region", { name: "計算履歴" })).getByRole("alert")).toBe(alert);
    expect(rows()).toHaveLength(2);

    await user.click(moreButton() as HTMLElement);
    expect(record.calls).toHaveLength(3);
    expect(record.calls[2]?.cursor).toBe("cursor-2");
    await settle(() => {
      last(record).resolve(ok([E3], null));
    });
    expect(rows()).toHaveLength(3);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  test("続きが 400 なら先頭から読み直す(cursor なし)。一覧は新しい先頭ページに置き換わる", async () => {
    const user = userEvent.setup();
    const { record } = await renderLoaded([E1, E2], "stale-cursor");
    await user.click(moreButton() as HTMLElement);
    await settle(() => {
      last(record).resolve(fail("invalid_input", "カーソルが不正です"));
    });
    expect(record.calls).toHaveLength(3);
    expect(record.calls[2]?.cursor).toBeUndefined();
    await settle(() => {
      last(record).resolve(ok([E3], null));
    });
    expect(rows().map((row) => row.textContent)).toEqual([expect.stringContaining("9005-000")]);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  test("読み直しも 400 なら alert で止まる(無限に繰り返さない)", async () => {
    const user = userEvent.setup();
    const { record } = await renderLoaded([E1], "stale-cursor");
    await user.click(moreButton() as HTMLElement);
    await settle(() => {
      last(record).resolve(fail("invalid_input", "x"));
    });
    await settle(() => {
      last(record).resolve(fail("invalid_input", "カーソルが不正です"));
    });
    expect(record.calls).toHaveLength(3);
    expect(screen.getByRole("alert")).toHaveTextContent("カーソルが不正です");
  });
});

describe("AC-4 失敗の閉じ込め・オフライン・ポーリングなし", () => {
  test("最初の取得の失敗は節の中の alert(見出し+サーバーの message)。一覧は出さない", async () => {
    const record = createFakeRecord();
    render(<CalcHistorySection recordClient={record} />);
    await settle(() => {
      last(record).resolve(fail("upstream_unavailable", "記録サービスに届きません"));
    });
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(calcHistoryText.errorHeading);
    expect(alert).toHaveTextContent("記録サービスに届きません");
    expect(within(screen.getByRole("region", { name: "計算履歴" })).getByRole("alert")).toBe(alert);
    expect(screen.queryByRole("list", { name: calcHistoryText.listLabel })).toBeNull();
    expect(moreButton()).toBeNull();
  });

  test("ポーリングしない(時間が経っても再取得しない)", async () => {
    vi.useFakeTimers();
    const record = createFakeRecord();
    render(<CalcHistorySection recordClient={record} />);
    await settle(() => {
      last(record).resolve(ok([E1], null));
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10 * 60 * 1000);
    });
    expect(record.calls).toHaveLength(1);
  });

  test("recordClient が無い(オフライン)ときは何も出さず、何も呼ばない", () => {
    const { container } = render(<CalcHistorySection />);
    expect(container).toBeEmptyDOMElement();
  });
});

describe("AC-5 取り直し・後始末", () => {
  test("reloadToken が変わったら先頭(cursor なし)から取り直し、古い取得は abort。一覧は置き換わる", async () => {
    const record = createFakeRecord();
    const view = render(<CalcHistorySection recordClient={record} reloadToken={0} />);
    const first = last(record);
    await settle(() => {
      first.resolve(ok([E1, E2], "cursor-2"));
    });
    expect(rows()).toHaveLength(2);

    view.rerender(<CalcHistorySection recordClient={record} reloadToken={1} />);
    expect(record.calls).toHaveLength(2);
    expect(record.calls[1]?.cursor).toBeUndefined();
    expect(screen.getByText(calcHistoryText.loadingNotice)).toBeInTheDocument();
    expect(screen.queryByRole("list", { name: calcHistoryText.listLabel })).toBeNull();
    await settle(() => {
      last(record).resolve(ok([], null));
    });
    expect(screen.getByText(calcHistoryText.emptyNotice)).toBeInTheDocument();
    expect(moreButton()).toBeNull();
  });

  test("古い応答が後から届いても上書きしない", async () => {
    const record = createFakeRecord();
    const view = render(<CalcHistorySection recordClient={record} reloadToken={0} />);
    const stale = last(record);
    view.rerender(<CalcHistorySection recordClient={record} reloadToken={1} />);
    expect(stale.signal?.aborted).toBe(true);
    await settle(() => {
      last(record).resolve(ok([E2], null));
    });
    await settle(() => {
      stale.resolve(ok([E1], null));
    });
    expect(rows()).toHaveLength(1);
    expect(rows()[0]?.textContent).toContain("9003-000");
  });

  test("取り直しの前の「もっと見る」の応答が後から届いても足さない", async () => {
    const user = userEvent.setup();
    const record = createFakeRecord();
    const view = render(<CalcHistorySection recordClient={record} reloadToken={0} />);
    await settle(() => {
      last(record).resolve(ok([E1], "cursor-2"));
    });
    await user.click(moreButton() as HTMLElement);
    const staleMore = last(record);
    view.rerender(<CalcHistorySection recordClient={record} reloadToken={1} />);
    expect(staleMore.signal?.aborted).toBe(true);
    await settle(() => {
      last(record).resolve(ok([E2], null));
    });
    await settle(() => {
      staleMore.resolve(ok([E3], null));
    });
    expect(rows()).toHaveLength(1);
    expect(rows()[0]?.textContent).toContain("9003-000");
  });

  test("アンマウントで取得を abort し、後から応答が届いても警告(console.error)を出さない", async () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const record = createFakeRecord();
    const view = render(<CalcHistorySection recordClient={record} />);
    const pending = last(record);
    view.unmount();
    expect(pending.signal?.aborted).toBe(true);
    await settle(() => {
      pending.resolve(ok([E1], null));
    });
    expect(errorSpy).not.toHaveBeenCalled();
  });
});

describe("AC-6 計算に使う", () => {
  test("onUse を渡さないときはボタンを出さない", async () => {
    await renderLoaded([E1], null);
    expect(screen.queryByRole("button", { name: calcHistoryText.useLabel })).toBeNull();
  });

  test("onUse を渡すと各行にボタンを出し、押した行の calc を Favorite 形で1回渡す。一覧は取り直さない", async () => {
    const user = userEvent.setup();
    const onUse = vi.fn<(favorite: Schemas["Favorite"]) => void>();
    const { record } = await renderLoaded([E1, E2], null, { onUse });
    const second = rows()[1] as HTMLElement;
    await user.click(within(second).getByRole("button", { name: calcHistoryText.useLabel }));

    expect(onUse).toHaveBeenCalledTimes(1);
    const favorite = onUse.mock.calls[0]?.[0];
    expect(favorite?.calc).toEqual(E2.calc);
    expect(favorite?.individual).toEqual(E2.calc.attacker);
    expect(favorite?.label).toContain("9003-000");
    expect(favorite?.label).toContain("9004-000");
    expect(record.calls).toHaveLength(1);
  });
});
