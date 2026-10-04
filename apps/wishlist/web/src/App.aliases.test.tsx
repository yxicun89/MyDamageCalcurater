import { screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { makeGenre } from "./test/factories";
import { openHome } from "./test/render";
import { scenario } from "./test/scenario";

// AC-SET-08〜12(docs/phase4-spec.md): ジャンルの編集で表記揺れの辞書(別名グループ)を編集する。
// 1 グループ = 1 行(カンマ区切り)のテキスト欄。名前は「別名グループN」(1 始まり)。

const FIGUARTS = ["S.H.Figuarts", "SHフィギュアーツ", "フィギュアーツ"];

/** S.H.Figuarts(id 2)に別名グループを持たせた初期データで設定を開き、そのジャンルの編集ダイアログを返す。 */
async function openFiguartsDialog(aliases: string[][] = [FIGUARTS, ["HG", "ハイグレード"]]) {
  const s = scenario();
  const genres = s.genres.map((g) => (g.id === 2 ? makeGenre({ ...g, aliases }) : g));
  const r = await openHome({ genres });
  await r.user.click(screen.getByRole("button", { name: "設定" }));
  const section = screen.getByRole("region", { name: "ジャンル" });
  await r.user.click(within(section).getByRole("button", { name: "S.H.Figuartsを編集" }));
  return { ...r, dlg: screen.getByRole("dialog", { name: "ジャンル" }) };
}

const row = (dlg: HTMLElement, n: number) =>
  within(dlg).getByRole("textbox", { name: `別名グループ${String(n)}` });

describe("表記揺れの辞書", () => {
  it("AC-SET-08 編集ダイアログに既存の別名グループが 1 行ずつ「, 」区切りで出る", async () => {
    const { dlg } = await openFiguartsDialog();
    expect(row(dlg, 1)).toHaveValue("S.H.Figuarts, SHフィギュアーツ, フィギュアーツ");
    expect(row(dlg, 2)).toHaveValue("HG, ハイグレード");
    expect(within(dlg).queryByRole("textbox", { name: "別名グループ3" })).not.toBeInTheDocument();
  });

  it("AC-SET-09 行の編集・追加・削除で、PATCH は aliases だけを全件置き換えで送る", async () => {
    const { user, api, dlg } = await openFiguartsDialog();
    await user.clear(row(dlg, 2));
    await user.type(row(dlg, 2), "HG，ハイグレード、エイチジー");
    await user.click(within(dlg).getByRole("button", { name: "別名グループを追加" }));
    await user.type(row(dlg, 3), "MG, マスターグレード");
    await user.click(within(dlg).getByRole("button", { name: "別名グループ1を削除" }));
    // 削除後は番号を詰める
    expect(row(dlg, 1)).toHaveValue("HG，ハイグレード、エイチジー");
    expect(row(dlg, 2)).toHaveValue("MG, マスターグレード");
    await user.click(within(dlg).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(api.callsTo("PATCH", "/api/genres/2")).toHaveLength(1);
    });
    expect(api.callsTo("PATCH", "/api/genres/2")[0]?.json).toEqual({
      aliases: [
        ["HG", "ハイグレード", "エイチジー"],
        ["MG", "マスターグレード"],
      ],
    });
  });

  it("AC-SET-09 全部の行を消すと aliases: [] を送る。空の行は送らない", async () => {
    const { user, api, dlg } = await openFiguartsDialog([FIGUARTS]);
    await user.click(within(dlg).getByRole("button", { name: "別名グループを追加" }));
    await user.click(within(dlg).getByRole("button", { name: "別名グループ1を削除" }));
    expect(row(dlg, 1)).toHaveValue("");
    await user.click(within(dlg).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(api.callsTo("PATCH", "/api/genres/2")).toHaveLength(1);
    });
    expect(api.callsTo("PATCH", "/api/genres/2")[0]?.json).toEqual({ aliases: [] });
  });

  it("AC-SET-10 1 語だけの行があると alert を出し、API を呼ばない", async () => {
    const { user, api, dlg } = await openFiguartsDialog();
    await user.clear(row(dlg, 2));
    await user.type(row(dlg, 2), "HG");
    await user.click(within(dlg).getByRole("button", { name: "保存" }));
    expect(await within(dlg).findByRole("alert")).toHaveTextContent("2語以上");
    expect(api.callsTo("PATCH", "/api/genres/2")).toHaveLength(0);
  });

  it("AC-SET-11 ジャンルの追加でも別名グループを送る(空の行は除く)", async () => {
    const { user, api } = await openHome();
    await user.click(screen.getByRole("button", { name: "設定" }));
    const section = screen.getByRole("region", { name: "ジャンル" });
    await user.click(within(section).getByRole("button", { name: "ジャンルを追加" }));
    const dlg = screen.getByRole("dialog", { name: "ジャンル" });
    await user.type(within(dlg).getByRole("textbox", { name: "ジャンル名" }), "ガンプラ");
    await user.click(within(dlg).getByRole("button", { name: "別名グループを追加" }));
    await user.type(row(dlg, 1), "HG, ハイグレード");
    await user.click(within(dlg).getByRole("button", { name: "別名グループを追加" }));
    await user.click(within(dlg).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(api.callsTo("POST", "/api/genres")).toHaveLength(1);
    });
    expect(api.callsTo("POST", "/api/genres")[0]?.json).toMatchObject({
      name: "ガンプラ",
      aliases: [["HG", "ハイグレード"]],
    });
  });

  it("AC-SET-12 aliases を返さない古い応答でも開け、別名を変えなければ aliases を送らない", async () => {
    const { user, api } = await openHome();
    await user.click(screen.getByRole("button", { name: "設定" }));
    const section = screen.getByRole("region", { name: "ジャンル" });
    await user.click(within(section).getByRole("button", { name: "デュエマを編集" }));
    const dlg = screen.getByRole("dialog", { name: "ジャンル" });
    expect(within(dlg).queryByRole("textbox", { name: "別名グループ1" })).not.toBeInTheDocument();
    expect(within(dlg).getByRole("button", { name: "別名グループを追加" })).toBeInTheDocument();
    const name = within(dlg).getByRole("textbox", { name: "ジャンル名" });
    await user.clear(name);
    await user.type(name, "デュエル・マスターズ");
    await user.click(within(dlg).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(api.callsTo("PATCH", "/api/genres/1")).toHaveLength(1);
    });
    expect(api.callsTo("PATCH", "/api/genres/1")[0]?.json).toEqual({ name: "デュエル・マスターズ" });
  });
});
