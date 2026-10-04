// F-13(I-web-11、ADR-0337): 用語集(docs/glossary.md)と画面の文言資源(web/src/i18n/*.ts)の一致を保つ検査。
// 用語集の「言い換える語(禁止語)」の表を読み、i18n の文字列(テスト以外の .ts に書いた文字列・テンプレートの固定部分)に
// 禁止語が含まれないことを確かめる。例外は用語集の「例外」の表に書いたものだけ許す。
// 期待値は実装の写しを持たず、用語集そのものから作る(コーディング規約 §2 の独立な検証)。
//
// 文字列は TypeScript の構文木から集める(コメントの「API」「マスタ」などを拾わないため)。
// 画面に出ない文字列(import 先・型・オブジェクトのキー・=== / case の比較相手)は除く。

import { readdirSync, readFileSync } from "node:fs";
import ts from "typescript";
import { describe, expect, test } from "vitest";
import { localPath } from "../test/localPath";

const glossaryPath = localPath("../../../docs/glossary.md", import.meta.url);
const i18nDir = localPath("./", import.meta.url);

// ---- 用語集の読み取り ----

/** `## 見出し` から次の `## ` 見出しまでの本文。見つからなければ例外。 */
function h2Section(markdown: string, heading: string): string {
  const lines = markdown.split("\n");
  const start = lines.findIndex((line) => line.trim() === `## ${heading}`);
  if (start < 0) {
    throw new Error(`glossary.md に「## ${heading}」が無い`);
  }
  const rest = lines.slice(start + 1);
  const end = rest.findIndex((line) => /^#{1,2} /.test(line));
  return (end < 0 ? rest : rest.slice(0, end)).join("\n");
}

/** Markdown の表の本文行(見出し行と区切り行を除く)をセルの配列で返す。 */
function tableRows(section: string): string[][] {
  const rows = section
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line.startsWith("|"))
    .map((line) =>
      line
        .replace(/^\|/, "")
        .replace(/\|$/, "")
        .split("|")
        .map((cell) => cell.trim()),
    );
  return rows.slice(1).filter((cells) => !cells.every((cell) => /^:?-+:?$/.test(cell)));
}

/** セルの先頭のバッククォートで囲んだ語(`語`)。無ければ例外(表の書き方の崩れを黙って通さない)。 */
function codeOf(cell: string | undefined): string {
  const match = /^`([^`]+)`/.exec(cell ?? "");
  if (match?.[1] === undefined) {
    throw new Error(`用語集の表のセルが \`語\` の形ではない: ${cell ?? "(空)"}`);
  }
  return match[1];
}

interface ForbiddenTerm {
  readonly word: string;
  readonly replacement: string;
}

interface GlossaryException {
  readonly file: string;
  readonly key: string;
  readonly word: string;
}

interface Glossary {
  readonly keep: readonly string[];
  readonly forbidden: readonly ForbiddenTerm[];
  readonly exceptions: readonly GlossaryException[];
}

function readGlossary(): Glossary {
  const markdown = readFileSync(glossaryPath, "utf8");
  return {
    keep: tableRows(h2Section(markdown, "変えない語")).map((cells) => codeOf(cells[0])),
    forbidden: tableRows(h2Section(markdown, "言い換える語(禁止語)")).map((cells) => ({
      word: codeOf(cells[0]),
      replacement: cells[1] ?? "",
    })),
    exceptions: tableRows(h2Section(markdown, "例外")).map((cells) => ({
      file: codeOf(cells[0]),
      key: codeOf(cells[1]),
      word: codeOf(cells[2]),
    })),
  };
}

// ---- 文言資源の文字列の収集 ----

interface I18nLiteral {
  /** i18n のファイル名(例 "ja.ts")。 */
  readonly file: string;
  /** 文字列が属する宣言とプロパティのたどり(例 "adjustScreenText.modeLabel.indices")。配列の添字は含めない。 */
  readonly keyPath: string;
  /** 文字列の中身。テンプレートは固定部分を「{}」でつないだもの。 */
  readonly text: string;
}

function propertyNameText(name: ts.PropertyName): string {
  if (ts.isIdentifier(name) || ts.isStringLiteral(name) || ts.isNumericLiteral(name)) {
    return name.text;
  }
  return name.getText();
}

/** node から宣言までさかのぼり、「宣言名.プロパティ名…」を作る。 */
function keyPathOf(node: ts.Node): string {
  const parts: string[] = [];
  // 最上位(SourceFile)まで辿る。宣言(変数・関数)に着いたらそこで止める。
  for (let current: ts.Node = node.parent; !ts.isSourceFile(current); current = current.parent) {
    if (ts.isPropertyAssignment(current)) {
      parts.unshift(propertyNameText(current.name));
    } else if (ts.isVariableDeclaration(current) && ts.isIdentifier(current.name)) {
      parts.unshift(current.name.text);
      break;
    } else if (ts.isFunctionDeclaration(current) && current.name !== undefined) {
      parts.unshift(current.name.text);
      break;
    }
  }
  return parts.join(".");
}

/** 画面に出ない位置の文字列(キー・比較の相手・case)なら true。 */
function isNonDisplayPosition(node: ts.Node): boolean {
  const parent = node.parent;
  if (ts.isPropertyAssignment(parent) && parent.name === node) {
    return true;
  }
  if (
    ts.isBinaryExpression(parent) &&
    (parent.operatorToken.kind === ts.SyntaxKind.EqualsEqualsEqualsToken ||
      parent.operatorToken.kind === ts.SyntaxKind.ExclamationEqualsEqualsToken)
  ) {
    return true;
  }
  return ts.isCaseClause(parent) && parent.expression === node;
}

