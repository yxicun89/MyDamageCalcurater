// PR2(ADR-0307): E2E 専用の pokedex フィクスチャ(pokedexFixture.ts)が、本物の pokedex-svc の
// 公開 API の契約から外れていないことを確かめる契約テスト。
//
// 契約の正は api/openapi.yaml で、Web はその生成型(src/api/openapi.gen.ts。`make gen-ts` が作る)を通して読む。
// ここでは「生成型のフィールドを過不足なく満たすか」を、
//   (a) 型レベル: 期待するキーの一覧を `satisfies (keyof Schemas[...])[]` で宣言し、足りないキーを
//       AssertNever で typecheck に検出させる(契約が増えたらこのファイルがコンパイルできなくなる)
//   (b) 実行時: 応答の各要素を型ガードに通し、キー集合が期待どおり**ちょうど**であることを確かめる
// の2段で見る。(b) の「ちょうど」が大事で、例データの `Item.effect`・`Ability.effect`・
// `MasterSpecies.learnset` が SpeciesSummary に漏れるといった、契約に無いフィールドの混入を捕まえる。
//
// フィクスチャは架空の例データ(src/master/example/)だけを使う(実マスタを持ち込まない。ADR-0002)。

import { beforeAll, describe, expect, test } from "vitest";
import type { components } from "../../src/api/openapi.gen";
import { exampleMasterSource } from "../../src/master/exampleSource";
import { MOVES_BATCH_MAX_IDS } from "../../src/master/onlineSource";
import type { MasterData } from "../../src/master/types";
import {
  SEARCH_LIMIT_DEFAULT,
  SEARCH_LIMIT_MAX,
  handlePokedexRequest,
  type FixtureRequest,
  type FixtureResponse,
} from "./pokedexFixture";

type Schemas = components["schemas"];

/** 型引数が never でなければ typecheck を失敗させる(契約に増えたキーを取りこぼさないため)。 */
type AssertNever<T extends never> = T;

// --- 期待するキーの一覧(生成型との突き合わせ) -------------------------------------------------

const ITEM_KEYS = ["id", "nameJa"] as const satisfies readonly (keyof Schemas["Item"])[];
// 省略可のキー(issue #211・ADR-0218: 効果を持つ持ち物だけが effect を伴う)。
const ITEM_OPTIONAL_KEYS = [
  "effect",
  "roles",
  "isMegaStone",
] as const satisfies readonly (keyof Schemas["Item"])[];
export type ItemKeysAreComplete = AssertNever<
  Exclude<keyof Schemas["Item"], (typeof ITEM_KEYS)[number] | (typeof ITEM_OPTIONAL_KEYS)[number]>
>;

const NATURE_KEYS = ["id", "nameJa", "plus", "minus"] as const satisfies readonly (keyof Schemas["Nature"])[];
export type NatureKeysAreComplete = AssertNever<
  Exclude<keyof Schemas["Nature"], (typeof NATURE_KEYS)[number]>
>;

const SPECIES_SUMMARY_KEYS = [
  "key",
  "dexNo",
  "form",
  "nameJa",
  "types",
] as const satisfies readonly (keyof Schemas["SpeciesSummary"])[];
export type SpeciesSummaryKeysAreComplete = AssertNever<
  Exclude<keyof Schemas["SpeciesSummary"], (typeof SPECIES_SUMMARY_KEYS)[number]>
>;

const SPECIES_DETAIL_KEYS = [
  ...SPECIES_SUMMARY_KEYS,
  "baseStats",
  "abilities",
  "learnset",
] as const satisfies readonly (keyof Schemas["SpeciesDetail"])[];
// 省略可のキー(issue #515: メガシンカ。isMega は常に返るが、古いサーバーとの互換で契約上は optional)。
const SPECIES_DETAIL_OPTIONAL_KEYS = [
  "isMega",
  "requiredItemId",
  "baseSpeciesKey",
  "baseSpeciesNameJa",
  "weightHg", // ADR-0143: 取り込み前・古いサーバーは返さない
] as const satisfies readonly (keyof Schemas["SpeciesDetail"])[];
export type SpeciesDetailKeysAreComplete = AssertNever<
  Exclude<
    keyof Schemas["SpeciesDetail"],
    (typeof SPECIES_DETAIL_KEYS)[number] | (typeof SPECIES_DETAIL_OPTIONAL_KEYS)[number]
  >
