import { fireEvent, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { stubClipboard } from "./test/clipboard";
import { openHome } from "./test/render";

// phase6-spec 6-1: 選ばれているジャンルのボタンだけ、絵柄の隣に名前を出す(ユーザー決定 2026-10-12)。
// 選ばれていないボタンは絵柄だけ。どのボタンも aria-label はジャンル名のまま。

/** ボタンの見える文字(svg の中身は含めない前提で textContent)。 */
const visible = (b: HTMLElement) => b.textContent.trim();
/** グループ内の全ボタンについて { aria-label: 見える文字 } を並び順で返す。 */
const labels = (group: HTMLElement) =>
  within(group)
    .getAllByRole("button")
    .map((b) => [b.getAttribute("aria-label"), visible(b)]);

describe("ホームのジャンル: 選択中だけ名前を出す", () => {
  it("AC-6-1-1 最初は「すべて」だけ名前が見える。他は絵柄のみで aria-label は名前", async () => {
    await openHome();
    const nav = screen.getByRole("navigation", { name: "ジャンル" });
    expect(labels(nav)).toEqual([
      ["すべて", "すべて"],
      ["S.H.Figuarts", ""],
      ["デュエマ", ""],
    ]);
  });

  it("AC-6-1-2 選び直すと名前が移る(前に選んでいたボタンの名前は消える)", async () => {
    const { user } = await openHome();
    const nav = screen.getByRole("navigation", { name: "ジャンル" });
    await user.click(within(nav).getByRole("button", { name: "S.H.Figuarts" }));
    expect(labels(nav)).toEqual([
      ["すべて", ""],
      ["S.H.Figuarts", "S.H.Figuarts"],
      ["デュエマ", ""],
    ]);
    await user.click(within(nav).getByRole("button", { name: "デュエマ" }));
    expect(labels(nav)).toEqual([
      ["すべて", ""],
      ["S.H.Figuarts", ""],
      ["デュエマ", "デュエマ"],
    ]);
    await user.click(within(nav).getByRole("button", { name: "すべて" }));
    expect(labels(nav)[0]).toEqual(["すべて", "すべて"]);
  });

  it("AC-6-1-3 名前はボタンの中の文字なので、名前つきでもボタンは 1 つ・aria-pressed は選択中だけ true", async () => {
    await openHome();
    const nav = screen.getByRole("navigation", { name: "ジャンル" });
    expect(within(nav).getAllByRole("button")).toHaveLength(3);
    expect(within(nav).getAllByRole("button", { pressed: true })).toHaveLength(1);
  });
});

describe("追加・編集のジャンル: 選択中だけ名前を出す", () => {
  const URL1 = "https://shop.example/p/1";

  it("AC-6-1-4 追加の確認: 推定されたジャンルだけ名前が見える。押すと移る。未決定(null)なら誰も名前を出さない", async () => {
    stubClipboard(() => Promise.resolve(URL1));
    const r = await openHome();
    r.api.draft = {
      name: "OGP のタイトル",
      image_url: "https://img.example/og.png",
      source_url: URL1,
      genre_id: 2,
      genre_guessed: true,
    };
    await r.user.click(screen.getByRole("button", { name: "追加" }));
    const dialog = screen.getByRole("dialog", { name: "追加" });
    await r.user.click(within(dialog).getByRole("button", { name: "コピーした URL を追加" }));
    const group = await within(dialog).findByRole("group", { name: "ジャンル" });
    expect(labels(group)).toEqual([
      ["S.H.Figuarts", "S.H.Figuarts"],
      ["デュエマ", ""],
    ]);
    await r.user.click(within(group).getByRole("button", { name: "デュエマ" }));
    expect(labels(group)).toEqual([
      ["S.H.Figuarts", ""],
      ["デュエマ", "デュエマ"],
    ]);
  });

  it("AC-6-1-4 追加の確認: ジャンルが決まらないとき(genre_id null)は全部絵柄のみ", async () => {
    stubClipboard(() => Promise.resolve(URL1));
    const r = await openHome();
    r.api.draft = {
      name: "OGP のタイトル",
      image_url: "https://img.example/og.png",
      source_url: URL1,
      genre_id: null,
      genre_guessed: false,
    };
    await r.user.click(screen.getByRole("button", { name: "追加" }));
    const dialog = screen.getByRole("dialog", { name: "追加" });
    await r.user.click(within(dialog).getByRole("button", { name: "コピーした URL を追加" }));
    const group = await within(dialog).findByRole("group", { name: "ジャンル" });
    expect(labels(group)).toEqual([
      ["S.H.Figuarts", ""],
      ["デュエマ", ""],
    ]);
  });

  it("AC-6-1-5 編集: いまのジャンルだけ名前が見える。押すと移る", async () => {
    const r = await openHome();
    fireEvent.pointerDown(
      within(screen.getByTestId("item-grid")).getByRole("button", { name: "ボルシャック" }),
      {
        pointerId: 1,
        button: 0,
      },
    );
    const menu = await screen.findByRole("menu", {}, { timeout: 1500 });
    fireEvent.pointerUp(
      within(screen.getByTestId("item-grid")).getByRole("button", { name: "ボルシャック" }),
      {
        pointerId: 1,
      },
    );
    await r.user.click(within(menu).getByRole("menuitem", { name: "編集" }));
    const dlg = screen.getByRole("dialog", { name: "編集" });
    const group = within(dlg).getByRole("group", { name: "ジャンル" });
    expect(labels(group)).toEqual([
      ["S.H.Figuarts", ""],
      ["デュエマ", "デュエマ"],
    ]);
    await r.user.click(within(group).getByRole("button", { name: "S.H.Figuarts" }));
    expect(labels(group)).toEqual([
      ["S.H.Figuarts", "S.H.Figuarts"],
      ["デュエマ", ""],
    ]);
  });
});
