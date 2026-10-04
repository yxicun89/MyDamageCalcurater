// I-web-8 = F-09(ADR-0333): お気に入りに「そのときの計算の入力」(calc = API の CalcRequest)を保存し、
// 一覧から選んだら計算画面に戻すための純粋関数。計算画面の状態(FavoriteCalcState)と CalcRequest の往復を1か所で決める。

import type { components } from "../api/openapi.gen";
import { resolveNatureId } from "../api/apiEngine";
import {
  resolveAttackerStats,
  type AttackerStatInputs,
  type NatureModifier,
} from "../domain/attackerStatInputs";
import {
  DEFAULT_CALC_CONDITIONS,
  TERRAIN_IDS,
  WEATHER_IDS,
  conditionRequestParts,
  type CalcConditions,
  type TerrainId,
  type WeatherId,
} from "../domain/calcConditions";
import { itemsForRole } from "../domain/itemRoles";
import { megaItemLock, megaStoneItemIds } from "../domain/mega";
import { damagingLearnsetMoves } from "../domain/moves";
import { BATTLE_LEVEL, NEUTRAL_NATURE, selectableAbilities } from "../domain/requests";
import type { Ability, Move, MoveCategory } from "../engine/types";
import type { MasterItem, MasterNature, MasterSpecies } from "../master/types";

type Schemas = components["schemas"];
type CalcRequest = Schemas["CalcRequest"];
type Individual = Schemas["Individual"];
type Favorite = Schemas["Favorite"];

/** 保存内容(本文)の上限バイト数(ADR-0228 §3。api/openapi.yaml の FavoriteInput.calc の説明と同じ)。 */
export const MAX_FAVORITE_SAVED_BYTES = 4096;

/** 計算画面の状態のうち、お気に入りに保存・復元するもの。 */
export interface FavoriteCalcState {
  readonly attackerKey: string;
  readonly defenderKey: string;
  readonly moveId: string;
  readonly attackerItemId: string;
  readonly defenderItemId: string;
  /** 攻撃側の特性。保存するときは実際に使う特性の ID(復元では引けなければ "" = 種族の先頭)。 */
  readonly attackerAbilityId: string;
  /** 防御側の特性。"" は「おまかせ」。 */
  readonly defenderAbilityId: string;
  readonly attackerStatInputs: AttackerStatInputs;
  readonly conditions: CalcConditions;
}

/** 復元で引く先(画面のマスタ。オンラインは検索で解決した分を足した一覧)。 */
export interface FavoriteRestoreLookup {
  readonly attackerSpecies: MasterSpecies | null;
  readonly defenderSpecies: MasterSpecies | null;
  readonly moves: readonly Move[];
  readonly items: readonly MasterItem[];
  /** メガストーンの判別集合(画面と同じ。省略は lookup の2種族から導く)。 */
  readonly stoneIds?: ReadonlySet<string>;
  readonly abilities: readonly Ability[];
  readonly natures: readonly MasterNature[];
}

/** 画面に渡す復元の要求。token が変わったときだけ適用する(同じお気に入りでも token が進めばもう一度戻す)。 */
export interface FavoriteRestoreRequest {
  readonly token: number;
  readonly favorite: Favorite;
}

type Side = "attacker" | "defender";

/** 戻せなかった・反映しなかった項目。 */
export type FavoriteRestoreIssue =
  | { readonly kind: "species"; readonly side: Side; readonly id: string }
  | { readonly kind: "move"; readonly id: string }
  | { readonly kind: "item"; readonly side: Side; readonly id: string }
  | { readonly kind: "ability"; readonly side: Side; readonly id: string }
  | { readonly kind: "nature"; readonly id: string }
  | {
      readonly kind: "megaItem";
      readonly side: Side;
      readonly savedItemId: string;
      readonly lockedItemId: string;
    }
  | { readonly kind: "ignored"; readonly field: string };

export interface FavoriteCalcOfInput {
  readonly state: FavoriteCalcState;
  readonly natures: readonly MasterNature[];
  /** 技の分類(性格の決め方に使う。ADR-0329 §4)。 */
  readonly moveCategory: MoveCategory;
}

