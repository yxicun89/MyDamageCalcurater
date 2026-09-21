// WASM 境界の DTO を TypeScript で書いたもの(ADR-0300 §2、契約は ADR-0011 §3・§13)。
// engine/wasmapi/dto.go・requests.go のフィールド名(lowerCamelCase)と1対1に対応させる。
// WASM 境界は HTTP を通らず OpenAPI の契約ではないので、ここは手書きの型が正(ADR-0300 §2)。
// ずれは engine/wasmEngine.wasm.test.ts(本物の engine.wasm との結合テスト)が検出する。

/** 対戦形式(engine.Format と同じ文字列)。 */
export type FormatId = "single" | "double";

/** ステータスキー(HP を含む)。 */
export type StatKey = "hp" | "atk" | "def" | "spa" | "spd" | "spe";

/** 性格補正のキー。空文字は「補正なし」(ADR-0011 §4)。 */
export type NatureStatKey = StatKey | "";

/** 技の分類。 */
export type MoveCategory = "physical" | "special" | "status";

/** 技の分類のキー。空文字は「全分類」(持ち物・特性の効果が分類を問わないとき)。 */
export type MoveCategoryOrNone = MoveCategory | "";

/** 実数値・種族値・SP の6ステータス。 */
export interface Stats {
  readonly hp: number;
  readonly atk: number;
  readonly def: number;
  readonly spa: number;
  readonly spd: number;
  readonly spe: number;
}

/** 性格補正。plus のステータスが+10%、minus が-10%。両方 "" は無補正。 */
export interface Nature {
  readonly plus: NatureStatKey;
  readonly minus: NatureStatKey;
}

/** ランク補正(戦闘中の能力変化)。HP は無い。 */
export interface Ranks {
  readonly atk: number;
  readonly def: number;
  readonly spa: number;
  readonly spd: number;
  readonly spe: number;
}

/** 壁の展開状況。 */
export interface Screens {
  readonly reflect: boolean;
  readonly lightScreen: boolean;
  readonly auroraVeil: boolean;
}

/** タイプ相性表(ADR-0013・ADR-0011 §13)。倍率は整数コード(0/1/2/4)、等倍は省略可。 */
export interface TypeChart {
  readonly types: readonly string[];
  readonly effectiveness: Readonly<Record<string, Readonly<Record<string, number>>>>;
}

/** 種族(engine の形。マスタの learnset は含まない)。 */
export interface Species {
  readonly key: string;
  readonly dexNo: number;
  readonly form: number;
  readonly nameJa: string;
  readonly types: readonly string[];
  readonly baseStats: Stats;
  readonly abilities: readonly string[];
}

/** 技。 */
export interface Move {
  readonly id: string;
  readonly nameJa: string;
  readonly type: string;
  readonly category: MoveCategory;
  readonly power: number;
  readonly priority: number;
}

/** 持ち物の効果(4096 基準の固定小数。CLAUDE.md ドメイン規約)。すべて省略可(省略はengineの既定値)。 */
export interface ItemEffect {
  readonly statMods?: Readonly<Partial<Record<StatKey, number>>>;
  readonly damageMod?: number;
  readonly powerMod?: number;
  readonly powerCategory?: MoveCategoryOrNone;
  readonly onlySuperEffective?: boolean;
  readonly boostType?: string;
  readonly boostTypeMod?: number;
  readonly resistBerryType?: string;
}

/** 持ち物。effect が null は補正なし。 */
export interface Item {
  readonly id: string;
  readonly nameJa: string;
  readonly effect: ItemEffect | null;
}

/** 特性の効果(4096 基準の固定小数)。すべて省略可。 */
export interface AbilityEffect {
  readonly stabMod?: number;
  readonly offBoostType?: string;
  readonly offBoostTypeMod?: number;
  readonly defResistType?: Readonly<Record<string, number>>;
  readonly reduceSuperEffective?: number;
  readonly ignoresBurn?: boolean;
}

/** 特性。effect が null は補正なし。 */
export interface Ability {
  readonly id: string;
  readonly nameJa: string;
  readonly effect: AbilityEffect | null;
}

/** 個体(攻撃側・防御側)。ranks・teraType・status の省略は engine の既定値(補正なし)。 */
export interface Individual {
  readonly species: Species;
  readonly level: number;
  readonly nature: Nature;
  readonly ability: Ability;
  readonly item: Item | null;
  readonly sp: Stats;
  readonly ranks?: Ranks;
  readonly teraType?: string;
  readonly status?: string;
}

/** 天候・フィールド・壁。省略は engine の既定値(補正なし)。 */
export interface Field {
  readonly weather?: string;
  readonly terrain?: string;
  readonly attackerScreens?: Screens;
  readonly defenderScreens?: Screens;
}

/** 一括計算の防御側プリセット(engine の既定カタログを使うときは渡さない。ADR-0009)。 */
export interface DefenderPreset {
  readonly key: string;
  readonly label: string;
  readonly sp: Stats;
  readonly nature: Nature;
  readonly applies: MoveCategoryOrNone;
}

