// I-speed-4b(ADR-0610 §4・Web 画面の具体): 素早さ画面を実ブラウザ(chromium)で確かめる。
// 旧 F-06 の仮想スクロール・位置マーカーの e2e(ADR-0608)は、表が画面から外れたので置き換えた。
// speed-svc はこの構成では動いていないので、speed API は page.route で架空の応答(9xxx の架空ポケモン)に差し替える。
// jsdom にはレイアウトが無いので、ここでは「実ブラウザでしか分からないこと」(操作の通り道・はみ出し・キーボード)を見る。
// 値の細かい表示規則は web/src/speed/SpeedScreen.test.tsx(コンポーネントテスト)が守る。

import { expect, test, type Page } from "@playwright/test";

const BIRD = { pokemonId: "9001-000", nameJa: "テストカソウドリ", types: ["fire", "flying"], baseSpeed: 100 };
const FISH = { pokemonId: "9002-000", nameJa: "テストカソウギョ", types: ["water"], baseSpeed: 80 };

interface Captured {
  matchup: unknown[];
  reach: unknown[];
  table: number;
  position: number;
}

/** 通常の場: 最速は上昇 S12 以上で抜け、最速スカーフは届かない。 */
function matchupFor(trickRoom: boolean) {
  const lines = trickRoom
    ? [
        {
          assumption: { preset: "max" },
          opponentSpeed: 145,
          natures: [
            { nature: "minus", movesFirst: { min: 0, max: 32 } },
            { nature: "neutral", movesFirst: { min: 0, max: 28 }, tie: { min: 29, max: 29 } },
            { nature: "plus", movesFirst: { min: 0, max: 20 } },
          ],
        },
      ]
    : [
        {
          assumption: { preset: "max" },
          opponentSpeed: 145,
          current: "opponent_first",
          natures: [
            { nature: "minus" },
            { nature: "neutral" },
            { nature: "plus", movesFirst: { min: 12, max: 32 }, tie: { min: 11, max: 11 } },
          ],
        },
        {
          assumption: { preset: "max-scarf" },
          opponentSpeed: 217,
          natures: [{ nature: "minus" }, { nature: "neutral" }, { nature: "plus" }],
        },
      ];
  return { self: BIRD, opponent: FISH, trickRoom, selfSpeed: 140, lines };
}

const REACH = {
  speed: 140,
  pokemon: BIRD,
  lines: [
    {
      preset: "uninvested",
      slower: { min: 1, max: 119 },
      faster: { min: 120, max: 255 },
      slowerCount: 20,
      tieCount: 0,
      fasterCount: 22,
      fastestSlower: FISH,
      slowestFaster: BIRD,
    },
    ...(["neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"] as const).map((preset) => ({
      preset,
      slower: { min: 1, max: 60 },
      faster: { min: 61, max: 255 },
      slowerCount: 5,
      tieCount: 0,
      fasterCount: 37,
      fastestSlower: FISH,
      slowestFaster: BIRD,
    })),
  ],
};

async function mockSpeedApi(page: Page): Promise<Captured> {
  const captured: Captured = { matchup: [], reach: [], table: 0, position: 0 };
  await page.route("**/api/speed/v1/**", async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname.endsWith("/pokemon")) {
      await route.fulfill({ json: { regulationId: "example", pokemon: [BIRD, FISH] } });
    } else if (url.pathname.endsWith("/matchup")) {
      const body = route.request().postDataJSON() as { trickRoom?: boolean };
      captured.matchup.push(body);
      await route.fulfill({ json: matchupFor(body.trickRoom === true) });
    } else if (url.pathname.endsWith("/reach")) {
      captured.reach.push(route.request().postDataJSON());
      await route.fulfill({ json: REACH });
    } else if (url.pathname.endsWith("/table")) {
      captured.table += 1;
      await route.fulfill({ status: 500, json: { code: "internal_error", message: "x" } });
    } else {
      captured.position += 1;
      await route.fulfill({ status: 500, json: { code: "internal_error", message: "x" } });
    }
  });
  return captured;
}

