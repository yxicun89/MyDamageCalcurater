// マスタデータの型(ADR-0300 §3)。種族・技・持ち物・特性・相性表は WASM 境界の DTO(engine/types.ts)と
// 同じ形にする(解決済みの実体をそのまま engine に渡せる)。種族が覚える技の一覧(learnset)だけは
// 画面のための追加フィールドで、engine には渡さない(domain/requests.ts の toEngineSpecies が落とす)。

import type { Ability, Item, Move, Species, StatKey, TypeChart } from "../engine/types";

/** マスタの種族。learnset は画面のための追加フィールド。 */
export interface MasterSpecies extends Species {
  readonly learnset: readonly string[];
}

/**
 * マスタの性格(API の Nature と同じ形。ADR-0301 §2)。上昇補正(plus)・下降補正(minus)を受ける
 * ステータス。無補正は plus・minus とも null(HP を指すことはない)。
 */
export interface MasterNature {
  readonly id: string;
  readonly nameJa: string;
  readonly plus: StatKey | null;
  readonly minus: StatKey | null;
}

/**
 * マスタの取得口が備える機能(P4-16、ADR-0304 §追記)。オフライン(架空の例データ)は全部そろうが、
 * オンライン(pokedex-svc の公開 API)は公開 API の制約でいくつか欠ける。画面は「オンラインかどうか」ではなく
 * 「この機能が使えるか」で分岐する(モードの名前を画面に持ち込まない)。
 */
export interface MasterCapabilities {
  /**
   * MasterData.species が全件そろっているか。false なら一覧(ドロップダウン)を出してはいけない
   * (`searchSpecies` の `limit` 上限200 < 実データ349件。ADR-0304 §1)。MasterSpeciesSearch で都度引く。
   */
  readonly speciesList: boolean;
  /**
   * MasterData.moves が使えるか。false なら技を選べない(`getSpecies.learnset` は技の ID 配列だけを返し、
   * ID から技の実体を引く公開 API が無い。ADR-0304 §3。データ/API レーンの対応待ち)。
   */
  readonly moves: boolean;
  /**
   * 持ち物・特性が効果データ(ItemEffect / AbilityEffect)を持つか。false なら効果で選ぶ機能
   * (持ち物の候補比較・逆算の持ち物候補)を出してはいけない(公開 API の Item・Ability は id と nameJa だけ)。
   */
  readonly effects: boolean;
}

/** 画面が使うマスタ一式(ADR-0300 §3)。 */
export interface MasterData {
  readonly species: readonly MasterSpecies[];
  readonly moves: readonly Move[];
  readonly items: readonly Item[];
  readonly abilities: readonly Ability[];
  readonly natures: readonly MasterNature[];
  readonly typeChart: TypeChart;
  /**
   * 使える機能(P4-16)。**省略はオフライン相当の「全部使える」**(FULL_MASTER_CAPABILITIES)。
   * 既存の MasterData の作り手(exampleMasterSource・各テストの fixture)を変えずに済むよう省略可にしてある。
   * 読むときは capabilities.ts の masterCapabilities() を通し、既定の補完を1か所にまとめる。
   */
  readonly capabilities?: MasterCapabilities;
}

/**
 * マスタの取得口。オフラインは架空の例データ(exampleMasterSource)、オンラインは pokedex-svc の
 * 公開 API(onlineSource.ts)。load() は全件そろう部分だけを返し、そろわない部分(オンラインの種族)は
 * MasterSpeciesSearch で都度引く(ADR-0304 §4)。
 */
export interface MasterSource {
  load(): Promise<MasterData>;
}

/**
 * 種族の検索結果の1件(公開 API の SpeciesSummary 相当。P4-16)。一覧に出すのに要る分だけを持ち、
 * 種族値・特性・learnset は持たない(選んだあとに MasterSpeciesSearch.resolveSpecies で引く)。
 * MasterSpecies はこの形を構造的に満たすので、オフラインの種族をそのまま候補として渡せる。
 */
export interface MasterSpeciesSummary {
  readonly key: string;
  readonly dexNo: number;
  readonly form: number;
  readonly nameJa: string;
  readonly types: readonly string[];
}

/**
 * resolveSpecies の結果(P4-16)。種族の実体と、その種族の特性の実体。
 * 特性は「全件の一覧を引く公開 API が無い」ため種族ごとに付いてきて、呼び出し側が MasterData.abilities に
 * 足していく(defaultAbility が species.abilities の ID を引けるようにするため)。
 */
export interface MasterSpeciesResolution {
  readonly species: MasterSpecies;
  readonly abilities: readonly Ability[];
}

/**
 * 種族を検索して都度引く口(P4-16、ADR-0304 §1・§4)。capabilities.speciesList が false のマスタが持つ。
 * 実装は取得した結果を覚えない(記憶は呼び出し側の責務)。
 */
export interface MasterSpeciesSearch {
  /**
   * 日本語名の前方一致検索。空白だけ・空の query は「全件」ではなく**空配列**(fetch もしない)。
   * 返す件数の上限は実装が決める(onlineSource.ts の SPECIES_SEARCH_LIMIT)。
   * 通信・応答の失敗は reject する(空配列で握りつぶさない)。
   */
  searchSpecies(query: string, signal?: AbortSignal): Promise<readonly MasterSpeciesSummary[]>;
  /** 選ばれた種族の実体(種族値・特性・learnset)を引く。見つからない・通信の失敗は reject する。 */
  resolveSpecies(key: string, signal?: AbortSignal): Promise<MasterSpeciesResolution>;
}

/** 検索に対応したマスタの取得口(オンライン。P4-16)。 */
export interface SearchableMasterSource extends MasterSource {
  readonly search: MasterSpeciesSearch;
}

/** 計算モードごとのマスタの取得口(App.tsx。ADR-0301 §4 の「オンラインは API からマスタを読む」)。 */
export interface MasterSources {
  readonly offline: MasterSource;
  readonly online: MasterSource;
}
