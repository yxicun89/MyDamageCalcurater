import { screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { stubClipboard } from "./test/clipboard";
import type { FakeApi } from "./test/fakeApi";
import { openHome } from "./test/render";

// 追加のシート(phase5-spec 5-2「追加」・ユーザー決定 2026-10-11)。
// 旧 AC-REG-*(登録ダイアログ: 写真・URL・名前・ジャンルの select を 1 画面に並べて「登録」)は、
// 入口 3 つ(コピーした URL / URL を入力 / 写真)→ 確認(1 タップで「追加」)の 2 段に置き換えた。
// 旧テストの趣旨(写真の multipart・URL 下書きの JSON 登録・初期ジャンル・空は押せない・失敗は alert・キャンセル)は下で引き継ぐ。

const png = () => new File([new Uint8Array([137, 80, 78, 71])], "photo.png", { type: "image/png" });
const URL1 = "https://shop.example/p/1";

type Draft = FakeApi["draft"];
const draft = (over: Partial<Draft> = {}): Draft => ({
  name: "OGP のタイトル",
  image_url: "https://img.example/og.png",
  source_url: URL1,
  genre_id: 2,
  genre_guessed: true,
  ...over,
});

async function openAdd(over: Partial<Draft> = {}) {
  const r = await openHome();
  r.api.draft = draft(over);
  await r.user.click(screen.getByRole("button", { name: "追加" }));
  const dialog = screen.getByRole("dialog", { name: "追加" });
  return { ...r, dialog };
}
const created = (api: FakeApi) => api.callsTo("POST", "/api/items").filter((c) => c.path === "/api/items");
const pressed = (dialog: HTMLElement) =>
  within(dialog)
    .queryAllByRole("button", { pressed: true })
    .map((b) => b.getAttribute("aria-label"));

describe("追加: 入口", () => {
  it("AC-5-2d-1 入口は「コピーした URL を追加」「URL を入力」「写真」の 3 つ", async () => {
    const { dialog } = await openAdd();
    expect(within(dialog).getByRole("button", { name: "コピーした URL を追加" })).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "URL を入力" })).toBeInTheDocument();
    expect(within(dialog).getByLabelText("写真")).toHaveAttribute("type", "file");
  });

  it("AC-5-2d-2 シートを開いただけではクリップボードを読まない。「コピーした URL を追加」を押したときに読み、URL なら from-url へ", async () => {
    const readText = stubClipboard(() => Promise.resolve(` ${URL1} `));
    const { dialog, user, api } = await openAdd();
    expect(readText).not.toHaveBeenCalled();
    await user.click(within(dialog).getByRole("button", { name: "コピーした URL を追加" }));
    expect(readText).toHaveBeenCalledTimes(1);
    await waitFor(() => {
      expect(api.callsTo("POST", "/api/items/from-url")).toHaveLength(1);
    });
    // 前後の空白を除いた URL だけを送る。ジャンルは推定に任せるので genre_id は付けない
    expect(api.callsTo("POST", "/api/items/from-url")[0]?.json).toEqual({ url: URL1 });
    expect(await within(dialog).findByRole("textbox", { name: "名前" })).toHaveValue("OGP のタイトル");
  });

  it("AC-5-2d-3 クリップボードが読めない(拒否・未対応)・URL でないときは、URL の入力欄を開く(from-url は呼ばない)", async () => {
    const denied = stubClipboard(() => Promise.reject(new DOMException("denied", "NotAllowedError")));
    const a = await openAdd();
    await a.user.click(within(a.dialog).getByRole("button", { name: "コピーした URL を追加" }));
    expect(denied).toHaveBeenCalled();
    expect(await within(a.dialog).findByRole("textbox", { name: "URL" })).toBeInTheDocument();
    expect(a.api.callsTo("POST", "/api/items/from-url")).toHaveLength(0);
    expect(within(a.dialog).queryByRole("alert")).not.toBeInTheDocument(); // 失敗扱いにせず、そのまま入力へ
  });

  it("AC-5-2d-3 URL でない文字列(javascript: を含む)がコピーされていても入力欄を開く", async () => {
    stubClipboard(() => Promise.resolve("javascript:alert(1)"));
    const { dialog, user, api } = await openAdd();
    await user.click(within(dialog).getByRole("button", { name: "コピーした URL を追加" }));
    expect(await within(dialog).findByRole("textbox", { name: "URL" })).toHaveValue("");
    expect(api.callsTo("POST", "/api/items/from-url")).toHaveLength(0);
  });

  it("AC-5-2d-3 navigator.clipboard が無い環境でも入力欄を開く", async () => {
    const { dialog, user } = await openAdd(); // stubClipboard しない = navigator.clipboard は undefined
    await user.click(within(dialog).getByRole("button", { name: "コピーした URL を追加" }));
    expect(await within(dialog).findByRole("textbox", { name: "URL" })).toBeInTheDocument();
  });

  it("AC-5-2d-4 「URL を入力」→ 貼って「取得」→ 確認へ(from-url に URL を送る)", async () => {
    const { dialog, user, api } = await openAdd();
    await user.click(within(dialog).getByRole("button", { name: "URL を入力" }));
    await user.type(within(dialog).getByRole("textbox", { name: "URL" }), URL1);
    await user.click(within(dialog).getByRole("button", { name: "取得" }));
    await waitFor(() => {
      expect(api.callsTo("POST", "/api/items/from-url")[0]?.json).toEqual({ url: URL1 });
    });
    expect(await within(dialog).findByRole("textbox", { name: "名前" })).toHaveValue("OGP のタイトル");
  });

  it("AC-5-2d-5 取得に失敗したら alert を出し、入力した URL は残る", async () => {
    const { dialog, user, api } = await openAdd();
    await user.click(within(dialog).getByRole("button", { name: "URL を入力" }));
    await user.type(within(dialog).getByRole("textbox", { name: "URL" }), URL1);
    api.offline = true;
    await user.click(within(dialog).getByRole("button", { name: "取得" }));
    expect(await within(dialog).findByRole("alert")).toBeInTheDocument();
    expect(within(dialog).getByRole("textbox", { name: "URL" })).toHaveValue(URL1);
  });
});

