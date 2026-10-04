// ADR-0332(F-08 / I-web-7): 作り直した構築画面の主な流れ(docs/test-strategy.md「E2E」)。
// e2e/team.spec.ts・e2e/teamShowdown.spec.ts の置き換え(構築名の欄・[メンバーを追加]・行の[メンバーを編集]・
// 取り込みの構築名欄を使わない。古い spec は実装の PR で、この spec が通ってから外す。ADR-0332 §5)。
// team-svc は起動しない。`/api/team/**` は spec の中の fake(page.route)で受け、name の省略にはサーバーと同じく
// 既定名「名称未設定」を補う(ADR-0229)。マスタは pokedex フィクスチャ(既定のオンライン。種族は検索欄)。
// 確かめること:
//   - 新しい構築 → 6枠 → 1体目にポケモンを選ぶ → 技 → 保存(PUT に name が無い)→ 一覧に戻ると「構築 1」「1/6体」
//   - SP の合計 67 は保存できず、66 に戻すと保存できる
//   - 編集中にタブを往復しても下書きが残る(ADR-0308)
//   - メガ種族は持ち物がストーンに固定され、保存に載る
//   - Showdown 形式: 閉じた折りたたみに説明と入力例 → 貼り付け → 確認 → 作成(POST に name が無い)→ 一覧に「構築 1」
//   - 書き出しは開いた構築の折りたたみから

import { expect, test, type Locator, type Page, type Route } from "@playwright/test";
import { MEGA, SPECIES, combobox, openApp } from "./support/calcPage.ts";

const DEFAULT_NAME = "名称未設定";

interface FakeTeam {
  id: string;
  name: string;
  members: unknown[];
  createdAt: string;
  updatedAt: string;
}

interface TeamBackend {
  readonly teams: FakeTeam[];
  readonly postBodies: Record<string, unknown>[];
  readonly putBodies: Record<string, unknown>[];
}

const TEAM_PATH = /\/api\/team\/teams(?:\/([^/?]+))?$/;

/** 名前の省略・空・空白だけはサーバーの既定名にする(ADR-0229)。 */
function nameOrDefault(name: unknown): string {
  return typeof name === "string" && name.trim() !== "" ? name : DEFAULT_NAME;
}

async function installTeamBackend(page: Page): Promise<TeamBackend> {
  const backend: TeamBackend = { teams: [], postBodies: [], putBodies: [] };
  let sequence = 0;
  await page.route(TEAM_PATH, async (route: Route) => {
    const request = route.request();
    const id = TEAM_PATH.exec(new URL(request.url()).pathname)?.[1];
    const now = `2026-10-04T09:00:0${String(sequence)}Z`;
    const json = (status: number, body: unknown) =>
      route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
    if (request.method() === "GET" && id === undefined) {
      return json(200, backend.teams);
    }
    if (request.method() === "POST") {
      sequence += 1;
      const input = request.postDataJSON() as Record<string, unknown> & { members?: unknown[] };
      backend.postBodies.push(input);
      const created: FakeTeam = {
        id: `00000000-0000-4000-8000-00000000000${String(sequence)}`,
        name: nameOrDefault(input.name),
        members: input.members ?? [],
        createdAt: now,
        updatedAt: now,
      };
      backend.teams.unshift(created);
      return json(201, created);
    }
    if (request.method() === "PUT" && id !== undefined) {
      const input = request.postDataJSON() as Record<string, unknown> & { members?: unknown[] };
      backend.putBodies.push(input);
      const target = backend.teams.find((candidate) => candidate.id === id);
      if (target === undefined) {
        return json(404, { code: "not_found", message: "構築が見つかりません" });
      }
      target.name = nameOrDefault(input.name);
      target.members = input.members ?? [];
      target.updatedAt = now;
      return json(200, target);
    }
    return json(405, { code: "method_not_allowed", message: "対応していない操作です" });
  });
  return backend;
}

async function openTeamTab(page: Page): Promise<void> {
  await page.getByRole("tab", { name: "構築", exact: true }).click();
  await expect(page.getByRole("button", { name: "新しい構築", exact: true })).toBeVisible();
}

