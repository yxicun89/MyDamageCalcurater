// @smogon/calc の技1件をスナップショットの形にする(fetch-calc.mjs から分けた純粋な関数。テスト用)。
// calc は未設定の項目を省略するので、Go 側が「無い」と「省略」を区別できるよう空値で埋める。
// target(ADR-0136)は calc が全体技(allAdjacent・allAdjacentFoes)にだけ持つ。省略は '' で出す。
export const calcMoveEntry = (m) => ({
  name: m.name,
  type: m.type ?? '',
  category: m.category ?? '',
  basePower: m.basePower ?? 0,
  priority: m.priority ?? 0,
  target: m.target ?? '',
  // 技のフラグの判定材料(ADR-0178)。Showdown と同じ形(flags は真のキーの昇順)。secondaries は calc の真偽値。
  flags: Object.keys(m.flags ?? {}).filter((k) => m.flags[k]).sort(),
  recoil: m.recoil ?? null,
  hasCrashDamage: m.hasCrashDamage === true,
  secondaries: m.secondaries === true,
});
