// F-06(Web): 素早さ表の仮想スクロールの計算(ADR-0608)。DOM を知らない純粋な関数だけを置く。
//
// 段(同速のグループ)の高さは「同速の行数」だけで決まる(1行 = ENTRY_HEIGHT。幅で折り返さない)ので、
// 全段の高さ・offset を先に計算でき、スクロール位置から表示範囲を二分探索で引ける。
// 自分の位置(同速の段、または段の間の境界の行)も同じ座標の1つの行(アンカー)として持つので、
// 右の位置マーカーは「アンカーの offset - スクロール量」で左の行と同じ高さに置ける。

/** 同速の1行(ポケモン1匹)の高さ(px)。段の中の行はこの高さに固定する。 */
export const ENTRY_HEIGHT = 24;
/** 段の上下の余白の合計(px)。 */
export const TIER_PADDING = 16;
/** 行と行の間の隙間(px)。行の「枠」(slot)は中身 + この隙間。 */
export const ROW_GAP = 8;
/** 境界の行の枠(slot)の高さ(px)。 */
export const BOUNDARY_SLOT_HEIGHT = 32;
/** 画面外の行をどれだけ先読みするか(行数)。 */
export const OVERSCAN_ROWS = 6;
/** ビューポートの高さが測れないとき(初回描画前・テスト環境)の仮の高さ(px)。 */
export const DEFAULT_VIEWPORT_HEIGHT = 600;

/** 段の最小限の形(speed.gen の SpeedTier が満たす)。 */
export interface TierLike {
  readonly speed: number;
  readonly entries: readonly unknown[];
}

/** 段1つの枠の高さ(中身 + 隙間)。0行でも1行分は確保する。 */
export function tierSlotHeight(entryCount: number): number {
  return Math.max(1, entryCount) * ENTRY_HEIGHT + TIER_PADDING + ROW_GAP;
}

export interface TierItem<T extends TierLike> {
  readonly kind: "tier";
  readonly tier: T;
  /** 境界を数えない、表の中の元の順(0 始まり。aria-posinset の元)。 */
  readonly tierIndex: number;
  readonly top: number;
  /** 枠の高さ(隙間を含む)。 */
  readonly height: number;
  /** 自分と同じ実数値の段か。 */
  readonly selfTie: boolean;
}

export interface BoundaryItem {
  readonly kind: "boundary";
  readonly top: number;
  readonly height: number;
  readonly afterSpeed?: number;
  readonly beforeSpeed?: number;
}

export type LayoutItem<T extends TierLike> = TierItem<T> | BoundaryItem;

export interface TableLayout<T extends TierLike> {
  readonly items: readonly LayoutItem<T>[];
  readonly totalHeight: number;
  /** 自分の位置を表す行(同速の段か境界)の添字。自分が無ければ null。 */
  readonly anchorIndex: number | null;
}

/**
 * 表の全行の offset を計算する。ownSpeed が null でなければ、同じ実数値の段をアンカーにし、
 * 無ければ境界の行を挿入してアンカーにする。トリックルーム中の表は昇順(遅い順。ADR-0607 §4)なので、
 * 自分より速い最初の段の前に境界を置く。
 */
export function buildTableLayout<T extends TierLike>(
  tiers: readonly T[],
  ownSpeed: number | null,
  trickRoom: boolean,
): TableLayout<T> {
  const selfTieIndex = ownSpeed === null ? -1 : tiers.findIndex((tier) => tier.speed === ownSpeed);
  let boundaryAt = -1;
  if (ownSpeed !== null && selfTieIndex === -1) {
    const first = tiers.findIndex((tier) => (trickRoom ? tier.speed > ownSpeed : tier.speed < ownSpeed));
    boundaryAt = first === -1 ? tiers.length : first;
  }

  const items: LayoutItem<T>[] = [];
  let top = 0;
  let anchorIndex: number | null = null;
  function pushBoundary(): void {
    anchorIndex = items.length;
    items.push({
      kind: "boundary",
      top,
      height: BOUNDARY_SLOT_HEIGHT,
      afterSpeed: tiers[boundaryAt - 1]?.speed,
      beforeSpeed: tiers[boundaryAt]?.speed,
    });
    top += BOUNDARY_SLOT_HEIGHT;
  }
  tiers.forEach((tier, tierIndex) => {
    if (tierIndex === boundaryAt) {
      pushBoundary();
    }
    const height = tierSlotHeight(tier.entries.length);
    const selfTie = tierIndex === selfTieIndex;
    if (selfTie) {
      anchorIndex = items.length;
    }
    items.push({ kind: "tier", tier, tierIndex, top, height, selfTie });
    top += height;
  });
  if (boundaryAt === tiers.length) {
    pushBoundary();
  }
  return { items, totalHeight: top, anchorIndex };
}

/** 先頭から順に offset が増える items のうち、条件を初めて満たす添字(無ければ items.length)。 */
function firstIndexWhere(
  items: readonly { readonly top: number; readonly height: number }[],
  pred: (item: { top: number; height: number }) => boolean,
): number {
  let low = 0;
  let high = items.length;
  while (low < high) {
    const mid = (low + high) >> 1;
    const item = items[mid];
    if (item !== undefined && pred(item)) {
      high = mid;
    } else {
      low = mid + 1;
    }
  }
  return low;
}

/**
 * スクロール位置とビューポートの高さから、描画する行の範囲 [start, end) を返す。
 * 見えている行の前後に overscan 行を足す。範囲は 0〜items.length に収める。
 */
export function visibleRange(
  items: readonly { readonly top: number; readonly height: number }[],
  scrollTop: number,
  viewportHeight: number,
  overscan: number,
): { readonly start: number; readonly end: number } {
  if (items.length === 0) {
    return { start: 0, end: 0 };
  }
  const first = firstIndexWhere(items, (item) => item.top + item.height > scrollTop);
  const last = firstIndexWhere(items, (item) => item.top >= scrollTop + viewportHeight);
  const start = Math.max(0, first - overscan);
  const end = Math.min(items.length, Math.max(last, first) + overscan);
  return { start: Math.min(start, end), end };
}

export interface MarkerPlacement {
  /** visible = 左の行と同じ高さ。above / below = 行が画面の上 / 下に隠れていて、端に張り付いている。 */
  readonly state: "visible" | "above" | "below";
  /** ビューポート上端からの位置(px)。 */
  readonly top: number;
  /** マーカーの高さ(px)。左の行の中身と同じ(隙間を除く)。 */
  readonly height: number;
}

/** 右の位置マーカーの置き場所。アンカーの offset からスクロール量を引くので、左のスクロールに追従する。 */
export function markerPlacement(
  anchor: { readonly top: number; readonly height: number } | undefined,
  scrollTop: number,
  viewportHeight: number,
): MarkerPlacement | null {
  if (anchor === undefined) {
    return null;
  }
  const height = anchor.height - ROW_GAP;
  const top = anchor.top - scrollTop;
  if (top < 0) {
    return { state: "above", top: 0, height };
  }
  if (top + height > viewportHeight) {
    return { state: "below", top: Math.max(0, viewportHeight - height), height };
  }
  return { state: "visible", top, height };
}

/** アンカーをビューポートの中央に寄せるスクロール量(0〜全体の高さ-ビューポートに丸める)。 */
export function scrollTopToReveal(
  anchor: { readonly top: number; readonly height: number },
  viewportHeight: number,
  totalHeight: number,
): number {
  const centered = Math.round(anchor.top - (viewportHeight - anchor.height) / 2);
  return Math.max(0, Math.min(centered, Math.max(0, totalHeight - viewportHeight)));
}