/** [新しい構築] → すぐ開く編集画面(表示名は「構築 1」)。 */
async function createTeam(page: Page): Promise<Locator> {
  await page.getByRole("button", { name: "新しい構築", exact: true }).click();
  const editor = page.getByRole("region", { name: "「構築 1」のメンバー編集", exact: true });
  await expect(editor).toBeVisible();
  return editor;
}

/** 枠の種族を検索欄で選び、欄が展開する(技1が出る)まで待つ。 */
async function selectSlotSpecies(slot: Locator, name: string): Promise<void> {
  const input = slot.getByRole("combobox", { name: "ポケモン", exact: true });
  await input.fill(name);
  await slot
    .getByRole("listbox", { name: "ポケモン", exact: true })
    .getByRole("option", { name, exact: true })
    .click();
  await expect(input).toHaveValue(name);
  await expect(slot.getByRole("combobox", { name: "技1", exact: true })).toBeVisible();
}

test("新しい構築 → ポケモンを選ぶ → 技 → 保存すると PUT に name が無く、一覧に「構築 1」で出る", async ({
  page,
}) => {
  const backend = await installTeamBackend(page);
  await openApp(page);
  await openTeamTab(page);
  const editor = await createTeam(page);

  expect(backend.postBodies).toEqual([{ members: [] }]);
  for (let position = 1; position <= 6; position += 1) {
    await expect(editor.getByRole("group", { name: `${String(position)}体目`, exact: true })).toBeVisible();
  }
  const empty = editor.getByRole("group", { name: "2体目", exact: true });
  await expect(empty.getByText("ポケモンを選ぶと、技・持ち物・特性などを決められます")).toBeVisible();
  await expect(empty.getByRole("combobox", { name: "技1", exact: true })).toHaveCount(0);

  const member = editor.getByRole("group", { name: "1体目", exact: true });
  await selectSlotSpecies(member, SPECIES.fire.nameJa);
  await member.getByRole("combobox", { name: "技1", exact: true }).selectOption({ index: 1 });
  await member.getByRole("textbox", { name: "SP A", exact: true }).fill("32");
  await member.getByRole("textbox", { name: "SP S", exact: true }).fill("32");
  await expect(member).toContainText("合計 64/66(残り 2)");
  await expect(editor.getByText("保存していない変更があります", { exact: true })).toBeVisible();

  await editor.getByRole("button", { name: "保存", exact: true }).click();
  await expect(editor.getByRole("status")).toContainText("保存しました");
  await expect(editor.getByText("保存していない変更があります", { exact: true })).toHaveCount(0);

  expect(backend.putBodies).toHaveLength(1);
  const body = backend.putBodies[0] as {
    members: { speciesKey: string; sp: Record<string, number>; moveIds: string[] }[];
  };
  expect(Object.keys(body)).toEqual(["members"]);
  expect(body.members).toHaveLength(1);
  expect(body.members[0]?.speciesKey).toBe(SPECIES.fire.key);
  expect(body.members[0]?.sp).toMatchObject({ atk: 32, spe: 32 });
  expect(body.members[0]?.moveIds).toHaveLength(1);

  await page.getByRole("button", { name: "一覧に戻る", exact: true }).click();
  const list = page.getByRole("list", { name: "保存した構築", exact: true });
  await expect(list).toContainText("構築 1");
  await expect(list).toContainText("1/6体");
  await expect(list).not.toContainText(DEFAULT_NAME);
  await expect(list.getByRole("button", { name: "「構築 1」を開く", exact: true })).toBeVisible();
});

test("SP の合計が 67 になると明示エラーで保存できず、66 に戻すと保存できる", async ({ page }) => {
  await installTeamBackend(page);
  await openApp(page);
  await openTeamTab(page);
  const editor = await createTeam(page);
  const member = editor.getByRole("group", { name: "1体目", exact: true });
  await selectSlotSpecies(member, SPECIES.water.nameJa);

  await member.getByRole("textbox", { name: "SP H", exact: true }).fill("3");
  await member.getByRole("textbox", { name: "SP A", exact: true }).fill("32");
  await member.getByRole("textbox", { name: "SP S", exact: true }).fill("32");
  await expect(member.getByRole("alert")).toContainText("合計が66を1超えています");
  await expect(editor.getByRole("button", { name: "保存", exact: true })).toBeDisabled();

  await member.getByRole("textbox", { name: "SP H", exact: true }).fill("2");
  await expect(member.getByRole("alert")).toHaveCount(0);
  await expect(editor.getByRole("button", { name: "保存", exact: true })).toBeEnabled();
});

