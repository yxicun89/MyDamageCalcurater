// P4-16: マスタの取得口が備える機能(MasterCapabilities)の既定と読み出し(ADR-0304 §追記)。
// MasterData.capabilities は省略可(既存の作り手を変えないため)なので、既定の補完はこの1か所だけで行う
// (コーディング規約 §2「単一の正を決める」)。

import type { MasterCapabilities, MasterData, MasterSource, SearchableMasterSource } from "./types";

/**
 * 既定の機能(オフラインの架空の例データ相当)。MasterData.capabilities を省いたマスタはこれとみなす。
 * P4-16 より前に書かれたマスタ(exampleMasterSource・テストの fixture)が今までどおり動くための既定。
 */
export const FULL_MASTER_CAPABILITIES: MasterCapabilities = {
  speciesList: true,
  moves: true,
  effects: true,
};

/** マスタが使える機能(capabilities を省いたマスタは FULL_MASTER_CAPABILITIES)。 */
export function masterCapabilities(master: MasterData): MasterCapabilities {
  return master.capabilities ?? FULL_MASTER_CAPABILITIES;
}

/** マスタの取得口が種族の検索に対応しているか(capabilities.speciesList が false のときに使う)。 */
export function isSearchableMasterSource(source: MasterSource): source is SearchableMasterSource {
  return "search" in source;
}
