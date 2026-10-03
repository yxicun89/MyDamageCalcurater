// P5-5e(ADR-0321): 構築の Showdown 形式の取り込み・書き出しの主な流れ(docs/test-strategy.md「E2E」)。
// team-svc は起動しない。`/api/team/**` は spec の中の fake(page.route。構築の保存先はこの配列だけ)、
// マスタは pokedex フィクスチャ(既定のオンライン。種族・特性・技は検索/解決 API で都度引く)を使う。
// 確かめること: 貼り付けて取り込む → 一覧に出る(POST の members に種族・持ち物・技・SP が載る) /
//   書き出す → 読み取り専用 textarea に Showdown 形式のテキストが出る / 取り込み途中でタブを往復しても入力が残る(ADR-0308)。

import { expect, test, type Page, type Route } from "@playwright/test";
import { SPECIES, openApp } from "./support/calcPage.ts";

interface FakeTeam {
  id: string;
  name: string;
  members: unknown[];
  createdAt: string;
  updatedAt: string;
}

interface TeamBackend {
  readonly teams: FakeTeam[];
  readonly postBodies: unknown[];
}

const TEAM_PATH = /\/api\/team\/teams(?:\/([^/?]+))?$/;

async function installTeamBackend(page: Page): Promise<TeamBackend> {
  const backend: TeamBackend = { teams: [], postBodies: [] };
  let sequence = 0;
  await page.route(TEAM_PATH, async (route: Route) => {
    const request = route.request();
    const now = "2026-10-03T09:00:00Z";
    const json = (status: number, body: unknown) =>
      route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
    if (request.method() === "GET") {
      return json(200, backend.teams);
    }
    if (request.method() === "POST") {
      sequence += 1;
      const input = request.postDataJSON() as { name: string; members?: unknown[] };
      backend.postBodies.push(input);
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
    return json(405, { code: "method_not_allowed", message: "対応していない操作です" });
  });
  return backend;
}

async function openTeamTab(page: Page): Promise<void> {
  await page.getByRole("tab", { name: "構築", exact: true }).click();
  await expect(page.getByRole("textbox", { name: "構築名", exact: true })).toBeVisible();
}

const IMPORT_TEXT = [
  `${SPECIES.fire.nameJa} @ テストぼうぎょだま`,
  "Ability: テストむこう",
  "EVs: 2 HP / 32 Atk / 32 Spe",
  "テストいじっぱり Nature",
  "- テストたいあたり",
].join("\n");

test("Showdown 形式を貼り付けて取り込むと、構築が作られて一覧に出る(確認 → 作成の2段階)", async ({
  page,
}) => {
  const backend = await installTeamBackend(page);
  await openApp(page);
  await openTeamTab(page);

  const region = page.getByRole("region", { name: "Showdown 形式から取り込む", exact: true });
  await region.getByRole("textbox", { name: "取り込むテキスト", exact: true }).fill(IMPORT_TEXT);
  await region.getByRole("textbox", { name: "取り込む構築名", exact: true }).fill("取り込み構築");
  await region.getByRole("button", { name: "内容を確認", exact: true }).click();
  await expect(region.getByRole("status")).toContainText("1体を取り込めます");
  expect(backend.postBodies).toHaveLength(0);

  await region.getByRole("button", { name: "この内容で作成", exact: true }).click();
  await expect(page.getByRole("list", { name: "保存した構築", exact: true })).toContainText("取り込み構築");
  await expect(page.getByRole("list", { name: "保存した構築", exact: true })).toContainText("1/6体");

  expect(backend.postBodies).toHaveLength(1);
  const body = backend.postBodies[0] as {
    name: string;
    members: { speciesKey: string; itemId: string | null; moveIds: string[]; sp: Record<string, number> }[];
  };
  expect(body.name).toBe("取り込み構築");
  expect(body.members).toHaveLength(1);
  expect(body.members[0]?.speciesKey).toBe(SPECIES.fire.key);
  expect(body.members[0]?.itemId).toBe("exampleitemdef");
  expect(body.members[0]?.moveIds).toEqual(["examplemovetackle"]);
  expect(body.members[0]?.sp).toMatchObject({ hp: 2, atk: 32, spe: 32 });
});

test("取り込めない項目は日本語の問題として出て、取り込める分だけ作れる", async ({ page }) => {
  await installTeamBackend(page);
  await openApp(page);
  await openTeamTab(page);
  const region = page.getByRole("region", { name: "Showdown 形式から取り込む", exact: true });
  await region
    .getByRole("textbox", { name: "取り込むテキスト", exact: true })
    .fill(`${IMPORT_TEXT}\n\nふめいなもん\nテストいじっぱり Nature`);
  await region.getByRole("textbox", { name: "取り込む構築名", exact: true }).fill("一部だけ");
  await region.getByRole("button", { name: "内容を確認", exact: true }).click();
  await expect(region.getByRole("alert")).toContainText("2体目");
  await expect(region.getByRole("alert")).toContainText("ふめいなもん");
  await expect(region.getByText("1体を取り込めます", { exact: true })).toBeVisible();
});

test("構築を Showdown 形式で書き出すと、読み取り専用の textarea にテキストが出る", async ({ page }) => {
  const backend = await installTeamBackend(page);
  backend.teams.push({
    id: "00000000-0000-4000-8000-0000000000aa",
    name: "出力構築",
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

  await page.getByRole("button", { name: "「出力構築」を Showdown 形式で書き出す", exact: true }).click();
  const textarea = page.getByRole("textbox", { name: "「出力構築」の書き出しテキスト", exact: true });
  await expect(textarea).toHaveAttribute("readonly", "");
  await expect(textarea).toHaveValue(new RegExp(`^${SPECIES.fire.nameJa} @ テストぼうぎょだま`));
  await expect(textarea).toHaveValue(/Ability: テストむこう/);
  await expect(textarea).toHaveValue(/EVs: 2 HP \/ 32 Atk \/ 32 Spe/);
  await expect(textarea).toHaveValue(/- テストたいあたり/);
  await expect(textarea).not.toHaveValue(new RegExp(SPECIES.fire.key));
});

test("取り込みの入力はタブを往復しても残る(ADR-0308)", async ({ page }) => {
  await installTeamBackend(page);
  await openApp(page);
  await openTeamTab(page);
  const region = page.getByRole("region", { name: "Showdown 形式から取り込む", exact: true });
  await region.getByRole("textbox", { name: "取り込むテキスト", exact: true }).fill(IMPORT_TEXT);
  await region.getByRole("textbox", { name: "取り込む構築名", exact: true }).fill("保持");

  await page.getByRole("tab", { name: "計算", exact: true }).click();
  await page.getByRole("tab", { name: "構築", exact: true }).click();

  await expect(region.getByRole("textbox", { name: "取り込むテキスト", exact: true })).toHaveValue(
    IMPORT_TEXT,
  );
  await expect(region.getByRole("textbox", { name: "取り込む構築名", exact: true })).toHaveValue("保持");
});