/** 画面の状態を CalcRequest にする。入力が揃っていない・不正・性格を決められないときは null。既定の条件はキーごと省く。 */
export function favoriteCalcOf(input: FavoriteCalcOfInput): CalcRequest | null {
  const { state, natures, moveCategory } = input;
  if (state.attackerKey === "" || state.defenderKey === "" || state.moveId === "") {
    return null;
  }
  const stats = resolveAttackerStats(state.attackerStatInputs, natures, moveCategory);
  if (!stats.ok) {
    return null;
  }
  const attackerNatureId = resolveNatureId(natures, stats.nature);
  const defenderNatureId = resolveNatureId(natures, NEUTRAL_NATURE);
  if (attackerNatureId === undefined || defenderNatureId === undefined) {
    return null;
  }
  const parts = conditionRequestParts(state.conditions);
  const attacker: Individual = {
    speciesKey: state.attackerKey,
    level: BATTLE_LEVEL,
    natureId: attackerNatureId,
    sp: stats.sp,
    ...(state.attackerAbilityId === "" ? {} : { abilityId: state.attackerAbilityId }),
    ...(state.attackerItemId === "" ? {} : { itemId: state.attackerItemId }),
    ...(parts.ranks === undefined ? {} : { ranks: parts.ranks }),
    ...(parts.status === undefined ? {} : { status: parts.status }),
  };
  const defender: Individual = {
    speciesKey: state.defenderKey,
    level: BATTLE_LEVEL,
    natureId: defenderNatureId,
    sp: { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 },
    ...(state.defenderAbilityId === "" ? {} : { abilityId: state.defenderAbilityId }),
    ...(state.defenderItemId === "" ? {} : { itemId: state.defenderItemId }),
    ...(parts.defenderOverride === undefined ? {} : { ranks: parts.defenderOverride.ranks }),
  };
  return {
    format: "single",
    attacker,
    defender,
    moveId: state.moveId,
    ...(parts.field === undefined ? {} : { field: fieldOf(state.conditions) }),
    ...(parts.critical === undefined ? {} : { options: { critical: true } }),
  };
}

/** 場(天候・フィールド・防御側の壁)。既定のキーは省く(conditionRequestParts と同じ条件)。 */
function fieldOf(conditions: CalcConditions): NonNullable<CalcRequest["field"]> {
  const { weather, terrain, defenderScreens } = conditions;
  const anyScreen = defenderScreens.reflect || defenderScreens.lightScreen || defenderScreens.auroraVeil;
  return {
    ...(weather === "none" ? {} : { weather }),
    ...(terrain === "none" ? {} : { terrain }),
    ...(anyScreen ? { defenderScreens } : {}),
  };
}

export interface FavoriteLabelInput {
  readonly attackerNameJa: string;
  readonly defenderNameJa: string | null;
  readonly moveNameJa: string | null;
}

/** お気に入りの見出し。揃っていれば「攻撃側→防御側(技)」、揃っていなければ攻撃側の名前。 */
export function favoriteLabelOf(input: FavoriteLabelInput): string {
  const { attackerNameJa, defenderNameJa, moveNameJa } = input;
  if (defenderNameJa === null || moveNameJa === null) {
    return attackerNameJa;
  }
  return `${attackerNameJa}→${defenderNameJa}(${moveNameJa})`;
}

// ---- 復元 ----

/** 性格の補正から、攻撃(A)・特攻(C)の補正表示を決める。 */
function modifierOf(nature: MasterNature, stat: "atk" | "spa"): NatureModifier {
  if (nature.plus === stat) {
    return "up";
  }
  return nature.minus === stat ? "down" : "neutral";
}

interface RestoredAttackerSide {
  readonly attackerKey: string;
  readonly attackerItemId: string;
  readonly attackerAbilityId: string;
  readonly attackerStatInputs: AttackerStatInputs;
  readonly issues: FavoriteRestoreIssue[];
}

function isSpecies(species: MasterSpecies | null): species is MasterSpecies {
  return species !== null;
}