function collectLiterals(file: string, source: string): I18nLiteral[] {
  const sourceFile = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true);
  const literals: I18nLiteral[] = [];
  const visit = (node: ts.Node): void => {
    if (
      ts.isImportDeclaration(node) ||
      ts.isExportDeclaration(node) ||
      ts.isTypeAliasDeclaration(node) ||
      ts.isInterfaceDeclaration(node) ||
      ts.isTypeNode(node)
    ) {
      return;
    }
    if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) {
      if (!isNonDisplayPosition(node)) {
        literals.push({ file, keyPath: keyPathOf(node), text: node.text });
      }
      return;
    }
    if (ts.isTemplateExpression(node)) {
      const text = [node.head.text, ...node.templateSpans.map((span) => span.literal.text)].join("{}");
      literals.push({ file, keyPath: keyPathOf(node), text });
      // ${} の中の式にも文字列があり得る(三項演算子の「動けます」など)ので、続けて辿る。
      node.templateSpans.forEach((span) => {
        visit(span.expression);
      });
      return;
    }
    ts.forEachChild(node, visit);
  };
  visit(sourceFile);
  return literals;
}

function i18nSourceFiles(): string[] {
  return readdirSync(i18nDir)
    .filter((name) => name.endsWith(".ts") && !name.endsWith(".test.ts"))
    .sort();
}

function allLiterals(): I18nLiteral[] {
  return i18nSourceFiles().flatMap((file) =>
    collectLiterals(file, readFileSync(`${i18nDir}/${file}`, "utf8")),
  );
}

function isExcepted(literal: I18nLiteral, word: string, exceptions: readonly GlossaryException[]): boolean {
  return exceptions.some(
    (exception) =>
      exception.file === literal.file &&
      (exception.word === "*" || exception.word === word) &&
      (exception.key === "*" ||
        literal.keyPath === exception.key ||
        literal.keyPath.startsWith(`${exception.key}.`)),
  );
}

// ---- 検査 ----

describe("用語集(docs/glossary.md)", () => {
  const glossary = readGlossary();

  test("禁止語の表に、F-13 で決めた最低限の語がある(用語集を空にして検査を素通りさせない)", () => {
    const words = glossary.forbidden.map((term) => term.word);
    for (const required of ["観測", "プリセット", "指数", "16n", "API", "WASM", "マスタ"]) {
      expect(words).toContain(required);
    }
  });

  test("禁止語にはそれぞれ、やさしい言い換えが書いてある", () => {
    for (const term of glossary.forbidden) {
      expect(term.replacement, `禁止語「${term.word}」の言い換え`).not.toBe("");
    }
  });

  test("言い換えの語そのものが禁止語を含まない", () => {
    const words = glossary.forbidden.map((term) => term.word);
    for (const term of glossary.forbidden) {
      for (const word of words) {
        expect(
          term.replacement.includes(word),
          `「${term.word}」の言い換え「${term.replacement}」が「${word}」を含む`,
        ).toBe(false);
      }
    }
  });

  test("変えない語と禁止語が重ならない", () => {
    const words = new Set(glossary.forbidden.map((term) => term.word));
    for (const keep of glossary.keep) {
      expect(words.has(keep), `「${keep}」が両方の表にある`).toBe(false);
    }
  });
});

describe("文言資源(web/src/i18n/*.ts)の文字列の収集", () => {
  const literals = allLiterals();

  test("画面の文言を拾えている(収集が空振りしていない)", () => {
    expect(literals).toContainEqual({ file: "ja.ts", keyPath: "resultText.determinedPrefix", text: "確定" });
    expect(literals).toContainEqual({ file: "common.ts", keyPath: "pokemonFieldLabel", text: "ポケモン" });
    expect(
      literals.some(
        (literal) => literal.file === "adjust.ts" && literal.keyPath.startsWith("adjustScreenText."),
      ),
    ).toBe(true);
  });

  test("コメント・import 先・型・オブジェクトのキーは拾わない", () => {
    // ja.ts のコメントには「API」「WASM」が、adjust.ts の型とキーには「16n」がある。これらは画面に出ない。
    expect(literals.some((literal) => literal.text.startsWith("../"))).toBe(false);
    expect(literals.some((literal) => literal.keyPath === "")).toBe(false);
  });
});

describe("禁止語が画面の文言に出てこない(用語集と i18n の一致)", () => {
  const glossary = readGlossary();
  const literals = allLiterals();

  test.each(glossary.forbidden.map((term) => [term.word, term.replacement] as const))(
    "「%s」を使わない(言い換え: %s)",
    (word) => {
      const violations = literals
        .filter((literal) => literal.text.includes(word) && !isExcepted(literal, word, glossary.exceptions))
        .map((literal) => `${literal.file} ${literal.keyPath}: ${literal.text}`);
      expect(violations).toEqual([]);
    },
  );

  test("用語集の例外は、どれも実際の文言に当たる(使われなくなった例外を残さない)", () => {
    for (const exception of glossary.exceptions) {
      const hit = literals.some(
        (literal) =>
          literal.file === exception.file &&
          (exception.key === "*" ||
            literal.keyPath === exception.key ||
            literal.keyPath.startsWith(`${exception.key}.`)) &&
          (exception.word === "*" || literal.text.includes(exception.word)),
      );
      expect(hit, `例外 ${exception.file} ${exception.key} ${exception.word}`).toBe(true);
    }
  });
});

describe("変えない語(対戦の標準用語)が画面の文言に残っている", () => {
  const glossary = readGlossary();
  const literals = allLiterals().filter((literal) => literal.file !== "judge.ts");

  test.each(glossary.keep)("「%s」がどれかの文言にある(言い換えすぎない)", (word) => {
    expect(literals.some((literal) => literal.text.includes(word))).toBe(true);
  });
});
