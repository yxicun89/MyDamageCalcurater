// ADR-0144 §3(I-web-13): 計算画面の「対戦の状態」。攻撃側・防御側の残り HP の入力(折りたたみ。既定は閉じる)と、
// 範囲の多段技のときだけ出す回数の選択。状態は持たない(入力は CalcScreen が持つ)。開閉の状態だけここに持つ。
// 最大 HP が分からない側(種族が未選択等)は欄を出さない。範囲外は欄の下に誤りを出す(送らない判定は CalcScreen)。

import { useId, useState, type ReactElement } from "react";
import { hitsOptions, parseHpInput, percentText } from "../domain/battleState";
import { battleStateText } from "../i18n/ja";

interface HpFieldProps {
  readonly label: string;
  readonly hint: string;
  readonly max: number;
  readonly value: string;
  readonly clampedTo: number | null;
  readonly onChange: (next: string) => void;
}

function HpField({ label, hint, max, value, clampedTo, onChange }: HpFieldProps): ReactElement {
  const id = useId();
  const hintId = `${id}-hint`;
  const errorId = `${id}-error`;
  const parsed = parseHpInput(value, max);
  const invalid = parsed.kind === "error";
  return (
    <div className="ui-field battle-state__field">
      <label htmlFor={id} className="battle-state__label">
        {label}
      </label>
      <div className="battle-state__row">
        <input
          id={id}
          type="text"
          inputMode="numeric"
          autoComplete="off"
          className="battle-state__input"
          value={value}
          aria-invalid={invalid}
          aria-describedby={invalid ? `${hintId} ${errorId}` : hintId}
          onChange={(event) => {
            onChange(event.target.value);
          }}
        />
        <span className="battle-state__max">{battleStateText.maxSuffix(max)}</span>
        {parsed.kind === "ok" && (
          <span className="battle-state__percent">{percentText(parsed.value, max)}</span>
        )}
      </div>
      <p id={hintId} className="battle-state__hint">
        {hint}
      </p>
      {invalid && (
        <p id={errorId} role="alert" className="battle-state__error">
          {battleStateText.hpError(max)}
        </p>
      )}
      {clampedTo !== null && (
        <p role="status" className="battle-state__hint">
          {battleStateText.clamped(clampedTo)}
        </p>
      )}
    </div>
  );
}

interface BattleStatePanelProps {
  readonly attackerHp: string;
  readonly defenderHp: string;
  /** 最大 HP(実数値)。null は欄を出さない。 */
  readonly attackerMaxHp: number | null;
  readonly defenderMaxHp: number | null;
  /** 最大 HP に合わせて丸めた旨を出す値(丸めていなければ null)。 */
  readonly attackerClampedTo: number | null;
  readonly defenderClampedTo: number | null;
  /** 何か指定している(閉じていても分かる目印に使う)。 */
  readonly active: boolean;
  readonly onAttackerHpChange: (next: string) => void;
  readonly onDefenderHpChange: (next: string) => void;
}

export function BattleStatePanel({
  attackerHp,
  defenderHp,
  attackerMaxHp,
  defenderMaxHp,
  attackerClampedTo,
  defenderClampedTo,
  active,
  onAttackerHpChange,
  onDefenderHpChange,
}: BattleStatePanelProps): ReactElement | null {
  const [open, setOpen] = useState(false);
  const bodyId = useId();
  if (attackerMaxHp === null && defenderMaxHp === null) {
    return null;
  }
  return (
    <div className="battle-state">
      <button
        type="button"
        className="ui-button ui-button--secondary calc-conditions__toggle"
        aria-expanded={open}
        aria-controls={bodyId}
        onClick={() => {
          setOpen(!open);
        }}
      >
        {battleStateText.toggleLabel}
        {active && battleStateText.activeMark}
      </button>
      <div id={bodyId}>
        {open && (
          <div className="battle-state__body">
            {attackerMaxHp !== null && (
              <HpField
                label={battleStateText.attackerHpLabel}
                hint={battleStateText.attackerHpHint}
                max={attackerMaxHp}
                value={attackerHp}
                clampedTo={attackerClampedTo}
                onChange={onAttackerHpChange}
              />
            )}
            {defenderMaxHp !== null && (
              <HpField
                label={battleStateText.defenderHpLabel}
                hint={battleStateText.defenderHpHint}
                max={defenderMaxHp}
                value={defenderHp}
                clampedTo={defenderClampedTo}
                onChange={onDefenderHpChange}
              />
            )}
          </div>
        )}
      </div>
    </div>
  );
}

interface HitsSelectProps {
  readonly range: { readonly min: number; readonly max: number };
  /** null は「既定」。 */
  readonly value: number | null;
  readonly onChange: (next: number | null) => void;
}

/** 多段の回数(範囲の多段技のときだけ CalcScreen が出す)。 */
export function HitsSelect({ range, value, onChange }: HitsSelectProps): ReactElement {
  const id = useId();
  return (
    <div className="ui-field battle-state__field">
      <label htmlFor={id} className="battle-state__label">
        {battleStateText.hitsLabel}
      </label>
      <select
        id={id}
        value={value === null ? "" : String(value)}
        onChange={(event) => {
          onChange(event.target.value === "" ? null : Number(event.target.value));
        }}
      >
        <option value="">{battleStateText.hitsDefault(range.min, range.max)}</option>
        {hitsOptions(range).map((count) => (
          <option key={count} value={String(count)}>
            {battleStateText.hitsOption(count)}
          </option>
        ))}
      </select>
    </div>
  );
}
