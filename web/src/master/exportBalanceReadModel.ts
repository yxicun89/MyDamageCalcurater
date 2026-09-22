// P4-12a(ADR-0303 §4): Web の例データを balance-svc の read model(services/balance/schema/ の3つ)にも書き出す。
// pokedex の read model に揃うまでの間、ローカルと E2E ではこの出力で balance-svc を起動し、Web の例データの ID
// (pokemonId = 種族キー・moveId・abilityId)がそのまま balance の API に通るようにする。
// 特性は、engine の効果定義から balance の正規化された効果(無効・タイプ倍率・弱点半減倍率)に写せるものだけを書く。
// 写せる効果が無い特性(effect が null、または全ての効果が写せない)は effects: [] にする。

import type { Ability, AbilityEffect } from "../engine/types";
import type { MasterData } from "./types";

// P2-3b(ADR-0106)追記: engine.AbilityEffect に defImmuneTypes・defAbsorbTypes が増えたので、
// balance の read model にも "immune"・"absorb" を書く(ADR-0106 §決定7)。
// engine 側の無効は defImmuneTypes が正で、defResistType はあくまで倍率(0 を書いても DefResistType が
// ダメージを 0 にすることはなく、最低1ダメージの床で 1 になる。ADR-0106「却下した案」参照)。
// ただし P4-12a で作ったこのエクスポータは defResistType[t] === 0 を無効の書き方として受け付けており、
// 既存テストがそれを固定しているため、当面は両方の書き方を immune に写す(engine の規約ではなく、
// この Web ローカルのエクスポータ固有の互換対応)。

/** balance の read model の schemaVersion(services/balance/schema/*.schema.json)。 */
const BALANCE_SCHEMA_VERSION = 1;

/** ADR-0017 §3: 効果の分数は 4096 分の m(engine の固定小数の基準)。 */
const FIXED_POINT_BASE = 4096;

/** services/balance/schema/abilities.schema.json の factor(既約分数の分子・分母)の範囲。 */
const FACTOR_MIN = 1;
const FACTOR_MAX = 16;

/** balance の pokemon-types.schema.json の1件。 */
export interface BalancePokemonTypeEntry {
  readonly pokemonId: string;
  readonly nameJa: string;
  readonly types: readonly string[];
  readonly abilityIds: readonly string[];
}

/** balance の pokemon-types.schema.json(BALANCE_POKEMON_TYPES_PATH)。 */
export interface BalancePokemonTypes {
  readonly schemaVersion: 1;
  readonly pokemon: readonly BalancePokemonTypeEntry[];
}

/** balance の moves.schema.json の1件。 */
export interface BalanceMoveEntry {
  readonly moveId: string;
  readonly type: string;
  readonly category: string;
}

/** balance の moves.schema.json(BALANCE_MOVES_PATH)。 */
export interface BalanceMoves {
  readonly schemaVersion: 1;
  readonly moves: readonly BalanceMoveEntry[];
}

/** balance の abilities.schema.json の効果(ADR-0017 §2)。 */
export type BalanceAbilityEffect =
  | { readonly kind: "immune"; readonly attackType: string }
  | { readonly kind: "absorb"; readonly attackType: string }
  | {
      readonly kind: "type_multiplier";
      readonly attackType: string;
      readonly numerator: number;
      readonly denominator: number;
    }
  | { readonly kind: "super_effective_multiplier"; readonly numerator: number; readonly denominator: number };

/** balance の abilities.schema.json の1件。 */
export interface BalanceAbilityEntry {
  readonly abilityId: string;
  readonly effects: readonly BalanceAbilityEffect[];
}

/** balance の abilities.schema.json(BALANCE_ABILITIES_PATH)。 */
export interface BalanceAbilities {
  readonly schemaVersion: 1;
  readonly abilities: readonly BalanceAbilityEntry[];
}

/** MasterData を balance の pokemon-types.schema.json の形にする(learnset・baseStats は書かない)。 */
export function toBalancePokemonTypes(master: MasterData): BalancePokemonTypes {
  return {
    schemaVersion: BALANCE_SCHEMA_VERSION,
    pokemon: master.species.map((species) => ({
      pokemonId: species.key,
      nameJa: species.nameJa,
      types: species.types,
      abilityIds: species.abilities,
    })),
  };
}

/** MasterData を balance の moves.schema.json の形にする(変化技も含める)。 */
export function toBalanceMoves(master: MasterData): BalanceMoves {
  return {
    schemaVersion: BALANCE_SCHEMA_VERSION,
    moves: master.moves.map((move) => ({ moveId: move.id, type: move.type, category: move.category })),
  };
}

