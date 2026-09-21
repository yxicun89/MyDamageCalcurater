// マスタデータの型(ADR-0300 §3)。種族・技・持ち物・特性・相性表は WASM 境界の DTO(engine/types.ts)と
// 同じ形にする(解決済みの実体をそのまま engine に渡せる)。種族が覚える技の一覧(learnset)だけは
// 画面のための追加フィールドで、engine には渡さない(domain/requests.ts の toEngineSpecies が落とす)。

import type { Ability, Item, Move, Species, TypeChart } from "../engine/types";

/** マスタの種族。learnset は画面のための追加フィールド。 */
export interface MasterSpecies extends Species {
  readonly learnset: readonly string[];
}

/** 画面が使うマスタ一式(ADR-0300 §3)。 */
export interface MasterData {
  readonly species: readonly MasterSpecies[];
  readonly moves: readonly Move[];
  readonly items: readonly Item[];
  readonly abilities: readonly Ability[];
  readonly typeChart: TypeChart;
}

/**
 * マスタの取得口。いまの実装は架空の例データ(exampleMasterSource)。
 * pokedex-svc ができたら API 実装に差し替える(P4-5 以降、ADR-0300 §3)。
 */
export interface MasterSource {
  load(): Promise<MasterData>;
}
