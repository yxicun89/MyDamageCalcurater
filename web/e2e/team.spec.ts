// P5-5b PR-A2(ADR-0316): 構築のメンバー編集の主な流れ(docs/test-strategy.md「E2E」)。
// team-svc は起動しない。`/api/team/**` は spec の中の fake で受け(page.route。構築の保存先はこの配列だけ)、
// マスタは架空の例データ(オフライン。種族は <select>)を使う。
// 確かめること: 構築を作る → 編集領域を開く → メンバーを追加して種族・技・SP を入れる → 保存すると
// PUT に全置換で送られ、一覧のメンバー数が変わる / SP 合計 67 は保存できない / タブを切り替えても下書きが残る(ADR-0308)。

import { expect, test, type Page, type Route, type Locator } from "@playwright/test";
import { MEGA, SPECIES, combobox, openApp } from "./support/calcPage.ts";

interface FakeTeam {
  id: string;
  name: string;
  members: unknown[];
  createdAt: string;
  updatedAt: string;
}

interface TeamBackend {
  readonly teams: FakeTeam[];
  readonly putBodies: unknown[];
}

const TEAM_PATH = /\/api\/team\/teams(?:\/([^/?]+))?$/;

/** team-svc の最小の fake(list / create / update)。応答の形は api/openapi.yaml の Team。 */
async function installTeamBackend(page: Page): Promise<TeamBackend> {
  const backend: TeamBackend = { teams: [], putBodies: [] };
  let sequence = 0;
  await page.route(TEAM_PATH, async (route: Route) => {
    const request = route.request();
    const id = TEAM_PATH.exec(new URL(request.url()).pathname)?.[1];
    const now = "2026-10-02T09:00:00Z";
    const json = (status: number, body: unknown) =>
      route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
    if (request.method() === "GET" && id === undefined) {
      return json(200, backend.teams);
    }
    if (request.method() === "POST") {
      sequence += 1;
      const input = request.postDataJSON() as { name: string; members?: unknown[] };
      const created: FakeTeam = {
        id: `00000000-0000-4000-8000-00000000000${String(sequence)}`,
        name: input.name,
        members: input.members ?? [],
        createdAt: now,
        updatedAt: now,
      };
      backend.teams.unshift(created);
      return json(201, created);
    }
    if (request.method() === "PUT" && id !== undefined) {
      const input = request.postDataJSON() as { name: string; members?: unknown[] };
      backend.putBodies.push(input);
      const target = backend.teams.find((candidate) => candidate.id === id);
      if (target === undefined) {
        return json(404, { code: "not_found", message: "構築が見つかりません" });
      }
      target.name = input.name;
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
  await expect(page.getByRole("textbox", { name: "構築名", exact: true })).toBeVisible();
}

async function createTeam(page: Page, name: string): Promise<void> {
  await page.getByRole("textbox", { name: "構築名", exact: true }).fill(name);
  await page.getByRole("button", { name: "作成", exact: true }).click();
  await expect(page.getByRole("list", { name: "保存した構築", exact: true })).toContainText(name);
}

/** メンバーの種族を検索欄で選ぶ(ADR-0313: 既定がオンラインになり、種族は select ではなく検索欄。speciesList が false)。 */
async function selectMemberSpecies(member: Locator, name: string): Promise<void> {
  const input = member.getByRole("combobox", { name: "ポケモン", exact: true });
  await expect(input).toHaveAttribute("aria-expanded", /^(true|false)$/);
  await input.fill(name);
  await member
    .getByRole("listbox", { name: "ポケモン", exact: true })
    .getByRole("option", { name, exact: true })
    .click();
  await expect(input).toHaveValue(name);
}

test("メンバーを追加して種族・技・SP を入れて保存すると、PUT に全置換で送られ一覧のメンバー数が変わる", async ({
  page,
}) => {
  const backend = await installTeamBackend(page);
  await openApp(page);
  await openTeamTab(page);
  await createTeam(page, "E2E構築");

  await page.getByRole("button", { name: "「E2E構築」のメンバーを編集", exact: true }).click();
  const editor = page.getByRole("region", { name: "「E2E構築」のメンバー編集", exact: true });
  await editor.getByRole("button", { name: "メンバーを追加", exact: true }).click();
  const member = editor.getByRole("group", { name: "1体目", exact: true });

  // 種族を選ぶまでは保存できない。
  await expect(editor.getByRole("button", { name: "メンバーを保存", exact: true })).toBeDisabled();
  await selectMemberSpecies(member, SPECIES.fire.nameJa);
  await member.getByRole("combobox", { name: "技1", exact: true }).selectOption({ index: 1 });
  await member.getByRole("textbox", { name: "SP A", exact: true }).fill("32");
  await member.getByRole("textbox", { name: "SP S", exact: true }).fill("32");
  await expect(member).toContainText("合計 64/66(残り 2)");

  await editor.getByRole("button", { name: "メンバーを保存", exact: true }).click();
  await expect(editor.getByRole("status")).toContainText("保存しました");
  await expect(page.getByRole("list", { name: "保存した構築", exact: true })).toContainText("1/6体");

  expect(backend.putBodies).toHaveLength(1);
  const body = backend.putBodies[0] as {
    name: string;
    members: { speciesKey: string; sp: Record<string, number>; moveIds: string[] }[];
  };
  expect(body.name).toBe("E2E構築");
  expect(body.members).toHaveLength(1);
  expect(body.members[0]?.speciesKey).toBe(SPECIES.fire.key);
  expect(body.members[0]?.sp).toMatchObject({ atk: 32, spe: 32 });
  expect(body.members[0]?.moveIds).toHaveLength(1);
});

test("SP の合計が 67 になると明示エラーで保存できず、66 に戻すと保存できる", async ({ page }) => {
  await installTeamBackend(page);
  await openApp(page);
  await openTeamTab(page);
  await createTeam(page, "SP構築");
  await page.getByRole("button", { name: "「SP構築」のメンバーを編集", exact: true }).click();
  const editor = page.getByRole("region", { name: "「SP構築」のメンバー編集", exact: true });
  await editor.getByRole("button", { name: "メンバーを追加", exact: true }).click();
  const member = editor.getByRole("group", { name: "1体目", exact: true });
  await selectMemberSpecies(member, SPECIES.water.nameJa);

  await member.getByRole("textbox", { name: "SP H", exact: true }).fill("3");
  await member.getByRole("textbox", { name: "SP A", exact: true }).fill("32");
  await member.getByRole("textbox", { name: "SP S", exact: true }).fill("32");
  await expect(member.getByRole("alert")).toContainText("合計が66を1超えています");
  await expect(editor.getByRole("button", { name: "メンバーを保存", exact: true })).toBeDisabled();

  await member.getByRole("textbox", { name: "SP H", exact: true }).fill("2");
  await expect(member.getByRole("alert")).toHaveCount(0);
  await expect(editor.getByRole("button", { name: "メンバーを保存", exact: true })).toBeEnabled();
});

test("編集中にタブを切り替えて戻っても、未保存の下書きが残る(ADR-0308)", async ({ page }) => {
  await installTeamBackend(page);
  await openApp(page);
  await openTeamTab(page);
  await createTeam(page, "保持構築");
  await page.getByRole("button", { name: "「保持構築」のメンバーを編集", exact: true }).click();
  const editor = page.getByRole("region", { name: "「保持構築」のメンバー編集", exact: true });
  await editor.getByRole("button", { name: "メンバーを追加", exact: true }).click();
  await selectMemberSpecies(editor.getByRole("group", { name: "1体目", exact: true }), SPECIES.water.nameJa);

  await page.getByRole("tab", { name: "計算", exact: true }).click();
  await expect(combobox(page, "攻撃側のポケモン")).toBeVisible();
  await page.getByRole("tab", { name: "構築", exact: true }).click();

  const restored = page
    .getByRole("region", { name: "「保持構築」のメンバー編集", exact: true })
    .getByRole("group", { name: "1体目", exact: true });
  // 種族は検索欄(ADR-0313)なので、入力欄の値で確かめる。
  await expect(restored.getByRole("combobox", { name: "ポケモン", exact: true })).toHaveValue(
    SPECIES.water.nameJa,
  );
});

// issue #515・ADR-0320 PR-B: 構築でメガ種族を選ぶと持ち物がメガストーンに固定され、保存の PUT にストーンが載る。
test("メンバーでメガ種族を選ぶと持ち物がメガストーンに固定され、保存に載り、非メガに変えると解除される", async ({
  page,
}) => {
  const backend = await installTeamBackend(page);
  await openApp(page);
  await openTeamTab(page);
  await createTeam(page, "メガ構築");
  await page.getByRole("button", { name: "「メガ構築」のメンバーを編集", exact: true }).click();
  const editor = page.getByRole("region", { name: "「メガ構築」のメンバー編集", exact: true });
  await editor.getByRole("button", { name: "メンバーを追加", exact: true }).click();
  const member = editor.getByRole("group", { name: "1体目", exact: true });
  const item = member.getByRole("combobox", { name: "持ち物", exact: true });

  await selectMemberSpecies(member, MEGA.fire.nameJa);
  await expect(item).toBeDisabled();
  await expect(item.locator("option:checked")).toHaveText(MEGA.fire.stoneNameJa);
  await expect(member.getByText(MEGA.lockedReason)).toBeVisible();
  await expect(item).toHaveAccessibleDescription(MEGA.lockedReason);

  await editor.getByRole("button", { name: "メンバーを保存", exact: true }).click();
  await expect(editor.getByRole("status")).toContainText("保存しました");
  const body = backend.putBodies[0] as { members: { itemId: string | null }[] };
  expect(body.members[0]?.itemId).toBe("examplemegastonefire");

  await selectMemberSpecies(member, SPECIES.fire.nameJa);
  await expect(item).toBeEnabled();
  await expect(item).toHaveValue("");
  await expect(item.getByRole("option", { name: MEGA.fire.stoneNameJa })).toHaveCount(0);
});
