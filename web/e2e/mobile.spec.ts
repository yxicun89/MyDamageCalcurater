// P4-21(issue #98): モバイル幅のレイアウトの回帰検査。docs/design.md「幅への対応(ブレークポイント)」。
// 狭い幅(320・375)で計算・逆算のページが横に溢れず、主要な操作部品が表示領域の中に収まること、
// 広い幅(768)では従来どおり攻撃側・防御側が左右に並ぶことを見る。
//
// viewport は project を増やさずテストごとに指定する(全 spec を2回走らせないため。
// Desktop Chrome の既定の viewport は playwright.config.ts の project 側)。
// CSS クラスには頼らず、アクセシブルな名前(src/i18n/ja.ts の語)で要素を引く(support/calcPage.ts と同じ作法)。

import { expect, test, type Locator, type Page } from "@playwright/test";
import {
  DEFAULT_ROW_COUNT,
  SPECIES,
  calcRows,
  chooseRadio,
  combobox,
  selectMatchup,
} from "./support/calcPage.ts";

/** issue #98 の再現幅(実測で横に溢れていた viewport)。 */
const NARROW_WIDTHS = [320, 375] as const;

/** 左右配置のまま成り立つ幅(実測で溢れていない viewport。design.md のブレークポイント以上)。 */
const WIDE_WIDTH = 768;

const VIEWPORT_HEIGHT = 800;

/** 位置の比較で許す誤差(小数のレイアウト値・枠線1pxの丸め)。 */
const EPSILON = 1;

interface Box {
  readonly x: number;
  readonly y: number;
  readonly width: number;
  readonly height: number;
}

async function boxOf(locator: Locator, name: string): Promise<Box> {
  const box = await locator.boundingBox();
  expect(box, `${name} の位置が取れない(表示されていない)`).not.toBeNull();
  return box ?? { x: 0, y: 0, width: 0, height: 0 };
}

/** ページ全体が横に溢れていないこと(issue #98 の受け入れ条件)。 */
async function expectNoHorizontalOverflow(page: Page): Promise<void> {
  const size = await page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
  }));
  expect(
    size.scrollWidth,
    `横に溢れている(scrollWidth ${String(size.scrollWidth)} > clientWidth ${String(size.clientWidth)})`,
  ).toBeLessThanOrEqual(size.clientWidth);
}

/** 部品が左右とも表示領域の中にあること(見えていても画面外にはみ出していれば操作できない)。 */
async function expectInsideViewport(page: Page, locator: Locator, name: string): Promise<void> {
  const clientWidth = await page.evaluate(() => document.documentElement.clientWidth);
  const box = await boxOf(locator, name);
  expect(box.x, `${name} が左にはみ出している`).toBeGreaterThanOrEqual(-EPSILON);
  expect(box.x + box.width, `${name} が右にはみ出している`).toBeLessThanOrEqual(clientWidth + EPSILON);
}

/** 計算画面を開き、結果まで出た状態にする(入力が全部そろった、いちばん横に広がる状態)。 */
async function openCalcScreen(page: Page): Promise<void> {
  await page.goto("/calc");
  await expect(page.getByRole("tablist", { name: "画面の切り替え" })).toBeVisible();
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  await expect(calcRows(page)).toHaveCount(DEFAULT_ROW_COUNT);
}

/** 逆算画面を開き、観測を2件入れた状態にする(観測の行が増えても溢れないことを見るため)。 */
async function openReverseScreen(page: Page): Promise<void> {
  await page.goto("/reverse");
  await expect(page.getByRole("tablist", { name: "画面の切り替え" })).toBeVisible();
  await combobox(page, "自分のポケモン").selectOption({ label: SPECIES.fire.nameJa });
  await combobox(page, "相手のポケモン").selectOption({ label: SPECIES.water.nameJa });
  await page.getByRole("textbox", { name: "観測1", exact: true }).fill("50");
  await page.getByRole("button", { name: "観測を追加", exact: true }).click();
  await page.getByRole("textbox", { name: "観測2", exact: true }).fill("50");
  await expect(page.getByRole("textbox", { name: "観測2", exact: true })).toHaveValue("50");
}

/** 計算画面の主要な操作部品(すべて表示領域の中で操作できること)。 */
function calcControls(page: Page): { name: string; locator: Locator }[] {
  const named = (role: "combobox" | "button" | "checkbox" | "radiogroup", name: string) => ({
    name,
    locator: page.getByRole(role, { name, exact: true }),
  });
  return [
    named("combobox", "攻撃側のポケモン"),
    named("combobox", "攻撃側の持ち物"),
    named("radiogroup", "攻撃側の調整"),
    named("button", "攻守入れ替え"),
    named("combobox", "防御側のポケモン"),
    named("combobox", "防御側の持ち物"),
    named("combobox", "技"),
    named("checkbox", "持ち物の候補も比較"),
    { name: "計算結果", locator: page.getByRole("list", { name: "計算結果", exact: true }) },
  ];
}

