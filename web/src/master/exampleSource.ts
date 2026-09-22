// P4-2: 架空の例データを返す MasterSource(ADR-0300 §3)。pokedex-svc ができるまでの既定実装で、
// P4-5 以降で API 実装に差し替える(例データはテストの fixture として残る)。
// タイプ相性表だけは架空にせず、P1-13 のデータ(@typechart)を読む(単一の正。複製しない)。

import typeChartData from "@typechart";
import { exampleAbilities } from "./example/abilities";
import { exampleItems } from "./example/items";
import { exampleMoves } from "./example/moves";
import { exampleNatures } from "./example/natures";
import { exampleSpecies } from "./example/species";
import { typeChartFromData } from "./typeChart";
import type { MasterData, MasterSource } from "./types";

function loadExampleMasterData(): Promise<MasterData> {
  return Promise.resolve({
    species: exampleSpecies,
    moves: exampleMoves,
    items: exampleItems,
    abilities: exampleAbilities,
    natures: exampleNatures,
    typeChart: typeChartFromData(typeChartData),
  });
}

/** 架空の例データを返す MasterSource(既定の実装)。 */
export const exampleMasterSource: MasterSource = {
  load: loadExampleMasterData,
};
