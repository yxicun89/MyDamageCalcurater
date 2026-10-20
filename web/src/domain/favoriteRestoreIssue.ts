// お気に入りを画面へ戻すときに、戻せなかった・反映しなかった項目(favorites/favoriteCalc.ts が作り、
// i18n/favorites.ts が文にする)。shared(i18n)から機能(favorites)を import しないよう、型を domain に置く(ADR-0811 W4)。

/** 攻撃側・防御側。 */
export type FavoriteRestoreSide = "attacker" | "defender";

/** 戻せなかった・反映しなかった項目。 */
export type FavoriteRestoreIssue =
  | { readonly kind: "species"; readonly side: FavoriteRestoreSide; readonly id: string }
  | { readonly kind: "move"; readonly id: string }
  | { readonly kind: "item"; readonly side: FavoriteRestoreSide; readonly id: string }
  | { readonly kind: "ability"; readonly side: FavoriteRestoreSide; readonly id: string }
  | { readonly kind: "nature"; readonly id: string }
  | {
      readonly kind: "megaItem";
      readonly side: FavoriteRestoreSide;
      readonly savedItemId: string;
      readonly lockedItemId: string;
    }
  | { readonly kind: "ignored"; readonly field: string };
