// P5-5e(ADR-0321): 構築の Showdown 入出力の文言(i18n/team.ts の teamShowdownText。ja.ts が再エクスポートする)。
// 画面テスト・E2E が引く固定の語をここで固定する(E2E は文言そのものを書くので、変えるときは両方)。
// issueReason は ShowdownIssueCode の全件を持つ(足し忘れをコンパイルと実行の両方で防ぐ)。

import { describe, expect, test } from "vitest";
import type { ShowdownIssue, ShowdownIssueCode } from "../team/showdownFormat";
import { teamShowdownText } from "./team";

const ALL_CODES = [
  "empty_input",
  "too_many_members",
  "malformed_line",
  "unresolved_name",
  "missing_nature",
  "sp_out_of_range",
  "sp_total_exceeded",
  "ev_like_value",
  "level_not_50",
  "iv_not_31",
  "too_many_moves",
  "duplicate_move",
  "nickname_too_long",
  "missing_name",
  "ambiguous_name",
  "duplicate_line",
  "input_too_large",
] as const satisfies readonly ShowdownIssueCode[];

describe("teamShowdownText の固定の語", () => {
  test("ラベル・ボタン", () => {
    expect(teamShowdownText).toMatchObject({
      importRegionLabel: "Showdown 形式から取り込む",
      importTextLabel: "取り込むテキスト",
      importPreviewLabel: "内容を確認",
      importCreateLabel: "この内容で作成",
      issuesLabel: "取り込みの問題",
      notesLabel: "取り込み時の補正",
      exportCopyLabel: "コピー",
      exportCloseLabel: "書き出しを閉じる",
      exportCopied: "コピーしました",
      exportEmptyNotice: "メンバーがいないので書き出せません",
      exportIssuesLabel: "書き出しの問題",
    });
    expect(teamShowdownText.exportLabel("A")).toBe("「A」を Showdown 形式で書き出す");
    expect(teamShowdownText.exportRegionLabel("A")).toBe("「A」の Showdown 形式");
    expect(teamShowdownText.exportTextLabel("A")).toBe("「A」の書き出しテキスト");
    expect(teamShowdownText.previewSummary(3)).toBe("3体を取り込めます");
    expect(teamShowdownText.importCreated(3)).toBe("3体の構築を作りました");
  });

  test("issueReason は全コードに空でない日本語を持つ。コードの生の文字列を出さない", () => {
    for (const code of ALL_CODES) {
      const reason = teamShowdownText.issueReason[code];
      expect(reason.length).toBeGreaterThan(0);
      expect(reason).not.toBe(code);
      expect(reason).not.toMatch(/^[a-z_]+$/);
    }
    expect(Object.keys(teamShowdownText.issueReason).sort()).toEqual([...ALL_CODES].sort());
  });

  test("issueText: 重大度・何体目(1 始まり)・理由・値を含み、全体の issue には体の番号を付けない", () => {
    const withMember: ShowdownIssue = {
      severity: "error",
      code: "unresolved_name",
      memberIndex: 1,
      field: "species",
      value: "ふめいなもん",
    };
    const text = teamShowdownText.issueText(withMember);
    expect(text).toContain("エラー");
    expect(text).toContain("2体目");
    expect(text).toContain(teamShowdownText.issueReason.unresolved_name);
    expect(text).toContain("ふめいなもん");

    const global: ShowdownIssue = {
      severity: "warning",
      code: "too_many_members",
      memberIndex: null,
      field: null,
    };
    const globalText = teamShowdownText.issueText(global);
    expect(globalText).toContain("警告");
    expect(globalText).not.toMatch(/\d体目/);
  });

  test("メガの補正の note の文言(mega_item_fixed / mega_item_unavailable)は何体目かと種族名を含む", () => {
    expect(
      teamShowdownText.megaNoteText({ kind: "mega_item_fixed", memberIndex: 0, speciesKey: "k" }, "メガX"),
    ).toMatch(/1体目.*メガX/);
    expect(
      teamShowdownText.megaNoteText(
        { kind: "mega_item_unavailable", memberIndex: 2, speciesKey: "k" },
        "メガY",
      ),
    ).toMatch(/3体目.*メガY/);
  });
});
