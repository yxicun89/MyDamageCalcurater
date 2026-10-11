// G-05(ADR-0339): 増減ボタン。SP(0〜32)の − / + と数値欄。数値欄は残す(直接打てる)。
// 名前は「<欄の名前>を増やす/減らす」(i18n/ui.ts。用語集「共通の部品の語」)。押せる範囲は 44px(CSS)。

import { useState, type ReactElement } from "react";
import { uiText } from "../i18n/ui";
import { Icon } from "./Icon";

export interface StepperProps {
  /** 欄の名前(見えるラベルと同じ語。例: 「攻撃の能力ポイント」)。 */
  readonly label: string;
  readonly value: number;
  readonly min: number;
  readonly max: number;
  readonly step?: number;
  readonly onChange: (value: number) => void;
  readonly disabled?: boolean;
  readonly className?: string;
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

export function Stepper({
  label,
  value,
  min,
  max,
  step = 1,
  onChange,
  disabled = false,
  className,
}: StepperProps): ReactElement {
  // 打っている途中の欄の文字。範囲に収めるのは確定(blur / Enter)のときだけ(min が 0 でない欄でも打てるように)。
  const [draft, setDraft] = useState<string | null>(null);
  // 親が value を外から変えたら、打っている途中の文字は捨てて新しい値を出す(レンダー中の state 調整)。
  const [seenValue, setSeenValue] = useState(value);
  if (seenValue !== value) {
    setSeenValue(value);
    setDraft(null);
  }
  const atMin = value <= min;
  const atMax = value >= max;

  function commit(next: number): void {
    setDraft(null);
    onChange(clamp(next, min, max));
  }

  function commitDraft(): void {
    if (draft === null) {
      return;
    }
    const number = Number(draft);
    setDraft(null);
    if (draft.trim() !== "" && !Number.isNaN(number)) {
      onChange(clamp(Math.trunc(number), min, max));
    }
  }

  return (
    <div
      role="group"
      aria-label={label}
      className={className === undefined ? "ui-stepper" : `ui-stepper ${className}`}
    >
      <button
        type="button"
        className="ui-stepper__button"
        aria-label={`${label}${uiText.stepperDecrease}`}
        disabled={disabled}
        aria-disabled={atMin ? true : undefined}
        onClick={() => {
          if (!atMin) {
            commit(value - step);
          }
        }}
      >
        <Icon name="minus" size={16} />
      </button>
      <input
        type="number"
        inputMode="numeric"
        className="ui-stepper__input"
        aria-label={label}
        min={min}
        max={max}
        step={step}
        disabled={disabled}
        value={draft ?? String(value)}
        onChange={(event) => {
          const text = event.target.value;
          setDraft(text);
          const number = Number(text);
          // 範囲に入った整数になったときだけ、打ちながら通知する。範囲外・空は確定(blur / Enter)まで待つ。
          if (text.trim() !== "" && Number.isInteger(number) && number >= min && number <= max) {
            onChange(number);
          }
        }}
        onBlur={commitDraft}
        onKeyDown={(event) => {
          if (event.key === "Enter") {
            commitDraft();
          }
        }}
      />
      <button
        type="button"
        className="ui-stepper__button"
        aria-label={`${label}${uiText.stepperIncrease}`}
        disabled={disabled}
        aria-disabled={atMax ? true : undefined}
        onClick={() => {
          if (!atMax) {
            commit(value + step);
          }
        }}
      >
        <Icon name="plus" size={16} />
      </button>
    </div>
  );
}
