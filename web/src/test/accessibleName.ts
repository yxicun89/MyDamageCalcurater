// テスト専用: 「見えるラベル」と accessible name を DOM から読む小さな道具(issue #304)。
// 正は docs/design.md「入力のラベル」と WCAG 2.2 SC 3.3.2(ラベル又は説明)・SC 2.5.3(見出しどおりの名前)。
//
// dom-accessibility-api のような計算器には頼らず、この画面群が実際に使う範囲
// (aria-labelledby / aria-label / label 要素)だけを自前で読む。実装の写しを持たず、
// 仕様の側から確かめるため(コーディング規約 §2 の「独立した検証」。cssRules.ts と同じ作法)。

/** 要素の見える文字(空白をつぶした textContent)。 */
export function visibleTextOf(element: Element): string {
  return element.textContent.replace(/\s+/g, " ").trim();
}

/** 自分か祖先が hidden / aria-hidden="true" なら「画面に出ていない」と見なす。 */
function hiddenFromScreen(element: Element): boolean {
  for (let node: Element | null = element; node !== null; node = node.parentElement) {
    if (node instanceof HTMLElement && node.hidden) {
      return true;
    }
    if (node.getAttribute("aria-hidden") === "true") {
      return true;
    }
  }
  return false;
}

function escapeId(id: string): string {
  // jsdom は CSS.escape を持つが、無い環境でも壊れないように素の属性セレクタへ落とす。
  return typeof CSS !== "undefined" && typeof CSS.escape === "function"
    ? CSS.escape(id)
    : id.replace(/["\\]/g, "\\$&");
}

function elementById(root: Document, id: string): HTMLElement | null {
  return root.querySelector<HTMLElement>(`#${escapeId(id)}`);
}

/**
 * 欄に結び付いた、画面に出ているラベル要素(`label[for]` か、欄を包む `label`)。
 * 見つからない・画面に出ていないときは null。
 */
export function labelElementOf(control: HTMLElement): HTMLLabelElement | null {
  const id = control.getAttribute("id");
  if (id !== null && id !== "") {
    const forLabel = control.ownerDocument.querySelector<HTMLLabelElement>(`label[for="${escapeId(id)}"]`);
    if (forLabel !== null && !hiddenFromScreen(forLabel)) {
      return forLabel;
    }
  }
  const wrapping = control.closest("label");
  return wrapping !== null && !hiddenFromScreen(wrapping) ? wrapping : null;
}

/** 欄の「見えるラベル」の文字。結び付いた label が無ければ null。 */
export function visibleLabelOf(control: HTMLElement): string | null {
  const label = labelElementOf(control);
  return label === null ? null : visibleTextOf(label);
}

/** aria-labelledby が指す要素の文字を、指された順に空白でつないだもの。属性が無ければ null。 */
function labelledByTextOf(control: HTMLElement): string | null {
  const ids = (control.getAttribute("aria-labelledby") ?? "").trim();
  if (ids === "") {
    return null;
  }
  const texts = ids
    .split(/\s+/)
    .map((id) => elementById(control.ownerDocument, id))
    .map((element) => (element === null ? "" : visibleTextOf(element)))
    .filter((text) => text !== "");
  return texts.length === 0 ? null : texts.join(" ");
}

/** 名前を中身の文字から取る要素(ボタン・見出しなど)。領域や欄は含めない。 */
const NAME_FROM_CONTENT = new Set(["BUTTON", "A", "SUMMARY", "LEGEND", "OPTION", "H1", "H2", "H3", "H4"]);

/**
 * 欄・領域の accessible name
 * (aria-labelledby → aria-label → fieldset の legend / label 要素 → 中身の文字 の順)。
 * ARIA の完全な計算ではなく、この画面群が使う範囲だけ。名前が無いときは空文字。
 */
export function accessibleNameOf(control: HTMLElement): string {
  const labelledBy = labelledByTextOf(control);
  if (labelledBy !== null) {
    return labelledBy;
  }
  const ariaLabel = (control.getAttribute("aria-label") ?? "").trim();
  if (ariaLabel !== "") {
    return ariaLabel;
  }
  if (control.tagName === "FIELDSET") {
    const legend = control.querySelector("legend");
    return legend === null ? "" : visibleTextOf(legend);
  }
  const label = visibleLabelOf(control);
  if (label !== null && label !== "") {
    return label;
  }
  return NAME_FROM_CONTENT.has(control.tagName) ? visibleTextOf(control) : "";
}

/** aria-describedby が指す要素の文字を、指された順に並べたもの(説明の文の確認用)。 */
export function describedByTextsOf(control: HTMLElement): string[] {
  const ids = (control.getAttribute("aria-describedby") ?? "").trim();
  if (ids === "") {
    return [];
  }
  return ids
    .split(/\s+/)
    .map((id) => elementById(control.ownerDocument, id))
    .filter((element): element is HTMLElement => element !== null && !hiddenFromScreen(element))
    .map((element) => visibleTextOf(element));
}
