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

/** 画面が使うマスタ一式(ADR-0300 §3)。 */
export interface MasterData {
  readonly species: readonly MasterSpecies[];
  readonly moves: readonly Move[];
  readonly items: readonly Item[];
  readonly abilities: readonly Ability[];
  readonly natures: readonly MasterNature[];
  readonly typeChart: TypeChart;
}

/**
 * マスタの取得口。いまの実装は架空の例データ(exampleMasterSource)。
 * pokedex-svc ができたら API 実装に差し替える(P4-5 以降、ADR-0300 §3)。
 */
export interface MasterSource {
  load(): Promise<MasterData>;
}