/** 整数の既約分数(gcd で約分する。0除算はここでは起きない: 呼び出し側が分母 4096・分子>0 を渡す)。 */
function reduceFraction(numerator: number, denominator: number): { numerator: number; denominator: number } {
  const gcd = (a: number, b: number): number => (b === 0 ? a : gcd(b, a % b));
  const divisor = gcd(numerator, denominator);
  return { numerator: numerator / divisor, denominator: denominator / divisor };
}

/** 既約分数の分子・分母がどちらも services/balance/schema/abilities.schema.json の factor の範囲(1〜16)に収まるか。 */
function isFactorInRange(value: number): boolean {
  return value >= FACTOR_MIN && value <= FACTOR_MAX;
}

/**
 * AbilityEffect を balance の効果(defResistType・defImmuneTypes・defAbsorbTypes・reduceSuperEffective)に
 * 写す。出力順は ADR-0106 §決定7: immune(タイプ順)→ absorb(タイプ順)→ type_multiplier(タイプ順)→
 * super_effective_multiplier。既約分数が 1〜16 に収まらない効果は書かない。stabMod など防御相性に
 * 関係しない効果は写さない。副次効果(回復・能力上昇)は absorb の行に出さない(ADR-0106 §決定7)。
 * defResistType[t] === 0 も immune として受け付けるのは engine の規約ではなく、このエクスポータが
 * P4-12a から持つローカルな書き方(engine 側の無効は defImmuneTypes が正。上のコメント参照)。
 */
function toBalanceAbilityEffects(
  effect: AbilityEffect | null,
  typeOrder: readonly string[],
): BalanceAbilityEffect[] {
  if (effect === null) {
    return [];
  }
  const effects: BalanceAbilityEffect[] = [];
  const defResistType = effect.defResistType;

  // 無効(immune): defImmuneTypes(ADR-0106)と defResistType の 0(P4-12a からの書き方)の両方を受け付け、
  // 同じタイプが両方にあっても1件にまとめる。
  const immuneTypes = new Set<string>(effect.defImmuneTypes ?? []);
  if (defResistType !== undefined) {
    for (const attackType of typeOrder) {
      if (defResistType[attackType] === 0) {
        immuneTypes.add(attackType);
      }
    }
  }
  for (const attackType of typeOrder) {
    if (immuneTypes.has(attackType)) {
      effects.push({ kind: "immune", attackType });
    }
  }

  // 吸収(absorb): defAbsorbTypes のキーだけを見る。副次効果(回復・能力上昇)は balance に出さない。
  // 同じタイプが immune と absorb の両方にあるのは不正な入力(WASM 境界が拒否する。ADR-0106 §決定1)だが、
  // それでも結果が割れないよう engine と同じ優先規則にする: 無効が吸収に勝つ(immune を絶対に落とさない)。
  const defAbsorbTypes = effect.defAbsorbTypes;
  if (defAbsorbTypes !== undefined) {
    for (const attackType of typeOrder) {
      if (attackType in defAbsorbTypes && !immuneTypes.has(attackType)) {
        effects.push({ kind: "absorb", attackType });
      }
    }
  }

  // タイプ倍率(type_multiplier): defResistType の 0 以外の値(0 は上で immune に回した)。
  if (defResistType !== undefined) {
    for (const attackType of typeOrder) {
      const value = defResistType[attackType];
      if (value === undefined || value === 0) {
        continue;
      }
      const { numerator, denominator } = reduceFraction(value, FIXED_POINT_BASE);
      if (isFactorInRange(numerator) && isFactorInRange(denominator)) {
        effects.push({ kind: "type_multiplier", attackType, numerator, denominator });
      }
    }
  }

  // 弱点半減倍率(super_effective_multiplier): 最後。
  if (effect.reduceSuperEffective !== undefined) {
    const { numerator, denominator } = reduceFraction(effect.reduceSuperEffective, FIXED_POINT_BASE);
    if (isFactorInRange(numerator) && isFactorInRange(denominator)) {
      effects.push({ kind: "super_effective_multiplier", numerator, denominator });
    }
  }
  return effects;
}

/**
 * MasterData を balance の abilities.schema.json の形にする。特性はすべて書き(analyze はどの特性も選べる
 * ようにするため)、写せる効果が無い特性は effects: [] にする(ADR-0303 §4)。
 */
export function toBalanceAbilities(master: MasterData): BalanceAbilities {
  const typeOrder = master.typeChart.types;
  return {
    schemaVersion: BALANCE_SCHEMA_VERSION,
    abilities: master.abilities.map((ability: Ability) => ({
      abilityId: ability.id,
      effects: toBalanceAbilityEffects(ability.effect, typeOrder),
    })),
  };
}
