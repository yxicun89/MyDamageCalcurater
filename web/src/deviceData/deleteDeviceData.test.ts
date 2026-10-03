// P5-5d: 「この端末のデータを削除」の手順を担う純粋な関数(ADR-0318 §2。iOS の DeviceDataDeletionViewModel と同じ規則)。
// UI を持たず、record / team のクライアント(deleteDeviceData() だけを持つ DeviceDataDeleter)を注入して確かめる。
// 規則(ADR-0209 §5・§8、DECISIONS.md 2026-10-01「P6-7」):
//   - record と team は独立に呼ぶ。片方が失敗・未完了でももう片方は進める
//   - partial は同じ要求を completed まで繰り返す。1対象あたりの上限は DEVICE_DATA_MAX_REQUESTS_PER_TARGET(20)回で、
//     超えたら incomplete(失敗ではない。無限ループしない)
//   - 通信エラー等は自動で再送せず failed(code)。サーバーの message は結果に持たない(表示は固定文言)
//   - targets を渡すと、その対象だけを呼ぶ(再試行)。previous の completed は引き継ぐ
//   - signal が中断されたら以後の要求を送らず、対象は pending のまま(失敗にしない)
//   - describeDeviceDataDeletion が結果を画面に出す文言(ADR-0209 §8。完全一致)に直す。失敗に completed は混ぜない
// 文言はあえてリテラルで書く(i18n/ja.ts の取り違えを検出するため)。

import { describe, expect, test, vi } from "vitest";
import {
  DEVICE_DATA_MAX_REQUESTS_PER_TARGET,
  describeDeviceDataDeletion,
  runDeviceDataDeletion,
  type DeviceDataDeleter,
  type DeviceDataDeletionResult,
} from "./deleteDeviceData";

type Step = "completed" | "partial" | "fail" | "throw";

/** 呼ばれるたびに steps の次を返す fake。steps を使い切ったら最後の値を繰り返す。 */
function scripted(steps: readonly Step[]): DeviceDataDeleter & { readonly calls: () => number } {
  let count = 0;
  const fn = vi.fn((): ReturnType<DeviceDataDeleter["deleteDeviceData"]> => {
    const step = steps[Math.min(count, steps.length - 1)] ?? "completed";
    count += 1;
    if (step === "throw") {
      return Promise.reject(new Error("契約違反: deleteDeviceData は reject しない"));
    }
    if (step === "fail") {
      return Promise.resolve({
        ok: false,
        error: { code: "upstream_unavailable", message: "サーバー側の文" },
      });
    }
    return Promise.resolve({
      ok: true,
      value: { status: step, purgedAt: "2026-10-02T01:00:00Z", deleted: {} as never },
    });
  });
  return { deleteDeviceData: fn, calls: () => fn.mock.calls.length };
}

