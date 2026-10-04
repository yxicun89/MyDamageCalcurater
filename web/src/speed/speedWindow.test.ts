// F-06(Web): 素早さ表の仮想スクロール(表示範囲・位置マーカーの高さ揃え)の純粋な計算(ADR-0608)。
import { describe, expect, test } from "vitest";
import {
  ENTRY_HEIGHT,
  ROW_GAP,
  TIER_PADDING,
  BOUNDARY_SLOT_HEIGHT,
  buildTableLayout,
  markerPlacement,
  scrollTopToReveal,
  tierSlotHeight,
  visibleRange,
  type TierLike,
} from "./speedWindow";

function tier(speed: number, entryCount = 1): TierLike {
  return { speed, entries: Array.from({ length: entryCount }, (_, i) => ({ id: i })) };
}

/** 速い順(降順)の段を n 個(速さ 1000, 990, ...)。 */
function descending(n: number): TierLike[] {
  return Array.from({ length: n }, (_, i) => tier(1000 - i * 10));
}

describe("tierSlotHeight", () => {
  test("段の高さは行数だけで決まる(幅に依存しない)", () => {
    expect(tierSlotHeight(1)).toBe(ENTRY_HEIGHT + TIER_PADDING + ROW_GAP);
    expect(tierSlotHeight(3)).toBe(3 * ENTRY_HEIGHT + TIER_PADDING + ROW_GAP);
  });
  test("0行でも1行分の高さを持つ", () => {
    expect(tierSlotHeight(0)).toBe(tierSlotHeight(1));
  });
});

describe("buildTableLayout", () => {
  test("自分が無ければ段だけで、offset は高さの累積、全体の高さは合計", () => {
    const layout = buildTableLayout([tier(300, 1), tier(200, 2), tier(100, 1)], null, false);
    expect(layout.items.map((item) => item.kind)).toEqual(["tier", "tier", "tier"]);
    expect(layout.items.map((item) => item.top)).toEqual([
      0,
      tierSlotHeight(1),
      tierSlotHeight(1) + tierSlotHeight(2),
    ]);
    expect(layout.totalHeight).toBe(2 * tierSlotHeight(1) + tierSlotHeight(2));
    expect(layout.anchorIndex).toBeNull();
  });

  test("同速の段があれば、その段が位置の基準(境界は挿入しない)", () => {
    const layout = buildTableLayout([tier(300), tier(200, 2), tier(100)], 200, false);
    expect(layout.items).toHaveLength(3);
    expect(layout.anchorIndex).toBe(1);
    const anchor = layout.items[1];
    expect(anchor?.kind === "tier" && anchor.selfTie).toBe(true);
  });

  test("同速が無ければ、速い段と遅い段の間に境界の行を挿入し、それが基準になる", () => {
    const layout = buildTableLayout([tier(300), tier(200), tier(100)], 250, false);
    expect(layout.items.map((item) => item.kind)).toEqual(["tier", "boundary", "tier", "tier"]);
    expect(layout.anchorIndex).toBe(1);
    const boundary = layout.items[1];
    expect(boundary?.kind === "boundary" && boundary.afterSpeed).toBe(300);
    expect(boundary?.kind === "boundary" && boundary.beforeSpeed).toBe(200);
    expect(boundary?.top).toBe(tierSlotHeight(1));
    expect(layout.totalHeight).toBe(3 * tierSlotHeight(1) + BOUNDARY_SLOT_HEIGHT);
  });

  test("先頭より速いなら表の先頭、末尾より遅いなら表の末尾に境界を置く", () => {
    const fastest = buildTableLayout([tier(300), tier(200)], 999, false);
    expect(fastest.items[0]?.kind).toBe("boundary");
    expect(fastest.items[0]?.top).toBe(0);
    expect(fastest.anchorIndex).toBe(0);
    const slowest = buildTableLayout([tier(300), tier(200)], 1, false);
    expect(slowest.items.at(-1)?.kind).toBe("boundary");
    expect(slowest.anchorIndex).toBe(2);
  });

  test("トリックルーム(昇順の表)でも、自分より速い最初の段の前に境界を置く", () => {
    const layout = buildTableLayout([tier(100), tier(200), tier(300)], 250, true);
    expect(layout.items.map((item) => item.kind)).toEqual(["tier", "tier", "boundary", "tier"]);
    expect(layout.anchorIndex).toBe(2);
    const boundary = layout.items[2];
    expect(boundary?.kind === "boundary" && boundary.afterSpeed).toBe(200);
    expect(boundary?.kind === "boundary" && boundary.beforeSpeed).toBe(300);
  });

  test("トリックルームで自分が最遅なら表の先頭、最速なら表の末尾", () => {
    const slowest = buildTableLayout([tier(100), tier(200)], 1, true);
    expect(slowest.anchorIndex).toBe(0);
    expect(slowest.items[0]?.kind).toBe("boundary");
    const fastest = buildTableLayout([tier(100), tier(200)], 999, true);
    expect(fastest.anchorIndex).toBe(2);
  });

  test("段が空でも壊れない", () => {
    const layout = buildTableLayout([], 100, false);
    expect(layout.items.map((item) => item.kind)).toEqual(["boundary"]);
    expect(layout.anchorIndex).toBe(0);
    expect(buildTableLayout([], null, false).totalHeight).toBe(0);
  });

  test("段の tierIndex は境界を数えない元の順(aria-posinset の元)", () => {
    const layout = buildTableLayout([tier(300), tier(200), tier(100)], 250, false);
    const indexes = layout.items.flatMap((item) => (item.kind === "tier" ? [item.tierIndex] : []));
    expect(indexes).toEqual([0, 1, 2]);
  });
});