/** 逆算画面の主要な操作部品。 */
function reverseControls(page: Page): { name: string; locator: Locator }[] {
  const named = (role: "combobox" | "button" | "textbox" | "radiogroup", name: string) => ({
    name,
    locator: page.getByRole(role, { name, exact: true }),
  });
  return [
    named("radiogroup", "観測したダメージ"),
    named("combobox", "自分のポケモン"),
    named("combobox", "自分の持ち物"),
    named("radiogroup", "自分の調整"),
    named("combobox", "相手のポケモン"),
    named("combobox", "技"),
    named("textbox", "観測1"),
    named("radiogroup", "観測1の単位"),
    named("textbox", "観測2"),
    // 削除ボタンが出るのは末尾の観測だけ(ReverseScreen.tsx)。
    named("button", "観測2を削除"),
    named("button", "観測を追加"),
  ];
}

type CalcZone = "attacker" | "swap" | "defender" | "other";
type ReverseZone = "mine" | "theirs" | "other";

/** いまフォーカスがある要素が、計算画面のどの区画にあるか。 */
async function focusedCalcZone(page: Page): Promise<CalcZone> {
  return page.evaluate<CalcZone>(() => {
    const element = document.activeElement;
    if (element === null) {
      return "other";
    }
    if (element instanceof HTMLButtonElement && element.textContent.trim() === "攻守入れ替え") {
      return "swap";
    }
    if (element.closest('[aria-label="攻撃側"]') !== null) {
      return "attacker";
    }
    if (element.closest('[aria-label="防御側"]') !== null) {
      return "defender";
    }
    return "other";
  });
}

/** いまフォーカスがある要素が、逆算画面のどちら側のカードにあるか。 */
async function focusedReverseZone(page: Page): Promise<ReverseZone> {
  return page.evaluate<ReverseZone>(() => {
    const element = document.activeElement;
    if (element === null) {
      return "other";
    }
    if (element.closest('section[aria-label="自分のポケモン"]') !== null) {
      return "mine";
    }
    if (element.closest('section[aria-label="相手のポケモン"]') !== null) {
      return "theirs";
    }
    return "other";
  });
}

/** Tab を押しながら、フォーカスが通る区画を順に集める(`last` に届くか、上限に達するまで)。 */
async function tabZones<Zone extends string>(
  page: Page,
  zoneOf: (page: Page) => Promise<Zone>,
  last: Zone,
  maxSteps = 20,
): Promise<Zone[]> {
  const zones: Zone[] = [await zoneOf(page)];
  for (let step = 0; step < maxSteps && !zones.includes(last); step += 1) {
    await page.keyboard.press("Tab");
    zones.push(await zoneOf(page));
  }
  return zones;
}

