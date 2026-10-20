// issue 272(ADR-0311): 特性セレクト。計算画面(攻撃側・防御側)と逆算画面(自分・相手)で共通に使う。
// autoOptionLabel を渡すと先頭に「おまかせ」(value は "")を足す。選択肢が 0 件のとき(特性の無いマスタ)は出さない。

import { useId } from "react";
import type { Ability } from "../engine/types";
import { calcScreenText } from "../i18n/ja";

interface AbilitySelectProps extends AbilitySelectConfig {
  /** 見えるラベルの CSS クラス(カードごとに違う)。 */
  readonly labelClassName: string;
}

/** カードごとに決まる設定(ラベルの CSS クラス以外)。 */
export interface AbilitySelectConfig {
  /** select の accessible name(「攻撃側の特性」等)。 */
  readonly ariaLabel: string;
  readonly options: readonly Ability[];
  /** 選択中の特性の ID。おまかせは ""。 */
  readonly value: string;
  readonly onChange: (id: string) => void;
  /** 先頭の「おまかせ」の文言。省略すると「おまかせ」を出さない。 */
  readonly autoOptionLabel?: string;
}

export function AbilitySelect({
  ariaLabel,
  labelClassName,
  options,
  value,
  onChange,
  autoOptionLabel,
}: AbilitySelectProps) {
  const selectId = useId();
  if (options.length === 0) {
    return null;
  }
  return (
    <>
      <label className={labelClassName} htmlFor={selectId}>
        {calcScreenText.abilityFieldLabel}
      </label>
      <select
        id={selectId}
        aria-label={ariaLabel}
        value={value}
        onChange={(event) => {
          onChange(event.target.value);
        }}
      >
        {autoOptionLabel !== undefined && <option value="">{autoOptionLabel}</option>}
        {options.map((ability) => (
          <option key={ability.id} value={ability.id}>
            {ability.nameJa}
          </option>
        ))}
      </select>
    </>
  );
}
