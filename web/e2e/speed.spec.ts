// F-06(Web・ADR-0608): 素早さ表の仮想スクロールと、右の位置マーカーの高さ揃えを実ブラウザ(chromium)で確かめる。
// speed-svc はこの構成では動いていないので、speed API は page.route で架空の応答(9xxx の架空ポケモン)に差し替える。
// jsdom にはレイアウトが無いので、「左の該当行とマーカーが同じ高さに並ぶ」は実ブラウザの座標で確かめる。

import { expect, test, type Page } from "@playwright/test";

const BIRD = { pokemonId: "9001-000", nameJa: "テストカソウドリ", types: ["fire", "flying"], baseSpeed: 100 };
const TIER_COUNT = 300;
/** 自分の素早さ。2560 の段と 2550 の段の間(同速なし)に入る。 */
const OWN_SPEED = 2555;

function tiersFor(ascending: boolean) {
  const speeds = Array.from({ length: TIER_COUNT }, (_, i) => 3000 - i * 10);
  if (ascending) {
    speeds.reverse();
  }
  return speeds.map((speed) => ({ speed, entries: [{ ...BIRD, preset: "max" }] }));
}

async function mockSpeedApi(page: Page): Promise<void> {
  await page.route("**/api/speed/v1/**", async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname.endsWith("/pokemon")) {
      await route.fulfill({ json: { regulationId: "example", pokemon: [BIRD] } });
    } else if (url.pathname.endsWith("/table")) {
      const trickRoom = url.searchParams.get("trickRoom") === "true";
      await route.fulfill({
        json: {
          regulationId: "example",
          presets: ["uninvested", "neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"],
          tiers: tiersFor(trickRoom),
        },
      });
    } else {
      await route.fulfill({
        json: { speed: OWN_SPEED, faster: 45, slower: 255, tie: [], pokemon: BIRD },
      });
    }
  });
}

async function openWithSelf(page: Page): Promise<void> {
  await mockSpeedApi(page);
  await page.goto("/speed");
  await expect(page.getByTestId("speed-viewport")).toBeVisible();
  await page.getByRole("combobox", { name: "ポケモン", exact: true }).selectOption(BIRD.pokemonId);
  await expect(page.getByTestId("speed-marker")).toBeVisible();
}

test("素早さ表は周辺の段だけを描画し、スクロールで入れ替わる(総数は aria-setsize で伝わる)", async ({
  page,
}) => {
  await mockSpeedApi(page);
  await page.goto("/speed");
  const viewport = page.getByTestId("speed-viewport");
  await expect(viewport).toBeVisible();

  const rows = viewport.getByTestId("speed-tier");
  await expect(rows.first()).toHaveAttribute("data-speed", "3000");
  expect(await rows.count()).toBeLessThan(40);
  await expect(rows.first()).toHaveAttribute("aria-setsize", String(TIER_COUNT));

  await viewport.evaluate((element) => {
    element.scrollTop = element.scrollHeight;
  });
  await expect(rows.last()).toHaveAttribute("data-speed", String(3000 - (TIER_COUNT - 1) * 10));
  await expect(rows.last()).toHaveAttribute("aria-posinset", String(TIER_COUNT));
  expect(await rows.count()).toBeLessThan(40);
});

test("キーボード(フォーカス + PageDown)でスクロールできる", async ({ page }) => {
  await mockSpeedApi(page);
  await page.goto("/speed");
  const viewport = page.getByTestId("speed-viewport");
  await expect(viewport.getByTestId("speed-tier").first()).toBeVisible();
  await viewport.focus();
  await page.keyboard.press("PageDown");
  await expect.poll(() => viewport.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
});

test("右の位置マーカーが、左の境界の行と同じ高さに並び、スクロールに追従する", async ({ page }) => {
  await openWithSelf(page);
  await page.getByRole("button", { name: "自分の位置へ移動" }).click();

  const boundary = page.getByTestId("speed-boundary");
  const marker = page.getByTestId("speed-marker");
  await expect(boundary).toBeVisible();
  await expect(marker).toHaveAttribute("data-state", "visible");

  async function boxes() {
    const b = await boundary.boundingBox();
    const m = await marker.boundingBox();
    if (b === null || m === null) {
      throw new Error("境界の行・マーカーの位置が取れない");
    }
    return { b, m };
  }
  const first = await boxes();
  expect(Math.abs(first.b.y - first.m.y)).toBeLessThan(2);

  // 少しスクロールしても同じ高さのまま(左のスクロールに追従)。
  await page.getByTestId("speed-viewport").evaluate((element) => {
    element.scrollTop += 90;
  });
  await expect
    .poll(async () => {
      const { b, m } = await boxes();
      return Math.abs(b.y - m.y) < 2;
    })
    .toBe(true);
});

test("トリックルーム(昇順の表)でもマーカーが境界の行と揃う", async ({ page }) => {
  await openWithSelf(page);
  await page.getByRole("checkbox", { name: "トリックルーム" }).check();
  await expect(page.getByTestId("speed-viewport").getByTestId("speed-tier").first()).toHaveAttribute(
    "data-speed",
    "10",
  );
  await page.getByRole("button", { name: "自分の位置へ移動" }).click();
  const boundary = page.getByTestId("speed-boundary");
  await expect(boundary).toHaveAttribute("data-after-speed", "2550");
  await expect(boundary).toHaveAttribute("data-before-speed", "2560");
  const b = await boundary.boundingBox();
  const m = await page.getByTestId("speed-marker").boundingBox();
  if (b === null || m === null) {
    throw new Error("境界の行・マーカーの位置が取れない");
  }
  expect(Math.abs(b.y - m.y)).toBeLessThan(2);
});

// F-12 のデザイン基盤が行の見た目を変えても、仮想スクロールの前提(行の実高さ = 計算した高さ、内容がはみ出さない)を保つ。
test("段・境界の行の実高さが計算値(1行24px・余白16px)と一致し、内容がはみ出さない", async ({ page }) => {
  await openWithSelf(page);
  await page.getByRole("button", { name: "自分の位置へ移動" }).click();
  await expect(page.getByTestId("speed-boundary")).toBeVisible();
  const result = await page.getByTestId("speed-viewport").evaluate((viewport) => {
    const rows = Array.from(viewport.querySelectorAll<HTMLElement>("li.speed-tier, li.speed-boundary"));
    return rows.map((row) => ({
      expected: Number.parseFloat(row.style.height),
      actual: row.getBoundingClientRect().height,
      overflow: row.scrollHeight > row.clientHeight,
      entries: row.querySelectorAll("li.speed-entry").length,
    }));
  });
  expect(result.length).toBeGreaterThan(0);
  for (const row of result) {
    expect(Math.abs(row.actual - row.expected)).toBeLessThan(1);
    expect(row.overflow).toBe(false);
    if (row.entries > 0) {
      // 1行の段: 24px + 余白16px = 40px(枠から隙間8pxを引いた値)。
      expect(row.expected).toBe(row.entries * 24 + 16);
    }
  }
});
