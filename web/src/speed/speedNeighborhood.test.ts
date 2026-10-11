// G-04(Web): 自分の周りの切り出し(純粋関数。ADR-0609)。
//   N1 通常の場: 自分の上下の直近 N 段を、表示順(上から下)で返す
//   N2 同速の段があれば tie に入り、前後はその段を除く(同速の面々を全員出す)
//   N3 端(先頭・末尾)では片側が空で、合計体数と「まだ先がある」が正しい
//   N4 合計体数は表示した段の外も含めた片側の全体
//   N5 トリックルーム(昇順の表)でも「表の並びで前 = 先に動く」で成立する
//   N6 空の表・自分未決定(null)
//   N7 名前は代表だけ(多ければ moreCount)。steps などの指定

import { describe, expect, test } from "vitest";
import { DEFAULT_NEIGHBOR_STEPS, buildNeighborhood, type NeighborTierInput } from "./speedNeighborhood";

function tier(speed: number, ...names: string[]): NeighborTierInput {
  return { speed, entries: names.map((nameJa) => ({ nameJa })) };
}

/** 降順(速い順)の表。 */
const DESC: readonly NeighborTierInput[] = [
  tier(300, "A1", "A2"),
  tier(280, "B1"),
  tier(260, "C1", "C2", "C3"),
  tier(240, "D1"),
  tier(220, "E1"),
  tier(200, "F1"),
  tier(180, "G1"),
  tier(160, "H1", "H2"),
  tier(140, "I1"),
];

function speeds(tiers: readonly { speed: number }[]): number[] {
  return tiers.map((t) => t.speed);
}

describe("N1 通常の場(同速なし)", () => {
  test("既定は前後 3 段。上 = 遠い → 下 = 自分に近い、後ろ側は上 = 自分に近い → 下 = 遠い", () => {
    expect(DEFAULT_NEIGHBOR_STEPS).toBe(3);
    const n = buildNeighborhood(DESC, 230, false);
    expect(n).not.toBeNull();
    expect(n?.ownSpeed).toBe(230);
    expect(n?.tie).toBeNull();
    // 230 より速い段: 300,280,260,240 → 直近 3 段 = 280,260,240(遠い順)
    expect(speeds(n?.before ?? [])).toEqual([280, 260, 240]);
    // 230 より遅い段: 220,200,180,160,140 → 直近 3 段 = 220,200,180
    expect(speeds(n?.after ?? [])).toEqual([220, 200, 180]);
  });

  test("steps を指定できる", () => {
    const n = buildNeighborhood(DESC, 230, false, { steps: 1 });
    expect(speeds(n?.before ?? [])).toEqual([240]);
    expect(speeds(n?.after ?? [])).toEqual([220]);
  });

  test("各段は実数値と体数を持つ", () => {
    const n = buildNeighborhood(DESC, 230, false);
    const c = n?.before.find((t) => t.speed === 260);
    expect(c?.entryCount).toBe(3);
  });
});

describe("N2 同速の段", () => {
  test("自分と同じ実数値の段は tie に入り、同速の面々を全員(上限まで)出す", () => {
    const n = buildNeighborhood(DESC, 260, false);
    expect(n?.tie?.speed).toBe(260);
    expect(n?.tie?.names).toEqual(["C1", "C2", "C3"]);
    expect(n?.tie?.moreCount).toBe(0);
    expect(speeds(n?.before ?? [])).toEqual([300, 280]);
    expect(speeds(n?.after ?? [])).toEqual([240, 220, 200]);
  });

  test("同速が多いときは tieNames まで並べ、残りは moreCount", () => {
    const crowd = [tier(200, "P1", "P2", "P3", "P4", "P5", "P6")];
    const n = buildNeighborhood(crowd, 200, false, { tieNames: 4 });
    expect(n?.tie?.names).toEqual(["P1", "P2", "P3", "P4"]);
    expect(n?.tie?.moreCount).toBe(2);
    expect(n?.tie?.entryCount).toBe(6);
  });
});