test("編集中にタブを切り替えて戻っても、未保存の下書きが残る(ADR-0308)", async ({ page }) => {
  await installTeamBackend(page);
  await openApp(page);
  await openTeamTab(page);
  const editor = await createTeam(page);
  await selectSlotSpecies(editor.getByRole("group", { name: "3体目", exact: true }), SPECIES.water.nameJa);

  await page.getByRole("tab", { name: "計算", exact: true }).click();
  await expect(combobox(page, "攻撃側のポケモン")).toBeVisible();
  await page.getByRole("tab", { name: "構築", exact: true }).click();

  const restored = page
    .getByRole("region", { name: "「構築 1」のメンバー編集", exact: true })
    .getByRole("group", { name: "3体目", exact: true });
  await expect(restored.getByRole("combobox", { name: "ポケモン", exact: true })).toHaveValue(
    SPECIES.water.nameJa,
  );
  await expect(page.getByText("保存していない変更があります", { exact: true })).toBeVisible();
});

test("メガ種族を選ぶと持ち物がメガストーンに固定され、保存に載る", async ({ page }) => {
  const backend = await installTeamBackend(page);
  await openApp(page);
  await openTeamTab(page);
  const editor = await createTeam(page);
  const member = editor.getByRole("group", { name: "1体目", exact: true });
  await selectSlotSpecies(member, MEGA.fire.nameJa);

  const item = member.getByRole("combobox", { name: "持ち物", exact: true });
  await expect(item).toBeDisabled();
  await expect(item.locator("option:checked")).toHaveText(MEGA.fire.lockedLabel);
  await expect(member.getByText(MEGA.lockedReason)).toBeVisible();
  await expect(item).toHaveAccessibleDescription(MEGA.lockedReason);

  await editor.getByRole("button", { name: "保存", exact: true }).click();
  await expect(editor.getByRole("status")).toContainText("保存しました");
  const body = backend.putBodies[0] as { members: { itemId: string | null }[] };
  expect(body.members[0]?.itemId).toBe("examplemegastonefire");

  // 非メガに変えると固定が解除される。
  await selectSlotSpecies(member, SPECIES.fire.nameJa);
  await expect(item).toBeEnabled();
  await expect(item).toHaveValue("");
  await expect(item.getByRole("option", { name: MEGA.fire.stoneNameJa })).toHaveCount(0);
});

const IMPORT_TEXT = [
  `${SPECIES.fire.nameJa} @ テストぼうぎょだま`,
  "Ability: テストむこう",
  "EVs: 2 HP / 32 Atk / 32 Spe",
  "テストいじっぱり Nature",
  "- テストたいあたり",
].join("\n");

