import { screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { openHome } from "./test/render";

const png = () => new File([new Uint8Array([137, 80, 78, 71])], "photo.png", { type: "image/png" });

// AC-REG-*: 登録(写真 / URL)
describe("登録", () => {
  it("AC-REG-01 写真を選び、名前とジャンルを入れて登録すると multipart で POST /api/items し、グリッドに増える", async () => {
    const { user, api } = await openHome();
    await user.click(screen.getByRole("button", { name: "追加" }));
    const dlg = screen.getByRole("dialog", { name: "登録" });
    await user.upload(within(dlg).getByLabelText("写真"), png());
    await user.type(within(dlg).getByRole("textbox", { name: "名前" }), "新商品");
    await user.selectOptions(within(dlg).getByRole("combobox", { name: "ジャンル" }), "S.H.Figuarts");
    await user.click(within(dlg).getByRole("button", { name: "登録" }));

    await waitFor(() => {
      expect(api.callsTo("POST", "/api/items")).toHaveLength(1);
    });
    const form = api.callsTo("POST", "/api/items")[0]?.form;
    expect(form?.get("name")).toBe("新商品");
    expect(form?.get("genre_id")).toBe("2");
    expect((form?.get("image") as File).name).toBe("photo.png");
    await waitFor(() => {
      expect(screen.queryByRole("dialog", { name: "登録" })).not.toBeInTheDocument();
    });
    expect(await screen.findByRole("button", { name: "新商品" })).toBeInTheDocument();
  });

  it("AC-REG-02 URL を貼って「取得」すると from-url で下書きが入り、確認して JSON で登録できる", async () => {
    const { user, api } = await openHome();
    api.draft = {
      name: "OGP のタイトル",
      image_url: "https://img.example/og.png",
      source_url: "https://shop.example/p/1",
      genre_id: 1,
    };
    await user.click(screen.getByRole("button", { name: "追加" }));
    const dlg = screen.getByRole("dialog", { name: "登録" });
    await user.type(within(dlg).getByRole("textbox", { name: "URL" }), "https://shop.example/p/1");
    await user.click(within(dlg).getByRole("button", { name: "取得" }));

    await waitFor(() => {
      expect(within(dlg).getByRole("textbox", { name: "名前" })).toHaveValue("OGP のタイトル");
    });
    expect(api.callsTo("POST", "/api/items/from-url")[0]?.json).toMatchObject({
      url: "https://shop.example/p/1",
    });
    // 下書きの genre_id が選択に反映される
    expect(within(dlg).getByRole("combobox", { name: "ジャンル" })).toHaveDisplayValue("デュエマ");

    await user.clear(within(dlg).getByRole("textbox", { name: "名前" }));
    await user.type(within(dlg).getByRole("textbox", { name: "名前" }), "直した名前");
    await user.click(within(dlg).getByRole("button", { name: "登録" }));
    await waitFor(() => {
      expect(api.callsTo("POST", "/api/items").filter((c) => c.json !== undefined)).toHaveLength(1);
    });
    const posted = api.callsTo("POST", "/api/items").find((c) => c.json !== undefined)?.json;
    expect(posted).toEqual({
      genre_id: 1,
      name: "直した名前",
      image_url: "https://img.example/og.png",
      source_url: "https://shop.example/p/1",
    });
    expect(await screen.findByRole("button", { name: "直した名前" })).toBeInTheDocument();
  });

  it("AC-REG-03 チップでジャンルを選んでいるときは、そのジャンルが登録の初期値", async () => {
    const { user } = await openHome();
    await user.click(
      within(screen.getByRole("navigation", { name: "ジャンル" })).getByRole("button", { name: "デュエマ" }),
    );
    await user.click(screen.getByRole("button", { name: "追加" }));
    expect(
      within(screen.getByRole("dialog", { name: "登録" })).getByRole("combobox", { name: "ジャンル" }),
    ).toHaveDisplayValue("デュエマ");
  });

  it("AC-REG-04 名前が空・画像も下書きも無いときは「登録」を押せない(API を呼ばない)", async () => {
    const { user, api } = await openHome();
    await user.click(screen.getByRole("button", { name: "追加" }));
    const dlg = screen.getByRole("dialog", { name: "登録" });
    expect(within(dlg).getByRole("button", { name: "登録" })).toBeDisabled();
    await user.type(within(dlg).getByRole("textbox", { name: "名前" }), "名前だけ");
    expect(within(dlg).getByRole("button", { name: "登録" })).toBeDisabled();
    expect(api.callsTo("POST", "/api/items")).toHaveLength(0);
  });

  it("AC-REG-05 取得・登録に失敗したら alert を出し、入力は残る", async () => {
    const { user, api } = await openHome();
    await user.click(screen.getByRole("button", { name: "追加" }));
    const dlg = screen.getByRole("dialog", { name: "登録" });
    await user.type(within(dlg).getByRole("textbox", { name: "URL" }), "https://shop.example/p/1");
    api.offline = true;
    await user.click(within(dlg).getByRole("button", { name: "取得" }));
    expect(await within(dlg).findByRole("alert")).toBeInTheDocument();
    expect(within(dlg).getByRole("textbox", { name: "URL" })).toHaveValue("https://shop.example/p/1");
  });

  it("AC-REG-06 キャンセルで閉じる(何も送らない)", async () => {
    const { user, api } = await openHome();
    const before = api.calls.length;
    await user.click(screen.getByRole("button", { name: "追加" }));
    await user.click(
      within(screen.getByRole("dialog", { name: "登録" })).getByRole("button", { name: "キャンセル" }),
    );
    expect(screen.queryByRole("dialog", { name: "登録" })).not.toBeInTheDocument();
    expect(api.calls.slice(before).filter((c) => c.method !== "GET")).toHaveLength(0);
  });
});
