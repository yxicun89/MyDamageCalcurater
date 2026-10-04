// P5-3c: 計算画面の攻撃側から、お気に入りの作成本文(FavoriteInput)を組み立てる(ADR-0327 §2)。純粋関数。

import type { components } from "../api/openapi.gen";
import { BATTLE_LEVEL } from "../domain/requests";
import { MAX_FAVORITE_SAVED_BYTES } from "./favoriteCalc";

type Schemas = components["schemas"];

/** label の上限(api/openapi.yaml の FavoriteInput.label の maxLength。Unicode コードポイント数)。 */
export const MAX_FAVORITE_LABEL_LENGTH = 30;

export interface FavoriteInputSource {
  readonly label: string;
  readonly speciesKey: string;
  readonly natureId: string;
  readonly sp: Schemas["StatBlock"];
  readonly itemId: string | null;
  /** ADR-0333: 計算の入力(favoriteCalcOf の結果)。無い・保存内容が上限を超えるときは従来の本文にする。 */
  readonly calc?: Schemas["CalcRequest"] | null;
}

/** label を上限のコードポイント数で切る(サロゲートペアを割らない)。 */
function truncateLabel(label: string): string {
  return Array.from(label).slice(0, MAX_FAVORITE_LABEL_LENGTH).join("");
}

function utf8Bytes(value: unknown): number {
  return new TextEncoder().encode(JSON.stringify(value)).length;
}

export function favoriteInputOf(source: FavoriteInputSource): Schemas["FavoriteInput"] {
  const label = truncateLabel(source.label);
  const { calc } = source;
  if (calc !== undefined && calc !== null) {
    const withCalc = { label, individual: calc.attacker, calc };
    // 万一上限を超える(異常に長い ID)ときは calc を付けず、追加を 400 で失敗させない。
    if (utf8Bytes(withCalc) <= MAX_FAVORITE_SAVED_BYTES) {
      return withCalc;
    }
  }
  return {
    label,
    individual: {
      speciesKey: source.speciesKey,
      level: BATTLE_LEVEL,
      natureId: source.natureId,
      sp: source.sp,
      ...(source.itemId === null ? {} : { itemId: source.itemId }),
    },
  };
}
