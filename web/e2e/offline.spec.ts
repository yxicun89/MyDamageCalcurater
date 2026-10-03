// ADR-0313(issue #210): オフライン(WASM)は、オンラインで一度取得したマスタのキャッシュ(IndexedDB)から読む。
// 実データ相当のマスタ(pokedex フィクスチャ。ADR-0307)で、
//   - 既定の計算モードはオンライン。一度開いて種族を引いたあと /api を遮断してもオフラインで計算できる
//   - /api を遮断すると通信は一切起きず、engine.wasm は最初の計算で初めて取得する(遅延読み込み。ADR-0300 §2)
//   - 初回(キャッシュ空)で /api を遮断してオフラインを選ぶと、架空データを出さず「一度オンラインで開く」案内になる
//   - キャッシュが壊れていれば破棄して案内になり、オンラインで開き直すと作り直せる
//   - IndexedDB が使えなくてもオンラインの計算は成功する

import { expect, test, type Request } from "@playwright/test";
import {
  DEFAULT_ROW_COUNT,
  MASTER_CACHE_DB_NAME,
  PERCENT_RANGE_PATTERN,
  SPECIES,
  calcRows,
  chooseRadio,
  openApp,
  rowTexts,
  selectMatchup,
  warmOfflineCache,
} from "./support/calcPage.ts";

/** 案内の文言(src/i18n/ja.ts の appText.masterCacheEmptyError)の一部。 */
const GUIDE_PATTERN = /オンライン/;

function isEngineWasm(request: Request): boolean {
  return new URL(request.url()).pathname.endsWith("/engine.wasm");
}

function isApi(request: Request): boolean {
  return new URL(request.url()).pathname.startsWith("/api/");
}

test("既定はオンライン。一度開いて種族を引いたあと /api を遮断しても、オフラインで計算できる", async ({
  page,
}) => {
  const apiRequests: string[] = [];
  const wasmRequests: string[] = [];
  page.on("request", (request) => {
    if (isEngineWasm(request)) {
      wasmRequests.push(request.url());
    }
    if (isApi(request)) {
      apiRequests.push(request.url());
    }
  });

  // 1. オンラインで一度開く(マスタを取得・種族を解決 → キャッシュに保存される)。架空のテストモンは出ない。
  await warmOfflineCache(page);
  await expect(page.getByText("テストモン")).toHaveCount(0);
  await page.waitForLoadState("networkidle");
  expect(wasmRequests, "オンラインの間は engine.wasm を取得しない").toEqual([]);

  // 2. オフラインに切り替え、バックエンドが落ちている状態を再現して開き直す(届いた要求は全て通信エラー)。
  await chooseRadio(page, "ダメージ計算の実行場所", "オフライン(WASM)");
  await page.route("**/api/**", (route) => route.abort("connectionrefused"));
  apiRequests.length = 0;
  await page.goto("/calc");
  await expect(page.getByRole("tablist", { name: "画面の切り替え" })).toBeVisible();
  await expect(page.getByRole("radio", { name: "オフライン(WASM)", exact: true })).toBeChecked();
  await expect(page.getByRole("alert")).toHaveCount(0);
  await page.waitForLoadState("networkidle");
  expect(wasmRequests, "オフライン表示の時点では engine.wasm を取得しない").toEqual([]);

  // 3. キャッシュ済みの種族(検索はキャッシュ内が対象)で計算できる。
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  for (const text of await rowTexts(calcRows(page), DEFAULT_ROW_COUNT)) {
    expect(text).toMatch(PERCENT_RANGE_PATTERN);
  }
  await expect(page.getByRole("alert")).toHaveCount(0);

  expect(wasmRequests, "最初の計算で engine.wasm を1回だけ取得する").toHaveLength(1);
  expect(
    apiRequests.map((url) => new URL(url).pathname),
    "オフラインでは /api を呼ばない",
  ).toEqual([]);

  // 2回目以降の計算で engine.wasm を取り直さない。
  const beforeSwap = await calcRows(page).allInnerTexts();
  await page.getByRole("button", { name: "攻守入れ替え", exact: true }).click();
  await expect.poll(async () => calcRows(page).allInnerTexts()).not.toEqual(beforeSwap);
  await expect(calcRows(page)).toHaveCount(DEFAULT_ROW_COUNT);
  expect(wasmRequests).toHaveLength(1);
});

test("オフラインではキャッシュに無い種族は引けない(架空データは出ない)", async ({ page }) => {
  await warmOfflineCache(page);
  await chooseRadio(page, "ダメージ計算の実行場所", "オフライン(WASM)");
  await page.route("**/api/**", (route) => route.abort("connectionrefused"));
  await page.goto("/calc");
  await expect(page.getByRole("tablist", { name: "画面の切り替え" })).toBeVisible();

  const input = page.getByRole("combobox", { name: "攻撃側のポケモン", exact: true });
  await input.fill("テスト");
  const listbox = page.getByRole("listbox", { name: "攻撃側のポケモン", exact: true });
  // 解決済み(キャッシュ済み)の fire・water だけが候補に出る。
  await expect(listbox.getByRole("option", { name: SPECIES.fire.nameJa, exact: true })).toBeVisible();
  await expect(page.getByText("テストモン")).toHaveCount(0);
});

