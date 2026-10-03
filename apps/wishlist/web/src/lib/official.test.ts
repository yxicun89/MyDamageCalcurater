import { describe, expect, it } from "vitest";
import type { OfficialState, OfficialStatus } from "../api/types";
import {
  OFFICIAL_LABELS,
  officialChange,
  officialEvidence,
  officialLastAttempt,
  officialSummary,
} from "./official";

// フェーズ4-3 公式サイトの販売状況の表示(純粋関数。docs/phase4-spec.md AC-OFF-01〜04)。

const status = (over: Partial<OfficialStatus> = {}): OfficialStatus => ({
  status: "preorder",
  evidence: ["予約受付中", "予約する"],
  checked_at: "2026-10-03T18:00:00Z", // JST 10/4 3:00
  changed_at: null,
  previous_status: null,
  last_result: "preorder",
  last_attempt_at: "2026-10-03T18:00:00Z",
  ...over,
});

describe("OFFICIAL_LABELS(AC-OFF-01)", () => {
  it.each<[OfficialState, string]>([
    ["available", "販売中"],
    ["preorder", "予約受付中"],
    ["soldout", "在庫切れ"],
    ["ended", "販売終了"],
    ["unknown", "判定できません"],
    ["ambiguous", "判定できません(複数の表示)"],
    ["blocked", "取得しません(robots.txt)"],
    ["failed", "取得できませんでした"],
  ])("%s → %s", (s, want) => {
    expect(OFFICIAL_LABELS[s]).toBe(want);
  });
});

describe("officialSummary(AC-OFF-01)", () => {
  it.each<[string, OfficialStatus | null | undefined, string]>([
    ["判定済み", status(), "公式: 予約受付中(10/4 確認)"],
    ["販売終了", status({ status: "ended", last_result: "ended" }), "公式: 販売終了(10/4 確認)"],
    ["unknown", status({ status: "unknown", evidence: [] }), "公式: 判定できません(10/4 確認)"],
    ["ambiguous", status({ status: "ambiguous" }), "公式: 判定できません(複数の表示)(10/4 確認)"],
    [
      "一度も判定できていない failed は「確認」を付けない",
      status({ status: "failed", evidence: [], last_result: "failed" }),
      "公式: 取得できませんでした(10/4)",
    ],
    [
      "一度も判定できていない blocked",
      status({ status: "blocked", evidence: [], last_result: "blocked" }),
      "公式: 取得しません(robots.txt)(10/4)",
    ],
    ["まだ確かめていない(null)", null, "公式: まだ確認していません"],
    ["まだ確かめていない(undefined)", undefined, "公式: まだ確認していません"],
    [
      "日付は checked_at(last_attempt_at ではない)",
      status({ last_result: "failed", last_attempt_at: "2026-10-05T18:00:00Z" }),
      "公式: 予約受付中(10/4 確認)",
    ],
  ])("%s", (_name, s, want) => {
    expect(officialSummary(s)).toBe(want);
  });
});

describe("officialEvidence(AC-OFF-02)", () => {
  it.each<[string, OfficialStatus | null, string | null]>([
    ["2 語", status(), "根拠: 予約受付中・予約する"],
    ["1 語", status({ evidence: ["SOLD OUT"] }), "根拠: SOLD OUT"],
    ["空", status({ status: "unknown", evidence: [] }), null],
    ["null", null, null],
  ])("%s", (_name, s, want) => {
    expect(officialEvidence(s)).toBe(want);
  });
});

describe("officialChange(AC-OFF-03。changed_at から 7 日以内だけ)", () => {
  const changed = status({
    status: "ended",
    evidence: ["販売終了"],
    changed_at: "2026-10-02T18:00:00Z", // JST 10/3
    previous_status: "available",
    last_result: "ended",
  });
  it.each<[string, OfficialStatus | null, string, string | null]>([
    ["当日", changed, "2026-10-02T20:00:00Z", "10/3 に 販売中 → 販売終了"],
    ["ちょうど 7 日", changed, "2026-10-09T18:00:00Z", "10/3 に 販売中 → 販売終了"],
    ["7 日を 1 秒過ぎた", changed, "2026-10-09T18:00:01Z", null],
    ["時計のずれ(未来の changed_at)", changed, "2026-10-01T00:00:00Z", "10/3 に 販売中 → 販売終了"],
    ["変化なし", status(), "2026-10-04T00:00:00Z", null],
    [
      "判定できない状態への変化も出す",
      status({
        status: "unknown",
        evidence: [],
        changed_at: "2026-10-02T18:00:00Z",
        previous_status: "preorder",
      }),
      "2026-10-04T00:00:00Z",
      "10/3 に 予約受付中 → 判定できません",
    ],
    ["null", null, "2026-10-04T00:00:00Z", null],
  ])("%s", (_name, s, now, want) => {
    expect(officialChange(s, new Date(now))).toBe(want);
  });
});

describe("officialLastAttempt(AC-OFF-04。判定済みの状態を残したまま最後の試行が失敗したとき)", () => {
  it.each<[string, OfficialStatus | null, string | null]>([
    [
      "failed",
      status({ last_result: "failed", last_attempt_at: "2026-10-04T18:00:00Z" }),
      "最新の確認(10/5): 取得できませんでした",
    ],
    [
      "blocked",
      status({ last_result: "blocked", last_attempt_at: "2026-10-04T18:00:00Z" }),
      "最新の確認(10/5): 取得しません(robots.txt)",
    ],
    ["最後も判定できた", status(), null],
    ["status 自体が failed", status({ status: "failed", evidence: [], last_result: "failed" }), null],
    ["null", null, null],
  ])("%s", (_name, s, want) => {
    expect(officialLastAttempt(s)).toBe(want);
  });
});
