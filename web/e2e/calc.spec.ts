// P4-6: 計算画面の主な流れ(オフライン = WASM、既定の計算モード)。docs/test-strategy.md「E2E(Playwright)」:
// ポケモン選択 → 攻撃側/防御側選択 → 技選択 → 一括結果、攻守入れ替え・プリセット変更で結果が更新される。

import { expect, test } from "@playwright/test";
import {
  DEFAULT_ROW_COUNT,
  KO_PATTERN,
  PERCENT_RANGE_PATTERN,
  SPECIES,
  calcRows,
  chooseRadio,
  combobox,
  openAppOffline,
  parsePercentRange,
  rowTexts,
  selectMatchup,
} from "./support/calcPage.ts";

test.beforeEach(async ({ page }) => {
  // ADR-0313: キャッシュを温めてからオフライン(WASM)で確かめる(既定はオンライン)。
  await openAppOffline(page);
});

test("攻撃側・防御側を選ぶと技が自動で選ばれ、既定の5行の一括結果が出る", async ({ page }) => {
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);

  // 技は攻撃側の最初のダメージ技(威力つき)が選ばれている。
  const move = combobox(page, "技");
  await expect(move).not.toHaveValue("");
  await expect(move.locator("option:checked")).toContainText("威力");

  const rows = calcRows(page);
  const texts = await rowTexts(rows, DEFAULT_ROW_COUNT);
  for (const text of texts) {
    expect(text).toMatch(PERCENT_RANGE_PATTERN);
    expect(text).toMatch(KO_PATTERN);
  }
  // 各行にダメージバーがある。issue #306 でバーは装飾(aria-hidden)にしたので、
  // role ではなく testid で数え、支援技術に名前の無い meter が残っていないことも確かめる。
  await expect(rows.getByTestId("damage-bar")).toHaveCount(DEFAULT_ROW_COUNT);
  await expect(page.getByRole("meter")).toHaveCount(0);
});

test("攻撃の調整を A特化 に変えると、先頭行の最大%が増える", async ({ page }) => {
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  const rows = calcRows(page);
  const [before] = await rowTexts(rows, DEFAULT_ROW_COUNT);
  const beforeMax = parsePercentRange(before ?? "").max;

  await chooseRadio(page, "攻撃の調整", "A特化");

  await expect
    .poll(async () => {
      const [after] = await rowTexts(rows, DEFAULT_ROW_COUNT);
      return parsePercentRange(after ?? "").max;
    })
    .toBeGreaterThan(beforeMax);
});

// I-web-1・I-web-3(ADR-0329): 「技」の次に「攻撃」「特攻」の2ブロック。選んだ技が使う方を強調し、
// SP の数値入力と性格補正(上昇)で結果が強くなる向きに変わる(丸めは engine のまま。Web は値を渡すだけ)。
test("技が使う側のブロックで SP を数値で入れ、性格補正を上昇にすると、先頭行の最大%が段階的に増える", async ({
  page,
}) => {
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  const rows = calcRows(page);
  const firstMax = async () => parsePercentRange((await rowTexts(rows, DEFAULT_ROW_COUNT))[0] ?? "").max;
  const before = await firstMax();

  // 2ブロックとも出ていて、強調(この技で使用)はちょうど1つ。
  await expect(page.getByRole("group", { name: /^(攻撃|特攻)(\(この技で使用\))?$/ })).toHaveCount(2);
  const used = page.getByRole("group", { name: /^(攻撃|特攻)\(この技で使用\)$/ });
  await expect(used).toHaveCount(1);
  await expect(used).toHaveAttribute("aria-current", "true");

  await used.getByRole("textbox", { name: /のSP$/ }).fill("20");
  await expect.poll(firstMax).toBeGreaterThan(before);
  const afterSp = await firstMax();

  const nature = used.getByRole("radiogroup", { name: /の性格補正$/ });
  await nature.getByText("上昇", { exact: true }).click();
  await expect(nature.getByRole("radio", { name: "上昇", exact: true })).toBeChecked();
  await expect.poll(firstMax).toBeGreaterThan(afterSp);
  await expect(page.getByRole("alert")).toHaveCount(0);
});

test("攻守入れ替えで攻撃側・防御側の名前が入れ替わり、結果が更新される", async ({ page }) => {
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  const rows = calcRows(page);
  const before = await rowTexts(rows, DEFAULT_ROW_COUNT);

  await page.getByRole("button", { name: "攻守入れ替え", exact: true }).click();

  await expect(combobox(page, "攻撃側のポケモン")).toHaveValue(SPECIES.water.nameJa);
  await expect(combobox(page, "防御側のポケモン")).toHaveValue(SPECIES.fire.nameJa);
  // issue #304: カードの見出し(h2)は「攻撃側」「防御側」のまま動かず、
  // 入れ替わるのはその中のポケモンの名前(h3。design.md「入力のラベル」の見出しの階層)。
  await expect(
    page.getByRole("region", { name: "攻撃側", exact: true }).getByRole("heading", { level: 2 }),
  ).toHaveText("攻撃側");
  await expect(
    page.getByRole("region", { name: "攻撃側", exact: true }).getByRole("heading", { level: 3 }),
  ).toHaveText(SPECIES.water.nameJa);
  await expect(
    page.getByRole("region", { name: "防御側", exact: true }).getByRole("heading", { level: 3 }),
  ).toHaveText(SPECIES.fire.nameJa);

  // 技は新しい攻撃側(テストみず)の learnset から選び直され、結果は入れ替え前と異なる。
  await expect(combobox(page, "技").locator("option:checked")).toContainText("威力");
  await expect.poll(async () => rowTexts(rows, DEFAULT_ROW_COUNT)).not.toEqual(before);
  for (const text of await rowTexts(rows, DEFAULT_ROW_COUNT)) {
    expect(text).toMatch(PERCENT_RANGE_PATTERN);
  }
});

