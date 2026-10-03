// Showdown 形式のテキストと TeamMember の相互変換(P5-5。仕様の正は ADR-0310)。
// 純粋関数だけ(fetch・DOM・時刻・乱数を持たない)。名前→ID はマスタから引き、名前リストは持たない。

import type { components } from "../api/openapi.gen";
import type { MasterData } from "../master/types";

type TeamMember = components["schemas"]["TeamMember"];
type StatBlock = components["schemas"]["StatBlock"];
type PokeType = components["schemas"]["PokeType"];

/** 名前引きに要る分だけの読み取り専用のマスタ。MasterData はそのまま渡せる。nameEn は任意。 */
export interface ShowdownNamed {
  readonly nameJa: string;
  readonly nameEn?: string;
}
export interface ShowdownMaster {
  readonly species: readonly (ShowdownNamed & { readonly key: string })[];
  readonly moves: readonly (ShowdownNamed & { readonly id: string })[];
  readonly items: readonly (ShowdownNamed & { readonly id: string })[];
  readonly abilities: readonly (ShowdownNamed & { readonly id: string })[];
  readonly natures: readonly (ShowdownNamed & { readonly id: string })[];
}

/** MasterData を ShowdownMaster として渡せることをコンパイル時に保つ(そのまま渡してもよい)。 */
export const toShowdownMaster = (m: MasterData): ShowdownMaster => m;

export type ShowdownIssueCode =
  | "empty_input"
  | "too_many_members"
  | "malformed_line"
  | "unresolved_name"
  | "missing_nature"
  | "sp_out_of_range"
  | "sp_total_exceeded"
  | "ev_like_value"
  | "level_not_50"
  | "iv_not_31"
  | "too_many_moves"
  | "duplicate_move"
  | "nickname_too_long"
  | "missing_name"
  | "ambiguous_name"
  | "duplicate_line"
  | "input_too_large";

export interface ShowdownIssue {
  readonly severity: "error" | "warning";
  readonly code: ShowdownIssueCode;
  /** 何体目か(0 始まり)。全体に関わるときは null。 */
  readonly memberIndex: number | null;
  /** どの項目か(species / item / ability / move / teraType / nature / sp / level / ivs / nickname / line)。全体は null。 */
  readonly field: string | null;
  readonly value?: string;
}

export interface ParseResult {
  readonly members: TeamMember[];
  readonly issues: ShowdownIssue[];
}
export interface ExportResult {
  readonly text: string;
  readonly issues: ShowdownIssue[];
}

const MAX_MEMBERS = 6;
const MAX_MOVES = 4;
const SP_MAX = 32;
const SP_TOTAL_MAX = 66;
const NICKNAME_MAX = 24;
const STAT_ORDER = ["hp", "atk", "def", "spa", "spd", "spe"] as const;
const STAT_LABEL: Record<(typeof STAT_ORDER)[number], string> = {
  hp: "HP",
  atk: "Atk",
  def: "Def",
  spa: "SpA",
  spd: "SpD",
  spe: "Spe",
};

// API の PokeType 列挙(マスタの名前リストではなくスキーマの値)。Record で網羅をコンパイル時に保つ。
const POKE_TYPES: Record<PokeType, true> = {
  normal: true,
  fire: true,
  water: true,
  electric: true,
  grass: true,
  ice: true,
  fighting: true,
  poison: true,
  ground: true,
  flying: true,
  psychic: true,
  bug: true,
  rock: true,
  ghost: true,
  dragon: true,
  dark: true,
  steel: true,
  fairy: true,
};

function norm(s: string): string {
  return s.trim().toLowerCase();
}

interface Lookup {
  readonly ids: Map<string, string>;
  /** 同じ名前が複数の ID に付いているもの(先勝ちで解決し、ambiguous_name を出す)。 */
  readonly ambiguous: Set<string>;
}

function buildLookup<T extends ShowdownNamed>(list: readonly T[], idOf: (t: T) => string): Lookup {
  const ids = new Map<string, string>();
  const ambiguous = new Set<string>();
  for (const e of list) {
    const id = idOf(e);
    for (const name of [e.nameEn, e.nameJa]) {
      if (name === undefined) continue;
      const k = norm(name);
      if (k === "") continue;
      const prev = ids.get(k);
      if (prev === undefined) ids.set(k, id);
      else if (prev !== id) ambiguous.add(k);
    }
  }
  return { ids, ambiguous };
}

function buildNameOf<T extends ShowdownNamed>(
  list: readonly T[],
  idOf: (t: T) => string,
): Map<string, string> {
  const m = new Map<string, string>();
  for (const e of list) {
    const id = idOf(e);
    const name = e.nameEn !== undefined && e.nameEn.trim() !== "" ? e.nameEn.trim() : e.nameJa;
    if (!m.has(id)) m.set(id, name);
  }
  return m;
}