/** 1対1のダメージ計算リクエスト。 */
export interface CalcRequest {
  readonly format: FormatId;
  readonly attacker: Individual;
  readonly defender: Individual;
  readonly move: Move;
  readonly field?: Field;
  readonly critical?: boolean;
  readonly typeChart: TypeChart;
}

/** 一括計算リクエスト(ADR-0009)。presetKeys / presets を省くと engine の既定5行になる。 */
export interface BulkRequest {
  readonly format: FormatId;
  readonly attacker: Individual;
  readonly defenderSpecies: Species;
  readonly move: Move;
  readonly field?: Field;
  readonly critical?: boolean;
  readonly presets?: readonly DefenderPreset[];
  readonly presetKeys?: readonly string[];
  readonly itemVariants?: ReadonlyArray<Item | null>;
  readonly typeChart: TypeChart;
}

/** 逆算の観測1件。percent / percentTenths / damage のちょうど1つを指定する(ADR-0010 §R2)。 */
export interface Observation {
  readonly percent?: number;
  readonly percentTenths?: number;
  readonly damage?: number;
  readonly note?: string;
}

/** 逆算の対象側。 */
export type ReverseSide = "attacker" | "defender";

/** 逆算リクエスト(ADR-0010 §R、ADR-0011 §3)。 */
export interface ReverseRequest {
  readonly format?: FormatId;
  readonly side: ReverseSide;
  readonly known: Individual;
  readonly unknownSpecies: Species;
  readonly move: Move;
  readonly field?: Field;
  readonly critical?: boolean;
  readonly itemCandidates?: ReadonlyArray<Item | null>;
  readonly observations: readonly Observation[];
  readonly maxCandidates?: number;
  readonly typeChart: TypeChart;
}

/** 確定数(engine.KOChance の写し)。表示は displayChancePercent を使う(ADR-0011 §3)。 */
export interface KOChance {
  readonly hits: number;
  readonly guaranteed: boolean;
  readonly chancePercent: number;
  readonly displayChancePercent: number;
}

/** 1回のダメージ計算の結果。%は engine が小数第1位で返す表示%(丸め直さない)。 */
export interface CalcResult {
  readonly rolls: readonly number[];
  readonly minDamage: number;
  readonly maxDamage: number;
  readonly minPercent: number;
  readonly maxPercent: number;
  readonly defenderHP: number;
  readonly effectiveness: number;
  readonly stab: boolean;
  readonly category: MoveCategory;
  readonly ko: KOChance;
}

/** 一括計算の1行の防御側(SP・性格・実数値だけ。ADR-0011 §3)。 */
export interface BulkDefender {
  readonly sp: Stats;
  readonly nature: Nature;
  readonly stats: Stats;
}

/** 一括計算の1行。 */
export interface BulkRow {
  readonly preset: string;
  readonly presetLabel: string;
  readonly itemId: string;
  readonly defender: BulkDefender;
  readonly result: CalcResult;
}

/** 一括計算の結果。 */
export interface BulkResult {
  readonly defenderSpeciesKey: string;
  readonly rows: readonly BulkRow[];
}

/** 逆算候補の SP 範囲(両端を含む)。 */
export interface SPRange {
  readonly min: number;
  readonly max: number;
}

/**
 * 逆算の性格クラス(engine.NatureClass の写し。engine/reverse.go)。下降補正は探索しない
 * (ADR-0010 §R1: `NatureClassMinus` は廃止された)。
 */
export type NatureClass = "neutral" | "plus";

/** 逆算候補1件(ADR-0010 §R3・§R8)。 */
export interface ReverseCandidate {
  readonly natureClass: NatureClass;
  readonly nature: Nature;
  readonly itemId: string;
  readonly ranges: readonly SPRange[];
  readonly spCount: number;
  readonly exact: boolean;
  readonly mismatch: number;
  readonly support: number;
  readonly minPercent: number;
  readonly maxPercent: number;
}

/** 逆算の結果。 */
export interface ReverseResult {
  readonly side: ReverseSide;
  readonly stat: StatKey;
  readonly assumedHpSp: number;
  readonly exactCount: number;
  readonly candidates: readonly ReverseCandidate[];
}

/** 境界のエラー封筒(ADR-0011 §5)。code は engine/wasmapi の sentinel を写した安定した文字列。 */
export interface EngineError {
  readonly code: string;
  readonly message: string;
}

/** 計算の成否(判別 union)。ok の側だけ value、not ok の側だけ error を持つ。 */
export type EngineResult<T> =
  { readonly ok: true; readonly value: T } | { readonly ok: false; readonly error: EngineError };

/**
 * 画面が依存する計算の差し替え口(ADR-0300 §2)。WASM 実装(createWasmEngine)と、
 * 将来の API 実装(P4-5)が同じ形で後ろに入る。画面は実装を知らない。
 */
export interface CalcEngine {
  calc(request: CalcRequest): Promise<EngineResult<CalcResult>>;
  calcBulk(request: BulkRequest): Promise<EngineResult<BulkResult>>;
  calcReverse(request: ReverseRequest): Promise<EngineResult<ReverseResult>>;
}
