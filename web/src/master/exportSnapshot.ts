// P4-5(ADR-0301 §5): Web の例データを calc-svc が読むマスタ一式(api/openapi.yaml の MasterExport。
// ADR-0204)に書き出す。calc-svc をこの出力で起動すれば、Web の例データの ID がそのまま API に通る
// (オンラインの動作確認・P4-6 の E2E)。
//
// 形の規則(ADR-0204 で変わった点。詳細は ADR-0301 §5 追記):
//   - 相性表は MasterExport **本体**に含める(types / typeChart)。CALC_TYPECHART_PATH は廃止された
//     (services/calc/cmd/calc/main.go。設定されていると calc-svc は起動しない)ので、別出しにしない。
//   - calc-svc のローダー(services/calc/internal/master/export.go)は未知のフィールドをエラーにするので、
//     画面だけの追加フィールド(species.learnset)は書かない。種族・技・持ち物・特性・性格は
//     MasterExport の形(MasterSpecies / MasterMove / MasterItem / MasterAbility / MasterNature)にそろえる。
//   - 効果(items / abilities の effect)のキーは共通マスタ(services/internal/master/effects.go)が受け付ける
//     名前(PascalCase)。Web の DTO は camelCase なので、書き出しのときに変換する
//     (値そのもの・タイプ/ステータス ID をキーに持つ辞書の中のキーは変換しない)。

import { isTypeId, typeNameJa } from "../i18n/ja";
import type { AbilityEffect, ItemEffect, Move, Stats } from "../engine/types";
import type { MasterData, MasterNature } from "./types";

/** MasterExport の schemaVersion(api/openapi.yaml)。いまは 1 だけ。 */
const CALC_SNAPSHOT_SCHEMA_VERSION = 1;

/**
 * dataVersion(api/openapi.yaml の MasterExport.dataVersion)。空でない識別子で、例データ由来だと
 * 分かる名前にする(ADR-0002: 架空のデータだと分かる名前)。呼ぶたびに変わらない定数。
 */
const CALC_SNAPSHOT_DATA_VERSION = "web-example-master-v1";

/** スナップショットのタイプ(MasterType)。 */
export interface CalcSnapshotType {
  readonly id: string;
  readonly sortOrder: number;
  readonly nameJa: string;
}

/** スナップショットの相性表の行(MasterTypeChartEntry)。 */
export interface CalcSnapshotTypeChartEntry {
  readonly attackType: string;
  readonly defenseType: string;
  readonly code: number;
}

/** スナップショットの種族の特性の1行(MasterSpeciesAbility)。 */
export interface CalcSnapshotSpeciesAbility {
  readonly slot: number;
  readonly abilityId: string;
}

/** スナップショットの種族(MasterSpecies。learnset は含まない)。 */
export interface CalcSnapshotSpecies {
  readonly key: string;
  readonly dexNo: number;
  readonly form: number;
  readonly showdownId: string;
  readonly nameJa: string;
  readonly type1: string;
  readonly type2: string | null;
  readonly baseStats: Stats;
  readonly isMega: boolean;
  readonly baseSpeciesKey: string | null;
  readonly requiredItemId: string | null;
  readonly abilities: readonly CalcSnapshotSpeciesAbility[];
}

/** スナップショットの技(MasterMove。例データに追加効果・機構は無いので effect は null・mechanisms は空配列)。 */
export interface CalcSnapshotMove extends Move {
  readonly effect: null;
  readonly mechanisms: readonly string[];
}

/** 共通マスタ(services/internal/master/effects.go)が読む形にした効果(キーは PascalCase)。 */
export type CalcSnapshotEffect = Readonly<Record<string, unknown>>;

/** スナップショットの持ち物(MasterItem)。 */
export interface CalcSnapshotItem {
  readonly id: string;
  readonly nameJa: string;
  readonly effect: CalcSnapshotEffect | null;
}

/** スナップショットの特性(MasterAbility)。 */
export interface CalcSnapshotAbility {
  readonly id: string;
  readonly nameJa: string;
  readonly effect: CalcSnapshotEffect | null;
}

/** calc-svc が読むマスタ一式(api/openapi.yaml の MasterExport。ADR-0204)。 */
export interface CalcSnapshot {
  readonly schemaVersion: 1;
  readonly dataVersion: string;
  readonly types: readonly CalcSnapshotType[];
  readonly typeChart: readonly CalcSnapshotTypeChartEntry[];
  readonly species: readonly CalcSnapshotSpecies[];
  readonly moves: readonly CalcSnapshotMove[];
  readonly items: readonly CalcSnapshotItem[];
  readonly abilities: readonly CalcSnapshotAbility[];
  readonly natures: readonly MasterNature[];
}

