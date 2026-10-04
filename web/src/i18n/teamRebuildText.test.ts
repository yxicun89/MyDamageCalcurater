// ADR-0332 §6(F-08 / I-web-7): 構築の作り直しの文言(i18n/team.ts)。iOS と同じ語に揃える前提で固定する
// (画面テストは定数を引くが、E2E は文言そのものを書くので、変えるときは両方と iOS を直す)。
// 構築名に関する文言(作成・名前変更・取り込みの構築名)は機能ごと廃止する。
// 入力例(importExample)は実在名を含みうるので、ここでは文字列そのものではなく形だけを固定する(テストに実在名を書かない。ADR-0002)。

import { describe, expect, test } from "vitest";
import { exampleMasterSource } from "../master/exampleSource";
import { parseShowdownTeam } from "../team/showdownFormat";
import { planShowdownImport } from "../team/showdownImportPlan";
import { teamMemberText, teamScreenText, teamShowdownText } from "./team";

describe("構築の一覧・新規作成の文言", () => {
  test("ボタン・案内・表示名", () => {
    expect(teamScreenText.createLabel).toBe("新しい構築");
    expect(teamScreenText.emptyNotice).toBe(
      "まだ構築がありません。「新しい構築」を押すと、ポケモンを6体まで選んで構築を作れます",
    );
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

describe("Showdown 形式(補助の入口)の文言", () => {
  test("折りたたみ・説明・入力例の見出し", () => {
    expect(teamShowdownText.importFoldLabel).toBe("Showdown 形式で取り込む");
    expect(teamShowdownText.exportFoldLabel).toBe("Showdown 形式で書き出す");
    expect(teamShowdownText.importHelp).toBe(
      "Pokémon Showdown などで作った構築のテキストを貼り付けると、新しい構築として取り込めます。ポケモン・持ち物・特性・技は日本語の名前で書き、ポケモンごとに空の行で区切ります",
    );
    expect(teamShowdownText.importExampleLabel).toBe("入力の例(1体分)");
    expect(teamShowdownText.exportHelp).toBe(
      "保存した内容を Showdown 形式のテキストにします。コピーして他のアプリに貼り付けられます",
    );
    expect(teamShowdownText.importCreated(3)).toBe("3体の構築を作りました");
  });

  test("取り込みの構築名欄の文言は無い", () => {
    expect(Object.keys(teamShowdownText)).not.toContain("importNameLabel");
  });

  test("入力例は1体分で、ADR-0310 の形(名前 @ 持ち物・Ability・EVs・Nature・技の行)を持つ", () => {
    const lines = teamShowdownText.importExample.split("\n");
    expect(lines.length).toBeGreaterThanOrEqual(5);
    expect(lines.length).toBeLessThanOrEqual(7);
    // 空行が無い = 1体分(ポケモンごとに空の行で区切る形式)。
    expect(lines.every((line) => line.trim() !== "")).toBe(true);
    expect(lines[0]).toMatch(/^\S.* @ \S/);
    expect(lines.some((line) => line.startsWith("Ability: "))).toBe(true);
    // SP は EVs 行に 0〜32 をそのまま書く(ADR-0310)。合計66以下。
    const evs = lines.find((line) => line.startsWith("EVs: "));
    expect(evs).toBeDefined();
    const values = [...(evs ?? "").matchAll(/(\d+) (HP|Atk|Def|SpA|SpD|Spe)/g)].map((match) =>
      Number(match[1]),
    );
    expect(values.length).toBeGreaterThan(0);
    expect(values.every((value) => value >= 0 && value <= 32)).toBe(true);
    expect(values.reduce((sum, value) => sum + value, 0)).toBeLessThanOrEqual(66);
    expect(lines.some((line) => / Nature$/.test(line))).toBe(true);
    const moves = lines.filter((line) => line.startsWith("- "));
    expect(moves.length).toBeGreaterThanOrEqual(1);
    expect(moves.length).toBeLessThanOrEqual(4);
  });
});

describe("入力例と同じ形の文章(名前だけ架空)は、そのまま取り込める", () => {
  test("名前の行だけ例データの名前に置き換えた例が、parse → plan で作成できる(日本語名で書く形)", async () => {
    const master = await exampleMasterSource.load();
    const text = teamShowdownText.importExample
      .split("\n")
      .map((line, index, all) => {
        if (line.startsWith("- ") && all.findIndex((other) => other.startsWith("- ")) !== index) {
          return "- テストかえんパンチ";
        }
        if (line.includes(" @ ")) {
          return "テストほのお @ テストぼうぎょだま";
        }
        if (line.startsWith("Ability: ")) {
          return "Ability: テストむこう";
        }
        if (line.endsWith(" Nature")) {
          return "テストいじっぱり Nature";
        }
        if (line.startsWith("- ")) {
          return "- テストたいあたり";
        }
        return line;
      })
      .join("\n");
    const plan = planShowdownImport(parseShowdownTeam(text, master), master);
    expect(plan.issues).toEqual([]);
    expect(plan.canCreate).toBe(true);
    expect(plan.members).toHaveLength(1);
  });
});