>;

const MOVE_KEYS = [
  "id",
  "nameJa",
  "type",
  "category",
  "power",
  "priority",
] as const satisfies readonly (keyof Schemas["Move"])[];
// 省略可のキー(issue 288・ADR-0223: 対象を取り込んだ技だけが target を伴う。値は single / spread)。
// ADR-0178: mechanisms(古いサーバーは返さない)・flags(取り込み前・古いサーバーは返さない)も省略可。
const MOVE_OPTIONAL_KEYS = [
  "target",
  "mechanisms",
  "flags",
  "mechanismParams", // ADR-0143: 中身の無い技・古いサーバーは返さない
  "rule",
] as const satisfies readonly (keyof Schemas["Move"])[];
export type MoveKeysAreComplete = AssertNever<
  Exclude<keyof Schemas["Move"], (typeof MOVE_KEYS)[number] | (typeof MOVE_OPTIONAL_KEYS)[number]>
>;

const ABILITY_KEYS = ["id", "nameJa"] as const satisfies readonly (keyof Schemas["Ability"])[];
// 省略可のキー(issue #211・ADR-0218: 効果を持つ特性だけが effect を伴う)。
const ABILITY_OPTIONAL_KEYS = ["effect"] as const satisfies readonly (keyof Schemas["Ability"])[];
export type AbilityKeysAreComplete = AssertNever<
  Exclude<keyof Schemas["Ability"], (typeof ABILITY_KEYS)[number] | (typeof ABILITY_OPTIONAL_KEYS)[number]>
>;

const STAT_KEYS = ["hp", "atk", "def", "spa", "spd", "spe"] as const satisfies readonly StatKeyName[];
type StatKeyName = Schemas["StatKey"];

const POKE_TYPES: readonly Schemas["PokeType"][] = [
  "normal",
  "fire",
  "water",
  "electric",
  "grass",
  "ice",
  "fighting",
  "poison",
  "ground",
  "flying",
  "psychic",
  "bug",
  "rock",
  "ghost",
  "dragon",
  "dark",
  "steel",
  "fairy",
];

const MOVE_CATEGORIES: readonly Schemas["MoveCategory"][] = ["physical", "special", "status"];

// --- 実行時の型ガード ---------------------------------------------------------------------------

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isString(value: unknown): value is string {
  return typeof value === "string";
}

function isNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

/** 余分なフィールドが無く、期待するキーがちょうど揃っていること。 */
function hasExactKeys(value: unknown, keys: readonly string[]): boolean {
  return isRecord(value) && [...Object.keys(value)].sort().join() === [...keys].sort().join();
}

/** 必須のキーがすべてあり、それ以外は省略可のキーだけであること。 */
function hasKeys(value: unknown, required: readonly string[], optional: readonly string[]): boolean {
  if (!isRecord(value)) {
    return false;
  }
  const keys = Object.keys(value);
  return (
    required.every((key) => keys.includes(key)) &&
    keys.every((key) => required.includes(key) || optional.includes(key))
  );
}

/** effect は省略するか、オブジェクト(MasterEffect)。null は返さない(ADR-0218)。 */
function isOptionalEffect(value: Record<string, unknown>): boolean {
  return !("effect" in value) || isRecord(value.effect);
}

function isStatBlock(value: unknown): value is Schemas["StatBlock"] {
  return (
    isRecord(value) && STAT_KEYS.every((key) => isNumber(value[key])) && hasExactKeys(value, [...STAT_KEYS])
  );
}

function isPokeType(value: unknown): value is Schemas["PokeType"] {
  return isString(value) && POKE_TYPES.includes(value as Schemas["PokeType"]);
}