/**
 * defAbsorbTypes の値(AbsorbEffect)のように、値そのものがさらにフィールド名を持つオブジェクトの辞書。
 * これらは外側のキー(タイプ/ステータス ID)は変換せず、値のフィールド名だけ PascalCase にする。
 * それ以外(statMods・defResistType 等)は値の中身がタイプ/ステータス ID をキーに持つだけなので、
 * 何もしない(トップレベルのフィールド名だけを変換する既定の扱いでよい)。
 */
const NESTED_EFFECT_OBJECT_FIELDS = new Set<string>(["defAbsorbTypes"]);

function capitalizeKey(key: string): string {
  return `${key.charAt(0).toUpperCase()}${key.slice(1)}`;
}

/**
 * 効果オブジェクトのフィールド名を camelCase → PascalCase に変換する(services/internal/master/effects.go
 * の itemEffectFields / abilityEffectFields / absorbEffectFields が読む形)。タイプ/ステータス ID を
 * キーに持つ辞書(statMods・defResistType・defAbsorbTypes の外側のキー)は変換しない。
 * defAbsorbTypes の値(AbsorbEffect)のように、値自身がフィールド名を持つオブジェクトの辞書は
 * 同じ規則を再帰的に適用する(将来の入れ子の効果に備える。例データには無い)。
 */
function toPascalCaseEffect(effect: object | null): CalcSnapshotEffect | null {
  if (effect === null) {
    return null;
  }
  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(effect as Record<string, unknown>)) {
    if (NESTED_EFFECT_OBJECT_FIELDS.has(key) && value !== null && typeof value === "object") {
      out[capitalizeKey(key)] = Object.fromEntries(
        Object.entries(value as Record<string, unknown>).map(([id, nested]) => [
          id,
          toPascalCaseEffect(nested as object),
        ]),
      );
      continue;
    }
    out[capitalizeKey(key)] = value;
  }
  return out;
}

function toCalcSnapshotItemEffect(effect: ItemEffect | null): CalcSnapshotEffect | null {
  return toPascalCaseEffect(effect);
}

function toCalcSnapshotAbilityEffect(effect: AbilityEffect | null): CalcSnapshotEffect | null {
  return toPascalCaseEffect(effect);
}

/**
 * MasterData を calc-svc のマスタ一式(MasterExport)の形にする。入力を書き換えない
 * (species・moves・items・abilities は新しい配列・オブジェクトに詰め直す。natures・baseStats は
 * 変換が要らないので参照をそのまま使うが、こちらも書き換えはしない)。
 */
export function toCalcSnapshot(master: MasterData): CalcSnapshot {
  const typeIds = master.typeChart.types;

  const types: CalcSnapshotType[] = typeIds.map((id, index) => {
    if (!isTypeId(id)) {
      throw new Error(`相性表に未知のタイプがある: ${id}`);
    }
    return { id, sortOrder: index, nameJa: typeNameJa[id] };
  });

  const typeChart: CalcSnapshotTypeChartEntry[] = [];
  for (const attackType of typeIds) {
    for (const defenseType of typeIds) {
      typeChart.push({
        attackType,
        defenseType,
        code: master.typeChart.effectiveness[attackType]?.[defenseType] ?? 2,
      });
    }
  }

  const species: CalcSnapshotSpecies[] = master.species.map((entry) => {
    const [type1, type2] = entry.types;
    if (type1 === undefined) {
      throw new Error(`種族 ${entry.key} にタイプが無い`);
    }
    return {
      key: entry.key,
      dexNo: entry.dexNo,
      form: entry.form,
      showdownId: entry.key.replace(/-/g, ""),
      nameJa: entry.nameJa,
      type1,
      type2: type2 ?? null,
      baseStats: entry.baseStats,
      isMega: false,
      baseSpeciesKey: null,
      requiredItemId: null,
      abilities: entry.abilities.map((abilityId, index) => ({ slot: index + 1, abilityId })),
    };
  });

  // critic指摘: species・items・abilitiesと同じく明示的にフィールドを拾う(スプレッドだと入力に
  // 余計なフィールドが混ざったとき黙って書き出され、calc-svcのDisallowUnknownFieldsで起動失敗になる)。
  const moves: CalcSnapshotMove[] = master.moves.map((move) => ({
    id: move.id,
    nameJa: move.nameJa,
    type: move.type,
    category: move.category,
    power: move.power,
    priority: move.priority,
    effect: null,
    mechanisms: [],
  }));

  const items: CalcSnapshotItem[] = master.items.map((item) => ({
    id: item.id,
    nameJa: item.nameJa,
    effect: toCalcSnapshotItemEffect(item.effect),
  }));

  const abilities: CalcSnapshotAbility[] = master.abilities.map((ability) => ({
    id: ability.id,
    nameJa: ability.nameJa,
    effect: toCalcSnapshotAbilityEffect(ability.effect),
  }));

  return {
    schemaVersion: CALC_SNAPSHOT_SCHEMA_VERSION,
    dataVersion: CALC_SNAPSHOT_DATA_VERSION,
    types,
    typeChart,
    species,
    moves,
    items,
    abilities,
    natures: master.natures,
  };
}
