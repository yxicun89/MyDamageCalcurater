// P4-2: engine に渡すリクエストの組み立て(純粋関数)。形は ADR-0011 §3 の WASM 境界の DTO
// (engine/wasmapi/dto.go・requests.go)。境界は未知のフィールドを拒否する(unknown_field)ので、
// 画面のための追加フィールド(learnset)を engine に渡さない(toEngineSpecies)。
// 一括計算は presetKeys / presets を省き、engine の既定(技の分類で HB 系 / HD 系の5行)を使う(ADR-0009、ADR-0016 §6)。

import type {
  Ability,
  BulkRequest,
  CalcRequest,
  Individual,
  Item,
  Move,
  Nature,
  Species,
  Stats,
  StatKey,
  TypeChart,
} from "../engine/types";
import type { MasterSpecies } from "../master/types";

/** バトルのレベル。Lv50 固定(CLAUDE.md ドメイン規約)。 */
export const BATTLE_LEVEL = 50;

/** SP の1ステータスあたりの上限(CLAUDE.md ドメイン規約: 1ステータス最大32)。 */
export const MAX_SP_PER_STAT = 32;

/** SP の合計の上限(CLAUDE.md ドメイン規約: 合計66)。 */
export const MAX_SP_TOTAL = 66;

/** 無振りの SP(6ステータスとも 0)。 */
export const ZERO_SP: Stats = { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 };

/** 無補正の性格(上昇・下降なし)。 */
export const NEUTRAL_NATURE: Nature = { plus: "", minus: "" };

/** 特性なし(種族の特性が1つも一覧に解決できなかったときの既定)。 */
export const NO_ABILITY: Ability = { id: "", nameJa: "", effect: null };

/** 防御を上げる・特防を上げる判定のしきい値(4096 が等倍。CLAUDE.md ドメイン規約の4096基準)。 */
const STAT_MOD_NEUTRAL = 4096;

/** マスタの種族から engine の Species の形だけにする(画面のための learnset を落とす)。 */
export function toEngineSpecies(species: MasterSpecies): Species {
  const { key, dexNo, form, nameJa, types, baseStats, abilities } = species;
  return { key, dexNo, form, nameJa, types, baseStats, abilities };
}

/** 種族の特性の先頭(特性一覧に解決できるもの)を使う。1つも解決できなければ特性なし。 */
export function defaultAbility(species: MasterSpecies, abilities: readonly Ability[]): Ability {
  for (const id of species.abilities) {
    const found = abilities.find((ability) => ability.id === id);
    if (found !== undefined) {
      return found;
    }
  }
  return NO_ABILITY;
}

export interface BuildIndividualInput {
  readonly sp: Stats;
  readonly nature: Nature;
  readonly item: Item | null;
  readonly ability: Ability;
}

/** レベル 50・指定の SP・性格・持ち物・特性の個体を作る(P4-2 ではランクを入力しない)。 */
export function buildIndividual(species: MasterSpecies, input: BuildIndividualInput): Individual {
  return {
    species: toEngineSpecies(species),
    level: BATTLE_LEVEL,
    nature: input.nature,
    ability: input.ability,
    item: input.item,
    sp: input.sp,
  };
}

export interface BuildBulkRequestInput {
  readonly attacker: Individual;
  readonly defenderSpecies: MasterSpecies;
  readonly move: Move;
  readonly typeChart: TypeChart;
  /** 防御側の持ち物の差し替え候補(ADR-0016 §6)。省くと engine は持ち物なしの5行を返す。 */
  readonly itemVariants?: ReadonlyArray<Item | null>;
}

/** 一括計算リクエスト。presetKeys・presets を省いて engine の既定の5行にする(ADR-0009)。 */
export function buildBulkRequest(input: BuildBulkRequestInput): BulkRequest {
  const { attacker, defenderSpecies, move, typeChart, itemVariants } = input;
  return {
    format: "single",
    attacker,
    defenderSpecies: toEngineSpecies(defenderSpecies),
    move,
    typeChart,
    ...(itemVariants === undefined ? {} : { itemVariants }),
  };
}

export interface BuildCalcRequestInput {
  readonly attacker: Individual;
  readonly defender: Individual;
  readonly move: Move;
  readonly typeChart: TypeChart;
}

/** 1対1の計算リクエスト。 */
export function buildCalcRequest(input: BuildCalcRequestInput): CalcRequest {
  const { attacker, defender, move, typeChart } = input;
  return { format: "single", attacker, defender, move, typeChart };
}

/**
 * 防御側の持ち物の差し替え候補(ADR-0016 §6)。ID・名前では選ばず、効果データから選ぶ:
 * 技の分類に対応する防御側ステータス(物理→def、特殊→spd)を上げる、または技のタイプを半減する
 * (resistBerryType が技のタイプと一致)。マスタの順序をそのまま使う(並べ替えない)。
 */
export function defensiveItemCandidates(items: readonly Item[], move: Move): Item[] {
  if (move.category === "status") {
    return [];
  }
  const relevantStat: StatKey = move.category === "physical" ? "def" : "spd";
  return items.filter((item) => {
    const effect = item.effect;
    if (effect === null) {
      return false;
    }
    const statMod = effect.statMods?.[relevantStat] ?? 0;
    return statMod > STAT_MOD_NEUTRAL || effect.resistBerryType === move.type;
  });
}

export interface DefenderItemVariantsInput {
  /** 画面で選んだ防御側の持ち物(未選択は null)。 */
  readonly selectedItem: Item | null;
  /** 「持ち物の候補も比較」のトグル。 */
  readonly compare: boolean;
  /** defensiveItemCandidates の候補。 */
  readonly candidates: readonly Item[];
}

/**
 * 一括計算に渡す itemVariants を、選んだ持ち物と「候補も比較」のトグルから決める(ADR-0016 §6)。
 * 比較なしで持ち物も選ばなければ undefined を返す(engine に itemVariants を渡さず、既定の持ち物なし5行にする)。
 */
export function defenderItemVariants(
  input: DefenderItemVariantsInput,
): ReadonlyArray<Item | null> | undefined {
  const { selectedItem, compare, candidates } = input;
  if (!compare) {
    return selectedItem === null ? undefined : [selectedItem];
  }
  if (selectedItem === null || candidates.some((item) => item.id === selectedItem.id)) {
    return [null, ...candidates];
  }
  return [null, selectedItem, ...candidates];
}
