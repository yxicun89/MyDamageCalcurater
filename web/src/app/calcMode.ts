// P4-5: 計算モード(オフライン = WASM / オンライン = API)の保存(ADR-0301 §4)。
// 既定はオフライン(サーバーのマスタと Web のマスタの ID がまだ一致しないため)。選択は localStorage に
// 覚え、端末ごとの好みにする。ストレージが使えない・壊れた値でも失敗させず既定に戻す。

import { defaultStorage, type StorageLike } from "./storage";

/** 計算モード。オフラインは WASM、オンラインは API(ADR-0301 §4)。 */
export type CalcMode = "offline" | "online";

/** 計算モードの保存先キー。 */
export const CALC_MODE_STORAGE_KEY = "pokecalc.calcMode";

/** 既定の計算モード(ADR-0301 §4: pokedex-svc・gateway が揃うまではオフラインが既定)。 */
export const DEFAULT_CALC_MODE: CalcMode = "offline";

function isCalcMode(value: string): value is CalcMode {
  return value === "offline" || value === "online";
}

/** 保存済みの計算モードを読む。ストレージが無い・例外・未知の値は既定(オフライン)にする。 */
export function loadCalcMode(storage: StorageLike | null = defaultStorage()): CalcMode {
  if (storage === null) {
    return DEFAULT_CALC_MODE;
  }
  try {
    const stored = storage.getItem(CALC_MODE_STORAGE_KEY);
    return stored !== null && isCalcMode(stored) ? stored : DEFAULT_CALC_MODE;
  } catch {
    return DEFAULT_CALC_MODE;
  }
}

/** 計算モードを保存する。ストレージが無い・例外は無視する(失敗させない。ADR-0301 §4)。 */
export function saveCalcMode(mode: CalcMode, storage: StorageLike | null = defaultStorage()): void {
  if (storage === null) {
    return;
  }
  try {
    storage.setItem(CALC_MODE_STORAGE_KEY, mode);
  } catch {
    // 保存できなくても失敗させない(プライベートブラウズ・容量超過など)。
  }
}