describe("runDeviceDataDeletion", () => {
  test("上限は 20 回", () => {
    expect(DEVICE_DATA_MAX_REQUESTS_PER_TARGET).toBe(20);
  });

  test("両方がすぐ completed なら、それぞれ1回だけ呼ぶ", async () => {
    const record = scripted(["completed"]);
    const team = scripted(["completed"]);
    const result = await runDeviceDataDeletion({ record, team });
    expect(result).toEqual({ record: { kind: "completed" }, team: { kind: "completed" } });
    expect(record.calls()).toBe(1);
    expect(team.calls()).toBe(1);
  });

  test("partial は completed まで繰り返す(record 3回・team 1回)", async () => {
    const record = scripted(["partial", "partial", "completed"]);
    const team = scripted(["completed"]);
    const result = await runDeviceDataDeletion({ record, team });
    expect(result.record).toEqual({ kind: "completed" });
    expect(record.calls()).toBe(3);
    expect(team.calls()).toBe(1);
  });

  test("partial が続き続けても対象ごとに 20 回で止まり incomplete。もう片方は実行される", async () => {
    const record = scripted(["partial"]);
    const team = scripted(["completed"]);
    const result = await runDeviceDataDeletion({ record, team });
    expect(record.calls()).toBe(20);
    expect(result.record).toEqual({ kind: "incomplete" });
    expect(result.team).toEqual({ kind: "completed" });
  });

  test("maxRequestsPerTarget で上限を変えられる", async () => {
    const record = scripted(["partial"]);
    const team = scripted(["partial"]);
    await runDeviceDataDeletion({ record, team, maxRequestsPerTarget: 3 });
    expect(record.calls()).toBe(3);
    expect(team.calls()).toBe(3);
  });

  test("record の失敗でも team を呼び、team の失敗でも record を呼ぶ。失敗は自動再送しない(failed に code を運ぶ)", async () => {
    const recordFail = scripted(["fail"]);
    const teamOk = scripted(["completed"]);
    const a = await runDeviceDataDeletion({ record: recordFail, team: teamOk });
    expect(a.record).toEqual({ kind: "failed", code: "upstream_unavailable" });
    expect(a.team).toEqual({ kind: "completed" });
    expect(recordFail.calls()).toBe(1);
    expect(teamOk.calls()).toBe(1);

    const recordOk = scripted(["completed"]);
    const teamFail = scripted(["fail"]);
    const b = await runDeviceDataDeletion({ record: recordOk, team: teamFail });
    expect(b.record).toEqual({ kind: "completed" });
    expect(b.team).toEqual({ kind: "failed", code: "upstream_unavailable" });
    expect(teamFail.calls()).toBe(1);
  });

  test("partial の途中で失敗したら、そこで止めて failed(それ以上呼ばない)", async () => {
    const record = scripted(["partial", "fail"]);
    const team = scripted(["completed"]);
    const result = await runDeviceDataDeletion({ record, team });
    expect(result.record).toEqual({ kind: "failed", code: "upstream_unavailable" });
    expect(record.calls()).toBe(2);
  });

  test("クライアントが想定外に reject しても、例外を投げず failed(unexpected_error)にする", async () => {
    const record = scripted(["throw"]);
    const team = scripted(["completed"]);
    const result = await runDeviceDataDeletion({ record, team });
    expect(result.record).toEqual({ kind: "failed", code: "unexpected_error" });
    expect(result.team).toEqual({ kind: "completed" });
  });

  test("targets を渡すとその対象だけ呼ぶ。previous の completed は引き継ぐ(再試行)", async () => {
    const record = scripted(["completed"]);
    const team = scripted(["completed"]);
    const previous: DeviceDataDeletionResult = {
      record: { kind: "failed", code: "upstream_unavailable" },
      team: { kind: "completed" },
    };
    const result = await runDeviceDataDeletion({ record, team, targets: ["record"], previous });
    expect(record.calls()).toBe(1);
    expect(team.calls()).toBe(0);
    expect(result).toEqual({ record: { kind: "completed" }, team: { kind: "completed" } });
  });

  test("再試行でも上限は数え直す(前回 20 回 incomplete → 今回また最大 20 回)", async () => {
    const record = scripted(["partial"]);
    const team = scripted(["completed"]);
    await runDeviceDataDeletion({ record, team, targets: ["record"] });
    expect(record.calls()).toBe(20);
  });

  test("onProgress: partial を受けた直後に lastResponsePartial が true で通知される", async () => {
    const record = scripted(["partial", "completed"]);
    const team = scripted(["completed"]);
    const seen: boolean[] = [];
    await runDeviceDataDeletion({
      record,
      team,
      onProgress: (progress) => {
        seen.push(progress.lastResponsePartial);
      },
    });
    expect(seen).toContain(true);
    expect(seen.at(-1)).toBe(false);
  });

  test("signal が中断済みなら1回も呼ばず、両方 pending(失敗にしない)", async () => {
    const record = scripted(["completed"]);
    const team = scripted(["completed"]);
    const controller = new AbortController();
    controller.abort();
    const result = await runDeviceDataDeletion({ record, team, signal: controller.signal });
    expect(record.calls()).toBe(0);
    expect(team.calls()).toBe(0);
    expect(result).toEqual({ record: { kind: "pending" }, team: { kind: "pending" } });
  });

  test("partial の途中で中断したら、以後送らず pending のまま(完了済みの対象は残る)", async () => {
    const controller = new AbortController();
    const record: DeviceDataDeleter = {
      deleteDeviceData: vi.fn(() => {
        controller.abort();
        return Promise.resolve({
          ok: true as const,
          value: { status: "partial" as const, purgedAt: "2026-10-02T01:00:00Z", deleted: {} as never },
        });
      }),
    };
    const team = scripted(["completed"]);
    const result = await runDeviceDataDeletion({ record, team, signal: controller.signal });
    expect(record.deleteDeviceData).toHaveBeenCalledTimes(1);
    expect(result.record).toEqual({ kind: "pending" });
    expect(team.calls()).toBe(0);
  });
});

const FAILURE = "サーバーに届きませんでした。通信を確認してもう一度お試しください。";

describe("describeDeviceDataDeletion(ADR-0209 §8 の文言。完全一致)", () => {
  test("両方 completed のときだけ「削除しました。」(それだけ)", () => {
    expect(
      describeDeviceDataDeletion({ record: { kind: "completed" }, team: { kind: "completed" } }),
    ).toEqual({
      tone: "success",
      lines: ["削除しました。"],
    });
  });

  test("失敗なしで incomplete を含むときは「まだ残っています。続けて削除します。」(completed は出さない)", () => {
    expect(
      describeDeviceDataDeletion({ record: { kind: "incomplete" }, team: { kind: "completed" } }),
    ).toEqual({
      tone: "partial",
      lines: ["まだ残っています。続けて削除します。"],
    });
  });

  test("record だけ失敗: 失敗文言 + 「構築は削除済みです。」", () => {
    expect(
      describeDeviceDataDeletion({ record: { kind: "failed", code: "x" }, team: { kind: "completed" } }),
    ).toEqual({ tone: "failure", lines: [FAILURE, "構築は削除済みです。"] });
  });

  test("team だけ失敗: 失敗文言 + 「履歴・お気に入りは削除済みです。」", () => {
    expect(
      describeDeviceDataDeletion({ record: { kind: "completed" }, team: { kind: "failed", code: "x" } }),
    ).toEqual({ tone: "failure", lines: [FAILURE, "履歴・お気に入りは削除済みです。"] });
  });

  test("両方失敗: 失敗文言だけ。失敗と incomplete の組み合わせは失敗を優先し、completed の側だけ添える", () => {
    expect(
      describeDeviceDataDeletion({
        record: { kind: "failed", code: "x" },
        team: { kind: "failed", code: "y" },
      }),
    ).toEqual({ tone: "failure", lines: [FAILURE] });
    expect(
      describeDeviceDataDeletion({ record: { kind: "failed", code: "x" }, team: { kind: "incomplete" } }),
    ).toEqual({ tone: "failure", lines: [FAILURE] });
  });

  test("サーバーの message・code は文言に含めない", () => {
    const text = describeDeviceDataDeletion({
      record: { kind: "failed", code: "store_unavailable" },
      team: { kind: "completed" },
    }).lines.join("\n");
    expect(text).not.toContain("store_unavailable");
  });
});