describe("N3 端", () => {
  test("自分が最速より速い: 先に動く側は空", () => {
    const n = buildNeighborhood(DESC, 999, false);
    expect(n?.before).toEqual([]);
    expect(n?.beforeTotal).toBe(0);
    expect(n?.beforeHasMore).toBe(false);
    expect(speeds(n?.after ?? [])).toEqual([300, 280, 260]);
    expect(n?.afterHasMore).toBe(true);
  });

  test("自分が最遅より遅い: 後に動く側は空", () => {
    const n = buildNeighborhood(DESC, 1, false);
    expect(n?.after).toEqual([]);
    expect(n?.afterTotal).toBe(0);
    expect(n?.afterHasMore).toBe(false);
    expect(speeds(n?.before ?? [])).toEqual([180, 160, 140]);
  });

  test("先頭の段と同速: 先に動く側は空", () => {
    const n = buildNeighborhood(DESC, 300, false);
    expect(n?.tie?.speed).toBe(300);
    expect(n?.before).toEqual([]);
    expect(n?.beforeHasMore).toBe(false);
  });

  test("表示した段で全部まかなえるときは HasMore が false", () => {
    const n = buildNeighborhood(DESC, 150, false); // 速い側 7 段、遅い側 1 段(140)
    expect(n?.afterHasMore).toBe(false);
    expect(n?.beforeHasMore).toBe(true);
  });
});

describe("N4 合計体数", () => {
  test("片側の全体の体数(表示した段の外も含む)", () => {
    const n = buildNeighborhood(DESC, 230, false);
    // 速い側: 300(2) + 280(1) + 260(3) + 240(1) = 7
    expect(n?.beforeTotal).toBe(7);
    // 遅い側: 220(1) + 200(1) + 180(1) + 160(2) + 140(1) = 6
    expect(n?.afterTotal).toBe(6);
    expect(n?.beforeHasMore).toBe(true);
    expect(n?.afterHasMore).toBe(true);
  });

  test("同速の段は前後どちらの合計にも入らない", () => {
    const n = buildNeighborhood(DESC, 260, false);
    expect(n?.beforeTotal).toBe(3);
    expect(n?.afterTotal).toBe(1 + 1 + 1 + 1 + 2 + 1);
  });
});

describe("N5 トリックルーム(昇順の表)", () => {
  const ASC = [...DESC].reverse();

  test("表の並びで前 = 先に動く側(遅い実数値)。速さを計算し直さず並びだけで決める", () => {
    const n = buildNeighborhood(ASC, 230, true);
    // 表は 140,160,...,300。230 より表の前 = 220,200,180,160,140 → 直近 3 段 = 180,200,220(遠い順 → 近い順)
    expect(speeds(n?.before ?? [])).toEqual([180, 200, 220]);
    // 表の後 = 240,260,... → 直近 3 段 = 240,260,280
    expect(speeds(n?.after ?? [])).toEqual([240, 260, 280]);
    expect(n?.beforeTotal).toBe(6);
    expect(n?.afterTotal).toBe(7);
  });

  test("同速の段があれば tie。前後はその段を除く", () => {
    const n = buildNeighborhood(ASC, 260, true);
    expect(n?.tie?.speed).toBe(260);
    expect(speeds(n?.before ?? [])).toEqual([200, 220, 240]);
    expect(speeds(n?.after ?? [])).toEqual([280, 300]);
  });

  test("全員より遅い自分は、トリックルームでは最初に動くので先に動く側が空", () => {
    const n = buildNeighborhood(ASC, 1, true);
    expect(n?.before).toEqual([]);
    expect(n?.beforeTotal).toBe(0);
    expect(speeds(n?.after ?? [])).toEqual([140, 160, 180]);
  });
});

describe("N6 空・自分未決定", () => {
  test("ownSpeed が null なら null", () => {
    expect(buildNeighborhood(DESC, null, false)).toBeNull();
  });

  test("tiers が空でも自分がいれば、前後が空の周りを返す", () => {
    const n = buildNeighborhood([], 200, false);
    expect(n).not.toBeNull();
    expect(n?.tie).toBeNull();
    expect(n?.before).toEqual([]);
    expect(n?.after).toEqual([]);
    expect(n?.beforeTotal).toBe(0);
    expect(n?.afterTotal).toBe(0);
  });
});

describe("N7 代表名", () => {
  test("近傍の段は代表 1 体の名前と、残りの体数(moreCount)", () => {
    const n = buildNeighborhood(DESC, 230, false);
    const c = n?.before.find((t) => t.speed === 260);
    expect(c?.names).toEqual(["C1"]);
    expect(c?.moreCount).toBe(2);
    const b = n?.before.find((t) => t.speed === 280);
    expect(b?.names).toEqual(["B1"]);
    expect(b?.moreCount).toBe(0);
  });

  test("namesPerTier を増やせる", () => {
    const n = buildNeighborhood(DESC, 230, false, { namesPerTier: 2 });
    const c = n?.before.find((t) => t.speed === 260);
    expect(c?.names).toEqual(["C1", "C2"]);
    expect(c?.moreCount).toBe(1);
  });

  test("入力を書き換えない", () => {
    const copy = JSON.stringify(DESC);
    buildNeighborhood(DESC, 230, false);
    expect(JSON.stringify(DESC)).toBe(copy);
  });
});
