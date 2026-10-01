// P4-6: E2E の画面操作の部品。要素はアクセシブルな名前(src/i18n/ja.ts の語)で引き、CSS クラスには頼らない。
// 種族・技の名前は架空の例データ(src/master/example/)のもの。ADR-0313: 例データは利用者の画面には出ず、
// E2E では pokedex フィクスチャ(e2e/support/pokedexFixture.ts)が公開 API の形で返す。

import { expect, type Locator, type Page } from "@playwright/test";

/** 例データの種族の表示名(src/master/example/species.ts)。 */
export const SPECIES = {
  fire: { key: "9001-000", nameJa: "テストほのお" },
  water: { key: "9002-000", nameJa: "テストみず" },
} as const;

/** 計算結果の1行が持つ表示%の書式(domain/format.ts formatPercentRange)。 */
export const PERCENT_RANGE_PATTERN = /(\d+\.\d)〜(\d+\.\d)%/;

/** 確定数の書式(domain/format.ts formatKO)。 */
export const KO_PATTERN = /^(確定\d+発|乱数\d+発\(\d+\.\d%\)|倒せない)$/m;

/** 計算の既定の行数(engine の既定の防御側プリセット5行。ADR-0009、ADR-0300 §6)。 */
export const DEFAULT_ROW_COUNT = 5;

/** アプリを開き、マスタの読み込みが終わって画面の切り替えタブが出るまで待つ。 */
export async function openApp(page: Page): Promise<void> {
  await page.goto("/");
  await expect(page.getByRole("tablist", { name: "画面の切り替え" })).toBeVisible();
}

export function combobox(page: Page, name: string): Locator {
  return page.getByRole("combobox", { name, exact: true });
}

/** 計算画面の結果の行(「計算結果」のリストの項目)。 */
export function calcRows(page: Page): Locator {
  return page.getByRole("list", { name: "計算結果", exact: true }).getByRole("listitem");
}

/** 逆算画面の候補の行(「推定結果」のリストの項目)。 */
export function reverseRows(page: Page): Locator {
  return page.getByRole("list", { name: "推定結果", exact: true }).getByRole("listitem");
}

/**
 * PR2(ADR-0307): 種族の検索欄(SpeciesSearchField)で1体選ぶ。capabilities.speciesList が false のマスタ
 * (オンライン)では、種族のスロットが `<select>` ではなく検索入力になる(ADR-0304 A-4・A-10)。
 *
 * 同じ accessible name(`label`)の要素が `<select>` と検索入力の両方になりうるので、まず
 * `aria-expanded` を持っている(= 検索欄に入れ替わり、マスタの読み込みが終わっている)ことを待ってから
 * 入力する。候補は入力のデバウンス(SPECIES_SEARCH_DEBOUNCE_MS)後に出るが、Playwright が自動で待つ。
 */
export async function selectSpeciesBySearch(page: Page, label: string, name: string): Promise<void> {
  const input = combobox(page, label);
  await expect(input).toHaveAttribute("aria-expanded", /^(true|false)$/);
  await input.fill(name);
  // 候補リストの accessible name は入力欄と同じ(SpeciesSearchField の <ul aria-label={label}>)なので、
  // 攻撃側・防御側の候補を取り違えないようリストの中から選ぶ。
  const listbox = page.getByRole("listbox", { name: label, exact: true });
  await listbox.getByRole("option", { name, exact: true }).click();
  // 候補を選ぶと入力欄がその名前に置き換わる(SpeciesSearchField の selectCandidate)。
  await expect(input).toHaveValue(name);
}

/**
 * 計算画面で攻撃側・防御側を選ぶ(技は攻撃側の最初のダメージ技が自動で選ばれる)。
 * ADR-0313: オンライン・オフラインとも種族は検索欄で選ぶ(どちらも capabilities.speciesList が false)。
 * 以前の `<select>` 版の selectMatchup(オフラインの架空の例データ前提)は無くなった。
 */
export async function selectMatchup(page: Page, attackerName: string, defenderName: string): Promise<void> {
  await selectSpeciesBySearch(page, "攻撃側のポケモン", attackerName);
  await selectSpeciesBySearch(page, "防御側のポケモン", defenderName);
}

