// G-04(Web): 「自分の周り」パネル(ADR-0609)。スクロールなし・折りたたみなしで、自分の上下を見せる。
// 並びの切り出しは speedNeighborhood.ts(純粋)。ここは描画だけ。
// ※ いまはスタブ(テスト先行)。実装者が描画を書き、SpeedScreen の自分のカードの近くに置く。
//
// 契約(テストが見るもの):
//   - role=region・名前 speedScreenText.neighborhoodRegionLabel。表のスクロール領域(speed-viewport)の外に置く
//   - 先に動く側/後に動く側は ul(aria-label = 通常は速い側/遅い側、トリックルームは先に動く側/後に動く側)
//   - 近傍の段: data-testid="neighbor-before" / "neighbor-after"(data-speed を持つ)、
//     自分: data-testid="neighbor-self"(「自分」バッジの文字を含む)
//   - 端の合計行: data-testid="neighbor-total-before" / "neighbor-total-after"

import type { NeighborTierInput } from "./speedNeighborhood";

export interface SpeedNeighborhoodProps {
  /** 表の並びのままの tiers(通常は降順、トリックルームは昇順)。 */
  readonly tiers: readonly NeighborTierInput[];
  /** 自分の実数値。未決定なら null。 */
  readonly ownSpeed: number | null;
  readonly trickRoom: boolean;
}

/* eslint-disable @typescript-eslint/no-unused-vars -- スタブ。実装時にこの行も消す */
export function SpeedNeighborhood(_props: SpeedNeighborhoodProps) {
  return null;
}