describe("追加: 確認(URL から)", () => {
  async function toConfirm(over: Partial<Draft> = {}) {
    stubClipboard(() => Promise.resolve(URL1));
    const r = await openAdd(over);
    await r.user.click(within(r.dialog).getByRole("button", { name: "コピーした URL を追加" }));
    await within(r.dialog).findByRole("textbox", { name: "名前" });
    return r;
  }

  it("AC-5-2e-1 下書きの画像・整えた名前を出し、推定したジャンルが選ばれている。「追加」1 回で JSON 登録", async () => {
    const { dialog, user, api } = await toConfirm({ genre_id: 2, genre_guessed: true });
    expect(within(dialog).getByRole("img", { name: "OGP のタイトル" })).toHaveAttribute(
      "src",
      "https://img.example/og.png",
    );
    // ジャンルは絵柄のボタン(aria-label が名前)。推定した S.H.Figuarts だけが押されている
    const genres = within(dialog).getByRole("group", { name: "ジャンル" });
    expect(within(genres).getAllByRole("button")).toHaveLength(2);
    expect(pressed(genres)).toEqual(["S.H.Figuarts"]);
    expect(
      within(genres).getByRole("button", { name: "S.H.Figuarts" }).querySelector("[data-icon]"),
    ).toHaveAttribute("data-icon", "figure");

    await user.click(within(dialog).getByRole("button", { name: "追加" }));
    await waitFor(() => {
      expect(created(api)).toHaveLength(1);
    });
    expect(created(api)[0]?.json).toEqual({
      genre_id: 2,
      name: "OGP のタイトル",
      image_url: "https://img.example/og.png",
      source_url: URL1,
    });
    await waitFor(() => {
      expect(screen.queryByRole("dialog", { name: "追加" })).not.toBeInTheDocument();
    });
    expect(await screen.findByRole("button", { name: "OGP のタイトル" })).toBeInTheDocument();
  });

  it("AC-5-2e-2 ジャンルは絵柄を押して替えられる(押したものだけが選ばれ、登録にも反映)", async () => {
    const { dialog, user, api } = await toConfirm({ genre_id: 2, genre_guessed: true });
    await user.click(within(dialog).getByRole("button", { name: "デュエマ" }));
    expect(pressed(within(dialog).getByRole("group", { name: "ジャンル" }))).toEqual(["デュエマ"]);
    await user.click(within(dialog).getByRole("button", { name: "追加" }));
    await waitFor(() => {
      expect(created(api)).toHaveLength(1);
    });
    expect(created(api)[0]?.json).toMatchObject({ genre_id: 1 });
  });

  it("AC-5-2e-3 ジャンルが決まらない(genre_id null)ときは何も選ばず、選ぶまで「追加」を押せない", async () => {
    const { dialog, user, api } = await toConfirm({ genre_id: null, genre_guessed: false });
    const genres = within(dialog).getByRole("group", { name: "ジャンル" });
    expect(pressed(genres)).toEqual([]);
    expect(within(dialog).getByRole("button", { name: "追加" })).toBeDisabled();
    await user.click(within(genres).getByRole("button", { name: "デュエマ" }));
    expect(within(dialog).getByRole("button", { name: "追加" })).toBeEnabled();
    expect(created(api)).toHaveLength(0);
  });

  it("AC-5-2e-3 ホームでジャンルを絞っているとき、推定が決まらなければそのジャンルを選んでおく", async () => {
    stubClipboard(() => Promise.resolve(URL1));
    const r = await openHome();
    r.api.draft = draft({ genre_id: null, genre_guessed: false });
    await r.user.click(
      within(screen.getByRole("navigation", { name: "ジャンル" })).getByRole("button", { name: "デュエマ" }),
    );
    await r.user.click(screen.getByRole("button", { name: "追加" }));
    const dialog = screen.getByRole("dialog", { name: "追加" });
    await r.user.click(within(dialog).getByRole("button", { name: "コピーした URL を追加" }));
    await within(dialog).findByRole("textbox", { name: "名前" });
    expect(pressed(within(dialog).getByRole("group", { name: "ジャンル" }))).toEqual(["デュエマ"]);
  });

  it("AC-5-2e-4 名前は直せる(直した名前で登録)。空(駿河屋のように名前が取れない)なら入れるまで「追加」を押せない", async () => {
    const { dialog, user, api } = await toConfirm({ name: "", genre_id: 2 });
    expect(within(dialog).getByRole("textbox", { name: "名前" })).toHaveValue("");
    expect(within(dialog).getByRole("button", { name: "追加" })).toBeDisabled();
    await user.type(within(dialog).getByRole("textbox", { name: "名前" }), "直した名前");
    await user.click(within(dialog).getByRole("button", { name: "追加" }));
    await waitFor(() => {
      expect(created(api)).toHaveLength(1);
    });
    expect(created(api)[0]?.json).toMatchObject({ name: "直した名前", genre_id: 2 });
    expect(await screen.findByRole("button", { name: "直した名前" })).toBeInTheDocument();
  });

  it("AC-5-2e-5 画像が取れない(image_url null)ときは「写真を選ぶ」の枠と名前の欄。写真を選ぶまで「追加」を押せない", async () => {
    const { dialog, user, api } = await toConfirm({ image_url: null, genre_id: 2 });
    expect(within(dialog).queryByRole("img", { name: "OGP のタイトル" })).not.toBeInTheDocument();
    expect(within(dialog).getByText("このページの画像は取れませんでした")).toBeInTheDocument();
    expect(within(dialog).getByRole("textbox", { name: "名前" })).toHaveValue("OGP のタイトル");
    expect(within(dialog).getByRole("button", { name: "追加" })).toBeDisabled();

    await user.upload(within(dialog).getByLabelText("写真を選ぶ"), png());
    expect(within(dialog).getByRole("button", { name: "追加" })).toBeEnabled();
    await user.click(within(dialog).getByRole("button", { name: "追加" }));
    await waitFor(() => {
      expect(created(api)).toHaveLength(1);
    });
    // 写真は multipart。source_url も残す
    const form = created(api)[0]?.form;
    expect(form?.get("name")).toBe("OGP のタイトル");
    expect(form?.get("genre_id")).toBe("2");
    expect(form?.get("source_url")).toBe(URL1);
    expect((form?.get("image") as File).name).toBe("photo.png");
  });

  it("AC-5-2e-6 登録に失敗したら alert を出し、シートは閉じず入力は残る", async () => {
    const { dialog, user, api } = await toConfirm({ genre_id: 2 });
    api.offline = true;
    await user.click(within(dialog).getByRole("button", { name: "追加" }));
    expect(await within(dialog).findByRole("alert")).toBeInTheDocument();
    expect(within(dialog).getByRole("textbox", { name: "名前" })).toHaveValue("OGP のタイトル");
    expect(pressed(within(dialog).getByRole("group", { name: "ジャンル" }))).toEqual(["S.H.Figuarts"]);
  });
});

