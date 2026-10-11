// ADR-0332 §6(F-08 / I-web-7): 構築の作り直しの文言(i18n/team.ts)。iOS と同じ語に揃える前提で固定する
// (画面テストは定数を引くが、E2E は文言そのものを書くので、変えるときは両方と iOS を直す)。
// 構築名に関する文言(作成・名前変更・取り込みの構築名)は機能ごと廃止する。
// Showdown 形式の文言は G-03 で廃止(ADR-0342)。

import { describe, expect, test } from "vitest";
import { teamMemberText, teamScreenText } from "./team";

describe("構築の一覧・新規作成の文言", () => {
  test("ボタン・案内・表示名", () => {
    expect(teamScreenText.createLabel).toBe("新しい構築");
    // G-05(I-web-13d): 長い案内の文をやめ、大きいアイコン + 短い一言にした(ADR-0347)。
    expect(teamScreenText.emptyNotice).toBe("まだ構築がありません");
    expect(teamScreenText.untitledTeamName(3)).toBe("構築 3");
    expect(teamScreenText.memberIconsLabel("構築 1")).toBe("「構築 1」のポケモン");
    expect(teamScreenText.unknownMemberIcon(2)).toBe("2体目");
  });

  test("構築名の入力・名前変更の文言は無い(機能の廃止)", () => {
    for (const key of [
      "createHeading",
      "nameLabel",
      "nameRequiredNotice",
      "nameTooLongNotice",
      "renameLabel",
      "renameFieldLabel",
      "renameSaveLabel",
      "renameCancelLabel",
      "renameErrorHeading",
    ]) {
      expect(Object.keys(teamScreenText)).not.toContain(key);
    }
  });
});

describe("6枠の編集の文言", () => {
  test("開く・戻る・保存・未保存の印・戻るときの確認", () => {
    expect(teamMemberText.editLabel("構築 1")).toBe("「構築 1」を開く");
    expect(teamMemberText.closeLabel).toBe("一覧に戻る");
    expect(teamMemberText.saveLabel).toBe("保存");
    expect(teamMemberText.unsavedNotice).toBe("保存していない変更があります");
    expect(teamMemberText.leaveConfirmNotice).toBe(
      "保存していない変更があります。保存せずに一覧に戻りますか",
    );
    expect(teamMemberText.leaveDiscardLabel).toBe("保存せずに戻る");
    expect(teamMemberText.leaveCancelLabel).toBe("編集を続ける");
  });

  test("空の枠の案内と、枠の操作", () => {
    expect(teamMemberText.emptySlotHint).toBe("ポケモンを選ぶと、技・持ち物・特性などを決められます");
    expect(teamMemberText.removeLabel(3)).toBe("3体目を外す");
    expect(teamMemberText.moveUpLabel(2)).toBe("2体目を上へ");
    expect(teamMemberText.moveDownLabel(2)).toBe("2体目を下へ");
    expect(teamMemberText.memberLegend(6)).toBe("6体目");
  });

  test("[メンバーを追加] の文言は無い(6枠が最初からある)", () => {
    expect(Object.keys(teamMemberText)).not.toContain("addLabel");
    expect(Object.keys(teamMemberText)).not.toContain("addDisabledNotice");
  });

  test("機械的な語を使わない(「観測」「メンバーを保存」など)", () => {
    const texts = [
      teamMemberText.unsavedNotice,
      teamMemberText.leaveConfirmNotice,
      teamMemberText.emptySlotHint,
      teamScreenText.emptyNotice,
    ];
    for (const text of texts) {
      expect(text).not.toMatch(/観測|メンバーを保存|構築名/);
    }
  });
});