const MAX_TEXT_LENGTH = 100_000;
const MAX_LINE_LENGTH = 1000;
const MAX_CANDIDATES = 8;

interface HeaderParts {
  readonly nick: string;
  readonly species: string;
  readonly item: string | null;
}

/**
 * 1行目 `Nick (Species) (F) @ Item` を分ける。ニックネームは `@`・括弧を含みえ、名前も括弧を含みうるので、
 * 候補(`@` の位置・括弧の位置)を作り、種族が引けて持ち物も引ける候補を優先して選ぶ。
 */
function splitHeader(header: string, species: Lookup, items: Lookup): HeaderParts {
  const ats: (number | null)[] = [];
  let from = header.length;
  while (ats.length < MAX_CANDIDATES) {
    const i = header.lastIndexOf("@", from - 1);
    if (i < 0) break;
    ats.push(i);
    from = i;
  }
  ats.push(null);
  const all: HeaderParts[] = [];
  for (const at of ats) {
    const leftRaw = at === null ? header : header.slice(0, at);
    const item = at === null ? null : header.slice(at + 1).trim();
    const left = leftRaw
      .trim()
      .replace(/\s*\((?:M|F)\)$/i, "")
      .trim();
    all.push({ nick: "", species: left, item });
    if (!left.endsWith(")")) continue;
    let n = 0;
    for (let i = left.lastIndexOf("("); i >= 0 && n < MAX_CANDIDATES; i = left.lastIndexOf("(", i - 1)) {
      all.push({ nick: left.slice(0, i).trim(), species: left.slice(i + 1, -1).trim(), item });
      n++;
    }
  }
  const speciesOk = (h: HeaderParts): boolean => species.ids.has(norm(h.species));
  const itemOk = (h: HeaderParts): boolean => h.item === null || h.item === "" || items.ids.has(norm(h.item));
  const simple = all[0];
  return (
    all.find((h) => speciesOk(h) && itemOk(h)) ??
    all.find(speciesOk) ??
    all.find((h) => h.nick !== "") ??
    simple ?? { nick: "", species: header, item: null }
  );
}

const LABEL_RE = /^(ability|level|tera type|evs|ivs)\s*:\s*(.*)$/i;
const STAT_PART_RE = /^(-?\d+(?:\.\d+)?)\s+(hp|atk|def|spa|spd|spe)$/i;