async function openAndChoose(page: Page): Promise<Captured> {
  const captured = await mockSpeedApi(page);
  await page.goto("/speed");
  await page.getByRole("combobox", { name: "自分のポケモンを選ぶ" }).selectOption(BIRD.pokemonId);
  await page.getByRole("combobox", { name: "相手のポケモンを選ぶ" }).selectOption(FISH.pokemonId);
  return captured;
}

test("自分と相手を選ぶと、想定ごとのカードに先に動くための最低ラインが出る(表は出ない)", async ({ page }) => {
  const captured = await openAndChoose(page);

  const cards = page.getByTestId("speed-matchup-card");
  await expect(cards.first()).toBeVisible();
  const max = page.locator('[data-testid="speed-matchup-card"][data-preset="max"]');
  await expect(max.getByRole("heading", { level: 3 })).toContainText("最速");
  await expect(max.locator('[data-testid="speed-nature-line"][data-nature="plus"]')).toContainText("S12");
  const scarf = page.locator('[data-testid="speed-matchup-card"][data-preset="max-scarf"]');
  await expect(scarf).toContainText("どう振っても上を取れません");

  await expect(page.getByTestId("speed-viewport")).toHaveCount(0);
  await expect(page.getByRole("region", { name: "素早さの表" })).toHaveCount(0);
  expect(captured.table).toBe(0);
  expect(captured.position).toBe(0);
  expect(captured.matchup.length).toBeGreaterThan(0);
});

test("トリックルームを入れると上限 SP の読み方に切り替わり、下降補正の行が出る", async ({ page }) => {
  await openAndChoose(page);
  await expect(page.getByTestId("speed-matchup-card").first()).toBeVisible();

  await page.getByRole("checkbox", { name: "トリックルーム" }).check();

  const max = page.locator('[data-testid="speed-matchup-card"][data-preset="max"]');
  await expect(max.locator('[data-testid="speed-nature-line"][data-nature="minus"]')).toBeVisible();
  await expect(max.locator('[data-testid="speed-nature-line"][data-nature="neutral"]')).toContainText("S28");
});

test("今の振りを入れると、行動順と「これ以上」の一覧が出る(相手を選ばなくても一覧は出る)", async ({
  page,
}) => {
  const captured = await mockSpeedApi(page);
  await page.goto("/speed");
  await page.getByRole("combobox", { name: "自分のポケモンを選ぶ" }).selectOption(BIRD.pokemonId);
  await page.getByRole("checkbox", { name: "今の振りも入れる" }).check();
  await page.getByRole("radio", { name: "上昇" }).check();

  const rows = page.getByTestId("speed-reach-line");
  await expect(rows).toHaveCount(6);
  await expect(rows.first()).toContainText("無振り 119 族まで抜ける");
  await expect(rows.first()).toContainText("無振り 120 族以上に抜かれる");
  expect(captured.matchup).toHaveLength(0);
  expect(captured.reach.length).toBeGreaterThan(0);

  await page.getByRole("combobox", { name: "相手のポケモンを選ぶ" }).selectOption(FISH.pokemonId);
  await expect(
    page.locator('[data-testid="speed-matchup-card"][data-preset="max"]').getByTestId("speed-current-order"),
  ).toContainText("後に動く");
});

test("ポケモンを選んだあと、想定のチップとトリックルームをキーボードだけで操作できる", async ({ page }) => {
  await mockSpeedApi(page);
  await page.goto("/speed");
  // <select> の矢印キーは OS ごとに挙動が違う(macOS の Chromium はポップアップを開くだけ)ので、値の選択は selectOption にする。
  const self = page.getByRole("combobox", { name: "自分のポケモンを選ぶ" });
  await self.selectOption({ index: 1 });
  await expect(self).not.toHaveValue("");

  const chip = page.getByRole("checkbox", { name: "最速スカーフ" });
  await chip.focus();
  await page.keyboard.press("Space");
  await expect(chip).not.toBeChecked();

  const trickRoom = page.getByRole("checkbox", { name: "トリックルーム" });
  await trickRoom.focus();
  await page.keyboard.press("Space");
  await expect(trickRoom).toBeChecked();
});

test("画面の幅いっぱいでも横にはみ出さない(スマホ幅)", async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 800 });
  await openAndChoose(page);
  await expect(page.getByTestId("speed-matchup-card").first()).toBeVisible();
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
  );
  expect(overflow).toBeLessThanOrEqual(0);
});
