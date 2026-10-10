// G-05(ADR-0339): 計算画面の条件の入力(急所・やけど・天候・壁・持ち物の候補を比べる)のチップ。
// ネイティブのチェックボックス・ラジオを包む label(`.ui-chip`)で、選択中は塗り + チェックの印(色だけに頼らない)。
// 入力そのものは残す(role・名前・キーボード操作はネイティブのまま)。

import type { ReactElement } from "react";
import { Icon } from "../ui/Icon";

interface ToggleChipProps {
  readonly type: "checkbox" | "radio";
  readonly label: string;
  readonly checked: boolean;
  readonly onChange: (checked: boolean) => void;
  /** ラジオのグループ名(同じ欄の選択肢で共通)。 */
  readonly name?: string;
  readonly disabled?: boolean;
  readonly describedBy?: string;
}

export function ToggleChip({
  type,
  label,
  checked,
  onChange,
  name,
  disabled = false,
  describedBy,
}: ToggleChipProps): ReactElement {
  return (
    <label className={`ui-chip calc-chip${checked ? " ui-chip--selected" : ""}`}>
      <input
        type={type}
        name={name}
        className="calc-chip__input"
        checked={checked}
        disabled={disabled}
        aria-describedby={describedBy}
        onChange={(event) => {
          onChange(event.target.checked);
        }}
      />
      {checked && <Icon name="check" size={16} />}
      {label}
    </label>
  );
}