export function parseShowdownTeam(text: string, master: ShowdownMaster): ParseResult {
  const issues: ShowdownIssue[] = [];
  const members: TeamMember[] = [];

  if (text.length > MAX_TEXT_LENGTH) {
    issues.push({
      severity: "error",
      code: "input_too_large",
      memberIndex: null,
      field: null,
      value: String(text.length),
    });
    return { members, issues };
  }

  const blocks: string[][] = [];
  let cur: string[] = [];
  for (const raw of text.split(/\r\n|\r|\n/)) {
    const line = raw.trim();
    if (line === "") {
      if (cur.length > 0) blocks.push(cur);
      cur = [];
    } else {
      cur.push(line);
    }
  }
  if (cur.length > 0) blocks.push(cur);

  if (blocks.length === 0) {
    issues.push({ severity: "error", code: "empty_input", memberIndex: null, field: null });
    return { members, issues };
  }

  const lookups = {
    species: buildLookup(master.species, (s) => s.key),
    moves: buildLookup(master.moves, (s) => s.id),
    items: buildLookup(master.items, (s) => s.id),
    abilities: buildLookup(master.abilities, (s) => s.id),
    natures: buildLookup(master.natures, (s) => s.id),
  };

  if (blocks.length > MAX_MEMBERS) {
    issues.push({
      severity: "error",
      code: "too_many_members",
      memberIndex: null,
      field: null,
      value: String(blocks.length),
    });
  }

  blocks.slice(0, MAX_MEMBERS).forEach((lines, index) => {
    const add = (
      severity: ShowdownIssue["severity"],
      code: ShowdownIssueCode,
      field: string,
      value?: string,
    ): void => {
      issues.push({
        severity,
        code,
        memberIndex: index,
        field,
        ...(value !== undefined ? { value } : {}),
      });
    };
    let drop = false;

    // 1行目: Nick (Species) (F) @ Item
    const header = lines[0] ?? "";
    if (header.length > MAX_LINE_LENGTH) {
      add("error", "malformed_line", "line", header.slice(0, 100));
      return;
    }
    const resolve = (lk: Lookup, name: string, field: string): string | undefined => {
      const k = norm(name);
      const id = lk.ids.get(k);
      if (id !== undefined && lk.ambiguous.has(k)) add("warning", "ambiguous_name", field, name);
      return id;
    };
    const parts = splitHeader(header, lookups.species, lookups.items);
    const nick = parts.nick;
    const speciesName = parts.species;

    const speciesKey = resolve(lookups.species, speciesName, "species");
    if (speciesKey === undefined) {
      add("error", "unresolved_name", "species", speciesName);
      drop = true;
    }

    let itemId: string | null = null;
    if (parts.item !== null && parts.item !== "") {
      const id = resolve(lookups.items, parts.item, "item");
      if (id === undefined) add("warning", "unresolved_name", "item", parts.item);
      else itemId = id;
    }

    let nickname: string | null = null;
    if (nick !== "") {
      if (Array.from(nick).length > NICKNAME_MAX) add("warning", "nickname_too_long", "nickname", nick);
      else nickname = nick;
    }

    let abilityId: string | null = null;
    let teraType: PokeType | null = null;
    let natureId: string | null = null;
    let natureSeen = false;
    let sp: StatBlock = { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 };
    const moveLines: string[] = [];
    const seenLabels = new Set<string>();

    for (const line of lines.slice(1)) {
      if (line.length > MAX_LINE_LENGTH) {
        add("warning", "malformed_line", "line", line.slice(0, 100));
        continue;
      }
      if (line.startsWith("-")) {
        const name = line.slice(1).trim();
        if (name === "") add("warning", "malformed_line", "line", line);
        else moveLines.push(name);
        continue;
      }
      const label = LABEL_RE.exec(line);
      if (label) {
        const key = (label[1] ?? "").toLowerCase();
        const value = (label[2] ?? "").trim();
        if (seenLabels.has(key)) {
          add("warning", "duplicate_line", key === "tera type" ? "teraType" : key, line);
        } else if (value === "") {
          add("warning", "malformed_line", "line", line);
        } else if (key === "ability") {
          seenLabels.add(key);
          const id = resolve(lookups.abilities, value, "ability");
          if (id === undefined) add("warning", "unresolved_name", "ability", value);
          else abilityId = id;
        } else if (key === "tera type") {
          seenLabels.add(key);
          const t = value.toLowerCase();
          if (Object.hasOwn(POKE_TYPES, t)) teraType = t as PokeType;
          else add("warning", "unresolved_name", "teraType", value);
        } else if (key === "level") {
          seenLabels.add(key);
          if (!/^\d+$/.test(value) || Number(value) !== 50) add("warning", "level_not_50", "level", value);
        } else if (key === "ivs") {
          seenLabels.add(key);
          const bad = parseStatParts(
            value,
            (v) => v !== 31,
            () => {
              add("warning", "malformed_line", "line", line);
            },
          );
          if (bad) add("warning", "iv_not_31", "ivs", value);
        } else {
          // evs(2行目以降は duplicate_line。先の行が勝つ)
          seenLabels.add(key);
          const parsed = parseEvs(value, () => {
            add("warning", "malformed_line", "line", line);
          });
          sp = parsed.sp;
          if (parsed.outOfRange.length > 0) {
            add("error", "sp_out_of_range", "sp", parsed.outOfRange.join(", "));
            drop = true;
          }
          if (parsed.evLike) add("warning", "ev_like_value", "sp", value);
          if (parsed.outOfRange.length === 0) {
            const total = STAT_ORDER.reduce((a, k) => a + sp[k], 0);
            if (total > SP_TOTAL_MAX) {
              add("error", "sp_total_exceeded", "sp", String(total));
              drop = true;
            }
          }
        }
        continue;
      }
      const natName = line.toLowerCase().endsWith("nature") ? line.slice(0, -6) : "";
      if (natName !== "" && natName.trimEnd() !== natName && natName.trim() !== "") {
        if (natureSeen) {
          add("warning", "duplicate_line", "nature", line);
          continue;
        }
        natureSeen = true;
        const name = natName.trim();
        const id = resolve(lookups.natures, name, "nature");
        if (id === undefined) {
          add("error", "unresolved_name", "nature", name);
          drop = true;
        } else natureId = id;
        continue;
      }
      add("warning", "malformed_line", "line", line);
    }

    if (!natureSeen) {
      add("error", "missing_nature", "nature");
      drop = true;
    }

    if (moveLines.length > MAX_MOVES) {
      add("warning", "too_many_moves", "move", String(moveLines.length));
    }
    const moveIds: string[] = [];
    for (const name of moveLines.slice(0, MAX_MOVES)) {
      const id = resolve(lookups.moves, name, "move");
      if (id === undefined) add("warning", "unresolved_name", "move", name);
      else if (moveIds.includes(id)) add("warning", "duplicate_move", "move", name);
      else moveIds.push(id);
    }

    if (drop || speciesKey === undefined || natureId === null) return;
    members.push({ speciesKey, nickname, moveIds, itemId, abilityId, natureId, sp, teraType });
  });

  return { members, issues };
}

