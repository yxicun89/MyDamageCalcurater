// P4-5: API 実装 createApiEngine(ADR-0301 §1〜§4)。画面の入力(解決済みの実体の DTO。engine/types.ts)を
// API のリクエスト(ID。openapi.gen.ts の生成型)に写して POST し、応答を DTO に戻す。
// 「ID → 実体」の解決は MasterData の読み込み時に済んでいるので、ここは実体 → ID の写像だけを持つ
// (ADR-0301 §1: 両経路が同じ DTO を共有し、解決層は MasterData の1か所にする)。
// エラーは境界の封筒({code, message})のまま運ぶ。通信できない・応答が読めないときは engine_unavailable にし、
// 自動でオフラインへフォールバックしない(ADR-0301 §4。切り替えは利用者が選ぶ)。

import { apiEngineText, engineAbortText } from "../i18n/ja";
import {
  REQUEST_ABORTED_CODE,
  type BulkRequest,
  type BulkResult,
  type BulkRow,
  type CalcEngine,
  type CalcRequest,
  type CalcResult,
  type EngineResult,
  type Field,
  type Individual,
  type Nature,
  type ReverseCandidate,
  type ReverseRequest,
  type ReverseResult,
} from "../engine/types";
import type { MasterData } from "../master/types";
import type { ClientIds } from "./clientIds";
import type { components } from "./openapi.gen";

type Schemas = components["schemas"];

/** createApiEngine の引数。master は性格の一覧だけ(natureId の写像に使う。ADR-0301 §2)。 */
export interface CreateApiEngineInput {
  /** API の基点 URL(末尾はスラッシュ1つ。api/config.ts の apiBaseUrl() で作る)。 */
  readonly baseUrl: string;
  /** 注入する fetch(テストは fake、実行時は globalThis.fetch)。 */
  readonly fetch: typeof fetch;
  readonly master: Pick<MasterData, "natures">;
  readonly ids: ClientIds;
}

/** 性格補正(Nature)が無補正(plus・minus とも "")かどうか。 */
function isNeutralNature(nature: Nature): boolean {
  return nature.plus === "" && nature.minus === "";
}

/**
 * signal が abort 済みか(issue 113)。関数越しにすることで、await をまたいだ後の再チェックを
 * TypeScript の(誤った)narrowing で「あり得ない比較」と拒否されないようにする(signal.aborted は
 * ミュータブルな getter で、直前のチェックの後も変わり得るため)。
 */
function isAborted(signal: AbortSignal | undefined): boolean {
  return signal?.aborted === true;
}

/**
 * (plus, minus) から natureId を選ぶ(ADR-0301 §2、services/calc/README.md の Store.NatureID と同じ規則)。
 * 無補正は「無補正の性格(マスタの plus == minus。null 同士だけでなく、同じステータスの plus/minus も含む)を
 * ID の昇順で並べた最初」、それ以外は (plus, minus) が一致する性格。該当なしは undefined。
 */
function resolveNatureId(
  natures: CreateApiEngineInput["master"]["natures"],
  nature: Nature,
): string | undefined {
  if (isNeutralNature(nature)) {
    const neutralIds = natures.filter((entry) => entry.plus === entry.minus).map((entry) => entry.id);
    return [...neutralIds].sort()[0];
  }
  return natures.find((entry) => entry.plus === nature.plus && entry.minus === nature.minus)?.id;
}

/** unknown_nature の失敗(natureId が決められなかったとき。fetch しない。ADR-0301 §2)。 */
function unknownNatureError<T>(): EngineResult<T> {
  return { ok: false, error: { code: "unknown_nature", message: apiEngineText.unknownNature } };
}

/** engine_unavailable の失敗(通信できない・応答が読めない・エラー本文の形が不正。ADR-0301 §4)。 */
function unavailableError<T>(): EngineResult<T> {
  return { ok: false, error: { code: "engine_unavailable", message: apiEngineText.unavailable } };
}

/**
 * request_aborted の失敗(画面が新しい入力で取り消した計算。issue 113、ADR-0300 §11・ADR-0301 §4 追記)。
 * engine_unavailable(API が使えない)とは区別する: 取り消しは engine・API の失敗ではない。
 */
function abortedError<T>(): EngineResult<T> {
  return { ok: false, error: { code: REQUEST_ABORTED_CODE, message: engineAbortText.aborted } };
}

/** invalid_preset の失敗(engine のカスタムプリセット定義は API に送れない。ADR-0301 §2)。fetch しない。 */
function invalidPresetError<T>(): EngineResult<T> {
  return { ok: false, error: { code: "invalid_preset", message: apiEngineText.invalidPreset } };
}

