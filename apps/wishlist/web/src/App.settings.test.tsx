import { screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { makeGenre } from "./test/factories";
import { openHome } from "./test/render";
import { scenario } from "./test/scenario";

async function openSettings(over: Parameters<typeof openHome>[0] = {}) {
  const r = await openHome(over);
  await r.user.click(screen.getByRole("button", { name: "設定" }));
  return r;
}

// AC-SET-*: 設定
describe("設定", () => {
  // 旧 AC-SET-01(トークン未設定で設定画面から始まる)・AC-SET-02(API の URL とトークンを保存)は、
  // トークンと接続先の設定を無くした(ユーザー決定 2026-10-11 / phase5-spec 5-1a・5-2)ので、AC-SET-01(欄が無い)に置き換え、
  // 旧 AC-SET-02 の「保存した設定が以降の呼び出しに使われる」は AC-5-2a-2(保存済みの設定を読まない)に置き換えた。
  // 初回起動でホームが出ること・Authorization が付かないことは App.noauth.test.tsx。
  it("AC-SET-01 設定画面にトークンと API の URL の欄が無い(「接続」の区画も無い)", async () => {
    await openSettings();
    expect(screen.queryByLabelText("トークン")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("APIのURL")).not.toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "接続" })).not.toBeInTheDocument();
    expect(screen.queryByDisplayValue("test-token")).not.toBeInTheDocument();
    expect(screen.getByRole("region", { name: "ジャンル" })).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "サイト" })).toBeInTheDocument();
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

// AC-5-2l-*: ジャンルの設定(絵柄・色・件数・URL の手がかり。phase5-spec 5-2「設定」「ジャンルの編集」)
const COLOR_NAMES = ["赤", "朱", "橙", "黄", "黄緑", "緑", "青緑", "水色", "青", "藍", "紫", "桃"];
const ICON_NAMES = ["カード", "フィギュア", "ロボ", "グッズ", "星", "箱"];
const pressedName = (group: HTMLElement) =>
  within(group)
    .getAllByRole("button")
    .filter((b) => b.getAttribute("aria-pressed") === "true")
    .map((b) => b.getAttribute("aria-label"));

async function openGenreEdit(name: string, over: Parameters<typeof openHome>[0] = {}) {
  const r = await openSettings(over);
  const section = screen.getByRole("region", { name: "ジャンル" });
  await r.user.click(within(section).getByRole("button", { name: `${name}を編集` }));
  return { ...r, section, dlg: screen.getByRole("dialog", { name: "ジャンル" }) };
}

