// テスト専用: CSS を「規則(セレクタ + 宣言)」と「@規則(@media など + 中の規則)」の木に分ける小さなパーサー。
// デザイントークン(docs/design.md)と tokens.css の同期テストで使う。完全な CSS パーサーではなく、
// tokens.css が使う範囲(コメント・入れ子の @media・宣言)だけを扱う。

export interface CssDeclaration {
  readonly property: string;
  readonly value: string;
  /** `!important` が付いているか(reduced-motion の無効化はこれで確かめる)。 */
  readonly important: boolean;
}

export interface CssRule {
  readonly kind: "rule";
  readonly selector: string;
  readonly declarations: readonly CssDeclaration[];
}

export interface CssAtRule {
  readonly kind: "at-rule";
  readonly prelude: string;
  readonly children: readonly CssNode[];
}

export type CssNode = CssRule | CssAtRule;

function collapseWhitespace(text: string): string {
  return text.replace(/\s+/g, " ").trim();
}

function parseDeclarations(body: string): CssDeclaration[] {
  return body
    .split(";")
    .map((part) => part.trim())
    .filter((part) => part.length > 0)
    .flatMap((part) => {
      const colon = part.indexOf(":");
      if (colon < 0) {
        return [];
      }
      const rawValue = part.slice(colon + 1);
      return [
        {
          property: part.slice(0, colon).trim(),
          value: collapseWhitespace(rawValue.replace(/!important/i, "")),
          important: /!important/i.test(rawValue),
        },
      ];
    });
}

function parseBlockList(source: string): CssNode[] {
  const nodes: CssNode[] = [];
  let index = 0;
  while (index < source.length) {
    const open = source.indexOf("{", index);
    if (open < 0) {
      break;
    }
    const prelude = collapseWhitespace(source.slice(index, open).replace(/^[\s;]+/, ""));
    let depth = 1;
    let cursor = open + 1;
    while (cursor < source.length && depth > 0) {
      const char = source[cursor];
      if (char === "{") {
        depth += 1;
      } else if (char === "}") {
        depth -= 1;
      }
      cursor += 1;
    }
    if (depth !== 0) {
      throw new Error(`CSS の波括弧が閉じていない: ${prelude}`);
    }
    const body = source.slice(open + 1, cursor - 1);
    if (prelude.startsWith("@")) {
      nodes.push({ kind: "at-rule", prelude, children: parseBlockList(body) });
    } else {
      nodes.push({ kind: "rule", selector: prelude, declarations: parseDeclarations(body) });
    }
    index = cursor;
  }
  return nodes;
}

export function parseCss(source: string): CssNode[] {
  return parseBlockList(source.replace(/\/\*[\s\S]*?\*\//g, ""));
}

export function splitSelectors(selector: string): string[] {
  return selector.split(",").map((part) => collapseWhitespace(part));
}

/** 最上位(@規則の外)の規則。 */
export function topLevelRules(nodes: readonly CssNode[]): CssRule[] {
  return nodes.filter((node): node is CssRule => node.kind === "rule");
}

/** prelude が条件に合う @規則の中の規則(入れ子の @規則は辿らない)。 */
export function rulesInAtRules(nodes: readonly CssNode[], matches: (prelude: string) => boolean): CssRule[] {
  return nodes
    .filter((node): node is CssAtRule => node.kind === "at-rule" && matches(node.prelude))
    .flatMap((node) => topLevelRules(node.children));
}

/** 規則群の宣言を1つの対応表にまとめる(後勝ち)。 */
export function declarationMap(rules: readonly CssRule[]): Map<string, string> {
  const map = new Map<string, string>();
  for (const rule of rules) {
    for (const declaration of rule.declarations) {
      map.set(declaration.property, declaration.value);
    }
  }
  return map;
}

/** 規則群の宣言のうち `!important` が付いているかどうかの対応表(後勝ち)。 */
export function importantDeclarationMap(rules: readonly CssRule[]): Map<string, boolean> {
  const map = new Map<string, boolean>();
  for (const rule of rules) {
    for (const declaration of rule.declarations) {
      map.set(declaration.property, declaration.important);
    }
  }
  return map;
}

/**
 * 色の比較用の正規形。#RGB / #RRGGBB / #RRGGBBAA / rgb() / rgba()(カンマ区切り、または空白 + `/` 区切り)
 * を "r,g,b,a"(a は小数)に揃える。解釈できないものは小文字・空白除去だけして返す。
 */
export function normalizeColor(value: string): string {
  const text = value.trim().toLowerCase();
  const hex = /^#([0-9a-f]{3}|[0-9a-f]{6}|[0-9a-f]{8})$/.exec(text);
  if (hex?.[1] !== undefined) {
    let digits = hex[1];
    if (digits.length === 3) {
      digits = digits
        .split("")
        .map((digit) => digit + digit)
        .join("");
    }
    const channel = (offset: number): number => parseInt(digits.slice(offset, offset + 2), 16);
    const alpha = digits.length === 8 ? channel(6) / 255 : 1;
    return formatColor(channel(0), channel(2), channel(4), alpha);
  }
  const functional = /^rgba?\((.*)\)$/.exec(text);
  if (functional?.[1] !== undefined) {
    const parts = functional[1]
      .split(/[\s,/]+/)
      .map((part) => part.trim())
      .filter((part) => part.length > 0);
    if (parts.length === 3 || parts.length === 4) {
      const [r, g, b, a] = parts.map((part) =>
        part.endsWith("%") ? parseFloat(part) / 100 : parseFloat(part),
      );
      if (r !== undefined && g !== undefined && b !== undefined) {
        return formatColor(r, g, b, a ?? 1);
      }
    }
  }
  return text.replace(/\s+/g, "");
}

function formatColor(r: number, g: number, b: number, a: number): string {
  return [r, g, b, Math.round(a * 1000) / 1000].join(",");
}