/** 行の文言から表示%の最小・最大を読む。書式に合わなければ失敗させる。 */
export function parsePercentRange(text: string): { min: number; max: number } {
  const match = PERCENT_RANGE_PATTERN.exec(text);
  if (match === null) {
    throw new Error(`表示%の書式に合わない: ${text}`);
  }
  return { min: Number(match[1]), max: Number(match[2]) };
}

/** 結果の全行の文言(行数が期待どおりになってから読む)。 */
export async function rowTexts(rows: Locator, count: number): Promise<string[]> {
  await expect(rows).toHaveCount(count);
  return rows.allInnerTexts();
}

/**
 * ピル型ラジオグループ(input は見た目の上で隠れ、label を押す作り。CalcScreen.tsx の AttackerPresetSelector・
 * App.tsx の CalcModeSelector)の選択肢を、利用者と同じく表示名のラベルを押して選び、選ばれたことを確かめる。
 */
export async function chooseRadio(page: Page, groupName: string, optionName: string): Promise<void> {
  const group = page.getByRole("radiogroup", { name: groupName, exact: true });
  await group.getByText(optionName, { exact: true }).click();
  await expect(group.getByRole("radio", { name: optionName, exact: true })).toBeChecked();
}

/** 既存の呼び出し(online.spec.ts)の名前。selectMatchup と同じ(ADR-0313 で両方とも検索欄になった)。 */
export const selectMatchupBySearch = selectMatchup;

/** 逆算画面で「自分」「相手」のポケモンを検索欄で選ぶ(ADR-0313。以前は `<select>`)。 */
export async function selectReverseMatchup(page: Page, ownName: string, opponentName: string): Promise<void> {
  await selectSpeciesBySearch(page, "自分のポケモン", ownName);
  await selectSpeciesBySearch(page, "相手のポケモン", opponentName);
}

/** マスタのキャッシュ(IndexedDB)のデータベース名。src/master/cache/browserStore.ts の MASTER_CACHE_DB_NAME と同じ値。 */
export const MASTER_CACHE_DB_NAME = "pokecalc-master-cache";

/**
 * ADR-0313: オンライン(既定)で一度開いて、持ち物・性格と、使う種族(fire・water)を解決し、
 * それが IndexedDB に保存されるまで待つ。オフラインで使うマスタを温める操作(利用者の「一度オンラインで開く」)。
 */
export async function warmOfflineCache(page: Page): Promise<void> {
  await page.goto("/calc");
  await expect(page.getByRole("tablist", { name: "画面の切り替え" })).toBeVisible();
  await expect(page.getByRole("radio", { name: "オンライン(API)", exact: true })).toBeChecked();
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  // データベースができただけでは足りない(開くのは保存より先)。種族(fire・water)まで書き込まれるのを待つ。
  await expect
    .poll(async () =>
      page.evaluate(async (name) => {
        // 無いデータベースを open すると空のものを作ってしまうので、あるときだけ開く。
        if (!(await indexedDB.databases()).some((db) => db.name === name)) {
          return false;
        }
        return new Promise<boolean>((resolve) => {
          const open = indexedDB.open(name);
          open.onerror = () => {
            resolve(false);
          };
          open.onsuccess = () => {
            const db = open.result;
            const storeName = db.objectStoreNames[0];
            if (storeName === undefined) {
              db.close();
              resolve(false);
              return;
            }
            const request = db.transaction(storeName, "readonly").objectStore(storeName).getAll();
            request.onerror = () => {
              db.close();
              resolve(false);
            };
            request.onsuccess = () => {
              db.close();
              const [record] = request.result as { species?: Record<string, unknown> }[];
              resolve(Object.keys(record?.species ?? {}).length >= 2);
            };
          };
        });
      }, MASTER_CACHE_DB_NAME),
    )
    .toBe(true);
}

/**
 * ADR-0313: キャッシュを温めてからオフライン(WASM)に切り替え、`path` を開き直して、マスタの読み込みが
 * 終わって画面の切り替えタブが出るまで待つ。以降の画面はキャッシュのマスタ(fire・water)で動く。
 */
export async function openAppOffline(page: Page, path = "/calc"): Promise<void> {
  await warmOfflineCache(page);
  await chooseRadio(page, "計算モード", "オフライン(WASM)");
  await page.goto(path);
  await expect(page.getByRole("tablist", { name: "画面の切り替え" })).toBeVisible();
  await expect(page.getByRole("radio", { name: "オフライン(WASM)", exact: true })).toBeChecked();
  await expect(page.getByRole("alert")).toHaveCount(0);
}