test("初回(キャッシュ空)で /api を遮断してオフラインを選ぶと、架空データを出さず案内を出す", async ({
  page,
}) => {
  await page.addInitScript(() => {
    localStorage.setItem("pokecalc.calcMode", "offline");
  });
  await page.route("**/api/**", (route) => route.abort("connectionrefused"));

  await page.goto("/");

  const alert = page.getByRole("alert");
  await expect(alert).toBeVisible();
  await expect(alert).toContainText(GUIDE_PATTERN);
  await expect(page.getByText("テストモン")).toHaveCount(0);
  await expect(page.getByRole("combobox", { name: "攻撃側のポケモン", exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "再試行", exact: true })).toBeVisible();
});

test("既定(オンライン)で /api を遮断・キャッシュ空: オフラインへ黙って落とさず、エラーと再試行を出す", async ({
  page,
}) => {
  await page.route("**/api/**", (route) => route.abort("connectionrefused"));

  await page.goto("/");

  await expect(page.getByRole("alert")).toBeVisible();
  await expect(page.getByRole("radio", { name: "オンライン(API)", exact: true })).toBeChecked();
  await expect(page.getByRole("button", { name: "再試行", exact: true })).toBeVisible();
  await expect(page.getByText("テストモン")).toHaveCount(0);
});

test("キャッシュが壊れていれば破棄して案内を出し、オンラインで開き直すと作り直せる", async ({ page }) => {
  await warmOfflineCache(page);
  // 壊れたデータを直接書く(スキーマに合わない値)。
  await page.evaluate(async (dbName) => {
    await new Promise<void>((resolve, reject) => {
      const open = indexedDB.open(dbName);
      open.onerror = () => {
        reject(open.error ?? new Error("open failed"));
      };
      open.onsuccess = () => {
        const db = open.result;
        const storeName = db.objectStoreNames[0];
        if (storeName === undefined) {
          reject(new Error("object store が無い"));
          return;
        }
        const tx = db.transaction(storeName, "readwrite");
        const store = tx.objectStore(storeName);
        const keys = store.getAllKeys();
        keys.onsuccess = () => {
          for (const key of keys.result) {
            store.put("garbage", key);
          }
        };
        tx.oncomplete = () => {
          db.close();
          resolve();
        };
        tx.onerror = () => {
          reject(tx.error ?? new Error("tx failed"));
        };
      };
    });
  }, MASTER_CACHE_DB_NAME);
  await chooseRadio(page, "ダメージ計算の実行場所", "オフライン(WASM)");
  await page.route("**/api/**", (route) => route.abort("connectionrefused"));

  await page.goto("/calc");
  await expect(page.getByRole("alert")).toContainText(GUIDE_PATTERN);

  // API を戻してオンラインで開き直すと、再取得してキャッシュを作り直し、オフラインで使える。
  await page.unroute("**/api/**");
  await page.evaluate(() => {
    localStorage.setItem("pokecalc.calcMode", "online");
  });
  await warmOfflineCache(page);
  await chooseRadio(page, "ダメージ計算の実行場所", "オフライン(WASM)");
  await page.goto("/calc");
  await expect(page.getByRole("tablist", { name: "画面の切り替え" })).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);
});

test("スキーマ版が違うキャッシュは破棄して案内を出す", async ({ page }) => {
  await warmOfflineCache(page);
  // 保存済みのレコードの schemaVersion だけを未来の版に書き換える(形は正しいが版違い)。
  await page.evaluate(async (dbName) => {
    await new Promise<void>((resolve, reject) => {
      const open = indexedDB.open(dbName);
      open.onerror = () => {
        reject(open.error ?? new Error("open failed"));
      };
      open.onsuccess = () => {
        const db = open.result;
        const storeName = db.objectStoreNames[0];
        if (storeName === undefined) {
          reject(new Error("object store が無い"));
          return;
        }
        const tx = db.transaction(storeName, "readwrite");
        const store = tx.objectStore(storeName);
        const all = store.getAll();
        const keys = store.getAllKeys();
        tx.oncomplete = () => {
          db.close();
          resolve();
        };
        tx.onerror = () => {
          reject(tx.error ?? new Error("tx failed"));
        };
        all.onsuccess = () => {
          keys.onsuccess = () => {
            const record = all.result[0] as Record<string, unknown> | undefined;
            const key = keys.result[0];
            if (record === undefined || key === undefined) {
              reject(new Error("レコードが無い"));
              return;
            }
            store.put({ ...record, schemaVersion: 999999 }, key);
          };
        };
      };
    });
  }, MASTER_CACHE_DB_NAME);
  await chooseRadio(page, "ダメージ計算の実行場所", "オフライン(WASM)");
  await page.route("**/api/**", (route) => route.abort("connectionrefused"));

  await page.goto("/calc");

  await expect(page.getByRole("alert")).toContainText(GUIDE_PATTERN);
  await expect(page.getByText("テストモン")).toHaveCount(0);
});

test("IndexedDB が使えなくても、オンラインの計算は成功する(キャッシュ書き込みの失敗は握りつぶす)", async ({
  page,
}) => {
  await page.addInitScript(() => {
    // private mode 相当: open を失敗させる。
    Object.defineProperty(window, "indexedDB", {
      configurable: true,
      value: {
        open() {
          throw new DOMException("denied", "SecurityError");
        },
        databases() {
          return Promise.resolve([]);
        },
      },
    });
  });
  await openApp(page);
  await selectMatchup(page, SPECIES.fire.nameJa, SPECIES.water.nameJa);
  // オンライン(既定)の計算は calc-svc が無いので結果行までは見ない。マスタ・種族の取得が成功して
  // 画面にエラーが出ないことを確かめる。
  await expect(page.getByRole("alert")).toHaveCount(0);
});
