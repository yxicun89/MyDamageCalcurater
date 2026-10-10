// G-04(Web): 「自分の周り」パネルの純粋な切り出し(ADR-0609)。DOM を知らない。
// 入力は表の並び(speed-svc の tiers。通常 = 速い順、トリックルーム = 遅い順 = 先に動く順。ADR-0607 §4)と
// 自分の実数値。素早さは計算し直さず、tiers の実数値と自分の実数値の直接比較だけで位置を決める(ADR-0608 §3)。
// ※ いまはスタブ(テスト先行)。実装者が中身を書く。

/** 自分の前後に出す段の既定数。 */
export const DEFAULT_NEIGHBOR_STEPS = 3;

/** 近傍の段の代表名の数の既定(1段につき)。残りは「ほか n 体」。 */
export const DEFAULT_NAMES_PER_TIER = 1;

/** 同速の段で名前を並べる数の既定。残りは「ほか n 体」。 */
export const DEFAULT_TIE_NAMES = 4;

/** tiers の最小限の形(speed.gen の SpeedTier が満たす)。 */
export interface NeighborTierInput {
  readonly speed: number;
  readonly entries: readonly { readonly nameJa: string }[];
}

export interface NeighborhoodOptions {
  readonly steps?: number;
  readonly namesPerTier?: number;
  readonly tieNames?: number;
}

/** 近傍に出す1段。 */
export interface NeighborTier {
  readonly speed: number;
  /** 代表のポケモン名(表の並びの先頭から)。 */
  readonly names: readonly string[];
  /** names に入らなかった体数(「ほか n 体」)。0 なら出さない。 */
  readonly moreCount: number;
  /** この段の総体数。 */
  readonly entryCount: number;
}

export interface Neighborhood {
  /** 自分の実数値。 */
  readonly ownSpeed: number;
  /** 自分と同じ実数値の段。無ければ null(自分だけが入る境界)。 */
  readonly tie: NeighborTier | null;
  /** 先に動く側の直近の段。表示順(上 = 遠い → 下 = 自分に近い)。最大 steps 段。 */
  readonly before: readonly NeighborTier[];
  /** 後に動く側の直近の段。表示順(上 = 自分に近い → 下 = 遠い)。最大 steps 段。 */
  readonly after: readonly NeighborTier[];
  /** 先に動く側の全体の体数(表示した段も含む合計)。 */
  readonly beforeTotal: number;
  /** 後に動く側の全体の体数(同上)。 */
  readonly afterTotal: number;
  /** 表示した段の外にまだ段が残っているか。 */
  readonly beforeHasMore: boolean;
  readonly afterHasMore: boolean;
}

/**
 * 自分の周りを切り出す。tiers は表の並び(通常は降順、トリックルームは昇順)のまま渡す。
 * 自分より「先に動く側」は表の並びで自分より前、「後に動く側」は後ろ。
 * ownSpeed が null(まだ自分が決まっていない)なら null。
 */
/* eslint-disable @typescript-eslint/no-unused-vars -- スタブ。実装時にこの行も消す */
export function buildNeighborhood(
  _tiers: readonly NeighborTierInput[],
  _ownSpeed: number | null,
  _trickRoom: boolean,
  _options?: NeighborhoodOptions,
): Neighborhood | null {
  throw new Error("buildNeighborhood: 未実装(G-04)");
}