test("Showdown 形式: 閉じた折りたたみに説明と入力例。貼り付けて確認 → 作成すると POST に name が無く、一覧に出る", async ({
  page,
}) => {
  const backend = await installTeamBackend(page);
  await openApp(page);
  await openTeamTab(page);

  const region = page.getByRole("region", { name: "Showdown 形式から取り込む", exact: true });
  await expect(region).toBeHidden();
  await page.getByText("Showdown 形式で取り込む", { exact: true }).click();
  await expect(region).toBeVisible();
  await expect(
    page.getByText(
      "Pokémon Showdown などで作った構築のテキストを貼り付けると、新しい構築として取り込めます。ポケモン・持ち物・特性・技は日本語の名前で書き、ポケモンごとに空の行で区切ります",
      { exact: true },
    ),
  ).toBeVisible();
  await expect(page.getByText("入力の例(1体分)", { exact: true })).toBeVisible();
  const fold = page.locator("details").filter({ hasText: "Showdown 形式で取り込む" });
  await expect(fold.locator("pre")).toContainText("Ability:");
  await expect(fold.locator("pre")).toContainText("EVs:");
  await expect(region.getByRole("textbox", { name: /構築名/ })).toHaveCount(0);

  await region.getByRole("textbox", { name: "取り込むテキスト", exact: true }).fill(IMPORT_TEXT);
  await region.getByRole("button", { name: "内容を確認", exact: true }).click();
  await expect(region.getByRole("status")).toContainText("1体を取り込めます");
  expect(backend.postBodies).toHaveLength(0);

  await region.getByRole("button", { name: "この内容で作成", exact: true }).click();
  await expect(region.getByRole("status")).toContainText("1体の構築を作りました");
  const list = page.getByRole("list", { name: "保存した構築", exact: true });
  await expect(list).toContainText("構築 1");
  await expect(list).toContainText("1/6体");

  expect(backend.postBodies).toHaveLength(1);
  const body = backend.postBodies[0] as {
    members: { speciesKey: string; itemId: string | null; moveIds: string[]; sp: Record<string, number> }[];
  };
  expect(Object.keys(body)).toEqual(["members"]);
  expect(body.members[0]?.speciesKey).toBe(SPECIES.fire.key);
  expect(body.members[0]?.itemId).toBe("exampleitemdef");
  expect(body.members[0]?.moveIds).toEqual(["examplemovetackle"]);
  expect(body.members[0]?.sp).toMatchObject({ hp: 2, atk: 32, spe: 32 });
});

test("書き出しは開いた構築の折りたたみから。読み取り専用の textarea に Showdown 形式が出る", async ({
  page,
}) => {
  const backend = await installTeamBackend(page);
  backend.teams.push({
    id: "00000000-0000-4000-8000-0000000000aa",
    name: DEFAULT_NAME,
    members: [
      {
        speciesKey: SPECIES.fire.key,
        moveIds: ["examplemovetackle"],
        itemId: "exampleitemdef",
        abilityId: "exampleabilitynone",
        natureId: "example-nature-atk",
        sp: { hp: 2, atk: 32, def: 0, spa: 0, spd: 0, spe: 32 },
        teraType: null,
      },
    ],
    createdAt: "2026-10-03T09:00:00Z",
    updatedAt: "2026-10-03T09:00:00Z",
  });
  await openApp(page);
  await openTeamTab(page);

  await page.getByRole("button", { name: "「構築 1」を開く", exact: true }).click();
  await page.getByText("Showdown 形式で書き出す", { exact: true }).click();
  await page.getByRole("button", { name: "「構築 1」を Showdown 形式で書き出す", exact: true }).click();
  const textarea = page.getByRole("textbox", { name: "「構築 1」の書き出しテキスト", exact: true });
  await expect(textarea).toHaveAttribute("readonly", "");
  await expect(textarea).toHaveValue(new RegExp(`^${SPECIES.fire.nameJa} @ テストぼうぎょだま`));
  await expect(textarea).toHaveValue(/Ability: テストむこう/);
  await expect(textarea).toHaveValue(/EVs: 2 HP \/ 32 Atk \/ 32 Spe/);
  await expect(textarea).toHaveValue(/- テストたいあたり/);
  await expect(textarea).not.toHaveValue(new RegExp(SPECIES.fire.key));
});

test("取り込めない項目は日本語の問題として出て、取り込める分だけ作れる", async ({ page }) => {
  await installTeamBackend(page);
  await openApp(page);
  await openTeamTab(page);
  await page.getByText("Showdown 形式で取り込む", { exact: true }).click();
  const region = page.getByRole("region", { name: "Showdown 形式から取り込む", exact: true });
  await region
    .getByRole("textbox", { name: "取り込むテキスト", exact: true })
    .fill(`${IMPORT_TEXT}\n\nふめいなもん\nテストいじっぱり Nature`);
  await region.getByRole("button", { name: "内容を確認", exact: true }).click();
  await expect(region.getByRole("alert")).toContainText("2体目");
  await expect(region.getByRole("alert")).toContainText("ふめいなもん");
  await expect(region.getByText("1体を取り込めます", { exact: true })).toBeVisible();
});
