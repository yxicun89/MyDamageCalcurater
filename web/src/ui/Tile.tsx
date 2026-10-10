// G-05(ADR-0339): タイル。選択肢が少なく絵で選べるとき(構築の 6 枠など)の押せる四角。
// selected を渡すと aria-pressed + チェックの印(色だけに頼らない)。empty は「+」と見える名前の空き枠。

import type { ReactElement, ReactNode } from "react";
import { uiText } from "../i18n/ui";
import { Icon } from "./Icon";

export interface TileProps {
  readonly onClick: () => void;
  /** 選べるタイルのとき、選択中か。渡さなければ aria-pressed を付けない。 */
  readonly selected?: boolean;
  /** 空き枠(「+」のタイル)。 */
  readonly empty?: boolean;
  /** 空き枠の見える名前。既定は「追加」。 */
  readonly label?: string;
  readonly disabled?: boolean;
  readonly className?: string;
  readonly children?: ReactNode;
}

export function Tile({
  onClick,
  selected,
  empty = false,
  label = uiText.add,
  disabled = false,
  className,
  children,
}: TileProps): ReactElement {
  const classes = ["ui-tile"];
  if (empty) {
    classes.push("ui-tile--empty");
  }
  if (selected === true) {
    classes.push("ui-tile--selected");
  }
  if (className !== undefined) {
    classes.push(className);
  }
  return (
    <button
      type="button"
      className={classes.join(" ")}
      aria-pressed={selected}
      disabled={disabled}
      onClick={onClick}
    >
      {empty ? (
        <>
          <Icon name="plus" size={24} />
          {label}
        </>
      ) : (
        <>
          {selected === true && <Icon name="check" size={16} className="ui-tile__mark" />}
          {children}
        </>
      )}
    </button>
  );
}
