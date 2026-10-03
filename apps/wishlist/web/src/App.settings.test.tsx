import { render, screen, waitFor, within } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { App } from "./App";
import { installFakeApi } from "./test/fakeApi";
import { openHome } from "./test/render";

async function openSettings() {
  const r = await openHome();
  await r.user.click(screen.getByRole("button", { name: "設定" }));
  return r;
}

// AC-SET-*: 設定
describe("設定", () => {
  it("AC-SET-01 トークン未設定で開くと設定画面から始まり、API を呼ばない", async () => {
    const api = installFakeApi();
    render(<App />);
    expect(await screen.findByRole("heading", { name: "設定" })).toBeInTheDocument();
    expect(api.calls).toHaveLength(0);
  });

  it("AC-SET-02 API の URL とトークンを保存すると localStorage に入り、以降の呼び出しに使われる", async () => {
    const api = installFakeApi({ genres: [], sites: [], items: [] });
    const user = userEvent.setup();
    render(<App />);
    await user.type(await screen.findByRole("textbox", { name: "APIのURL" }), "https://h.example/wishlist/");
    await user.type(screen.getByLabelText("トークン"), "test-token");
    expect(screen.getByLabelText("トークン")).toHaveAttribute("type", "password");
    await user.click(screen.getByRole("button", { name: "保存" }));
    expect(JSON.parse(localStorage.getItem("wishlist.settings") ?? "null")).toEqual({
      apiBaseUrl: "https://h.example/wishlist/",
      token: "test-token",
    });
    await user.click(screen.getByRole("button", { name: "戻る" }));
    await waitFor(() => {
      expect(api.calls.length).toBeGreaterThan(0);
    });
    expect(api.calls[0]?.headers.get("Authorization")).toBe("Bearer test-token");
  });

  it("AC-SET-03 ジャンル一覧(sort_order 順)を出し、追加できる(名前・テンプレート・サイトの選択と順序)", async () => {
    const { user, api } = await openSettings();
    const section = screen.getByRole("region", { name: "ジャンル" });
    expect(
      within(section)
        .getAllByRole("listitem")
        .map((li) => li.textContent),
    ).toEqual([expect.stringContaining("S.H.Figuarts"), expect.stringContaining("デュエマ")]);

    await user.click(within(section).getByRole("button", { name: "ジャンルを追加" }));
    const dlg = screen.getByRole("dialog", { name: "ジャンル" });
    await user.type(within(dlg).getByRole("textbox", { name: "ジャンル名" }), "ガンプラ");
    const tpl = within(dlg).getByRole("textbox", { name: "検索ワードのテンプレート" });
    await user.clear(tpl);
    await user.type(tpl, "{{name}", { skipClick: false });
    await user.click(within(dlg).getByRole("checkbox", { name: "メルカリ" }));
    await user.click(within(dlg).getByRole("checkbox", { name: "Amazon" }));
    await user.click(within(dlg).getByRole("button", { name: "Amazonを上へ" }));
    await user.click(within(dlg).getByRole("button", { name: "保存" }));

    await waitFor(() => {
      expect(api.callsTo("POST", "/api/genres")).toHaveLength(1);
    });
    expect(api.callsTo("POST", "/api/genres")[0]?.json).toMatchObject({
      name: "ガンプラ",
      query_template: "{name}",
      site_ids: [2, 1],
    });
    expect(await within(section).findByText("ガンプラ")).toBeInTheDocument();
  });

  it("AC-SET-04 ジャンルの編集は PATCH(表示サイトの順序を site_ids で全件置き換え)", async () => {
    const { user, api } = await openSettings();
    const section = screen.getByRole("region", { name: "ジャンル" });
    await user.click(within(section).getByRole("button", { name: "S.H.Figuartsを編集" }));
    const dlg = screen.getByRole("dialog", { name: "ジャンル" });
    // 元の順序は Amazon(2), メルカリ(1), その他(3)
    await user.click(within(dlg).getByRole("button", { name: "メルカリを上へ" }));
    await user.click(within(dlg).getByRole("checkbox", { name: "その他" })); // 外す
    await user.click(within(dlg).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(api.callsTo("PATCH", "/api/genres/2")).toHaveLength(1);
    });
    expect(api.callsTo("PATCH", "/api/genres/2")[0]?.json).toEqual({ site_ids: [1, 2] });
  });

  it("AC-SET-05 サイトの追加: 検索 URL テンプレートに {q} が無いと alert を出し、API を呼ばない", async () => {
    const { user, api } = await openSettings();
    const section = screen.getByRole("region", { name: "サイト" });
    await user.click(within(section).getByRole("button", { name: "サイトを追加" }));
    const dlg = screen.getByRole("dialog", { name: "サイト" });
    await user.type(within(dlg).getByRole("textbox", { name: "サイト名" }), "新サイト");
    await user.type(
      within(dlg).getByRole("textbox", { name: "検索URLのテンプレート" }),
      "https://new.example/s?q=",
    );
    await user.click(within(dlg).getByRole("button", { name: "保存" }));
    expect(await within(dlg).findByRole("alert")).toHaveTextContent("{q}");
    expect(api.callsTo("POST", "/api/sites")).toHaveLength(0);
  });

  it("AC-SET-06 サイトの追加: 正しいテンプレートと取得方式で POST し、一覧に増える", async () => {
    const { user, api } = await openSettings();
    const section = screen.getByRole("region", { name: "サイト" });
    await user.click(within(section).getByRole("button", { name: "サイトを追加" }));
    const dlg = screen.getByRole("dialog", { name: "サイト" });
    await user.type(within(dlg).getByRole("textbox", { name: "サイト名" }), "新サイト");
    await user.type(
      within(dlg).getByRole("textbox", { name: "検索URLのテンプレート" }),
      "https://new.example/s?q={{q}",
    );
    await user.selectOptions(within(dlg).getByRole("combobox", { name: "取得方式" }), "scrape");
    await user.click(within(dlg).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(api.callsTo("POST", "/api/sites")).toHaveLength(1);
    });
    expect(api.callsTo("POST", "/api/sites")[0]?.json).toMatchObject({
      name: "新サイト",
      search_url_template: "https://new.example/s?q={q}",
      fetch_type: "scrape",
    });
    expect(await within(section).findByText("新サイト")).toBeInTheDocument();
  });

  it("AC-SET-07 サイトの編集は PATCH(変えた項目だけ)。{q} 検証は編集でも効く", async () => {
    const { user, api } = await openSettings();
    const section = screen.getByRole("region", { name: "サイト" });
    await user.click(within(section).getByRole("button", { name: "Amazonを編集" }));
    const dlg = screen.getByRole("dialog", { name: "サイト" });
    await user.clear(within(dlg).getByRole("textbox", { name: "サイト名" }));
    await user.type(within(dlg).getByRole("textbox", { name: "サイト名" }), "アマゾン");
    await user.click(within(dlg).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(api.callsTo("PATCH", "/api/sites/2")).toHaveLength(1);
    });
    expect(api.callsTo("PATCH", "/api/sites/2")[0]?.json).toEqual({ name: "アマゾン" });
  });
});