/** Individual(実体)を API の Individual(ID)に写す。性格が解決できなければ undefined。 */
function mapIndividual(
  natures: CreateApiEngineInput["master"]["natures"],
  individual: Individual,
): Schemas["Individual"] | undefined {
  const natureId = resolveNatureId(natures, individual.nature);
  if (natureId === undefined) {
    return undefined;
  }
  const mapped: Schemas["Individual"] = {
    speciesKey: individual.species.key,
    level: individual.level,
    natureId,
    abilityId: individual.ability.id === "" ? null : individual.ability.id,
    itemId: individual.item === null ? null : individual.item.id,
    sp: individual.sp,
  };
  if (individual.ranks !== undefined) {
    mapped.ranks = individual.ranks;
  }
  if (individual.teraType !== undefined) {
    // engine DTO の teraType は string(""は無補正)。API は PokeType の enum で、無補正は null(ADR-0301 §2)。
    mapped.teraType = individual.teraType === "" ? null : (individual.teraType as Schemas["PokeType"]);
  }
  if (individual.status !== undefined && individual.status !== "") {
    // engine DTO の status は string。API は StatusCondition の enum で、既定(未指定)が "none" 相当。
    mapped.status = individual.status as Schemas["StatusCondition"];
  }
  return mapped;
}

/** 計算 API のパス(api/openapi.yaml の paths。基点 URL からの相対。基点は末尾が "/")。 */
const CALC_PATHS = {
  calc: "api/calc",
  bulk: "api/calc/bulk",
  reverse: "api/calc/reverse",
} as const;

/** engine の Field を API の FieldState に写す(天候・地形は同じドメインの値を共有する。省略は省略のまま)。 */
function mapField(field: Field | undefined): Schemas["FieldState"] | undefined {
  if (field === undefined) {
    return undefined;
  }
  const mapped: Schemas["FieldState"] = {};
  // engine DTO の weather / terrain は "" を「なし」として受ける(ADR-0011 §4)が、API は enum で "" を拒否する。
  // "" は送らず、API の既定(none)に任せる(status と同じ扱い。両モードで同じ入力が同じ結果になるように)。
  if (field.weather !== undefined && field.weather !== "") {
    mapped.weather = field.weather as Schemas["Weather"];
  }
  if (field.terrain !== undefined && field.terrain !== "") {
    mapped.terrain = field.terrain as Schemas["Terrain"];
  }
  if (field.attackerScreens !== undefined) {
    mapped.attackerScreens = field.attackerScreens;
  }
  if (field.defenderScreens !== undefined) {
    mapped.defenderScreens = field.defenderScreens;
  }
  return mapped;
}

/** critical を options.critical に写す。critical が未指定なら options 自体を送らない(サーバーの既定 false)。 */
function mapOptions(critical: boolean | undefined): Schemas["CalcOptions"] | undefined {
  return critical === undefined ? undefined : { critical };
}

/** API の CalcResult を DTO の CalcResult に写す。ko.chancePercent は省略時 0(確定・倒せないときの値)。 */
function mapCalcResult(result: Schemas["CalcResult"]): CalcResult {
  return {
    rolls: result.rolls,
    minDamage: result.minDamage,
    maxDamage: result.maxDamage,
    minPercent: result.minPercent,
    maxPercent: result.maxPercent,
    defenderHP: result.defenderHP,
    effectiveness: result.effectiveness,
    stab: result.stab,
    category: result.category,
    ko: {
      hits: result.ko.hits,
      guaranteed: result.ko.guaranteed,
      chancePercent: result.ko.chancePercent ?? 0,
      displayChancePercent: result.ko.displayChancePercent,
    },
    // 「未対応」の印(ADR-0123)。engine が決めた並びのまま素通しする(並べ替え・重複除去をしない。
    // ADR-0300 §8)。target・reason の値は契約と DTO で同じ文字列なので、写しは代入だけでよい。
    unsupported: result.unsupported,
  };
}

/** API の NatureModifier(null は無補正)を DTO の Nature("" は無補正)に写す。 */
function mapNatureModifier(nature: Schemas["NatureModifier"]): Nature {
  return { plus: nature.plus ?? "", minus: nature.minus ?? "" };
}

