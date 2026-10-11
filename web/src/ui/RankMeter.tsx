// ADR-0357: 能力ランクのメーター。左 6 段(下降)・中央 0・右 6 段(上昇)の 13 段階。
// タップ / ドラッグ / 矢印キー / − + ボタンで動かす。読み上げは role="slider" の aria-valuetext(「攻撃 +2」)。
// 色だけに頼らない: 符号付きの値を文字でも出し、0 の目印は常に見える。上昇 = brand.primary、下降 = warning。

import { useRef, type KeyboardEvent, type PointerEvent, type ReactElement } from "react";
import { uiText } from "../i18n/ui";
import { Icon } from "./Icon";

export const RANK_MIN = -6;
export const RANK_MAX = 6;
/** PageUp / PageDown の歩幅。 */
const BIG_STEP = 3;

export interface RankMeterProps {
  /** 欄の名前(group の名前。例: 「攻撃側のランク」)。− / + は「<label>を下げる/上げる」。 */
  readonly label: string;
  /** 読み上げ用のステータス名(「攻撃」)。aria-valuetext は「攻撃 +2」。 */
  readonly statName: string;
  /** 見える値の表記(「A +2」など)。省くと符号付きの数だけ。 */
  readonly valueLabel?: string;
  readonly value: number;
  readonly onChange: (value: number) => void;
  /** − / + の名前を上書きする(既定は `${label}を下げる` / `${label}を上げる`)。 */
  readonly decreaseLabel?: string;
  readonly increaseLabel?: string;
  readonly disabled?: boolean;
  /** false にすると外側の role="group" を付けない(外が fieldset + legend で同じ名前の group を作るとき)。 */
  readonly asGroup?: boolean;
  /** 誤りの欄(aria-invalid)と、その説明の id(aria-describedby)。 */
  readonly invalid?: boolean;
  readonly describedBy?: string;
}

/** 符号付きの表記(+2 / -3 / ±0)。 */
export function signedRank(value: number): string {
  return value > 0 ? `+${String(value)}` : value < 0 ? String(value) : "±0";
}

function clamp(value: number): number {
  return Math.min(RANK_MAX, Math.max(RANK_MIN, Math.round(value)));
}

const STEPS = Array.from({ length: RANK_MAX - RANK_MIN + 1 }, (_, index) => RANK_MIN + index);

function segmentState(step: number, value: number): "up" | "down" | "zero" | "off" {
  if (step === 0) {
    return "zero";
  }
  if (value > 0 && step > 0 && step <= value) {
    return "up";
  }
  if (value < 0 && step < 0 && step >= value) {
    return "down";
  }
  return "off";
}

export function RankMeter({
  label,
  statName,
  valueLabel,
  value,
  onChange,
  decreaseLabel,
  increaseLabel,
  disabled = false,
  asGroup = true,
  invalid = false,
  describedBy,
}: RankMeterProps): ReactElement {
  const trackRef = useRef<HTMLDivElement>(null);
  const dragging = useRef(false);
  const current = clamp(value);

  function stepAt(clientX: number): number | null {
    const rect = trackRef.current?.getBoundingClientRect();
    if (rect === undefined || rect.width <= 0) {
      return null;
    }
    const ratio = (clientX - rect.left) / rect.width;
    return clamp(RANK_MIN + ratio * (RANK_MAX - RANK_MIN + 1) - 0.5);
  }

  function moveTo(next: number): void {
    if (next !== current) {
      onChange(next);
    }
  }

  function onPointerDown(event: PointerEvent<HTMLDivElement>): void {
    // マウスは主ボタン(左)だけ。右・中クリックでは動かさない。
    if (disabled || (event.pointerType === "mouse" && event.button !== 0)) {
      return;
    }
    const next = stepAt(event.clientX);
    if (next === null) {
      return;
    }
    dragging.current = true;
    // jsdom には setPointerCapture が無いので、あるときだけ呼ぶ(実ブラウザでは常にある)。
    // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
    event.currentTarget.setPointerCapture?.(event.pointerId);
    event.currentTarget.focus();
    moveTo(next);
  }

  function onPointerMove(event: PointerEvent<HTMLDivElement>): void {
    if (!dragging.current) {
      return;
    }
    const next = stepAt(event.clientX);
    if (next !== null) {
      moveTo(next);
    }
  }

  function endDrag(): void {
    dragging.current = false;
  }

  function onKeyDown(event: KeyboardEvent<HTMLDivElement>): void {
    if (disabled) {
      return;
    }
    const targets: Partial<Record<string, number>> = {
      ArrowRight: current + 1,
      ArrowUp: current + 1,
      ArrowLeft: current - 1,
      ArrowDown: current - 1,
      PageUp: current + BIG_STEP,
      PageDown: current - BIG_STEP,
      Home: RANK_MIN,
      End: RANK_MAX,
    };
    const target = targets[event.key];
    if (target === undefined) {
      return;
    }
    event.preventDefault();
    moveTo(clamp(target));
  }

  return (
    <div {...(asGroup ? { role: "group", "aria-label": label } : {})} className="ui-rank">
      <button
        type="button"
        className="ui-stepper__button"
        aria-label={decreaseLabel ?? `${label}${uiText.rankDecreaseSuffix}`}
        disabled={disabled || current <= RANK_MIN}
        onClick={() => {
          moveTo(clamp(current - 1));
        }}
      >
        <Icon name="minus" size={16} />
      </button>
      <div
        ref={trackRef}
        role="slider"
        tabIndex={disabled ? -1 : 0}
        aria-label={label}
        aria-orientation="horizontal"
        aria-valuemin={RANK_MIN}
        aria-valuemax={RANK_MAX}
        aria-valuenow={current}
        aria-valuetext={`${statName} ${signedRank(current)}`}
        aria-disabled={disabled ? true : undefined}
        aria-invalid={invalid ? true : undefined}
        aria-describedby={describedBy}
        className="ui-rank__track"
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={endDrag}
        onPointerCancel={endDrag}
        onLostPointerCapture={endDrag}
        onKeyDown={onKeyDown}
      >
        {STEPS.map((step) => (
          <span
            key={step}
            aria-hidden="true"
            className="ui-rank__segment"
            data-state={segmentState(step, current)}
          />
        ))}
      </div>
      <button
        type="button"
        className="ui-stepper__button"
        aria-label={increaseLabel ?? `${label}${uiText.rankIncreaseSuffix}`}
        disabled={disabled || current >= RANK_MAX}
        onClick={() => {
          moveTo(clamp(current + 1));
        }}
      >
        <Icon name="plus" size={16} />
      </button>
      <span className="ui-rank__value">{valueLabel ?? signedRank(current)}</span>
    </div>
  );
}