/** "31 HP / 31 Atk" を分解する。pred を満たす値があれば true。解釈できない部分は onMalformed。 */
function parseStatParts(value: string, pred: (v: number) => boolean, onMalformed: () => void): boolean {
  let hit = false;
  for (const part of value.split("/")) {
    const m = STAT_PART_RE.exec(part.trim());
    if (!m) {
      onMalformed();
      continue;
    }
    if (pred(Number(m[1]))) hit = true;
  }
  return hit;
}

function parseEvs(
  value: string,
  onMalformed: () => void,
): { sp: StatBlock; outOfRange: string[]; evLike: boolean } {
  const sp: StatBlock = { hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0 };
  const outOfRange: string[] = [];
  let evLike = false;
  for (const part of value.split("/")) {
    const m = STAT_PART_RE.exec(part.trim());
    if (!m) {
      onMalformed();
      continue;
    }
    const n = Number(m[1]);
    const stat = (m[2] ?? "").toLowerCase() as keyof StatBlock;
    if (!Number.isInteger(n) || n < 0 || n > SP_MAX) {
      outOfRange.push(part.trim());
      if (n > SP_MAX) evLike = true;
    } else {
      sp[stat] = n;
    }
  }
  return { sp, outOfRange, evLike };
}

export function exportShowdownTeam(members: readonly TeamMember[], master: ShowdownMaster): ExportResult {
  const issues: ShowdownIssue[] = [];
  const lk = {
    species: buildLookup(master.species, (s) => s.key),
    moves: buildLookup(master.moves, (s) => s.id),
    items: buildLookup(master.items, (s) => s.id),
    abilities: buildLookup(master.abilities, (s) => s.id),
    natures: buildLookup(master.natures, (s) => s.id),
  };
  const species = buildNameOf(master.species, (s) => s.key);
  const moves = buildNameOf(master.moves, (s) => s.id);
  const items = buildNameOf(master.items, (s) => s.id);
  const abilities = buildNameOf(master.abilities, (s) => s.id);
  const natures = buildNameOf(master.natures, (s) => s.id);

  const blocks: string[] = [];
  members.forEach((m, index) => {
    const missing = (severity: ShowdownIssue["severity"], field: string, value: string): void => {
      issues.push({ severity, code: "missing_name", memberIndex: index, field, value });
    };
    const check = (l: Lookup, id: string, name: string, field: string): void => {
      const k = norm(name);
      if (l.ids.get(k) !== id || l.ambiguous.has(k)) {
        issues.push({ severity: "warning", code: "ambiguous_name", memberIndex: index, field, value: name });
      }
    };
    const speciesName = species.get(m.speciesKey);
    const natureName = natures.get(m.natureId);
    if (speciesName === undefined) missing("error", "species", m.speciesKey);
    if (natureName === undefined) missing("error", "nature", m.natureId);
    if (speciesName === undefined || natureName === undefined) return;

    const lines: string[] = [];
    let nick = (m.nickname ?? "").replace(/[\r\n]+/g, " ").trim();
    if (Array.from(nick).length > NICKNAME_MAX) {
      issues.push({
        severity: "warning",
        code: "nickname_too_long",
        memberIndex: index,
        field: "nickname",
        value: nick,
      });
      nick = "";
    }
    check(lk.species, m.speciesKey, speciesName, "species");
    check(lk.natures, m.natureId, natureName, "nature");
    let head = nick !== "" ? `${nick} (${speciesName})` : speciesName;
    if (m.itemId) {
      const name = items.get(m.itemId);
      if (name === undefined) missing("warning", "item", m.itemId);
      else {
        check(lk.items, m.itemId, name, "item");
        head += ` @ ${name}`;
      }
    }
    lines.push(head);
    if (m.abilityId) {
      const name = abilities.get(m.abilityId);
      if (name === undefined) missing("warning", "ability", m.abilityId);
      else {
        check(lk.abilities, m.abilityId, name, "ability");
        lines.push(`Ability: ${name}`);
      }
    }
    if (m.teraType) lines.push(`Tera Type: ${m.teraType.charAt(0).toUpperCase()}${m.teraType.slice(1)}`);
    const evs = STAT_ORDER.filter((k) => m.sp[k] > 0).map((k) => `${m.sp[k]} ${STAT_LABEL[k]}`);
    if (evs.length > 0) lines.push(`EVs: ${evs.join(" / ")}`);
    lines.push(`${natureName} Nature`);
    for (const id of m.moveIds) {
      const name = moves.get(id);
      if (name === undefined) missing("warning", "move", id);
      else {
        check(lk.moves, id, name, "move");
        lines.push(`- ${name}`);
      }
    }
    blocks.push(lines.join("\n"));
  });

  return { text: blocks.join("\n\n"), issues };
}
