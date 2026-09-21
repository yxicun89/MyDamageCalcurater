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
  Observation,
  ReverseRequest,
  ReverseSide,
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

/**
 * 補正なし(等倍)を表す固定小数(4096 基準。CLAUDE.md ドメイン規約)。持ち物の効果が「上げる」側かを
 * 判定するしきい値として使う(defensiveItemCandidates・domain/reverseItems.ts の共通の正)。
 */
export const NEUTRAL_MODIFIER = 4096;

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
 * 防御側の持ち物候補の判定(ADR-0016 §6。技の分類に対応する防御側ステータス(物理→def、特殊→spd)を
 * 上げる、または技のタイプを半減する(resistBerryType が技のタイプと一致))。変化技には対応する
 * 防御側ステータスが無いので候補にしない。この1つの定義を defensiveItemCandidates(このファイル、
 * CalcScreen 用)と domain/reverseItems.ts の防御側の判定(逆算の持ち物候補)が共有する
 * (コーディング規約 §2「同じ定義を複数箇所に書かない」)。
 */
export function isDefensiveItemCandidate(item: Item, move: Move): boolean {
  if (move.category === "status") {
    return false;
  }
  const effect = item.effect;
  if (effect === null) {
    return false;
  }
  const relevantStat: StatKey = move.category === "physical" ? "def" : "spd";
  const statMod = effect.statMods?.[relevantStat] ?? 0;
  return statMod > NEUTRAL_MODIFIER || effect.resistBerryType === move.type;
}

/**
 * 防御側の持ち物の差し替え候補(ADR-0016 §6)。ID・名前では選ばず、効果データから選ぶ
 * (isDefensiveItemCandidate)。マスタの順序をそのまま使う(並べ替えない)。
 */
export function defensiveItemCandidates(items: readonly Item[], move: Move): Item[] {
  return items.filter((item) => isDefensiveItemCandidate(item, move));
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

export interface BuildReverseRequestInput {
  readonly side: ReverseSide;
  /** 既知側(自分)の個体。side=defender なら攻撃側、side=attacker なら防御側として渡す(ADR-0010 §R1)。 */
  readonly known: Individual;
  /** 逆算する相手の種族。SP・性格・持ち物は探索対象なので渡さない。 */
  readonly unknownSpecies: MasterSpecies;
  readonly move: Move;
  readonly typeChart: TypeChart;
  /** 探索する持ち物候補(domain/reverseItems.ts の reverseItemCandidates)。先頭は必ず null。 */
  readonly itemCandidates: ReadonlyArray<Item | null>;
  readonly observations: readonly Observation[];
}

/**
 * 逆算リクエスト(ADR-0010 §R、ADR-0011 §3)。maxCandidates は渡さない
 * (engine は 2 × 持ち物候補数の全候補を返し、候補は高々十数件なので切り取る必要が無い)。
 */
export function buildReverseRequest(input: BuildReverseRequestInput): ReverseRequest {
  const { side, known, unknownSpecies, move, typeChart, itemCandidates, observations } = input;
  return {
    format: "single",
    side,
    known,
    unknownSpecies: toEngineSpecies(unknownSpecies),
    move,
    typeChart,
    itemCandidates,
    observations,
  };
}