function isItem(value: unknown): value is Schemas["Item"] {
  return (
    hasKeys(value, [...ITEM_KEYS], [...ITEM_OPTIONAL_KEYS]) &&
    isRecord(value) &&
    isString(value.id) &&
    isString(value.nameJa) &&
    isOptionalEffect(value) &&
    // ADR-0175: pokedex-svc は roles・isMegaStone を常に返す。
    Array.isArray(value.roles) &&
    value.roles.every((role) => role === "attacker" || role === "defender") &&
    typeof value.isMegaStone === "boolean"
  );
}

function isAbility(value: unknown): value is Schemas["Ability"] {
  return (
    hasKeys(value, [...ABILITY_KEYS], [...ABILITY_OPTIONAL_KEYS]) &&
    isRecord(value) &&
    isString(value.id) &&
    isString(value.nameJa) &&
    isOptionalEffect(value)
  );
}

function isNature(value: unknown): value is Schemas["Nature"] {
  if (!hasExactKeys(value, [...NATURE_KEYS]) || !isRecord(value)) {
    return false;
  }
  const isStatOrNull = (item: unknown): boolean =>
    item === null || (isString(item) && (STAT_KEYS as readonly string[]).includes(item));
  return (
    isString(value.id) && isString(value.nameJa) && isStatOrNull(value.plus) && isStatOrNull(value.minus)
  );
}

function isSpeciesSummaryShape(value: unknown): boolean {
  return (
    isRecord(value) &&
    isString(value.key) &&
    /^[0-9]{4}-[0-9]{3}$/.test(value.key) &&
    isNumber(value.dexNo) &&
    isNumber(value.form) &&
    isString(value.nameJa) &&
    Array.isArray(value.types) &&
    value.types.length > 0 &&
    value.types.every(isPokeType)
  );
}

function isSpeciesSummary(value: unknown): value is Schemas["SpeciesSummary"] {
  return hasExactKeys(value, [...SPECIES_SUMMARY_KEYS]) && isSpeciesSummaryShape(value);
}

function isSpeciesDetail(value: unknown): value is Schemas["SpeciesDetail"] {
  return (
    hasKeys(value, [...SPECIES_DETAIL_KEYS], [...SPECIES_DETAIL_OPTIONAL_KEYS]) &&
    isSpeciesSummaryShape(value) &&
    isRecord(value) &&
    isStatBlock(value.baseStats) &&
    Array.isArray(value.abilities) &&
    value.abilities.every(isAbility) &&
    Array.isArray(value.learnset) &&
    value.learnset.every(isString) &&
    // メガの項目(issue 515)は省略可。在るときは型まで確かめる(requiredItemId は null も可)。
    (value.isMega === undefined || typeof value.isMega === "boolean") &&
    (value.requiredItemId === undefined || value.requiredItemId === null || isString(value.requiredItemId)) &&
    // ADR-0175: 基本種の項目は pokedex-svc が null でも常に返す。
    (value.baseSpeciesKey === null || isString(value.baseSpeciesKey)) &&
    (value.baseSpeciesNameJa === null || isString(value.baseSpeciesNameJa))
  );
}

function isMove(value: unknown): value is Schemas["Move"] {
  return (
    hasKeys(value, [...MOVE_KEYS], [...MOVE_OPTIONAL_KEYS]) &&
    isRecord(value) &&
    isString(value.id) &&
    isString(value.nameJa) &&
    isPokeType(value.type) &&
    isString(value.category) &&
    MOVE_CATEGORIES.includes(value.category as Schemas["MoveCategory"]) &&
    isNumber(value.power) &&
    isNumber(value.priority) &&
    (value.target === undefined || value.target === "single" || value.target === "spread")
  );
}

function isErrorBody(value: unknown): value is Schemas["Error"] {
  return (
    hasExactKeys(value, ["code", "message"]) &&
    isRecord(value) &&
    isString(value.code) &&
    isString(value.message)
  );
}

// --- リクエストの組み立て -----------------------------------------------------------------------