/** API の BulkCalcRow を DTO の BulkRow に写す(natureId は画面が使わないので捨てる。ADR-0301 §2)。 */
function mapBulkRow(row: Schemas["BulkCalcRow"]): BulkRow {
  return {
    preset: row.preset,
    presetLabel: row.presetLabel,
    itemId: row.itemId ?? "",
    defender: {
      sp: row.defender.sp,
      nature: mapNatureModifier(row.defender.nature),
      stats: row.defender.stats,
    },
    result: mapCalcResult(row.result),
  };
}

/** API の BulkCalcResult を DTO の BulkResult に写す。 */
function mapBulkResult(result: Schemas["BulkCalcResult"]): BulkResult {
  return { defenderSpeciesKey: result.defenderSpeciesKey, rows: result.rows.map(mapBulkRow) };
}

/** API の ReverseCandidate を DTO の ReverseCandidate に写す(natureId は捨てる)。 */
function mapReverseCandidate(candidate: Schemas["ReverseCandidate"]): ReverseCandidate {
  return {
    natureClass: candidate.natureClass,
    nature: mapNatureModifier(candidate.nature),
    itemId: candidate.itemId ?? "",
    ranges: candidate.ranges,
    spCount: candidate.spCount,
    exact: candidate.exact,
    mismatch: candidate.mismatch,
    support: candidate.support,
    minPercent: candidate.minPercent,
    maxPercent: candidate.maxPercent,
    unsupported: candidate.unsupported,
  };
}

/** API の ReverseResult を DTO の ReverseResult に写す。 */
function mapReverseResult(result: Schemas["ReverseResult"]): ReverseResult {
  return {
    side: result.side,
    stat: result.stat,
    assumedHpSp: result.assumedHpSp,
    exactCount: result.exactCount,
    candidates: result.candidates.map(mapReverseCandidate),
  };
}

/** サーバーのエラー本文({code, message})の形をしているかの型ガード。 */
function isErrorBody(value: unknown): value is Schemas["Error"] {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const record = value as Record<string, unknown>;
  return typeof record.code === "string" && typeof record.message === "string";
}

// ---- 応答の実行時検証(issue 67、ADR-0301 §4 追記) ----
//
// HTTP 200 で JSON として読めても、本文が契約(api/openapi.yaml)の応答の形とは限らない。写像関数
// (mapCalcResult など)が実際に読むフィールドだけを、存在すること・JS 上の種類が合うことを再帰的に
// 検査する型ガードをここに置く。列挙の値そのもの・数値の範囲・配列の件数・余分なフィールドは見ない
// (サーバーが語彙を増やしても Web を壊さないため)。写像が `??` で既定値を補うフィールド
// (ko.chancePercent・itemId・nature.plus/minus)は、欠落・null を許す。

/** オブジェクト(配列・null を除く)かどうか。 */
function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isNumber(value: unknown): value is number {
  return typeof value === "number";
}

function isBoolean(value: unknown): value is boolean {
  return typeof value === "boolean";
}

function isString(value: unknown): value is string {
  return typeof value === "string";
}

/** 配列で、すべての要素が guard を満たすか。 */
function isArrayOf<T>(value: unknown, guard: (item: unknown) => item is T): value is T[] {
  return Array.isArray(value) && value.every(guard);
}

/** 写像が `??` で既定値を補う・捨てるフィールド用: 欠落・null は許し、あるなら型が合うこと。 */
function isOptionalNullable(value: unknown, guard: (item: unknown) => boolean): boolean {
  return value === undefined || value === null || guard(value);
}

const STAT_KEYS = ["hp", "atk", "def", "spa", "spd", "spe"] as const;

/** 6ステータスの値(種族値・実数値・SP などに共用)。すべて数値。 */
function isStatBlock(value: unknown): value is Schemas["StatBlock"] {
  return isRecord(value) && STAT_KEYS.every((key) => isNumber(value[key]));
}

/**
 * 性格補正の構造値。plus/minus は契約では必須だが、写像(mapNatureModifier)は `?? ""` で
 * 既定値を補うので、欠落・null を許す(ADR-0301 §4 追記)。
 */
function isNatureModifier(value: unknown): value is Schemas["NatureModifier"] {
  return (
    isRecord(value) && isOptionalNullable(value.plus, isString) && isOptionalNullable(value.minus, isString)
  );
}

/**
 * 確定数/乱数n発。chancePercent は写像(mapCalcResult)が `?? 0` で既定値を補うので、
 * 他の欠落許容フィールド(itemId・nature.plus/minus)と同じく欠落・null を許す(ADR-0301 §4 追記)。
 */
function isKoChance(value: unknown): value is Schemas["KOChance"] {
  return (
    isRecord(value) &&
    isNumber(value.hits) &&
    isBoolean(value.guaranteed) &&
    isOptionalNullable(value.chancePercent, isNumber) &&
    isNumber(value.displayChancePercent)
  );
}

