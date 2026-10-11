// タイプバランスの部品(useTeamBalance・BalancePanel)が共有する型(docs/adr/0421-balance-in-team-builder.md §3)。
import type { components } from "../api/balance.gen";

export type Schemas = components["schemas"];

/**
 * バランスへ渡す入力の1体分(構築の1枠 = 1要素)。呼び出し側はバランスの内部(キー・debounce・古い応答)を知らない。
 * speciesKey が "" の枠は無視される。abilityId が null なら送らない。moveIds の空欄と重複は取り除かれる。
 */
export interface BalanceInputMember {
  readonly speciesKey: string;
  readonly abilityId: string | null;
  readonly moveIds: readonly string[];
}

/** balance 呼び出し1本の状態(判別 union)。idle は送るメンバーが無い(呼んでいない)。 */
export type RequestState<T> =
  | { readonly status: "idle" }
  | { readonly status: "loading" }
  | { readonly status: "success"; readonly value: T }
  | { readonly status: "error"; readonly error: { readonly code: string; readonly message: string } };

/** useTeamBalance の戻り値 = BalancePanel が表示する状態。 */
export interface TeamBalanceState {
  readonly analyze: RequestState<Schemas["AnalyzeResponse"]>;
  readonly coverage: RequestState<Schemas["CoverageResponse"]>;
  readonly threats: RequestState<Schemas["ThreatsResponse"]>;
  readonly recommendations: RequestState<Schemas["RecommendationsResponse"]>;
  /** 「仮想敵n」の入力欄の番号(1始まり)を、送った順(=threats 応答の順)のまま持ち回る。 */
  readonly threatSlotNumbers: readonly number[];
}
