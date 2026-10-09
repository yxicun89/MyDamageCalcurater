// @smogon/calc の種族1件をスナップショットの形にする(fetch-calc.mjs から分けた純粋な関数。テスト用)。
// weightkg(ADR-0143 §4)は calc の数値のまま出す(hg への変換・Showdown との照合は Go 側)。
// 無い・正でない値は黙って 0 にせず例外で止める(重さで威力が決まる技の計算が黙って誤るため)。
export const calcSpeciesEntry = (s) => {
  if (typeof s.weightkg !== 'number' || !Number.isFinite(s.weightkg) || s.weightkg <= 0) {
    throw new Error(`fetch-calc: 種族 ${s.name} の weightkg が正の数でない: ${s.weightkg}`);
  }
  return {
    name: s.name,
    types: s.types,
    baseStats: {
      hp: s.baseStats.hp, atk: s.baseStats.atk, def: s.baseStats.def,
      spa: s.baseStats.spa, spd: s.baseStats.spd, spe: s.baseStats.spe,
    },
    weightkg: s.weightkg,
  };
};
