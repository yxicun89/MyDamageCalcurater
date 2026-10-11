// G-05(ADR-0339): 説明ボタン。画面に説明の文を出し続けず、見出しの横の「説明」で 1 段だけ開く(本文は 3 行まで)。

import { useId, useState, type ReactElement, type ReactNode } from "react";
import { uiText } from "../i18n/ui";
import { Icon } from "./Icon";

export interface HelpButtonProps {
  /** 見える名前。既定は「説明」。 */
  readonly label?: string;
  /** 開いたときの本文(3 行まで)。 */
  readonly children: ReactNode;
}

export function HelpButton({ label = uiText.help, children }: HelpButtonProps): ReactElement {
  const bodyId = useId();
  const [open, setOpen] = useState(false);
  return (
    <span
      className="ui-help"
      onKeyDown={(event) => {
        if (open && event.key === "Escape") {
          event.stopPropagation();
          setOpen(false);
        }
      }}
    >
      <button
        type="button"
        className="ui-help__button"
        aria-expanded={open}
        aria-controls={open ? bodyId : undefined}
        onClick={() => {
          setOpen((current) => !current);
        }}
      >
        <Icon name="help" size={16} />
        {label}
      </button>
      {open && (
        <span id={bodyId} className="ui-help__body">
          {children}
        </span>
      )}
    </span>
  );
}