const DEVICE_ID = "11111111-1111-4111-8111-111111111111";
const SESSION_ID = "22222222-2222-4222-8222-222222222222";

const DEFAULT_HEADERS: Readonly<Record<string, string | undefined>> = {
  "x-device-id": DEVICE_ID,
  "x-session-id": SESSION_ID,
};

function get(
  path: string,
  query: Readonly<Record<string, string | readonly string[]>> = {},
  headers: Readonly<Record<string, string | undefined>> = DEFAULT_HEADERS,
): FixtureRequest {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    for (const item of typeof value === "string" ? [value] : value) {
      params.append(key, item);
    }
  }
  return { method: "GET", path, query: params, headers };
}

let master: MasterData;

beforeAll(async () => {
  master = await exampleMasterSource.load();
});

function call(request: FixtureRequest): FixtureResponse {
  return handlePokedexRequest(master, request);
}

/** 200 を期待して本文を配列として受け取る。 */
function okArray(request: FixtureRequest): unknown[] {
  const response = call(request);
  expect(response.status).toBe(200);
  expect(Array.isArray(response.body)).toBe(true);
  return response.body as unknown[];
}

function fireSpeciesKey(): string {
  const species = master.species.find((candidate) => candidate.nameJa === "テストほのお");
  if (species === undefined) {
    throw new Error("例データに テストほのお が無い");
  }
  return species.key;
}

// --- 正常系: 契約のスキーマちょうど ---------------------------------------------------------------

describe("正常系の本文が公開 API のスキーマを過不足なく満たす(api/openapi.yaml)", () => {
  test("GET /api/pokedex/items は Item[](効果を持つ持ち物だけ effect を伴い、形は PascalCase)", () => {
    const body = okArray(get("/api/pokedex/items", { limit: String(SEARCH_LIMIT_MAX) }));
    expect(body.length).toBe(master.items.length);
    for (const item of body) {
      expect(isItem(item), `Item の契約に合わない: ${JSON.stringify(item)}`).toBe(true);
      // 契約上 effect は省略可(ADR-0218): 効果を持つ持ち物だけ伴い、キーは DB の形(PascalCase)。
      const source = master.items.find((candidate) => candidate.id === (item as Schemas["Item"]).id);
      expect(
        Object.hasOwn(item as object, "effect"),
        `effect の有無が例データと違う: ${JSON.stringify(item)}`,
      ).toBe(source?.effect != null);
      if ("effect" in (item as object)) {
        expect(Object.keys((item as Schemas["Item"]).effect ?? {}).every((key) => /^[A-Z]/.test(key))).toBe(
          true,
        );
      }
    }
    expect(new Set(body.map((item) => (item as Schemas["Item"]).id))).toEqual(
      new Set(master.items.map((item) => item.id)),
    );
    // effects を有効にする(ADR-0322)ので、効果を持つ持ち物が1件以上ある。
    expect(body.some((item) => "effect" in (item as object))).toBe(true);
  });

  test("GET /api/pokedex/natures は Nature[](全件・無補正は plus/minus が null)", () => {
    const body = okArray(get("/api/pokedex/natures"));
    expect(body.length).toBe(master.natures.length);
    for (const nature of body) {
      expect(isNature(nature), `Nature の契約に合わない: ${JSON.stringify(nature)}`).toBe(true);
    }
    const neutral = body.find((nature) => (nature as Schemas["Nature"]).plus === null);
    expect(neutral, "例データの無補正性格が null で返っていない").toBeDefined();
  });

  test("GET /api/pokedex/species は SpeciesSummary[](baseStats・abilities・learnset を含めない)", () => {
    const body = okArray(get("/api/pokedex/species", { q: "テスト", limit: String(SEARCH_LIMIT_MAX) }));
    expect(body.length).toBe(master.species.length);
    for (const summary of body) {
      expect(isSpeciesSummary(summary), `SpeciesSummary の契約に合わない: ${JSON.stringify(summary)}`).toBe(
        true,
      );
    }
  });

  test("GET /api/pokedex/species/{key} は SpeciesDetail(abilities は Ability の配列。ID の配列ではない)", () => {
    const response = call(get(`/api/pokedex/species/${fireSpeciesKey()}`));
    expect(response.status).toBe(200);
    expect(
      isSpeciesDetail(response.body),
      `SpeciesDetail の契約に合わない: ${JSON.stringify(response.body)}`,
    ).toBe(true);
    const detail = response.body as Schemas["SpeciesDetail"];
    const source = master.species.find((candidate) => candidate.key === detail.key);
    expect(detail.learnset).toEqual(source?.learnset);
    expect(detail.abilities.map((ability) => ability.id)).toEqual(source?.abilities);
    for (const ability of detail.abilities) {
      const abilitySource = master.abilities.find((candidate) => candidate.id === ability.id);
      expect(
        Object.hasOwn(ability, "effect"),
        `effect の有無が例データと違う: ${JSON.stringify(ability)}`,
      ).toBe(abilitySource?.effect != null);
    }
  });

  test("GET /api/pokedex/moves/batch は Move[](learnset の ID をそのまま解決できる)", () => {
    const detail = call(get(`/api/pokedex/species/${fireSpeciesKey()}`)).body as Schemas["SpeciesDetail"];
    const ids = detail.learnset ?? [];
    expect(ids.length).toBeGreaterThan(0);
    const body = okArray(get("/api/pokedex/moves/batch", { ids }));
    for (const move of body) {
      expect(isMove(move), `Move の契約に合わない: ${JSON.stringify(move)}`).toBe(true);
    }
    expect(body.map((move) => (move as Schemas["Move"]).id)).toEqual(ids);
  });

  test("GET /healthz は 200(Playwright の webServer の待ち受け確認)", () => {
    expect(call(get("/healthz")).status).toBe(200);
  });
});

