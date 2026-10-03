// issue 515・ADR-0320: メガ種族の持ち物固定の理由(見える文言)。持ち物欄の aria-describedby から結ぶ。
// 計算・逆算の両画面が使う(文言は i18n/ja.ts の megaItemText)。

import type { MegaItemLock } from "../domain/mega";
import { megaItemText } from "../i18n/ja";

/** 固定の理由の文言。固定されていない(none)ときは null。 */
export function megaLockReasonText(lock: MegaItemLock): string | null {
  switch (lock.kind) {
    case "none":
      return null;
    case "locked":
      return megaItemText.lockedReason;
    case "missing":
      return megaItemText.missingReason;
  }
}

interface MegaItemReasonProps {
  readonly id: string;
  readonly lock: MegaItemLock;
  readonly className: string;
}

export function MegaItemReason({ id, lock, className }: MegaItemReasonProps) {
  const text = megaLockReasonText(lock);
  if (text === null) {
    return null;
  }
  return (
    <p id={id} className={className}>
      {text}
    </p>
  );
}