/** 持ち物を戻す。メガ種族は固定を優先し、食い違いは megaItem。引けなければ空で item。 */
function restoreItemId(
  side: Side,
  species: MasterSpecies | null,
  savedItemId: string | null | undefined,
  lookup: FavoriteRestoreLookup,
  issues: FavoriteRestoreIssue[],
): string {
  const saved = savedItemId ?? "";
  const { items } = lookup;
  const lock = megaItemLock(species, items);
  if (lock.kind === "locked") {
    if (saved !== lock.item.id) {
      issues.push({ kind: "megaItem", side, savedItemId: saved, lockedItemId: lock.item.id });
    }
    return lock.item.id;
  }
  if (lock.kind === "missing" || saved === "") {
    return "";
  }
  // 画面の持ち物欄と同じ判定(ADR-0326: 役割に合うものだけ・メガストーンは単独で選べない)で戻せるか調べる。
  const stones =
    lookup.stoneIds ?? megaStoneItemIds([lookup.attackerSpecies, lookup.defenderSpecies].filter(isSpecies));
  if (itemsForRole(items, side, stones).some((item) => item.id === saved)) {
    return saved;
  }
  issues.push({ kind: "item", side, id: saved });
  return "";
}

/** 特性を戻す。種族が引けない・種族の特性に無ければ空(既定)で、無いときだけ ability。 */
function restoreAbilityId(
  side: Side,
  species: MasterSpecies | null,
  savedAbilityId: string | null | undefined,
  abilities: readonly Ability[],
  issues: FavoriteRestoreIssue[],
): string {
  const saved = savedAbilityId ?? "";
  if (species === null || saved === "") {
    return "";
  }
  if (selectableAbilities(species, abilities).some((ability) => ability.id === saved)) {
    return saved;
  }
  issues.push({ kind: "ability", side, id: saved });
  return "";
}

/** 攻撃側の個体(種族・持ち物・特性・A/C の SP と補正)を戻す。 */
function restoreAttackerSide(individual: Individual, lookup: FavoriteRestoreLookup): RestoredAttackerSide {
  const issues: FavoriteRestoreIssue[] = [];
  const species = lookup.attackerSpecies;
  if (species === null) {
    issues.push({ kind: "species", side: "attacker", id: individual.speciesKey });
  }
  const attackerItemId = restoreItemId("attacker", species, individual.itemId, lookup, issues);
  const attackerAbilityId = restoreAbilityId(
    "attacker",
    species,
    individual.abilityId,
    lookup.abilities,
    issues,
  );
  const nature = lookup.natures.find((entry) => entry.id === individual.natureId);
  if (nature === undefined) {
    issues.push({ kind: "nature", id: individual.natureId });
  }
  const statInput = (stat: "atk" | "spa"): AttackerStatInputs[typeof stat] => ({
    spText: String(individual.sp[stat]),
    modifier: nature === undefined ? "neutral" : modifierOf(nature, stat),
  });
  const attackerStatInputs: AttackerStatInputs = { atk: statInput("atk"), spa: statInput("spa") };
  return { attackerKey: species?.key ?? "", attackerItemId, attackerAbilityId, attackerStatInputs, issues };
}

/** calc の無い旧お気に入り: individual から攻撃側だけを戻す(AC-5)。 */
export function restoreFavoriteAttacker(
  individual: Individual,
  lookup: FavoriteRestoreLookup,
): RestoredAttackerSide {
  return restoreAttackerSide(individual, lookup);
}

const SP_STATS_NOT_EDITABLE = ["hp", "def", "spd", "spe"] as const;

function anyNonZero(values: readonly number[]): boolean {
  return values.some((value) => value !== 0);
}