describe("設定: ジャンル(5-2)", () => {
  it("AC-5-2l-1 ジャンルの行は 絵柄(data-icon)・ジャンル色・名前・件数(そのジャンルの商品数)", async () => {
    await openSettings();
    const rows = within(screen.getByRole("region", { name: "ジャンル" })).getAllByRole("listitem");
    expect(rows).toHaveLength(2); // sort_order 順 = S.H.Figuarts, デュエマ
    const fig = rows[0];
    const dm = rows[1];
    if (!fig || !dm) throw new Error("no rows");
    expect(fig).toHaveTextContent("S.H.Figuarts");
    expect(fig.querySelector("[data-icon='figure']")).not.toBeNull();
    expect(within(fig).getByTestId("genre-icon")).toHaveAttribute("data-genre-color", "#7c4dff");
    expect(within(fig).getByTestId("genre-count")).toHaveTextContent("2"); // グリス・ビルド
    expect(dm.querySelector("[data-icon='card']")).not.toBeNull();
    expect(within(dm).getByTestId("genre-icon")).toHaveAttribute("data-genre-color", "#e0393e");
    expect(within(dm).getByTestId("genre-count")).toHaveTextContent("1");
  });

  it("AC-5-2l-2 色の見本は 12 色。いまの色が押されていて、選ぶとホームでの見え方(プレビュー)の枠の色が変わる。保存は color だけ PATCH", async () => {
    const { dlg, user, api } = await openGenreEdit("S.H.Figuarts");
    const swatches = within(dlg).getByRole("group", { name: "色" });
    expect(
      within(swatches)
        .getAllByRole("button")
        .map((b) => b.getAttribute("aria-label")),
    ).toEqual(COLOR_NAMES);
    expect(pressedName(swatches)).toEqual(["紫"]); // #7c4dff
    expect(within(dlg).getByTestId("genre-preview")).toHaveAttribute("data-genre-color", "#7c4dff");

    await user.click(within(swatches).getByRole("button", { name: "赤" }));
    expect(pressedName(swatches)).toEqual(["赤"]);
    expect(within(dlg).getByTestId("genre-preview")).toHaveAttribute("data-genre-color", "#e0393e");
    await user.click(within(dlg).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(api.callsTo("PATCH", "/api/genres/2")).toHaveLength(1);
    });
    expect(api.callsTo("PATCH", "/api/genres/2")[0]?.json).toEqual({ color: "#e0393e" });
    // 反映: ホームに戻るとタイルの枠の色が変わる
    await user.click(screen.getByRole("button", { name: "戻る" }));
    expect(await screen.findByRole("button", { name: "グリス" })).toHaveAttribute(
      "data-genre-color",
      "#e0393e",
    );
  });

  it("AC-5-2l-3 絵柄は 6 つ(カード・フィギュア・ロボ・グッズ・星・箱)。選ぶと icon だけ PATCH", async () => {
    const { dlg, user, api } = await openGenreEdit("S.H.Figuarts");
    const icons = within(dlg).getByRole("group", { name: "絵柄" });
    expect(
      within(icons)
        .getAllByRole("button")
        .map((b) => b.getAttribute("aria-label")),
    ).toEqual(ICON_NAMES);
    expect(pressedName(icons)).toEqual(["フィギュア"]);
    await user.click(within(icons).getByRole("button", { name: "ロボ" }));
    expect(pressedName(icons)).toEqual(["ロボ"]);
    await user.click(within(dlg).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(api.callsTo("PATCH", "/api/genres/2")).toHaveLength(1);
    });
    expect(api.callsTo("PATCH", "/api/genres/2")[0]?.json).toEqual({ icon: "robot" });
  });

  it("AC-5-2l-4 URL の手がかり: ホストと語をカンマか改行で区切って入力し、全件置き換えで PATCH(前後の空白と空の項目は除く)", async () => {
    const { dlg, user, api } = await openGenreEdit("S.H.Figuarts");
    expect(within(dlg).getByRole("textbox", { name: "手がかりのホスト" })).toHaveValue("");
    await user.type(
      within(dlg).getByRole("textbox", { name: "手がかりのホスト" }),
      "tamashiiweb.com, bandai-hobby.net{enter} ,{enter}example.com",
    );
    await user.type(
      within(dlg).getByRole("textbox", { name: "手がかりの語" }),
      "S.H.Figuarts,フィギュアーツ{enter}SHF",
    );
    await user.click(within(dlg).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(api.callsTo("PATCH", "/api/genres/2")).toHaveLength(1);
    });
    expect(api.callsTo("PATCH", "/api/genres/2")[0]?.json).toEqual({
      match_hosts: ["tamashiiweb.com", "bandai-hobby.net", "example.com"],
      match_keywords: ["S.H.Figuarts", "フィギュアーツ", "SHF"],
    });
  });

  it("AC-5-2l-4 既にある手がかりを編集欄に出す。全部消して保存すると空配列(全件置き換え)。触らなければ送らない", async () => {
    const s = scenario();
    s.genres = [
      makeGenre({
        id: 2,
        name: "S.H.Figuarts",
        sort_order: 1,
        site_ids: [2, 1, 3],
        color: "#7c4dff",
        icon: "figure",
        match_hosts: ["tamashiiweb.com"],
        match_keywords: ["S.H.Figuarts", "フィギュアーツ"],
      }),
    ];
    s.items = s.items.filter((i) => i.genre_id === 2);
    const { dlg, user, api } = await openGenreEdit("S.H.Figuarts", s);
    expect(within(dlg).getByRole("textbox", { name: "手がかりのホスト" })).toHaveDisplayValue(
      /tamashiiweb\.com/,
    );
    const kw = within(dlg).getByRole("textbox", { name: "手がかりの語" });
    expect(kw).toHaveDisplayValue(/S\.H\.Figuarts/);
    expect(kw).toHaveDisplayValue(/フィギュアーツ/);
    // 触らずに保存 → 何も送らない
    await user.click(within(dlg).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(screen.queryByRole("dialog", { name: "ジャンル" })).not.toBeInTheDocument();
    });
    expect(api.callsTo("PATCH", "/api/genres/2")).toHaveLength(0);

    await user.click(
      within(screen.getByRole("region", { name: "ジャンル" })).getByRole("button", {
        name: "S.H.Figuartsを編集",
      }),
    );
    const dlg2 = screen.getByRole("dialog", { name: "ジャンル" });
    await user.clear(within(dlg2).getByRole("textbox", { name: "手がかりのホスト" }));
    await user.click(within(dlg2).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(api.callsTo("PATCH", "/api/genres/2")).toHaveLength(1);
    });
    expect(api.callsTo("PATCH", "/api/genres/2")[0]?.json).toEqual({ match_hosts: [] });
  });

  it("AC-5-2l-5 ジャンルの追加: 何も選ばなければ #1d5fd1・箱・手がかり空で POST。色と絵柄を選べばその値", async () => {
    const { user, api } = await openSettings();
    const section = screen.getByRole("region", { name: "ジャンル" });
    await user.click(within(section).getByRole("button", { name: "ジャンルを追加" }));
    let dlg = screen.getByRole("dialog", { name: "ジャンル" });
    // 初期値は強調色 #1d5fd1。12 色の見本には無い色なので、どの見本も押されず、プレビューだけがその色
    expect(pressedName(within(dlg).getByRole("group", { name: "色" }))).toEqual([]);
    expect(within(dlg).getByTestId("genre-preview")).toHaveAttribute("data-genre-color", "#1d5fd1");
    await user.type(within(dlg).getByRole("textbox", { name: "ジャンル名" }), "その他グッズ");
    await user.click(within(dlg).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(api.callsTo("POST", "/api/genres")).toHaveLength(1);
    });
    expect(api.callsTo("POST", "/api/genres")[0]?.json).toMatchObject({
      name: "その他グッズ",
      color: "#1d5fd1",
      icon: "box",
      match_hosts: [],
      match_keywords: [],
    });

    await user.click(within(section).getByRole("button", { name: "ジャンルを追加" }));
    dlg = screen.getByRole("dialog", { name: "ジャンル" });
    await user.type(within(dlg).getByRole("textbox", { name: "ジャンル名" }), "ポケモン");
    await user.click(
      within(within(dlg).getByRole("group", { name: "色" })).getByRole("button", { name: "黄" }),
    );
    await user.click(
      within(within(dlg).getByRole("group", { name: "絵柄" })).getByRole("button", { name: "グッズ" }),
    );
    await user.click(within(dlg).getByRole("button", { name: "保存" }));
    await waitFor(() => {
      expect(api.callsTo("POST", "/api/genres")).toHaveLength(2);
    });
    expect(api.callsTo("POST", "/api/genres")[1]?.json).toMatchObject({
      name: "ポケモン",
      color: "#f2b705",
      icon: "goods",
    });
  });
});
