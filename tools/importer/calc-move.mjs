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
});