describe("追加: 写真から", () => {
  it("AC-5-2f-1 「写真」を選ぶと、名前とジャンルを入れて「追加」→ multipart で POST /api/items し、グリッドに増える(旧 AC-REG-01)", async () => {
    const { dialog, user, api } = await openAdd();
    await user.upload(within(dialog).getByLabelText("写真"), png());
    expect(await within(dialog).findByRole("textbox", { name: "名前" })).toHaveValue("");
    // 名前もジャンルも入るまで押せない
    expect(within(dialog).getByRole("button", { name: "追加" })).toBeDisabled();
    await user.type(within(dialog).getByRole("textbox", { name: "名前" }), "新商品");
    expect(within(dialog).getByRole("button", { name: "追加" })).toBeDisabled();
    await user.click(
      within(within(dialog).getByRole("group", { name: "ジャンル" })).getByRole("button", {
        name: "S.H.Figuarts",
      }),
    );
    await user.click(within(dialog).getByRole("button", { name: "追加" }));

    await waitFor(() => {
      expect(created(api)).toHaveLength(1);
    });
    const form = created(api)[0]?.form;
    expect(form?.get("name")).toBe("新商品");
    expect(form?.get("genre_id")).toBe("2");
    expect((form?.get("image") as File).name).toBe("photo.png");
    expect(api.callsTo("POST", "/api/items/from-url")).toHaveLength(0);
    await waitFor(() => {
      expect(screen.queryByRole("dialog", { name: "追加" })).not.toBeInTheDocument();
    });
    expect(await screen.findByRole("button", { name: "新商品" })).toBeInTheDocument();
  });

  it("AC-5-2f-2 ホームでジャンルを絞っているときは、そのジャンルが初期値(旧 AC-REG-03)", async () => {
    const { user } = await openHome();
    await user.click(
      within(screen.getByRole("navigation", { name: "ジャンル" })).getByRole("button", { name: "デュエマ" }),
    );
    await user.click(screen.getByRole("button", { name: "追加" }));
    const dialog = screen.getByRole("dialog", { name: "追加" });
    await user.upload(within(dialog).getByLabelText("写真"), png());
    expect(pressed(within(dialog).getByRole("group", { name: "ジャンル" }))).toEqual(["デュエマ"]);
  });

  it("AC-5-2f-3 キャンセル(閉じる)で何も送らずに閉じる(旧 AC-REG-06)", async () => {
    const { user, api } = await openHome();
    const before = api.calls.length;
    await user.click(screen.getByRole("button", { name: "追加" }));
    await user.click(
      within(screen.getByRole("dialog", { name: "追加" })).getByRole("button", { name: "キャンセル" }),
    );
    expect(screen.queryByRole("dialog", { name: "追加" })).not.toBeInTheDocument();
    expect(api.calls.slice(before).filter((c) => c.method !== "GET")).toHaveLength(0);
  });
});
