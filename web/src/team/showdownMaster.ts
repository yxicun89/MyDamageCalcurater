// P5-5e(ADR-0321 §1): Showdown の取り込み・書き出しに使う「名前引き用のマスタ」を作る。
// 実際のマスタは種族・特性・技の全件を持たない(capabilities.speciesList / moves が false)ので、
// MasterSpeciesSearch で必要な種族だけ都度引いて足す。元のマスタは書き換えない。
// 名前は日本語名のみ(MasterData に英語名が無い)。検索・解決の失敗は握りつぶし、その種族は足さない
// (後段の parse が unresolved_name を出す)。

import type { components } from "../api/openapi.gen";
import { masterCapabilities } from "../master/capabilities";
import type { MasterData, MasterSpeciesResolution, MasterSpeciesSearch } from "../master/types";

type TeamMember = components["schemas"]["TeamMember"];

/** 1メンバー(1行目)あたりの種族名の候補数の上限(showdownFormat の MAX_CANDIDATES と同じ)。 */
const MAX_CANDIDATES_PER_MEMBER = 8;
/** 取り込めるメンバー数の上限(showdownFormat の MAX_MEMBERS と同じ)。 */
const MAX_MEMBERS = 6;
/** これを超える入力は parse が input_too_large にするので、検索しない(showdownFormat の MAX_TEXT_LENGTH と同じ)。 */
const MAX_TEXT_LENGTH = 100_000;
/** 括弧(ニックネーム (種族))をたどる深さの上限。 */
const MAX_PAREN_DEPTH = 3;

function normalize(name: string): string {
  return name.trim().toLowerCase();
}

/** 1メンバーの1行目から種族名の候補を作る(行全体・@ の前・括弧の中・括弧の前)。 */
function headerCandidates(header: string): string[] {
  const found: string[] = [];
  const push = (name: string): void => {
    const trimmed = name.trim();
    if (trimmed !== "" && !found.includes(trimmed)) {
      found.push(trimmed);
    }
  };
  push(header);
  const at = header.lastIndexOf("@");
  const left = (at < 0 ? header : header.slice(0, at)).trim().replace(/\s*\((?:M|F)\)$/i, "");
  push(left);
  if (left.endsWith(")")) {
    let depth = 0;
    for (let i = left.lastIndexOf("("); i >= 0 && depth < MAX_PAREN_DEPTH; i = left.lastIndexOf("(", i - 1)) {
      push(left.slice(i + 1, -1));
      push(left.slice(0, i));
      depth += 1;
    }
  }
  return found.slice(0, MAX_CANDIDATES_PER_MEMBER);
}

/** 各メンバー(空行区切り)の1行目から種族名の候補を、重複なしで返す(純粋)。 */
export function candidateSpeciesNames(text: string): string[] {
  const names: string[] = [];
  let atBlockStart = true;
  let blocks = 0;
  for (const raw of text.split(/\r\n|\r|\n/)) {
    const line = raw.trim();
    if (line === "") {
      atBlockStart = true;
      continue;
    }
    if (!atBlockStart) {
      continue;
    }
    atBlockStart = false;
    blocks += 1;
    if (blocks > MAX_MEMBERS) {
      break;
    }
    for (const name of headerCandidates(line)) {
      if (!names.includes(name)) {
        names.push(name);
      }
    }
  }
  return names;
}

/** 解決した種族(と特性・技)を、元を変えずにマスタへ足す。同じ key・id は二重に足さない。 */
function mergeResolutions(master: MasterData, resolutions: readonly MasterSpeciesResolution[]): MasterData {
  if (resolutions.length === 0) {
    return master;
  }
  const species = [...master.species];
  const abilities = [...master.abilities];
  const moves = [...master.moves];
  for (const resolution of resolutions) {
    if (!species.some((s) => s.key === resolution.species.key)) {
      species.push(resolution.species);
    }
    for (const ability of resolution.abilities) {
      if (!abilities.some((a) => a.id === ability.id)) {
        abilities.push(ability);
      }
    }
    for (const move of resolution.moves) {
      if (!moves.some((m) => m.id === move.id)) {
        moves.push(move);
      }
    }
  }
  return { ...master, species, abilities, moves };
}

/** key を解決する。失敗は null(握りつぶす)。 */
async function tryResolve(search: MasterSpeciesSearch, key: string): Promise<MasterSpeciesResolution | null> {
  try {
    return await search.resolveSpecies(key);
  } catch {
    return null;
  }
}

/** 候補の名前を検索し、日本語名が完全一致した種族の key を返す。失敗は空(握りつぶす)。 */
async function exactMatchKeys(search: MasterSpeciesSearch, name: string): Promise<string[]> {
  try {
    const found = await search.searchSpecies(name);
    return found.filter((s) => normalize(s.nameJa) === normalize(name)).map((s) => s.key);
  } catch {
    return [];
  }
}

function compact<T>(values: readonly (T | null)[]): T[] {
  return values.filter((value): value is T => value !== null);
}

/**
 * 取り込み用のマスタ。種族の一覧があるマスタ・検索口が無いときはそのまま返す。
 * 一覧が無いときは、完全一致した種族だけ解決して足す。
 */
export async function resolveMasterForImport(
  text: string,
  master: MasterData,
  masterSearch?: MasterSpeciesSearch,
): Promise<MasterData> {
  if (masterCapabilities(master).speciesList || masterSearch === undefined || text.length > MAX_TEXT_LENGTH) {
    return master;
  }
  const keyLists = await Promise.all(
    candidateSpeciesNames(text).map((name) => exactMatchKeys(masterSearch, name)),
  );
  const keys = [...new Set(keyLists.flat())].filter((key) => !master.species.some((s) => s.key === key));
  const resolutions = await Promise.all(keys.map((key) => tryResolve(masterSearch, key)));
  return mergeResolutions(master, compact(resolutions));
}

/** 書き出し用のマスタ。メンバーの種族でマスタに無いものだけ解決して足す。 */
export async function resolveMasterForExport(
  members: readonly TeamMember[],
  master: MasterData,
  masterSearch?: MasterSpeciesSearch,
): Promise<MasterData> {
  if (masterSearch === undefined) {
    return master;
  }
  const keys = [...new Set(members.map((m) => m.speciesKey))].filter(
    (key) => !master.species.some((s) => s.key === key),
  );
  const resolutions = await Promise.all(keys.map((key) => tryResolve(masterSearch, key)));
  return mergeResolutions(master, compact(resolutions));
}