/** mapCalcResult が読むフィールドをすべて検査する(入れ子の ko も同じ規則)。 */
function isCalcResult(value: unknown): value is Schemas["CalcResult"] {
  return (
    isRecord(value) &&
    isArrayOf(value.rolls, isNumber) &&
    isNumber(value.minDamage) &&
    isNumber(value.maxDamage) &&
    isNumber(value.minPercent) &&
    isNumber(value.maxPercent) &&
    isNumber(value.defenderHP) &&
    isNumber(value.effectiveness) &&
    isBoolean(value.stab) &&
    isString(value.category) &&
    isKoChance(value.ko)
  );
}

/** BulkDefender: sp・stats は StatBlock、nature は NatureModifier(natureId は捨てるので見ない)。 */
function isBulkDefender(value: unknown): value is Schemas["BulkDefender"] {
  return (
    isRecord(value) && isStatBlock(value.sp) && isNatureModifier(value.nature) && isStatBlock(value.stats)
  );
}

/** BulkCalcRow: itemId は省略時 "" で埋める(欠落・null を許す)。result は CalcResult と同じ規則で再帰的に検査。 */
function isBulkCalcRow(value: unknown): value is Schemas["BulkCalcRow"] {
  return (
    isRecord(value) &&
    isString(value.preset) &&
    isString(value.presetLabel) &&
    isOptionalNullable(value.itemId, isString) &&
    isBulkDefender(value.defender) &&
    isCalcResult(value.result)
  );
}

/** mapBulkResult が読むフィールドをすべて検査する。 */
function isBulkCalcResult(value: unknown): value is Schemas["BulkCalcResult"] {
  return isRecord(value) && isString(value.defenderSpeciesKey) && isArrayOf(value.rows, isBulkCalcRow);
}

/** SP の区間(min・max とも数値)。 */
function isSpRange(value: unknown): value is Schemas["SPRange"] {
  return isRecord(value) && isNumber(value.min) && isNumber(value.max);
}

/** ReverseCandidate: itemId は省略時 "" で埋める(欠落・null を許す)。 */
function isReverseCandidate(value: unknown): value is Schemas["ReverseCandidate"] {
  return (
    isRecord(value) &&
    isString(value.natureClass) &&
    isNatureModifier(value.nature) &&
    isOptionalNullable(value.itemId, isString) &&
    isArrayOf(value.ranges, isSpRange) &&
    isNumber(value.spCount) &&
    isBoolean(value.exact) &&
    isNumber(value.mismatch) &&
    isNumber(value.support) &&
    isNumber(value.minPercent) &&
    isNumber(value.maxPercent)
  );
}

/** mapReverseResult が読むフィールドをすべて検査する。 */
function isReverseResult(value: unknown): value is Schemas["ReverseResult"] {
  return (
    isRecord(value) &&
    isString(value.side) &&
    isString(value.stat) &&
    isNumber(value.assumedHpSp) &&
    isNumber(value.exactCount) &&
    isArrayOf(value.candidates, isReverseCandidate)
  );
}

/**
 * 計算の API 実装(ADR-0301)。画面が渡す実体の DTO を実体 → ID に写して POST し、応答を DTO に戻す。
 * 性格が解決できない・engine のカスタムプリセット定義は fetch せずに失敗を返す。通信・応答の失敗は
 * engine_unavailable にし、自動でオフラインへフォールバックしない(切り替えは利用者が選ぶ。ADR-0301 §4)。
 */
