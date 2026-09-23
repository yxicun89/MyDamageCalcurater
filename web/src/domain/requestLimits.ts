// P4-19(issue 110、ADR-0208): calc の候補・観測の件数上限。api/openapi.yaml の BulkCalcRequest.itemVariants /
// ReverseRequest.itemCandidates / ReverseRequest.observations の maxItems と同じ値を Web でも持つ。
//
// なぜ Web にも持つのか: 上限を超えた配列を送ると API は 400 invalid_input、engine(WASM)は同じ上限の
// sentinel で失敗するので、画面が黙って壊れる。上限に当たることが分かるのは入力を組み立てる画面側だけなので、
// ここで決定的に絞り込み、絞り込んだことを利用者に見せる(黙って切り捨てない。DECISIONS.md 2026-09-23)。
//
// 値の正は api/openapi.yaml(実行時に YAML は読めないので、この定数はその写し)。ずれたら
// requestLimits.test.ts が openapi.yaml を読んで失敗する(コーディング規約 §2「同期を検査するテストを置く」)。

/** 一括計算に渡せる持ち物の通り数(`BulkCalcRequest.itemVariants` の maxItems)。null(持ち物なし)も1通りと数える。 */
export const MAX_ITEM_VARIANTS = 64;

/** 逆算で探索できる持ち物の通り数(`ReverseRequest.itemCandidates` の maxItems)。null(持ち物なし)も1通りと数える。 */
export const MAX_ITEM_CANDIDATES = 64;

/** 逆算に渡せる観測の件数(`ReverseRequest.observations` の maxItems)。 */
export const MAX_OBSERVATIONS = 16;

/** 上限で絞り込んだ結果。truncated は「落とした要素があるか」(ちょうど上限のときは false)。 */
export interface LimitedValues<T> {
  readonly values: readonly T[];
  readonly truncated: boolean;
}

/**
 * 上限までを先頭から取る(マスタの順序をそのまま使う既存の規約。ADR-0300 §6・§7)。
 * 並べ替え・間引きはせず、末尾から落とすだけなので、同じ入力からは常に同じ結果になる。
 */
export function limitToMax<T>(values: readonly T[], max: number): LimitedValues<T> {
  return values.length <= max
    ? { values, truncated: false }
    : { values: values.slice(0, max), truncated: true };
}
