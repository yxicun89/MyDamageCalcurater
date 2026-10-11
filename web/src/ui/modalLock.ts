// Sheet(モーダル)が開いている間の共通の処理: body のスクロールを止め、背景の兄弟要素に inert を付ける。
// シートが重なっても壊れないよう、回数で数える(最後の 1 つが閉じたときに元へ戻す)。
// 一番上のシートだけが Esc に反応できるよう、開いた順の積み(stack)も持つ。

let scrollLocks = 0;
let savedOverflow = "";
const inertCounts = new Map<Element, number>();
const stack: symbol[] = [];

/** モーダルを開いた合図。`modal` 以外の body 直下の要素を inert にする。返り値で元に戻す。 */
export function lockModal(modal: Element): { readonly id: symbol; readonly unlock: () => void } {
  const id = Symbol("modal");
  stack.push(id);
  if (scrollLocks === 0) {
    savedOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
  }
  scrollLocks += 1;
  const marked: Element[] = [];
  for (const child of document.body.children) {
    if (child === modal || child instanceof HTMLScriptElement) {
      continue;
    }
    const count = inertCounts.get(child) ?? 0;
    if (count === 0) {
      child.setAttribute("inert", "");
    }
    inertCounts.set(child, count + 1);
    marked.push(child);
  }
  let released = false;
  return {
    id,
    unlock: () => {
      if (released) {
        return;
      }
      released = true;
      stack.splice(stack.indexOf(id), 1);
      scrollLocks -= 1;
      if (scrollLocks === 0) {
        document.body.style.overflow = savedOverflow;
      }
      for (const child of marked) {
        const count = (inertCounts.get(child) ?? 1) - 1;
        if (count <= 0) {
          inertCounts.delete(child);
          child.removeAttribute("inert");
        } else {
          inertCounts.set(child, count);
        }
      }
    },
  };
}

/** いま一番上に開いているモーダルか。 */
export function isTopModal(id: symbol): boolean {
  return stack[stack.length - 1] === id;
}