export function createApiEngine(input: CreateApiEngineInput): CalcEngine {
  const { baseUrl, fetch: fetchImpl, master, ids } = input;

  /**
   * JSON を POST し、応答(成功の値、または境界のエラー封筒)を返す。例外を投げない。
   * signal(issue 113、ADR-0300 §11・ADR-0301 §4 追記): 渡さなければ init に signal を付けない。
   * 呼ぶ前に abort 済みなら fetch せずに request_aborted を返す。fetch・本文の読み取りが abort で
   * 失敗したときも request_aborted にする(abort していない通信失敗は従来どおり engine_unavailable)。
   * guard(issue 67、ADR-0301 §4 追記): 2xx の本文を実行時に検証し、契約外なら engine_unavailable
   * にする(本文が読めたうえでの契約違反は、abort 済みでも engine_unavailable。通信は成立しているため)。
   */
  async function postJson<T>(
    path: string,
    body: unknown,
    guard: (value: unknown) => value is T,
    signal?: AbortSignal,
  ): Promise<EngineResult<T>> {
    if (isAborted(signal)) {
      return abortedError();
    }
    let response: Response;
    try {
      const init: RequestInit = {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Device-Id": ids.deviceId,
          "X-Session-Id": ids.sessionId,
        },
        body: JSON.stringify(body),
      };
      if (signal !== undefined) {
        init.signal = signal;
      }
      response = await fetchImpl(`${baseUrl}${path}`, init);
    } catch {
      return isAborted(signal) ? abortedError() : unavailableError();
    }
    let parsed: unknown;
    try {
      parsed = await response.json();
    } catch {
      return isAborted(signal) ? abortedError() : unavailableError();
    }
    if (!response.ok) {
      return isErrorBody(parsed)
        ? { ok: false, error: { code: parsed.code, message: parsed.message } }
        : unavailableError();
    }
    if (!guard(parsed)) {
      return unavailableError();
    }
    return { ok: true, value: parsed };
  }

  return {
    async calc(request: CalcRequest, signal?: AbortSignal): Promise<EngineResult<CalcResult>> {
      const attacker = mapIndividual(master.natures, request.attacker);
      const defender = mapIndividual(master.natures, request.defender);
      if (attacker === undefined || defender === undefined) {
        return unknownNatureError();
      }
      const body: Schemas["CalcRequest"] = {
        format: request.format,
        attacker,
        defender,
        moveId: request.move.id,
      };
      const field = mapField(request.field);
      if (field !== undefined) {
        body.field = field;
      }
      const options = mapOptions(request.critical);
      if (options !== undefined) {
        body.options = options;
      }
      const response = await postJson(CALC_PATHS.calc, body, isCalcResult, signal);
      if (!response.ok) {
        return response;
      }
      return { ok: true, value: mapCalcResult(response.value) };
    },

    async calcBulk(request: BulkRequest, signal?: AbortSignal): Promise<EngineResult<BulkResult>> {
      // engine のカスタムプリセット定義(SP・性格を任意に決めたもの)は API の DefenderPreset(文字列)で
      // 表せない(ADR-0301 §2)。presetKeys(名前の一覧)は API の presets にそのまま写す。
      if (request.presets !== undefined) {
        return invalidPresetError();
      }
      const attacker = mapIndividual(master.natures, request.attacker);
      if (attacker === undefined) {
        return unknownNatureError();
      }
      const body: Schemas["BulkCalcRequest"] = {
        format: request.format,
        attacker,
        defenderSpeciesKey: request.defenderSpecies.key,
        moveId: request.move.id,
      };
      const field = mapField(request.field);
      if (field !== undefined) {
        body.field = field;
      }
      const options = mapOptions(request.critical);
      if (options !== undefined) {
        body.options = options;
      }
      if (request.presetKeys !== undefined) {
        body.presets = request.presetKeys as Schemas["DefenderPreset"][];
      }
      if (request.itemVariants !== undefined) {
        body.itemVariants = request.itemVariants.map((item) => item?.id ?? null);
      }
      const response = await postJson(CALC_PATHS.bulk, body, isBulkCalcResult, signal);
      if (!response.ok) {
        return response;
      }
      return { ok: true, value: mapBulkResult(response.value) };
    },

    async calcReverse(request: ReverseRequest, signal?: AbortSignal): Promise<EngineResult<ReverseResult>> {
      const known = mapIndividual(master.natures, request.known);
      if (known === undefined) {
        return unknownNatureError();
      }
      // maxCandidates は送らない(API の既定は無制限。画面は切らずに全候補を受け取る。ADR-0301 §2)。
      const body: Omit<Schemas["ReverseRequest"], "maxCandidates"> = {
        format: request.format ?? "single",
        side: request.side,
        known,
        unknownSpeciesKey: request.unknownSpecies.key,
        moveId: request.move.id,
        // API の observations は可変配列(生成型)。DTO は readonly なので新しい配列にコピーする。
        observations: [...request.observations],
      };
      const field = mapField(request.field);
      if (field !== undefined) {
        body.field = field;
      }
      const options = mapOptions(request.critical);
      if (options !== undefined) {
        body.options = options;
      }
      if (request.itemCandidates !== undefined) {
        body.itemCandidates = request.itemCandidates.map((item) => item?.id ?? null);
      }
      const response = await postJson(CALC_PATHS.reverse, body, isReverseResult, signal);
      if (!response.ok) {
        return response;
      }
      return { ok: true, value: mapReverseResult(response.value) };
    },
  };
}