// --- 正常系: 検索・解決のふるまい -----------------------------------------------------------------

describe("検索と解決のふるまいが契約どおり", () => {
  test("species の q は日本語名の前方一致(部分一致では引けない)", () => {
    const hit = okArray(get("/api/pokedex/species", { q: "テストみ" }));
    expect(hit.map((summary) => (summary as Schemas["SpeciesSummary"]).nameJa)).toEqual(["テストみず"]);
    expect(okArray(get("/api/pokedex/species", { q: "みず" }))).toEqual([]);
  });

  test("species の並びは key 昇順で、q 省略は全件(limit まで)", () => {
    const keys = okArray(get("/api/pokedex/species", { limit: String(SEARCH_LIMIT_MAX) })).map(
      (summary) => (summary as Schemas["SpeciesSummary"]).key,
    );
    expect(keys).toEqual([...keys].sort());
    expect(keys.length).toBe(master.species.length);
  });

  test("species の limit は件数を切る", () => {
    expect(okArray(get("/api/pokedex/species", { q: "テスト", limit: "2" })).length).toBe(2);
  });

  test("limit を省くと既定(50)が効く", () => {
    // 例データは50件未満なので、既定でも全件返る(既定値が 0 や 1 になっていないことの確認)。
    expect(okArray(get("/api/pokedex/species", { q: "テスト" })).length).toBe(
      Math.min(master.species.length, SEARCH_LIMIT_DEFAULT),
    );
  });

  test("items の並びは nameJa の日本語の照合順序の昇順", () => {
    const names = okArray(get("/api/pokedex/items")).map((item) => (item as Schemas["Item"]).nameJa);
    expect(names).toEqual([...names].sort((a, b) => a.localeCompare(b, "ja")));
  });

  test("natures の並びは id 昇順", () => {
    const ids = okArray(get("/api/pokedex/natures")).map((nature) => (nature as Schemas["Nature"]).id);
    expect(ids).toEqual([...ids].sort());
  });

  test("moves/batch はマスタに無い ID を黙って省く(エラーにしない)", () => {
    const known = master.moves[0];
    if (known === undefined) {
      throw new Error("例データに技が無い");
    }
    const body = okArray(get("/api/pokedex/moves/batch", { ids: [known.id, "no-such-move"] }));
    expect(body.map((move) => (move as Schemas["Move"]).id)).toEqual([known.id]);
  });

  test("moves/batch は重複した ID を重複したまま、ids の順で返す", () => {
    const [first, second] = master.moves;
    if (first === undefined || second === undefined) {
      throw new Error("例データの技が2件に満たない");
    }
    const body = okArray(get("/api/pokedex/moves/batch", { ids: [second.id, first.id, second.id] }));
    expect(body.map((move) => (move as Schemas["Move"]).id)).toEqual([second.id, first.id, second.id]);
  });
});