/** 画面で表せない項目(戻さず ignored として知らせる)。 */
function ignoredFields(calc: CalcRequest, natures: readonly MasterNature[]): string[] {
  const { attacker, defender } = calc;
  const fields: string[] = [];
  if (calc.format !== "single") {
    fields.push("format");
  }
  if (anyNonZero(SP_STATS_NOT_EDITABLE.map((stat) => attacker.sp[stat]))) {
    fields.push("attackerSp");
  }
  if (attacker.teraType !== undefined && attacker.teraType !== null) {
    fields.push("attackerTeraType");
  }
  if (attacker.status !== undefined && attacker.status !== "none" && attacker.status !== "burn") {
    fields.push("attackerStatus");
  }
  if (
    attacker.ranks !== undefined &&
    anyNonZero([attacker.ranks.def, attacker.ranks.spd, attacker.ranks.spe])
  ) {
    fields.push("attackerRanks");
  }
  const screens = calc.field?.attackerScreens;
  if (screens !== undefined && (screens.reflect || screens.lightScreen || screens.auroraVeil)) {
    fields.push("attackerScreens");
  }
  const defenderNature = natures.find((entry) => entry.id === defender.natureId);
  const defenderRanks = defender.ranks;
  const defenderBuilt =
    anyNonZero(Object.values(defender.sp)) ||
    (defenderNature !== undefined && (defenderNature.plus !== null || defenderNature.minus !== null)) ||
    (defender.teraType !== undefined && defender.teraType !== null) ||
    (defender.status !== undefined && defender.status !== "none") ||
    (defenderRanks !== undefined && anyNonZero([defenderRanks.atk, defenderRanks.spa, defenderRanks.spe]));
  if (defenderBuilt) {
    fields.push("defenderBuild");
  }
  return fields;
}

function weatherOf(value: string | undefined): WeatherId {
  return WEATHER_IDS.find((id) => id === value) ?? "none";
}

function terrainOf(value: string | undefined): TerrainId {
  return TERRAIN_IDS.find((id) => id === value) ?? "none";
}

/** 保存された calc の条件(急所・やけど・天候・場・壁・ランク)を画面の条件に戻す。 */
function restoreConditions(calc: CalcRequest): CalcConditions {
  const screens = calc.field?.defenderScreens;
  return {
    ...DEFAULT_CALC_CONDITIONS,
    critical: calc.options?.critical === true,
    burned: calc.attacker.status === "burn",
    weather: weatherOf(calc.field?.weather),
    terrain: terrainOf(calc.field?.terrain),
    defenderScreens: {
      reflect: screens?.reflect === true,
      lightScreen: screens?.lightScreen === true,
      auroraVeil: screens?.auroraVeil === true,
    },
    ranks: { atk: calc.attacker.ranks?.atk ?? 0, spa: calc.attacker.ranks?.spa ?? 0 },
    defenderRanks: { def: calc.defender.ranks?.def ?? 0, spd: calc.defender.ranks?.spd ?? 0 },
  };
}

export interface FavoriteRestoreResult {
  readonly state: FavoriteCalcState;
  readonly issues: readonly FavoriteRestoreIssue[];
}

/** 保存された calc を画面の状態に戻す。引けたものは戻し、引けないものは空(既定)にして issues に入れる。 */
export function restoreFavoriteCalc(calc: CalcRequest, lookup: FavoriteRestoreLookup): FavoriteRestoreResult {
  const attacker = restoreAttackerSide(calc.attacker, lookup);
  const issues = [...attacker.issues];

  const defenderSpecies = lookup.defenderSpecies;
  if (defenderSpecies === null) {
    issues.push({ kind: "species", side: "defender", id: calc.defender.speciesKey });
  }
  const defenderItemId = restoreItemId("defender", defenderSpecies, calc.defender.itemId, lookup, issues);
  const defenderAbilityId = restoreAbilityId(
    "defender",
    defenderSpecies,
    calc.defender.abilityId,
    lookup.abilities,
    issues,
  );

  let moveId = "";
  const attackerSpecies = lookup.attackerSpecies;
  if (attackerSpecies !== null) {
    if (damagingLearnsetMoves(attackerSpecies, lookup.moves).some((move) => move.id === calc.moveId)) {
      moveId = calc.moveId;
    } else {
      issues.push({ kind: "move", id: calc.moveId });
    }
  }
  for (const field of ignoredFields(calc, lookup.natures)) {
    issues.push({ kind: "ignored", field });
  }

  return {
    state: {
      attackerKey: attacker.attackerKey,
      defenderKey: defenderSpecies?.key ?? "",
      moveId,
      attackerItemId: attacker.attackerItemId,
      defenderItemId,
      attackerAbilityId: attacker.attackerAbilityId,
      defenderAbilityId,
      attackerStatInputs: attacker.attackerStatInputs,
      conditions: restoreConditions(calc),
    },
    issues,
  };
}
