// ADR-0358 段 2: 計算画面の直近の入力を 1 件、localStorage に覚える(端末の中だけ。オフラインでも使える)。
// 中身はお気に入りの作成本文(FavoriteInput。ADR-0333)と同じ形にして、復元もお気に入りの復元(restoreFavoriteInputs)を
// 使い回す。保存の範囲は計算画面の主要入力(攻撃側の種族・性格・SP・持ち物・特性、防御側、技、条件、対戦の状態)。
// 逆算・調整の入力と、お気に入りの一覧は含めない(ADR-0358 追記)。
// calcMode.ts と同じ流儀: ストレージが無い・例外・壊れた値・版違いは失敗させず、保存が無いものとして扱う。

import type { components } from "../api/openapi.gen";
import { defaultStorage, type StorageLike } from "./storage";

type FavoriteInput = components["schemas"]["FavoriteInput"];
type Favorite = components["schemas"]["Favorite"];

/** 直近の計算入力の保存先キー。 */
export const LAST_CALC_INPUT_STORAGE_KEY = "pokecalc.lastCalcInput";

/** 保存する形の版。形を変えたら上げる(版違いは無いものとして扱う)。 */
export const LAST_CALC_INPUT_SCHEMA_VERSION = 1;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function hasSpeciesKey(value: unknown): boolean {
  return isRecord(value) && typeof value.speciesKey === "string" && value.speciesKey !== "";
}

/** 保存値が FavoriteInput として復元に渡せる形か(種族 key まで。ID が今のマスタに在るかは復元側が安全に捨てる)。 */
function isLastCalcInput(value: unknown): value is FavoriteInput {
  if (!isRecord(value) || !hasSpeciesKey(value.individual)) {
    return false;
  }
  if (value.label !== undefined && value.label !== null && typeof value.label !== "string") {
    return false;
  }
  if (value.calc === undefined) {
    return true;
  }
  return isRecord(value.calc) && hasSpeciesKey(value.calc.attacker) && hasSpeciesKey(value.calc.defender);
}

/** 保存済みの直近の入力を読む。無い・読めない・壊れた・版違いは null。 */
export function loadLastCalcInput(storage: StorageLike | null = defaultStorage()): FavoriteInput | null {
  if (storage === null) {
    return null;
  }
  try {
    const raw = storage.getItem(LAST_CALC_INPUT_STORAGE_KEY);
    if (raw === null) {
      return null;
    }
    const parsed: unknown = JSON.parse(raw);
    if (!isRecord(parsed) || parsed.schemaVersion !== LAST_CALC_INPUT_SCHEMA_VERSION) {
      return null;
    }
    return isLastCalcInput(parsed.input) ? parsed.input : null;
  } catch {
    return null;
  }
}

/** 直近の入力を保存する(1 件を上書き)。ストレージが無い・例外は無視する(失敗させない)。 */
export function saveLastCalcInput(
  input: FavoriteInput,
  storage: StorageLike | null = defaultStorage(),
): void {
  if (storage === null) {
    return;
  }
  try {
    storage.setItem(
      LAST_CALC_INPUT_STORAGE_KEY,
      JSON.stringify({ schemaVersion: LAST_CALC_INPUT_SCHEMA_VERSION, input }),
    );
  } catch {
    // 保存できなくても失敗させない(プライベートブラウズ・容量超過など)。
  }
}

/** 保存した入力を、お気に入りの復元(FavoriteRestoreRequest)に渡せる Favorite の形にする(id・日時は使われない)。 */
export function favoriteOfLastCalcInput(input: FavoriteInput): Favorite {
  return {
    id: "last-calc-input",
    label: input.label ?? null,
    individual: input.individual,
    ...(input.calc === undefined ? {} : { calc: input.calc }),
    createdAt: "1970-01-01T00:00:00Z",
    updatedAt: "1970-01-01T00:00:00Z",
  };
}