// --- 異常系 -------------------------------------------------------------------------------------

describe("異常系は本物と同じ code を返す(ErrorCode の対応表)", () => {
  function expectError(response: FixtureResponse, status: number, code: Schemas["ErrorCode"]): void {
    expect(response.status).toBe(status);
    expect(isErrorBody(response.body), `Error の契約に合わない: ${JSON.stringify(response.body)}`).toBe(true);
    expect((response.body as Schemas["Error"]).code).toBe(code);
  }

  test("端末 ID のヘッダが無ければ 400 missing_header", () => {
    expectError(call(get("/api/pokedex/natures", {}, { "x-session-id": SESSION_ID })), 400, "missing_header");
  });

  test("セッション ID のヘッダが空でも 400 missing_header", () => {
    const headers = { "x-device-id": DEVICE_ID, "x-session-id": "" };
    expectError(call(get("/api/pokedex/natures", {}, headers)), 400, "missing_header");
  });

  test("ヘッダが UUID でなければ 400 invalid_header", () => {
    const headers = { "x-device-id": "not-a-uuid", "x-session-id": SESSION_ID };
    expectError(call(get("/api/pokedex/natures", {}, headers)), 400, "invalid_header");
  });

  test("limit が範囲外・整数でなければ 400 invalid_input", () => {
    for (const limit of ["0", String(SEARCH_LIMIT_MAX + 1), "abc", "1.5"]) {
      expectError(call(get("/api/pokedex/species", { limit })), 400, "invalid_input");
      expectError(call(get("/api/pokedex/items", { limit })), 400, "invalid_input");
    }
  });

  test("SpeciesKey の形式が違えば 400 invalid_input", () => {
    expectError(call(get("/api/pokedex/species/9001")), 400, "invalid_input");
    expectError(call(get("/api/pokedex/species/abcd-000")), 400, "invalid_input");
  });

  test("マスタに無い SpeciesKey は 404 not_found", () => {
    expectError(call(get("/api/pokedex/species/9999-000")), 404, "not_found");
  });

  test("moves/batch の ids が無い・0件・上限超過なら 400 invalid_input", () => {
    expectError(call(get("/api/pokedex/moves/batch")), 400, "invalid_input");
    expectError(call(get("/api/pokedex/moves/batch", { ids: [] })), 400, "invalid_input");
    const tooMany = Array.from({ length: MOVES_BATCH_MAX_IDS + 1 }, (_, index) => `id${String(index)}`);
    expectError(call(get("/api/pokedex/moves/batch", { ids: tooMany })), 400, "invalid_input");
  });

  test("ちょうど上限(64件)の ids は通る(境界)", () => {
    const known = master.moves[0];
    if (known === undefined) {
      throw new Error("例データに技が無い");
    }
    const ids = Array.from({ length: MOVES_BATCH_MAX_IDS }, () => known.id);
    expect(okArray(get("/api/pokedex/moves/batch", { ids })).length).toBe(MOVES_BATCH_MAX_IDS);
  });

  test("担当外のパス・GET 以外のメソッドは 404 not_found", () => {
    expectError(call(get("/api/pokedex/nope")), 404, "not_found");
    expectError(call(get("/api/calc/bulk")), 404, "not_found");
    expectError(call({ ...get("/api/pokedex/natures"), method: "POST" }), 404, "not_found");
  });
});