for (const width of NARROW_WIDTHS) {
  test.describe(`狭い幅(${String(width)}px)`, () => {
    test.use({ viewport: { width, height: VIEWPORT_HEIGHT } });

    test("計算画面が横に溢れず、主要な操作部品が表示領域に収まる", async ({ page }) => {
      await openCalcScreen(page);

      await expectNoHorizontalOverflow(page);
      for (const control of calcControls(page)) {
        await expectInsideViewport(page, control.locator, control.name);
      }
    });

    test("計算画面: カードは縦積みで、攻撃側 → 攻守入れ替え → 防御側 の順に並ぶ", async ({ page }) => {
      await openCalcScreen(page);

      const attacker = await boxOf(page.getByRole("region", { name: "攻撃側", exact: true }), "攻撃側カード");
      const swap = await boxOf(
        page.getByRole("button", { name: "攻守入れ替え", exact: true }),
        "攻守入れ替え",
      );
      const defender = await boxOf(page.getByRole("region", { name: "防御側", exact: true }), "防御側カード");

      // 縦積み: 上から 攻撃側 → 入れ替え → 防御側(左右には並ばない)。
      expect(swap.y, "攻守入れ替えが攻撃側カードの下にない").toBeGreaterThanOrEqual(
        attacker.y + attacker.height - EPSILON,
      );
      expect(defender.y, "防御側カードが攻守入れ替えの下にない").toBeGreaterThanOrEqual(
        swap.y + swap.height - EPSILON,
      );
      // 2枚のカードは同じ左端・同じ幅(1列)。
      expect(Math.abs(attacker.x - defender.x)).toBeLessThanOrEqual(EPSILON);
      expect(Math.abs(attacker.width - defender.width)).toBeLessThanOrEqual(EPSILON);
    });

    test("計算画面: キーボードのフォーカスは 攻撃側 → 攻守入れ替え → 防御側 の順に進む", async ({ page }) => {
      await openCalcScreen(page);
      await combobox(page, "攻撃側のポケモン").focus();

      const zones = await tabZones(page, focusedCalcZone, "defender");
      const visited = zones.filter((zone): zone is Exclude<CalcZone, "other"> => zone !== "other");
      expect(visited, "攻守入れ替えにフォーカスが来ない").toContain("swap");
      expect(visited, "防御側にフォーカスが来ない").toContain("defender");

      const order: Record<Exclude<CalcZone, "other">, number> = { attacker: 0, swap: 1, defender: 2 };
      const ranks = visited.map((zone) => order[zone]);
      expect(ranks, `フォーカス順が 攻撃側 → 入れ替え → 防御側 でない: ${visited.join(" → ")}`).toEqual(
        [...ranks].sort((a, b) => a - b),
      );
    });

    test("計算画面: 攻守入れ替えと調整のラジオが操作できる", async ({ page }) => {
      await openCalcScreen(page);

      await chooseRadio(page, "攻撃側の調整", "A特化");

      await page.getByRole("button", { name: "攻守入れ替え", exact: true }).click();
      await expect(
        page.getByRole("region", { name: "攻撃側", exact: true }).getByRole("heading", { level: 2 }),
      ).toHaveText(SPECIES.water.nameJa);
      await expectNoHorizontalOverflow(page);
    });

    test("逆算画面が横に溢れず、主要な操作部品が表示領域に収まる", async ({ page }) => {
      await openReverseScreen(page);

      await expectNoHorizontalOverflow(page);
      for (const control of reverseControls(page)) {
        await expectInsideViewport(page, control.locator, control.name);
      }
    });

    test("逆算画面: カードは縦積みで、自分側 → 相手側 の順に並ぶ", async ({ page }) => {
      await openReverseScreen(page);

      const mine = await boxOf(
        page.getByRole("region", { name: "自分のポケモン", exact: true }),
        "自分側カード",
      );
      const theirs = await boxOf(
        page.getByRole("region", { name: "相手のポケモン", exact: true }),
        "相手側カード",
      );

      expect(theirs.y, "相手側カードが自分側カードの下にない").toBeGreaterThanOrEqual(
        mine.y + mine.height - EPSILON,
      );
      expect(Math.abs(mine.x - theirs.x)).toBeLessThanOrEqual(EPSILON);
      expect(Math.abs(mine.width - theirs.width)).toBeLessThanOrEqual(EPSILON);
    });

    test("逆算画面: キーボードのフォーカスは 自分側 → 相手側 の順に進む", async ({ page }) => {
      await openReverseScreen(page);
      await combobox(page, "自分のポケモン").focus();

      const zones = await tabZones(page, focusedReverseZone, "theirs");
      const visited = zones.filter((zone): zone is Exclude<ReverseZone, "other"> => zone !== "other");
      expect(visited, "相手側にフォーカスが来ない").toContain("theirs");
      const order: Record<Exclude<ReverseZone, "other">, number> = { mine: 0, theirs: 1 };
      const ranks = visited.map((zone) => order[zone]);
      expect(ranks, `フォーカス順が 自分側 → 相手側 でない: ${visited.join(" → ")}`).toEqual(
        [...ranks].sort((a, b) => a - b),
      );
    });
  });
}

test.describe(`広い幅(${String(WIDE_WIDTH)}px)`, () => {
  test.use({ viewport: { width: WIDE_WIDTH, height: VIEWPORT_HEIGHT } });

  test("計算画面は攻撃側 ⇄ 防御側の左右配置のまま(design.md「画面: ダメージ計算」)", async ({ page }) => {
    await openCalcScreen(page);

    const attacker = await boxOf(page.getByRole("region", { name: "攻撃側", exact: true }), "攻撃側カード");
    const swap = await boxOf(page.getByRole("button", { name: "攻守入れ替え", exact: true }), "攻守入れ替え");
    const defender = await boxOf(page.getByRole("region", { name: "防御側", exact: true }), "防御側カード");

    // 左から 攻撃側 → 入れ替え → 防御側 で、2枚のカードの上端はそろう。
    expect(attacker.x + attacker.width).toBeLessThanOrEqual(swap.x + EPSILON);
    expect(swap.x + swap.width).toBeLessThanOrEqual(defender.x + EPSILON);
    expect(Math.abs(attacker.y - defender.y)).toBeLessThanOrEqual(EPSILON);
    await expectNoHorizontalOverflow(page);
  });

  test("逆算画面は自分側・相手側の左右配置のまま", async ({ page }) => {
    await openReverseScreen(page);

    const mine = await boxOf(
      page.getByRole("region", { name: "自分のポケモン", exact: true }),
      "自分側カード",
    );
    const theirs = await boxOf(
      page.getByRole("region", { name: "相手のポケモン", exact: true }),
      "相手側カード",
    );

    expect(mine.x + mine.width).toBeLessThanOrEqual(theirs.x + EPSILON);
    expect(Math.abs(mine.y - theirs.y)).toBeLessThanOrEqual(EPSILON);
    await expectNoHorizontalOverflow(page);
  });
});