test("持ち物の候補も比較 は、オフライン(キャッシュ)では選べない(ADR-0313 §3)", async ({ page }) => {
  // 以前は架空の例データの持ち物効果で行が増えることを確かめていた。実データ相当のマスタは公開 API が持ち物の
  // 効果データを持たない(ADR-0304 A-1)ので、オフラインのキャッシュでも比較は出せない。増える挙動は
  // 効果データを持つマスタの単体テスト(CalcScreen.test.tsx・requests.test.ts)で確かめる。
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  await expect(calcRows(page)).toHaveCount(DEFAULT_ROW_COUNT);

  await expect(page.getByRole("checkbox", { name: "持ち物の候補も比較", exact: true })).toBeDisabled();
  await expect(calcRows(page)).toHaveCount(DEFAULT_ROW_COUNT);
});

test("「詳細」を開いて 急所 とはれを入れると、先頭行の最大%が増える(issue #274)", async ({ page }) => {
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  const rows = calcRows(page);
  const [before] = await rowTexts(rows, DEFAULT_ROW_COUNT);
  const beforeMax = parsePercentRange(before ?? "").max;

  // 「詳細」は既定で閉じていて、中の入力は出ていない。
  const details = page.getByRole("button", { name: "詳細", exact: true });
  await expect(details).toHaveAttribute("aria-expanded", "false");
  await expect(page.getByRole("checkbox", { name: "急所", exact: true })).toHaveCount(0);

  await details.click();
  await expect(details).toHaveAttribute("aria-expanded", "true");
  await page.getByRole("checkbox", { name: "急所", exact: true }).check();

  await expect
    .poll(async () => {
      const [after] = await rowTexts(rows, DEFAULT_ROW_COUNT);
      return parsePercentRange(after ?? "").max;
    })
    .toBeGreaterThan(beforeMax);

  // 閉じても条件は効いたまま(結果は元に戻らない)。
  await details.click();
  await expect(details).toHaveAttribute("aria-expanded", "false");
  const [closed] = await rowTexts(rows, DEFAULT_ROW_COUNT);
  expect(parsePercentRange(closed ?? "").max).toBeGreaterThan(beforeMax);
});

test("「詳細」で防御側のランク B を +1 にすると、物理技の先頭行の最大%が下がる(issue #274)", async ({
  page,
}) => {
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  const rows = calcRows(page);
  const [before] = await rowTexts(rows, DEFAULT_ROW_COUNT);
  const beforeMax = parsePercentRange(before ?? "").max;

  await page.getByRole("button", { name: "詳細", exact: true }).click();
  const group = page.getByRole("group", { name: "防御側のランク", exact: true });
  await expect(group.getByText("B ±0")).toBeVisible();
  await group.getByRole("button", { name: "防御側のランクを上げる" }).click();
  await expect(group.getByText("B +1")).toBeVisible();

  await expect
    .poll(async () => {
      const [after] = await rowTexts(rows, DEFAULT_ROW_COUNT);
      return parsePercentRange(after ?? "").max;
    })
    .toBeLessThan(beforeMax);
});

test("技の並びを 五十音順 に切り替えると、技の選択肢が並び替わり、選んでいた技は変わらない(I-web-9 = F-02)", async ({
  page,
}) => {
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  const move = combobox(page, "技");
  const optionNames = () => move.locator("option").allTextContents();
  const nameOf = (text: string) => text.split("・")[0] ?? text;

  // 既定は習得順(テストほのお: たいあたり → かえんパンチ)。選ばれている技を覚えておく。
  const selectedBefore = await move.inputValue();
  expect((await optionNames()).map(nameOf)).toEqual(["テストたいあたり", "テストかえんパンチ"]);

  await chooseRadio(page, "技の並び", "五十音順");
  // 五十音順(か行 → た行)に入れ替わる。
  await expect
    .poll(async () => (await optionNames()).map(nameOf))
    .toEqual(["テストかえんパンチ", "テストたいあたり"]);
  await expect(move).toHaveValue(selectedBefore);

  await chooseRadio(page, "技の並び", "習得順");
  await expect
    .poll(async () => (await optionNames()).map(nameOf))
    .toEqual(["テストたいあたり", "テストかえんパンチ"]);
});
