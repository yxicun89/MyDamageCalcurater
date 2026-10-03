// P5-3c: 計算画面の攻撃側から、お気に入りの作成本文(FavoriteInput)を組み立てる(ADR-0327 §2)。純粋関数。

import type { components } from "../api/openapi.gen";
import { BATTLE_LEVEL } from "../domain/requests";

type Schemas = components["schemas"];

/** label の上限(api/openapi.yaml の FavoriteInput.label の maxLength。Unicode コードポイント数)。 */
export const MAX_FAVORITE_LABEL_LENGTH = 30;

export interface FavoriteInputSource {
  readonly label: string;
  readonly speciesKey: string;
  readonly natureId: string;
  readonly sp: Schemas["StatBlock"];
  readonly itemId: string | null;
}

/** label を上限のコードポイント数で切る(サロゲートペアを割らない)。 */
function truncateLabel(label: string): string {
  return Array.from(label).slice(0, MAX_FAVORITE_LABEL_LENGTH).join("");
}

export function favoriteInputOf(source: FavoriteInputSource): Schemas["FavoriteInput"] {
  return {
    label: truncateLabel(source.label),
    individual: {
      speciesKey: source.speciesKey,
      level: BATTLE_LEVEL,
      natureId: source.natureId,
      sp: source.sp,
      ...(source.itemId === null ? {} : { itemId: source.itemId }),
    },
  };
}