describe("visibleRange", () => {
  const layout = buildTableLayout(descending(1000), null, false);
  const slot = tierSlotHeight(1);

  test("先頭: スクロール0では先頭から、ビューポート+余白の分だけ", () => {
    const range = visibleRange(layout.items, 0, slot * 10, 2);
    expect(range.start).toBe(0);
    expect(range.end).toBe(12);
  });

  test("途中: 見えている行の前後に余白(overscan)を足す", () => {
    const range = visibleRange(layout.items, slot * 100, slot * 10, 3);
    expect(range.start).toBe(97);
    expect(range.end).toBe(113);
  });

  test("行の途中までスクロールしたら、その行も含める", () => {
    const range = visibleRange(layout.items, slot * 100 + 5, slot * 10, 0);
    expect(range.start).toBe(100);
    expect(range.end).toBe(111);
  });

  test("末尾: 範囲は件数を超えない", () => {
    const range = visibleRange(layout.items, layout.totalHeight - slot * 10, slot * 10, 5);
    expect(range.end).toBe(1000);
    expect(range.start).toBe(985);
  });

  test("スクロールが全体を超えても(絞り込みで縮んだ直後)負の範囲にならない", () => {
    const range = visibleRange(layout.items, layout.totalHeight * 2, slot * 10, 2);
    expect(range.end).toBe(1000);
    expect(range.start).toBeLessThanOrEqual(range.end);
  });

  test("空の表は空の範囲", () => {
    expect(visibleRange([], 0, 600, 4)).toEqual({ start: 0, end: 0 });
  });

  test("可変の高さ(同速で行が増える段)でも offset で正しく引ける", () => {
    const mixed = buildTableLayout([tier(5, 1), tier(4, 10), tier(3, 1), tier(2, 1)], null, false);
    const range = visibleRange(mixed.items, tierSlotHeight(1) + 1, 10, 0);
    expect(range).toEqual({ start: 1, end: 2 });
  });

  test("描画する行数は全体の件数より十分少ない(DOM に全行を出さない)", () => {
    const range = visibleRange(layout.items, slot * 500, 600, 4);
    expect(range.end - range.start).toBeLessThan(40);
  });
});

describe("markerPlacement", () => {
  const layout = buildTableLayout(descending(100), 555, false);
  const anchor = layout.items[layout.anchorIndex ?? 0];

  test("自分が無ければマーカーは無い", () => {
    expect(markerPlacement(undefined, 0, 600)).toBeNull();
  });

  test("基準の行が見えていれば、左の行と同じ高さ(スクロールを引いた位置)に置く", () => {
    if (anchor === undefined) throw new Error("anchor");
    const scrollTop = anchor.top - 100;
    const placement = markerPlacement(anchor, scrollTop, 600);
    expect(placement).toEqual({ state: "visible", top: 100, height: anchor.height - ROW_GAP });
  });

  test("スクロールに追従する(スクロール量が増えれば位置が同じだけ上がる)", () => {
    if (anchor === undefined) throw new Error("anchor");
    const a = markerPlacement(anchor, anchor.top - 200, 600);
    const b = markerPlacement(anchor, anchor.top - 150, 600);
    expect((a?.top ?? 0) - (b?.top ?? 0)).toBe(50);
  });

  test("上に隠れたら上端に張り付き above、下に隠れたら下端に張り付き below", () => {
    if (anchor === undefined) throw new Error("anchor");
    const above = markerPlacement(anchor, anchor.top + 500, 600);
    expect(above?.state).toBe("above");
    expect(above?.top).toBe(0);
    const below = markerPlacement(anchor, anchor.top - 5000, 600);
    expect(below?.state).toBe("below");
    expect(below?.top).toBe(600 - (anchor.height - ROW_GAP));
  });
});

describe("scrollTopToReveal", () => {
  test("基準の行をビューポートの中央に寄せ、範囲(0〜全体-ビューポート)に丸める", () => {
    const layout = buildTableLayout(descending(100), 555, false);
    const anchor = layout.items[layout.anchorIndex ?? 0];
    if (anchor === undefined) throw new Error("anchor");
    const target = scrollTopToReveal(anchor, 600, layout.totalHeight);
    expect(target).toBe(Math.round(anchor.top - (600 - anchor.height) / 2));
    expect(scrollTopToReveal({ ...anchor, top: 0 }, 600, layout.totalHeight)).toBe(0);
    expect(scrollTopToReveal({ ...anchor, top: layout.totalHeight }, 600, layout.totalHeight)).toBe(
      layout.totalHeight - 600,
    );
  });

  test("全体がビューポートより低ければ 0", () => {
    const layout = buildTableLayout(descending(2), 5, false);
    const anchor = layout.items[layout.anchorIndex ?? 0];
    if (anchor === undefined) throw new Error("anchor");
    expect(scrollTopToReveal(anchor, 600, layout.totalHeight)).toBe(0);
  });
});
