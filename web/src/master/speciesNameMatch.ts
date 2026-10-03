// issue 515・ADR-0324: 種族の名前検索の規則。オンライン(pokedex-svc の GET /api/pokedex/species?q=)と同じ結果にする。
//   - nameJa が q で始まる、または
//   - メガ種族で、nameJa が「メガ + q」で始まる(メガを除いた基本種名でも、基本種とメガの両方が当たる)。
// 並びは図鑑番号・フォーム番号の昇順(基本種が先)。名前の表は持たない(接頭辞 1 語の命名規則だけ)。

import type { MasterSpecies, MasterSpeciesSummary } from "./types";

/**
 * メガ種族の日本語名の接頭辞。importer が生成する名前(services/internal/master の MegaNamePrefix)と
 * pokedex-svc の検索の規則と同じ語にそろえる。
 */
export const MEGA_NAME_PREFIX = "メガ";

type NameBearing = Pick<MasterSpecies, "nameJa" | "isMega">;

/** q(空白除去済み)に種族が当たるか。空の q は全件。 */
export function matchesSpeciesName(species: NameBearing, query: string): boolean {
  if (species.nameJa.startsWith(query)) {
    return true;
  }
  return species.isMega === true && species.nameJa.startsWith(MEGA_NAME_PREFIX + query);
}

/** 規則に当たる種族を、図鑑番号・フォーム番号の昇順で返す(オフラインの検索の本体)。 */
export function searchSpeciesByName<T extends NameBearing & Pick<MasterSpeciesSummary, "dexNo" | "form">>(
  species: readonly T[],
  query: string,
): T[] {
  return species
    .filter((candidate) => matchesSpeciesName(candidate, query))
    .sort((a, b) => a.dexNo - b.dexNo || a.form - b.form);
}
